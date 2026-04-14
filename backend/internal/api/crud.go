package api

import (
	"database/sql"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/ws"
)

const maxImportRows = 100_000 // CSV import safety limit.

// validRoles is the set of allowed user roles.
var validRoles = map[string]bool{
	auth.RoleAdmin:    true,
	auth.RoleListener: true,
}

// AdminHandler handles admin CRUD endpoints.
type AdminHandler struct {
	queries       *db.Queries
	hub           *ws.Hub
	sqlDB         *sql.DB
	dwReload      DirwatchReloader
	dsReload      DownstreamReloader
	recordingsDir string
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(queries *db.Queries, hub *ws.Hub, sqlDB *sql.DB, dwReload DirwatchReloader, dsReload DownstreamReloader, recordingsDir ...string) *AdminHandler {
	rd := "."
	if len(recordingsDir) > 0 && strings.TrimSpace(recordingsDir[0]) != "" {
		rd = recordingsDir[0]
	}
	return &AdminHandler{queries: queries, hub: hub, sqlDB: sqlDB, dwReload: dwReload, dsReload: dsReload, recordingsDir: rd}
}

// parseID extracts and parses the :id path parameter.
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

// isUniqueViolation checks if an error is a SQLite UNIQUE constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE")
}

// validHTTPURL checks that a URL string parses as a valid http or https URL
// (defence-in-depth against SSRF via file://, gopher://, etc.).
func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// ---------- Users ----------

type createUserRequest struct {
	Username    string  `json:"username"`
	Password    string  `json:"password"`
	Role        string  `json:"role"`
	Disabled    int64   `json:"disabled"`
	SystemsJson *string `json:"systemsJson"`
	Expiration  *int64  `json:"expiration"`
	Limit       *int64  `json:"limit"`
} // @name CreateUserRequest

type updateUserRequest struct {
	Username    string  `json:"username"`
	Role        string  `json:"role"`
	Disabled    int64   `json:"disabled"`
	SystemsJson *string `json:"systemsJson"`
	Expiration  *int64  `json:"expiration"`
	Limit       *int64  `json:"limit"`
} // @name UpdateUserRequest

type serverDirectoryEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
} // @name ServerDirectoryEntry

type listServerDirectoriesResponse struct {
	Path        string                 `json:"path"`
	Parent      *string                `json:"parent"`
	Directories []serverDirectoryEntry `json:"directories"`
} // @name ListServerDirectoriesResponse

// hiddenTopLevelDirs are directories excluded from the root listing because
// they are kernel/system virtual filesystems that are not useful for selecting
// a recording source.
var hiddenTopLevelDirs = map[string]bool{
	"bin": true, "boot": true, "dev": true, "lib": true,
	"lib32": true, "lib64": true, "libx32": true,
	"proc": true, "run": true, "sbin": true, "sys": true,
	"usr": true, "etc": true, "snap": true, "lost+found": true,
}

// ListServerDirectories handles GET /api/admin/fs/directories.
// Returns immediate child directories for a given absolute server path.
// This endpoint is admin-only (JWT-protected) so the admin can navigate
// to wherever their recorder (SDR Trunk, trunk-recorder, etc.) writes files.
//
// @Summary  List server directories
// @Description  Returns immediate child directories for a given absolute server path.
// @Tags     Admin
// @Produce  json
// @Param    path  query  string  false  "Absolute directory path"  default(/)
// @Success  200  {object}  listServerDirectoriesResponse
// @Failure  422  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/fs/directories [get]
func (h *AdminHandler) ListServerDirectories(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "path must be absolute"})
		return
	}

	info, err := os.Stat(clean)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "directory does not exist or is not accessible: " + err.Error(),
		})
		return
	}
	if !info.IsDir() {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "path is not a directory: " + clean})
		return
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "failed to read directory: " + err.Error(),
		})
		return
	}

	dirs := make([]serverDirectoryEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// At root level, hide kernel/system virtual directories.
		if clean == "/" && hiddenTopLevelDirs[name] {
			continue
		}
		// Skip hidden directories (dotfiles).
		if strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, serverDirectoryEntry{
			Name: name,
			Path: filepath.Join(clean, name),
		})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	var parent *string
	if clean != "/" {
		p := filepath.Dir(clean)
		parent = &p
	}

	c.JSON(http.StatusOK, listServerDirectoriesResponse{
		Path:        clean,
		Parent:      parent,
		Directories: dirs,
	})
}

