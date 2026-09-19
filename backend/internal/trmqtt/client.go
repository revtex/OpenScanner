package trmqtt

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

// ClientConfig is the per-instance Client configuration. Password is plaintext
// at construction time and zeroed inside Run() once the connection has been
// dialled. It must NEVER be logged.
type ClientConfig struct {
	InstanceID       int64  // tr_instances.id
	Label            string // human-friendly label for logs
	PluginInstanceID string // TR config.instance_id (used for routing-mismatch defense)

	BrokerURL     string
	BaseTopic     string
	UnitTopic     string
	MessageTopic  string
	Username      string
	Password      string // plaintext; cleared after dial
	TLSSkipVerify bool   // honored by user request; explicitly insecure
	QoS           byte

	// ReconnectBackoff overrides autopaho's default for tests.
	ReconnectBackoff time.Duration
	ConnectTimeout   time.Duration
}

// Client wraps an autopaho ConnectionManager for a single tr_instances row.
// It is created and supervised by Manager. Tests can construct a Client
// directly and pass an emitter recorder.
type Client struct {
	cfg    ClientConfig
	cm     *autopaho.ConnectionManager
	sub    *subscriber
	out    eventEmitter
	cancel context.CancelFunc
	done   chan struct{}

	mu        sync.Mutex
	closed    bool
	startedAt time.Time
}

// NewClient builds a Client with snapshot/subscriber wired. The connection is
// not opened until Start is called.
func NewClient(cfg ClientConfig, snapshot *Snapshot, out eventEmitter, enrich Enricher) *Client {
	if enrich == nil {
		enrich = noopEnricher{}
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	if cfg.ReconnectBackoff == 0 {
		cfg.ReconnectBackoff = 5 * time.Second
	}
	c := &Client{
		cfg:  cfg,
		out:  out,
		done: make(chan struct{}),
	}
	c.sub = &subscriber{
		instanceID:     cfg.InstanceID,
		label:          cfg.Label,
		expectedPlugin: cfg.PluginInstanceID,
		baseTopic:      cfg.BaseTopic,
		unitTopic:      cfg.UnitTopic,
		messageTopic:   cfg.MessageTopic,
		snapshot:       snapshot,
		out:            out,
		enrich:         enrich,
	}
	return c
}

// Start dials the broker and subscribes to the configured topics. It runs the
// supervisor goroutine until ctx is cancelled or Stop is called. Start
// returns immediately after the initial dial attempt is queued; reconnects
// are handled by autopaho.
func (c *Client) Start(ctx context.Context) error {
	u, err := url.Parse(c.cfg.BrokerURL)
	if err != nil {
		return fmt.Errorf("parse broker url: %w", err)
	}
	// Hide the password as soon as we've staged it for the dialer. autopaho
	// internally stores ConnectPassword, but we also clear our held copy so
	// callers who Get the cfg later see no plaintext.
	password := []byte(c.cfg.Password)
	c.cfg.Password = ""

	subCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	clientID := fmt.Sprintf("openscanner-tr-%d-%d", c.cfg.InstanceID, time.Now().UnixNano())

	apCfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{u},
		KeepAlive:                     30,
		CleanStartOnInitialConnection: true,
		SessionExpiryInterval:         60,
		ConnectUsername:               c.cfg.Username,
		ConnectPassword:               password,
		ConnectTimeout:                c.cfg.ConnectTimeout,
		ReconnectBackoff:              func(int) time.Duration { return c.cfg.ReconnectBackoff },

		// TLS skip-verify is honored by user request. It is explicitly insecure
		// and only applies when the broker URL is tls:// or wss://.
		TlsCfg: c.tlsConfig(),

		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			pkgMetrics.connected.Add(1)
			c.sub.snapshot.setConnected()
			slog.Info("trmqtt: connected to broker",
				"instance_id", c.cfg.InstanceID, "label", c.cfg.Label, "url", redactURL(u))
			c.out.emit(Event{Type: EventInstanceConnected, InstanceID: c.cfg.InstanceID, Label: c.cfg.Label})

			specs := c.sub.topicSpecs(c.cfg.QoS)
			subs := make([]paho.SubscribeOptions, len(specs))
			for i, s := range specs {
				subs[i] = paho.SubscribeOptions{Topic: s.filter, QoS: s.qos}
			}
			subCtx2, subCancel := context.WithTimeout(subCtx, c.cfg.ConnectTimeout)
			defer subCancel()
			if _, err := cm.Subscribe(subCtx2, &paho.Subscribe{Subscriptions: subs}); err != nil {
				slog.Error("trmqtt: subscribe failed",
					"instance_id", c.cfg.InstanceID, "error", err)
			}
		},
		OnConnectionDown: func() bool {
			pkgMetrics.disconnects.Add(1)
			c.sub.snapshot.setDisconnected("connection lost")
			slog.Warn("trmqtt: connection down",
				"instance_id", c.cfg.InstanceID, "label", c.cfg.Label)
			c.out.emit(Event{Type: EventInstanceDisconnected, InstanceID: c.cfg.InstanceID, Label: c.cfg.Label, Err: errors.New("connection down")})
			return true // keep trying
		},
		OnConnectError: func(err error) {
			pkgMetrics.connectAttempts.Add(1)
			slog.Warn("trmqtt: connect attempt failed",
				"instance_id", c.cfg.InstanceID, "error", err)
		},
		ClientConfig: paho.ClientConfig{
			ClientID: clientID,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					c.dispatch(subCtx, pr.Packet)
					return true, nil
				},
			},
			OnServerDisconnect: func(d *paho.Disconnect) {
				slog.Warn("trmqtt: server-initiated disconnect",
					"instance_id", c.cfg.InstanceID, "reason_code", d.ReasonCode)
			},
		},
	}

	cm, err := autopaho.NewConnection(subCtx, apCfg)
	if err != nil {
		cancel()
		return fmt.Errorf("autopaho.NewConnection: %w", err)
	}
	c.cm = cm
	c.startedAt = time.Now()

	// We've handed the password bytes to autopaho's connect goroutine; it
	// reads them asynchronously when establishing the TCP/TLS session, so we
	// must NOT mutate the slice here (race vs. paho.Client.Connect). The
	// plaintext is already cleared from c.cfg.Password above; autopaho holds
	// the bytes only until the CONNECT packet is on the wire.

	go func() {
		defer close(c.done)
		<-cm.Done()
	}()
	return nil
}

