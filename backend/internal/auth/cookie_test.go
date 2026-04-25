package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newCookieContext returns a gin.Context backed by an httptest recorder and
// a request configured with the given scheme. scheme is "http" or "https".
func newCookieContext(t *testing.T, scheme string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if scheme == "https" {
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	c.Request = req
	return c, w
}

// findSetCookie returns the first Set-Cookie header value whose name matches.
func findSetCookie(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	resp := http.Response{Header: w.Header()}
	for _, ck := range resp.Cookies() {
		if ck.Name == name {
			return ck
		}
	}
	t.Fatalf("cookie %q not found in Set-Cookie headers: %v", name, w.Header().Values("Set-Cookie"))
	return nil
}

func TestSetRefreshCookie(t *testing.T) {
	tests := []struct {
		name       string
		scheme     string
		maxAge     int
		token      string
		wantSecure bool
	}{
		{"http (dev)", "http", 3600, "raw-token-abc", false},
		{"https (prod)", "https", 3600, "raw-token-xyz", true},
		{"session cookie (maxAge=0)", "https", 0, "session-token", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, w := newCookieContext(t, tc.scheme)
			auth.SetRefreshCookie(c, tc.token, tc.maxAge)

			ck := findSetCookie(t, w, auth.RefreshCookieName)

			if ck.Value != tc.token {
				t.Errorf("value = %q, want %q", ck.Value, tc.token)
			}
			if !ck.HttpOnly {
				t.Error("HttpOnly = false, want true")
			}
			if ck.Secure != tc.wantSecure {
				t.Errorf("Secure = %v, want %v", ck.Secure, tc.wantSecure)
			}
			if ck.SameSite != http.SameSiteLaxMode {
				t.Errorf("SameSite = %v, want Lax (%v)", ck.SameSite, http.SameSiteLaxMode)
			}
			if ck.Path != auth.RefreshCookiePath {
				t.Errorf("Path = %q, want %q", ck.Path, auth.RefreshCookiePath)
			}
			if tc.maxAge > 0 {
				if ck.MaxAge != tc.maxAge {
					t.Errorf("MaxAge = %d, want %d", ck.MaxAge, tc.maxAge)
				}
			}
		})
	}
}

func TestClearRefreshCookie(t *testing.T) {
	c, w := newCookieContext(t, "https")
	auth.ClearRefreshCookie(c)

	ck := findSetCookie(t, w, auth.RefreshCookieName)

	if ck.Value != "" {
		t.Errorf("value = %q, want empty", ck.Value)
	}
	if ck.MaxAge > 0 {
		t.Errorf("MaxAge = %d, want <= 0", ck.MaxAge)
	}
	if ck.Path != auth.RefreshCookiePath {
		t.Errorf("Path = %q, want %q", ck.Path, auth.RefreshCookiePath)
	}
	if !ck.HttpOnly {
		t.Error("HttpOnly = false, want true")
	}
	if ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", ck.SameSite)
	}
}

func TestSetSessionCookie(t *testing.T) {
	tests := []struct {
		name       string
		scheme     string
		maxAge     int
		token      string
		wantSecure bool
	}{
		{"http (dev)", "http", 900, "jwt-dev", false},
		{"https (prod)", "https", 900, "jwt-prod", true},
		{"session (maxAge=0)", "https", 0, "jwt-session", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, w := newCookieContext(t, tc.scheme)
			auth.SetSessionCookie(c, tc.token, tc.maxAge)

			ck := findSetCookie(t, w, auth.SessionCookieName)

			if ck.Value != tc.token {
				t.Errorf("value = %q, want %q", ck.Value, tc.token)
			}
			if !ck.HttpOnly {
				t.Error("HttpOnly = false, want true")
			}
			if ck.Secure != tc.wantSecure {
				t.Errorf("Secure = %v, want %v", ck.Secure, tc.wantSecure)
			}
			if ck.SameSite != http.SameSiteStrictMode {
				t.Errorf("SameSite = %v, want Strict (%v)", ck.SameSite, http.SameSiteStrictMode)
			}
			if ck.Path != auth.SessionCookiePath {
				t.Errorf("Path = %q, want %q", ck.Path, auth.SessionCookiePath)
			}
			if tc.maxAge > 0 && ck.MaxAge != tc.maxAge {
				t.Errorf("MaxAge = %d, want %d", ck.MaxAge, tc.maxAge)
			}
		})
	}
}

func TestClearSessionCookie(t *testing.T) {
	c, w := newCookieContext(t, "https")
	auth.ClearSessionCookie(c)

	ck := findSetCookie(t, w, auth.SessionCookieName)

	if ck.Value != "" {
		t.Errorf("value = %q, want empty", ck.Value)
	}
	if ck.MaxAge > 0 {
		t.Errorf("MaxAge = %d, want <= 0", ck.MaxAge)
	}
	if ck.Path != auth.SessionCookiePath {
		t.Errorf("Path = %q, want %q", ck.Path, auth.SessionCookiePath)
	}
	if !ck.HttpOnly {
		t.Error("HttpOnly = false, want true")
	}
	if ck.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", ck.SameSite)
	}
}