// ListUsers handles GET /api/admin/users.
//
// @Summary  List users
// @Description  Returns all users.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   userResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.queries.ListUsers(c.Request.Context())
	if err != nil {
		slog.Error("failed to list users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}
	c.JSON(http.StatusOK, toUserResponses(users))
}

// CreateUser handles POST /api/admin/users.
//
// @Summary  Create a user
// @Description  Creates a new user account.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      createUserRequest  true  "User to create"
// @Success  201   {object}  userResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/users [post]
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Username == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "username is required"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "password must be at least 8 characters"})
		return
	}
	if req.Role == "" {
		req.Role = "listener"
	}
	if !validRoles[req.Role] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "role must be 'admin' or 'listener'"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	now := time.Now().Unix()
	params := db.CreateUserParams{
		Username:           req.Username,
		PasswordHash:       hash,
		Role:               req.Role,
		Disabled:           req.Disabled,
		SystemsJson:        ptrToNullStr(req.SystemsJson),
		Expiration:         ptrToNullInt(req.Expiration),
		Limit:              ptrToNullInt(req.Limit),
		PasswordNeedChange: 1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	id, err := h.queries.CreateUser(c.Request.Context(), params)
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to create user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	user, err := h.queries.GetUser(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created user"})
		return
	}
	c.JSON(http.StatusCreated, toUserResponse(user))
}

// UpdateUser handles PUT /api/admin/users/:id.
//
// @Summary  Update a user
// @Description  Updates an existing user by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int                true  "User ID"
// @Param    body  body      updateUserRequest  true  "User fields to update"
// @Success  200   {object}  userResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/users/{id} [put]
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Username == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "username is required"})
		return
	}
	if req.Role == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "role is required"})
		return
	}
	if !validRoles[req.Role] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "role must be 'admin' or 'listener'"})
		return
	}

	if _, err := h.queries.GetUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Prevent disabling the bootstrap admin (id=1).
	if id == 1 && req.Disabled != 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot disable the primary admin account"})
		return
	}
	// Prevent changing role, expiration, or limit for the bootstrap admin (id=1).
	if id == 1 {
		req.Role = "admin"
		req.Expiration = nil
		req.Limit = nil
	}

	params := db.UpdateUserParams{
		ID:          id,
		Username:    req.Username,
		Role:        req.Role,
		Disabled:    req.Disabled,
		SystemsJson: ptrToNullStr(req.SystemsJson),
		Expiration:  ptrToNullInt(req.Expiration),
		Limit:       ptrToNullInt(req.Limit),
		UpdatedAt:   time.Now().Unix(),
	}

	err := h.queries.UpdateUser(c.Request.Context(), params)
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to update user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}

	// Revoke all tokens so stale claims (role, disabled, expiration, grants)
	// are not trusted after an admin update.
	auth.Tokens.RevokeAllForUser(id)

	user, err := h.queries.GetUser(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated user"})
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

// DeleteUser handles DELETE /api/admin/users/:id.
//
// @Summary  Delete a user
// @Description  Deletes a user by ID. Cannot delete your own account or the primary admin.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "User ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  403  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/users/{id} [delete]
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	userIDVal, _ := c.Get("userID")
	if currentID, ok := userIDVal.(int64); ok && currentID == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete your own account"})
		return
	}

	// Prevent deleting the bootstrap admin (id=1).
	if id == 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete the primary admin account"})
		return
	}

	if _, err := h.queries.GetUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := h.queries.DeleteUser(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- Systems ----------

// ListSystems handles GET /api/admin/systems.
//
// @Summary  List systems
// @Description  Returns all systems.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   systemResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/systems [get]
func (h *AdminHandler) ListSystems(c *gin.Context) {
	systems, err := h.queries.ListSystems(c.Request.Context())
	if err != nil {
		slog.Error("failed to list systems", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list systems"})
		return
	}
	c.JSON(http.StatusOK, toSystemResponses(systems))
}

// CreateSystem handles POST /api/admin/systems.
//
// @Summary  Create a system
// @Description  Creates a new radio system.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      createSystemRequest  true  "System to create"
// @Success  201   {object}  systemResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/systems [post]
func (h *AdminHandler) CreateSystem(c *gin.Context) {
	var req createSystemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	id, err := h.queries.CreateSystem(c.Request.Context(), req.toParams())
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "system_id already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to create system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create system"})
		return
	}

	system, err := h.queries.GetSystem(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created system"})
		return
	}
	c.JSON(http.StatusCreated, toSystemResponse(system))
}