// Stop disconnects the client and waits for the supervisor goroutine to exit
// (bounded by the supplied ctx).
func (c *Client) Stop(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	if c.cm != nil {
		_ = c.cm.Disconnect(ctx)
	}
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AwaitConnection blocks until the client is connected or ctx is cancelled.
// Useful in tests.
func (c *Client) AwaitConnection(ctx context.Context) error {
	if c.cm == nil {
		return errors.New("client not started")
	}
	return c.cm.AwaitConnection(ctx)
}

// dispatch is the per-message hot path. It enforces the 256 KB cap and hands
// off to the subscriber.
func (c *Client) dispatch(ctx context.Context, p *paho.Publish) {
	if p == nil {
		return
	}
	if len(p.Payload) > MaxPayloadBytes {
		pkgMetrics.dropOversize.Add(1)
		// Never log payload contents.
		slog.Warn("trmqtt: dropped oversize payload",
			"instance_id", c.cfg.InstanceID, "topic", p.Topic, "size", len(p.Payload))
		return
	}
	c.sub.handle(ctx, p.Topic, p.Payload)
}

// tlsConfig builds a tls.Config when TLSSkipVerify is set. autopaho only
// applies this when the URL scheme is tls:// or wss://.
func (c *Client) tlsConfig() *tls.Config {
	if !c.cfg.TLSSkipVerify {
		return nil
	}
	// #nosec G402 — explicitly opt-in by user; documented in admin UI as insecure.
	return &tls.Config{InsecureSkipVerify: true}
}

// redactURL strips userinfo from URLs before logging. autopaho passes a
// *url.URL we built ourselves so it shouldn't carry credentials, but we
// strip defensively.
func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	cp := *u
	cp.User = nil
	return cp.String()
}
