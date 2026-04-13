package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/api"
	"github.com/openscanner/openscanner/internal/db"
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

func TestGetCallShare_DisabledReturns404(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	callID := seedCallWithSystem(t, queries)

	// shareableLinks is NOT set — should return 404.
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/share", callID), nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestGetCallShare_NotFound(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)

	// Enable shareable links.
	if err := queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "shareableLinks", Value: "true",
	}); err != nil {
		t.Fatalf("UpsertSetting: %v", err)
	}

	// Request a non-existent call ID.
	req := httptest.NewRequest(http.MethodGet, "/api/calls/999999/share", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestGetCallShare_Success(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	callID := seedCallWithSystem(t, queries)

	// Enable shareable links.
	if err := queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "shareableLinks", Value: "true",
	}); err != nil {
		t.Fatalf("UpsertSetting: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/share", callID), nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ShareResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ID != callID {
		t.Errorf("ID = %d, want %d", resp.ID, callID)
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
	expectedAudioURL := fmt.Sprintf("/api/calls/%d/audio", callID)
	if resp.AudioURL != expectedAudioURL {
		t.Errorf("AudioURL = %q, want %q", resp.AudioURL, expectedAudioURL)
	}
	if resp.Frequency != 851000000 {
		t.Errorf("Frequency = %d, want %d", resp.Frequency, 851000000)
	}
	if resp.Duration != 15 {
		t.Errorf("Duration = %d, want %d", resp.Duration, 15)
	}
	if resp.Source != 12345 {
		t.Errorf("Source = %d, want %d", resp.Source, 12345)
	}
}

func TestGetCallShare_InvalidID(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)

	// Enable shareable links.
	if err := queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "shareableLinks", Value: "true",
	}); err != nil {
		t.Fatalf("UpsertSetting: %v", err)
	}

	tests := []struct {
		name string
		id   string
	}{
		{"non-numeric", "abc"},
		{"negative", "-1"},
		{"zero", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/calls/"+tt.id+"/share", nil)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
			}
		})
	}
}
