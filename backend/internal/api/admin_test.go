package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/api"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/ws"
)

// ---------- helpers ----------

// newAdminTestEngine creates a Gin engine with Hub and SQLDB wired in, which
// the admin CRUD endpoints require for broadcasting and transactional imports.
func newAdminTestEngine(t *testing.T) (*gin.Engine, *db.Queries) {
	t.Helper()
	sqlDB, queries := newTestDB(t)

	hub := ws.NewHub(queries, "test")

	router := gin.New()
	rl := auth.NewRateLimiter(context.Background())
	api.RegisterRoutes(router, api.Deps{
		Queries:     queries,
		RateLimiter: rl,
		Hub:         hub,
		SQLDB:       sqlDB,
		Version:     "test",
	})
	return router, queries
}

// adminToken returns a signed JWT for an admin user.
func adminToken(t *testing.T, userID int64, username string) string {
	t.Helper()
	tok, _, err := auth.GenerateToken(userID, username, auth.RoleAdmin)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}
	return tok
}

// listenerToken returns a signed JWT for a listener user.
func listenerToken(t *testing.T, userID int64, username string) string {
	t.Helper()
	tok, _, err := auth.GenerateToken(userID, username, auth.RoleListener)
	if err != nil {
		t.Fatalf("generate listener token: %v", err)
	}
	return tok
}

// doRequest is a shorthand for building and serving an HTTP request.
func doRequest(engine *gin.Engine, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	var reqBody *bytes.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// jsonBody marshals v to JSON bytes, fataling on error.
func jsonBody(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return b
}

// seedSystem inserts a system and returns its row ID.
func seedSystem(t *testing.T, q *db.Queries, systemID int64, label string) int64 {
	t.Helper()
	id, err := q.CreateSystem(context.Background(), db.CreateSystemParams{
		SystemID: systemID,
		Label:    label,
	})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}
	return id
}

// decodeJSON decodes the recorder body into dest.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, dest any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(dest); err != nil {
		t.Fatalf("decode json: %v (body: %s)", err, w.Body.String())
	}
}

// ---------- 1. Auth enforcement ----------

func TestAdminEndpoints_NoJWT(t *testing.T) {
	engine, _ := newAdminTestEngine(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/admin/users"},
		{http.MethodPost, "/api/admin/users"},
		{http.MethodPut, "/api/admin/users/1"},
		{http.MethodDelete, "/api/admin/users/1"},
		{http.MethodGet, "/api/admin/systems"},
		{http.MethodPost, "/api/admin/systems"},
		{http.MethodGet, "/api/admin/talkgroups"},
		{http.MethodGet, "/api/admin/groups"},
		{http.MethodGet, "/api/admin/tags"},
		{http.MethodGet, "/api/admin/apikeys"},
		{http.MethodGet, "/api/admin/config"},
		{http.MethodPut, "/api/admin/config"},
		{http.MethodGet, "/api/admin/logs"},
		{http.MethodGet, "/api/admin/export/config"},
		{http.MethodPost, "/api/admin/import/config"},
		{http.MethodPost, "/api/admin/import/talkgroups"},
		{http.MethodPost, "/api/admin/import/units"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := doRequest(engine, ep.method, ep.path, nil, nil)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("got %d, want 401", w.Code)
			}
		})
	}
}

func TestAdminEndpoints_ListenerJWT(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	_ = seedAdminUser(t, queries, "admin1", "password1234")
	tok := listenerToken(t, 99, "listener1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/admin/users"},
		{http.MethodPost, "/api/admin/users"},
		{http.MethodGet, "/api/admin/systems"},
		{http.MethodGet, "/api/admin/config"},
		{http.MethodGet, "/api/admin/logs"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := doRequest(engine, ep.method, ep.path, nil, hdrs)
			if w.Code != http.StatusForbidden {
				t.Errorf("got %d, want 403", w.Code)
			}
		})
	}
}

// ---------- 2. Users CRUD ----------

