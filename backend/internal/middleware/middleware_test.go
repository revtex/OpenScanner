package middleware_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/logging"
	"github.com/openscanner/openscanner/internal/middleware"
	_ "modernc.org/sqlite"
)

func init() {
	gin.SetMode(gin.TestMode)
	logging.Configure(true, "")
}

// newMiddlewareDB opens an in-memory SQLite database with all embedded
// migrations applied.
func newMiddlewareDB(t *testing.T) *db.Queries {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db.New(sqlDB)
}

// seedAPIKey inserts an api_keys row with the given raw key (hashed before
// insert) and disabled flag. Returns the row id.
func seedAPIKey(t *testing.T, queries *db.Queries, rawKey, ident string, disabled int64) int64 {
	t.Helper()
	id, err := queries.CreateAPIKey(context.Background(), db.CreateAPIKeyParams{
		Key:      auth.HashAPIKey(rawKey),
		Ident:    sql.NullString{String: ident, Valid: true},
		Disabled: disabled,
	})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	return id
}

// --------------------------------------------------------------------------
// CORS
// --------------------------------------------------------------------------

func TestCORS(t *testing.T) {
	origMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(origMode) })

	tests := []struct {
		name         string
		mode         string
		origin       string
		host         string
		method       string
		wantStatus   int
		wantAllowHdr string // empty if not expected
	}{
		{
			name:         "same origin is echoed",
			mode:         gin.ReleaseMode,
			origin:       "http://example.com",
			host:         "example.com",
			method:       http.MethodGet,
			wantStatus:   http.StatusOK,
			wantAllowHdr: "http://example.com",
		},
		{
			name:       "cross origin is rejected in release",
			mode:       gin.ReleaseMode,
			origin:     "http://attacker.example",
			host:       "example.com",
			method:     http.MethodGet,
			wantStatus: http.StatusForbidden,
		},
		{
			name:         "preflight options returns 204",
			mode:         gin.ReleaseMode,
			origin:       "http://example.com",
			host:         "example.com",
			method:       http.MethodOptions,
			wantStatus:   http.StatusNoContent,
			wantAllowHdr: "http://example.com",
		},
		{
			name:         "localhost allowed in debug mode",
			mode:         gin.DebugMode,
			origin:       "http://localhost:5173",
			host:         "localhost:8080",
			method:       http.MethodGet,
			wantStatus:   http.StatusOK,
			wantAllowHdr: "http://localhost:5173",
		},
		{
			name:       "localhost rejected in release mode",
			mode:       gin.ReleaseMode,
			origin:     "http://localhost:5173",
			host:       "localhost:8080",
			method:     http.MethodGet,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing origin passes through",
			mode:       gin.ReleaseMode,
			origin:     "",
			host:       "example.com",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(tc.mode)
			r := gin.New()
			r.Use(middleware.CORS())
			r.Any("/thing", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(tc.method, "/thing", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if tc.wantAllowHdr != "" {
				if got := w.Header().Get("Access-Control-Allow-Origin"); got != tc.wantAllowHdr {
					t.Errorf("Allow-Origin = %q, want %q", got, tc.wantAllowHdr)
				}
			}
		})
	}
}

// --------------------------------------------------------------------------
// APIKeyAuth
// --------------------------------------------------------------------------

func buildMultipartBody(t *testing.T, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	return body, mw.FormDataContentType()
}

func newAPIKeyRouter(queries *db.Queries) (*gin.Engine, *int64) {
	var seenID int64
	r := gin.New()
	r.Use(middleware.APIKeyAuth(queries))
	r.POST("/upload", func(c *gin.Context) {
		if v, ok := c.Get("apiKeyID"); ok {
			seenID = v.(int64)
		}
		c.Status(http.StatusOK)
	})
	return r, &seenID
}