// UpdateSystem handles PUT /api/admin/systems/:id.
//
// @Summary  Update a system
// @Description  Updates an existing system by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int                  true  "System ID"
// @Param    body  body      updateSystemRequest  true  "System fields to update"
// @Success  200   {object}  systemResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/systems/{id} [put]
func (h *AdminHandler) UpdateSystem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetSystem(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "system not found"})
		return
	}

	var req updateSystemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.queries.UpdateSystem(c.Request.Context(), req.toParams(id))
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "system_id already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to update system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update system"})
		return
	}

	system, err := h.queries.GetSystem(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated system"})
		return
	}
	c.JSON(http.StatusOK, toSystemResponse(system))
}

// ReorderSystems handles PUT /api/admin/systems/reorder.
// Applies all order updates in one transaction to avoid many per-row updates.
//
// @Summary  Reorder systems
// @Description  Updates the display order of multiple systems in a single transaction.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      reorderSystemsRequest  true  "Systems with new order values"
// @Success  200   {object}  object{ok=bool}
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/systems/reorder [put]
func (h *AdminHandler) ReorderSystems(c *gin.Context) {
	var req reorderSystemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(req.Systems) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "systems is required"})
		return
	}

	ctx := c.Request.Context()
	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin systems reorder transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder systems"})
		return
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := h.queries.WithTx(tx)
	for _, item := range req.Systems {
		sys, err := qtx.GetSystem(ctx, item.ID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "system not found"})
				return
			}
			slog.Error("failed to load system for reorder", "id", item.ID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder systems"})
			return
		}

		err = qtx.UpdateSystem(ctx, db.UpdateSystemParams{
			ID:             sys.ID,
			SystemID:       sys.SystemID,
			Label:          sys.Label,
			AutoPopulate:   sys.AutoPopulate,
			BlacklistsJson: sys.BlacklistsJson,
			Led:            sys.Led,
			Order:          item.Order,
		})
		if err != nil {
			slog.Error("failed to update system order", "id", item.ID, "order", item.Order, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder systems"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit systems reorder transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder systems"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteSystem handles DELETE /api/admin/systems/:id.
//
// @Summary  Delete a system
// @Description  Deletes a system by ID.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "System ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/systems/{id} [delete]
func (h *AdminHandler) DeleteSystem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetSystem(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "system not found"})
		return
	}

	if err := h.queries.DeleteSystem(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete system"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- Talkgroups ----------

// ListTalkgroups handles GET /api/admin/talkgroups.
//
// @Summary  List talkgroups
// @Description  Returns all talkgroups.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   talkgroupResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/talkgroups [get]
func (h *AdminHandler) ListTalkgroups(c *gin.Context) {
	talkgroups, err := h.queries.ListAllTalkgroups(c.Request.Context())
	if err != nil {
		slog.Error("failed to list talkgroups", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list talkgroups"})
		return
	}
	c.JSON(http.StatusOK, toTalkgroupResponses(talkgroups))
}

// CreateTalkgroup handles POST /api/admin/talkgroups.
//
// @Summary  Create a talkgroup
// @Description  Creates a new talkgroup.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      createTalkgroupRequest  true  "Talkgroup to create"
// @Success  201   {object}  talkgroupResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/talkgroups [post]
func (h *AdminHandler) CreateTalkgroup(c *gin.Context) {
	var req createTalkgroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	id, err := h.queries.CreateTalkgroup(c.Request.Context(), req.toParams())
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "talkgroup already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to create talkgroup", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create talkgroup"})
		return
	}

	tg, err := h.queries.GetTalkgroup(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created talkgroup", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created talkgroup"})
		return
	}
	c.JSON(http.StatusCreated, toTalkgroupResponse(tg))
}

