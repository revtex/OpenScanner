package routes_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/handler/routes"
)

// audioFixtureBytes is a fixed payload used across the dual-auth audio tests.
// It is small and not a valid WAV — the handler does not parse the body.
var audioFixtureBytes = []byte("FAKE_AUDIO_BYTES")

// newTestEngineWithAudio is a variant of newTestEngineWithCalls that exposes
// the recordings directory so tests can write a real on-disk audio file at the
// path stored on the calls row. The dual-auth tests need an end-to-end 200
// response from GET /api/calls/:id/audio, which means the file must exist.
func newTestEngineWithAudio(t *testing.T) (*gin.Engine, *db.Queries, string) {
	t.Helper()
	_, queries := newTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	recordingsDir := t.TempDir()
	pool := audio.NewWorkerPool(ctx)
	proc := audio.NewProcessor(recordingsDir, pool)

	router := gin.New()
	rl := auth.NewRateLimiter(context.Background())
	routes.RegisterRoutes(router, routes.Deps{
		Queries:     queries,
		RateLimiter: rl,
		Processor:   proc,
		Version:     "test",
	})
	return router, queries, recordingsDir
}

// seedAudioCall writes audioFixtureBytes to a file under recordingsDir and
// inserts a matching system + talkgroup + call row. Returns the call ID.
func seedAudioCall(t *testing.T, q *db.Queries, recordingsDir string) int64 {
	t.Helper()
	ctx := context.Background()

	// Write the fake audio file at recordingsDir/test/audio.wav so the handler
	// can resolve it relative to the os.Root.
	relPath := filepath.Join("test", "audio.wav")
	absPath := filepath.Join(recordingsDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("mkdir audio dir: %v", err)
	}
	if err := os.WriteFile(absPath, audioFixtureBytes, 0o644); err != nil {
		t.Fatalf("write audio file: %v", err)
	}

	sysID, err := q.CreateSystem(ctx, db.CreateSystemParams{
		SystemID: 100,
		Label:    "Test System",
	})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}

	tgID, err := q.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
		TalkgroupID: 200,
		SystemID:    sysID,
		Label:       sql.NullString{String: "Fire Dispatch", Valid: true},
		Name:        sql.NullString{String: "FD", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup: %v", err)
	}

	callID, err := q.CreateCall(ctx, db.CreateCallParams{
		AudioPath:   relPath,
		AudioName:   "audio.wav",
		AudioType:   "audio/wav",
		DateTime:    time.Now().Unix(),
		SystemID:    sysID,
		TalkgroupID: sql.NullInt64{Int64: tgID, Valid: true},
		Frequency:   sql.NullInt64{Int64: 851000000, Valid: true},
		Duration:    sql.NullInt64{Int64: 15, Valid: true},
		Source:      sql.NullInt64{Int64: 12345, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateCall: %v", err)
	}
	return callID
}

// setPublicAccess writes the publicAccess setting via the queries helper used
// elsewhere in the routes tests.
func setPublicAccess(t *testing.T, q *db.Queries, enabled bool) {
	t.Helper()
	val := "false"
	if enabled {
		val = "true"
	}
	if err := q.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "publicAccess", Value: val,
	}); err != nil {
		t.Fatalf("UpsertSetting publicAccess=%s: %v", val, err)
	}
}

// loginAndGetToken seeds an admin user, performs a login, and returns the
// access JWT (from the JSON body) and the os_session cookie issued by the
// server.
func loginAndGetToken(t *testing.T, engine http.Handler, q *db.Queries, username, password string) (string, *http.Cookie) {
	t.Helper()
	seedAdminUser(t, q, username, password)
	w := login(t, engine, username, password)
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if body.Token == "" {
		t.Fatalf("login returned empty token; body cookies: %v", w.Header().Values("Set-Cookie"))
	}
	ck := findCookie(w.Result().Cookies(), auth.SessionCookieName)
	if ck == nil {
		t.Fatalf("login did not set %s cookie", auth.SessionCookieName)
	}
	return body.Token, ck
}

// TestGetCallAudio_BearerStillWorks verifies the existing Bearer-header path
// is unaffected by the new dual-auth middleware.
func TestGetCallAudio_BearerStillWorks(t *testing.T) {
	engine, queries, recordingsDir := newTestEngineWithAudio(t)
	callID := seedAudioCall(t, queries, recordingsDir)
	token, _ := loginAndGetToken(t, engine, queries, "alice", "password123")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.Bytes(); string(got) != string(audioFixtureBytes) {
		t.Errorf("body = %q, want %q", got, audioFixtureBytes)
	}
}

