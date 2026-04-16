// Package api — admin handlers (auth, config, CRUD endpoints).
package api

import (
	"context"
	"database/sql"
	"encoding/json"
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
		slog.Error("auth: failed to generate token", "user_id", user.ID, "error", err)
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
		slog.Error("failed to load user for password change", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	if !auth.CheckPassword(req.CurrentPassword, user.PasswordHash) {
		ip := c.ClientIP()
		slog.Warn("auth: password change rejected - wrong current password", "user_id", userID, "ip", ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		slog.Error("failed to hash new password", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	if err := h.queries.UpdateUserPassword(c.Request.Context(), db.UpdateUserPasswordParams{
		PasswordHash: hash,
		UpdatedAt:    time.Now().Unix(),
		ID:           userID,
	}); err != nil {
		slog.Error("failed to update password", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	// Revoke all existing tokens so compromised sessions are immediately invalidated.
	auth.Tokens.RevokeAllForUser(userID)

	ip := c.ClientIP()
	slog.Info("auth: password changed", "user_id", userID, "ip", ip)
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

type tgSelectionResponse struct {
	DisabledTGs []int64        `json:"disabledTGs"`
	AvoidList   []avoidTGEntry `json:"avoidList"`
} // @name TGSelectionResponse

type tgSelectionRequest struct {
	DisabledTGs []int64        `json:"disabledTGs"`
	AvoidList   []avoidTGEntry `json:"avoidList"`
} // @name TGSelectionRequest

type avoidTGEntry struct {
	TalkgroupID int64 `json:"talkgroupId"`
	ExpiresAt   int64 `json:"expiresAt"`
}

// GetTGSelection handles GET /api/auth/tg-selection (JWT required).
// Returns the list of talkgroup IDs the user has disabled.
//
// @Summary      Get talkgroup selection
// @Description  Return the authenticated user's disabled talkgroup IDs.
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  tgSelectionResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /auth/tg-selection [get]
func (h *AuthHandler) GetTGSelection(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID, _ := userIDVal.(int64)

	user, err := h.queries.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	resp := tgSelectionResponse{DisabledTGs: []int64{}, AvoidList: []avoidTGEntry{}}
	if user.TgSelectionJson.Valid && user.TgSelectionJson.String != "" {
		raw := []byte(user.TgSelectionJson.String)

		// Current format: { disabledTGs: number[], avoidList: [{talkgroupId, expiresAt}] }
		if err := json.Unmarshal(raw, &resp); err != nil {
			// Backward compatibility: legacy format was a bare number[]
			var legacyDisabled []int64
			if legacyErr := json.Unmarshal(raw, &legacyDisabled); legacyErr != nil {
				slog.Warn("malformed tg_selection_json", "userId", userID, "error", err)
				// Return empty lists rather than failing
			} else {
				resp.DisabledTGs = legacyDisabled
			}
		}

		if resp.DisabledTGs == nil {
			resp.DisabledTGs = []int64{}
		}
		if resp.AvoidList == nil {
			resp.AvoidList = []avoidTGEntry{}
		}
	}

	c.JSON(http.StatusOK, resp)
}

// PutTGSelection handles PUT /api/auth/tg-selection (JWT required).
// Saves the list of talkgroup IDs the user wants disabled.
//
// @Summary      Update talkgroup selection
// @Description  Save the authenticated user's disabled talkgroup IDs.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      tgSelectionRequest  true  "Disabled talkgroup IDs"
// @Success      200   {object}  object{ok=bool}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /auth/tg-selection [put]
func (h *AuthHandler) PutTGSelection(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID, _ := userIDVal.(int64)

	var req tgSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.DisabledTGs == nil {
		req.DisabledTGs = []int64{}
	}
	if req.AvoidList == nil {
		req.AvoidList = []avoidTGEntry{}
	}

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode selection"})
		return
	}

	if err := h.queries.UpdateUserTGSelection(c.Request.Context(), db.UpdateUserTGSelectionParams{
		TgSelectionJson: sql.NullString{String: string(jsonBytes), Valid: true},
		UpdatedAt:       time.Now().Unix(),
		ID:              userID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save selection"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
