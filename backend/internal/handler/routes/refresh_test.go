package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"io"
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// login drives a POST /api/auth/login and returns the response recorder. On
// success the refresh token cookie will be present on the recorder.
func login(t *testing.T, engine http.Handler, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login %q status = %d, want 200; body: %s", username, w.Code, w.Body.String())
	}
	return w
}

// countActiveFamilies returns the number of active refresh token families for
// the given user (revoked=0 and not expired).
func countActiveFamilies(t *testing.T, queries *db.Queries, userID int64) int64 {
	t.Helper()
	n, err := queries.CountActiveRefreshTokenFamilies(context.Background(), db.CountActiveRefreshTokenFamiliesParams{
		UserID:    userID,
		ExpiresAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("CountActiveRefreshTokenFamilies: %v", err)
	}
	return n
}

// TestRefreshToken_FamilyCap_EvictsOldest verifies that after MaxRefreshFamilies+1
// logins by the same user, the oldest family is revoked and exactly 5 active
// families remain.
func TestRefreshToken_FamilyCap_EvictsOldest(t *testing.T) {
	engine, queries := newTestEngine(t)
	userID := seedAdminUser(t, queries, "alice", "password123")

	// Issue MaxRefreshFamilies (=5) logins — all should be active.
	for i := 0; i < auth.MaxRefreshFamilies; i++ {
		login(t, engine, "alice", "password123")
	}
	if got, want := countActiveFamilies(t, queries, userID), int64(auth.MaxRefreshFamilies); got != want {
		t.Fatalf("after %d logins: active families = %d, want %d", auth.MaxRefreshFamilies, got, want)
	}

	// Capture the oldest family id — it is the one that should be evicted.
	oldestFamily, err := queries.GetOldestActiveRefreshTokenFamily(context.Background(), db.GetOldestActiveRefreshTokenFamilyParams{
		UserID:    userID,
		ExpiresAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("GetOldestActiveRefreshTokenFamily: %v", err)
	}

	// 6th login — should evict the oldest family, leaving exactly 5 active.
	login(t, engine, "alice", "password123")

	if got, want := countActiveFamilies(t, queries, userID), int64(auth.MaxRefreshFamilies); got != want {
		t.Errorf("after cap+1 logins: active families = %d, want %d", got, want)
	}

	// The oldest family should no longer be returned by GetOldestActiveRefreshTokenFamily.
	newOldest, err := queries.GetOldestActiveRefreshTokenFamily(context.Background(), db.GetOldestActiveRefreshTokenFamilyParams{
		UserID:    userID,
		ExpiresAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("GetOldestActiveRefreshTokenFamily (post): %v", err)
	}
	if newOldest == oldestFamily {
		t.Errorf("oldest family %q was NOT evicted after cap+1 logins", oldestFamily)
	}
}

// TestRefreshToken_ReplayWithinGrace_Idempotent verifies that presenting the
// same already-rotated refresh token a second time within the grace window
// (e.g. parallel tab, service-worker retry, reload mid-rotation) returns the
// cached successor tokens and does NOT revoke the family. This is the
// "small leeway period" mandated by the OAuth 2.0 Security BCP §4.13.
func TestRefreshToken_ReplayWithinGrace_Idempotent(t *testing.T) {
	engine, queries := newTestEngine(t)
	userID := seedAdminUser(t, queries, "alice", "password123")

	// Login and capture the refresh cookie.
	w := login(t, engine, "alice", "password123")
	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.RefreshCookieName {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatalf("refresh cookie not set on login; got cookies: %+v", cookies)
	}

	// First refresh — should rotate successfully.
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req1.AddCookie(refreshCookie)
	w1 := httptest.NewRecorder()
	engine.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first refresh status = %d, want 200; body: %s", w1.Code, w1.Body.String())
	}

	// Second refresh using the ORIGINAL (now-rotated) cookie — within the
	// grace window. Must succeed and return identical tokens.
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req2.AddCookie(refreshCookie)
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("replay within grace status = %d, want 200; body: %s", w2.Code, w2.Body.String())
	}

	// Both responses should yield the same access JWT and refresh cookie.
	type respBody struct {
		Token string `json:"token"`
	}
	var b1, b2 respBody
	_ = json.Unmarshal(w1.Body.Bytes(), &b1)
	_ = json.Unmarshal(w2.Body.Bytes(), &b2)
	if b1.Token == "" || b1.Token != b2.Token {
		t.Errorf("expected identical access JWTs from rotation and grace-replay; got %q vs %q", b1.Token, b2.Token)
	}
	cookieValue := func(rec *httptest.ResponseRecorder) string {
		for _, c := range rec.Result().Cookies() {
			if c.Name == auth.RefreshCookieName {
				return c.Value
			}
		}
		return ""
	}
	if v1, v2 := cookieValue(w1), cookieValue(w2); v1 == "" || v1 != v2 {
		t.Errorf("expected identical refresh cookie from rotation and grace-replay; got %q vs %q", v1, v2)
	}

	// Family must remain active.
	if got := countActiveFamilies(t, queries, userID); got != 1 {
		t.Errorf("after grace-replay, active families = %d, want 1", got)
	}
}

// TestRefreshToken_ReplayAfterGrace_RevokesFamily verifies that a refresh
// token presented after the cached successor entry has expired (or never
// existed — e.g. server restart between rotation and replay) is treated as
// theft and revokes the entire token family.
func TestRefreshToken_ReplayAfterGrace_RevokesFamily(t *testing.T) {
	engine, queries := newTestEngine(t)
	userID := seedAdminUser(t, queries, "alice", "password123")

	w := login(t, engine, "alice", "password123")
	var refreshCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.RefreshCookieName {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatalf("refresh cookie not set on login")
	}

	// Manually mark the token row as revoked WITHOUT going through PostRefresh
	// — this simulates "rotated long ago, cache entry has expired".
	tokenHash := auth.HashRefreshToken(refreshCookie.Value)
	rt, err := queries.GetRefreshTokenByHash(context.Background(), tokenHash)
	if err != nil {
		t.Fatalf("GetRefreshTokenByHash: %v", err)
	}
	if err := queries.RevokeRefreshToken(context.Background(), rt.ID); err != nil {
		t.Fatalf("RevokeRefreshToken: %v", err)
	}

	// Replay the (now-revoked, not-in-cache) token. Must 401 and revoke family.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(refreshCookie)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("post-grace replay status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}
	if got := countActiveFamilies(t, queries, userID); got != 0 {
		t.Errorf("after post-grace replay, active families = %d, want 0", got)
	}
}

// TestRefreshToken_SeparateUsersIndependent confirms that user A hitting the
// family cap does not affect user B's refresh tokens.
func TestRefreshToken_SeparateUsersIndependent(t *testing.T) {
	engine, queries := newTestEngine(t)
	userA := seedAdminUser(t, queries, "alice", "password123")
	// Seed a second user via direct DB insert (cannot call SetSetupComplete twice).
	hash, err := auth.HashPassword("hunter2xyz")
	if err != nil {
		t.Fatalf("hash pw: %v", err)
	}
	now := time.Now().Unix()
	userB, err := queries.CreateUser(context.Background(), db.CreateUserParams{
		Username: "bob", PasswordHash: hash, Role: auth.RoleListener,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create user B: %v", err)
	}

	// User A: hit the cap exactly.
	for i := 0; i < auth.MaxRefreshFamilies; i++ {
		login(t, engine, "alice", "password123")
	}
	if got := countActiveFamilies(t, queries, userA); got != int64(auth.MaxRefreshFamilies) {
		t.Fatalf("user A active families = %d, want %d", got, auth.MaxRefreshFamilies)
	}

	// User B logs in once — should not touch user A.
	login(t, engine, "bob", "hunter2xyz")

	if got := countActiveFamilies(t, queries, userA); got != int64(auth.MaxRefreshFamilies) {
		t.Errorf("user A families after user B login = %d, want %d", got, auth.MaxRefreshFamilies)
	}
	if got := countActiveFamilies(t, queries, userB); got != 1 {
		t.Errorf("user B families = %d, want 1", got)
	}
}

// TestRefreshCookie_DeliveredToEveryRefreshEndpoint is an end-to-end
// regression test for the 1.3.0 cookie-path bug: the refresh cookie was
// scoped to /api/auth, which per RFC 6265 §5.1.4 path-matching does NOT
// cover /api/v1/auth/refresh. Browsers silently dropped the cookie on the
// v1 refresh endpoint and every silent-refresh attempt 401'd, bouncing the
// user to the login screen ~15 minutes after login.
//
// The previous tests used httptest.NewRequest + req.AddCookie(...) which
// force-attaches the cookie regardless of path-matching, so they could
// never have caught this. This test uses a real http.Client + cookiejar
// against httptest.NewServer so the standard library enforces RFC 6265
// path-matching exactly the way a browser does.
func TestRefreshCookie_DeliveredToEveryRefreshEndpoint(t *testing.T) {
	engine, queries := newTestEngine(t)
	seedAdminUser(t, queries, "alice", "password123")

	srv := httptest.NewServer(engine)
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	client := &http.Client{Jar: jar}

	// Log in via the v1 endpoint so the cookie jar captures the
	// Set-Cookie header exactly as a browser would.
	loginBody, _ := json.Marshal(map[string]string{"username": "alice", "password": "password123"})
	loginResp, err := client.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login POST: %v", err)
	}
	_ = loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.StatusCode)
	}

	// Sanity: jar must hold the refresh cookie. Query at the refresh
	// endpoint path (not "/") because the cookie's declared Path scopes
	// it under /api — RFC 6265 path-matching means jar.Cookies("/")
	// would (correctly) hide it, while a query at /api/v1/auth/refresh
	// must return it for the cookie to be delivered there.
	probeURL, _ := url.Parse(srv.URL + "/api/v1/auth/refresh")
	var refreshCookieFound bool
	for _, c := range jar.Cookies(probeURL) {
		if c.Name == auth.RefreshCookieName {
			refreshCookieFound = true
			break
		}
	}
	if !refreshCookieFound {
		t.Fatalf("refresh cookie not delivered to /api/v1/auth/refresh (Set-Cookie headers: %v)", loginResp.Header.Values("Set-Cookie"))
	}

	// For every refresh endpoint URL the frontend can hit, posting an
	// empty body MUST result in the cookie being delivered (i.e. the
	// server sees it and returns 200, not 401 "no refresh token"). Each
	// refresh rotates the cookie; the jar captures the new value on the
	// response so subsequent calls in the loop keep working.
	endpoints := []string{
		"/api/auth/refresh",    // legacy
		"/api/v1/auth/refresh", // v1 (the path that broke in 1.3.0)
	}
	for _, ep := range endpoints {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+ep, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", ep, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("POST %s status = %d, want 200 (cookie not delivered? body: %s)", ep, resp.StatusCode, string(body))
		}
	}
}
