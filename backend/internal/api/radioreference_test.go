package api_test

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

func seedTalkgroupFull(t *testing.T, q *db.Queries, systemID, talkgroupID int64, label, name string, groupID, tagID int64) int64 {
	t.Helper()
	gid := sql.NullInt64{}
	if groupID > 0 {
		gid = sql.NullInt64{Int64: groupID, Valid: true}
	}
	tid := sql.NullInt64{}
	if tagID > 0 {
		tid = sql.NullInt64{Int64: tagID, Valid: true}
	}
	id, err := q.CreateTalkgroup(context.Background(), db.CreateTalkgroupParams{
		SystemID:    systemID,
		TalkgroupID: talkgroupID,
		Label:       sql.NullString{String: label, Valid: label != ""},
		Name:        sql.NullString{String: name, Valid: name != ""},
		GroupID:     gid,
		TagID:       tid,
	})
	if err != nil {
		t.Fatalf("CreateTalkgroupFull(%d, %d): %v", systemID, talkgroupID, err)
	}
	return id
}

func seedGroup(t *testing.T, q *db.Queries, label string) int64 {
	t.Helper()
	id, err := q.CreateGroup(context.Background(), label)
	if err != nil {
		t.Fatalf("CreateGroup(%q): %v", label, err)
	}
	return id
}

func seedTag(t *testing.T, q *db.Queries, label string) int64 {
	t.Helper()
	id, err := q.CreateTag(context.Background(), label)
	if err != nil {
		t.Fatalf("CreateTag(%q): %v", label, err)
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

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/admin/radioreference/preview/csv"},
		{http.MethodPost, "/api/admin/radioreference/apply"},
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

func TestRadioReference_ListenerJWT(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	// Create a listener token (should be forbidden from admin endpoints)
	tok := listenerToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/admin/radioreference/preview/csv"},
		{http.MethodPost, "/api/admin/radioreference/apply"},
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

// ---------- 3. Apply ----------

func TestRadioReference_Apply_FillMissing(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")
	groupID := seedGroup(t, queries, "Fire")
	tagID := seedTag(t, queries, "Dispatch")
	seedTalkgroup(t, queries, sysID, 1001, "", "") // empty → should be updated

	reqBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [
			{"row":1, "talkgroupId":1001, "label":"Engine1", "name":"Fire Engine 1", "group":"Fire", "tag":"Dispatch"}
		],
		"mergeMode": "fill_missing"
	}`, sysID)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(reqBody), hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, w, &resp)

	if int(resp["updated"].(float64)) != 1 {
		t.Errorf("updated = %v, want 1", resp["updated"])
	}

	// Verify the talkgroup was actually updated.
	tg, err := queries.GetTalkgroupBySystemAndTGID(context.Background(), db.GetTalkgroupBySystemAndTGIDParams{
		SystemID: sysID, TalkgroupID: 1001,
	})
	if err != nil {
		t.Fatalf("GetTalkgroup: %v", err)
	}
	if !tg.Label.Valid || tg.Label.String != "Engine1" {
		t.Errorf("label = %q, want %q", tg.Label.String, "Engine1")
	}
	if !tg.Name.Valid || tg.Name.String != "Fire Engine 1" {
		t.Errorf("name = %q, want %q", tg.Name.String, "Fire Engine 1")
	}
	if !tg.GroupID.Valid || tg.GroupID.Int64 != groupID {
		t.Errorf("groupID = %v, want %d", tg.GroupID, groupID)
	}
	if !tg.TagID.Valid || tg.TagID.Int64 != tagID {
		t.Errorf("tagID = %v, want %d", tg.TagID, tagID)
	}
}

func TestRadioReference_Apply_FillMissing_SkipsExisting(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")
	seedTalkgroup(t, queries, sysID, 1001, "Existing", "Existing Name")

	reqBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [
			{"row":1, "talkgroupId":1001, "label":"NewLabel", "name":"New Name"}
		],
		"mergeMode": "fill_missing"
	}`, sysID)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(reqBody), hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, w, &resp)

	if int(resp["skipped"].(float64)) != 1 {
		t.Errorf("skipped = %v, want 1", resp["skipped"])
	}
	if int(resp["updated"].(float64)) != 0 {
		t.Errorf("updated = %v, want 0", resp["updated"])
	}

	// Verify existing values unchanged.
	tg, _ := queries.GetTalkgroupBySystemAndTGID(context.Background(), db.GetTalkgroupBySystemAndTGIDParams{
		SystemID: sysID, TalkgroupID: 1001,
	})
	if tg.Label.String != "Existing" {
		t.Errorf("label = %q, want %q", tg.Label.String, "Existing")
	}
}

