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
			tokenStr, err := auth.GenerateToken(tc.userID, tc.username, tc.role)
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
		tokenStr, err := token.SignedString(auth.JWTSecret)
		if err != nil {
			t.Fatalf("sign expired token: %v", err)
		}

		_, err = auth.ParseToken(tokenStr)
		if err == nil {
			t.Error("ParseToken should return an error for an expired token")
		}
	})
}
