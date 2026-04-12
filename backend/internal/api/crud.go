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
	queries  *db.Queries
	hub      *ws.Hub
	sqlDB    *sql.DB
	dwReload DirwatchReloader
	dsReload DownstreamReloader
	baseDir  string
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') {
			continue
		}
		return false
	}
	return true
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(queries *db.Queries, hub *ws.Hub, sqlDB *sql.DB, dwReload DirwatchReloader, dsReload DownstreamReloader, baseDir ...string) *AdminHandler {
	b := "."
	if len(baseDir) > 0 && strings.TrimSpace(baseDir[0]) != "" {
		b = baseDir[0]
	}
	return &AdminHandler{queries: queries, hub: hub, sqlDB: sqlDB, dwReload: dwReload, dsReload: dsReload, baseDir: b}
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
}

type updateUserRequest struct {
	Username    string  `json:"username"`
	Role        string  `json:"role"`
	Disabled    int64   `json:"disabled"`
	SystemsJson *string `json:"systemsJson"`
	Expiration  *int64  `json:"expiration"`
	Limit       *int64  `json:"limit"`
}

type serverDirectoryEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type listServerDirectoriesResponse struct {
	Path        string                 `json:"path"`
	Parent      *string                `json:"parent"`
	Directories []serverDirectoryEntry `json:"directories"`
}

// allowedBrowseRoots are the top-level directories that the admin FS browser
// is allowed to enumerate. This limits exposure if an admin account is
// compromised (defence-in-depth — OWASP A01).
var allowedBrowseRoots = []string{
	"/home",
	"/opt",
	"/srv",
	"/tmp",
	"/var",
	"/mnt",
	"/media",
}

// isUnderAllowedRoot checks whether a cleaned absolute path falls under one of
// the allowed browse roots.
func isUnderAllowedRoot(clean string) bool {
	for _, root := range allowedBrowseRoots {
		if clean == root || strings.HasPrefix(clean, root+"/") {
			return true
		}
	}
	return false
}

// ListServerDirectories handles GET /api/admin/fs/directories.
// Returns immediate child directories for a given absolute server path.
// Browsing is restricted to a set of safe directory roots.
func (h *AdminHandler) ListServerDirectories(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		// Return the list of allowed roots that actually exist.
		dirs := make([]serverDirectoryEntry, 0, len(allowedBrowseRoots))
		for _, root := range allowedBrowseRoots {
			if info, err := os.Stat(root); err == nil && info.IsDir() {
				dirs = append(dirs, serverDirectoryEntry{
					Name: filepath.Base(root),
					Path: root,
				})
			}
		}
		c.JSON(http.StatusOK, listServerDirectoriesResponse{
			Path:        "/",
			Parent:      nil,
			Directories: dirs,
		})
		return
	}

	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "path must be absolute"})
		return
	}

	if !isUnderAllowedRoot(clean) {
		c.JSON(http.StatusForbidden, gin.H{"error": "browsing this path is not allowed"})
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
		dirs = append(dirs, serverDirectoryEntry{
			Name: e.Name(),
			Path: filepath.Join(clean, e.Name()),
		})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	var parent *string
	p := filepath.Dir(clean)
	if isUnderAllowedRoot(p) {
		parent = &p
	} else {
		// Parent would be outside allowed roots — link back to the root listing.
		root := "/"
		parent = &root
	}

	c.JSON(http.StatusOK, listServerDirectoriesResponse{
		Path:        clean,
		Parent:      parent,
		Directories: dirs,
	})
}

// ListUsers handles GET /api/admin/users.
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

	user, err := h.queries.GetUser(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated user"})
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

// DeleteUser handles DELETE /api/admin/users/:id.
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
}

// ListGroups handles GET /api/admin/groups.
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
}

// ListTags handles GET /api/admin/tags.
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

// MigrateAPIKeysHashing handles POST /api/admin/apikeys/migrate-hash.
// It hashes legacy plaintext API keys in place and returns the migrated count.
func (h *AdminHandler) MigrateAPIKeysHashing(c *gin.Context) {
	ctx := c.Request.Context()
	keys, err := h.queries.ListAPIKeys(ctx)
	if err != nil {
		slog.Error("failed to list API keys for migration", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to migrate API keys"})
		return
	}

	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin API key migration transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to migrate API keys"})
		return
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := h.queries.WithTx(tx)
	migrated := 0
	for _, k := range keys {
		if isSHA256Hex(k.Key) {
			continue
		}

		err := qtx.UpdateAPIKey(ctx, db.UpdateAPIKeyParams{
			ID:          k.ID,
			Key:         auth.HashAPIKey(k.Key),
			Ident:       k.Ident,
			Disabled:    k.Disabled,
			SystemsJson: k.SystemsJson,
			Order:       k.Order,
		})
		if err != nil {
			slog.Error("failed to hash API key", "id", k.ID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to migrate API keys"})
			return
		}
		migrated++
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit API key migration transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to migrate API keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"migrated": migrated})
}

// DeleteAPIKey handles DELETE /api/admin/apikeys/:id.
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

// ---------- Accesses ----------

// ListAccesses handles GET /api/admin/accesses.
func (h *AdminHandler) ListAccesses(c *gin.Context) {
	accesses, err := h.queries.ListAccesses(c.Request.Context())
	if err != nil {
		slog.Error("failed to list accesses", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list accesses"})
		return
	}
	c.JSON(http.StatusOK, toAccessResponses(accesses))
}

// CreateAccess handles POST /api/admin/accesses.
func (h *AdminHandler) CreateAccess(c *gin.Context) {
	var req createAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Code == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "code is required"})
		return
	}

	id, err := h.queries.CreateAccess(c.Request.Context(), req.toParams())
	if err != nil {
		slog.Error("failed to create access", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create access"})
		return
	}

	access, err := h.queries.GetAccess(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch created access", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created access"})
		return
	}
	c.JSON(http.StatusCreated, toAccessResponse(access))
}

// UpdateAccess handles PUT /api/admin/accesses/:id.
func (h *AdminHandler) UpdateAccess(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetAccess(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "access not found"})
		return
	}

	var req updateAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.queries.UpdateAccess(c.Request.Context(), req.toParams(id)); err != nil {
		slog.Error("failed to update access", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update access"})
		return
	}

	access, err := h.queries.GetAccess(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to fetch updated access", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated access"})
		return
	}
	c.JSON(http.StatusOK, toAccessResponse(access))
}

// DeleteAccess handles DELETE /api/admin/accesses/:id.
func (h *AdminHandler) DeleteAccess(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetAccess(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "access not found"})
		return
	}

	if err := h.queries.DeleteAccess(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete access", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete access"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