func TestRadioReference_Apply_OverwriteSelected(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")
	seedTalkgroup(t, queries, sysID, 1001, "OldLabel", "Old Name")

	reqBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [
			{"row":1, "talkgroupId":1001, "label":"NewLabel", "name":"New Name"}
		],
		"mergeMode": "overwrite_selected",
		"selectedFields": ["label"]
	}`, sysID)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(reqBody), hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, w, &resp)

	if int(resp["updated"].(float64)) != 1 {
		t.Errorf("updated = %v, want 1", resp["updated"])
	}

	tg, _ := queries.GetTalkgroupBySystemAndTGID(context.Background(), db.GetTalkgroupBySystemAndTGIDParams{
		SystemID: sysID, TalkgroupID: 1001,
	})
	// Label should be overwritten.
	if tg.Label.String != "NewLabel" {
		t.Errorf("label = %q, want %q", tg.Label.String, "NewLabel")
	}
	// Name should NOT be overwritten (not in selectedFields).
	if tg.Name.String != "Old Name" {
		t.Errorf("name = %q, want %q (should not be overwritten)", tg.Name.String, "Old Name")
	}
}

func TestRadioReference_Apply_FrequencyNeverUpdated(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")
	// Create talkgroup with a known frequency.
	tgID, err := queries.CreateTalkgroup(context.Background(), db.CreateTalkgroupParams{
		SystemID:    sysID,
		TalkgroupID: 1001,
		Frequency:   sql.NullInt64{Int64: 460250000, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup: %v", err)
	}
	_ = tgID

	reqBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [
			{"row":1, "talkgroupId":1001, "label":"Engine1"}
		],
		"mergeMode": "overwrite_selected",
		"selectedFields": ["label", "frequency"]
	}`, sysID)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(reqBody), hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	tg, _ := queries.GetTalkgroupBySystemAndTGID(context.Background(), db.GetTalkgroupBySystemAndTGIDParams{
		SystemID: sysID, TalkgroupID: 1001,
	})
	if tg.Frequency.Int64 != 460250000 {
		t.Errorf("frequency = %d, want 460250000 (should never be modified)", tg.Frequency.Int64)
	}
}

func TestRadioReference_Apply_UnmatchedTalkgroup(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")

	reqBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [
			{"row":1, "talkgroupId":9999, "label":"Ghost"}
		],
		"mergeMode": "fill_missing"
	}`, sysID)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(reqBody), hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, w, &resp)

	if int(resp["skipped"].(float64)) != 1 {
		t.Errorf("skipped = %v, want 1", resp["skipped"])
	}
	rowErrors, ok := resp["rowErrors"].([]any)
	if !ok || len(rowErrors) != 1 {
		t.Errorf("rowErrors length = %d, want 1", len(rowErrors))
	}
}

func TestRadioReference_Apply_InvalidMergeMode(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")

	reqBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [{"row":1, "talkgroupId":1001}],
		"mergeMode": "yolo"
	}`, sysID)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(reqBody), hdrs)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestRadioReference_Apply_EmptyCandidates(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")

	reqBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [],
		"mergeMode": "fill_missing"
	}`, sysID)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(reqBody), hdrs)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestRadioReference_Apply_UnknownGroup(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")
	seedTalkgroup(t, queries, sysID, 1001, "", "")

	reqBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [
			{"row":1, "talkgroupId":1001, "group":"NewAutoGroup"}
		],
		"mergeMode": "fill_missing"
	}`, sysID)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(reqBody), hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, w, &resp)

	// Unknown group should be auto-created, not trigger an error.
	if int(resp["errors"].(float64)) != 0 {
		t.Errorf("errors = %v, want 0 (unknown group auto-created)", resp["errors"])
	}
	if int(resp["updated"].(float64)) != 1 {
		t.Errorf("updated = %v, want 1", resp["updated"])
	}

	// Verify the auto-created group is now assigned to the talkgroup.
	tg, err := queries.GetTalkgroupBySystemAndTGID(context.Background(), db.GetTalkgroupBySystemAndTGIDParams{
		SystemID: sysID, TalkgroupID: 1001,
	})
	if err != nil {
		t.Fatalf("GetTalkgroupBySystemAndTGID: %v", err)
	}
	if !tg.GroupID.Valid {
		t.Errorf("expected talkgroup GroupID to be set after auto-create, got NULL")
	}
}

