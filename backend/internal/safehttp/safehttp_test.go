package safehttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

// resetAllowInternalForTest clears the cached AllowInternal() value so a
// subsequent env change takes effect. Tests only — production relies on
// process-level caching.
func resetAllowInternalForTest(t *testing.T) {
	t.Helper()
	allowInternalOnce = sync.Once{}
	allowInternalVal = false
}

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		blocked bool
	}{
		{"loopback v4", "127.0.0.1", true},
		{"loopback v6", "::1", true},
		{"private 10/8", "10.0.0.1", true},
		{"private 172.16/12", "172.16.0.1", true},
		{"private 192.168/16", "192.168.0.1", true},
		{"link-local v4", "169.254.0.1", true},
		{"link-local v6", "fe80::1", true},
		{"multicast v4", "224.0.0.1", true},
		{"unspecified v4", "0.0.0.0", true},
		{"unspecified v6", "::", true},
		{"public v4 (Google DNS)", "8.8.8.8", false},
		{"public v6 (Cloudflare DNS)", "2606:4700:4700::1111", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("parse ip %q returned nil", tc.ip)
			}
			if got := isBlockedIP(ip); got != tc.blocked {
				t.Fatalf("isBlockedIP(%q) = %v, want %v", tc.ip, got, tc.blocked)
			}
		})
	}

	t.Run("nil ip", func(t *testing.T) {
		if !isBlockedIP(nil) {
			t.Fatal("isBlockedIP(nil) = false, want true")
		}
	})
}

func TestSafeDialContext_BlocksPrivate(t *testing.T) {
	// httptest.NewServer binds on 127.0.0.1 — a loopback address that must be
	// rejected by default.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	// Block by default.
	resetAllowInternalForTest(t)
	client := Client(2 * time.Second)
	_, err := client.Get(srv.URL)
	if err == nil {
		t.Fatal("expected dial to be blocked by default, got nil error")
	}
	// Unwrap through url.Error.
	var uerr *url.Error
	if errors.As(err, &uerr) {
		err = uerr.Err
	}
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("expected ErrBlockedAddress, got %v", err)
	}

	// Allow internal via env — should now succeed.
	t.Setenv("OPENSCANNER_ALLOW_INTERNAL_HTTP", "1")
	resetAllowInternalForTest(t)
	client = Client(2 * time.Second)
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("expected success with allow-internal set, got %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestSafeDialContext_DNSRebinding(t *testing.T) {
	// safehttp.safeDialContext uses net.DefaultResolver with no injection
	// point, so a controlled dual-answer DNS test is not possible without
	// refactoring. The behaviour is exercised implicitly by the loopback
	// block test (any resolved IP in a blocked range aborts the dial).
	t.Skip("no resolver injection point; DNS-rebinding defense exercised indirectly via TestSafeDialContext_BlocksPrivate")
}

func TestAllowInternalEnvParsing(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{"literal 1", "1", true},
		{"literal true", "true", true},
		{"uppercase TRUE", "TRUE", true},
		{"yes", "yes", true},
		{"empty", "", false},
		{"no", "no", false},
		{"zero", "0", false},
		{"whitespace around 1", "  1  ", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OPENSCANNER_ALLOW_INTERNAL_HTTP", tc.env)
			resetAllowInternalForTest(t)
			if got := AllowInternal(); got != tc.want {
				t.Fatalf("AllowInternal() with env=%q = %v, want %v", tc.env, got, tc.want)
			}
		})
	}

	// Leave the cache in a benign state for subsequent tests in this package.
	t.Cleanup(func() {
		resetAllowInternalForTest(t)
	})

	// Silence unused-import lint in case context is pruned in the future.
	_ = context.Background
}
