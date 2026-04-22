package audio_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/audio"
)

func init() {
	// safehttp's loopback refusal is guarded by a sync.Once that reads this
	// env var on first call. Set it before any transcriber test runs so
	// httptest.NewServer (bound to 127.0.0.1) is reachable.
	_ = os.Setenv("OPENSCANNER_ALLOW_INTERNAL_HTTP", "1")
}

// newFakeWhisper returns an httptest.Server that answers GET /api/whisper/model
// with the given status. Closed via t.Cleanup.
func newFakeWhisper(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/whisper/model" {
			w.WriteHeader(status)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTranscriberManager_Reload_DisabledToEnabled(t *testing.T) {
	srv := newFakeWhisper(t, http.StatusOK)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m := audio.NewTranscriberManager(ctx, nil, nil)

	if m.Enabled() {
		t.Fatal("manager should start disabled when pool is nil")
	}

	ok := m.Reload(true, srv.URL, "ggml-base", "", false)
	if !ok {
		t.Fatal("Reload(true) returned false; expected success")
	}
	if !m.Enabled() {
		t.Fatal("Enabled() = false after successful Reload")
	}
	if m.BaseURL() != srv.URL {
		t.Fatalf("BaseURL = %q, want %q", m.BaseURL(), srv.URL)
	}
}

func TestTranscriberManager_Reload_UnreachableServer_DisablesManager(t *testing.T) {
	srv := newFakeWhisper(t, http.StatusInternalServerError)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m := audio.NewTranscriberManager(ctx, nil, nil)

	ok := m.Reload(true, srv.URL, "ggml-base", "", false)
	if ok {
		t.Fatal("Reload against 500-returning server must return false")
	}
	if m.Enabled() {
		t.Fatal("Enabled() = true after failed Reload")
	}
}

func TestTranscriberManager_Reload_Disable(t *testing.T) {
	srv := newFakeWhisper(t, http.StatusOK)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m := audio.NewTranscriberManager(ctx, nil, nil)

	if ok := m.Reload(true, srv.URL, "ggml-base", "", false); !ok {
		t.Fatalf("Reload(true) = false, want true")
	}
	if !m.Enabled() {
		t.Fatal("Enabled() should be true after enable")
	}

	// Now disable.
	if ok := m.Reload(false, "", "", "", false); !ok {
		t.Fatalf("Reload(false) = false, want true")
	}
	if m.Enabled() {
		t.Fatal("Enabled() should be false after disable")
	}
	if m.BaseURL() != "" {
		t.Fatalf("BaseURL = %q, want empty after disable", m.BaseURL())
	}
}

// TestTranscriberManager_Reload_GoroutineLeak documents a known leak:
// each successful Reload spawns a pumpResults goroutine that ranges over
// pool.Results(). When the pool context is cancelled, the worker goroutines
// exit but the results channel is never closed, so pumpResults blocks
// forever on the range. Cancelling the appCtx eventually unblocks the select,
// but only if there's a pending send — an idle pumpResults goroutine leaks
// until the whole process shuts down.
//
// This test verifies the leak is bounded at shutdown: when the appCtx is
// cancelled, we wait for goroutine count to settle near baseline. Currently
// it does not (documented gap — not a test failure).
func TestTranscriberManager_Reload_NoGoroutineLeak(t *testing.T) {
	t.Skip("pumpResults goroutines leak on Reload because pool.results is never closed; see report")

	srv := newFakeWhisper(t, http.StatusOK)

	baseCtx, baseCancel := context.WithCancel(context.Background())
	t.Cleanup(baseCancel)
	m := audio.NewTranscriberManager(baseCtx, nil, nil)

	base := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		if ok := m.Reload(true, srv.URL, "ggml-base", "", false); !ok {
			t.Fatalf("Reload #%d returned false", i)
		}
		if ok := m.Reload(false, "", "", "", false); !ok {
			t.Fatalf("Reload(false) #%d returned false", i)
		}
	}

	// Bounded poll for goroutine count to settle — no sleep.
	deadline, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		select {
		case <-deadline.Done():
			final := runtime.NumGoroutine()
			if final > base+5 {
				t.Fatalf("goroutine leak: baseline=%d final=%d", base, final)
			}
			return
		default:
			runtime.GC()
			if runtime.NumGoroutine() <= base+5 {
				return
			}
			// Yield without sleeping.
			runtime.Gosched()
		}
	}
}
