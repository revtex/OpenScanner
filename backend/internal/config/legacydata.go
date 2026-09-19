package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LegacyDBName is the database filename Squelch used when it was called
// OpenScanner.
const LegacyDBName = "openscanner.db"

// CheckLegacyDataDir reports an error when the configured data directory
// still holds a pre-rename database and no current one.
//
// Squelch will not rename the file itself. Starting fresh would look like
// catastrophic data loss — every call, user and talkgroup apparently gone
// — while silently renaming would leave an operator with no way back if
// they meant to point at a different directory. Refusing to start, and
// printing the exact command, is the only option that cannot lose data.
//
// Returns nil when there is nothing to migrate, which is the case for
// every fresh install and every already-migrated one.
func CheckLegacyDataDir(dbFile string) error {
	if dbFile == "" {
		return nil
	}
	if _, err := os.Stat(dbFile); err == nil {
		return nil // Current database exists; nothing to do.
	}
	legacy := filepath.Join(filepath.Dir(dbFile), LegacyDBName)
	if _, err := os.Stat(legacy); err != nil {
		return nil // No pre-rename database either; fresh install.
	}
	return fmt.Errorf(`found %s but no %s

Squelch was renamed from OpenScanner, and the database filename changed
with it. Your data is intact — it just needs the new name:

    mv %s %s%s

Rename the write-ahead log and shared-memory files too if they are
present (%s-wal, %s-shm). Alternatively, keep the old filename by
pointing Squelch at it explicitly:

    --db-file %s`,
		legacy, filepath.Base(dbFile),
		legacy, dbFile, walHint(legacy, dbFile),
		LegacyDBName, LegacyDBName,
		legacy)
}

// walHint adds the -wal/-shm rename lines only when those files exist, so
// the printed command matches what is actually on disk.
func walHint(legacy, dbFile string) string {
	var b strings.Builder
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(legacy + suffix); err == nil {
			fmt.Fprintf(&b, "\n    mv %s %s", legacy+suffix, dbFile+suffix)
		}
	}
	return b.String()
}
