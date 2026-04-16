package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/openscanner/openscanner/migrations"
	_ "modernc.org/sqlite" // register "sqlite" driver
)

// Open opens (or creates) the SQLite database at path, enables WAL mode and
// foreign keys, applies any pending embedded migrations, and returns the
// *sql.DB ready for use.
func Open(path string) (*sql.DB, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite %q: %w", path, err)
	}

	if err := applyMigrations(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return sqlDB, nil
}

// applyMigrations creates the schema_migrations tracking table if needed and
// applies any unapplied migration files from the embedded FS in order.
func applyMigrations(db *sql.DB) error {
	const createTracking = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT    PRIMARY KEY,
    applied_at INTEGER NOT NULL
)`
	if _, err := db.Exec(createTracking); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read embedded migrations dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		if err := applyOne(db, entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

// applyOne applies a single migration file if it has not been recorded yet.
func applyOne(db *sql.DB, name string) error {
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name,
	).Scan(&count); err != nil {
		return fmt.Errorf("check migration %q: %w", name, err)
	}
	if count > 0 {
		return nil // already applied
	}

	data, err := migrations.FS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read migration %q: %w", name, err)
	}

	upSQL := extractUpSection(string(data))
	if strings.TrimSpace(upSQL) == "" {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx for migration %q: %w", name, err)
	}

	if _, err := tx.Exec(upSQL); err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("exec migration %q: %w", name, err)
	}

	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		name, time.Now().Unix(),
	); err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("record migration %q: %w", name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", name, err)
	}

	slog.Info("db: migration applied", "version", name)
	return nil
}

// extractUpSection returns the SQL between the "-- +migrate Up" marker and
// either the "-- +migrate Down" marker or end of file.
func extractUpSection(content string) string {
	var (
		inUp bool
		sb   strings.Builder
	)
	for _, line := range strings.Split(content, "\n") {
		switch strings.TrimSpace(line) {
		case "-- +migrate Up":
			inUp = true
		case "-- +migrate Down":
			return sb.String()
		default:
			if inUp {
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
	}
	return sb.String()
}
