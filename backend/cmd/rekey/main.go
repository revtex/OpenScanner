// Command squelch-rekey re-encrypts secrets at rest after the v3.0.0
// change to the key-derivation inputs.
//
// It is deliberately a separate binary. The server never re-encrypts
// anything on its own: a process that silently rewrites every secret it
// finds is indistinguishable, from the outside, from one that has
// corrupted them. This tool does it once, on purpose, with a backup and
// a dry run first.
//
// Usage:
//
//	squelch-rekey -db /var/lib/squelch/squelch.db            # dry run
//	squelch-rekey -db /var/lib/squelch/squelch.db -apply     # rewrite
//
// The encryption key comes from -key, or SQUELCH_ENCRYPTION_KEY. It is
// the same key the server runs with; this tool does not change it, only
// what is derived from it.
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // register "sqlite" driver

	"github.com/revtex/squelch/internal/auth"
)

// candidate is one encrypted value found in the database.
type candidate struct {
	table  string
	column string
	rowid  int64
	value  string
}

// ref names a candidate for an operator reading an error message.
func (c candidate) ref() string {
	return fmt.Sprintf("%s.%s rowid=%d", c.table, c.column, c.rowid)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nsquelch-rekey: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dbPath   = flag.String("db", os.Getenv("SQUELCH_DB_FILE"), "path to squelch.db (or SQUELCH_DB_FILE)")
		key      = flag.String("key", os.Getenv("SQUELCH_ENCRYPTION_KEY"), "encryption key (or SQUELCH_ENCRYPTION_KEY)")
		keyFile  = flag.String("key-file", os.Getenv("SQUELCH_ENCRYPTION_KEY_FILE"), "file holding the encryption key (or SQUELCH_ENCRYPTION_KEY_FILE)")
		apply    = flag.Bool("apply", false, "write the changes; without it, report and exit")
		backup   = flag.String("backup", "", "backup path (default: <db>.pre-rekey-<timestamp>)")
		noBackup = flag.Bool("no-backup", false, "skip the backup — only if you have taken one yourself")
	)
	flag.Parse()

	if *dbPath == "" {
		return errors.New("-db is required (or set SQUELCH_DB_FILE)")
	}
	// A key file is the better way to supply this, and it is what the
	// server supports, so the tool has to read it the same way — an
	// operator using the file pattern should not have to extract their
	// key onto a command line to run this.
	if *key == "" && *keyFile != "" {
		data, err := os.ReadFile(*keyFile)
		if err != nil {
			return fmt.Errorf("read encryption key file: %w", err)
		}
		*key = strings.TrimSpace(string(data))
		if *key == "" {
			return fmt.Errorf("encryption key file %s is empty", *keyFile)
		}
	}
	if *key == "" {
		return errors.New("no encryption key: pass -key or -key-file, or set\n" +
			"SQUELCH_ENCRYPTION_KEY or SQUELCH_ENCRYPTION_KEY_FILE.\n\n" +
			"This is the same key the server runs with. Without it the secrets\n" +
			"cannot be read, and this tool cannot help you.")
	}
	if _, err := os.Stat(*dbPath); err != nil {
		return fmt.Errorf("database not found: %w", err)
	}

	db, err := sql.Open("sqlite", *dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity check failed (%s) — do not run this tool on a damaged database", integrity)
	}

	found, err := scan(db)
	if err != nil {
		return err
	}
	if len(found) == 0 {
		fmt.Println("No encrypted values found. Nothing to do.")
		return nil
	}

	var pending, done []candidate
	for _, c := range found {
		switch {
		case decryptsWith(auth.DecryptString, c.value, *key):
			done = append(done, c)
		case decryptsWith(decryptLegacy, c.value, *key):
			pending = append(pending, c)
		default:
			return fmt.Errorf("cannot decrypt %s with either the current or the pre-v3.0.0 scheme.\n\n"+
				"That means the encryption key does not match this database, or the value is\n"+
				"damaged. Nothing has been changed. Check SQUELCH_ENCRYPTION_KEY against the\n"+
				"one the server was last running with.", c.ref())
		}
	}

	fmt.Printf("Found %d encrypted value(s): %d already current, %d to re-encrypt.\n",
		len(found), len(done), len(pending))
	for _, c := range pending {
		fmt.Printf("  re-encrypt  %s\n", c.ref())
	}
	for _, c := range done {
		fmt.Printf("  up to date  %s\n", c.ref())
	}

	if len(pending) == 0 {
		fmt.Println("\nEverything is already on the current scheme. Nothing to do.")
		return nil
	}
	if !*apply {
		fmt.Println("\nDry run — nothing was written. Re-run with -apply to make these changes.")
		return nil
	}

	if !*noBackup {
		path := *backup
		if path == "" {
			path = fmt.Sprintf("%s.pre-rekey-%s", *dbPath, time.Now().UTC().Format("20060102T150405Z"))
		}
		if err := backupTo(db, path); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		fmt.Printf("\nBackup written to %s\n", path)
	}

	if err := rewrite(db, pending, *key); err != nil {
		return err
	}
	fmt.Printf("Re-encrypted %d value(s). Start the server normally.\n", len(pending))
	return nil
}

