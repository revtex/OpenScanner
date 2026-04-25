package routes_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// seedListenerUser creates a listener user with the given systems_json grants.
// Pass grantsJSON == "" to leave grants NULL (all-access).
func seedListenerUser(t *testing.T, queries *db.Queries, username, grantsJSON string) int64 {
	t.Helper()
	hash, err := auth.HashPassword("listenerpass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now().Unix()
	var systems sql.NullString
	if grantsJSON != "" {
		systems = sql.NullString{String: grantsJSON, Valid: true}
	}
	id, err := queries.CreateUser(context.Background(), db.CreateUserParams{
		Username:     username,
		PasswordHash: hash,
		Role:         auth.RoleListener,
		Disabled:     0,
		SystemsJson:  systems,
		Expiration:   sql.NullInt64{},
		Limit:        sql.NullInt64{},
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create listener user: %v", err)
	}
	return id
}

// seedCall inserts a minimal call row attached to systemPK and returns its PK.
func seedCall(t *testing.T, queries *db.Queries, systemPK int64) int64 {
	t.Helper()
	id, err := queries.CreateCall(context.Background(), db.CreateCallParams{
		AudioPath: fmt.Sprintf("/tmp/%d.mp3", time.Now().UnixNano()),
		AudioName: "call.mp3",
		AudioType: "audio/mpeg",
		DateTime:  time.Now().Unix(),
		SystemID:  systemPK,
	})
	if err != nil {
		t.Fatalf("create call: %v", err)
	}
	return id
}

// bookmarkTogglePayload marshals the request body for POST /api/bookmarks.
func bookmarkTogglePayload(t *testing.T, callID int64) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]int64{"callId": callID})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func TestBookmarks_ListenerOffGrant_Returns404(t *testing.T) {
	engine, queries := newTestEngine(t)

	sys1 := seedSystem(t, queries, 1, "sys-one")
	sys2 := seedSystem(t, queries, 2, "sys-two")
	call := seedCall(t, queries, sys2) // call lives on system 2

	grants := fmt.Sprintf(`[{"id":%d}]`, sys1) // grants only cover system 1
	uid := seedListenerUser(t, queries, "limited", grants)
	token := listenerToken(t, uid, "limited")

	w := doRequest(engine, http.MethodPost, "/api/bookmarks",
		bookmarkTogglePayload(t, call),
		map[string]string{"Authorization": "Bearer " + token},
	)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}

	// Confirm no bookmark was created despite the POST.
	_, err := queries.GetBookmarkByCallAndUser(context.Background(), db.GetBookmarkByCallAndUserParams{
		CallID: call,
		UserID: sql.NullInt64{Int64: uid, Valid: true},
	})
	if err == nil {
		t.Fatal("bookmark row unexpectedly exists")
	}
}

func TestBookmarks_ListenerInGrant_Returns200(t *testing.T) {
	engine, queries := newTestEngine(t)

	sys1 := seedSystem(t, queries, 1, "sys-one")
	_ = seedSystem(t, queries, 2, "sys-two")
	call := seedCall(t, queries, sys1)

	grants := fmt.Sprintf(`[{"id":%d}]`, sys1)
	uid := seedListenerUser(t, queries, "granted", grants)
	token := listenerToken(t, uid, "granted")

	w := doRequest(engine, http.MethodPost, "/api/bookmarks",
		bookmarkTogglePayload(t, call),
		map[string]string{"Authorization": "Bearer " + token},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	row, err := queries.GetBookmarkByCallAndUser(context.Background(), db.GetBookmarkByCallAndUserParams{
		CallID: call,
		UserID: sql.NullInt64{Int64: uid, Valid: true},
	})
	if err != nil {
		t.Fatalf("expected bookmark row, got err: %v", err)
	}
	if row.CallID != call {
		t.Errorf("row.CallID = %d, want %d", row.CallID, call)
	}
}

func TestBookmarks_AdminBypassesGrants(t *testing.T) {
	engine, queries := newTestEngine(t)

	_ = seedSystem(t, queries, 1, "sys-one")
	sys2 := seedSystem(t, queries, 2, "sys-two")
	call := seedCall(t, queries, sys2)

	// Admin has no grants set (would be all-access anyway); PR is that admins
	// bypass the grant check entirely.
	uid := seedAdminUser(t, queries, "admin-u", "adminpass")
	token := adminToken(t, uid, "admin-u")

	w := doRequest(engine, http.MethodPost, "/api/bookmarks",
		bookmarkTogglePayload(t, call),
		map[string]string{"Authorization": "Bearer " + token},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestBookmarks_NoGrantsMeansAll(t *testing.T) {
	engine, queries := newTestEngine(t)

	_ = seedSystem(t, queries, 1, "sys-one")
	sys2 := seedSystem(t, queries, 2, "sys-two")
	call := seedCall(t, queries, sys2)

	// Listener with NULL systems_json — auth.HasSystemAccess returns true for
	// empty grants (documented "allow all" behaviour).
	uid := seedListenerUser(t, queries, "unrestricted", "")
	token := listenerToken(t, uid, "unrestricted")

	w := doRequest(engine, http.MethodPost, "/api/bookmarks",
		bookmarkTogglePayload(t, call),
		map[string]string{"Authorization": "Bearer " + token},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	// Cross-check: toggle again should delete, returning bookmarked:false.
	w2 := doRequest(engine, http.MethodPost, "/api/bookmarks",
		bookmarkTogglePayload(t, call),
		map[string]string{"Authorization": "Bearer " + token},
	)
	if w2.Code != http.StatusOK {
		t.Fatalf("second toggle status = %d, want 200; body=%s", w2.Code, w2.Body.String())
	}
	var resp struct {
		Bookmarked bool `json:"bookmarked"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Bookmarked {
		t.Error("expected bookmarked=false after second toggle")
	}

	// Silence unused-import warnings if helpers change.
	_ = bytes.NewReader
	_ = httptest.NewRequest
}