func TestUsers_List(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	w := doRequest(engine, http.MethodGet, "/api/admin/users", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var users []db.User
	decodeJSON(t, w, &users)
	if len(users) != 1 {
		t.Errorf("len = %d, want 1", len(users))
	}
}

func TestUsers_Create(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{
		"username": "newuser",
		"password": "securepassword",
		"role":     "listener",
	})

	w := doRequest(engine, http.MethodPost, "/api/admin/users", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	var user map[string]any
	decodeJSON(t, w, &user)
	if user["username"] != "newuser" {
		t.Errorf("username = %v, want newuser", user["username"])
	}
	// Password hash should NOT be in the response (json:"-" tag).
	if _, ok := user["password_hash"]; ok {
		t.Error("password_hash should not be in response")
	}
}

func TestUsers_Create_EmptyUsername(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{
		"username": "",
		"password": "securepassword",
	})

	w := doRequest(engine, http.MethodPost, "/api/admin/users", body, hdrs)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422; body: %s", w.Code, w.Body.String())
	}
}

func TestUsers_Create_ShortPassword(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{
		"username": "shortpw",
		"password": "short",
	})

	w := doRequest(engine, http.MethodPost, "/api/admin/users", body, hdrs)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422; body: %s", w.Code, w.Body.String())
	}
}

func TestUsers_Create_DuplicateUsername(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{
		"username": "admin1",
		"password": "anotherpassword",
	})

	w := doRequest(engine, http.MethodPost, "/api/admin/users", body, hdrs)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

func TestUsers_Update(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Create a second user to update.
	body := jsonBody(t, map[string]any{
		"username": "user2",
		"password": "securepassword",
		"role":     "listener",
	})
	w := doRequest(engine, http.MethodPost, "/api/admin/users", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	newID := int64(created["id"].(float64))

	updateBody := jsonBody(t, map[string]any{
		"username": "user2_updated",
		"role":     "admin",
	})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/users/%d", newID), updateBody, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var updated map[string]any
	decodeJSON(t, w, &updated)
	if updated["username"] != "user2_updated" {
		t.Errorf("username = %v, want user2_updated", updated["username"])
	}
}

func TestUsers_Update_NotFound(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"username": "x", "role": "admin"})
	w := doRequest(engine, http.MethodPut, "/api/admin/users/99999", body, hdrs)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

func TestUsers_Delete(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Create a user to delete.
	body := jsonBody(t, map[string]any{
		"username": "todelete",
		"password": "securepassword",
	})
	w := doRequest(engine, http.MethodPost, "/api/admin/users", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d", w.Code)
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	deleteID := int64(created["id"].(float64))

	w = doRequest(engine, http.MethodDelete, fmt.Sprintf("/api/admin/users/%d", deleteID), nil, hdrs)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestUsers_Delete_Self(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	w := doRequest(engine, http.MethodDelete, fmt.Sprintf("/api/admin/users/%d", uid), nil, hdrs)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (cannot delete self); body: %s", w.Code, w.Body.String())
	}
}

// ---------- 3. Systems CRUD ----------

func TestSystems_CRUD(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Create
	body := jsonBody(t, map[string]any{
		"system_id": 100,
		"label":     "Test System",
	})
	w := doRequest(engine, http.MethodPost, "/api/admin/systems", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	sysID := int64(created["id"].(float64))
	if created["label"] != "Test System" {
		t.Errorf("label = %v, want Test System", created["label"])
	}

	// List
	w = doRequest(engine, http.MethodGet, "/api/admin/systems", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var systems []map[string]any
	decodeJSON(t, w, &systems)
	if len(systems) != 1 {
		t.Errorf("len = %d, want 1", len(systems))
	}

	// Update
	updateBody := jsonBody(t, map[string]any{
		"system_id": 100,
		"label":     "Updated System",
	})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/systems/%d", sysID), updateBody, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d; body: %s", w.Code, w.Body.String())
	}
	var updated map[string]any
	decodeJSON(t, w, &updated)
	if updated["label"] != "Updated System" {
		t.Errorf("label = %v, want Updated System", updated["label"])
	}

	// Delete
	w = doRequest(engine, http.MethodDelete, fmt.Sprintf("/api/admin/systems/%d", sysID), nil, hdrs)
	if w.Code != http.StatusOK {
		t.Errorf("delete status = %d; body: %s", w.Code, w.Body.String())
	}
}

func TestSystems_Create_DuplicateSystemID(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"system_id": 200, "label": "Sys A"})
	w := doRequest(engine, http.MethodPost, "/api/admin/systems", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create: status = %d", w.Code)
	}

	w = doRequest(engine, http.MethodPost, "/api/admin/systems", body, hdrs)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate create: status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

