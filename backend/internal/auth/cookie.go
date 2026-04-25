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

	// SessionCookieName is the HTTP cookie name for the session access token.
	// The cookie value is the access JWT itself; ParseToken + Tokens.IsRevoked
	// remain the single source of truth for validity.
	SessionCookieName = "os_session"

	// SessionCookiePath scopes the cookie to /api so it accompanies API
	// requests (including <audio src="/api/calls/:id/audio">) but not
	// arbitrary same-origin asset requests.
	SessionCookiePath = "/api"
)

// isSecure returns true if the request arrived over HTTPS (directly or via proxy).
func isSecure(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

// SetRefreshCookie sets an HTTP-only, SameSite=Lax cookie with the raw refresh token.
// Lax (not Strict) is required so the cookie accompanies top-level navigations
// back to the app after an OAuth-style redirect. maxAge is in seconds; pass 0
// for a session-only cookie (no Max-Age / Expires).
func SetRefreshCookie(c *gin.Context, token string, maxAge int) {
	secure := isSecure(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(RefreshCookieName, token, maxAge, RefreshCookiePath, "", secure, true)
}

// ClearRefreshCookie deletes the refresh token cookie.
func ClearRefreshCookie(c *gin.Context) {
	secure := isSecure(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(RefreshCookieName, "", -1, RefreshCookiePath, "", secure, true)
}

// SetSessionCookie writes the access JWT as an httpOnly Secure SameSite=Strict
// cookie scoped to /api so that <audio src=…> and other same-origin browser
// requests authenticate without an Authorization header. The cookie's lifetime
// mirrors the access-token TTL via maxAgeSeconds (pass 0 for a session cookie).
//
// SameSite=Strict (deliberately stricter than the refresh cookie's Lax) is
// the primary CSRF defence: the cookie is never sent on cross-site navigations
// or sub-resource requests.
func SetSessionCookie(c *gin.Context, token string, maxAgeSeconds int) {
	secure := isSecure(c)
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(SessionCookieName, token, maxAgeSeconds, SessionCookiePath, "", secure, true)
}

// ClearSessionCookie expires the os_session cookie immediately.
func ClearSessionCookie(c *gin.Context) {
	secure := isSecure(c)
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(SessionCookieName, "", -1, SessionCookiePath, "", secure, true)
}