// UpdateTalkgroup handles PUT /api/admin/talkgroups/:id.
//
// @Summary  Update a talkgroup
// @Description  Updates an existing talkgroup by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int                     true  "Talkgroup ID"
// @Param    body  body      updateTalkgroupRequest  true  "Talkgroup fields to update"
// @Success  200   {object}  talkgroupResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/talkgroups/{id} [put]
func (h *AdminHandler) UpdateTalkgroup(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetTalkgroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "talkgroup not found"})
		return
	}

	var req updateTalkgroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.queries.UpdateTalkgroup(c.Request.Context(), req.toParams(id))
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "talkgroup already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to update talkgroup", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update talkgroup"})
		return
	}

	tg, err := h.queries.GetTalkgroup(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated talkgroup", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated talkgroup"})
		return
	}
	c.JSON(http.StatusOK, toTalkgroupResponse(tg))
}

// DeleteTalkgroup handles DELETE /api/admin/talkgroups/:id.
//
// @Summary  Delete a talkgroup
// @Description  Deletes a talkgroup by ID.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "Talkgroup ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/talkgroups/{id} [delete]
func (h *AdminHandler) DeleteTalkgroup(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetTalkgroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "talkgroup not found"})
		return
	}

	if err := h.queries.DeleteTalkgroup(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete talkgroup", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete talkgroup"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- Units ----------

// ListUnits handles GET /api/admin/units.
//
// @Summary  List units
// @Description  Returns all units.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   unitResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/units [get]
func (h *AdminHandler) ListUnits(c *gin.Context) {
	units, err := h.queries.ListAllUnits(c.Request.Context())
	if err != nil {
		slog.Error("failed to list units", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list units"})
		return
	}
	c.JSON(http.StatusOK, toUnitResponses(units))
}

// CreateUnit handles POST /api/admin/units.
//
// @Summary  Create a unit
// @Description  Creates a new unit.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      createUnitRequest  true  "Unit to create"
// @Success  201   {object}  unitResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/units [post]
func (h *AdminHandler) CreateUnit(c *gin.Context) {
	var req createUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	id, err := h.queries.CreateUnit(c.Request.Context(), req.toParams())
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "unit already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to create unit", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create unit"})
		return
	}

	unit, err := h.queries.GetUnit(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created unit", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created unit"})
		return
	}
	c.JSON(http.StatusCreated, toUnitResponse(unit))
}

// UpdateUnit handles PUT /api/admin/units/:id.
//
// @Summary  Update a unit
// @Description  Updates an existing unit by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int                true  "Unit ID"
// @Param    body  body      updateUnitRequest  true  "Unit fields to update"
// @Success  200   {object}  unitResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/units/{id} [put]
func (h *AdminHandler) UpdateUnit(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetUnit(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unit not found"})
		return
	}

	var req updateUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.queries.UpdateUnit(c.Request.Context(), req.toParams(id))
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "unit already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to update unit", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update unit"})
		return
	}

	unit, err := h.queries.GetUnit(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated unit", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated unit"})
		return
	}
	c.JSON(http.StatusOK, toUnitResponse(unit))
}

// DeleteUnit handles DELETE /api/admin/units/:id.
//
// @Summary  Delete a unit
// @Description  Deletes a unit by ID.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "Unit ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/units/{id} [delete]
func (h *AdminHandler) DeleteUnit(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetUnit(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unit not found"})
		return
	}

	if err := h.queries.DeleteUnit(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete unit", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete unit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- Groups ----------

type groupRequest struct {
	Label string `json:"label"`
} // @name GroupRequest

// ListGroups handles GET /api/admin/groups.
//
// @Summary  List groups
// @Description  Returns all groups.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   db.Group
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/groups [get]
func (h *AdminHandler) ListGroups(c *gin.Context) {
	groups, err := h.queries.ListGroups(c.Request.Context())
	if err != nil {
		slog.Error("failed to list groups", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list groups"})
		return
	}
	c.JSON(http.StatusOK, groups)
}

// CreateGroup handles POST /api/admin/groups.
//
// @Summary  Create a group
// @Description  Creates a new group.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      groupRequest  true  "Group to create"
// @Success  201   {object}  db.Group
// @Failure  400   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/groups [post]
func (h *AdminHandler) CreateGroup(c *gin.Context) {
	var req groupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Label == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "label is required"})
		return
	}

	id, err := h.queries.CreateGroup(c.Request.Context(), req.Label)
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "group label already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to create group", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create group"})
		return
	}

	group, err := h.queries.GetGroup(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created group", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created group"})
		return
	}
	c.JSON(http.StatusCreated, group)
}

