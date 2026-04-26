package auth_test

// Phase N-0 — legacy contract freeze for the auth handler package (REST endpoints).
//
// Pins today's wire format for /api/auth/login, /api/auth/refresh,
// /api/auth/logout, /api/auth/password, /api/auth/me, /api/auth/tg-selection
// (GET + PUT) and the os_session cookie issuance / clearance behaviour the
// frontend relies on. Plan reference:
// docs/plans/native-api-design-plan.md §4.1 (auth row group).

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
	pkgauth "github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/handler/routes"
	"github.com/openscanner/openscanner/internal/logging"
)

func init() {
	gin.SetMode(gin.TestMode)
	logging.Configure(true, "")
}

func authFixture(t *testing.T) (*gin.Engine, *db.Queries) {
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
	rl := pkgauth.NewRateLimiter(context.Background())
	routes.RegisterRoutes(r, routes.Deps{Queries: q, RateLimiter: rl, Processor: proc, Version: "test"})

	hash, _ := pkgauth.HashPassword("password123")
	now := time.Now().Unix()
	if _, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username: "alice", PasswordHash: hash, Role: pkgauth.RoleAdmin,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return r, q
}

func loginAlice(t *testing.T, engine http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	return w
}

func cookieByName(cs []*http.Cookie, name string) *http.Cookie {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestPostLogin_LegacyResponseShape pins the JSON envelope of POST
// /api/auth/login: {token, user{id,username,role}, passwordNeedChange} and
// the os_session + refresh_token cookies.
func TestPostLogin_LegacyResponseShape(t *testing.T) {
	engine, _ := authFixture(t)
	w := loginAlice(t, engine)

	var resp struct {
		Token string `json:"token"`
		User  struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"user"`
		PasswordNeedChange bool `json:"passwordNeedChange"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, w.Body.String())
	}
	if resp.Token == "" {
		t.Error("token: empty")
	}
	if resp.User.Username != "alice" || resp.User.Role != pkgauth.RoleAdmin || resp.User.ID <= 0 {
		t.Errorf("user = %+v, want {id>0,alice,admin}", resp.User)
	}

	cookies := w.Result().Cookies()
	if c := cookieByName(cookies, pkgauth.SessionCookieName); c == nil || c.Value != resp.Token {
		t.Errorf("os_session cookie: %v, want value=%q", c, resp.Token)
	}
	if c := cookieByName(cookies, pkgauth.RefreshCookieName); c == nil || c.Value == "" {
		t.Errorf("refresh_token cookie missing or empty: %v", c)
	}
}

// TestPostLogin_InvalidCreds_LegacyShape pins the 401 response body.
func TestPostLogin_InvalidCreds_LegacyShape(t *testing.T) {
	engine, _ := authFixture(t)
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "WRONG"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	var env map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := env["error"]; !ok {
		t.Errorf("missing 'error' key in 401 body: %s", w.Body.String())
	}
}

// TestPostRefresh_LegacyResponseShape pins {token, user{...}} and rotated
// cookies on POST /api/auth/refresh.
func TestPostRefresh_LegacyResponseShape(t *testing.T) {
	engine, _ := authFixture(t)
	loginW := loginAlice(t, engine)
	refreshCk := cookieByName(loginW.Result().Cookies(), pkgauth.RefreshCookieName)
	if refreshCk == nil {
		t.Fatal("login did not set refresh_token cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(refreshCk)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
		User  struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" || resp.User.Username != "alice" {
		t.Errorf("refresh resp = %+v, want token!=\"\" + user.alice", resp)
	}
	if c := cookieByName(w.Result().Cookies(), pkgauth.RefreshCookieName); c == nil || c.Value == "" {
		t.Error("refresh: did not rotate refresh_token cookie")
	}
	if c := cookieByName(w.Result().Cookies(), pkgauth.SessionCookieName); c == nil || c.Value == "" {
		t.Error("refresh: did not rotate os_session cookie")
	}
}

// TestGetMe_LegacyShape pins {id, username, role} on GET /api/auth/me.
func TestGetMe_LegacyShape(t *testing.T) {
	engine, _ := authFixture(t)
	loginW := loginAlice(t, engine)
	var l struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(loginW.Body.Bytes(), &l)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+l.Token)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Username != "alice" || resp.Role != pkgauth.RoleAdmin || resp.ID <= 0 {
		t.Errorf("me = %+v, want id>0,alice,admin", resp)
	}
}

