package routes_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/handler/share"
)

// seedCallWithSystem creates a system, talkgroup, and call in the DB and
// returns the call row ID.
func seedCallWithSystem(t *testing.T, q *db.Queries) int64 {
	t.Helper()
	ctx := context.Background()

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
		AudioPath:   "test/audio.wav",
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

// seedSharedLink creates a shared_links record for the given call+user and returns the token.
func seedSharedLink(t *testing.T, q *db.Queries, callID, userID int64) string {
	t.Helper()
	token := uuid.New().String()
	_, err := q.CreateSharedLink(context.Background(), db.CreateSharedLinkParams{
		CallID: callID,
		UserID: userID,
		Token:  token,
	})
	if err != nil {
		t.Fatalf("CreateSharedLink: %v", err)
	}
	return token
}

func TestGetSharedCallByToken_Success(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	callID := seedCallWithSystem(t, queries)
	userID := seedAdminUser(t, queries, "sharer", "password123")
	token := seedSharedLink(t, queries, callID, userID)

	req := httptest.NewRequest(http.MethodGet, "/api/shared/"+token, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp share.ShareResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Token != token {
		t.Errorf("Token = %q, want %q", resp.Token, token)
	}
	if resp.SystemLabel != "Test System" {
		t.Errorf("SystemLabel = %q, want %q", resp.SystemLabel, "Test System")
	}
	if resp.TalkgroupLabel != "Fire Dispatch" {
		t.Errorf("TalkgroupLabel = %q, want %q", resp.TalkgroupLabel, "Fire Dispatch")
	}
	if resp.TalkgroupName != "FD" {
		t.Errorf("TalkgroupName = %q, want %q", resp.TalkgroupName, "FD")
	}
	expectedAudioURL := fmt.Sprintf("/api/shared/%s/audio", token)
	if resp.AudioURL != expectedAudioURL {
		t.Errorf("AudioURL = %q, want %q", resp.AudioURL, expectedAudioURL)
	}
}

func TestGetSharedCallByToken_NotFound(t *testing.T) {
	engine, _ := newTestEngineWithCalls(t)

	req := httptest.NewRequest(http.MethodGet, "/api/shared/nonexistent-token", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestGetCallAudio_RequiresAuthWhenNotPublic(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	callID := seedCallWithSystem(t, queries)

	// No JWT, no publicAccess — should get 401.
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestGetCallAudio_AllowedWithPublicAccess(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	callID := seedCallWithSystem(t, queries)

	// Enable publicAccess.
	if err := queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "publicAccess", Value: "true",
	}); err != nil {
		t.Fatalf("UpsertSetting: %v", err)
	}

	// No JWT, but publicAccess=true — should not get 401.
	// (will get 404 because audio file doesn't exist, but not 401)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401 with publicAccess, got 401 (body: %s)", w.Body.String())
	}
}
