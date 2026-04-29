package share_test

// Phase N-0 — legacy contract freeze for the share handler package.
//
// Pins today's wire format for the rdio-scanner-shaped share endpoints under
// /api/calls/:id/share, /api/shared/:token and /api/shared/:token/audio so
// the upcoming /api/v1/* native API work cannot drift the legacy surface.
// Plan reference: docs/plans/native-api-design-plan.md §4.1.

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
	"github.com/google/uuid"

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

type shareFixture struct {
	engine *gin.Engine
	q      *db.Queries
	dir    string
	token  string // bearer JWT
	userID int64
	callID int64
}

func newShareFixture(t *testing.T) *shareFixture {
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
	dir := t.TempDir()
	proc := audio.NewProcessor(dir, pool)

	r := gin.New()
	rl := auth.NewRateLimiter(context.Background())
	routes.RegisterRoutes(r, routes.Deps{
		Queries: q, RateLimiter: rl, Processor: proc, Version: "test",
	})

	if err := q.UpsertSetting(context.Background(), db.UpsertSettingParams{Key: "shareableLinks", Value: "true"}); err != nil {
		t.Fatalf("UpsertSetting: %v", err)
	}

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

	// Audio file + call row.
	rel := filepath.Join("share", "a.wav")
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte("AUDIO"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sysID, err := q.CreateSystem(context.Background(), db.CreateSystemParams{SystemID: 1, Label: "S"})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}
	tgID, err := q.CreateTalkgroup(context.Background(), db.CreateTalkgroupParams{
		SystemID: sysID, TalkgroupID: 100,
		Label: sql.NullString{String: "Fire", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup: %v", err)
	}
	callID, err := q.CreateCall(context.Background(), db.CreateCallParams{
		AudioPath: rel, AudioName: "a.wav", AudioType: "audio/wav",
		DateTime:    now,
		SystemID:    sysID,
		TalkgroupID: sql.NullInt64{Int64: tgID, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateCall: %v", err)
	}

	return &shareFixture{engine: r, q: q, dir: dir, token: tok, userID: uid, callID: callID}
}

func (f *shareFixture) bearer(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+f.token)
}

// TestShareLegacyContract pins the request/response shapes for the four
// share endpoints + the public token endpoints. Each step asserts the keys
// and types that the legacy surface promises.
func TestShareLegacyContract(t *testing.T) {
	f := newShareFixture(t)

	// 1) POST /api/calls/:id/share — create.
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/calls/%d/share", f.callID), nil)
	f.bearer(req)
	w := httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("POST share: status = %d, want 200/201; body: %s", w.Code, w.Body.String())
	}
	var created struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("POST share decode: %v", err)
	}
	if created.Token == "" {
		t.Fatal("POST share: empty token")
	}
	if _, err := uuid.Parse(created.Token); err != nil {
		t.Errorf("POST share: token = %q, want UUID: %v", created.Token, err)
	}
	wantURL := "/call/" + created.Token
	if created.URL != wantURL {
		t.Errorf("POST share: url = %q, want %q", created.URL, wantURL)
	}

	// 2) GET /api/calls/:id/share — read existing.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/share", f.callID), nil)
	f.bearer(req)
	w = httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET share: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var got struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("GET share decode: %v", err)
	}
	if got.Token != created.Token {
		t.Errorf("GET share: token = %q, want %q", got.Token, created.Token)
	}

	// 3) GET /api/shared/:token — public payload shape.
	req = httptest.NewRequest(http.MethodGet, "/api/shared/"+created.Token, nil)
	w = httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET shared: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var public map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &public); err != nil {
		t.Fatalf("GET shared decode: %v", err)
	}
	for _, k := range []string{
		"token", "dateTime", "systemLabel", "talkgroupLabel", "talkgroupName",
		"frequency", "duration", "source", "audioUrl",
	} {
		if _, ok := public[k]; !ok {
			t.Errorf("GET shared: missing key %q", k)
		}
	}
	if public["token"] != created.Token {
		t.Errorf("GET shared: token = %v, want %q", public["token"], created.Token)
	}

	// 4) GET /api/shared/:token/audio — public stream.
	req = httptest.NewRequest(http.MethodGet, "/api/shared/"+created.Token+"/audio", nil)
	w = httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET shared/audio: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "AUDIO" {
		t.Errorf("GET shared/audio: body = %q, want %q", w.Body.String(), "AUDIO")
	}

	// 5) DELETE /api/calls/:id/share — revoke.
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/calls/%d/share", f.callID), nil)
	f.bearer(req)
	w = httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("DELETE share: status = %d, want 200/204; body: %s", w.Code, w.Body.String())
	}

	// 6) Subsequent GET /api/shared/:token must 404.
	req = httptest.NewRequest(http.MethodGet, "/api/shared/"+created.Token, nil)
	w = httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("GET shared after delete: status = %d, want 404", w.Code)
	}
}