// TestPostLogout_LegacyShape pins the {ok:true} body and that both auth
// cookies are cleared (Max-Age <= 0, empty value).
func TestPostLogout_LegacyShape(t *testing.T) {
	engine, _ := authFixture(t)
	loginW := loginAlice(t, engine)
	var l struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(loginW.Body.Bytes(), &l)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+l.Token)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("body = %v, want ok:true", resp)
	}
	for _, name := range []string{pkgauth.SessionCookieName, pkgauth.RefreshCookieName} {
		c := cookieByName(w.Result().Cookies(), name)
		if c == nil {
			t.Errorf("%s: cookie not cleared (missing Set-Cookie)", name)
			continue
		}
		if c.Value != "" || c.MaxAge > 0 {
			t.Errorf("%s: not cleared, value=%q maxAge=%d", name, c.Value, c.MaxAge)
		}
	}
}

// TestPutPassword_LegacyShape pins {ok:true} on PUT /api/auth/password.
func TestPutPassword_LegacyShape(t *testing.T) {
	engine, _ := authFixture(t)
	loginW := loginAlice(t, engine)
	var l struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(loginW.Body.Bytes(), &l)

	body, _ := json.Marshal(map[string]string{
		"currentPassword": "password123",
		"newPassword":     "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.Token)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("body = %v, want ok:true", resp)
	}
}

// TestTGSelection_LegacyRoundTrip pins the GET / PUT /api/auth/tg-selection
// envelope: {disabledTGs:int64[], avoidList:[{talkgroupId,expiresAt}]}.
func TestTGSelection_LegacyRoundTrip(t *testing.T) {
	engine, _ := authFixture(t)
	loginW := loginAlice(t, engine)
	var l struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(loginW.Body.Bytes(), &l)
	bearer := "Bearer " + l.Token

	// GET — empty default.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/tg-selection", nil)
	req.Header.Set("Authorization", bearer)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET: status = %d, want 200", w.Code)
	}
	var initial struct {
		DisabledTGs []int64          `json:"disabledTGs"`
		AvoidList   []map[string]any `json:"avoidList"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &initial); err != nil {
		t.Fatalf("GET decode: %v\nbody: %s", err, w.Body.String())
	}
	if initial.DisabledTGs == nil || initial.AvoidList == nil {
		t.Errorf("GET defaults must be empty arrays not null: got %+v", initial)
	}

	// PUT — round-trip.
	put, _ := json.Marshal(map[string]any{
		"disabledTGs": []int64{1, 2, 3},
		"avoidList": []map[string]any{
			{"talkgroupId": 99, "expiresAt": int64(1700000000)},
		},
	})
	req = httptest.NewRequest(http.MethodPut, "/api/auth/tg-selection", bytes.NewReader(put))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearer)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var ok map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ok); err != nil {
		t.Fatalf("PUT decode: %v", err)
	}
	if v, _ := ok["ok"].(bool); !v {
		t.Errorf("PUT body = %v, want ok:true", ok)
	}

	// GET again — verify persistence.
	req = httptest.NewRequest(http.MethodGet, "/api/auth/tg-selection", nil)
	req.Header.Set("Authorization", bearer)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	var final struct {
		DisabledTGs []int64 `json:"disabledTGs"`
		AvoidList   []struct {
			TalkgroupID int64 `json:"talkgroupId"`
			ExpiresAt   int64 `json:"expiresAt"`
		} `json:"avoidList"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &final); err != nil {
		t.Fatalf("GET2 decode: %v", err)
	}
	if len(final.DisabledTGs) != 3 || final.DisabledTGs[0] != 1 {
		t.Errorf("disabledTGs = %v, want [1 2 3]", final.DisabledTGs)
	}
	if len(final.AvoidList) != 1 || final.AvoidList[0].TalkgroupID != 99 || final.AvoidList[0].ExpiresAt != 1700000000 {
		t.Errorf("avoidList = %+v, want [{99,1700000000}]", final.AvoidList)
	}
}
