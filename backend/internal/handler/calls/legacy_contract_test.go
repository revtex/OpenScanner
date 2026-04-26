package calls_test

// Phase N-0 — legacy contract freeze for the calls handler package.
//
// These tests pin today's wire format on the rdio-scanner-shaped legacy REST
// surface so the upcoming /api/v1/* native API work cannot drift the legacy
// shape without breaking a regression. Plan reference:
// docs/plans/native-api-design-plan.md §4.1 (REST endpoints) and §5
// (multipart call-upload field map).
//
// Test-only file. No production code is touched.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// engineWithCalls wires a real Gin engine with audio Processor and a fresh
// in-memory DB. No Hub/notifier/transcriber — those code paths are nil-safe
// in the upload handler.
func engineWithCalls(t *testing.T) (*gin.Engine, *db.Queries, string) {
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
	recordingsDir := t.TempDir()
	proc := audio.NewProcessor(recordingsDir, pool)

	r := gin.New()
	rl := auth.NewRateLimiter(context.Background())
	routes.RegisterRoutes(r, routes.Deps{
		Queries:     q,
		RateLimiter: rl,
		Processor:   proc,
		Version:     "test",
	})
	return r, q, recordingsDir
}

func seedAPIKey(t *testing.T, q *db.Queries, raw string) {
	t.Helper()
	if _, err := q.CreateAPIKey(context.Background(), db.CreateAPIKeyParams{
		Key:      auth.HashAPIKey(raw),
		Disabled: 0,
	}); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
}

func enableAutoPopulate(t *testing.T, q *db.Queries) {
	t.Helper()
	ctx := context.Background()
	for k, v := range map[string]string{
		"autoPopulateSystems":       "true",
		"audioConversion":           "0",
		"disableDuplicateDetection": "true",
	} {
		if err := q.UpsertSetting(ctx, db.UpsertSettingParams{Key: k, Value: v}); err != nil {
			t.Fatalf("UpsertSetting %q: %v", k, err)
		}
	}
}

// buildUpload writes the canonical happy-path multipart body. Caller picks
// auth transport (header / query / form).
func buildUpload(t *testing.T) (body *bytes.Buffer, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range map[string]string{
		"systemId":    "1",
		"talkgroupId": "100",
		"dateTime":    strconv.FormatInt(time.Now().Unix(), 10),
	} {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField %q: %v", k, err)
		}
	}
	fw, err := w.CreateFormFile("audio", "test.wav")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write([]byte("RIFF\x24\x00\x00\x00WAVEfmt "))
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

// canonJSON returns a stable, key-sorted JSON byte representation of v so
// expected/actual bodies can be byte-compared without map-key-order flake.
func canonJSON(t *testing.T, v any) []byte {
	t.Helper()
	if s, ok := v.([]byte); ok {
		var raw any
		if err := json.Unmarshal(s, &raw); err != nil {
			t.Fatalf("canonJSON unmarshal: %v\nbody: %s", err, s)
		}
		v = raw
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("canonJSON marshal: %v", err)
	}
	return b
}

