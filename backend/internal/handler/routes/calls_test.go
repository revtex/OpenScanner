package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/handler/routes"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// newTestEngineWithCalls creates a Gin engine wired with all routes including a
// real audio Processor backed by t.TempDir().
func newTestEngineWithCalls(t *testing.T) (*gin.Engine, *db.Queries) {
	t.Helper()
	_, queries := newTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	pool := audio.NewWorkerPool(ctx)
	proc := audio.NewProcessor(t.TempDir(), pool)

	router := gin.New()
	rl := auth.NewRateLimiter(context.Background())
	routes.RegisterRoutes(router, routes.Deps{
		Queries:     queries,
		RateLimiter: rl,
		Processor:   proc,
		Version:     "test",
	})
	return router, queries
}

// seedAPIKey inserts an enabled API key (hashed) and returns its row ID.
func seedAPIKey(t *testing.T, q *db.Queries, key string) int64 {
	t.Helper()
	id, err := q.CreateAPIKey(context.Background(), db.CreateAPIKeyParams{
		Key:      auth.HashAPIKey(key),
		Disabled: 0,
	})
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	return id
}

// buildCallUpload builds a multipart/form-data body for POST /api/call-upload.
// fields is a map of non-file form fields. If includeAudio is true, a minimal
// fake WAV file is added under the "audio" field.
func buildCallUpload(t *testing.T, fields map[string]string, includeAudio bool) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField %q: %v", k, err)
		}
	}
	if includeAudio {
		fw, err := w.CreateFormFile("audio", "test.wav")
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		_, _ = fw.Write([]byte("RIFF\x24\x00\x00\x00WAVEfmt ")) // minimal fake WAV header
	}
	w.Close()
	return &body, w.FormDataContentType()
}

// TestPostCallUpload_ValidKey checks that a correctly authorised upload with
// all required fields returns 200 and a positive call ID.
func TestPostCallUpload_ValidKey(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	ctx := context.Background()

	seedAPIKey(t, queries, "valid-key")
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "autoPopulateSystems", Value: "true"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "audioConversion", Value: "0"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "disableDuplicateDetection", Value: "true"})

	body, ct := buildCallUpload(t, map[string]string{
		"systemId":    "1",
		"talkgroupId": "100",
		"dateTime":    strconv.FormatInt(time.Now().Unix(), 10),
	}, true)

	req := httptest.NewRequest(http.MethodPost, "/api/call-upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-API-Key", "valid-key")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID <= 0 {
		t.Errorf("id = %d, want > 0", resp.ID)
	}
}

// TestPostCallUpload_InvalidKey checks that an unknown API key is rejected
// with 401.
func TestPostCallUpload_InvalidKey(t *testing.T) {
	engine, _ := newTestEngineWithCalls(t)

	body, ct := buildCallUpload(t, map[string]string{
		"systemId":    "1",
		"talkgroupId": "100",
		"dateTime":    strconv.FormatInt(time.Now().Unix(), 10),
	}, true)

	req := httptest.NewRequest(http.MethodPost, "/api/call-upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-API-Key", "not-a-real-key")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// TestPostCallUpload_MissingAudio checks that a request with a valid API key
// and all required form fields but no audio file returns 417 with an
// rdio-scanner-compatible "Incomplete call data" plain-text message.
func TestPostCallUpload_MissingAudio(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	ctx := context.Background()

	seedAPIKey(t, queries, "key-noaudio")
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "autoPopulateSystems", Value: "true"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "audioConversion", Value: "0"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "disableDuplicateDetection", Value: "true"})

	// includeAudio=false → no audio part in the form.
	body, ct := buildCallUpload(t, map[string]string{
		"systemId":    "2",
		"talkgroupId": "200",
		"dateTime":    strconv.FormatInt(time.Now().Unix(), 10),
	}, false)

	req := httptest.NewRequest(http.MethodPost, "/api/call-upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-API-Key", "key-noaudio")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusExpectationFailed {
		t.Errorf("status = %d, want 417; body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "Incomplete call data: no audio\n" {
		t.Errorf("body = %q, want %q", got, "Incomplete call data: no audio\n")
	}
}