func TestSystems_Update_NotFound(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"system_id": 1, "label": "x"})
	w := doRequest(engine, http.MethodPut, "/api/admin/systems/99999", body, hdrs)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSystems_Delete_NotFound(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	w := doRequest(engine, http.MethodDelete, "/api/admin/systems/99999", nil, hdrs)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ---------- 4. Talkgroups CRUD ----------

func TestTalkgroups_CRUD(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Need a system first (FK).
	sysRowID := seedSystem(t, queries, 10, "TG Test System")

	// Create talkgroup
	body := jsonBody(t, map[string]any{
		"system_id":    sysRowID,
		"talkgroup_id": 500,
		"label":        map[string]any{"String": "Fire Dispatch", "Valid": true},
	})
	w := doRequest(engine, http.MethodPost, "/api/admin/talkgroups", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	tgID := int64(created["id"].(float64))

	// List
	w = doRequest(engine, http.MethodGet, "/api/admin/talkgroups", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var tgs []map[string]any
	decodeJSON(t, w, &tgs)
	if len(tgs) != 1 {
		t.Errorf("len = %d, want 1", len(tgs))
	}

	// Update
	updateBody := jsonBody(t, map[string]any{
		"system_id":    sysRowID,
		"talkgroup_id": 500,
		"label":        map[string]any{"String": "EMS Dispatch", "Valid": true},
	})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/talkgroups/%d", tgID), updateBody, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d; body: %s", w.Code, w.Body.String())
	}

	// Delete
	w = doRequest(engine, http.MethodDelete, fmt.Sprintf("/api/admin/talkgroups/%d", tgID), nil, hdrs)
	if w.Code != http.StatusOK {
		t.Errorf("delete status = %d; body: %s", w.Code, w.Body.String())
	}
}

func TestTalkgroups_Delete_NotFound(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	w := doRequest(engine, http.MethodDelete, "/api/admin/talkgroups/99999", nil, hdrs)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ---------- 5. Groups CRUD ----------

func TestGroups_CRUD(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Create
	body := jsonBody(t, map[string]any{"label": "Group Alpha"})
	w := doRequest(engine, http.MethodPost, "/api/admin/groups", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	grpID := int64(created["id"].(float64))
	if created["label"] != "Group Alpha" {
		t.Errorf("label = %v, want Group Alpha", created["label"])
	}

	// List
	w = doRequest(engine, http.MethodGet, "/api/admin/groups", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var groups []map[string]any
	decodeJSON(t, w, &groups)
	if len(groups) != 1 {
		t.Errorf("len = %d, want 1", len(groups))
	}

	// Update
	updateBody := jsonBody(t, map[string]any{"label": "Group Beta"})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/groups/%d", grpID), updateBody, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d; body: %s", w.Code, w.Body.String())
	}
	var updated map[string]any
	decodeJSON(t, w, &updated)
	if updated["label"] != "Group Beta" {
		t.Errorf("label = %v, want Group Beta", updated["label"])
	}

	// Delete
	w = doRequest(engine, http.MethodDelete, fmt.Sprintf("/api/admin/groups/%d", grpID), nil, hdrs)
	if w.Code != http.StatusOK {
		t.Errorf("delete status = %d; body: %s", w.Code, w.Body.String())
	}
}

func TestGroups_Create_DuplicateLabel(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"label": "Dup Group"})
	w := doRequest(engine, http.MethodPost, "/api/admin/groups", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create: status = %d", w.Code)
	}

	w = doRequest(engine, http.MethodPost, "/api/admin/groups", body, hdrs)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate: status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

func TestGroups_Create_EmptyLabel(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"label": ""})
	w := doRequest(engine, http.MethodPost, "/api/admin/groups", body, hdrs)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestGroups_Delete_NotFound(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	w := doRequest(engine, http.MethodDelete, "/api/admin/groups/99999", nil, hdrs)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ---------- 6. Tags CRUD ----------

func TestTags_CRUD(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Create
	body := jsonBody(t, map[string]any{"label": "Tag One"})
	w := doRequest(engine, http.MethodPost, "/api/admin/tags", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	tagID := int64(created["id"].(float64))

	// List
	w = doRequest(engine, http.MethodGet, "/api/admin/tags", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var tags []map[string]any
	decodeJSON(t, w, &tags)
	if len(tags) != 1 {
		t.Errorf("len = %d, want 1", len(tags))
	}

	// Update
	updateBody := jsonBody(t, map[string]any{"label": "Tag Two"})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/tags/%d", tagID), updateBody, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d; body: %s", w.Code, w.Body.String())
	}
	var updated map[string]any
	decodeJSON(t, w, &updated)
	if updated["label"] != "Tag Two" {
		t.Errorf("label = %v, want Tag Two", updated["label"])
	}

	// Delete
	w = doRequest(engine, http.MethodDelete, fmt.Sprintf("/api/admin/tags/%d", tagID), nil, hdrs)
	if w.Code != http.StatusOK {
		t.Errorf("delete status = %d; body: %s", w.Code, w.Body.String())
	}
}

func TestTags_Create_DuplicateLabel(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"label": "Dup Tag"})
	w := doRequest(engine, http.MethodPost, "/api/admin/tags", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create: status = %d", w.Code)
	}

	w = doRequest(engine, http.MethodPost, "/api/admin/tags", body, hdrs)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate: status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