// UpdateGroup handles PUT /api/admin/groups/:id.
//
// @Summary  Update a group
// @Description  Updates an existing group by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int           true  "Group ID"
// @Param    body  body      groupRequest  true  "Group fields to update"
// @Success  200   {object}  db.Group
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/groups/{id} [put]
func (h *AdminHandler) UpdateGroup(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetGroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	var req groupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Label == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "label is required"})
		return
	}

	err := h.queries.UpdateGroup(c.Request.Context(), db.UpdateGroupParams{ID: id, Label: req.Label})
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "group label already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to update group", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update group"})
		return
	}

	group, err := h.queries.GetGroup(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated group", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated group"})
		return
	}
	c.JSON(http.StatusOK, group)
}

// DeleteGroup handles DELETE /api/admin/groups/:id.
//
// @Summary  Delete a group
// @Description  Deletes a group by ID.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "Group ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/groups/{id} [delete]
func (h *AdminHandler) DeleteGroup(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetGroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	if err := h.queries.DeleteGroup(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete group", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete group"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- Tags ----------

type tagRequest struct {
	Label string `json:"label"`
} // @name TagRequest

// ListTags handles GET /api/admin/tags.
//
// @Summary  List tags
// @Description  Returns all tags.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   db.Tag
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/tags [get]
func (h *AdminHandler) ListTags(c *gin.Context) {
	tags, err := h.queries.ListTags(c.Request.Context())
	if err != nil {
		slog.Error("failed to list tags", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tags"})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// CreateTag handles POST /api/admin/tags.
//
// @Summary  Create a tag
// @Description  Creates a new tag.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      tagRequest  true  "Tag to create"
// @Success  201   {object}  db.Tag
// @Failure  400   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/tags [post]
func (h *AdminHandler) CreateTag(c *gin.Context) {
	var req tagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Label == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "label is required"})
		return
	}

	id, err := h.queries.CreateTag(c.Request.Context(), req.Label)
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "tag label already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to create tag", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tag"})
		return
	}

	tag, err := h.queries.GetTag(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created tag", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created tag"})
		return
	}
	c.JSON(http.StatusCreated, tag)
}

// UpdateTag handles PUT /api/admin/tags/:id.
//
// @Summary  Update a tag
// @Description  Updates an existing tag by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int         true  "Tag ID"
// @Param    body  body      tagRequest  true  "Tag fields to update"
// @Success  200   {object}  db.Tag
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/tags/{id} [put]
func (h *AdminHandler) UpdateTag(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetTag(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}

	var req tagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Label == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "label is required"})
		return
	}

	err := h.queries.UpdateTag(c.Request.Context(), db.UpdateTagParams{ID: id, Label: req.Label})
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "tag label already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to update tag", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tag"})
		return
	}

	tag, err := h.queries.GetTag(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated tag", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated tag"})
		return
	}
	c.JSON(http.StatusOK, tag)
}

// DeleteTag handles DELETE /api/admin/tags/:id.
//
// @Summary  Delete a tag
// @Description  Deletes a tag by ID.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "Tag ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/tags/{id} [delete]
func (h *AdminHandler) DeleteTag(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetTag(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}

	if err := h.queries.DeleteTag(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete tag", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete tag"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- API Keys ----------

// ListAPIKeys handles GET /api/admin/apikeys.
//
// @Summary  List API keys
// @Description  Returns all API keys.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   apiKeyResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/apikeys [get]
func (h *AdminHandler) ListAPIKeys(c *gin.Context) {
	keys, err := h.queries.ListAPIKeys(c.Request.Context())
	if err != nil {
		slog.Error("failed to list API keys", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list API keys"})
		return
	}
	c.JSON(http.StatusOK, toAPIKeyResponses(keys))
}

// CreateAPIKey handles POST /api/admin/apikeys.
//
// @Summary  Create an API key
// @Description  Creates a new API key and returns the plain-text key once.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      createAPIKeyRequest  true  "API key to create"
// @Success  201   {object}  apiKeyCreateResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/apikeys [post]
func (h *AdminHandler) CreateAPIKey(c *gin.Context) {
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	plainKey := uuid.New().String()
	if req.Key != nil && *req.Key != "" {
		plainKey = *req.Key
	}
	hashedKey := auth.HashAPIKey(plainKey)

	id, err := h.queries.CreateAPIKey(c.Request.Context(), req.toParams(hashedKey))
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "API key already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to create API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
		return
	}

	key, err := h.queries.GetAPIKey(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created API key"})
		return
	}
	c.JSON(http.StatusCreated, apiKeyCreateResponse{
		apiKeyResponse: toAPIKeyResponse(key),
		CreatedKey:     plainKey,
	})
}

// UpdateAPIKey handles PUT /api/admin/apikeys/:id.
//
// @Summary  Update an API key
// @Description  Updates an existing API key by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int                  true  "API key ID"
// @Param    body  body      updateAPIKeyRequest  true  "API key fields to update"
// @Success  200   {object}  apiKeyResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  409   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/apikeys/{id} [put]
func (h *AdminHandler) UpdateAPIKey(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetAPIKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	var req updateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	current, err := h.queries.GetAPIKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	keyHash := current.Key
	if req.Key != nil && *req.Key != "" {
		keyHash = auth.HashAPIKey(*req.Key)
	}

	err = h.queries.UpdateAPIKey(c.Request.Context(), db.UpdateAPIKeyParams{
		ID:          id,
		Key:         keyHash,
		Ident:       ptrToNullStr(req.Ident),
		Disabled:    req.Disabled,
		SystemsJson: ptrToNullStr(req.SystemsJson),
		Order:       req.Order,
	})
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "API key already exists"})
		return
	}
	if err != nil {
		slog.Error("failed to update API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update API key"})
		return
	}

	key, err := h.queries.GetAPIKey(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated API key"})
		return
	}
	c.JSON(http.StatusOK, toAPIKeyResponse(key))
}

