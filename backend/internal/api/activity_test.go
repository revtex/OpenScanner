package api_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/api"
	"github.com/openscanner/openscanner/internal/db"
)

// seedCallAt creates a system, talkgroup, and call at the given unix timestamp,
// returning the call row ID.
func seedCallAt(t *testing.T, q *db.Queries, sysID, tgID int64, ts int64) int64 {
	t.Helper()
	callID, err := q.CreateCall(context.Background(), db.CreateCallParams{
		AudioPath:   "test/audio.wav",
		AudioName:   "audio.wav",
		AudioType:   "audio/wav",
		DateTime:    ts,
		SystemID:    sysID,
		TalkgroupID: sql.NullInt64{Int64: tgID, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateCall: %v", err)
	}
	return callID
}

func TestGetActivityStats_Unauthorized(t *testing.T) {
	engine, _ := newAdminTestEngine(t)

	// No JWT — expect 401.
	req := httptest.NewRequest(http.MethodGet, "/api/admin/activity/stats", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestGetActivityStats_Success(t *testing.T) {
	engine, queries := newAdminTestEngine(t)

	adminID := seedAdminUser(t, queries, "admin", "Test1234!")
	tok := adminToken(t, adminID, "admin")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	ctx := context.Background()

	// Create a system and talkgroup for seeding calls.
	sysID := seedSystem(t, queries, 100, "Test System")
	tgID, err := queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
		TalkgroupID: 200,
		SystemID:    sysID,
		Label:       sql.NullString{String: "Fire", Valid: true},
		Name:        sql.NullString{String: "FD", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup: %v", err)
	}

	// Seed calls: 3 calls today (all within today and this week).
	now := time.Now()
	seedCallAt(t, queries, sysID, tgID, now.Unix())
	seedCallAt(t, queries, sysID, tgID, now.Add(-1*time.Hour).Unix())
	seedCallAt(t, queries, sysID, tgID, now.Add(-2*time.Hour).Unix())

	w := doRequest(engine, http.MethodGet, "/api/admin/activity/stats", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ActivityStatsResponse
	decodeJSON(t, w, &resp)

	if resp.CallsToday < 3 {
		t.Errorf("CallsToday = %d, want >= 3", resp.CallsToday)
	}
	if resp.CallsThisWeek < 3 {
		t.Errorf("CallsThisWeek = %d, want >= 3", resp.CallsThisWeek)
	}
	if resp.CallsTotal != 3 {
		t.Errorf("CallsTotal = %d, want 3", resp.CallsTotal)
	}
	if resp.Uptime < 0 {
		t.Errorf("Uptime = %d, want >= 0", resp.Uptime)
	}
}

func TestGetActivityChart_Success(t *testing.T) {
	engine, queries := newAdminTestEngine(t)

	adminID := seedAdminUser(t, queries, "admin", "Test1234!")
	tok := adminToken(t, adminID, "admin")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	ctx := context.Background()

	sysID := seedSystem(t, queries, 100, "Test System")
	tgID, err := queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
		TalkgroupID: 200,
		SystemID:    sysID,
		Label:       sql.NullString{String: "Fire", Valid: true},
		Name:        sql.NullString{String: "FD", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup: %v", err)
	}

	// Seed calls within the last 24h so they appear in chart buckets.
	now := time.Now()
	seedCallAt(t, queries, sysID, tgID, now.Add(-1*time.Hour).Unix())
	seedCallAt(t, queries, sysID, tgID, now.Add(-1*time.Hour).Unix())
	seedCallAt(t, queries, sysID, tgID, now.Add(-2*time.Hour).Unix())

	w := doRequest(engine, http.MethodGet, "/api/admin/activity/chart", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ActivityChartResponse
	decodeJSON(t, w, &resp)

	if len(resp.Buckets) == 0 {
		t.Fatal("expected at least one bucket, got 0")
	}

	// Verify total call count across all buckets matches seeded calls.
	var total int64
	for _, b := range resp.Buckets {
		total += b.Count
	}
	if total != 3 {
		t.Errorf("total calls in buckets = %d, want 3", total)
	}
}

func TestGetTopTalkgroups_Success(t *testing.T) {
	engine, queries := newAdminTestEngine(t)

	adminID := seedAdminUser(t, queries, "admin", "Test1234!")
	tok := adminToken(t, adminID, "admin")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	ctx := context.Background()

	sysID := seedSystem(t, queries, 100, "Test System")

	tgID1, err := queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
		TalkgroupID: 200,
		SystemID:    sysID,
		Label:       sql.NullString{String: "Fire Dispatch", Valid: true},
		Name:        sql.NullString{String: "FD", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup: %v", err)
	}

	tgID2, err := queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
		TalkgroupID: 300,
		SystemID:    sysID,
		Label:       sql.NullString{String: "Police Ops", Valid: true},
		Name:        sql.NullString{String: "PD", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup: %v", err)
	}

	now := time.Now()
	// Seed 3 calls for tg1, 1 call for tg2 — tg1 should rank first.
	seedCallAt(t, queries, sysID, tgID1, now.Unix())
	seedCallAt(t, queries, sysID, tgID1, now.Add(-10*time.Minute).Unix())
	seedCallAt(t, queries, sysID, tgID1, now.Add(-20*time.Minute).Unix())
	seedCallAt(t, queries, sysID, tgID2, now.Add(-5*time.Minute).Unix())

	w := doRequest(engine, http.MethodGet, "/api/admin/activity/top-talkgroups", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.TopTalkgroupsResponse
	decodeJSON(t, w, &resp)

	if len(resp.Talkgroups) < 2 {
		t.Fatalf("expected >= 2 talkgroups, got %d", len(resp.Talkgroups))
	}

	// First entry should be Fire Dispatch with 3 calls.
	top := resp.Talkgroups[0]
	if top.TalkgroupLabel != "Fire Dispatch" {
		t.Errorf("top talkgroup label = %q, want %q", top.TalkgroupLabel, "Fire Dispatch")
	}
	if top.CallCount != 3 {
		t.Errorf("top talkgroup call count = %d, want 3", top.CallCount)
	}
	if top.SystemLabel != "Test System" {
		t.Errorf("top talkgroup system label = %q, want %q", top.SystemLabel, "Test System")
	}

	// Second entry should be Police Ops with 1 call.
	second := resp.Talkgroups[1]
	if second.TalkgroupLabel != "Police Ops" {
		t.Errorf("second talkgroup label = %q, want %q", second.TalkgroupLabel, "Police Ops")
	}
	if second.CallCount != 1 {
		t.Errorf("second talkgroup call count = %d, want 1", second.CallCount)
	}
}

func TestGetActivityStats_EmptyDB(t *testing.T) {
	engine, queries := newAdminTestEngine(t)

	adminID := seedAdminUser(t, queries, "admin", "Test1234!")
	tok := adminToken(t, adminID, "admin")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	w := doRequest(engine, http.MethodGet, "/api/admin/activity/stats", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ActivityStatsResponse
	decodeJSON(t, w, &resp)

	if resp.CallsToday != 0 {
		t.Errorf("CallsToday = %d, want 0", resp.CallsToday)
	}
	if resp.CallsThisWeek != 0 {
		t.Errorf("CallsThisWeek = %d, want 0", resp.CallsThisWeek)
	}
	if resp.CallsTotal != 0 {
		t.Errorf("CallsTotal = %d, want 0", resp.CallsTotal)
	}
}
