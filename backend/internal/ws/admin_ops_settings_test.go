// Tests for the settings encryption round-trip in opConfigGet / opConfigUpdate.
//
// The handlers are methods on *Client; rather than standing up a full
// WebSocket connection, we construct a minimal Client with only the fields
// the handlers touch (hub, userID, isAdmin) and call the methods directly.
// This is an internal test (package ws) so it can reach unexported fields.
package ws

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	_ "modernc.org/sqlite"
)

// newAdminClientForSettings builds a minimal Client/Hub wired against an
// in-memory SQLite instance and returns everything the tests need.
func newAdminClientForSettings(t *testing.T, encryptionKey string) (*Client, *db.Queries, *sql.DB) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	queries := db.New(sqlDB)
	hub := NewHub(queries, "test", HubDeps{
		SQLDB:         sqlDB,
		EncryptionKey: encryptionKey,
	})

	c := &Client{
		hub:     hub,
		userID:  1,
		isAdmin: true,
	}
	return c, queries, sqlDB
}

func TestAdminOps_SettingsUpsert_EncryptsSensitiveKey(t *testing.T) {
	const encKey = "test-encryption-key"
	c, queries, _ := newAdminClientForSettings(t, encKey)

	params, _ := json.Marshal(map[string]any{
		"settings": []map[string]string{{"key": "vapidPrivateKey", "value": "secret123"}},
	})

	if _, err := c.opConfigUpdate(context.Background(), params); err != nil {
		t.Fatalf("opConfigUpdate: %v", err)
	}

	stored, err := queries.GetSetting(context.Background(), "vapidPrivateKey")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if !auth.IsEncrypted(stored.Value) {
		t.Errorf("stored value should start with enc::; got %q", stored.Value)
	}
	// Decrypt and confirm round-trip.
	plain, err := auth.DecryptString(stored.Value, encKey)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if plain != "secret123" {
		t.Errorf("decrypted value = %q, want %q", plain, "secret123")
	}
}

// TestAdminOps_SettingsUpsert_JwtSecret_NotUserMutable asserts that jwtSecret
// is marked sensitive (so ListSettings decrypts it when returning) BUT is not
// in the admin-allowed mutation set — it is managed exclusively by
// auth.InitJWTSecret at startup.
func TestAdminOps_SettingsUpsert_JwtSecret_NotUserMutable(t *testing.T) {
	const encKey = "another-test-key"
	c, _, _ := newAdminClientForSettings(t, encKey)

	if !SensitiveSettingKeys["jwtSecret"] {
		t.Error("jwtSecret must be in SensitiveSettingKeys")
	}
	if wsAllowedSettingKeys["jwtSecret"] {
		t.Error("jwtSecret must NOT be in wsAllowedSettingKeys (managed by InitJWTSecret)")
	}

	// Confirm that attempting to mutate it via the admin op is rejected.
	params, _ := json.Marshal(map[string]any{
		"settings": []map[string]string{{"key": "jwtSecret", "value": "raw-signing-secret"}},
	})
	_, err := c.opConfigUpdate(context.Background(), params)
	if err == nil {
		t.Fatal("opConfigUpdate should reject jwtSecret as an unknown key")
	}
	if !strings.Contains(err.Error(), "jwtSecret") {
		t.Errorf("error should mention 'jwtSecret'; got: %v", err)
	}
}

func TestAdminOps_SettingsUpsert_NonSensitiveNotEncrypted(t *testing.T) {
	const encKey = "test-encryption-key"
	c, queries, _ := newAdminClientForSettings(t, encKey)

	params, _ := json.Marshal(map[string]any{
		"settings": []map[string]string{{"key": "logLevel", "value": "debug"}},
	})

	if _, err := c.opConfigUpdate(context.Background(), params); err != nil {
		t.Fatalf("opConfigUpdate: %v", err)
	}

	stored, err := queries.GetSetting(context.Background(), "logLevel")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if auth.IsEncrypted(stored.Value) {
		t.Errorf("non-sensitive setting should NOT be encrypted; got %q", stored.Value)
	}
	if stored.Value != "debug" {
		t.Errorf("stored value = %q, want %q", stored.Value, "debug")
	}
}

func TestAdminOps_SettingsUpsert_NoEncryptionKey_StoresPlaintext(t *testing.T) {
	// Empty encryption key — sensitive keys are stored plaintext (with warning
	// logged at runtime; we don't assert on logs here).
	c, queries, _ := newAdminClientForSettings(t, "")

	params, _ := json.Marshal(map[string]any{
		"settings": []map[string]string{{"key": "vapidPrivateKey", "value": "plain-secret"}},
	})

	if _, err := c.opConfigUpdate(context.Background(), params); err != nil {
		t.Fatalf("opConfigUpdate: %v", err)
	}

	stored, err := queries.GetSetting(context.Background(), "vapidPrivateKey")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if auth.IsEncrypted(stored.Value) {
		t.Errorf("with no encryption key, value should be stored plaintext; got %q", stored.Value)
	}
	if stored.Value != "plain-secret" {
		t.Errorf("stored value = %q, want %q", stored.Value, "plain-secret")
	}
}

func TestAdminOps_SettingsList_DecryptsSensitiveKey(t *testing.T) {
	const encKey = "list-test-key"
	c, queries, _ := newAdminClientForSettings(t, encKey)

	// Seed an already-encrypted sensitive setting and a plaintext normal one.
	encrypted, err := auth.EncryptString("my-vapid-key", encKey)
	if err != nil {
		t.Fatalf("seed EncryptString: %v", err)
	}
	if err := queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "vapidPrivateKey", Value: encrypted,
	}); err != nil {
		t.Fatalf("seed UpsertSetting (vapid): %v", err)
	}
	if err := queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
		Key: "logLevel", Value: "info",
	}); err != nil {
		t.Fatalf("seed UpsertSetting (logLevel): %v", err)
	}

	result, err := c.opConfigGet(context.Background(), nil)
	if err != nil {
		t.Fatalf("opConfigGet: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", result)
	}
	list, ok := m["settings"].([]map[string]string)
	if !ok {
		t.Fatalf("settings is not []map[string]string: %T", m["settings"])
	}

	values := make(map[string]string, len(list))
	for _, s := range list {
		values[s["key"]] = s["value"]
	}

	if got, want := values["vapidPrivateKey"], "my-vapid-key"; got != want {
		t.Errorf("vapidPrivateKey returned = %q, want decrypted %q", got, want)
	}
	if got, want := values["logLevel"], "info"; got != want {
		t.Errorf("logLevel returned = %q, want %q", got, want)
	}
}

// TestSensitiveSettingKeys_Documented is a schema-level sanity check: any key
// added to the sensitive list without being wired through the encryption path
// would be a latent bug.
func TestSensitiveSettingKeys_Documented(t *testing.T) {
	want := []string{"vapidPrivateKey", "jwtSecret"}
	for _, k := range want {
		if !SensitiveSettingKeys[k] {
			t.Errorf("SensitiveSettingKeys missing expected key %q", k)
		}
	}
	if got, want := len(SensitiveSettingKeys), len(want); got != want {
		t.Errorf("SensitiveSettingKeys size = %d, want %d — if a new sensitive key was added, update this test", got, want)
	}
}