// ReorderAPIKeys handles PUT /api/admin/apikeys/reorder.
// Applies all order updates in one transaction.
//
// @Summary  Reorder API keys
// @Description  Updates the display order of multiple API keys in one transaction.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      reorderAPIKeysRequest  true  "API key order updates"
// @Success  200   {object}  object{ok=bool}
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/apikeys/reorder [put]
func (h *AdminHandler) ReorderAPIKeys(c *gin.Context) {
	var req reorderAPIKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(req.APIKeys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "apiKeys is required"})
		return
	}

	ctx := c.Request.Context()
	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin API key reorder transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder API keys"})
		return
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := h.queries.WithTx(tx)
	for _, item := range req.APIKeys {
		ak, err := qtx.GetAPIKey(ctx, item.ID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
				return
			}
			slog.Error("failed to load API key for reorder", "id", item.ID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder API keys"})
			return
		}

		err = qtx.UpdateAPIKey(ctx, db.UpdateAPIKeyParams{
			ID:          ak.ID,
			Key:         ak.Key,
			Ident:       ak.Ident,
			Disabled:    ak.Disabled,
			SystemsJson: ak.SystemsJson,
			Order:       item.Order,
		})
		if err != nil {
			slog.Error("failed to update API key order", "id", item.ID, "order", item.Order, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder API keys"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit API key reorder transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder API keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteAPIKey handles DELETE /api/admin/apikeys/:id.
//
// @Summary  Delete an API key
// @Description  Deletes an API key by ID.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "API key ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/apikeys/{id} [delete]
func (h *AdminHandler) DeleteAPIKey(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetAPIKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	if err := h.queries.DeleteAPIKey(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete API key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- Dirwatches ----------

// ListDirwatches handles GET /api/admin/dirwatches.
//
// @Summary  List directory watches
// @Description  Returns all directory watches.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   dirwatchResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/dirwatches [get]
func (h *AdminHandler) ListDirwatches(c *gin.Context) {
	dirwatches, err := h.queries.ListDirwatches(c.Request.Context())
	if err != nil {
		slog.Error("failed to list dirwatches", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list dirwatches"})
		return
	}
	c.JSON(http.StatusOK, toDirwatchResponses(dirwatches))
}

// CreateDirwatch handles POST /api/admin/dirwatches.
//
// @Summary  Create a directory watch
// @Description  Creates a new directory watch.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      createDirwatchRequest  true  "Dirwatch to create"
// @Success  201   {object}  dirwatchResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/dirwatches [post]
func (h *AdminHandler) CreateDirwatch(c *gin.Context) {
	var req createDirwatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Directory == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "directory is required"})
		return
	}
	if info, statErr := os.Stat(req.Directory); statErr != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "directory does not exist or is not accessible: " + statErr.Error(),
		})
		return
	} else if !info.IsDir() {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "path is not a directory: " + req.Directory,
		})
		return
	}

	id, err := h.queries.CreateDirwatch(c.Request.Context(), req.toParams())
	if err != nil {
		slog.Error("failed to create dirwatch", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create dirwatch"})
		return
	}

	dw, err := h.queries.GetDirwatch(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created dirwatch", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created dirwatch"})
		return
	}
	// Reload the dirwatch service so the new config takes effect immediately.
	if h.dwReload != nil {
		h.dwReload.Reload()
	}
	c.JSON(http.StatusCreated, toDirwatchResponse(dw))
}