func TestTags_Create_EmptyLabel(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"label": ""})
	w := doRequest(engine, http.MethodPost, "/api/admin/tags", body, hdrs)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestTags_Delete_NotFound(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	w := doRequest(engine, http.MethodDelete, "/api/admin/tags/99999", nil, hdrs)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ---------- 7. API Keys CRUD ----------

func TestAPIKeys_CRUD(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Create with auto-generated key (empty key field).
	body := jsonBody(t, map[string]any{"disabled": 0})
	w := doRequest(engine, http.MethodPost, "/api/admin/apikeys", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	keyID := int64(created["id"].(float64))
	generatedKey, _ := created["key"].(string)
	if generatedKey == "" {
		t.Error("auto-generated key should not be empty")
	}

	// List
	w = doRequest(engine, http.MethodGet, "/api/admin/apikeys", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var keys []map[string]any
	decodeJSON(t, w, &keys)
	if len(keys) != 1 {
		t.Errorf("len = %d, want 1", len(keys))
	}

	// Update — disable the key.
	updateBody := jsonBody(t, map[string]any{
		"key":      generatedKey,
		"disabled": 1,
	})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/apikeys/%d", keyID), updateBody, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d; body: %s", w.Code, w.Body.String())
	}

	// Delete
	w = doRequest(engine, http.MethodDelete, fmt.Sprintf("/api/admin/apikeys/%d", keyID), nil, hdrs)
	if w.Code != http.StatusOK {
		t.Errorf("delete status = %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAPIKeys_Create_ExplicitKey(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"key": "my-explicit-key", "disabled": 0})
	w := doRequest(engine, http.MethodPost, "/api/admin/apikeys", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	if created["key"] != "my-explicit-key" {
		t.Errorf("key = %v, want my-explicit-key", created["key"])
	}
}

func TestAPIKeys_Delete_NotFound(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	w := doRequest(engine, http.MethodDelete, "/api/admin/apikeys/99999", nil, hdrs)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ---------- 8. Config ----------

func TestConfig_GetPut(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Seed a setting.
	_ = queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "audioConversion", Value: "0",
	})

	// GET
	w := doRequest(engine, http.MethodGet, "/api/admin/config", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d; body: %s", w.Code, w.Body.String())
	}
	var config map[string]string
	decodeJSON(t, w, &config)
	if config["audioConversion"] != "0" {
		t.Errorf("audioConversion = %q, want 0", config["audioConversion"])
	}

	// PUT
	putBody := jsonBody(t, map[string]string{
		"audioConversion": "1",
		"branding":        "hello",
	})
	w = doRequest(engine, http.MethodPut, "/api/admin/config", putBody, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d; body: %s", w.Code, w.Body.String())
	}

	// Verify via GET.
	w = doRequest(engine, http.MethodGet, "/api/admin/config", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after PUT status = %d", w.Code)
	}
	var config2 map[string]string
	decodeJSON(t, w, &config2)
	if config2["audioConversion"] != "1" {
		t.Errorf("audioConversion after PUT = %q, want 1", config2["audioConversion"])
	}
	if config2["branding"] != "hello" {
		t.Errorf("branding = %q, want hello", config2["branding"])
	}

	// PUT with unknown key should be rejected.
	badBody := jsonBody(t, map[string]string{"unknownKey": "value"})
	w = doRequest(engine, http.MethodPut, "/api/admin/config", badBody, hdrs)
	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown key: status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

// ---------- 9. Logs ----------

func TestLogs_Get(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Seed logs.
	ctx := context.Background()
	now := time.Now().Unix()
	_ = queries.CreateLog(ctx, db.CreateLogParams{DateTime: now - 100, Level: "info", Message: "msg1"})
	_ = queries.CreateLog(ctx, db.CreateLogParams{DateTime: now - 50, Level: "error", Message: "msg2"})
	_ = queries.CreateLog(ctx, db.CreateLogParams{DateTime: now, Level: "info", Message: "msg3"})

	// All logs (no filters).
	w := doRequest(engine, http.MethodGet, "/api/admin/logs", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}
	var logs []map[string]any
	decodeJSON(t, w, &logs)
	if len(logs) != 3 {
		t.Errorf("len = %d, want 3", len(logs))
	}
}

func TestLogs_FilterByDateRange(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	ctx := context.Background()
	now := time.Now().Unix()
	_ = queries.CreateLog(ctx, db.CreateLogParams{DateTime: now - 200, Level: "info", Message: "old"})
	_ = queries.CreateLog(ctx, db.CreateLogParams{DateTime: now - 50, Level: "info", Message: "mid"})
	_ = queries.CreateLog(ctx, db.CreateLogParams{DateTime: now, Level: "info", Message: "new"})

	// Only logs from (now-100) to now.
	url := fmt.Sprintf("/api/admin/logs?from=%d&to=%d", now-100, now+1)
	w := doRequest(engine, http.MethodGet, url, nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}
	var logs []map[string]any
	decodeJSON(t, w, &logs)
	if len(logs) != 2 {
		t.Errorf("len = %d, want 2 (mid + new)", len(logs))
	}
}

func TestLogs_FilterByLevel(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	ctx := context.Background()
	now := time.Now().Unix()
	_ = queries.CreateLog(ctx, db.CreateLogParams{DateTime: now - 10, Level: "info", Message: "info1"})
	_ = queries.CreateLog(ctx, db.CreateLogParams{DateTime: now - 5, Level: "error", Message: "err1"})
	_ = queries.CreateLog(ctx, db.CreateLogParams{DateTime: now, Level: "error", Message: "err2"})

	w := doRequest(engine, http.MethodGet, "/api/admin/logs?level=error", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}
	var logs []map[string]any
	decodeJSON(t, w, &logs)
	if len(logs) != 2 {
		t.Errorf("len = %d, want 2 (error logs only)", len(logs))
	}
	for _, l := range logs {
		if l["level"] != "error" {
			t.Errorf("unexpected level = %v", l["level"])
		}
	}
}

// ---------- 10. Export / Import ----------

func TestExportConfig(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Seed some data.
	_ = queries.UpsertSetting(context.Background(), db.UpsertSettingParams{Key: "audioConversion", Value: "0"})
	seedSystem(t, queries, 1, "Sys 1")

	w := doRequest(engine, http.MethodGet, "/api/admin/export/config", nil, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}
	// Verify Content-Disposition header.
	cd := w.Header().Get("Content-Disposition")
	if cd == "" {
		t.Error("missing Content-Disposition header")
	}
	var export map[string]any
	decodeJSON(t, w, &export)
	if export["systems"] == nil {
		t.Error("export should contain systems")
	}
	if export["settings"] == nil {
		t.Error("export should contain settings")
	}
}

func TestImportConfig(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Import config with settings and a group.
	importData := map[string]any{
		"settings": []map[string]string{
			{"key": "audioConversion", "value": "2"},
		},
		"groups": []map[string]any{
			{"label": "Imported Group"},
		},
		"tags":        []any{},
		"systems":     []any{},
		"talkgroups":  []any{},
		"units":       []any{},
		"apiKeys":     []any{},
		"accesses":    []any{},
		"dirwatches":  []any{},
		"downstreams": []any{},
		"webhooks":    []any{},
	}
	body := jsonBody(t, importData)
	w := doRequest(engine, http.MethodPost, "/api/admin/import/config", body, hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("import status = %d; body: %s", w.Code, w.Body.String())
	}

	// Verify group was imported.
	groups, err := queries.ListGroups(context.Background())
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	found := false
	for _, g := range groups {
		if g.Label == "Imported Group" {
			found = true
		}
	}
	if !found {
		t.Error("imported group not found in database")
	}
}

func TestImportTalkgroups_CSV(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")

	// Need a system.
	sysRowID := seedSystem(t, queries, 42, "CSV System")

	// Build multipart form with CSV.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("system_id", fmt.Sprintf("%d", sysRowID))
	fw, _ := mw.CreateFormFile("file", "talkgroups.csv")
	csvData := "talkgroup_id,label,name\n100,Fire,Fire Dispatch\n200,EMS,EMS Dispatch\n"
	_, _ = fw.Write([]byte(csvData))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/import/talkgroups", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminToken(t, uid, "admin1"))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, w, &resp)
	imported, _ := resp["imported"].(float64)
	if int(imported) != 2 {
		t.Errorf("imported = %v, want 2", resp["imported"])
	}
}

func TestImportUnits_CSV(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")

	sysRowID := seedSystem(t, queries, 43, "Units System")

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("system_id", fmt.Sprintf("%d", sysRowID))
	fw, _ := mw.CreateFormFile("file", "units.csv")
	csvData := "unit_id,label\n1001,Unit Alpha\n1002,Unit Beta\n1003,Unit Gamma\n"
	_, _ = fw.Write([]byte(csvData))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/import/units", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminToken(t, uid, "admin1"))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, w, &resp)
	imported, _ := resp["imported"].(float64)
	if int(imported) != 3 {
		t.Errorf("imported = %v, want 3", resp["imported"])
	}
}

func TestImportTalkgroups_MissingSystemID(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	// No system_id field.
	fw, _ := mw.CreateFormFile("file", "talkgroups.csv")
	_, _ = fw.Write([]byte("100,Fire,Fire Dispatch\n"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/import/talkgroups", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminToken(t, uid, "admin1"))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestImportTalkgroups_MissingFile(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	sysRowID := seedSystem(t, queries, 44, "No File System")

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("system_id", fmt.Sprintf("%d", sysRowID))
	// No file field.
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/import/talkgroups", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminToken(t, uid, "admin1"))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

// ---------- Table-driven auth enforcement for all CRUD resources ----------

func TestAdminCRUD_AuthMatrix(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	adminTok := adminToken(t, uid, "admin1")
	listenerTok := listenerToken(t, 99, "listener1")

	resources := []struct {
		name       string
		method     string
		path       string
		body       map[string]any
		wantAdmin  int
	}{
		{"GET users", http.MethodGet, "/api/admin/users", nil, http.StatusOK},
		{"GET systems", http.MethodGet, "/api/admin/systems", nil, http.StatusOK},
		{"GET talkgroups", http.MethodGet, "/api/admin/talkgroups", nil, http.StatusOK},
		{"GET groups", http.MethodGet, "/api/admin/groups", nil, http.StatusOK},
		{"GET tags", http.MethodGet, "/api/admin/tags", nil, http.StatusOK},
		{"GET apikeys", http.MethodGet, "/api/admin/apikeys", nil, http.StatusOK},
		{"GET config", http.MethodGet, "/api/admin/config", nil, http.StatusOK},
		{"GET logs", http.MethodGet, "/api/admin/logs", nil, http.StatusOK},
		{"GET export/config", http.MethodGet, "/api/admin/export/config", nil, http.StatusOK},
	}

	for _, r := range resources {
		t.Run(r.name+"_admin", func(t *testing.T) {
			var body []byte
			if r.body != nil {
				body = jsonBody(t, r.body)
			}
			w := doRequest(engine, r.method, r.path, body, map[string]string{
				"Authorization": "Bearer " + adminTok,
			})
			if w.Code != r.wantAdmin {
				t.Errorf("admin: got %d, want %d; body: %s", w.Code, r.wantAdmin, w.Body.String())
			}
		})
		t.Run(r.name+"_listener", func(t *testing.T) {
			var body []byte
			if r.body != nil {
				body = jsonBody(t, r.body)
			}
			w := doRequest(engine, r.method, r.path, body, map[string]string{
				"Authorization": "Bearer " + listenerTok,
			})
			if w.Code != http.StatusForbidden {
				t.Errorf("listener: got %d, want 403; body: %s", w.Code, w.Body.String())
			}
		})
		t.Run(r.name+"_noauth", func(t *testing.T) {
			var body []byte
			if r.body != nil {
				body = jsonBody(t, r.body)
			}
			w := doRequest(engine, r.method, r.path, body, nil)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("noauth: got %d, want 401; body: %s", w.Code, w.Body.String())
			}
		})
	}
}

// ---------- Edge cases ----------

func TestUsers_Update_DuplicateUsername(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Create a second user.
	body := jsonBody(t, map[string]any{
		"username": "user2",
		"password": "securepassword",
		"role":     "listener",
	})
	w := doRequest(engine, http.MethodPost, "/api/admin/users", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d", w.Code)
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	u2ID := int64(created["id"].(float64))

	// Try to rename user2 to admin1.
	updateBody := jsonBody(t, map[string]any{"username": "admin1", "role": "listener"})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/users/%d", u2ID), updateBody, hdrs)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

func TestSystems_Update_DuplicateSystemID(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	// Create two systems.
	body1 := jsonBody(t, map[string]any{"system_id": 100, "label": "Sys A"})
	w := doRequest(engine, http.MethodPost, "/api/admin/systems", body1, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create sys A: %d", w.Code)
	}

	body2 := jsonBody(t, map[string]any{"system_id": 200, "label": "Sys B"})
	w = doRequest(engine, http.MethodPost, "/api/admin/systems", body2, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create sys B: %d", w.Code)
	}
	var sysB map[string]any
	decodeJSON(t, w, &sysB)
	sysBID := int64(sysB["id"].(float64))

	// Try updating Sys B's system_id to 100 (conflicts with Sys A).
	updateBody := jsonBody(t, map[string]any{"system_id": 100, "label": "Sys B"})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/systems/%d", sysBID), updateBody, hdrs)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

func TestAPIKeys_Create_DuplicateKey(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"key": "dup-key-123"})
	w := doRequest(engine, http.MethodPost, "/api/admin/apikeys", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create: %d", w.Code)
	}

	w = doRequest(engine, http.MethodPost, "/api/admin/apikeys", body, hdrs)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate: status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

// ---------- Edge case: UpdateUser with empty role ----------

func TestUsers_Update_EmptyRole(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{
		"username": "user2",
		"password": "securepassword",
		"role":     "listener",
	})
	w := doRequest(engine, http.MethodPost, "/api/admin/users", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d", w.Code)
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	u2ID := int64(created["id"].(float64))

	updateBody := jsonBody(t, map[string]any{"username": "user2", "role": ""})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/users/%d", u2ID), updateBody, hdrs)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty role: status = %d, want 422; body: %s", w.Code, w.Body.String())
	}
}

func TestUsers_Update_InvalidRole(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{
		"username": "user2",
		"password": "securepassword",
		"role":     "listener",
	})
	w := doRequest(engine, http.MethodPost, "/api/admin/users", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d", w.Code)
	}
	var created map[string]any
	decodeJSON(t, w, &created)
	u2ID := int64(created["id"].(float64))

	updateBody := jsonBody(t, map[string]any{"username": "user2", "role": "superadmin"})
	w = doRequest(engine, http.MethodPut, fmt.Sprintf("/api/admin/users/%d", u2ID), updateBody, hdrs)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid role: status = %d, want 422; body: %s", w.Code, w.Body.String())
	}
}

// ---------- Edge case: Downstream/Webhook URL scheme ----------

func TestDownstreams_Create_InvalidScheme(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"url": "file:///etc/passwd"})
	w := doRequest(engine, http.MethodPost, "/api/admin/downstreams", body, hdrs)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("file:// scheme: status = %d, want 422; body: %s", w.Code, w.Body.String())
	}

	body = jsonBody(t, map[string]any{"url": "https://example.com/api"})
	w = doRequest(engine, http.MethodPost, "/api/admin/downstreams", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Errorf("https scheme: status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

func TestWebhooks_Create_InvalidScheme(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	body := jsonBody(t, map[string]any{"url": "gopher://evil.com"})
	w := doRequest(engine, http.MethodPost, "/api/admin/webhooks", body, hdrs)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("gopher:// scheme: status = %d, want 422; body: %s", w.Code, w.Body.String())
	}

	body = jsonBody(t, map[string]any{"url": "http://example.com/hook"})
	w = doRequest(engine, http.MethodPost, "/api/admin/webhooks", body, hdrs)
	if w.Code != http.StatusCreated {
		t.Errorf("http scheme: status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}


