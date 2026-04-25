package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/handler/routes"
	"github.com/openscanner/openscanner/internal/audio"
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
	processor := audio.NewProcessor(t.TempDir(), nil)
	routes.RegisterRoutes(router, routes.Deps{
		Queries:     queries,
		RateLimiter: rl,
		Processor:   processor,
		Hub:         hub,
		SQLDB:       sqlDB,
		Version:     "test",
	})
	return router, queries
}

// adminToken returns a signed JWT for an admin user.
func adminToken(t *testing.T, userID int64, username string) string {
	t.Helper()
	tok, _, err := auth.GenerateToken(userID, username, auth.RoleAdmin, 0)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}
	return tok
}

// listenerToken returns a signed JWT for a listener user.
func listenerToken(t *testing.T, userID int64, username string) string {
	t.Helper()
	tok, _, err := auth.GenerateToken(userID, username, auth.RoleListener, 0)
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

// ---------- Import tests ----------

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
	inserted, _ := resp["inserted"].(float64)
	updated, _ := resp["updated"].(float64)
	skipped, _ := resp["skipped"].(float64)
	totalInserted := int(inserted) + int(updated)
	if totalInserted != 2 {
		t.Errorf("inserted+updated = %d, want 2 (inserted=%v, updated=%v, skipped=%v)",
			totalInserted, inserted, updated, skipped)
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
	inserted, _ := resp["inserted"].(float64)
	updated, _ := resp["updated"].(float64)
	skipped, _ := resp["skipped"].(float64)
	totalInserted := int(inserted) + int(updated)
	if totalInserted != 3 {
		t.Errorf("inserted+updated = %d, want 3 (inserted=%v, updated=%v, skipped=%v)",
			totalInserted, inserted, updated, skipped)
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
