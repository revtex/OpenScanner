// Package api — admin handlers (auth, config, CRUD endpoints).
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	queries     *db.Queries
	rateLimiter *auth.RateLimiter
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(queries *db.Queries, rateLimiter *auth.RateLimiter) *AuthHandler {
	return &AuthHandler{
		queries:     queries,
		rateLimiter: rateLimiter,
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
} // @name LoginRequest

type loginUserResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
} // @name LoginUserResponse

type loginResponse struct {
	Token              string            `json:"token"`
	User               loginUserResponse `json:"user"`
	PasswordNeedChange bool              `json:"passwordNeedChange"`
} // @name LoginResponse

// PostLogin handles POST /api/auth/login.
// Returns 429 if rate-limited (via middleware), 401 for invalid credentials, 200 with JWT on success.
//
// @Summary      Log in
// @Description  Authenticate with username and password, returns a JWT token.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      loginRequest   true  "Login credentials"
// @Success      200   {object}  loginResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      429   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /auth/login [post]
func (h *AuthHandler) PostLogin(c *gin.Context) {
	ip := c.ClientIP()

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	user, err := h.queries.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		// Always run bcrypt to normalise response time and prevent username
		// enumeration via timing side-channel (OWASP A07).
		_ = auth.CheckPassword(req.Password, auth.DummyHash)
		h.rateLimiter.RecordFailure(ip)
		h.logAuthEvent(c.Request.Context(), "warn", "login failed: invalid username", ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if user.Disabled != 0 {
		// Return the same generic error to avoid revealing account existence
		// or disabled status (OWASP A10 — sensitive data exposure).
		h.rateLimiter.RecordFailure(ip)
		h.logAuthEvent(c.Request.Context(), "warn", "login failed: disabled account", ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Check account expiration (OWASP A01 — broken access control).
	if user.Expiration.Valid && user.Expiration.Int64 > 0 {
		if time.Now().Unix() > user.Expiration.Int64 {
			h.rateLimiter.RecordFailure(ip)
			h.logAuthEvent(c.Request.Context(), "warn", "login failed: expired account for "+user.Username, ip)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		h.rateLimiter.RecordFailure(ip)
		h.logAuthEvent(c.Request.Context(), "warn", "login failed: wrong password for "+user.Username, ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	h.rateLimiter.Reset(ip)

	token, jti, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	auth.Tokens.Track(user.ID, jti, time.Now().Add(24*time.Hour))
	h.logAuthEvent(c.Request.Context(), "info", "login success: "+user.Username, ip)
	slog.Info("user logged in", "userId", user.ID, "username", user.Username, "ip", ip)

	c.JSON(http.StatusOK, loginResponse{
		Token: token,
		User: loginUserResponse{
			ID:       user.ID,
			Username: user.Username,
			Role:     user.Role,
		},
		PasswordNeedChange: user.PasswordNeedChange != 0,
	})
}

// logAuthEvent writes an authentication event to the logs table for auditing
// (OWASP A09 — security logging & monitoring).
func (h *AuthHandler) logAuthEvent(ctx context.Context, level, message, ip string) {
	_ = h.queries.CreateLog(ctx, db.CreateLogParams{
		DateTime: time.Now().Unix(),
		Level:    level,
		Message:  message + " [ip=" + ip + "]",
	})
}

// PostLogout handles POST /api/auth/logout (JWT required).
// Revokes the current token so it cannot be reused.
//
// @Summary      Log out
// @Description  Revoke the current JWT token.
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{ok=bool}
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /auth/logout [post]
func (h *AuthHandler) PostLogout(c *gin.Context) {
	if jtiVal, ok := c.Get("jti"); ok {
		if jti, ok := jtiVal.(string); ok {
			auth.Tokens.Revoke(jti)
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
} // @name ChangePasswordRequest

// PutPassword handles PUT /api/auth/password (JWT required, any role).
// Verifies the current password and updates it to the new one.
//
// @Summary      Change password
// @Description  Change the current user's password. Requires the current password for verification.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      changePasswordRequest  true  "Current and new password"
// @Success      200   {object}  object{ok=bool}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /auth/password [put]
func (h *AuthHandler) PutPassword(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID, _ := userIDVal.(int64)

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.CurrentPassword == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "currentPassword and newPassword are required"})
		return
	}

	if len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new password must be at least 8 characters"})
		return
	}

	user, err := h.queries.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	if !auth.CheckPassword(req.CurrentPassword, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	if err := h.queries.UpdateUserPassword(c.Request.Context(), db.UpdateUserPasswordParams{
		PasswordHash: hash,
		UpdatedAt:    time.Now().Unix(),
		ID:           userID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	// Revoke all existing tokens so compromised sessions are immediately invalidated.
	auth.Tokens.RevokeAllForUser(userID)

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetMe handles GET /api/auth/me (JWT required).
// Returns the current user's basic profile.
//
// @Summary      Current user
// @Description  Return the authenticated user's profile.
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  loginUserResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /auth/me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, _ := c.Get("userID")
	username, _ := c.Get("username")
	role, _ := c.Get("role")

	uid, _ := userID.(int64)
	uname, _ := username.(string)
	roleStr, _ := role.(string)

	c.JSON(http.StatusOK, loginUserResponse{
		ID:       uid,
		Username: uname,
		Role:     roleStr,
	})
}
