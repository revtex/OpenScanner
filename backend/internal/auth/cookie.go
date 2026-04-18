package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// RefreshCookieName is the HTTP cookie name for the refresh token.
	RefreshCookieName = "refresh_token"

	// RefreshCookiePath restricts the cookie to auth endpoints only.
	RefreshCookiePath = "/api/auth"
)

// isSecure returns true if the request arrived over HTTPS (directly or via proxy).
func isSecure(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

// SetRefreshCookie sets an HTTP-only, SameSite=Strict cookie with the raw refresh token.
// maxAge is in seconds; pass 0 for a session-only cookie (no Max-Age / Expires).
func SetRefreshCookie(c *gin.Context, token string, maxAge int) {
	secure := isSecure(c)
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(RefreshCookieName, token, maxAge, RefreshCookiePath, "", secure, true)
}

// ClearRefreshCookie deletes the refresh token cookie.
func ClearRefreshCookie(c *gin.Context) {
	secure := isSecure(c)
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(RefreshCookieName, "", -1, RefreshCookiePath, "", secure, true)
}
