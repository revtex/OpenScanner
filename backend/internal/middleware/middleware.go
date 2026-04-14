// Package middleware contains Gin middleware: JWT auth, API key auth, rate limiting, request ID (UUID v4), logging, CORS.
package middleware

import (
	"log/slog"
	"net/http"
	"net/url"
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

// CORS handles Cross-Origin Resource Sharing.
// In production the frontend is served from the same origin so cross-origin
// requests are rejected. The allowed origin is derived from the request's own
// Host header (same-origin only). Preflight requests are handled with 204.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// Default to same-origin: the Origin must match the Host.
		host := c.Request.Host
		// Build the expected origin from the request scheme + host.
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		expected := scheme + "://" + host
		allowed := origin == expected

		isLocalhost := func(h string) bool {
			h = strings.ToLower(h)
			return h == "localhost" || h == "127.0.0.1"
		}

		// Dev/local exception: allow localhost frontend origins on different ports
		// when backend is also running on localhost.
		if !allowed {
			if u, err := url.Parse(origin); err == nil {
				if reqHost, reqErr := url.Parse(scheme + "://" + host); reqErr == nil {
					if isLocalhost(u.Hostname()) && isLocalhost(reqHost.Hostname()) {
						allowed = true
					}
				}
			}
		}

		if !allowed {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
		c.Header("Access-Control-Max-Age", "86400")
		c.Header("Vary", "Origin")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

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

// APIKeyAuth reads the X-API-Key header, looks up the key in the database,
// and sets "apiKeyID" in the Gin context. Aborts with 401 if the key is
// missing, not found, or disabled.
func APIKeyAuth(queries *db.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, _ := c.Get("requestID")

		key := c.GetHeader("X-API-Key")
		// Trunk Recorder's rdioscanner_uploader plugin sends the API key
		// as a multipart form field named "key" rather than a header.
		if key == "" {
			key = c.PostForm("key")
		}
		if key == "" {
			slog.Warn("api key auth: missing X-API-Key header",
				"request_id", requestID,
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
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
			slog.Warn("api key auth: invalid key",
				"request_id", requestID,
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid API key"})
			return
		}
		if apiKey.Disabled != 0 {
			slog.Warn("api key auth: disabled key used",
				"request_id", requestID,
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
				"api_key_id", apiKey.ID,
			)
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

// MaxBodySize limits the size of request bodies to prevent memory exhaustion.
// Applies to non-multipart requests only (multipart is limited by
// router.MaxMultipartMemory).
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// SwaggerCookieAuth validates the short-lived HTTP-only cookie set by
// POST /api/admin/docs/session. Used to protect the Swagger UI route
// without exposing JWTs in URLs.
func SwaggerCookieAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(auth.SwaggerCookieName)
		if err != nil || !auth.ValidateSwaggerCookie(cookie) {
			c.AbortWithStatusJSON(401, gin.H{"error": "swagger session expired, please reopen from admin panel"})
			return
		}
		c.Next()
	}
}