// UpdateDirwatch handles PUT /api/admin/dirwatches/:id.
//
// @Summary  Update a directory watch
// @Description  Updates an existing directory watch by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int                    true  "Dirwatch ID"
// @Param    body  body      updateDirwatchRequest  true  "Dirwatch fields to update"
// @Success  200   {object}  dirwatchResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/dirwatches/{id} [put]
func (h *AdminHandler) UpdateDirwatch(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetDirwatch(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dirwatch not found"})
		return
	}

	var req updateDirwatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Directory == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "directory is required"})
		return
	}
	if info, statErr := os.Stat(req.Directory); statErr != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "directory does not exist or is not accessible: " + statErr.Error(),
		})
		return
	} else if !info.IsDir() {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "path is not a directory: " + req.Directory,
		})
		return
	}

	if err := h.queries.UpdateDirwatch(c.Request.Context(), req.toParams(id)); err != nil {
		slog.Error("failed to update dirwatch", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update dirwatch"})
		return
	}

	dw, err := h.queries.GetDirwatch(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated dirwatch", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated dirwatch"})
		return
	}
	// Reload the dirwatch service so the updated config takes effect immediately.
	if h.dwReload != nil {
		h.dwReload.Reload()
	}
	c.JSON(http.StatusOK, toDirwatchResponse(dw))
}

// DeleteDirwatch handles DELETE /api/admin/dirwatches/:id.
//
// @Summary  Delete a directory watch
// @Description  Deletes a directory watch by ID.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "Dirwatch ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/dirwatches/{id} [delete]
func (h *AdminHandler) DeleteDirwatch(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetDirwatch(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dirwatch not found"})
		return
	}

	if err := h.queries.DeleteDirwatch(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete dirwatch", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete dirwatch"})
		return
	}
	// Reload the dirwatch service so the deleted watcher is stopped immediately.
	if h.dwReload != nil {
		h.dwReload.Reload()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- Downstreams ----------

// ListDownstreams handles GET /api/admin/downstreams.
//
// @Summary  List downstreams
// @Description  Returns all downstream push targets.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   downstreamResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/downstreams [get]
func (h *AdminHandler) ListDownstreams(c *gin.Context) {
	downstreams, err := h.queries.ListDownstreams(c.Request.Context())
	if err != nil {
		slog.Error("failed to list downstreams", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list downstreams"})
		return
	}
	c.JSON(http.StatusOK, toDownstreamResponses(downstreams))
}

// CreateDownstream handles POST /api/admin/downstreams.
//
// @Summary  Create a downstream
// @Description  Creates a new downstream push target.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      createDownstreamRequest  true  "Downstream to create"
// @Success  201   {object}  downstreamResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/downstreams [post]
func (h *AdminHandler) CreateDownstream(c *gin.Context) {
	var req createDownstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Url == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "url is required"})
		return
	}
	if !validHTTPURL(req.Url) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "url must use http or https scheme"})
		return
	}

	id, err := h.queries.CreateDownstream(c.Request.Context(), req.toParams())
	if err != nil {
		slog.Error("failed to create downstream", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create downstream"})
		return
	}

	ds, err := h.queries.GetDownstream(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created downstream", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created downstream"})
		return
	}
	// Reload the downstream service so the new config takes effect immediately.
	if h.dsReload != nil {
		h.dsReload.Reload()
	}
	c.JSON(http.StatusCreated, toDownstreamResponse(ds))
}

