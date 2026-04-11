// Package api — first-run setup endpoints (POST /api/setup, GET /api/setup/status).
package api

import (
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// SetupHandler holds dependencies for first-run setup endpoints.
type SetupHandler struct {
	queries *db.Queries
	mu      sync.Mutex // guards the check-then-create in PostSetup (TOCTOU prevention)
}

// NewSetupHandler constructs a SetupHandler.
func NewSetupHandler(queries *db.Queries) *SetupHandler {
	return &SetupHandler{queries: queries}
}

type setupStatusResponse struct {
	NeedsSetup   bool `json:"needsSetup"`
	PublicAccess bool `json:"publicAccess"`
}

// GetSetupStatus handles GET /api/setup/status.
// Returns whether initial setup is needed and whether public access is active.
// This endpoint is always unauthenticated.
func (h *SetupHandler) GetSetupStatus(c *gin.Context) {
	ctx := c.Request.Context()

	state, err := h.queries.GetAppState(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read app state"})
		return
	}

	publicAccess := false
	setting, err := h.queries.GetSetting(ctx, "publicAccess")
	if err == nil {
		publicAccess = setting.Value == "true"
	} else if !errors.Is(err, sql.ErrNoRows) {
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
}

// PostSetup handles POST /api/setup.
// Creates the initial admin user and marks setup as complete.
// Returns 409 if setup is already done, 400 for invalid input.
func (h *SetupHandler) PostSetup(c *gin.Context) {
	// Serialise concurrent setup requests to prevent TOCTOU race (OWASP A01).
	h.mu.Lock()
	defer h.mu.Unlock()

	ctx := c.Request.Context()

	state, err := h.queries.GetAppState(ctx)
	if err != nil {
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
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	now := time.Now().Unix()
	if _, err = h.queries.CreateUser(ctx, db.CreateUserParams{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         auth.RoleAdmin,
		Disabled:     0,
		SystemsJson:  sql.NullString{},
		Expiration:   sql.NullInt64{},
		Limit:        sql.NullInt64{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	if err := h.queries.SetSetupComplete(ctx, 1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark setup complete"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
