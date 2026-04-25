// Package auth contains authentication handlers (login, refresh, logout, password change, /me, TG selection)
// and the Swagger docs session endpoint that mints a short-lived HTTP-only cookie.
package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// WSDisconnecter is the subset of ws.Hub used by Handler for session eviction.
type WSDisconnecter interface {
	DisconnectByUser(userID int64)
	DisconnectByJTI(jti string)
}

// Handler handles authentication endpoints.
type Handler struct {
	queries     *db.Queries
	rateLimiter *auth.RateLimiter
	hub         WSDisconnecter
}

// New constructs an auth Handler.
func New(queries *db.Queries, rateLimiter *auth.RateLimiter, hub WSDisconnecter) *Handler {
	return &Handler{
		queries:     queries,
		rateLimiter: rateLimiter,
		hub:         hub,
	}
}

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe *bool  `json:"rememberMe,omitempty"`
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
func (h *Handler) PostLogin(c *gin.Context) {
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
			slog.WarnContext(c.Request.Context(), "login failed: expired account", "user_id", user.ID, "ip", ip)
			h.logAuthEvent(c.Request.Context(), "warn", "login failed: expired account", ip)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		h.rateLimiter.RecordFailure(ip)
		slog.WarnContext(c.Request.Context(), "login failed: wrong password", "user_id", user.ID, "ip", ip)
		h.logAuthEvent(c.Request.Context(), "warn", "login failed: wrong password", ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	h.rateLimiter.Reset(ip)

	var accountExp int64
	if user.Expiration.Valid {
		accountExp = user.Expiration.Int64
	}

	token, jti, err := auth.GenerateToken(user.ID, user.Username, user.Role, accountExp)
	if err != nil {
		slog.Error("auth: failed to generate token", "user_id", user.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	auth.Tokens.Track(user.ID, jti, time.Now().Add(auth.AccessTokenExpiry))

	// Generate refresh token and store its hash in the DB.
	rawRefresh, hashRefresh, err := auth.GenerateRefreshToken()
	if err != nil {
		slog.Error("auth: failed to generate refresh token", "user_id", user.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	familyID := uuid.New().String()
	now := time.Now()

	// Enforce max refresh token families per user.
	count, err := h.queries.CountActiveRefreshTokenFamilies(c.Request.Context(), db.CountActiveRefreshTokenFamiliesParams{
		UserID:    user.ID,
		ExpiresAt: now.Unix(),
	})
	if err == nil && count >= auth.MaxRefreshFamilies {
		// Revoke the oldest family to make room.
		oldestFamily, err := h.queries.GetOldestActiveRefreshTokenFamily(c.Request.Context(), db.GetOldestActiveRefreshTokenFamilyParams{
			UserID:    user.ID,
			ExpiresAt: now.Unix(),
		})
		if err == nil {
			_ = h.queries.RevokeRefreshTokenFamily(c.Request.Context(), oldestFamily)
		}
	}

	if err := h.queries.CreateRefreshToken(c.Request.Context(), db.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: hashRefresh,
		FamilyID:  familyID,
		ExpiresAt: now.Add(auth.RefreshTokenExpiry).Unix(),
		CreatedAt: now.Unix(),
	}); err != nil {
		slog.Error("auth: failed to store refresh token", "user_id", user.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Set refresh token cookie. rememberMe defaults to true.
	rememberMe := req.RememberMe == nil || *req.RememberMe
	if rememberMe {
		auth.SetRefreshCookie(c, rawRefresh, int(auth.RefreshTokenExpiry.Seconds()))
	} else {
		// Session-only cookie (no Max-Age / Expires — cleared on browser close).
		auth.SetRefreshCookie(c, rawRefresh, 0)
	}

	// Also set the os_session cookie carrying the access JWT so that
	// <audio src=…> and other same-origin browser requests can authenticate
	// without injecting an Authorization header. Lifetime mirrors the
	// access-token TTL; the frontend bearer flow continues to work unchanged.
	auth.SetSessionCookie(c, token, int(auth.AccessTokenExpiry.Seconds()))

	h.logAuthEvent(c.Request.Context(), "info", "login success: "+user.Username, ip)
	slog.Info("user logged in", "user_id", user.ID, "username", user.Username, "ip", ip)

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
func (h *Handler) logAuthEvent(ctx context.Context, level, message, ip string) {
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
func (h *Handler) PostLogout(c *gin.Context) {
	if jtiVal, ok := c.Get("jti"); ok {
		if jti, ok := jtiVal.(string); ok {
			auth.Tokens.Revoke(jti)
			// Immediately disconnect the WS session using this token.
			if h.hub != nil {
				h.hub.DisconnectByJTI(jti)
			}
		}
	}

	// Revoke the refresh token family and clear the cookie.
	if rawToken, err := c.Cookie(auth.RefreshCookieName); err == nil && rawToken != "" {
		tokenHash := auth.HashRefreshToken(rawToken)
		if rt, err := h.queries.GetRefreshTokenByHash(c.Request.Context(), tokenHash); err == nil {
			_ = h.queries.RevokeRefreshTokenFamily(c.Request.Context(), rt.FamilyID)
		}
	}
	auth.ClearRefreshCookie(c)
	auth.ClearSessionCookie(c)

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type refreshResponse struct {
	Token string            `json:"token"`
	User  loginUserResponse `json:"user"`
} // @name RefreshResponse

// PostRefresh handles POST /api/auth/refresh (no JWT required — cookie is the auth).
// Validates the refresh token cookie, rotates it, and returns a new access token.
//
// @Summary      Refresh access token
// @Description  Exchange a valid refresh token cookie for a new access token and rotated refresh token.
// @Tags         Auth
// @Produce      json
// @Success      200  {object}  refreshResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /auth/refresh [post]
func (h *Handler) PostRefresh(c *gin.Context) {
	rawToken, err := c.Cookie(auth.RefreshCookieName)
	if err != nil || rawToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no refresh token"})
		return
	}

	tokenHash := auth.HashRefreshToken(rawToken)
	rt, err := h.queries.GetRefreshTokenByHash(c.Request.Context(), tokenHash)
	if err != nil {
		// Token not found — invalid or already consumed.
		auth.ClearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	// If the token has been revoked, this is a replay attack — revoke the entire family.
	if rt.Revoked != 0 {
		slog.Warn("auth: refresh token replay detected, revoking family",
			"family_id", rt.FamilyID, "user_id", rt.UserID)
		_ = h.queries.RevokeRefreshTokenFamily(c.Request.Context(), rt.FamilyID)
		auth.ClearRefreshCookie(c)
		ip := c.ClientIP()
		h.logAuthEvent(c.Request.Context(), "warn", "refresh token replay detected", ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	// Check expiration.
	if time.Now().Unix() > rt.ExpiresAt {
		_ = h.queries.RevokeRefreshToken(c.Request.Context(), rt.ID)
		auth.ClearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token expired"})
		return
	}

	// Revoke the old refresh token (rotation).
	_ = h.queries.RevokeRefreshToken(c.Request.Context(), rt.ID)

	// Load user — check disabled, expiration.
	user, err := h.queries.GetUser(c.Request.Context(), rt.UserID)
	if err != nil || user.Disabled != 0 {
		auth.ClearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	if user.Expiration.Valid && user.Expiration.Int64 > 0 {
		if time.Now().Unix() > user.Expiration.Int64 {
			auth.ClearRefreshCookie(c)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "account expired"})
			return
		}
	}

	// Generate new access token.
	var accountExp int64
	if user.Expiration.Valid {
		accountExp = user.Expiration.Int64
	}
	accessToken, jti, err := auth.GenerateToken(user.ID, user.Username, user.Role, accountExp)
	if err != nil {
		slog.Error("auth: failed to generate token on refresh", "user_id", user.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	auth.Tokens.Track(user.ID, jti, time.Now().Add(auth.AccessTokenExpiry))

	// Generate new refresh token (same family).
	newRaw, newHash, err := auth.GenerateRefreshToken()
	if err != nil {
		slog.Error("auth: failed to generate refresh token on refresh", "user_id", user.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	now := time.Now()
	if err := h.queries.CreateRefreshToken(c.Request.Context(), db.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: newHash,
		FamilyID:  rt.FamilyID,
		ExpiresAt: now.Add(auth.RefreshTokenExpiry).Unix(),
		CreatedAt: now.Unix(),
	}); err != nil {
		slog.Error("auth: failed to store rotated refresh token", "user_id", user.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Set new cookie with same Max-Age as original.
	auth.SetRefreshCookie(c, newRaw, int(auth.RefreshTokenExpiry.Seconds()))

	// Rotate the os_session cookie alongside the refresh cookie so the
	// browser-only <audio> auth path always carries a fresh access JWT.
	auth.SetSessionCookie(c, accessToken, int(auth.AccessTokenExpiry.Seconds()))

	c.JSON(http.StatusOK, refreshResponse{
		Token: accessToken,
		User: loginUserResponse{
			ID:       user.ID,
			Username: user.Username,
			Role:     user.Role,
		},
	})
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
func (h *Handler) PutPassword(c *gin.Context) {
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
	if len(req.NewPassword) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new password must be at most 128 characters"})
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
	_ = h.queries.RevokeAllRefreshTokensForUser(c.Request.Context(), userID)

	// Disconnect all active WS sessions for this user.
	if h.hub != nil {
		h.hub.DisconnectByUser(userID)
	}

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
func (h *Handler) GetMe(c *gin.Context) {
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
} // @name AvoidTGEntry

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
func (h *Handler) GetTGSelection(c *gin.Context) {
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
				slog.Warn("malformed tg_selection_json", "user_id", userID, "error", err)
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
func (h *Handler) PutTGSelection(c *gin.Context) {
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

// PostDocsSession handles POST /api/admin/docs/session.
// It mints a short-lived HTTP-only cookie so Swagger UI can be opened in a new
// browser tab without exposing the JWT. Kept in the auth package because it is
// fundamentally a session-cookie-minting endpoint.
//
// @Summary      Create Swagger docs session cookie
// @Description  Issues a short-lived HTTP-only cookie used to access /api/admin/docs.
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  object{ok=bool}
// @Security     BearerAuth
// @Router       /admin/docs/session [post]
func PostDocsSession(c *gin.Context) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	auth.SetSwaggerCookie(c, secure)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