// TestGetCallAudio_CookieWithSameOriginFetchSite verifies the cookie path is
// honoured when Sec-Fetch-Site indicates a same-origin request.
func TestGetCallAudio_CookieWithSameOriginFetchSite(t *testing.T) {
	engine, queries, recordingsDir := newTestEngineWithAudio(t)
	callID := seedAudioCall(t, queries, recordingsDir)
	_, sessionCk := loginAndGetToken(t, engine, queries, "alice", "password123")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
	req.AddCookie(sessionCk)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.Bytes(); string(got) != string(audioFixtureBytes) {
		t.Errorf("body = %q, want %q", got, audioFixtureBytes)
	}
}

// TestGetCallAudio_CookieWithMissingFetchSite verifies the test-friendly
// fallback when Sec-Fetch-Site is absent (older clients, server-to-server
// callers, tests without explicit fetch metadata).
func TestGetCallAudio_CookieWithMissingFetchSite(t *testing.T) {
	engine, queries, recordingsDir := newTestEngineWithAudio(t)
	callID := seedAudioCall(t, queries, recordingsDir)
	_, sessionCk := loginAndGetToken(t, engine, queries, "alice", "password123")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
	req.AddCookie(sessionCk)
	// Deliberately no Sec-Fetch-Site header.
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.Bytes(); string(got) != string(audioFixtureBytes) {
		t.Errorf("body = %q, want %q", got, audioFixtureBytes)
	}
}

// TestGetCallAudio_CookieWithCrossSiteFetchSite_TreatedAnonymous verifies that
// a cross-site request causes the cookie to be ignored, falling through to
// the anonymous path which is gated on publicAccess.
func TestGetCallAudio_CookieWithCrossSiteFetchSite_TreatedAnonymous(t *testing.T) {
	t.Run("publicAccess=false yields 401", func(t *testing.T) {
		engine, queries, recordingsDir := newTestEngineWithAudio(t)
		callID := seedAudioCall(t, queries, recordingsDir)
		_, sessionCk := loginAndGetToken(t, engine, queries, "alice", "password123")
		setPublicAccess(t, queries, false)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
		req.AddCookie(sessionCk)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("publicAccess=true yields 200", func(t *testing.T) {
		engine, queries, recordingsDir := newTestEngineWithAudio(t)
		callID := seedAudioCall(t, queries, recordingsDir)
		_, sessionCk := loginAndGetToken(t, engine, queries, "alice", "password123")
		setPublicAccess(t, queries, true)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
		req.AddCookie(sessionCk)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		if got := w.Body.Bytes(); string(got) != string(audioFixtureBytes) {
			t.Errorf("body = %q, want %q", got, audioFixtureBytes)
		}
	})
}

// TestGetCallAudio_StaleCookieFallsThroughToAnonymous verifies that an invalid
// or unparsable session cookie does not abort the request — the middleware
// silently falls through to the anonymous path.
func TestGetCallAudio_StaleCookieFallsThroughToAnonymous(t *testing.T) {
	staleCookie := &http.Cookie{
		Name:  auth.SessionCookieName,
		Value: "invalid.jwt.value",
	}

	t.Run("publicAccess=false yields 401", func(t *testing.T) {
		engine, queries, recordingsDir := newTestEngineWithAudio(t)
		callID := seedAudioCall(t, queries, recordingsDir)
		setPublicAccess(t, queries, false)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
		req.AddCookie(staleCookie)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("publicAccess=true yields 200", func(t *testing.T) {
		engine, queries, recordingsDir := newTestEngineWithAudio(t)
		callID := seedAudioCall(t, queries, recordingsDir)
		setPublicAccess(t, queries, true)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
		req.AddCookie(staleCookie)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		if got := w.Body.Bytes(); string(got) != string(audioFixtureBytes) {
			t.Errorf("body = %q, want %q", got, audioFixtureBytes)
		}
	})
}

// TestGetCallAudio_AnonymousWithPublicAccess verifies that a request with no
// auth at all succeeds when publicAccess is enabled.
func TestGetCallAudio_AnonymousWithPublicAccess(t *testing.T) {
	engine, queries, recordingsDir := newTestEngineWithAudio(t)
	callID := seedAudioCall(t, queries, recordingsDir)
	setPublicAccess(t, queries, true)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.Bytes(); string(got) != string(audioFixtureBytes) {
		t.Errorf("body = %q, want %q", got, audioFixtureBytes)
	}
}

// TestGetCallAudio_AnonymousWithoutPublicAccess verifies the default-deny
// path: no auth and no publicAccess setting yields 401.
func TestGetCallAudio_AnonymousWithoutPublicAccess(t *testing.T) {
	engine, queries, recordingsDir := newTestEngineWithAudio(t)
	callID := seedAudioCall(t, queries, recordingsDir)
	// Do not set publicAccess; default-unset should be treated as disabled.

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body: %s", w.Code, w.Body.String())
	}
}