// TestPostCallUpload_APIKeyTransports — Phase N-0 contract freeze for
// middleware.APIKeyAuth: all three legacy transports (X-API-Key header,
// ?key=… query, key=… form field) must succeed with the same key value and
// produce the same response body. This pins the precedence header → query →
// form documented in backend/internal/middleware/auth.go.
func TestPostCallUpload_APIKeyTransports(t *testing.T) {
	const apiKey = "super-secret-key"

	tests := []struct {
		name      string
		transport string // "header" | "query" | "form"
	}{
		{"X-API-Key header", "header"},
		{"key query param", "query"},
		{"key form field", "form"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine, queries, _ := engineWithCalls(t)
			seedAPIKey(t, queries, apiKey)
			enableAutoPopulate(t, queries)

			body, ct := buildUpload(t)

			urlPath := "/api/call-upload"
			if tc.transport == "query" {
				urlPath += "?key=" + url.QueryEscape(apiKey)
			}
			if tc.transport == "form" {
				// Rebuild the multipart with the key field included.
				var buf bytes.Buffer
				w := multipart.NewWriter(&buf)
				_ = w.WriteField("key", apiKey)
				_ = w.WriteField("systemId", "1")
				_ = w.WriteField("talkgroupId", "100")
				_ = w.WriteField("dateTime", strconv.FormatInt(time.Now().Unix(), 10))
				fw, _ := w.CreateFormFile("audio", "test.wav")
				_, _ = fw.Write([]byte("RIFF\x24\x00\x00\x00WAVEfmt "))
				_ = w.Close()
				body, ct = &buf, w.FormDataContentType()
			}

			req := httptest.NewRequest(http.MethodPost, urlPath, body)
			req.Header.Set("Content-Type", ct)
			if tc.transport == "header" {
				req.Header.Set("X-API-Key", apiKey)
			}
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("transport=%s: status = %d, want 200; body: %s", tc.transport, w.Code, w.Body.String())
			}
			if got, want := w.Header().Get("Content-Type"), "application/json"; !strings.HasPrefix(got, want) {
				t.Errorf("transport=%s: Content-Type = %q, want prefix %q", tc.transport, got, want)
			}

			var resp struct {
				ID      int64  `json:"id"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("transport=%s: decode: %v\nbody: %s", tc.transport, err, w.Body.String())
			}
			if resp.ID <= 0 {
				t.Errorf("transport=%s: id = %d, want > 0", tc.transport, resp.ID)
			}
			if resp.Message != "Call imported successfully." {
				t.Errorf("transport=%s: message = %q, want %q",
					tc.transport, resp.Message, "Call imported successfully.")
			}
		})
	}
}

// TestPostCallUpload_LegacyAlias pins that POST /api/trunk-recorder-call-upload
// is wire-equivalent to POST /api/call-upload.
func TestPostCallUpload_LegacyAlias(t *testing.T) {
	engine, queries, _ := engineWithCalls(t)
	seedAPIKey(t, queries, "tr-key")
	enableAutoPopulate(t, queries)

	body, ct := buildUpload(t)
	req := httptest.NewRequest(http.MethodPost, "/api/trunk-recorder-call-upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-API-Key", "tr-key")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ID      int64  `json:"id"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID <= 0 || resp.Message != "Call imported successfully." {
		t.Errorf("alias response = %+v, want id>0 + message=\"Call imported successfully.\"", resp)
	}
}

// TestPostCallUpload_TestConnectionCheck_PlainText pins the rdio-scanner /
// SDRTrunk connectivity-check shape: test=1 returns plain text 200 with the
// exact "Incomplete call data: no talkgroup\n" body.
func TestPostCallUpload_TestConnectionCheck_PlainText(t *testing.T) {
	engine, queries, _ := engineWithCalls(t)
	seedAPIKey(t, queries, "k1")

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("test", "1")
	_ = w.WriteField("system", "1")
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/call-upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-API-Key", "k1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Body.String(), "Incomplete call data: no talkgroup\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain prefix", got)
	}
}

