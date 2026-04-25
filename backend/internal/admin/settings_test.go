// Tests for the settings encryption round-trip in ConfigGet / ConfigUpdate.
//
// These live in the admin package (after the Phase-2 restructure) because
// the CRUD semantics belong here; the WebSocket framing layer is tested
// separately in internal/ws/admin_router_test.go.
package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	_ "modernc.org/sqlite"
)

// newTestOperations builds an Operations bound to an in-memory SQLite DB.
func newTestOperations(t *testing.T, encryptionKey string) (*Operations, *db.Queries) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	queries := db.New(sqlDB)
	ops := New(queries, Deps{
		SQLDB:         sqlDB,
		EncryptionKey: encryptionKey,
	}, nil)
	return ops, queries
}

func TestConfigUpdate_EncryptsSensitiveKey(t *testing.T) {
	const encKey = "test-encryption-key"
	ops, queries := newTestOperations(t, encKey)

	params, _ := json.Marshal(map[string]any{
		"settings": []map[string]string{{"key": "vapidPrivateKey", "value": "secret123"}},
	})

	if _, err := ops.ConfigUpdate(context.Background(), params, 1); err != nil {
		t.Fatalf("ConfigUpdate: %v", err)
	}

	stored, err := queries.GetSetting(context.Background(), "vapidPrivateKey")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if !auth.IsEncrypted(stored.Value) {
		t.Errorf("stored value should start with enc::; got %q", stored.Value)
	}
	plain, err := auth.DecryptString(stored.Value, encKey)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if plain != "secret123" {
		t.Errorf("decrypted value = %q, want %q", plain, "secret123")
	}
}

// TestConfigUpdate_JwtSecret_NotUserMutable asserts that jwtSecret is marked
// sensitive (so ConfigGet decrypts it) BUT is not in the admin-allowed
// mutation set — it is managed exclusively by auth.InitJWTSecret at startup.
func TestConfigUpdate_JwtSecret_NotUserMutable(t *testing.T) {
	const encKey = "another-test-key"
	ops, _ := newTestOperations(t, encKey)

	if !SensitiveSettingKeys["jwtSecret"] {
		t.Error("jwtSecret must be in SensitiveSettingKeys")
	}
	if AllowedSettingKeys("jwtSecret") {
		t.Error("jwtSecret must NOT be in allowedSettingKeys (managed by InitJWTSecret)")
	}

	params, _ := json.Marshal(map[string]any{
		"settings": []map[string]string{{"key": "jwtSecret", "value": "raw-signing-secret"}},
	})
	_, err := ops.ConfigUpdate(context.Background(), params, 1)
	if err == nil {
		t.Fatal("ConfigUpdate should reject jwtSecret as an unknown key")
	}
	if !strings.Contains(err.Error(), "jwtSecret") {
		t.Errorf("error should mention 'jwtSecret'; got: %v", err)
	}
}

func TestConfigUpdate_NonSensitiveNotEncrypted(t *testing.T) {
	const encKey = "test-encryption-key"
	ops, queries := newTestOperations(t, encKey)

	params, _ := json.Marshal(map[string]any{
		"settings": []map[string]string{{"key": "logLevel", "value": "debug"}},
	})

	if _, err := ops.ConfigUpdate(context.Background(), params, 1); err != nil {
		t.Fatalf("ConfigUpdate: %v", err)
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

func TestConfigUpdate_NoEncryptionKey_StoresPlaintext(t *testing.T) {
	// Empty encryption key — sensitive keys are stored plaintext.
	ops, queries := newTestOperations(t, "")

	params, _ := json.Marshal(map[string]any{
		"settings": []map[string]string{{"key": "vapidPrivateKey", "value": "plain-secret"}},
	})

	if _, err := ops.ConfigUpdate(context.Background(), params, 1); err != nil {
		t.Fatalf("ConfigUpdate: %v", err)
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

func TestConfigGet_DecryptsSensitiveKey(t *testing.T) {
	const encKey = "list-test-key"
	ops, queries := newTestOperations(t, encKey)

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

	result, err := ops.ConfigGet(context.Background(), nil, 1)
	if err != nil {
		t.Fatalf("ConfigGet: %v", err)
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

// TestSensitiveSettingKeys_Documented is a schema-level sanity check.
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
