package audio_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/db"
	_ "modernc.org/sqlite"
)

// newDuplicateTestDB opens an in-memory SQLite database with all migrations applied.
func newDuplicateTestDB(t *testing.T) *db.Queries {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db.New(sqlDB)
}

// seedCallAtTime inserts a system, talkgroup, and a call at the given Unix
// timestamp, returning the DB row IDs for the system and talkgroup.
func seedCallAtTime(t *testing.T, q *db.Queries, dateTimeUnix int64) (systemID, talkgroupID int64) {
	t.Helper()
	ctx := context.Background()

	sysID, err := q.CreateSystem(ctx, db.CreateSystemParams{
		SystemID:     1,
		Label:        "Test System",
		AutoPopulate: 0,
	})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}

	tgID, err := q.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
		SystemID:    sysID,
		TalkgroupID: 100,
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup: %v", err)
	}

	_, err = q.CreateCall(ctx, db.CreateCallParams{
		AudioPath:   "2026/01/01/test.wav",
		AudioName:   "test.wav",
		AudioType:   "audio/wav",
		DateTime:    dateTimeUnix,
		SystemID:    sysID,
		TalkgroupID: sql.NullInt64{Int64: tgID, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateCall: %v", err)
	}

	return sysID, tgID
}

// TestIsDuplicate_NoPreviousCall verifies that IsDuplicate returns false when
// no call exists for the given system+talkgroup.
func TestIsDuplicate_NoPreviousCall(t *testing.T) {
	q := newDuplicateTestDB(t)
	ctx := context.Background()

	sysID, err := q.CreateSystem(ctx, db.CreateSystemParams{
		SystemID:     1,
		Label:        "Test",
		AutoPopulate: 0,
	})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}
	tgID, err := q.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
		SystemID:    sysID,
		TalkgroupID: 100,
	})
	if err != nil {
		t.Fatalf("CreateTalkgroup: %v", err)
	}

	isDup, err := audio.IsDuplicate(ctx, q, sysID, tgID, time.Now(), 30000)
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if isDup {
		t.Error("IsDuplicate = true, want false (no previous call)")
	}
}

// TestIsDuplicate_WithinWindow verifies that a call checked within the
// duplicate window (5 s < 30 000 ms) is flagged as a duplicate.
func TestIsDuplicate_WithinWindow(t *testing.T) {
	q := newDuplicateTestDB(t)
	ctx := context.Background()

	baseTime := time.Now().UTC().Truncate(time.Second)
	sysID, tgID := seedCallAtTime(t, q, baseTime.Unix())

	// 5 seconds after the existing call — well within the 30 000 ms window.
	checkTime := baseTime.Add(5 * time.Second)
	isDup, err := audio.IsDuplicate(ctx, q, sysID, tgID, checkTime, 30000)
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if !isDup {
		t.Errorf("IsDuplicate = false, want true (5 s < 30 000 ms window)")
	}
}

// TestIsDuplicate_OutsideWindow verifies that a call checked beyond the
// duplicate window (35 s > 30 000 ms) is not flagged as a duplicate.
func TestIsDuplicate_OutsideWindow(t *testing.T) {
	q := newDuplicateTestDB(t)
	ctx := context.Background()

	baseTime := time.Now().UTC().Truncate(time.Second)
	sysID, tgID := seedCallAtTime(t, q, baseTime.Unix())

	// 35 seconds after the existing call — outside the 30 000 ms window.
	checkTime := baseTime.Add(35 * time.Second)
	isDup, err := audio.IsDuplicate(ctx, q, sysID, tgID, checkTime, 30000)
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if isDup {
		t.Errorf("IsDuplicate = true, want false (35 s > 30 000 ms window)")
	}
}

// TestIsDuplicate_ZeroWindow verifies that windowMs=0 disables duplicate
// detection and always returns false.
func TestIsDuplicate_ZeroWindow(t *testing.T) {
	q := newDuplicateTestDB(t)
	ctx := context.Background()

	baseTime := time.Now().UTC().Truncate(time.Second)
	sysID, tgID := seedCallAtTime(t, q, baseTime.Unix())

	// windowMs=0 → detection disabled, must always return false.
	isDup, err := audio.IsDuplicate(ctx, q, sysID, tgID, baseTime, 0)
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if isDup {
		t.Error("IsDuplicate = true, want false when windowMs=0 disables detection")
	}
}

// TestIsDuplicate_ExactTimestampReimport verifies idempotent behavior for
// replayed imports: exact same system+talkgroup+timestamp is duplicate even
// when window-based detection would otherwise not apply.
func TestIsDuplicate_ExactTimestampReimport(t *testing.T) {
	q := newDuplicateTestDB(t)
	ctx := context.Background()

	baseTime := time.Now().UTC().Truncate(time.Second)
	sysID, tgID := seedCallAtTime(t, q, baseTime.Unix())

	// Exact same timestamp should always be duplicate.
	isDup, err := audio.IsDuplicate(ctx, q, sysID, tgID, baseTime, 1)
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if !isDup {
		t.Error("IsDuplicate = false, want true for exact timestamp re-import")
	}
}