// TestGetCalls_LegacyResponseShape pins the legacy GET /api/calls envelope:
// {"calls":[…], "total": <int>}. Does not pin per-row volatile fields (id,
// dateTime); only the envelope keys and types.
func TestGetCalls_LegacyResponseShape(t *testing.T) {
	engine, queries, _ := engineWithCalls(t)
	ctx := context.Background()

	// Public access on so this works without a JWT.
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "publicAccess", Value: "true"})

	sysID, err := queries.CreateSystem(ctx, db.CreateSystemParams{SystemID: 1, Label: "S"})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}
	if _, err := queries.CreateCall(ctx, db.CreateCallParams{
		AudioPath: "x/a.wav", AudioName: "a.wav", AudioType: "audio/wav",
		DateTime: time.Now().Unix(), SystemID: sysID,
	}); err != nil {
		t.Fatalf("CreateCall: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/calls?limit=1", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var env struct {
		Calls []map[string]any `json:"calls"`
		Total int64            `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, w.Body.String())
	}
	if env.Total < 1 {
		t.Errorf("total = %d, want >= 1", env.Total)
	}
	if len(env.Calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(env.Calls))
	}
	row := env.Calls[0]
	for _, k := range []string{"id", "dateTime", "systemId"} {
		if _, ok := row[k]; !ok {
			t.Errorf("call row missing key %q (got keys: %v)", k, mapKeys(row))
		}
	}
}

// TestGetCallAudio_LegacyContentDisposition pins the inline disposition and
// streaming behaviour of GET /api/calls/:id/audio when the caller carries a
// valid Bearer JWT.
func TestGetCallAudio_LegacyContentDisposition(t *testing.T) {
	engine, queries, recordingsDir := engineWithCalls(t)
	ctx := context.Background()

	// Seed a real audio file + call row.
	relPath := filepath.Join("legacy", "audio.wav")
	abs := filepath.Join(recordingsDir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const body = "FAKE_AUDIO"
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatalf("write audio: %v", err)
	}

	sysID, err := queries.CreateSystem(ctx, db.CreateSystemParams{SystemID: 1, Label: "S"})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}
	callID, err := queries.CreateCall(ctx, db.CreateCallParams{
		AudioPath: relPath, AudioName: "audio.wav", AudioType: "audio/wav",
		DateTime: time.Now().Unix(), SystemID: sysID,
	})
	if err != nil {
		t.Fatalf("CreateCall: %v", err)
	}

	// Seed an admin user and mint a JWT for them.
	hash, _ := auth.HashPassword("pw")
	uid, err := queries.CreateUser(ctx, db.CreateUserParams{
		Username: "alice", PasswordHash: hash, Role: auth.RoleAdmin,
		CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	tok, _, err := auth.GenerateToken(uid, "alice", auth.RoleAdmin, 0)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/audio", callID), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if got, want := w.Body.String(), body; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got := w.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline") {
		t.Errorf("Content-Disposition = %q, want inline prefix", got)
	}
	if got, want := w.Header().Get("Content-Type"), "audio/wav"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
}

// TestGetCallTranscript_LegacyResponseShape pins the JSON keys returned by
// GET /api/calls/:id/transcript: {text, segments[], language, model}.
func TestGetCallTranscript_LegacyResponseShape(t *testing.T) {
	engine, queries, _ := engineWithCalls(t)
	ctx := context.Background()
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "publicAccess", Value: "true"})

	sysID, err := queries.CreateSystem(ctx, db.CreateSystemParams{SystemID: 1, Label: "S"})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}
	callID, err := queries.CreateCall(ctx, db.CreateCallParams{
		AudioPath: "x/a.wav", AudioName: "a.wav", AudioType: "audio/wav",
		DateTime: time.Now().Unix(), SystemID: sysID,
	})
	if err != nil {
		t.Fatalf("CreateCall: %v", err)
	}
	if _, err := queries.CreateTranscription(ctx, db.CreateTranscriptionParams{
		CallID:    callID,
		Text:      "hello world",
		Language:  sql.NullString{String: "en", Valid: true},
		Model:     sql.NullString{String: "tiny.en", Valid: true},
		Segments:  sql.NullString{String: `[{"start":0,"end":1,"text":"hello"}]`, Valid: true},
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("CreateTranscription: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calls/%d/transcript", callID), nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, k := range []string{"text", "segments", "language", "model"} {
		if _, ok := resp[k]; !ok {
			t.Errorf("transcript response missing key %q (got keys: %v)", k, mapKeys(resp))
		}
	}
	if resp["text"] != "hello world" {
		t.Errorf("text = %v, want \"hello world\"", resp["text"])
	}
	if resp["language"] != "en" {
		t.Errorf("language = %v, want \"en\"", resp["language"])
	}
	if resp["model"] != "tiny.en" {
		t.Errorf("model = %v, want \"tiny.en\"", resp["model"])
	}

	// The segments array must round-trip through canonical JSON without
	// shape drift.
	wantSegs := canonJSON(t, []map[string]any{{"start": 0, "end": 1, "text": "hello"}})
	gotSegs := canonJSON(t, resp["segments"])
	if !bytes.Equal(gotSegs, wantSegs) {
		t.Errorf("segments shape drift\n got:  %s\n want: %s", gotSegs, wantSegs)
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
