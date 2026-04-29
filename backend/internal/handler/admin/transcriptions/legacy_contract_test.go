package transcriptions_test

// Phase N-0 — legacy contract freeze for the admin/transcriptions handler package.
//
// Pins GET /api/admin/transcriptions/status response envelope:
// {enabled, url, model, diarize, totalTranscriptions, whisperAvailable}.
// Plan reference: docs/plans/native-api-design-plan.md §4.1.

import (
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

func TestGetTranscriptionStatus_LegacyResponseShape(t *testing.T) {
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
		Username: "admin", PasswordHash: hash, Role: auth.RoleAdmin,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	tok, _, err := auth.GenerateToken(uid, "admin", auth.RoleAdmin, 0)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	// Seed the transcription settings.
	for k, v := range map[string]string{
		"transcriptionEnabled": "true",
		"transcriptionUrl":     "http://whisper:9000",
		"transcriptionModel":   "tiny.en",
		"transcriptionDiarize": "false",
	} {
		if err := q.UpsertSetting(context.Background(), db.UpsertSettingParams{Key: k, Value: v}); err != nil {
			t.Fatalf("UpsertSetting %q: %v", k, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/transcriptions/status", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Enabled             bool   `json:"enabled"`
		URL                 string `json:"url"`
		Model               string `json:"model"`
		Diarize             bool   `json:"diarize"`
		TotalTranscriptions int64  `json:"totalTranscriptions"`
		WhisperAvailable    bool   `json:"whisperAvailable"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, w.Body.String())
	}

	if !resp.Enabled || resp.URL != "http://whisper:9000" || resp.Model != "tiny.en" || resp.Diarize {
		t.Errorf("status = %+v, want enabled=true url=http://whisper:9000 model=tiny.en diarize=false", resp)
	}

	// Also verify all expected keys are present in the marshalled output
	// (struct tag drift would change wire shape silently).
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("raw decode: %v", err)
	}
	for _, k := range []string{"enabled", "url", "model", "diarize", "totalTranscriptions", "whisperAvailable"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("response missing key %q (got: %s)", k, w.Body.String())
		}
	}
}
