package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/middleware"
)

func TestPostLogin_Success(t *testing.T) {
	engine, queries := newTestEngine(t)
	seedAdminUser(t, queries, "alice", "password123")

	payload, _ := json.Marshal(map[string]string{
		"username": "alice",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var body struct {
		Token string `json:"token"`
		User  struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"user"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Token == "" {
		t.Error("token should not be empty on successful login")
	}
	if body.User.Username != "alice" {
		t.Errorf("username = %q, want %q", body.User.Username, "alice")
	}
	if body.User.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want %q", body.User.Role, auth.RoleAdmin)
	}
}

func TestPostLogin_WrongPassword(t *testing.T) {
	engine, queries := newTestEngine(t)
	seedAdminUser(t, queries, "alice", "correctpassword")

	payload, _ := json.Marshal(map[string]string{
		"username": "alice",
		"password": "wrongpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestPostLogin_UserNotFound(t *testing.T) {
	engine, _ := newTestEngine(t)

	payload, _ := json.Marshal(map[string]string{
		"username": "nobody",
		"password": "doesnotmatter",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestPostLogin_RateLimited(t *testing.T) {
	engine, queries := newTestEngine(t)
	seedAdminUser(t, queries, "alice", "correctpassword")

	wrongPayload, _ := json.Marshal(map[string]string{
		"username": "alice",
		"password": "wrongpassword",
	})

	// Three failed attempts to trigger lockout.
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(wrongPayload))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.1.1.1:9999"
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i, w.Code)
		}
	}

	// Fourth attempt: should be rate limited.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(wrongPayload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.1.1.1:9999"
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("4th attempt status = %d, want 429", w.Code)
	}
}

func TestGetMe_Valid(t *testing.T) {
	engine, queries := newTestEngine(t)
	seedAdminUser(t, queries, "alice", "password123")

	token, err := auth.GenerateToken(1, "alice", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var body struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Username != "alice" {
		t.Errorf("username = %q, want %q", body.Username, "alice")
	}
	if body.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want %q", body.Role, auth.RoleAdmin)
	}
}

func TestGetMe_NoToken(t *testing.T) {
	engine, _ := newTestEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestRequireAdmin_ListenerGets403(t *testing.T) {
	// Wire up a minimal router with RequireAdmin on a test route.
	router := gin.New()
	router.GET("/admin-only",
		middleware.JWTAuth(),
		middleware.RequireAdmin(),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	token, err := auth.GenerateToken(2, "listener", auth.RoleListener)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestRequestIDPropagation(t *testing.T) {
	engine, _ := newTestEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("X-Request-ID header should be present in the response")
	}
	// Verify it looks like a UUID (36 chars including dashes).
	if len(requestID) != 36 {
		t.Errorf("X-Request-ID = %q, expected a 36-character UUID", requestID)
	}
}

// TestRequireAdmin_AdminGets200 verifies an admin JWT is accepted by the
// RequireAdmin middleware (positive counterpart of the 403 test).
func TestRequireAdmin_AdminGets200(t *testing.T) {
	router := gin.New()
	router.GET("/admin-only",
		middleware.JWTAuth(),
		middleware.RequireAdmin(),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	token, err := auth.GenerateToken(1, "admin", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
