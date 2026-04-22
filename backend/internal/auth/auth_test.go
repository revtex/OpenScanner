package auth_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openscanner/openscanner/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	t.Run("hash is not plaintext", func(t *testing.T) {
		hash, err := auth.HashPassword("secret123")
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		if hash == "secret123" {
			t.Error("hash should not equal the plaintext password")
		}
	})

	t.Run("CheckPassword succeeds with correct password", func(t *testing.T) {
		hash, err := auth.HashPassword("correcthorse")
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		if !auth.CheckPassword("correcthorse", hash) {
			t.Error("CheckPassword returned false for correct password")
		}
	})

	t.Run("CheckPassword fails with wrong password", func(t *testing.T) {
		hash, err := auth.HashPassword("correcthorse")
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		if auth.CheckPassword("wrongpassword", hash) {
			t.Error("CheckPassword returned true for wrong password")
		}
	})
}

func TestBcryptCost(t *testing.T) {
	hash, err := auth.HashPassword("testpassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost < 12 {
		t.Errorf("bcrypt cost = %d, want >= 12", cost)
	}
}

func TestGenerateAndParseToken(t *testing.T) {
	tests := []struct {
		name     string
		userID   int64
		username string
		role     string
	}{
		{"admin role", 1, "alice", auth.RoleAdmin},
		{"listener role", 2, "bob", auth.RoleListener},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenStr, _, err := auth.GenerateToken(tc.userID, tc.username, tc.role, 0)
			if err != nil {
				t.Fatalf("GenerateToken: %v", err)
			}

			claims, err := auth.ParseToken(tokenStr)
			if err != nil {
				t.Fatalf("ParseToken: %v", err)
			}

			if claims.UserID != tc.userID {
				t.Errorf("UserID = %d, want %d", claims.UserID, tc.userID)
			}
			if claims.Username != tc.username {
				t.Errorf("Username = %q, want %q", claims.Username, tc.username)
			}
			if claims.Role != tc.role {
				t.Errorf("Role = %q, want %q", claims.Role, tc.role)
			}
		})
	}

	t.Run("expired token returns error", func(t *testing.T) {
		now := time.Now()
		claims := &auth.Claims{
			UserID:   99,
			Username: "expired_user",
			Role:     auth.RoleAdmin,
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(now.Add(-25 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, err := token.SignedString(auth.JWTSecret())
		if err != nil {
			t.Fatalf("sign expired token: %v", err)
		}

		_, err = auth.ParseToken(tokenStr)
		if err == nil {
			t.Error("ParseToken should return an error for an expired token")
		}
	})
}

func TestTokenTracker_MaxFiveTokens(t *testing.T) {
	tt := auth.NewTokenTracker()
	expires := time.Now().Add(24 * time.Hour)

	// Issue 5 tokens for user 1 — all should be active.
	jtis := make([]string, 6)
	for i := 0; i < 5; i++ {
		jtis[i] = "jti-" + time.Now().Format("150405.000") + "-" + string(rune('a'+i))
		tt.Track(1, jtis[i], expires)
	}
	for i := 0; i < 5; i++ {
		if tt.IsRevoked(jtis[i]) {
			t.Errorf("token %d should not be revoked with only 5 active", i)
		}
	}

	// Issue a 6th token — the oldest (jtis[0]) should be revoked.
	jtis[5] = "jti-sixth"
	tt.Track(1, jtis[5], expires)

	if !tt.IsRevoked(jtis[0]) {
		t.Error("oldest token (index 0) should be revoked after 6th login")
	}
	for i := 1; i <= 5; i++ {
		if tt.IsRevoked(jtis[i]) {
			t.Errorf("token %d should still be active", i)
		}
	}
}

func TestTokenTracker_Revoke(t *testing.T) {
	tt := auth.NewTokenTracker()
	expires := time.Now().Add(24 * time.Hour)

	tt.Track(1, "t1", expires)
	if tt.IsRevoked("t1") {
		t.Error("t1 should not be revoked initially")
	}

	tt.Revoke("t1")
	if !tt.IsRevoked("t1") {
		t.Error("t1 should be revoked after Revoke()")
	}
}

func TestTokenTracker_RevokeAllForUser(t *testing.T) {
	tt := auth.NewTokenTracker()
	expires := time.Now().Add(24 * time.Hour)

	tt.Track(1, "u1-t1", expires)
	tt.Track(1, "u1-t2", expires)
	tt.Track(2, "u2-t1", expires)

	tt.RevokeAllForUser(1)

	if !tt.IsRevoked("u1-t1") {
		t.Error("u1-t1 should be revoked")
	}
	if !tt.IsRevoked("u1-t2") {
		t.Error("u1-t2 should be revoked")
	}
	if tt.IsRevoked("u2-t1") {
		t.Error("u2-t1 should not be affected by revoking user 1")
	}
}

func TestTokenTracker_ExpiredTokensCleanedUp(t *testing.T) {
	tt := auth.NewTokenTracker()
	pastExpiry := time.Now().Add(-1 * time.Hour)
	futureExpiry := time.Now().Add(24 * time.Hour)

	// Track 4 expired tokens + 1 valid one for the same user.
	for i := 0; i < 4; i++ {
		tt.Track(1, "expired-"+string(rune('a'+i)), pastExpiry)
	}
	tt.Track(1, "valid", futureExpiry)

	// Trigger cleanup by tracking another. Should NOT overflow — expired
	// entries were cleaned up so the count stays under 5.
	tt.Track(1, "valid2", futureExpiry)

	if tt.IsRevoked("valid") {
		t.Error("valid token should not be revoked (expired slots were freed)")
	}
	if tt.IsRevoked("valid2") {
		t.Error("valid2 token should not be revoked")
	}
}

func TestGenerateToken_ReturnsJTI(t *testing.T) {
	_, jti, err := auth.GenerateToken(1, "alice", auth.RoleAdmin, 0)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if jti == "" {
		t.Error("GenerateToken should return a non-empty JTI")
	}
	if len(jti) < 36 {
		t.Errorf("JTI should be a UUID (len >= 36), got len=%d", len(jti))
	}
}

// ── InitJWTSecret resolution order ────────────────────────────────────────────

// fakeSecretLoader records calls to Get/Upsert so tests can assert on them.
type fakeSecretLoader struct {
	stored      string
	storedErr   error
	getCalled   bool
	upsertCalls []struct{ key, val string }
	upsertErr   error
}

func (f *fakeSecretLoader) Get(_ context.Context, _ string) (string, error) {
	f.getCalled = true
	if f.storedErr != nil {
		return "", f.storedErr
	}
	return f.stored, nil
}

func (f *fakeSecretLoader) Upsert(_ context.Context, key, value string) error {
	f.upsertCalls = append(f.upsertCalls, struct{ key, val string }{key, value})
	f.stored = value
	if f.upsertErr != nil {
		return f.upsertErr
	}
	return nil
}

func TestInitJWTSecret_EnvVar(t *testing.T) {
	t.Setenv("OPENSCANNER_JWT_SECRET", "test-env-secret")
	loader := &fakeSecretLoader{}

	if err := auth.InitJWTSecret(context.Background(), loader, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	if got, want := string(auth.JWTSecret()), "test-env-secret"; got != want {
		t.Errorf("JWTSecret() = %q, want %q", got, want)
	}
	if loader.getCalled {
		t.Error("loader.Get should NOT be called when env var is set")
	}
	if len(loader.upsertCalls) != 0 {
		t.Errorf("loader.Upsert should NOT be called when env var is set; got %d calls", len(loader.upsertCalls))
	}
}

func TestInitJWTSecret_StoredEncrypted(t *testing.T) {
	t.Setenv("OPENSCANNER_JWT_SECRET", "")
	encKey := "test-encryption-key-do-not-use-in-prod"
	// Build a stored value: 32 random bytes → base64 → encrypt with encKey.
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	encrypted, err := auth.EncryptString(encoded, encKey)
	if err != nil {
		t.Fatalf("seed EncryptString: %v", err)
	}
	loader := &fakeSecretLoader{stored: encrypted}

	if err := auth.InitJWTSecret(context.Background(), loader, encKey); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	got := auth.JWTSecret()
	if !bytes.Equal(got, raw) {
		t.Errorf("JWTSecret() = %x, want %x", got, raw)
	}
	if len(loader.upsertCalls) != 0 {
		t.Errorf("loader.Upsert should NOT be called when stored secret is valid; got %d calls", len(loader.upsertCalls))
	}
}

func TestInitJWTSecret_StoredEncryptedWrongKey(t *testing.T) {
	t.Setenv("OPENSCANNER_JWT_SECRET", "")
	realKey := "real-key-12345"
	wrongKey := "wrong-key-67890"
	encrypted, err := auth.EncryptString("dGVzdA==", realKey) // "test" base64
	if err != nil {
		t.Fatalf("seed EncryptString: %v", err)
	}
	loader := &fakeSecretLoader{stored: encrypted}

	err = auth.InitJWTSecret(context.Background(), loader, wrongKey)
	if err == nil {
		t.Fatal("InitJWTSecret should return an error for wrong encryption key")
	}
	if !strings.Contains(err.Error(), "decode") && !strings.Contains(err.Error(), "decrypt") {
		t.Errorf("error should mention decode/decrypt failure, got: %v", err)
	}
}

func TestInitJWTSecret_GenerateAndPersist(t *testing.T) {
	t.Setenv("OPENSCANNER_JWT_SECRET", "")
	encKey := "test-enc-key"
	loader := &fakeSecretLoader{} // empty store

	if err := auth.InitJWTSecret(context.Background(), loader, encKey); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	if len(loader.upsertCalls) != 1 {
		t.Fatalf("loader.Upsert call count = %d, want 1", len(loader.upsertCalls))
	}
	call := loader.upsertCalls[0]
	if call.key != auth.JWTSecretKeyName {
		t.Errorf("Upsert key = %q, want %q", call.key, auth.JWTSecretKeyName)
	}
	if !auth.IsEncrypted(call.val) {
		t.Errorf("persisted value should have enc:: prefix, got %q", call.val)
	}
	// Round-trip: decrypt the stored value and confirm it decodes to the secret.
	plain, err := auth.DecryptString(call.val, encKey)
	if err != nil {
		t.Fatalf("decrypt persisted value: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(plain)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("generated secret length = %d, want 32", len(decoded))
	}
	if !bytes.Equal(auth.JWTSecret(), decoded) {
		t.Error("JWTSecret() should match decrypted+decoded persisted value")
	}
}

func TestInitJWTSecret_GeneratePlaintextWhenNoKey(t *testing.T) {
	t.Setenv("OPENSCANNER_JWT_SECRET", "")
	loader := &fakeSecretLoader{}

	if err := auth.InitJWTSecret(context.Background(), loader, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	if len(loader.upsertCalls) != 1 {
		t.Fatalf("Upsert call count = %d, want 1", len(loader.upsertCalls))
	}
	call := loader.upsertCalls[0]
	if auth.IsEncrypted(call.val) {
		t.Errorf("persisted value should NOT have enc:: prefix when no encryption key; got %q", call.val)
	}
	decoded, err := base64.StdEncoding.DecodeString(call.val)
	if err != nil {
		t.Fatalf("persisted value is not valid base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("decoded secret length = %d, want 32", len(decoded))
	}
	if !bytes.Equal(auth.JWTSecret(), decoded) {
		t.Error("JWTSecret() should match decoded persisted value")
	}
}

func TestInitJWTSecret_PrecedenceEnvOverStored(t *testing.T) {
	t.Setenv("OPENSCANNER_JWT_SECRET", "env-wins-value")
	encKey := "some-key"
	// Seed a valid stored value that *would* be used if env was empty.
	encrypted, err := auth.EncryptString(base64.StdEncoding.EncodeToString(make([]byte, 32)), encKey)
	if err != nil {
		t.Fatalf("seed EncryptString: %v", err)
	}
	loader := &fakeSecretLoader{stored: encrypted}

	if err := auth.InitJWTSecret(context.Background(), loader, encKey); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	if got, want := string(auth.JWTSecret()), "env-wins-value"; got != want {
		t.Errorf("JWTSecret() = %q, want %q", got, want)
	}
	if loader.getCalled {
		t.Error("loader.Get should NOT be called when env var is set (env takes precedence)")
	}
}
