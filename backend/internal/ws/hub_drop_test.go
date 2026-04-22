package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/db"
)

// fillSend pre-fills c.send to its capacity so every subsequent trySend drops.
func fillSend(t *testing.T, c *Client) {
	t.Helper()
	for {
		select {
		case c.send <- []byte("filler"):
		default:
			return
		}
	}
}

func TestHub_DropCounter_IncrementsOnSlowClient(t *testing.T) {
	// No hub loop needed — trySend is a property of the Client.
	c := &Client{send: make(chan []byte, 4)}
	fillSend(t, c)

	const extra = 150
	for i := 0; i < extra; i++ {
		c.trySend([]byte(`["TEST"]`))
	}

	if got := c.dropCount.Load(); got != extra {
		t.Fatalf("dropCount = %d, want %d", got, extra)
	}
}

func TestHub_DropCounter_LogsEveryNDrops(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	var buf bytes.Buffer
	var bufMu sync.Mutex
	// Wrap buf in a locking writer so slog's concurrent writes don't race.
	slog.SetDefault(slog.New(slog.NewJSONHandler(&lockedWriter{w: &buf, mu: &bufMu}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	c := &Client{send: make(chan []byte, 1)}
	fillSend(t, c)

	const drops = 250
	for i := 0; i < drops; i++ {
		c.trySend([]byte(`["TEST"]`))
	}

	bufMu.Lock()
	out := buf.String()
	bufMu.Unlock()

	// Count log lines that contain the "slow client dropping messages" marker.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	logged := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if msg, _ := rec["msg"].(string); strings.Contains(msg, "slow client dropping messages") {
			logged++
		}
	}

	// The warning fires every 100 drops → drops=250 → 2 log lines (at 100 and 200).
	wantMin := drops / 100
	if logged != wantMin {
		t.Fatalf("log line count for slow-client warnings = %d, want %d (drops=%d, buf=%q)", logged, wantMin, drops, out)
	}
}

// lockedWriter serialises concurrent writes for slog test capture.
type lockedWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func TestHub_ClientCloseOnce_NoPanicOnDoubleClose(t *testing.T) {
	c := &Client{send: make(chan []byte, 1)}

	c.closeSend()
	// Second call must be a no-op (sync.Once) — not a panic.
	c.closeSend()

	// Channel must be closed: reading returns zero value, ok=false.
	select {
	case _, ok := <-c.send:
		if ok {
			t.Fatal("expected c.send to be closed, but read a value")
		}
	case <-time.After(time.Second):
		t.Fatal("read from closed channel blocked")
	}

	// trySend after close must not panic (recover in trySend swallows).
	c.trySend([]byte("post-close"))
}

func TestHub_DisconnectByUser_ClosesAllSessionsForUser(t *testing.T) {
	hub, _ := newTestHub(t)

	a1 := &Client{hub: hub, send: make(chan []byte, sendBufSize), userID: 10, jti: "a1"}
	a2 := &Client{hub: hub, send: make(chan []byte, sendBufSize), userID: 10, jti: "a2"}
	b1 := &Client{hub: hub, send: make(chan []byte, sendBufSize), userID: 20, jti: "b1"}

	hub.Register(a1)
	hub.Register(a2)
	hub.Register(b1)
	waitForClientCount(t, hub, 3, 2*time.Second)

	hub.DisconnectByUser(10)

	// Both A clients must be unregistered; B untouched.
	waitForClientCount(t, hub, 1, 2*time.Second)

	// Verify A's clients each received an XPR message before close.
	for i, c := range []*Client{a1, a2} {
		select {
		case got, ok := <-c.send:
			if !ok {
				t.Fatalf("client a%d: channel closed before XPR drained", i+1)
			}
			if !bytes.Contains(got, []byte(`"XPR"`)) {
				t.Errorf("client a%d: got %q, want XPR", i+1, got)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("client a%d: timed out waiting for XPR", i+1)
		}
	}

	// B still connected: its send channel must be open and empty.
	select {
	case <-b1.send:
		t.Fatal("b1 should not have received XPR")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHub_DisconnectByJTI_ClosesOnlyMatchingSession(t *testing.T) {
	hub, _ := newTestHub(t)

	c1 := &Client{hub: hub, send: make(chan []byte, sendBufSize), userID: 10, jti: "jti-one"}
	c2 := &Client{hub: hub, send: make(chan []byte, sendBufSize), userID: 10, jti: "jti-two"}

	hub.Register(c1)
	hub.Register(c2)
	waitForClientCount(t, hub, 2, 2*time.Second)

	hub.DisconnectByJTI("jti-one")
	waitForClientCount(t, hub, 1, 2*time.Second)

	// c1 got XPR; c2 did not.
	select {
	case got, ok := <-c1.send:
		if !ok || !bytes.Contains(got, []byte(`"XPR"`)) {
			t.Fatalf("c1: got %q ok=%v, want XPR", got, ok)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("c1: timed out waiting for XPR")
	}

	select {
	case got := <-c2.send:
		t.Fatalf("c2 unexpectedly received %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHub_DisconnectByUser_NoOp_WhenAbsent(t *testing.T) {
	hub, _ := newTestHub(t)

	c := &Client{hub: hub, send: make(chan []byte, sendBufSize), userID: 1, jti: "only"}
	hub.Register(c)
	waitForClientCount(t, hub, 1, 2*time.Second)

	// User 999 has no sessions — must not panic and must not affect others.
	hub.DisconnectByUser(999)

	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("ClientCount = %d, want 1", got)
	}
	select {
	case v := <-c.send:
		t.Fatalf("unrelated client received %q", v)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestHub_LSCDebounce_Skipped documents that LSC debounce cannot be tested
// deterministically because the hub uses time.AfterFunc with a hardcoded
// 3-second duration and no injection point. Flakily asserting timing with
// real clocks would require a time.Sleep, which is forbidden by the testing
// conventions. Skipped with rationale.
func TestHub_LSCDebounce_Skipped(t *testing.T) {
	t.Skip("LSC debounce uses time.AfterFunc(3s) with no clock injection; cannot test without time.Sleep")
}

// Silence unused-import warning when DB isn't referenced (it's used via newTestHub).
var _ = db.New
var _ = context.Background
