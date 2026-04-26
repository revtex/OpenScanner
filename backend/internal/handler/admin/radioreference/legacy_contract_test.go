package radioreference_test

// Phase N-0 — legacy contract freeze for the admin/radioreference handler package.
//
// Pins POST /api/admin/radioreference/preview/csv response envelope.
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
)

func init() {
	gin.SetMode(gin.TestMode)
	logging.Configure(true, "")
}

// TestPreviewCSV_LegacyResponseShape pins the keys returned by the
// RadioReference CSV preview endpoint.
func TestPreviewCSV_LegacyResponseShape(t *testing.T) {
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
	rl := auth.NewRateLimiter(context.Background())
	routes.RegisterRoutes(r, routes.Deps{Queries: q, RateLimiter: rl, Processor: proc, Version: "test"})

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

	sysID, err := q.CreateSystem(context.Background(), db.CreateSystemParams{SystemID: 1, Label: "S"})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}

	csv := "Decimal,Alpha Tag,Description,Category,Tag\n100,Fire,Fire Dispatch,Public Safety,Dispatch\n"
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("system_id", fmt.Sprintf("%d", sysID))
	fw, _ := w.CreateFormFile("file", "rr.csv")
	_, _ = fw.Write([]byte(csv))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/radioreference/preview/csv", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}
	for _, k := range []string{
		"processed", "matched", "wouldUpdate", "skipped", "errors", "rowErrors", "rows",
	} {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q in preview response (got: %s)", k, rec.Body.String())
		}
	}
}