func TestRadioReference_Apply_GroupAndTag(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")
	groupID := seedGroup(t, queries, "Fire")
	tagID := seedTag(t, queries, "Dispatch")
	seedTalkgroup(t, queries, sysID, 1001, "", "")

	reqBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [
			{"row":1, "talkgroupId":1001, "group":"Fire", "tag":"Dispatch"}
		],
		"mergeMode": "fill_missing"
	}`, sysID)

	w := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(reqBody), hdrs)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	tg, _ := queries.GetTalkgroupBySystemAndTGID(context.Background(), db.GetTalkgroupBySystemAndTGIDParams{
		SystemID: sysID, TalkgroupID: 1001,
	})
	if !tg.GroupID.Valid || tg.GroupID.Int64 != groupID {
		t.Errorf("groupID = %v, want %d", tg.GroupID, groupID)
	}
	if !tg.TagID.Valid || tg.TagID.Int64 != tagID {
		t.Errorf("tagID = %v, want %d", tg.TagID, tagID)
	}
}

// ---------- 4. CSV header normalization ----------

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

// ---------- 5. Preview + Apply round-trip ----------

func TestRadioReference_PreviewThenApply_RoundTrip(t *testing.T) {
	engine, queries := newAdminTestEngine(t)
	uid := seedAdminUser(t, queries, "admin1", "password1234")
	tok := adminToken(t, uid, "admin1")
	hdrs := map[string]string{"Authorization": "Bearer " + tok}

	sysID := seedSystem(t, queries, 100, "Test System")
	seedGroup(t, queries, "Fire")
	seedTag(t, queries, "Dispatch")
	seedTalkgroup(t, queries, sysID, 1001, "", "")
	seedTalkgroup(t, queries, sysID, 1002, "", "")

	// Step 1: Preview
	csv := "Talkgroup ID,Alpha Tag,Description,Group,Tag\n1001,Engine1,Fire Engine 1,Fire,Dispatch\n1002,Medic1,Medic Unit 1,Fire,Dispatch\n"
	body, contentType := multipartCSV(t, fmt.Sprintf("%d", sysID), csv)
	req := newMultipartRequest(t, http.MethodPost, "/api/admin/radioreference/preview/csv", body, contentType, tok)
	previewResp := serveRequest(engine, req)
	if previewResp.Code != http.StatusOK {
		t.Fatalf("preview: got %d, want 200: %s", previewResp.Code, previewResp.Body.String())
	}
	var preview map[string]any
	decodeJSON(t, previewResp, &preview)
	if int(preview["wouldUpdate"].(float64)) != 2 {
		t.Fatalf("wouldUpdate = %v, want 2", preview["wouldUpdate"])
	}

	// Step 2: Apply the same candidates
	applyBody := fmt.Sprintf(`{
		"systemId": %d,
		"candidates": [
			{"row":1, "talkgroupId":1001, "label":"Engine1", "name":"Fire Engine 1", "group":"Fire", "tag":"Dispatch"},
			{"row":2, "talkgroupId":1002, "label":"Medic1", "name":"Medic Unit 1", "group":"Fire", "tag":"Dispatch"}
		],
		"mergeMode": "fill_missing"
	}`, sysID)

	applyResp := doRequest(engine, http.MethodPost, "/api/admin/radioreference/apply", []byte(applyBody), hdrs)
	if applyResp.Code != http.StatusOK {
		t.Fatalf("apply: got %d, want 200: %s", applyResp.Code, applyResp.Body.String())
	}
	var applied map[string]any
	decodeJSON(t, applyResp, &applied)
	if int(applied["updated"].(float64)) != 2 {
		t.Errorf("updated = %v, want 2", applied["updated"])
	}

	// Step 3: Preview again — should show no updates (already filled)
	body2, contentType2 := multipartCSV(t, fmt.Sprintf("%d", sysID), csv)
	req2 := newMultipartRequest(t, http.MethodPost, "/api/admin/radioreference/preview/csv", body2, contentType2, tok)
	rePreview := serveRequest(engine, req2)
	if rePreview.Code != http.StatusOK {
		t.Fatalf("re-preview: got %d, want 200", rePreview.Code)
	}
	var rePreviewResp map[string]any
	decodeJSON(t, rePreview, &rePreviewResp)
	if int(rePreviewResp["wouldUpdate"].(float64)) != 0 {
		t.Errorf("re-preview wouldUpdate = %v, want 0 (already filled)", rePreviewResp["wouldUpdate"])
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
