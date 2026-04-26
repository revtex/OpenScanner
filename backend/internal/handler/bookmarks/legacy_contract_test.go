package bookmarks_test

// Phase N-0 — legacy contract freeze for the bookmarks package.
//
// Pins today's wire format for /api/bookmarks, /api/bookmarks/calls and
// POST /api/bookmarks (toggle). Plan reference:
// docs/plans/native-api-design-plan.md §4.1.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/handler/routes"
	"github.com/openscanner/openscanner/internal/logging"
)

func init() {
	gin.SetMode(gin.TestMode)
	logging.Configure(true, "")
}

func bmFixture(t *testing.T) (*gin.Engine, *db.Queries, string, int64) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	q := db.New(sqlDB)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pool := audio.NewWorkerPool(ctx)
	proc := audio.NewProcessor(t.TempDir(), pool)

	r := gin.New()
	rl := auth.NewRateLimiter(context.Background())
	routes.RegisterRoutes(r, routes.Deps{Queries: q, RateLimiter: rl, Processor: proc, Version: "test"})

	hash, _ := auth.HashPassword("pw")
	now := time.Now().Unix()
	uid, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username: "alice", PasswordHash: hash, Role: auth.RoleAdmin,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	tok, _, err := auth.GenerateToken(uid, "alice", auth.RoleAdmin, 0)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	sysID, err := q.CreateSystem(context.Background(), db.CreateSystemParams{SystemID: 1, Label: "S"})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}
	callID, err := q.CreateCall(context.Background(), db.CreateCallParams{
		AudioPath: "x/a.wav", AudioName: "a.wav", AudioType: "audio/wav",
		DateTime: now, SystemID: sysID,
	})
	if err != nil {
		t.Fatalf("CreateCall: %v", err)
	}
	return r, q, tok, callID
}

// TestBookmarksLegacyContract pins request/response shapes for all three
// legacy bookmark endpoints.
func TestBookmarksLegacyContract(t *testing.T) {
	engine, _, tok, callID := bmFixture(t)
	bearer := func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+tok) }

	// 1) POST /api/bookmarks — create.
	body, _ := json.Marshal(map[string]int64{"callId": callID})
	req := httptest.NewRequest(http.MethodPost, "/api/bookmarks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	bearer(req)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var toggled struct {
		Bookmarked bool  `json:"bookmarked"`
		ID         int64 `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &toggled); err != nil {
		t.Fatalf("POST decode: %v", err)
	}
	if !toggled.Bookmarked || toggled.ID <= 0 {
		t.Errorf("POST toggle = %+v, want bookmarked=true id>0", toggled)
	}

	// 2) GET /api/bookmarks — IDs.
	req = httptest.NewRequest(http.MethodGet, "/api/bookmarks", nil)
	bearer(req)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET ids: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var ids struct {
		CallIDs []int64 `json:"callIds"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ids); err != nil {
		t.Fatalf("GET ids decode: %v", err)
	}
	if len(ids.CallIDs) != 1 || ids.CallIDs[0] != callID {
		t.Errorf("GET ids = %+v, want [%d]", ids.CallIDs, callID)
	}

	// 3) GET /api/bookmarks/calls — hydrated.
	req = httptest.NewRequest(http.MethodGet, "/api/bookmarks/calls", nil)
	bearer(req)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET calls: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var hydrated struct {
		Calls []map[string]any `json:"calls"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &hydrated); err != nil {
		t.Fatalf("GET calls decode: %v", err)
	}
	if len(hydrated.Calls) != 1 {
		t.Fatalf("GET calls len = %d, want 1", len(hydrated.Calls))
	}
	for _, k := range []string{"id", "dateTime"} {
		if _, ok := hydrated.Calls[0][k]; !ok {
			t.Errorf("GET calls row missing key %q", k)
		}
	}

	// 4) POST /api/bookmarks again — toggle off.
	req = httptest.NewRequest(http.MethodPost, "/api/bookmarks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	bearer(req)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST off: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var off struct {
		Bookmarked bool `json:"bookmarked"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &off); err != nil {
		t.Fatalf("POST off decode: %v", err)
	}
	if off.Bookmarked {
		t.Errorf("POST off = %+v, want bookmarked=false", off)
	}
}

// TestBookmarks_Unauthenticated_Returns401 pins the auth gate on the
// bookmark endpoints.
func TestBookmarks_Unauthenticated_Returns401(t *testing.T) {
	engine, _, _, _ := bmFixture(t)

	for _, p := range []string{"/api/bookmarks", "/api/bookmarks/calls"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", p, w.Code)
		}
	}

	body, _ := json.Marshal(map[string]int64{"callId": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/bookmarks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/bookmarks: status = %d, want 401", w.Code)
	}
}
