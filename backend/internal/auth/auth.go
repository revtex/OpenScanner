// Package auth provides JWT signing/verification and bcrypt password helpers.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// HashAPIKey returns a stable SHA-256 hex digest of an API key.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

const (
	RoleAdmin    = "admin"
	RoleListener = "listener"
)

// JWTSecret is the HS256 signing key. Set by cmd/server on startup; auto-generated
// by init() if left nil/empty so dev/test environments never panic.
var JWTSecret []byte

// DummyHash is a pre-computed bcrypt cost-12 hash used to normalise response
// timing in the login handler, preventing username enumeration via timing
// side-channel (OWASP A07 / timing attack mitigation).
var DummyHash string

// Tokens is the global token tracker. Initialised by init().
var Tokens *TokenTracker

func init() {
	if len(JWTSecret) == 0 {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			panic("auth: failed to generate random JWT secret: " + err.Error())
		}
		JWTSecret = secret
	}
	h, err := bcrypt.GenerateFromPassword([]byte(""), 12)
	if err != nil {
		panic("auth: failed to generate dummy bcrypt hash: " + err.Error())
	}
	DummyHash = string(h)
	Tokens = NewTokenTracker()
}

// Claims is the JWT payload.
type Claims struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// HashPassword hashes a plaintext password using bcrypt with cost 12.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether the plaintext password matches the bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateToken signs a new HS256 JWT for the given user, valid for 24 hours.
// Returns the signed token string and the unique JTI (token ID).
func GenerateToken(userID int64, username, role string) (string, string, error) {
	now := time.Now()
	jti := uuid.New().String()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(JWTSecret)
	return signed, jti, err
}

// TokenTracker tracks active JWTs per user. When a user exceeds MaxTokens
// the oldest token is invalidated by adding it to a deny-list.
type TokenTracker struct {
	mu         sync.Mutex
	MaxTokens  int
	userTokens map[int64][]tokenEntry
	denied     map[string]time.Time // JTI → expiry time
}

type tokenEntry struct {
	JTI       string
	ExpiresAt time.Time
}

// NewTokenTracker creates a TokenTracker with a default max of 5 tokens per user.
func NewTokenTracker() *TokenTracker {
	return &TokenTracker{
		MaxTokens:  5,
		userTokens: make(map[int64][]tokenEntry),
		denied:     make(map[string]time.Time),
	}
}

// Track records a new token for the given user. If the user already has
// MaxTokens active tokens, the oldest is moved to the deny list.
func (tt *TokenTracker) Track(userID int64, jti string, expiresAt time.Time) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	tt.cleanupLocked()

	entries := tt.userTokens[userID]
	if len(entries) >= tt.MaxTokens {
		// Revoke the oldest token.
		tt.denied[entries[0].JTI] = entries[0].ExpiresAt
		entries = entries[1:]
	}
	tt.userTokens[userID] = append(entries, tokenEntry{JTI: jti, ExpiresAt: expiresAt})
}

// IsRevoked reports whether the given JTI has been revoked.
func (tt *TokenTracker) IsRevoked(jti string) bool {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	exp, ok := tt.denied[jti]
	if !ok {
		return false
	}
	// Lazily clean up expired denied entries.
	if time.Now().After(exp) {
		delete(tt.denied, jti)
		return false
	}
	return true
}

// Revoke adds a single JTI to the deny list.
// Revoke adds a single JTI to the deny list with a 24-hour expiry.
func (tt *TokenTracker) Revoke(jti string) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	tt.denied[jti] = time.Now().Add(24 * time.Hour)
}

// RevokeAllForUser revokes all tokens for a user.
func (tt *TokenTracker) RevokeAllForUser(userID int64) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	for _, e := range tt.userTokens[userID] {
		tt.denied[e.JTI] = e.ExpiresAt
	}
	delete(tt.userTokens, userID)
}

// cleanupLocked removes expired entries from both userTokens and denied.
// Must be called with tt.mu held.
func (tt *TokenTracker) cleanupLocked() {
	now := time.Now()
	for uid, entries := range tt.userTokens {
		valid := entries[:0]
		for _, e := range entries {
			if e.ExpiresAt.After(now) {
				valid = append(valid, e)
			} else {
				delete(tt.denied, e.JTI)
			}
		}
		if len(valid) == 0 {
			delete(tt.userTokens, uid)
		} else {
			tt.userTokens[uid] = valid
		}
	}
	// Also clean expired entries from the denied map (including orphans from RevokeAllForUser).
	for jti, exp := range tt.denied {
		if now.After(exp) {
			delete(tt.denied, jti)
		}
	}
}

// ParseToken verifies and parses a signed JWT string, returning the Claims on success.
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return JWTSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

// swaggerCookieName is the cookie name used for Swagger UI session auth.
const SwaggerCookieName = "os_swagger"

// SetSwaggerCookie sets a short-lived, HTTP-only, SameSite=Strict cookie
// that authorises access to the Swagger UI docs route.
// The cookie value is an HMAC-SHA256 of "swagger:<expiry>" signed with JWTSecret.
func SetSwaggerCookie(c interface {
	SetSameSite(http.SameSite)
	SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool)
}) {
	const maxAge = 3600 // 1 hour
	expiry := time.Now().Add(time.Duration(maxAge) * time.Second).Unix()
	payload := fmt.Sprintf("swagger:%d", expiry)
	mac := hmac.New(sha256.New, JWTSecret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	value := fmt.Sprintf("%d.%s", expiry, sig)

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(SwaggerCookieName, value, maxAge, "/api/admin/docs", "", false, true)
}

// ValidateSwaggerCookie checks that the swagger cookie value is valid and
// not expired.
func ValidateSwaggerCookie(value string) bool {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expiry {
		return false
	}
	payload := fmt.Sprintf("swagger:%d", expiry)
	mac := hmac.New(sha256.New, JWTSecret)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}
