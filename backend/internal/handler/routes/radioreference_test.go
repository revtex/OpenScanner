package routes_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openscanner/openscanner/internal/db"
)

// ---------- helpers ----------

func seedTalkgroup(t *testing.T, q *db.Queries, systemID, talkgroupID int64, label, name string) int64 {
	t.Helper()
	id, err := q.CreateTalkgroup(context.Background(), db.CreateTalkgroupParams{
		SystemID:    systemID,
		TalkgroupID: talkgroupID,
		Label:       sql.NullString{String: label, Valid: label != ""},
		Name:        sql.NullString{String: name, Valid: name != ""},
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup(%d, %d): %v", systemID, talkgroupID, err)
	}
	return id
}



func multipartCSV(t *testing.T, systemID string, csvContent string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("system_id", systemID); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	part, err := writer.CreateFormFile("file", "rr_export.csv")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte(csvContent)); err != nil {
		t.Fatalf("Write CSV: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

// ---------- 1. Auth enforcement for RadioReference endpoints ----------

func TestRadioReference_NoJWT(t *testing.T) {
	engine, _ := newAdminTestEngine(t)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/preview/csv", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestRadioReference_ListenerJWT(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	// Create a listener token (should be forbidden from admin endpoints)
	tok := listenerToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/preview/csv", nil, hdrs)
	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}

// ---------- 2. CSV Preview ----------

func TestRadioReference_PreviewCSV_Success(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")

	sysID := seedSystem(t, queries, 100, "Test System")
	seedTalkgroup(t, queries, sysID, 1001, "", "")   // empty label/name → fill_missing would update
	seedTalkgroup(t, queries, sysID, 1002, "E", "N") // already has values → no update in fill_missing

	csv := "Talkgroup ID,Alpha Tag,Description,Group,Tag\n1001,Engine1,Fire Engine 1,Fire,Dispatch\n1002,Medic,Medic Unit,EMS,Dispatch\n9999,Ghost,Not in system,Law,Tactical\n"

	body, contentType := multipartCSV(t, fmt.Sprintf("%d", sysID), csv)

	req := newMultipartRequest(t, http.MethodPost, "/api/admin/radioreference/preview/csv", body, contentType, tok)
	w := serveRequest(engine, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, w, &resp)

	if int(resp["processed"].(float64)) != 3 {
		t.Errorf("processed = %v, want 3", resp["processed"])
	}
	if int(resp["matched"].(float64)) != 2 {
		t.Errorf("matched = %v, want 2", resp["matched"])
	}
	if int(resp["skipped"].(float64)) != 1 {
		t.Errorf("skipped = %v, want 1", resp["skipped"])
	}
}

func TestRadioReference_PreviewCSV_MissingFile(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")

	sysID := seedSystem(t, queries, 100, "Test System")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}
	w := doRequest(engine, http.MethodPost, fmt.Sprintf("/api/admin/radioreference/preview/csv?system_id=%d", sysID), nil, hdrs)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestRadioReference_PreviewCSV_BadSystem(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")

	csv := "Talkgroup ID,Alpha Tag\n1001,Engine1\n"
	body, contentType := multipartCSV(t, "99999", csv)
	req := newMultipartRequest(t, http.MethodPost, "/api/admin/radioreference/preview/csv", body, contentType, tok)
	w := serveRequest(engine, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestRadioReference_PreviewCSV_MissingHeader(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")

	sysID := seedSystem(t, queries, 100, "Test System")
	csv := "Alpha Tag,Description\nEngine1,Fire Engine 1\n"
	body, contentType := multipartCSV(t, fmt.Sprintf("%d", sysID), csv)
	req := newMultipartRequest(t, http.MethodPost, "/api/admin/radioreference/preview/csv", body, contentType, tok)
	w := serveRequest(engine, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400: %s", w.Code, w.Body.String())
	}
}

// ---------- 3. CSV header normalization ----------

func TestRadioReference_PreviewCSV_AlternateHeaders(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")

	sysID := seedSystem(t, queries, 100, "Test System")
	seedTalkgroup(t, queries, sysID, 1001, "", "")

	// Use alternate header names that should still be recognized.
	csv := "TGID,Alpha,Description,Category,Service Type\n1001,Engine1,Fire Engine 1,Fire,Dispatch\n"

	body, contentType := multipartCSV(t, fmt.Sprintf("%d", sysID), csv)
	req := newMultipartRequest(t, http.MethodPost, "/api/admin/radioreference/preview/csv", body, contentType, tok)
	w := serveRequest(engine, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, w, &resp)

	if int(resp["processed"].(float64)) != 1 {
		t.Errorf("processed = %v, want 1", resp["processed"])
	}
	if int(resp["matched"].(float64)) != 1 {
		t.Errorf("matched = %v, want 1", resp["matched"])
	}
}

// ---------- multipart helpers ----------

func newMultipartRequest(t *testing.T, method, path string, body *bytes.Buffer, contentType, token string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func serveRequest(engine http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}
