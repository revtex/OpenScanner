package ws

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// --- v1 message constructor shape tests ---

func TestNewWelcomeV1(t *testing.T) {
	b, err := NewWelcomeV1("1.2.3", "OpenScanner", "ops@example.com")
	if err != nil {
		t.Fatalf("NewWelcomeV1: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["type"] != TypeWelcome {
		t.Errorf("type = %v, want %s", m["type"], TypeWelcome)
	}
	if m["version"] != "1.2.3" {
		t.Errorf("version = %v", m["version"])
	}
	if m["branding"] != "OpenScanner" {
		t.Errorf("branding = %v", m["branding"])
	}
	if m["email"] != "ops@example.com" {
		t.Errorf("email = %v", m["email"])
	}
}

func TestNewScannerConfigV1(t *testing.T) {
	cfg := map[string]any{"systems": []any{}, "time12hFormat": true}
	b, err := NewScannerConfigV1(cfg)
	if err != nil {
		t.Fatalf("NewScannerConfigV1: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["type"] != TypeScannerConfig {
		t.Errorf("type = %v", m["type"])
	}
	inner, ok := m["config"].(map[string]any)
	if !ok {
		t.Fatalf("config not an object: %T", m["config"])
	}
	if inner["time12hFormat"] != true {
		t.Errorf("time12hFormat lost in round-trip")
	}
}

func TestNewCallNewV1(t *testing.T) {
	call := map[string]any{
		"id":          int64(42),
		"systemId":    int64(1),
		"talkgroupId": int64(100),
		"audioName":   "abc.m4a",
		"audioType":   "audio/mp4",
	}
	b, err := NewCallNewV1(call)
	if err != nil {
		t.Fatalf("NewCallNewV1: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["type"] != TypeCallNew {
		t.Errorf("type = %v, want %s", m["type"], TypeCallNew)
	}
	c, ok := m["call"].(map[string]any)
	if !ok {
		t.Fatalf("call not an object")
	}
	for _, k := range []string{"id", "systemId", "talkgroupId", "audioName", "audioType"} {
		if _, ok := c[k]; !ok {
			t.Errorf("call.%s missing", k)
		}
	}
}

func TestNewSessionExpiredV1_BytePin(t *testing.T) {
	b, err := NewSessionExpiredV1()
	if err != nil {
		t.Fatalf("NewSessionExpiredV1: %v", err)
	}
	const want = `{"type":"session.expired"}`
	if string(b) != want {
		t.Errorf("session.expired bytes = %s, want %s", b, want)
	}
}

func TestNewListenerCountV1(t *testing.T) {
	b, err := NewListenerCountV1(7)
	if err != nil {
		t.Fatalf("NewListenerCountV1: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["type"] != TypeListenerCount {
		t.Errorf("type = %v", m["type"])
	}
	// JSON numbers decode to float64
	if n, _ := m["count"].(float64); n != 7 {
		t.Errorf("count = %v, want 7", m["count"])
	}
}

func TestNewFeedMapSnapshotV1(t *testing.T) {
	b, err := NewFeedMapSnapshotV1(map[string]any{"1": map[string]bool{"100": true}})
	if err != nil {
		t.Fatalf("NewFeedMapSnapshotV1: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["type"] != TypeFeedMapSnapshot {
		t.Errorf("type = %v", m["type"])
	}
	if _, ok := m["feedMap"]; !ok {
		t.Errorf("feedMap missing")
	}
}

func TestNewRejectedV1(t *testing.T) {
	b, err := NewRejectedV1("max clients reached")
	if err != nil {
		t.Fatalf("NewRejectedV1: %v", err)
	}
	const want = `{"reason":"max clients reached","type":"connection.rejected"}`
	if string(b) != want {
		t.Errorf("rejected bytes = %s, want %s", b, want)
	}
}

func TestNewCallTranscriptV1(t *testing.T) {
	// Without segments
	b, err := NewCallTranscriptV1(99, "hello world", nil)
	if err != nil {
		t.Fatalf("NewCallTranscriptV1: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["type"] != TypeCallTranscript {
		t.Errorf("type = %v", m["type"])
	}
	if _, ok := m["segments"]; ok {
		t.Errorf("segments must be omitted when nil")
	}

	// With segments
	b2, _ := NewCallTranscriptV1(99, "hi", []any{map[string]any{"start": 0.0, "end": 1.0, "text": "hi"}})
	var m2 map[string]any
	_ = json.Unmarshal(b2, &m2)
	if _, ok := m2["segments"]; !ok {
		t.Errorf("segments must be present when non-nil")
	}
}

func TestNewAdminEventV1(t *testing.T) {
	b, err := NewAdminEventV1("activity.updated", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("NewAdminEventV1: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["type"] != TypeAdminEvent {
		t.Errorf("type = %v", m["type"])
	}
	if m["topic"] != "activity.updated" {
		t.Errorf("topic = %v", m["topic"])
	}
	if _, ok := m["at"]; !ok {
		t.Errorf("at timestamp missing")
	}
	if _, ok := m["data"]; !ok {
		t.Errorf("data missing")
	}
}

func TestNewAdminResponseV1(t *testing.T) {
	b, err := NewAdminResponseV1("req-1", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("NewAdminResponseV1: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["type"] != TypeAdminResponse {
		t.Errorf("type = %v", m["type"])
	}
	if m["reqId"] != "req-1" {
		t.Errorf("reqId = %v", m["reqId"])
	}
	if m["ok"] != true {
		t.Errorf("ok = %v, want true", m["ok"])
	}
	if _, ok := m["data"]; !ok {
		t.Errorf("data missing")
	}
}

func TestNewAdminResponseErrorV1(t *testing.T) {
	b, err := NewAdminResponseErrorV1("req-2", NativeErrCodeValidation, "bad input", map[string]any{"field": "name"})
	if err != nil {
		t.Fatalf("NewAdminResponseErrorV1: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["type"] != TypeAdminResponse {
		t.Errorf("type = %v", m["type"])
	}
	if m["reqId"] != "req-2" {
		t.Errorf("reqId = %v", m["reqId"])
	}
	if m["ok"] != false {
		t.Errorf("ok = %v, want false", m["ok"])
	}
	envObj, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("error not an object")
	}
	if envObj["code"] != NativeErrCodeValidation {
		t.Errorf("error.code = %v", envObj["code"])
	}
	if envObj["message"] != "bad input" {
		t.Errorf("error.message = %v", envObj["message"])
	}
	if _, ok := envObj["details"]; !ok {
		t.Errorf("error.details missing")
	}
}

func TestNewAdminResponseErrorV1_OmitsDetailsWhenNil(t *testing.T) {
	b, err := NewAdminResponseErrorV1("r", NativeErrCodeUnknownOp, "no", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.Contains(string(b), `"details"`) {
		t.Errorf("details must be omitted when nil; got %s", b)
	}
}

// --- v1 handler integration tests ---

func readV1Frame(ctx context.Context, t *testing.T, conn *websocket.Conn, timeout time.Duration) map[string]any {
	t.Helper()
	data := readTextMessage(ctx, t, conn, timeout)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal v1 frame %q: %v", data, err)
	}
	return m
}

func TestHandleListenerWSv1_PublicAccess_WelcomeAndConfig(t *testing.T) {
	queries := newWSTestDB(t)
	if err := queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "publicAccess", Value: "true",
	}); err != nil {
		t.Fatalf("upsert publicAccess: %v", err)
	}

	hub := NewHub(queries, "test-version")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(HandleListenerWSv1(hub, queries))
	defer srv.Close()
	wsURL := "ws" + srv.URL[len("http"):]

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	welcome := readV1Frame(ctx, t, conn, 5*time.Second)
	if welcome["type"] != TypeWelcome {
		t.Errorf("first frame type = %v, want %s", welcome["type"], TypeWelcome)
	}
	if welcome["version"] != "test-version" {
		t.Errorf("welcome.version = %v", welcome["version"])
	}

	cfg := readV1Frame(ctx, t, conn, 5*time.Second)
	if cfg["type"] != TypeScannerConfig {
		t.Errorf("second frame type = %v, want %s", cfg["type"], TypeScannerConfig)
	}
	if _, ok := cfg["config"].(map[string]any); !ok {
		t.Errorf("scanner.config.config must be an object, got %T", cfg["config"])
	}
}

func TestHandleListenerWSv1_BroadcastCallNew(t *testing.T) {
	queries := newWSTestDB(t)
	if err := queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "publicAccess", Value: "true",
	}); err != nil {
		t.Fatalf("upsert publicAccess: %v", err)
	}

	hub := NewHub(queries, "test-version")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(HandleListenerWSv1(hub, queries))
	defer srv.Close()
	wsURL := "ws" + srv.URL[len("http"):]

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	// Drain welcome + config.
	_ = readV1Frame(ctx, t, conn, 5*time.Second)
	_ = readV1Frame(ctx, t, conn, 5*time.Second)

	// Wait for the client to be registered with the hub before broadcasting.
	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	// Push a call through the hub.
	payload := map[string]any{
		"id":          int64(7),
		"systemId":    int64(1),
		"talkgroupId": int64(100),
		"audioName":   "x.m4a",
		"audioType":   "audio/mp4",
		"dateTime":    int64(1700000000),
	}
	hub.BroadcastCAL(payload, nil)

	got := readV1Frame(ctx, t, conn, 5*time.Second)
	if got["type"] != TypeCallNew {
		t.Fatalf("broadcast frame type = %v, want %s", got["type"], TypeCallNew)
	}
	c, ok := got["call"].(map[string]any)
	if !ok {
		t.Fatalf("call payload not an object")
	}
	if id, _ := c["id"].(float64); id != 7 {
		t.Errorf("call.id = %v, want 7", c["id"])
	}
	if c["audioName"] != "x.m4a" {
		t.Errorf("call.audioName = %v", c["audioName"])
	}
}

func TestHandleAdminWSv1_AuthAndUnknownOp(t *testing.T) {
	queries := newWSTestDB(t)

	now := time.Now().Unix()
	userID, err := queries.CreateUser(context.Background(), db.CreateUserParams{
		Username:     "admin1",
		PasswordHash: "unused",
		Role:         auth.RoleAdmin,
		Disabled:     0,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, _, err := auth.GenerateToken(userID, "admin1", auth.RoleAdmin, 0)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	hub := NewHub(queries, "test-version")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(HandleAdminWSv1(hub, queries))
	defer srv.Close()
	wsURL := "ws" + srv.URL[len("http"):]

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	// First-message JWT auth — wire shape is JSON array carrying the token,
	// matching the legacy admin auth handshake.
	authMsg, _ := json.Marshal([]any{token})
	if err := conn.Write(ctx, websocket.MessageText, authMsg); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	// Send an admin.request with an unknown op and assert the v1 error envelope.
	req := map[string]any{
		"type":  TypeAdminRequest,
		"reqId": "rq-1",
		"op":    "this.does.not.exist",
	}
	body, _ := json.Marshal(req)
	if err := conn.Write(ctx, websocket.MessageText, body); err != nil {
		t.Fatalf("write request: %v", err)
	}

	// The hub may emit admin.event frames (e.g. activity ticks) before the
	// response — skip until we see admin.response with our reqId.
	deadline := time.Now().Add(5 * time.Second)
	var resp map[string]any
	for time.Now().Before(deadline) {
		frame := readV1Frame(ctx, t, conn, 5*time.Second)
		if frame["type"] == TypeAdminResponse && frame["reqId"] == "rq-1" {
			resp = frame
			break
		}
	}
	if resp == nil {
		t.Fatalf("did not receive admin.response for rq-1")
	}
	if resp["ok"] != false {
		t.Errorf("ok = %v, want false", resp["ok"])
	}
	envObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("error not an object: %v", resp["error"])
	}
	if envObj["code"] != NativeErrCodeUnknownOp {
		t.Errorf("error.code = %v, want %s", envObj["code"], NativeErrCodeUnknownOp)
	}
	if msg, _ := envObj["message"].(string); !strings.Contains(msg, "this.does.not.exist") {
		t.Errorf("error.message = %v", envObj["message"])
	}
}