// UpdateDownstream handles PUT /api/admin/downstreams/:id.
//
// @Summary  Update a downstream
// @Description  Updates an existing downstream push target by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int                      true  "Downstream ID"
// @Param    body  body      updateDownstreamRequest  true  "Downstream fields to update"
// @Success  200   {object}  downstreamResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/downstreams/{id} [put]
func (h *AdminHandler) UpdateDownstream(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetDownstream(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "downstream not found"})
		return
	}

	var req updateDownstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Url != "" && !validHTTPURL(req.Url) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "url must use http or https scheme"})
		return
	}

	if err := h.queries.UpdateDownstream(c.Request.Context(), req.toParams(id)); err != nil {
		slog.Error("failed to update downstream", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update downstream"})
		return
	}

	ds, err := h.queries.GetDownstream(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated downstream", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated downstream"})
		return
	}
	// Reload the downstream service so the updated config takes effect immediately.
	if h.dsReload != nil {
		h.dsReload.Reload()
	}
	c.JSON(http.StatusOK, toDownstreamResponse(ds))
}

// DeleteDownstream handles DELETE /api/admin/downstreams/:id.
//
// @Summary  Delete a downstream
// @Description  Deletes a downstream push target by ID.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "Downstream ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/downstreams/{id} [delete]
func (h *AdminHandler) DeleteDownstream(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetDownstream(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "downstream not found"})
		return
	}

	if err := h.queries.DeleteDownstream(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete downstream", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete downstream"})
		return
	}
	// Reload the downstream service so the deleted pusher is stopped immediately.
	if h.dsReload != nil {
		h.dsReload.Reload()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- Webhooks ----------

// ListWebhooks handles GET /api/admin/webhooks.
//
// @Summary  List webhooks
// @Description  Returns all webhooks.
// @Tags     Admin
// @Produce  json
// @Success  200  {array}   webhookResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/webhooks [get]
func (h *AdminHandler) ListWebhooks(c *gin.Context) {
	webhooks, err := h.queries.ListWebhooks(c.Request.Context())
	if err != nil {
		slog.Error("failed to list webhooks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list webhooks"})
		return
	}
	c.JSON(http.StatusOK, toWebhookResponses(webhooks))
}

// CreateWebhook handles POST /api/admin/webhooks.
//
// @Summary  Create a webhook
// @Description  Creates a new webhook.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    body  body      createWebhookRequest  true  "Webhook to create"
// @Success  201   {object}  webhookResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/webhooks [post]
func (h *AdminHandler) CreateWebhook(c *gin.Context) {
	var req createWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Url == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "url is required"})
		return
	}
	if !validHTTPURL(req.Url) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "url must use http or https scheme"})
		return
	}

	id, err := h.queries.CreateWebhook(c.Request.Context(), req.toParams())
	if err != nil {
		slog.Error("failed to create webhook", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create webhook"})
		return
	}

	wh, err := h.queries.GetWebhook(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created webhook", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created webhook"})
		return
	}
	c.JSON(http.StatusCreated, toWebhookResponse(wh))
}

// UpdateWebhook handles PUT /api/admin/webhooks/:id.
//
// @Summary  Update a webhook
// @Description  Updates an existing webhook by ID.
// @Tags     Admin
// @Accept   json
// @Produce  json
// @Param    id    path      int                   true  "Webhook ID"
// @Param    body  body      updateWebhookRequest  true  "Webhook fields to update"
// @Success  200   {object}  webhookResponse
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse
// @Failure  422   {object}  ErrorResponse
// @Failure  500   {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/webhooks/{id} [put]
func (h *AdminHandler) UpdateWebhook(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetWebhook(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}

	var req updateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Url != "" && !validHTTPURL(req.Url) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "url must use http or https scheme"})
		return
	}

	if err := h.queries.UpdateWebhook(c.Request.Context(), req.toParams(id)); err != nil {
		slog.Error("failed to update webhook", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update webhook"})
		return
	}

	wh, err := h.queries.GetWebhook(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated webhook", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated webhook"})
		return
	}
	c.JSON(http.StatusOK, toWebhookResponse(wh))
}

// DeleteWebhook handles DELETE /api/admin/webhooks/:id.
//
// @Summary  Delete a webhook
// @Description  Deletes a webhook by ID.
// @Tags     Admin
// @Produce  json
// @Param    id   path      int  true  "Webhook ID"
// @Success  200  {object}  object{ok=bool}
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  500  {object}  ErrorResponse
// @Security BearerAuth
// @Router   /admin/webhooks/{id} [delete]
func (h *AdminHandler) DeleteWebhook(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetWebhook(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}

	if err := h.queries.DeleteWebhook(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete webhook", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete webhook"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
