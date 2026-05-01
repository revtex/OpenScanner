package trmqtt

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	pahopkts "github.com/eclipse/paho.golang/paho"
)

// startBroker spins up an in-process mochi MQTT broker bound to a random port
// on 127.0.0.1 and returns its broker URL plus a stop func. The broker accepts
// any client (AllowHook).
func startBroker(t *testing.T) (string, *mqtt.Server) {
	t.Helper()

	// Reserve a random local port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv := mqtt.New(&mqtt.Options{InlineClient: true})
	if err := srv.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatalf("add hook: %v", err)
	}
	tcp := listeners.NewTCP(listeners.Config{ID: "test-tcp", Address: addr})
	if err := srv.AddListener(tcp); err != nil {
		t.Fatalf("add listener: %v", err)
	}
	go func() {
		if err := srv.Serve(); err != nil {
			t.Logf("broker serve returned: %v", err)
		}
	}()
	t.Cleanup(func() { _ = srv.Close() })

	// Wait until accepting connections.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return "mqtt://" + addr, srv
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("broker never came up at %s", addr)
	return "", nil
}

// publish helper using the broker's own InlineClient (no second paho conn).
func brokerPublish(t *testing.T, srv *mqtt.Server, topic string, payload []byte) {
	t.Helper()
	if err := srv.Publish(topic, payload, false, 0); err != nil {
		t.Fatalf("publish %s: %v", topic, err)
	}
}

// waitForEvent polls the recordingEmitter until at least one event of typ is
// observed or the deadline passes.
func waitForEvent(t *testing.T, rec *recordingEmitter, typ EventType, d time.Duration) Event {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		for _, ev := range rec.snapshot() {
			if ev.Type == typ {
				return ev
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; observed=%+v", typ, rec.snapshot())
	return Event{}
}

func newTestClient(t *testing.T, brokerURL string, rec *recordingEmitter) (*Client, *Snapshot) {
	t.Helper()
	cfg := ClientConfig{
		InstanceID:       1,
		Label:            "test",
		PluginInstanceID: "site-a",
		BrokerURL:        brokerURL,
		BaseTopic:        "tr",
		UnitTopic:        "tr/units",
		MessageTopic:     "tr/messages",
		QoS:              0,
		ConnectTimeout:   2 * time.Second,
		ReconnectBackoff: 100 * time.Millisecond,
	}
	snap := NewSnapshot(cfg.InstanceID, cfg.Label)
	c := NewClient(cfg, snap, rec, noopEnricher{})
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	if err := c.Start(ctx); err != nil {
		t.Fatalf("client start: %v", err)
	}
	awaitCtx, awaitCancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer awaitCancel()
	if err := c.AwaitConnection(awaitCtx); err != nil {
		t.Fatalf("await connection: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = c.Stop(stopCtx)
	})
	return c, snap
}

func TestClient_ConnectAndReceiveRates(t *testing.T) {
	brokerURL, srv := startBroker(t)
	rec := &recordingEmitter{}
	_, snap := newTestClient(t, brokerURL, rec)

	waitForEvent(t, rec, EventInstanceConnected, 3*time.Second)
	// give the subscribe time to land
	time.Sleep(100 * time.Millisecond)

	brokerPublish(t, srv, "tr/rates", []byte(`{"type":"rates","instance_id":"site-a","rates":[{"sys_num":0,"decoderate":35.5}]}`))
	waitForEvent(t, rec, EventRates, 2*time.Second)

	view := snap.Get()
	if !view.Connection.Connected {
		t.Fatalf("snapshot not connected: %+v", view.Connection)
	}
	if view.Rates.InstanceID != "site-a" {
		t.Fatalf("rates not stored: %+v", view.Rates)
	}
}

func TestClient_DropsOversizePayload(t *testing.T) {
	brokerURL, srv := startBroker(t)
	rec := &recordingEmitter{}
	newTestClient(t, brokerURL, rec)
	waitForEvent(t, rec, EventInstanceConnected, 3*time.Second)
	time.Sleep(100 * time.Millisecond)

	before := pkgMetrics.dropOversize.Load()
	big := make([]byte, MaxPayloadBytes+10)
	for i := range big {
		big[i] = 'a'
	}
	brokerPublish(t, srv, "tr/rates", big)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pkgMetrics.dropOversize.Load() > before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("oversize counter did not increment")
}

func TestClient_PasswordNeverLogged(t *testing.T) {
	brokerURL, srv := startBroker(t)
	rec := &recordingEmitter{}

	// Capture slog output.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	const secret = "correct-horse-battery-staple"
	cfg := ClientConfig{
		InstanceID:       1,
		Label:            "test",
		PluginInstanceID: "site-a",
		BrokerURL:        brokerURL,
		BaseTopic:        "tr",
		Username:         "alice",
		Password:         secret,
		QoS:              0,
		ConnectTimeout:   2 * time.Second,
		ReconnectBackoff: 100 * time.Millisecond,
	}
	snap := NewSnapshot(cfg.InstanceID, cfg.Label)
	c := NewClient(cfg, snap, rec, noopEnricher{})
	if err := c.Start(t.Context()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = c.Stop(stopCtx)
	})
	awaitCtx, awaitCancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer awaitCancel()
	if err := c.AwaitConnection(awaitCtx); err != nil {
		t.Fatalf("await: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // let SUBACK land

	brokerPublish(t, srv, "tr/rates", []byte(`{"type":"rates","instance_id":"site-a"}`))
	waitForEvent(t, rec, EventRates, 2*time.Second)

	if strings.Contains(buf.String(), secret) {
		t.Fatalf("password leaked in slog output:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "enc::") {
		t.Fatalf("encrypted ciphertext appeared in logs")
	}
}

// guard against accidental use of the unused import.
var _ = pahopkts.Publish{}

// brokerError surfaces a recognisable error for use in test reports.
func brokerError(addr string, err error) error { return fmt.Errorf("broker %s: %w", addr, err) }
