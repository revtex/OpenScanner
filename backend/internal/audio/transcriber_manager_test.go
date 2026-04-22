package audio_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/audio"
)

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

// TestTranscriberManager_Reload_NoGoroutineLeak verifies that repeated
// enable/disable cycles do not leak pumpResults goroutines. The pool closes
// its results channel once all workers exit and reaps idle HTTP keep-alive
// connections, so pumpResults and the Transport's persistConn goroutines
// unwind cleanly when Reload cancels the pool context.
func TestTranscriberManager_Reload_NoGoroutineLeak(t *testing.T) {
	srv := newFakeWhisper(t, http.StatusOK)

	baseCtx, baseCancel := context.WithCancel(context.Background())
	t.Cleanup(baseCancel)
	m := audio.NewTranscriberManager(baseCtx, nil, nil)

	// Warm up: one cycle so any first-run initialisation (DNS, TLS setup,
	// httptest accept loops) is accounted for in the baseline.
	if ok := m.Reload(true, srv.URL, "ggml-base", "", false); !ok {
		t.Fatal("warm-up Reload(true) returned false")
	}
	if ok := m.Reload(false, "", "", "", false); !ok {
		t.Fatal("warm-up Reload(false) returned false")
	}
	waitForGoroutinesToSettle(2 * time.Second)
	base := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		if ok := m.Reload(true, srv.URL, "ggml-base", "", false); !ok {
			t.Fatalf("Reload(true) #%d returned false", i)
		}
		if ok := m.Reload(false, "", "", "", false); !ok {
			t.Fatalf("Reload(false) #%d returned false", i)
		}
	}

	waitForGoroutinesToSettle(2 * time.Second)
	if final := runtime.NumGoroutine(); final > base+3 {
		t.Fatalf("goroutine leak: baseline=%d final=%d (tolerance=3)", base, final)
	}
}

// waitForGoroutinesToSettle yields to the scheduler and forces GC in a
// bounded loop to give cancelled goroutines time to exit, without sleeping.
func waitForGoroutinesToSettle(timeout time.Duration) {
	deadline, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	prev := -1
	stable := 0
	for {
		runtime.GC()
		n := runtime.NumGoroutine()
		if n == prev {
			stable++
			if stable >= 3 {
				return
			}
		} else {
			stable = 0
			prev = n
		}
		select {
		case <-deadline.Done():
			return
		default:
			runtime.Gosched()
		}
	}
}
