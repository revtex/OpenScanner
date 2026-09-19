package main

import (
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/revtex/squelch/internal/auth"
)

const testPass = "correct horse battery staple"

// TestLegacyKeyIsPinned fixes the pre-v3.0.0 derived key.
//
// This is the same golden value internal/auth pinned before v3.0.0. It
// lives here now because this tool is the only thing that still reads
// ciphertext written under it. If this fails, the tool can no longer
// migrate any existing deployment.
func TestLegacyKeyIsPinned(t *testing.T) {
	key, err := deriveLegacyKey(testPass)
	if err != nil {
		t.Fatalf("deriveLegacyKey: %v", err)
	}
	const want = "6ffec789c8c5ddc5575e0909fc0244813ba9bcd799878aa8dfb696a7a61124dc"
	if got := hex.EncodeToString(key); got != want {
		t.Errorf("legacy derived key changed: got %s, want %s\n"+
			"This tool can no longer read pre-v3.0.0 secrets.", got, want)
	}
}

// TestLegacyAndCurrentDiffer guards the whole premise: if the two schemes
// derived the same key there would be nothing to migrate, and a silent
// no-op would look like success.
func TestLegacyAndCurrentDiffer(t *testing.T) {
	legacy, err := deriveLegacyKey(testPass)
	if err != nil {
		t.Fatal(err)
	}
	ct, err := auth.EncryptString("x", testPass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decryptLegacy(ct, testPass); err == nil {
		t.Fatal("current ciphertext decrypted under the legacy key — the schemes are not distinct")
	}
	_ = legacy
}

// encryptLegacy mirrors the pre-v3.0.0 EncryptString, so a test can build
// a database that looks like one written before the change.
func encryptLegacy(t *testing.T, plaintext string) string {
	t.Helper()
	// Round-trip through the legacy decrypt to be sure the fixture is
	// genuinely readable by the legacy path and not by the current one.
	ct := legacyFixture(t, plaintext)
	if got, err := decryptLegacy(ct, testPass); err != nil || got != plaintext {
		t.Fatalf("legacy fixture not readable by legacy path: %v / %q", err, got)
	}
	if _, err := auth.DecryptString(ct, testPass); err == nil {
		t.Fatal("legacy fixture decrypted under the current scheme")
	}
	return ct
}

func newTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "squelch.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	schema := `
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE downstreams (id INTEGER PRIMARY KEY, api_key TEXT NOT NULL);
CREATE TABLE tr_instances (id INTEGER PRIMARY KEY, password_enc TEXT);
CREATE TABLE calls (id INTEGER PRIMARY KEY, note TEXT);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return db, path
}

func TestScanFindsEveryEncryptedColumn(t *testing.T) {
	db, _ := newTestDB(t)
	jwt := encryptLegacy(t, "jwt-secret-value")
	api := encryptLegacy(t, "downstream-api-key")
	pw := encryptLegacy(t, "broker-password")

	mustExec(t, db, `INSERT INTO settings VALUES ('jwtSecret', ?)`, jwt)
	mustExec(t, db, `INSERT INTO settings VALUES ('branding', 'Squelch')`)
	mustExec(t, db, `INSERT INTO downstreams VALUES (1, ?)`, api)
	mustExec(t, db, `INSERT INTO tr_instances VALUES (1, ?)`, pw)
	mustExec(t, db, `INSERT INTO calls VALUES (1, 'not a secret')`)

	found, err := scan(db)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(found) != 3 {
		t.Fatalf("expected 3 encrypted values, got %d: %+v", len(found), found)
	}
	// The plaintext rows must not be picked up.
	for _, c := range found {
		if c.table == "calls" {
			t.Errorf("scan picked up a plaintext value: %s", c.ref())
		}
	}
}

func TestRewriteMigratesAndIsIdempotent(t *testing.T) {
	db, _ := newTestDB(t)
	const secret = "jwt-secret-value"
	mustExec(t, db, `INSERT INTO settings VALUES ('jwtSecret', ?)`, encryptLegacy(t, secret))

	found, err := scan(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := rewrite(db, found, testPass); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	var stored string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key='jwtSecret'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	// The value must now be readable by the server's current scheme...
	got, err := auth.DecryptString(stored, testPass)
	if err != nil {
		t.Fatalf("value not readable by the current scheme after rewrite: %v", err)
	}
	if got != secret {
		t.Fatalf("plaintext changed through the migration: got %q, want %q", got, secret)
	}
	// ...and must no longer be readable by the legacy one.
	if _, err := decryptLegacy(stored, testPass); err == nil {
		t.Error("value is still readable under the legacy scheme after rewrite")
	}

	// A second pass must find nothing left to do.
	again, err := scan(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range again {
		if decryptsWith(decryptLegacy, c.value, testPass) {
			t.Errorf("%s still needs migration after a completed pass", c.ref())
		}
	}
}

func TestRewriteIsAllOrNothing(t *testing.T) {
	db, _ := newTestDB(t)
	good := encryptLegacy(t, "readable")
	mustExec(t, db, `INSERT INTO settings VALUES ('jwtSecret', ?)`, good)

	// A candidate pointing at a table that does not exist makes the
	// transaction fail partway.
	pending := []candidate{
		{table: "settings", column: "value", rowid: 1, value: good},
		{table: "nope", column: "value", rowid: 1, value: good},
	}
	if err := rewrite(db, pending, testPass); err == nil {
		t.Fatal("expected rewrite to fail")
	}

	var stored string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key='jwtSecret'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != good {
		t.Error("a failed rewrite left a partially migrated database")
	}
}

func TestBackupRefusesToOverwrite(t *testing.T) {
	db, _ := newTestDB(t)
	path := filepath.Join(t.TempDir(), "backup.db")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := backupTo(db, path); err == nil {
		t.Fatal("expected backupTo to refuse an existing file")
	}
}

func TestBackupIsReadable(t *testing.T) {
	db, _ := newTestDB(t)
	mustExec(t, db, `INSERT INTO settings VALUES ('branding', 'Squelch')`)
	path := filepath.Join(t.TempDir(), "backup.db")
	if err := backupTo(db, path); err != nil {
		t.Fatalf("backupTo: %v", err)
	}
	cp, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	var v string
	if err := cp.QueryRow(`SELECT value FROM settings WHERE key='branding'`).Scan(&v); err != nil {
		t.Fatalf("backup unreadable: %v", err)
	}
	if v != "Squelch" {
		t.Fatalf("backup content wrong: %q", v)
	}
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %s: %v", q, err)
	}
}
