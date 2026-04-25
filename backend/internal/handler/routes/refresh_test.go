package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// TestRefreshToken_Rotate_ReuseDetected verifies the rotation-reuse defence:
// using a refresh token that was already rotated revokes the entire family.
func TestRefreshToken_Rotate_ReuseDetected(t *testing.T) {
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

	// Second refresh using the ORIGINAL (now-revoked) cookie — replay attack.
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req2.AddCookie(refreshCookie)
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("replay refresh status = %d, want 401; body: %s", w2.Code, w2.Body.String())
	}

	// The entire family should now be revoked → zero active families.
	if got := countActiveFamilies(t, queries, userID); got != 0 {
		t.Errorf("after replay detection, active families = %d, want 0", got)
	}

	// And a subsequent refresh with the rotated cookie (which is still part of
	// that family) should also be rejected.
	rotated := w1.Result().Cookies()
	var rotatedCookie *http.Cookie
	for _, c := range rotated {
		if c.Name == auth.RefreshCookieName {
			rotatedCookie = c
			break
		}
	}
	if rotatedCookie != nil {
		req3 := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
		req3.AddCookie(rotatedCookie)
		w3 := httptest.NewRecorder()
		engine.ServeHTTP(w3, req3)
		if w3.Code != http.StatusUnauthorized {
			t.Errorf("refresh with revoked-family cookie: status = %d, want 401", w3.Code)
		}
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