func TestAPIKeyAuth_Precedence(t *testing.T) {
	queries := newMiddlewareDB(t)
	headerKey := "header-key-value"
	queryKey := "query-key-value"
	formKey := "form-key-value"
	headerID := seedAPIKey(t, queries, headerKey, "header", 0)
	queryID := seedAPIKey(t, queries, queryKey, "query", 0)
	formID := seedAPIKey(t, queries, formKey, "form", 0)

	t.Run("none rejected with 401", func(t *testing.T) {
		r, _ := newAPIKeyRouter(queries)
		req := httptest.NewRequest(http.MethodPost, "/upload", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("header only", func(t *testing.T) {
		r, got := newAPIKeyRouter(queries)
		req := httptest.NewRequest(http.MethodPost, "/upload", nil)
		req.Header.Set("X-API-Key", headerKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if *got != headerID {
			t.Errorf("apiKeyID = %d, want %d", *got, headerID)
		}
	})

	t.Run("query only", func(t *testing.T) {
		r, got := newAPIKeyRouter(queries)
		req := httptest.NewRequest(http.MethodPost, "/upload?key="+queryKey, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if *got != queryID {
			t.Errorf("apiKeyID = %d, want %d", *got, queryID)
		}
	})

	t.Run("multipart form only", func(t *testing.T) {
		r, got := newAPIKeyRouter(queries)
		body, ct := buildMultipartBody(t, map[string]string{"key": formKey})
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if *got != formID {
			t.Errorf("apiKeyID = %d, want %d (form)", *got, formID)
		}
	})

	t.Run("header wins over query and form", func(t *testing.T) {
		r, got := newAPIKeyRouter(queries)
		body, ct := buildMultipartBody(t, map[string]string{"key": formKey})
		req := httptest.NewRequest(http.MethodPost, "/upload?key="+queryKey, body)
		req.Header.Set("Content-Type", ct)
		req.Header.Set("X-API-Key", headerKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if *got != headerID {
			t.Errorf("apiKeyID = %d, want %d (header precedence)", *got, headerID)
		}
	})
}

func TestAPIKeyAuth_LengthCap(t *testing.T) {
	queries := newMiddlewareDB(t)
	r, _ := newAPIKeyRouter(queries)

	// 129-char key — must be rejected before any DB lookup.
	big := strings.Repeat("a", 129)
	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	req.Header.Set("X-API-Key", big)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestAPIKeyAuth_DisabledKey(t *testing.T) {
	queries := newMiddlewareDB(t)
	rawKey := "disabled-key"
	seedAPIKey(t, queries, rawKey, "disabled", 1)

	r, _ := newAPIKeyRouter(queries)
	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	req.Header.Set("X-API-Key", rawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// --------------------------------------------------------------------------
// RequireAdmin
// --------------------------------------------------------------------------

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name     string
		role     any // nil to omit
		wantCode int
	}{
		{"admin passes", "admin", http.StatusOK},
		{"listener forbidden", "listener", http.StatusForbidden},
		{"no role forbidden", nil, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			// Inject role before RequireAdmin runs.
			r.Use(func(c *gin.Context) {
				if tc.role != nil {
					c.Set("role", tc.role)
				}
				c.Next()
			})
			r.Use(middleware.RequireAdmin())
			r.GET("/admin", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

// --------------------------------------------------------------------------
// RateLimitByIP
// --------------------------------------------------------------------------

func TestRateLimitByIP(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RateLimitByIP(3))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	// 3 requests allowed, 4th blocked.
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i, w.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("trigger status = %d, want 429", w.Code)
	}

	// Different IP remains unaffected.
	req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req2.RemoteAddr = "5.6.7.8:4567"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("other IP status = %d, want 200", w2.Code)
	}
}

// --------------------------------------------------------------------------
// MaxBodySize
// --------------------------------------------------------------------------

func TestMaxBodySize(t *testing.T) {
	const limit = 16

	r := gin.New()
	r.Use(middleware.MaxBodySize(limit))
	r.POST("/echo", func(c *gin.Context) {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", b)
	})

	t.Run("under cap", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("short"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("under-cap status = %d, want 200", w.Code)
		}
	})

	t.Run("over cap rejected", func(t *testing.T) {
		big := strings.Repeat("x", limit+1)
		req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(big))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			t.Fatalf("over-cap status = 200, want non-200 (413-ish)")
		}
	})
}

// --------------------------------------------------------------------------
// SwaggerCookieAuth
// --------------------------------------------------------------------------

// swaggerCookieIssuer is a minimal adapter satisfying auth.SetSwaggerCookie's
// interface signature without pulling in the gin context surface.
type swaggerCookieIssuer struct {
	header http.Header
}

func (s *swaggerCookieIssuer) SetSameSite(_ http.SameSite) {}
func (s *swaggerCookieIssuer) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: http.SameSiteStrictMode,
	}
	s.header.Add("Set-Cookie", c.String())
	_ = time.Now // silence
}

func TestSwaggerCookieAuth(t *testing.T) {
	// Issue a valid cookie once, capture its value for reuse.
	issuer := &swaggerCookieIssuer{header: http.Header{}}
	auth.SetSwaggerCookie(issuer, false)
	var validValue string
	for _, ck := range (&http.Response{Header: issuer.header}).Cookies() {
		if ck.Name == auth.SwaggerCookieName {
			validValue = ck.Value
		}
	}
	if validValue == "" {
		t.Fatal("failed to issue swagger cookie")
	}

	r := gin.New()
	r.Use(middleware.SwaggerCookieAuth())
	r.GET("/docs", func(c *gin.Context) { c.Status(http.StatusOK) })

	tests := []struct {
		name       string
		cookie     *http.Cookie
		wantStatus int
	}{
		{"valid", &http.Cookie{Name: auth.SwaggerCookieName, Value: validValue}, http.StatusOK},
		{"invalid value", &http.Cookie{Name: auth.SwaggerCookieName, Value: "deadbeef.notahex"}, http.StatusUnauthorized},
		{"missing", nil, http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/docs", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}

	// Silence unused import.
	_ = fmt.Sprintf
}
