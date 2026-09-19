package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/revtex/squelch/internal/auth"
	"github.com/revtex/squelch/internal/db"
)

const testEncKey = "test-encryption-key"

func secretsTestDB(t *testing.T) (*sql.DB, *db.Queries) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return sqlDB, db.New(sqlDB)
}

// seedTRInstance inserts an instance whose broker password is stored in
// the given raw form.
func seedTRInstance(t *testing.T, q *db.Queries, password string) int64 {
	t.Helper()
	ctx := context.Background()
	row, err := q.CreateTRInstance(ctx, db.CreateTRInstanceParams{
		Label:       "test-instance",
		InstanceID:  "tr-1",
		BrokerUrl:   "tcp://localhost:1883",
		BaseTopic:   "trunk_recorder",
		Username:    sql.NullString{String: "user", Valid: true},
		PasswordEnc: sql.NullString{String: password, Valid: password != ""},
		Enabled:     1,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("create tr instance: %v", err)
	}
	return row.ID
}

// TestMigrateSecretsEncryptsTRPassword is the regression for a gap that
// shipped with the MQTT integration: migrateSecrets covered settings and
// downstreams but not tr_instances, so enabling encryption on a
// deployment that already had an instance configured left its broker
// password in plaintext indefinitely.
func TestMigrateSecretsEncryptsTRPassword(t *testing.T) {
	sqlDB, q := secretsTestDB(t)
	id := seedTRInstance(t, q, "broker-pw")

	if err := migrateSecrets(context.Background(), q, sqlDB, testEncKey); err != nil {
		t.Fatalf("migrateSecrets: %v", err)
	}

	row, err := q.GetTRInstance(context.Background(), id)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if !auth.IsEncrypted(row.PasswordEnc.String) {
		t.Fatalf("broker password was not encrypted: %q", row.PasswordEnc.String)
	}
	plain, err := auth.DecryptString(row.PasswordEnc.String, testEncKey)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != "broker-pw" {
		t.Fatalf("password changed through migration: got %q", plain)
	}
}

func TestMigrateSecretsLeavesEncryptedTRPasswordAlone(t *testing.T) {
	sqlDB, q := secretsTestDB(t)
	enc, err := auth.EncryptString("broker-pw", testEncKey)
	if err != nil {
		t.Fatal(err)
	}
	id := seedTRInstance(t, q, enc)

	if err := migrateSecrets(context.Background(), q, sqlDB, testEncKey); err != nil {
		t.Fatalf("migrateSecrets: %v", err)
	}

	row, err := q.GetTRInstance(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if row.PasswordEnc.String != enc {
		t.Error("an already-encrypted password was rewritten")
	}
}

func TestMigrateSecretsRejectsTRPasswordItCannotDecrypt(t *testing.T) {
	sqlDB, q := secretsTestDB(t)
	enc, err := auth.EncryptString("broker-pw", "a-different-key")
	if err != nil {
		t.Fatal(err)
	}
	seedTRInstance(t, q, enc)

	if err := migrateSecrets(context.Background(), q, sqlDB, testEncKey); err == nil {
		t.Fatal("expected an error for a password encrypted under another key")
	}
}

func TestMigrateSecretsNoKeyRejectsEncryptedTRPassword(t *testing.T) {
	sqlDB, q := secretsTestDB(t)
	enc, err := auth.EncryptString("broker-pw", testEncKey)
	if err != nil {
		t.Fatal(err)
	}
	seedTRInstance(t, q, enc)

	// Starting with no key at all, against a database that holds an
	// encrypted password, must fail rather than silently ignore it.
	if err := migrateSecrets(context.Background(), q, sqlDB, ""); err == nil {
		t.Fatal("expected an error when an encrypted password has no key configured")
	}
}

func TestMigrateSecretsIgnoresInstanceWithNoPassword(t *testing.T) {
	sqlDB, q := secretsTestDB(t)
	seedTRInstance(t, q, "")

	if err := migrateSecrets(context.Background(), q, sqlDB, testEncKey); err != nil {
		t.Fatalf("migrateSecrets: %v", err)
	}
}
