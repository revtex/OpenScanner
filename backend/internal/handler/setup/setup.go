// Package setup provides first-run setup endpoints (POST /api/setup, GET /api/setup/status).
package setup

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// Handler holds dependencies for first-run setup endpoints.
type Handler struct {
	queries *db.Queries
	mu      sync.Mutex // guards the check-then-create in PostSetup (TOCTOU prevention)
}

// New constructs a Handler.
func New(queries *db.Queries) *Handler {
	return &Handler{queries: queries}
}

type setupStatusResponse struct {
	NeedsSetup   bool `json:"needsSetup"`
	PublicAccess bool `json:"publicAccess"`
} // @name SetupStatusResponse

// GetSetupStatus godoc
//
//	@Summary		Get setup status
//	@Description	Returns whether initial setup is needed and whether public access is active. Always unauthenticated.
//	@Tags			Setup
//	@Produce		json
//	@Success		200	{object}	setupStatusResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/setup/status [get]
func (h *Handler) GetSetupStatus(c *gin.Context) {
	ctx := c.Request.Context()
	requestID, _ := c.Get("requestID")

	state, err := h.queries.GetAppState(ctx)
	if err != nil {
		slog.Error("setup: failed to read app state", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read app state"})
		return
	}

	publicAccess := false
	setting, err := h.queries.GetSetting(ctx, "publicAccess")
	if err == nil {
		publicAccess = setting.Value == "true"
	} else if !errors.Is(err, sql.ErrNoRows) {
		slog.Error("setup: failed to read settings", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read settings"})
		return
	}

	c.JSON(http.StatusOK, setupStatusResponse{
		NeedsSetup:   state.SetupComplete == 0,
		PublicAccess: publicAccess,
	})
}

type setupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
} // @name SetupRequest

// PostSetup godoc
//
//	@Summary		Complete initial setup
//	@Description	Creates the initial admin user and marks setup as complete. Returns 409 if setup is already done, 400 for invalid input.
//	@Tags			Setup
//	@Accept			json
//	@Produce		json
//	@Param			request	body	setupRequest	true	"Admin credentials"
//	@Success		200	{object}	object{ok=bool}
//	@Failure		400	{object}	ErrorResponse
//	@Failure		409	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/setup [post]
func (h *Handler) PostSetup(c *gin.Context) {
	// Serialise concurrent setup requests to prevent TOCTOU race (OWASP A01).
	h.mu.Lock()
	defer h.mu.Unlock()

	ctx := c.Request.Context()
	requestID, _ := c.Get("requestID")

	state, err := h.queries.GetAppState(ctx)
	if err != nil {
		slog.Error("setup: failed to read app state", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read app state"})
		return
	}
	if state.SetupComplete != 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "setup already complete"})
		return
	}

	var req setupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must not be empty"})
		return
	}
	if len(req.Username) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be at most 64 characters"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}
	if len(req.Password) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at most 128 characters"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("setup: failed to hash password", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	now := time.Now().Unix()
	if _, err = h.queries.CreateUser(ctx, db.CreateUserParams{
		Username:           req.Username,
		PasswordHash:       hash,
		Role:               auth.RoleAdmin,
		Disabled:           0,
		SystemsJson:        sql.NullString{},
		Expiration:         sql.NullInt64{},
		Limit:              sql.NullInt64{},
		PasswordNeedChange: 0,
		CreatedAt:          now,
		UpdatedAt:          now,
	}); err != nil {
		slog.Error("setup: failed to create initial admin user", "request_id", requestID, "username", req.Username, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	if err := h.queries.SetSetupComplete(ctx, 1); err != nil {
		slog.Error("setup: failed to mark setup complete", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark setup complete"})
		return
	}

	slog.Info("setup: initial admin account created", "request_id", requestID, "username", req.Username)

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
