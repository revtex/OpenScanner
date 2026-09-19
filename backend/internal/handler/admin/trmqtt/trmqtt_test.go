package trmqtt_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"

	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/handler/routes"
	"github.com/openscanner/openscanner/internal/logging"
	trmqttsvc "github.com/openscanner/openscanner/internal/trmqtt"
)

func init() {
	gin.SetMode(gin.TestMode)
	logging.Configure(true, "")
}

const testEncryptionKey = "test-passphrase-very-long-key-1234567890"

// trTestEnv bundles the engine, queries, admin token, and (optionally) a
// real Manager wired into routes.Deps.
type trTestEnv struct {
	engine  *gin.Engine
	queries *db.Queries
	token   string
	manager *trmqttsvc.Manager // may be nil
}

// newTREnv builds a fresh test environment. When withManager is true, a real
// Manager is constructed (but not Start()-ed; admin handlers treat its errors
// as best-effort). When enable is true, the trMqttEnabled setting is set to
// "true" so endpoints are reachable.
func newTREnv(t *testing.T, withManager, enable bool) *trTestEnv {
	t.Helper()

	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	q := db.New(sqlDB)

	var mgr *trmqttsvc.Manager
	if withManager {
		mgr = trmqttsvc.NewManager(q, testEncryptionKey, nil)
	}

	r := gin.New()
	rl := auth.NewRateLimiter(context.Background())
	routes.RegisterRoutes(r, routes.Deps{
		Queries:       q,
		RateLimiter:   rl,
		Version:       "test",
		TRMqttManager: mgr,
		EncryptionKey: testEncryptionKey,
	})

	if enable {
		if err := q.UpsertSetting(context.Background(),
			db.UpsertSettingParams{Key: "trMqttEnabled", Value: "true"}); err != nil {
			t.Fatalf("UpsertSetting trMqttEnabled: %v", err)
		}
	}

	hash, err := auth.HashPassword("password1234")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
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

	return &trTestEnv{engine: r, queries: q, token: tok, manager: mgr}
}

func (e *trTestEnv) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(buf)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

// validCreate builds a valid create payload with a unique label.
func validCreate(label string) map[string]any {
	return map[string]any{
		"label":      label,
		"instanceId": "tr-1",
		"brokerUrl":  "mqtt://localhost:1883",
		"baseTopic":  "trunk_recorder",
		"username":   "user",
		"password":   "secret-pw",
		"qos":        1,
	}
}

// ---------- Disabled-feature gate ----------

