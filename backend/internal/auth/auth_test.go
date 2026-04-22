package auth_test

import (
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