// TestPostCallUpload_Duplicate checks that a second call with the same
// system, talkgroup, and timestamp within the duplicate window returns 200
// with {"message":"duplicate"}.
func TestPostCallUpload_Duplicate(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	ctx := context.Background()

	seedAPIKey(t, queries, "key-dup")
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "autoPopulateSystems", Value: "true"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "audioConversion", Value: "0"})
	// Use a 30-second window so the second call with the same timestamp is
	// always detected as a duplicate (diff = 0 ms < 30 000 ms).
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "duplicateDetectionTimeFrame", Value: "30000"})

	fixedTime := strconv.FormatInt(time.Now().Unix(), 10)

	sendCall := func() *httptest.ResponseRecorder {
		body, ct := buildCallUpload(t, map[string]string{
			"systemId":    "3",
			"talkgroupId": "300",
			"dateTime":    fixedTime,
		}, true)
		req := httptest.NewRequest(http.MethodPost, "/api/call-upload", body)
		req.Header.Set("Content-Type", ct)
		req.Header.Set("X-API-Key", "key-dup")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}

	// First call must be ingested normally.
	w1 := sendCall()
	if w1.Code != http.StatusOK {
		t.Fatalf("first call: status = %d, want 200; body: %s", w1.Code, w1.Body.String())
	}
	var r1 struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(w1.Body).Decode(&r1); err != nil {
		t.Fatalf("decode first call response: %v", err)
	}
	if r1.ID <= 0 {
		t.Errorf("first call id = %d, want > 0", r1.ID)
	}

	// Second identical call must be detected as a duplicate.
	w2 := sendCall()
	if w2.Code != http.StatusOK {
		t.Fatalf("duplicate call: status = %d, want 200; body: %s", w2.Code, w2.Body.String())
	}
	var r2 struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(w2.Body).Decode(&r2); err != nil {
		t.Fatalf("decode duplicate response: %v", err)
	}
	if r2.Message != "duplicate call rejected" {
		t.Errorf("message = %q, want \"duplicate call rejected\"", r2.Message)
	}
}

// TestPostCallUpload_RateLimited checks that exceeding the per-API-key call
// rate (set to 1 per minute) results in 429.
func TestPostCallUpload_RateLimited(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	ctx := context.Background()

	seedAPIKey(t, queries, "key-ratelimit")
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "autoPopulateSystems", Value: "true"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "audioConversion", Value: "0"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "apiKeyCallRate", Value: "1"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "disableDuplicateDetection", Value: "true"})

	sendCall := func(ts string) *httptest.ResponseRecorder {
		body, ct := buildCallUpload(t, map[string]string{
			"systemId":    "4",
			"talkgroupId": "400",
			"dateTime":    ts,
		}, true)
		req := httptest.NewRequest(http.MethodPost, "/api/call-upload", body)
		req.Header.Set("Content-Type", ct)
		req.Header.Set("X-API-Key", "key-ratelimit")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}

	now := time.Now().Unix()

	// First call must succeed (count=0 → 1, rateLimit=1 → allow).
	w1 := sendCall(strconv.FormatInt(now, 10))
	if w1.Code != http.StatusOK {
		t.Fatalf("first call: status = %d, want 200; body: %s", w1.Code, w1.Body.String())
	}

	// Second call must be rate-limited (count=1 >= rateLimit=1 → reject).
	w2 := sendCall(strconv.FormatInt(now+1, 10))
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second call: status = %d, want 429; body: %s", w2.Code, w2.Body.String())
	}
}