func TestDisabled_WhenManagerNil_All404(t *testing.T) {
	env := newTREnv(t, false /*withManager*/, true /*enable setting*/)

	endpoints := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/admin/tr/instances"},
		{http.MethodPost, "/api/v1/admin/tr/instances"},
		{http.MethodPatch, "/api/v1/admin/tr/instances/1"},
		{http.MethodDelete, "/api/v1/admin/tr/instances/1"},
		{http.MethodPost, "/api/v1/admin/tr/instances/1/test"},
		{http.MethodPost, "/api/v1/admin/tr/instances/1/reconnect"},
		{http.MethodGet, "/api/v1/admin/tr/instances/1/snapshot"},
		// legacy path mirrors
		{http.MethodGet, "/api/admin/tr/instances"},
		{http.MethodPost, "/api/admin/tr/instances/1/reconnect"},
	}
	for _, ep := range endpoints {
		ep := ep
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := env.do(t, ep.method, ep.path, nil)
			if w.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404; body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestDisabled_WhenSettingFalse_All404(t *testing.T) {
	env := newTREnv(t, true /*withManager*/, false /*disable setting*/)

	w := env.do(t, http.MethodGet, "/api/v1/admin/tr/instances", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

// ---------- Create ----------

func TestCreateInstance_Success_PasswordEncryptedAndNotEchoed(t *testing.T) {
	env := newTREnv(t, true, true)

	w := env.do(t, http.MethodPost, "/api/v1/admin/tr/instances", validCreate("Studio A"))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if bytes.Contains(w.Body.Bytes(), []byte("secret-pw")) {
		t.Fatalf("response leaked plaintext password: %s", body)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("password_enc")) ||
		bytes.Contains(w.Body.Bytes(), []byte("passwordEnc")) {
		t.Fatalf("response leaked password_enc field: %s", body)
	}

	var resp struct {
		ID          int64  `json:"id"`
		Label       string `json:"label"`
		HasPassword bool   `json:"hasPassword"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, body)
	}
	if !resp.HasPassword {
		t.Errorf("hasPassword = false, want true")
	}
	if resp.Label != "Studio A" {
		t.Errorf("label = %q, want %q", resp.Label, "Studio A")
	}

	// Verify the row in DB has an encrypted password.
	row, err := env.queries.GetTRInstance(context.Background(), resp.ID)
	if err != nil {
		t.Fatalf("GetTRInstance: %v", err)
	}
	if !row.PasswordEnc.Valid || !auth.IsEncrypted(row.PasswordEnc.String) {
		t.Fatalf("password not encrypted at rest: valid=%v value=%q",
			row.PasswordEnc.Valid, row.PasswordEnc.String)
	}
	if row.PasswordEnc.String == "secret-pw" {
		t.Fatal("password stored as plaintext")
	}

	// Subsequent GET shows hasPassword=true and never echoes ciphertext.
	w2 := env.do(t, http.MethodGet, "/api/v1/admin/tr/instances", nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("list status = %d; body: %s", w2.Code, w2.Body.String())
	}
	if bytes.Contains(w2.Body.Bytes(), []byte("secret-pw")) ||
		bytes.Contains(w2.Body.Bytes(), []byte(row.PasswordEnc.String)) {
		t.Fatalf("list leaked password material: %s", w2.Body.String())
	}
}

func TestCreateInstance_RejectsInvalidScheme(t *testing.T) {
	env := newTREnv(t, true, true)
	payload := validCreate("Studio Bad")
	payload["brokerUrl"] = "http://localhost:1883"

	w := env.do(t, http.MethodPost, "/api/v1/admin/tr/instances", payload)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestCreateInstance_RejectsDuplicateLabel(t *testing.T) {
	env := newTREnv(t, true, true)

	w := env.do(t, http.MethodPost, "/api/v1/admin/tr/instances", validCreate("Studio Dup"))
	if w.Code != http.StatusCreated {
		t.Fatalf("first create status = %d; body: %s", w.Code, w.Body.String())
	}
	w2 := env.do(t, http.MethodPost, "/api/v1/admin/tr/instances", validCreate("Studio Dup"))
	if w2.Code != http.StatusConflict {
		t.Fatalf("second create status = %d, want 409; body: %s", w2.Code, w2.Body.String())
	}
}

// ---------- Patch ----------

func createInstance(t *testing.T, env *trTestEnv, label string) int64 {
	t.Helper()
	w := env.do(t, http.MethodPost, "/api/v1/admin/tr/instances", validCreate(label))
	if w.Code != http.StatusCreated {
		t.Fatalf("create %q: status %d; body: %s", label, w.Code, w.Body.String())
	}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	return resp.ID
}

func TestUpdateInstance_OmittedPassword_KeepsExisting(t *testing.T) {
	env := newTREnv(t, true, true)
	id := createInstance(t, env, "Studio Patch1")

	before, err := env.queries.GetTRInstance(context.Background(), id)
	if err != nil {
		t.Fatalf("GetTRInstance: %v", err)
	}
	if !before.PasswordEnc.Valid {
		t.Fatal("expected password set on create")
	}

	// PATCH without password field.
	w := env.do(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tr/instances/%d", id),
		map[string]any{"qos": 2})
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d; body: %s", w.Code, w.Body.String())
	}

	after, err := env.queries.GetTRInstance(context.Background(), id)
	if err != nil {
		t.Fatalf("GetTRInstance after: %v", err)
	}
	if after.PasswordEnc.String != before.PasswordEnc.String {
		t.Fatalf("password_enc changed; before=%q after=%q",
			before.PasswordEnc.String, after.PasswordEnc.String)
	}
	if after.Qos != 2 {
		t.Fatalf("qos = %d, want 2", after.Qos)
	}
}

func TestUpdateInstance_EmptyPassword_Clears(t *testing.T) {
	env := newTREnv(t, true, true)
	id := createInstance(t, env, "Studio Patch2")

	w := env.do(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tr/instances/%d", id),
		map[string]any{"password": ""})
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d; body: %s", w.Code, w.Body.String())
	}

	row, err := env.queries.GetTRInstance(context.Background(), id)
	if err != nil {
		t.Fatalf("GetTRInstance: %v", err)
	}
	if row.PasswordEnc.Valid && row.PasswordEnc.String != "" {
		t.Fatalf("expected password cleared; got valid=%v value=%q",
			row.PasswordEnc.Valid, row.PasswordEnc.String)
	}

	var resp struct {
		HasPassword bool `json:"hasPassword"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.HasPassword {
		t.Errorf("hasPassword = true, want false after clear")
	}
}

func TestUpdateInstance_NewPassword_ReEncrypts(t *testing.T) {
	env := newTREnv(t, true, true)
	id := createInstance(t, env, "Studio Patch3")

	before, err := env.queries.GetTRInstance(context.Background(), id)
	if err != nil {
		t.Fatalf("GetTRInstance: %v", err)
	}

	w := env.do(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tr/instances/%d", id),
		map[string]any{"password": "new-pw"})
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d; body: %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("new-pw")) {
		t.Fatalf("response leaked plaintext password: %s", w.Body.String())
	}

	after, err := env.queries.GetTRInstance(context.Background(), id)
	if err != nil {
		t.Fatalf("GetTRInstance after: %v", err)
	}
	if !auth.IsEncrypted(after.PasswordEnc.String) {
		t.Fatalf("password not encrypted at rest: %q", after.PasswordEnc.String)
	}
	if after.PasswordEnc.String == before.PasswordEnc.String {
		t.Fatal("password ciphertext unchanged after PATCH with new password")
	}
	plain, err := auth.DecryptString(after.PasswordEnc.String, testEncryptionKey)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if plain != "new-pw" {
		t.Errorf("decrypted = %q, want %q", plain, "new-pw")
	}
}

// ---------- Delete ----------

func TestDeleteInstance_RemovesRow(t *testing.T) {
	env := newTREnv(t, true, true)
	id := createInstance(t, env, "Studio Del")

	w := env.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/tr/instances/%d", id), nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d; body: %s", w.Code, w.Body.String())
	}

	w2 := env.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/tr/instances/%d", id), nil)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404; body: %s", w2.Code, w2.Body.String())
	}
}

// ---------- Reconnect / Snapshot edge cases ----------

func TestReconnect_UnknownID_404(t *testing.T) {
	env := newTREnv(t, true, true)

	w := env.do(t, http.MethodPost, "/api/v1/admin/tr/instances/9999/reconnect", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

func TestSnapshot_UnknownID_404(t *testing.T) {
	env := newTREnv(t, true, true)

	w := env.do(t, http.MethodGet, "/api/v1/admin/tr/instances/9999/snapshot", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}
