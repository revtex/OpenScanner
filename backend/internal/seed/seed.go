// Package seed inserts default rows into a fresh database. All inserts are
// idempotent (INSERT OR IGNORE) and run inside a single transaction.
package seed

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// Seed inserts default application data if it does not already exist.
// It is safe to call on every startup.
func Seed(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rolled back intentionally on error path

	if err := seedAppState(ctx, tx); err != nil {
		return err
	}
	if err := seedSettings(ctx, tx); err != nil {
		return err
	}
	if err := seedGroups(ctx, tx); err != nil {
		return err
	}
	if err := seedTags(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}

	slog.Info("database seeded with defaults")
	return nil
}

func seedAppState(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO app_state (id, setup_complete) VALUES (1, 0)`)
	if err != nil {
		return fmt.Errorf("seed app_state: %w", err)
	}
	return nil
}

func seedSettings(ctx context.Context, tx *sql.Tx) error {
	defaults := []struct{ key, value string }{
		{"autoPopulate", "true"},
		{"pruneDays", "7"},
		{"maxClients", "200"},
		{"time12hFormat", "false"},
		{"dimmerDelay", "5000"},
		{"keypadBeeps", "uniden"},
		{"duplicateDetectionTimeFrame", "500"},
		{"disableDuplicateDetection", "false"},
		{"sortTalkgroups", "false"},
		{"audioConversion", "1"},
		{"showListenersCount", "false"},
		{"tagsToggle", "false"},
		{"playbackGoesLive", "false"},
		{"searchPatchedTalkgroups", "false"},
		{"publicAccess", "false"},
		{"shareableLinks", "false"},
		{"keyboardShortcuts", "true"},
		{"darkMode", "true"},
		{"pushNotifications", "false"},
		{"webhooksEnabled", "false"},
		{"transcriptionEnabled", "false"},
		{"transcriptionModel", "base"},
		{"transcriptionBinary", "whisper"},
		{"transcriptionLanguage", "en"},
		{"activityDashboard", "false"},
		{"afsSystems", ""},
		{"branding", ""},
		{"email", ""},
		{"vapidPublicKey", ""},
		{"vapidPrivateKey", ""},
	}

	for _, s := range defaults {
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)`,
			s.key, s.value)
		if err != nil {
			return fmt.Errorf("seed setting %q: %w", s.key, err)
		}
	}
	return nil
}

func seedGroups(ctx context.Context, tx *sql.Tx) error {
	groups := []string{"Air", "EMS", "Fire", "Interop", "Law", "Unknown"}
	for _, name := range groups {
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO groups (name) VALUES (?)`, name)
		if err != nil {
			return fmt.Errorf("seed group %q: %w", name, err)
		}
	}
	return nil
}

func seedTags(ctx context.Context, tx *sql.Tx) error {
	tags := []string{
		"Air Traffic Control",
		"Emergency",
		"Fire Dispatch",
		"Fire Tac",
		"Fire Talk",
		"Interop",
		"Security",
		"Service",
		"Untagged",
	}
	for _, name := range tags {
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO tags (name) VALUES (?)`, name)
		if err != nil {
			return fmt.Errorf("seed tag %q: %w", name, err)
		}
	}
	return nil
}