// scan finds every TEXT-ish column in every user table and collects the
// values carrying the "enc::" prefix.
//
// Deliberately generic rather than a hard-coded list of the three places
// secrets live today: a column added later would otherwise be missed
// silently, and a missed secret is one that stops working.
func scan(db *sql.DB) ([]candidate, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return nil, err
		}
		tables = append(tables, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var found []candidate
	for _, table := range tables {
		cols, err := textColumns(db, table)
		if err != nil {
			return nil, err
		}
		for _, col := range cols {
			q := fmt.Sprintf(`SELECT rowid, %q FROM %q WHERE %q LIKE 'enc::%%'`, col, table, col)
			cr, err := db.Query(q)
			if err != nil {
				// A WITHOUT ROWID table has no rowid; nothing Squelch
				// stores secrets in is one, so skip rather than fail.
				continue
			}
			for cr.Next() {
				c := candidate{table: table, column: col}
				if err := cr.Scan(&c.rowid, &c.value); err != nil {
					cr.Close()
					return nil, err
				}
				found = append(found, c)
			}
			cr.Close()
			if err := cr.Err(); err != nil {
				return nil, err
			}
		}
	}
	return found, nil
}

// textColumns returns the columns of table whose declared type can hold a
// string. SQLite is loosely typed, so this is a filter, not a guarantee.
func textColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, fmt.Errorf("table_info %s: %w", table, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var (
			cid        int
			name, typ  string
			notNull    int
			dflt       sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &primaryKey); err != nil {
			return nil, err
		}
		t := strings.ToUpper(typ)
		if t == "" || strings.Contains(t, "CHAR") || strings.Contains(t, "TEXT") ||
			strings.Contains(t, "CLOB") || strings.Contains(t, "BLOB") {
			cols = append(cols, name)
		}
	}
	return cols, rows.Err()
}

// decryptsWith reports whether fn can read value under passphrase.
func decryptsWith(fn func(string, string) (string, error), value, passphrase string) bool {
	_, err := fn(value, passphrase)
	return err == nil
}

// backupTo writes a consistent copy of the database, WAL included.
// VACUUM INTO is used rather than copying the file: it produces a single
// clean database even while a -wal and -shm exist beside the original.
func backupTo(db *sql.DB, path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists — refusing to overwrite a backup", path)
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	_, err := db.Exec(fmt.Sprintf("VACUUM INTO %q", path))
	return err
}

// rewrite re-encrypts every pending value in one transaction. All of them
// change or none do: a half-migrated database has no way to tell which
// half is which.
func rewrite(db *sql.DB, pending []candidate, passphrase string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	for _, c := range pending {
		plain, err := decryptLegacy(c.value, passphrase)
		if err != nil {
			return fmt.Errorf("decrypt %s: %w", c.ref(), err)
		}
		enc, err := auth.EncryptString(plain, passphrase)
		if err != nil {
			return fmt.Errorf("encrypt %s: %w", c.ref(), err)
		}
		q := fmt.Sprintf(`UPDATE %q SET %q = ? WHERE rowid = ?`, c.table, c.column)
		if _, err := tx.Exec(q, enc, c.rowid); err != nil {
			return fmt.Errorf("update %s: %w", c.ref(), err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
