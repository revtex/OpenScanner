package routes_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSetupStatus_NeedsSetup(t *testing.T) {
	engine, _ := newTestEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body struct {
		NeedsSetup bool `json:"needsSetup"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.NeedsSetup {
		t.Error("needsSetup should be true for a fresh database")
	}
}

func TestPostSetup_Success(t *testing.T) {
	engine, queries := newTestEngine(t)

	payload, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "strongpass1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/setup status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// Verify setup_complete is now 1.
	state, err := queries.GetAppState(t.Context())
	if err != nil {
		t.Fatalf("GetAppState: %v", err)
	}
	if state.SetupComplete == 0 {
		t.Error("setup_complete should be 1 after successful setup")
	}

	// Verify GET /api/setup/status now returns needsSetup=false.
	req2 := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, req2)

	var statusBody struct {
		NeedsSetup bool `json:"needsSetup"`
	}
	if err := json.NewDecoder(w2.Body).Decode(&statusBody); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if statusBody.NeedsSetup {
		t.Error("needsSetup should be false after setup completes")
	}
}

func TestPostSetup_AlreadyComplete(t *testing.T) {
	engine, queries := newTestEngine(t)

	// Seed setup as already complete.
	seedAdminUser(t, queries, "admin", "strongpass1")

	payload, _ := json.Marshal(map[string]string{
		"username": "admin2",
		"password": "anotherpass",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestPostSetup_EmptyUsername(t *testing.T) {
	engine, _ := newTestEngine(t)

	payload, _ := json.Marshal(map[string]string{
		"username": "",
		"password": "strongpass1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestPostSetup_ShortPassword(t *testing.T) {
	engine, _ := newTestEngine(t)

	payload, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "short",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
