package imports_test

// Phase N-0 — legacy contract freeze for the admin imports package.
//
// Pins today's response shape for the four admin CSV import endpoints:
//   POST /api/admin/import/talkgroups
//   POST /api/admin/import/units
//   POST /api/admin/import/groups
//   POST /api/admin/import/tags
//
// Plan reference: docs/plans/native-api-design-plan.md §4.1.

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

	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/handler/routes"
	"github.com/openscanner/openscanner/internal/logging"
	"github.com/openscanner/openscanner/internal/ws"
)

func init() {
	gin.SetMode(gin.TestMode)
	logging.Configure(true, "")
}

func importsFixture(t *testing.T) (*gin.Engine, *db.Queries, string) {
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

	hub := ws.NewHub(q, "test")
	r := gin.New()
	rl := auth.NewRateLimiter(context.Background())
	routes.RegisterRoutes(r, routes.Deps{
		Queries: q, RateLimiter: rl, Processor: proc, Hub: hub, SQLDB: sqlDB, Version: "test",
	})

	hash, _ := auth.HashPassword("pw")
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
	return r, q, tok
}

func multipartCSV(t *testing.T, fields map[string]string, csvContent string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField %q: %v", k, err)
		}
	}
	fw, err := w.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte(csvContent)); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

// TestImportsLegacyContract pins the response envelope for each of the four
// import endpoints. The talkgroups + units endpoints return
// {inserted, updated, skipped, failed[, message]}, while groups + tags
// return {inserted, skipped, failed[, message]} (no `updated` key — they are
// label-only insert-or-skip).
func TestImportsLegacyContract(t *testing.T) {
	engine, q, tok := importsFixture(t)
	bearer := func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+tok) }

	sysID, err := q.CreateSystem(context.Background(), db.CreateSystemParams{SystemID: 42, Label: "S"})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		fields      map[string]string
		csv         string
		wantKeys    []string
		notWantKeys []string
	}{
		{
			name:     "POST /api/admin/import/talkgroups",
			path:     "/api/admin/import/talkgroups",
			fields:   map[string]string{"system_id": fmt.Sprintf("%d", sysID)},
			csv:      "talkgroup_id,label,name\n100,Fire,Fire Dispatch\n200,EMS,EMS Dispatch\n",
			wantKeys: []string{"inserted", "updated", "skipped", "failed"},
		},
		{
			name:     "POST /api/admin/import/units",
			path:     "/api/admin/import/units",
			fields:   map[string]string{"system_id": fmt.Sprintf("%d", sysID)},
			csv:      "unit_id,label\n1001,Alpha\n1002,Beta\n",
			wantKeys: []string{"inserted", "updated", "skipped", "failed"},
		},
		{
			name:        "POST /api/admin/import/groups",
			path:        "/api/admin/import/groups",
			fields:      nil,
			csv:         "label\nPolice\nFire\n",
			wantKeys:    []string{"inserted", "skipped", "failed"},
			notWantKeys: []string{"updated"},
		},
		{
			name:        "POST /api/admin/import/tags",
			path:        "/api/admin/import/tags",
			fields:      nil,
			csv:         "label\nDispatch\nTac\n",
			wantKeys:    []string{"inserted", "skipped", "failed"},
			notWantKeys: []string{"updated"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, ct := multipartCSV(t, tc.fields, tc.csv)
			req := httptest.NewRequest(http.MethodPost, tc.path, body)
			req.Header.Set("Content-Type", ct)
			bearer(req)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
			}

			var resp map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v\nbody: %s", err, w.Body.String())
			}
			for _, k := range tc.wantKeys {
				if _, ok := resp[k]; !ok {
					t.Errorf("response missing key %q (got: %s)", k, w.Body.String())
				}
			}
			for _, k := range tc.notWantKeys {
				if _, ok := resp[k]; ok {
					t.Errorf("response unexpectedly has key %q (got: %s)", k, w.Body.String())
				}
			}
		})
	}
}

// TestImports_RequireAdmin pins the 401/403 gates on the import endpoints.
func TestImports_RequireAdmin(t *testing.T) {
	engine, _, _ := importsFixture(t)

	for _, p := range []string{
		"/api/admin/import/talkgroups",
		"/api/admin/import/units",
		"/api/admin/import/groups",
		"/api/admin/import/tags",
	} {
		req := httptest.NewRequest(http.MethodPost, p, nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s no auth: status = %d, want 401", p, w.Code)
		}
	}
}
