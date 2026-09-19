package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/revtex/squelch/internal/config"
)

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFreshInstallStarts(t *testing.T) {
	dir := t.TempDir()
	if err := config.CheckLegacyDataDir(filepath.Join(dir, "squelch.db")); err != nil {
		t.Errorf("fresh install must start, got: %v", err)
	}
}

func TestAlreadyMigratedStarts(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "squelch.db")
	write(t, db)
	// A leftover old file alongside the new one is not a reason to stop.
	write(t, filepath.Join(dir, "openscanner.db"))

	if err := config.CheckLegacyDataDir(db); err != nil {
		t.Errorf("migrated install must start, got: %v", err)
	}
}

func TestRefusesToStartOnLegacyDataDir(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "openscanner.db")
	write(t, legacy)
	db := filepath.Join(dir, "squelch.db")

	err := config.CheckLegacyDataDir(db)
	if err == nil {
		t.Fatal("expected a refusal; starting here would look like total data loss")
	}
	msg := err.Error()
	for _, want := range []string{legacy, db, "mv "} {
		if !strings.Contains(msg, want) {
			t.Errorf("message must contain %q so the operator can act on it:\n%s", want, msg)
		}
	}
}

func TestMentionsWALFilesOnlyWhenPresent(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "openscanner.db")
	write(t, legacy)
	db := filepath.Join(dir, "squelch.db")

	if strings.Contains(config.CheckLegacyDataDir(db).Error(), "openscanner.db-wal "+db) {
		t.Error("must not print a rename for a -wal file that does not exist")
	}

	write(t, legacy+"-wal")
	if !strings.Contains(config.CheckLegacyDataDir(db).Error(), legacy+"-wal") {
		t.Error("must print the -wal rename once that file exists")
	}
}
