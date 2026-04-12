// Package middleware contains Gin middleware: JWT auth, API key auth, rate limiting, request ID (UUID v4), logging.
package middleware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// RequestID adds a UUID v4 X-Request-ID response header and stores it in the
// Gin context under the key "requestID".
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()
		c.Set("requestID", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// Logger emits a structured slog line for every request including method, path,
// status code, latency, request ID, and client IP.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		requestID, _ := c.Get("requestID")
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
			"request_id", requestID,
			"ip", c.ClientIP(),
		)
	}
}

// JWTAuth validates a Bearer JWT and stores userID, username, and role in the
// Gin context. Aborts with 401 if the token is missing or invalid.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(401, gin.H{"error": "authorization header required"})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ParseToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid or expired token"})
			return
		}

		if auth.Tokens.IsRevoked(claims.ID) {
			c.AbortWithStatusJSON(401, gin.H{"error": "token has been revoked"})
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("jti", claims.ID)
		c.Next()
	}
}

// OptionalJWTAuth extracts user info from a Bearer JWT if present, but does not
// abort the request when the token is missing or invalid. Useful for endpoints
// that are publicly accessible but provide extra data to authenticated users.
func OptionalJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.Next()
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ParseToken(tokenStr)
		if err != nil {
			c.Next()
			return
		}

		if auth.Tokens.IsRevoked(claims.ID) {
			c.Next()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("jti", claims.ID)
		c.Next()
	}
}

// RequireAdmin checks that the authenticated user has the admin role.
// Must be chained after JWTAuth. Aborts with 403 if the role is not admin.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		if roleStr != auth.RoleAdmin {
			c.AbortWithStatusJSON(403, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

// APIKeyAuth reads the X-API-Key header (or ?key= query param), looks up the key
// in the database, and sets "apiKeyID" in the Gin context. Aborts with 401 if the
// key is missing, not found, or disabled.
func APIKeyAuth(queries *db.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.Query("key")
		}
		if key == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "API key required"})
			return
		}

		hashed := auth.HashAPIKey(key)
		apiKey, err := queries.GetAPIKeyByKey(c.Request.Context(), hashed)
		if err != nil {
			// Backward compatibility for legacy rows that still store plaintext keys.
			apiKey, err = queries.GetAPIKeyByKey(c.Request.Context(), key)
		}
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid API key"})
			return
		}
		if apiKey.Disabled != 0 {
			c.AbortWithStatusJSON(401, gin.H{"error": "API key is disabled"})
			return
		}

		c.Set("apiKeyID", apiKey.ID)
		c.Next()
	}
}

// RateLimit returns middleware that rejects requests with 429 if the client IP
// is locked out by the given rate limiter.
func RateLimit(rl *auth.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl.IsLockedOut(c.ClientIP()) {
			c.AbortWithStatusJSON(429, gin.H{"error": "too many failed attempts, try again later"})
			return
		}
		c.Next()
	}
}
