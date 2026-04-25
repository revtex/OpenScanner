package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// JWTAuth validates a Bearer JWT and stores userID, username, and role in the
// Gin context. Aborts with 401 if the token is missing or invalid.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			slog.Debug("middleware: jwt auth failed, no bearer header", "path", c.Request.URL.Path)
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

		// Check account expiration embedded in JWT claims (OWASP A01).
		if claims.AccountExp > 0 && time.Now().Unix() > claims.AccountExp {
			c.AbortWithStatusJSON(401, gin.H{"error": "account expired"})
			return
		}

		slog.Debug("middleware: jwt auth success", "user_id", claims.UserID, "role", claims.Role)
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

		// Check account expiration embedded in JWT claims (OWASP A01).
		if claims.AccountExp > 0 && time.Now().Unix() > claims.AccountExp {
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
			slog.Debug("middleware: admin check failed", "role", roleStr)
			c.AbortWithStatusJSON(403, gin.H{"error": "admin access required"})
			return
		}
		slog.Debug("middleware: admin check passed")
		c.Next()
	}
}

// APIKeyAuth reads the API key from the X-API-Key header, ?key= query param,
// or (for Trunk Recorder compatibility) a multipart "key" form field — in that
// order. It looks up the key in the database and sets "apiKeyID" in the Gin
// context. Aborts with 401 if the key is missing, not found, or disabled.
func APIKeyAuth(queries *db.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, _ := c.Get("requestID")

		// Prefer header, then query string. Only fall back to PostForm
		// (which parses the entire multipart body) when both are empty.
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.Query("key")
		}
		if key == "" {
			// Trunk Recorder's rdioscanner_uploader plugin sends the API key
			// as a multipart form field named "key" rather than a header.
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

		// Reject implausibly long keys before hashing (defense-in-depth; real
		// keys are 64 hex chars). Prevents CPU waste on attacker-controlled input.
		if len(key) > 128 {
			slog.Warn("api key auth: oversized key rejected",
				"request_id", requestID,
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
				"length", len(key),
			)
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid API key"})
			return
		}

		hashed := auth.HashAPIKey(key)
		apiKey, err := queries.GetAPIKeyByKey(c.Request.Context(), hashed)
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
		if apiKey.CallRateLimit.Valid {
			c.Set("apiKeyCallRate", apiKey.CallRateLimit.Int64)
		}
		slog.Debug("middleware: api key auth success",
			"api_key_id", apiKey.ID,
			"ident", apiKey.Ident.String,
			"path", c.Request.URL.Path,
		)
		c.Next()
	}
}

// SwaggerCookieAuth validates the short-lived docs session cookie.
func SwaggerCookieAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, err := c.Cookie(auth.SwaggerCookieName)
		if err != nil || !auth.ValidateSwaggerCookie(value) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "swagger session required"})
			return
		}
		c.Next()
	}
}
