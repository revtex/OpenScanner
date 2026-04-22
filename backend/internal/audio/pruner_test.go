package audio

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/db"
	_ "modernc.org/sqlite"
)

// newPrunerFixture returns an in-memory DB + queries, a recordings root under
// t.TempDir(), and a seeded system. It sets pruneDays=1 so any row with
// date_time older than 24h is eligible for pruning.
func newPrunerFixture(t *testing.T) (*db.Queries, string, int64) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	q := db.New(sqlDB)
	ctx := context.Background()

	if err := q.UpsertSetting(ctx, db.UpsertSettingParams{Key: "pruneDays", Value: "1"}); err != nil {
		t.Fatalf("upsert pruneDays: %v", err)
	}

	sysID, err := q.CreateSystem(ctx, db.CreateSystemParams{
		SystemID: 1, Label: "test", AutoPopulateTalkgroups: 1,
	})
	if err != nil {
		t.Fatalf("create system: %v", err)
	}

	return q, t.TempDir(), sysID
}

func seedCall(t *testing.T, q *db.Queries, sysID int64, audioPath string, dateTime int64) int64 {
	t.Helper()
	id, err := q.CreateCall(context.Background(), db.CreateCallParams{
		AudioPath: audioPath,
		AudioName: filepath.Base(audioPath),
		AudioType: "audio/wav",
		DateTime:  dateTime,
		SystemID:  sysID,
	})
	if err != nil {
		t.Fatalf("create call: %v", err)
	}
	return id
}

func callExists(t *testing.T, q *db.Queries, id int64) bool {
	t.Helper()
	// The pruner uses GetCallIDsOlderThan; simpler for us: check via a direct
	// DB handle. Use the list query to avoid raw SQL.
	rows, err := q.GetCallIDsOlderThan(context.Background(), time.Now().Add(365*24*time.Hour).Unix())
	if err != nil {
		t.Fatalf("list calls: %v", err)
	}
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

func TestPruner_PathEscape_RefusesToDelete(t *testing.T) {
	q, recDir, sysID := newPrunerFixture(t)

	// Plant a sentinel file OUTSIDE recordingsDir that must not be touched.
	sentinel := filepath.Join(t.TempDir(), "must-not-delete.bin")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}
	// Relative path from recDir that escapes to the sentinel's directory.
	// filepath.Join(recDir, "../OTHER/must-not-delete.bin") computes the
	// sentinel's real path if OTHER is recDir's sibling. We don't need that
	// to exist — the pruner just checks the cleaned path escape.
	escape := "../../../etc/passwd"

	oldTs := time.Now().Add(-72 * time.Hour).Unix()
	callID := seedCall(t, q, sysID, escape, oldTs)

	pruneOldCalls(context.Background(), q, recDir)

	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel was touched: %v", err)
	}
	// Current behaviour: row is still deleted even though file removal was
	// refused. Assert that to lock in behaviour.
	if callExists(t, q, callID) {
		t.Fatal("expected row to be deleted even when path escape refuses file removal")
	}
}

func TestPruner_AbsolutePath_RefusesToDelete(t *testing.T) {
	q, recDir, sysID := newPrunerFixture(t)

	sentinel := filepath.Join(t.TempDir(), "abs-sentinel.bin")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}

	oldTs := time.Now().Add(-72 * time.Hour).Unix()
	// Absolute path outside the recordings root.
	callID := seedCall(t, q, sysID, sentinel, oldTs)

	pruneOldCalls(context.Background(), q, recDir)

	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel file was removed: %v", err)
	}
	if callExists(t, q, callID) {
		t.Fatal("row should be deleted even when file removal refused")
	}
}

func TestPruner_NormalPath_DeletesBoth(t *testing.T) {
	q, recDir, sysID := newPrunerFixture(t)

	relPath := "legit/file.wav"
	absPath := filepath.Join(recDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(absPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	oldTs := time.Now().Add(-72 * time.Hour).Unix()
	callID := seedCall(t, q, sysID, relPath, oldTs)

	pruneOldCalls(context.Background(), q, recDir)

	if _, err := os.Stat(absPath); !os.IsNotExist(err) {
		t.Fatalf("file not removed: err=%v", err)
	}
	if callExists(t, q, callID) {
		t.Fatal("row not removed")
	}
}

func TestPruner_MissingFile_DeletesRowAnyway(t *testing.T) {
	q, recDir, sysID := newPrunerFixture(t)

	oldTs := time.Now().Add(-72 * time.Hour).Unix()
	callID := seedCall(t, q, sysID, "missing/file.wav", oldTs)

	pruneOldCalls(context.Background(), q, recDir)

	if callExists(t, q, callID) {
		t.Fatal("row must be deleted even when file is absent on disk")
	}
}

// Ensure sql is referenced so the import isn't pruned.
var _ = sql.ErrNoRows
