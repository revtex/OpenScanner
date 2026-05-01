package trmqtt

import (
	"context"
	"errors"
	"time"
)

// TestConnect performs a one-shot connect+subscribe attempt against the broker
// configured in cfg, returning nil on a successful CONNACK and a redacted
// error otherwise. It is intended for the admin "test connection" REST
// endpoint and never logs or returns the broker password in the error.
//
// The supplied ctx bounds the entire attempt; callers should pass a
// context.WithTimeout (5s in the admin handler).
func TestConnect(ctx context.Context, cfg ClientConfig) error {
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	snap := NewSnapshot(cfg.InstanceID, cfg.Label)
	c := NewClient(cfg, snap, noopEmitter{}, nil)
	if err := c.Start(ctx); err != nil {
		return err
	}
	// Always drain shutdown asynchronously — never block the response on
	// the broker disconnect handshake.
	defer func() {
		go func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = c.Stop(stopCtx)
		}()
	}()
	if err := c.AwaitConnection(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return errors.New("connection timed out")
		}
		return err
	}
	return nil
}

// noopEmitter discards all events; used by TestConnect to avoid wiring a
// Manager just for a one-shot dial.
type noopEmitter struct{}

func (noopEmitter) emit(Event) {}