// TestPostCallUpload_TestConnection checks that SDRTrunk's test=1 connection
// check returns 200 with the expected response string.
func TestPostCallUpload_TestConnection(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	seedAPIKey(t, queries, "key-test")

	body, ct := buildCallUpload(t, map[string]string{
		"test":   "1",
		"system": "1",
	}, false)

	req := httptest.NewRequest(http.MethodPost, "/api/call-upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-API-Key", "key-test")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "Incomplete call data: no talkgroup\n" {
		t.Errorf("body = %q, want %q", got, "Incomplete call data: no talkgroup\n")
	}
}

// TestPostCallUpload_SDRTrunkConnectivity checks that SDRTrunk's implicit
// connectivity test (multipart POST with only `key`, no test=1 flag) returns
// an rdio-scanner-compatible "Incomplete call data: no talkgroup" response at
// 417. SDRTrunk explicitly checks for this prefix to confirm connectivity.
func TestPostCallUpload_SDRTrunkConnectivity(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	seedAPIKey(t, queries, "key-sdrtrunk")

	// SDRTrunk sends only the API key field — no dateTime, system, talkgroup, or audio.
	body, ct := buildCallUpload(t, map[string]string{}, false)

	req := httptest.NewRequest(http.MethodPost, "/api/call-upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-API-Key", "key-sdrtrunk")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusExpectationFailed {
		t.Fatalf("status = %d, want 417; body: %s", w.Code, w.Body.String())
	}
	// rdio-scanner's IsValid() checks all fields without early returns; the
	// last failing check wins. With no talkgroup the last error is always
	// "no talkgroup", which SDRTrunk checks via startsWith().
	if got := w.Body.String(); got != "Incomplete call data: no talkgroup\n" {
		t.Errorf("body = %q, want %q", got, "Incomplete call data: no talkgroup\n")
	}
}

// TestPostCallUpload_SuccessMessage checks that the success response contains
// the string SDRTrunk expects.
func TestPostCallUpload_SuccessMessage(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	ctx := context.Background()

	seedAPIKey(t, queries, "key-msg")
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "autoPopulateSystems", Value: "true"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "audioConversion", Value: "0"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "disableDuplicateDetection", Value: "true"})

	body, ct := buildCallUpload(t, map[string]string{
		"systemId":    "10",
		"talkgroupId": "1000",
		"dateTime":    strconv.FormatInt(time.Now().Unix(), 10),
	}, true)

	req := httptest.NewRequest(http.MethodPost, "/api/call-upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-API-Key", "key-msg")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ID      int64  `json:"id"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID <= 0 {
		t.Errorf("id = %d, want > 0", resp.ID)
	}
	if resp.Message != "Call imported successfully." {
		t.Errorf("message = %q, want \"Call imported successfully.\"", resp.Message)
	}
}

// TestPostCallUpload_SystemLabelAutoPopulate checks that the systemLabel form
// field is used as the system label when auto-populating.
func TestPostCallUpload_SystemLabelAutoPopulate(t *testing.T) {
	engine, queries := newTestEngineWithCalls(t)
	ctx := context.Background()

	seedAPIKey(t, queries, "key-syslbl")
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "autoPopulateSystems", Value: "true"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "audioConversion", Value: "0"})
	_ = queries.UpsertSetting(ctx, db.UpsertSettingParams{Key: "disableDuplicateDetection", Value: "true"})

	body, ct := buildCallUpload(t, map[string]string{
		"system":      "99",
		"talkgroup":   "500",
		"dateTime":    strconv.FormatInt(time.Now().Unix(), 10),
		"systemLabel": "My P25 System",
	}, true)

	req := httptest.NewRequest(http.MethodPost, "/api/call-upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-API-Key", "key-syslbl")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	sys, err := queries.GetSystemBySystemID(ctx, 99)
	if err != nil {
		t.Fatalf("GetSystemBySystemID: %v", err)
	}
	if sys.Label != "My P25 System" {
		t.Errorf("system label = %q, want \"My P25 System\"", sys.Label)
	}
}
