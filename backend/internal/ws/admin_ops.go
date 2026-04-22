// Package ws — admin CRUD operation handlers for the WebSocket protocol.
package ws

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/logging"
)

// ── Helpers ──

func wsPtrToNullStr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func wsPtrToNullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func wsNullStr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}

func wsNullInt(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

func wsIsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE")
}

func wsValidHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// validRoles is the set of allowed user roles.
var wsValidRoles = map[string]bool{
	auth.RoleAdmin:    true,
	auth.RoleListener: true,
}

// SensitiveSettingKeys are settings whose values are encrypted at rest.
var SensitiveSettingKeys = map[string]bool{
	"vapidPrivateKey": true,
	"jwtSecret":       true,
}

// wsAllowedSettingKeys mirrors the allowed setting keys from config.go.
var wsAllowedSettingKeys = map[string]bool{
	"activityDashboard":           true,
	"afsSystems":                  true,
	"apiKeyCallRate":              true,
	"audioConversion":             true,
	"audioEncodingPreset":         true,
	"autoPopulateSystems":         true,
	"branding":                    true,
	"disableDuplicateDetection":   true,
	"duplicateDetectionTimeFrame": true,
	"email":                       true,
	"keypadBeeps":                 true,
	"logLevel":                    true,
	"maxClients":                  true,
	"playbackGoesLive":            true,
	"pruneDays":                   true,
	"publicAccess":                true,
	"pushNotifications":           true,
	"searchPatchedTalkgroups":     true,
	"shareableLinks":              true,
	"sharedLinkExpiry":            true,
	"showListenersCount":          true,
	"sortTalkgroups":              true,
	"tagsToggle":                  true,
	"time12hFormat":               true,
	"transcriptionDiarize":        true,
	"transcriptionEnabled":        true,
	"transcriptionLanguage":       true,
	"liveTranscriptDisplay":       true,
	"transcriptionModel":          true,
	"transcriptionUrl":            true,
	"vapidPrivateKey":             true,
	"vapidPublicKey":              true,
	"webhooksEnabled":             true,
}

// hiddenTopLevelDirs for FS browsing.
var wsHiddenTopLevelDirs = map[string]bool{
	"bin": true, "boot": true, "dev": true, "lib": true,
	"lib32": true, "lib64": true, "libx32": true,
	"proc": true, "run": true, "sbin": true, "sys": true,
	"usr": true, "etc": true, "snap": true, "lost+found": true,
}

// ── Response mappers ──

func mapUser(u db.User) map[string]any {
	return map[string]any{
		"id":          u.ID,
		"username":    u.Username,
		"role":        u.Role,
		"disabled":    u.Disabled,
		"systemsJson": wsNullStr(u.SystemsJson),
		"expiration":  wsNullInt(u.Expiration),
		"limit":       wsNullInt(u.Limit),
		"createdAt":   u.CreatedAt,
		"updatedAt":   u.UpdatedAt,
	}
}

func mapUsers(users []db.User) []map[string]any {
	out := make([]map[string]any, len(users))
	for i, u := range users {
		out[i] = mapUser(u)
	}
	return out
}

func mapSystem(s db.System) map[string]any {
	return map[string]any{
		"id":                     s.ID,
		"systemId":               s.SystemID,
		"label":                  s.Label,
		"autoPopulateTalkgroups": s.AutoPopulateTalkgroups,
		"blacklistsJson":         wsNullStr(s.BlacklistsJson),
		"led":                    wsNullStr(s.Led),
		"order":                  s.Order,
	}
}

func mapSystems(systems []db.System) []map[string]any {
	out := make([]map[string]any, len(systems))
	for i, s := range systems {
		out[i] = mapSystem(s)
	}
	return out
}

func mapTalkgroup(t db.Talkgroup) map[string]any {
	return map[string]any{
		"id":          t.ID,
		"systemId":    t.SystemID,
		"talkgroupId": t.TalkgroupID,
		"label":       wsNullStr(t.Label),
		"name":        wsNullStr(t.Name),
		"frequency":   wsNullInt(t.Frequency),
		"led":         wsNullStr(t.Led),
		"groupId":     wsNullInt(t.GroupID),
		"tagId":       wsNullInt(t.TagID),
		"order":       t.Order,
	}
}

func mapTalkgroups(tgs []db.Talkgroup) []map[string]any {
	out := make([]map[string]any, len(tgs))
	for i, t := range tgs {
		out[i] = mapTalkgroup(t)
	}
	return out
}

func mapUnit(u db.Unit) map[string]any {
	return map[string]any{
		"id":       u.ID,
		"systemId": u.SystemID,
		"unitId":   u.UnitID,
		"label":    wsNullStr(u.Label),
		"order":    u.Order,
	}
}

func mapUnits(units []db.Unit) []map[string]any {
	out := make([]map[string]any, len(units))
	for i, u := range units {
		out[i] = mapUnit(u)
	}
	return out
}

func mapAPIKey(k db.ApiKey) map[string]any {
	fingerprint := auth.HashAPIKey(k.Key)
	if len(fingerprint) > 12 {
		fingerprint = fingerprint[:12]
	}
	return map[string]any{
		"id":            k.ID,
		"fingerprint":   fingerprint,
		"ident":         wsNullStr(k.Ident),
		"disabled":      k.Disabled,
		"systemsJson":   wsNullStr(k.SystemsJson),
		"callRateLimit": wsNullInt(k.CallRateLimit),
		"order":         k.Order,
	}
}

func mapAPIKeys(keys []db.ApiKey) []map[string]any {
	out := make([]map[string]any, len(keys))
	for i, k := range keys {
		out[i] = mapAPIKey(k)
	}
	return out
}

func mapDirMonitor(d db.Dirmonitor) map[string]any {
	return map[string]any{
		"id":          d.ID,
		"directory":   d.Directory,
		"type":        d.Type,
		"mask":        wsNullStr(d.Mask),
		"extension":   wsNullStr(d.Extension),
		"frequency":   wsNullInt(d.Frequency),
		"delay":       wsNullInt(d.Delay),
		"deleteAfter": d.DeleteAfter,
		"usePolling":  d.UsePolling,
		"disabled":    d.Disabled,
		"systemId":    wsNullInt(d.SystemID),
		"talkgroupId": wsNullInt(d.TalkgroupID),
		"order":       d.Order,
	}
}

func mapDirMonitors(dms []db.Dirmonitor) []map[string]any {
	out := make([]map[string]any, len(dms))
	for i, d := range dms {
		out[i] = mapDirMonitor(d)
	}
	return out
}

func mapDownstream(d db.Downstream) map[string]any {
	return map[string]any{
		"id":          d.ID,
		"url":         d.Url,
		"hasApiKey":   d.ApiKey != "",
		"systemsJson": wsNullStr(d.SystemsJson),
		"disabled":    d.Disabled,
		"order":       d.Order,
	}
}

func mapDownstreams(ds []db.Downstream) []map[string]any {
	out := make([]map[string]any, len(ds))
	for i, d := range ds {
		out[i] = mapDownstream(d)
	}
	return out
}

func mapWebhook(w db.Webhook) map[string]any {
	return map[string]any{
		"id":          w.ID,
		"url":         w.Url,
		"type":        w.Type,
		"secret":      wsNullStr(w.Secret),
		"systemsJson": wsNullStr(w.SystemsJson),
		"disabled":    w.Disabled,
		"order":       w.Order,
	}
}

func mapWebhooks(ws []db.Webhook) []map[string]any {
	out := make([]map[string]any, len(ws))
	for i, w := range ws {
		out[i] = mapWebhook(w)
	}
	return out
}

func mapSharedLink(r db.ListSharedLinksRow) map[string]any {
	m := map[string]any{
		"id":             r.ID,
		"callId":         r.CallID,
		"token":          r.Token,
		"createdAt":      r.CreatedAt,
		"sharedBy":       r.SharedBy,
		"dateTime":       r.DateTime,
		"duration":       r.Duration.Int64,
		"systemLabel":    r.SystemLabel.String,
		"talkgroupLabel": r.TalkgroupLabel.String,
		"talkgroupName":  r.TalkgroupName.String,
	}
	if r.ExpiresAt.Valid {
		m["expiresAt"] = r.ExpiresAt.Int64
	} else {
		m["expiresAt"] = nil
	}
	return m
}

// ── Handler map ──

// adminOpHandlers returns the complete map of supported admin WS operations.
func (c *Client) adminOpHandlers() map[string]adminOpHandler {
	return map[string]adminOpHandler{
		// Activity & Logs (existing handlers in client.go)
		"activity.stats":          c.opActivityStats,
		"activity.chart":          c.opActivityChart,
		"activity.top-talkgroups": c.opTopTalkgroups,
		"logs.query":              c.opLogsQuery,
		"logs.level":              c.opLogsLevel,

		// Users
		"users.list":   c.opUsersList,
		"users.create": c.opUsersCreate,
		"users.update": c.opUsersUpdate,
		"users.delete": c.opUsersDelete,

		// Systems
		"systems.list":   c.opSystemsList,
		"systems.create": c.opSystemsCreate,
		"systems.update": c.opSystemsUpdate,
		"systems.delete": c.opSystemsDelete,

		// Talkgroups
		"talkgroups.list":   c.opTalkgroupsList,
		"talkgroups.create": c.opTalkgroupsCreate,
		"talkgroups.update": c.opTalkgroupsUpdate,
		"talkgroups.delete": c.opTalkgroupsDelete,

		// Units
		"units.list":   c.opUnitsList,
		"units.create": c.opUnitsCreate,
		"units.update": c.opUnitsUpdate,
		"units.delete": c.opUnitsDelete,

		// Groups
		"groups.list":   c.opGroupsList,
		"groups.create": c.opGroupsCreate,
		"groups.update": c.opGroupsUpdate,
		"groups.delete": c.opGroupsDelete,

		// Tags
		"tags.list":   c.opTagsList,
		"tags.create": c.opTagsCreate,
		"tags.update": c.opTagsUpdate,
		"tags.delete": c.opTagsDelete,

		// API Keys
		"apikeys.list":   c.opAPIKeysList,
		"apikeys.create": c.opAPIKeysCreate,
		"apikeys.update": c.opAPIKeysUpdate,
		"apikeys.delete": c.opAPIKeysDelete,

		// DirMonitors
		"dirmonitors.list":   c.opDirMonitorsList,
		"dirmonitors.create": c.opDirMonitorsCreate,
		"dirmonitors.update": c.opDirMonitorsUpdate,
		"dirmonitors.delete": c.opDirMonitorsDelete,

		// Downstreams
		"downstreams.list":   c.opDownstreamsList,
		"downstreams.create": c.opDownstreamsCreate,
		"downstreams.update": c.opDownstreamsUpdate,
		"downstreams.delete": c.opDownstreamsDelete,

		// Webhooks
		"webhooks.list":   c.opWebhooksList,
		"webhooks.create": c.opWebhooksCreate,
		"webhooks.update": c.opWebhooksUpdate,
		"webhooks.delete": c.opWebhooksDelete,

		// Shared Links
		"shared-links.list":   c.opSharedLinksList,
		"shared-links.delete": c.opSharedLinksDelete,

		// Config
		"config.get":    c.opConfigGet,
		"config.update": c.opConfigUpdate,

		// Filesystem
		"fs.directories": c.opFSDirectories,

		// Export
		"export.config":     c.opExportConfig,
		"export.talkgroups": c.opExportTalkgroups,
		"export.units":      c.opExportUnits,

		// Import
		"import.config": c.opImportConfig,

		// RadioReference
		"radioreference.apply": c.opRadioReferenceApply,

		// Transcription model management
		"transcription.status":   c.opTranscriptionStatus,
		"transcription.models":   c.opTranscriptionModels,
		"transcription.download": c.opTranscriptionDownload,
		"transcription.delete":   c.opTranscriptionDelete,
		"transcription.stats":    c.opTranscriptionStats,
	}
}

// ── Logs level (moved from inline in client.go) ──

func (c *Client) opLogsLevel(_ context.Context, _ json.RawMessage) (any, error) {
	return map[string]string{"level": logging.GetLevel()}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// USERS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opUsersList(ctx context.Context, _ json.RawMessage) (any, error) {
	users, err := c.hub.queries.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	return mapUsers(users), nil
}

func (c *Client) opUsersCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Username    string  `json:"username"`
		Password    string  `json:"password"`
		Role        string  `json:"role"`
		Disabled    int64   `json:"disabled"`
		SystemsJson *string `json:"systemsJson"`
		Expiration  *int64  `json:"expiration"`
		Limit       *int64  `json:"limit"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.Username == "" {
		return nil, userError("username is required")
	}
	if len(req.Username) > 64 {
		return nil, userError("username must be at most 64 characters")
	}
	if len(req.Password) < 8 {
		return nil, userError("password must be at least 8 characters")
	}
	if len(req.Password) > 128 {
		return nil, userError("password must be at most 128 characters")
	}
	if req.Role == "" {
		req.Role = "listener"
	}
	if !wsValidRoles[req.Role] {
		return nil, userError("role must be 'admin' or 'listener'")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().Unix()
	id, err := c.hub.queries.CreateUser(ctx, db.CreateUserParams{
		Username:           req.Username,
		PasswordHash:       hash,
		Role:               req.Role,
		Disabled:           req.Disabled,
		SystemsJson:        wsPtrToNullStr(req.SystemsJson),
		Expiration:         wsPtrToNullInt(req.Expiration),
		Limit:              wsPtrToNullInt(req.Limit),
		PasswordNeedChange: 1,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("username already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	user, err := c.hub.queries.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created user: %w", err)
	}
	slog.Info("admin: user created", "id", user.ID, "username", user.Username, "role", user.Role, "by", c.userID)
	c.hub.BroadcastAdminEvent("users.updated", nil)
	return mapUser(user), nil
}

func (c *Client) opUsersUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID          int64   `json:"id"`
		Username    string  `json:"username"`
		Role        string  `json:"role"`
		Disabled    int64   `json:"disabled"`
		SystemsJson *string `json:"systemsJson"`
		Expiration  *int64  `json:"expiration"`
		Limit       *int64  `json:"limit"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}
	if req.Username == "" {
		return nil, userError("username is required")
	}
	if len(req.Username) > 64 {
		return nil, userError("username must be at most 64 characters")
	}
	if req.Role == "" {
		return nil, userError("role is required")
	}
	if !wsValidRoles[req.Role] {
		return nil, userError("role must be 'admin' or 'listener'")
	}

	if _, err := c.hub.queries.GetUser(ctx, req.ID); err != nil {
		return nil, userError("user not found")
	}

	// Prevent disabling the bootstrap admin (id=1).
	if req.ID == 1 && req.Disabled != 0 {
		return nil, userError("cannot disable the primary admin account")
	}
	// Protect bootstrap admin role/expiration/limit.
	if req.ID == 1 {
		req.Role = "admin"
		req.Expiration = nil
		req.Limit = nil
	}

	err := c.hub.queries.UpdateUser(ctx, db.UpdateUserParams{
		ID:          req.ID,
		Username:    req.Username,
		Role:        req.Role,
		Disabled:    req.Disabled,
		SystemsJson: wsPtrToNullStr(req.SystemsJson),
		Expiration:  wsPtrToNullInt(req.Expiration),
		Limit:       wsPtrToNullInt(req.Limit),
		UpdatedAt:   time.Now().Unix(),
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("username already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Revoke all tokens so stale claims are not trusted after update.
	auth.Tokens.RevokeAllForUser(req.ID)

	// Immediately disconnect all active WS sessions for the updated user.
	c.hub.DisconnectByUser(req.ID)

	user, err := c.hub.queries.GetUser(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated user: %w", err)
	}
	slog.Info("admin: user updated", "id", user.ID, "username", user.Username, "role", user.Role, "disabled", user.Disabled, "by", c.userID)
	c.hub.BroadcastAdminEvent("users.updated", nil)
	return mapUser(user), nil
}

func (c *Client) opUsersDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	// Cannot delete your own account.
	if c.userID == req.ID {
		return nil, userError("cannot delete your own account")
	}
	// Cannot delete bootstrap admin.
	if req.ID == 1 {
		return nil, userError("cannot delete the primary admin account")
	}

	if _, err := c.hub.queries.GetUser(ctx, req.ID); err != nil {
		return nil, userError("user not found")
	}

	if err := c.hub.queries.DeleteUser(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete user: %w", err)
	}

	// Revoke tokens and disconnect active WS sessions for the deleted user.
	auth.Tokens.RevokeAllForUser(req.ID)
	c.hub.DisconnectByUser(req.ID)

	slog.Info("admin: user deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("users.updated", nil)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// SYSTEMS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opSystemsList(ctx context.Context, _ json.RawMessage) (any, error) {
	systems, err := c.hub.queries.ListSystems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list systems: %w", err)
	}
	return mapSystems(systems), nil
}

func (c *Client) opSystemsCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		SystemID               int64   `json:"systemId"`
		Label                  string  `json:"label"`
		AutoPopulateTalkgroups int64   `json:"autoPopulateTalkgroups"`
		BlacklistsJson         *string `json:"blacklistsJson"`
		Led                    *string `json:"led"`
		Order                  int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}

	id, err := c.hub.queries.CreateSystem(ctx, db.CreateSystemParams{
		SystemID:               req.SystemID,
		Label:                  req.Label,
		AutoPopulateTalkgroups: req.AutoPopulateTalkgroups,
		BlacklistsJson:         wsPtrToNullStr(req.BlacklistsJson),
		Led:                    wsPtrToNullStr(req.Led),
		Order:                  req.Order,
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("system_id already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create system: %w", err)
	}

	system, err := c.hub.queries.GetSystem(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created system: %w", err)
	}
	slog.Info("admin: system created", "id", system.ID, "system_id", system.SystemID, "label", system.Label, "by", c.userID)
	c.hub.BroadcastAdminEvent("systems.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return mapSystem(system), nil
}

func (c *Client) opSystemsUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID                     int64   `json:"id"`
		SystemID               int64   `json:"systemId"`
		Label                  string  `json:"label"`
		AutoPopulateTalkgroups int64   `json:"autoPopulateTalkgroups"`
		BlacklistsJson         *string `json:"blacklistsJson"`
		Led                    *string `json:"led"`
		Order                  int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetSystem(ctx, req.ID); err != nil {
		return nil, userError("system not found")
	}

	err := c.hub.queries.UpdateSystem(ctx, db.UpdateSystemParams{
		ID:                     req.ID,
		SystemID:               req.SystemID,
		Label:                  req.Label,
		AutoPopulateTalkgroups: req.AutoPopulateTalkgroups,
		BlacklistsJson:         wsPtrToNullStr(req.BlacklistsJson),
		Led:                    wsPtrToNullStr(req.Led),
		Order:                  req.Order,
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("system_id already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update system: %w", err)
	}

	system, err := c.hub.queries.GetSystem(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated system: %w", err)
	}
	slog.Info("admin: system updated", "id", system.ID, "system_id", system.SystemID, "by", c.userID)
	c.hub.BroadcastAdminEvent("systems.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return mapSystem(system), nil
}

func (c *Client) opSystemsDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetSystem(ctx, req.ID); err != nil {
		return nil, userError("system not found")
	}

	if err := c.hub.queries.DeleteSystem(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete system: %w", err)
	}
	slog.Info("admin: system deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("systems.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// TALKGROUPS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opTalkgroupsList(ctx context.Context, _ json.RawMessage) (any, error) {
	tgs, err := c.hub.queries.ListAllTalkgroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list talkgroups: %w", err)
	}
	return mapTalkgroups(tgs), nil
}

func (c *Client) opTalkgroupsCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		SystemID    int64   `json:"systemId"`
		TalkgroupID int64   `json:"talkgroupId"`
		Label       *string `json:"label"`
		Name        *string `json:"name"`
		Frequency   *int64  `json:"frequency"`
		Led         *string `json:"led"`
		GroupID     *int64  `json:"groupId"`
		TagID       *int64  `json:"tagId"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}

	id, err := c.hub.queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
		SystemID:    req.SystemID,
		TalkgroupID: req.TalkgroupID,
		Label:       wsPtrToNullStr(req.Label),
		Name:        wsPtrToNullStr(req.Name),
		Frequency:   wsPtrToNullInt(req.Frequency),
		Led:         wsPtrToNullStr(req.Led),
		GroupID:     wsPtrToNullInt(req.GroupID),
		TagID:       wsPtrToNullInt(req.TagID),
		Order:       req.Order,
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("talkgroup already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create talkgroup: %w", err)
	}

	tg, err := c.hub.queries.GetTalkgroup(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created talkgroup: %w", err)
	}
	slog.Info("admin: talkgroup created", "id", tg.ID, "talkgroup_id", tg.TalkgroupID, "by", c.userID)
	c.hub.BroadcastAdminEvent("talkgroups.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return mapTalkgroup(tg), nil
}

func (c *Client) opTalkgroupsUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID          int64   `json:"id"`
		TalkgroupID int64   `json:"talkgroupId"`
		Label       *string `json:"label"`
		Name        *string `json:"name"`
		Frequency   *int64  `json:"frequency"`
		Led         *string `json:"led"`
		GroupID     *int64  `json:"groupId"`
		TagID       *int64  `json:"tagId"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetTalkgroup(ctx, req.ID); err != nil {
		return nil, userError("talkgroup not found")
	}

	err := c.hub.queries.UpdateTalkgroup(ctx, db.UpdateTalkgroupParams{
		ID:          req.ID,
		TalkgroupID: req.TalkgroupID,
		Label:       wsPtrToNullStr(req.Label),
		Name:        wsPtrToNullStr(req.Name),
		Frequency:   wsPtrToNullInt(req.Frequency),
		Led:         wsPtrToNullStr(req.Led),
		GroupID:     wsPtrToNullInt(req.GroupID),
		TagID:       wsPtrToNullInt(req.TagID),
		Order:       req.Order,
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("talkgroup already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update talkgroup: %w", err)
	}

	tg, err := c.hub.queries.GetTalkgroup(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated talkgroup: %w", err)
	}
	slog.Info("admin: talkgroup updated", "id", tg.ID, "talkgroup_id", tg.TalkgroupID, "by", c.userID)
	c.hub.BroadcastAdminEvent("talkgroups.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return mapTalkgroup(tg), nil
}

func (c *Client) opTalkgroupsDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetTalkgroup(ctx, req.ID); err != nil {
		return nil, userError("talkgroup not found")
	}

	if err := c.hub.queries.DeleteTalkgroup(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete talkgroup: %w", err)
	}
	slog.Info("admin: talkgroup deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("talkgroups.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// UNITS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opUnitsList(ctx context.Context, params json.RawMessage) (any, error) {
	// Optional filter by systemId and unitId pattern.
	var req struct {
		SystemID      *int64  `json:"systemId"`
		UnitIDPattern *string `json:"unitIdPattern"`
	}
	if params != nil {
		_ = json.Unmarshal(params, &req) // ignore parse errors — treat as no filter
	}

	var units []db.Unit
	var err error
	if req.SystemID != nil {
		units, err = c.hub.queries.ListUnitsBySystem(ctx, *req.SystemID)
	} else {
		units, err = c.hub.queries.ListAllUnits(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list units: %w", err)
	}

	// Apply unit_id pattern filter if provided (prefix matching).
	if req.UnitIDPattern != nil && *req.UnitIDPattern != "" {
		filtered := make([]db.Unit, 0, len(units))
		for _, u := range units {
			if strings.HasPrefix(strconv.FormatInt(u.UnitID, 10), *req.UnitIDPattern) {
				filtered = append(filtered, u)
			}
		}
		units = filtered
	}

	return mapUnits(units), nil
}

func (c *Client) opUnitsCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		SystemID int64   `json:"systemId"`
		UnitID   int64   `json:"unitId"`
		Label    *string `json:"label"`
		Order    int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}

	id, err := c.hub.queries.CreateUnit(ctx, db.CreateUnitParams{
		SystemID: req.SystemID,
		UnitID:   req.UnitID,
		Label:    wsPtrToNullStr(req.Label),
		Order:    req.Order,
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("unit already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create unit: %w", err)
	}

	unit, err := c.hub.queries.GetUnit(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created unit: %w", err)
	}
	slog.Info("admin: unit created", "id", unit.ID, "unit_id", unit.UnitID, "by", c.userID)
	c.hub.BroadcastAdminEvent("units.updated", nil)
	return mapUnit(unit), nil
}

func (c *Client) opUnitsUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID     int64   `json:"id"`
		UnitID int64   `json:"unitId"`
		Label  *string `json:"label"`
		Order  int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetUnit(ctx, req.ID); err != nil {
		return nil, userError("unit not found")
	}

	err := c.hub.queries.UpdateUnit(ctx, db.UpdateUnitParams{
		ID:     req.ID,
		UnitID: req.UnitID,
		Label:  wsPtrToNullStr(req.Label),
		Order:  req.Order,
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("unit already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update unit: %w", err)
	}

	unit, err := c.hub.queries.GetUnit(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated unit: %w", err)
	}
	slog.Info("admin: unit updated", "id", unit.ID, "unit_id", unit.UnitID, "by", c.userID)
	c.hub.BroadcastAdminEvent("units.updated", nil)
	return mapUnit(unit), nil
}

func (c *Client) opUnitsDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetUnit(ctx, req.ID); err != nil {
		return nil, userError("unit not found")
	}

	if err := c.hub.queries.DeleteUnit(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete unit: %w", err)
	}
	slog.Info("admin: unit deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("units.updated", nil)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// GROUPS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opGroupsList(ctx context.Context, _ json.RawMessage) (any, error) {
	groups, err := c.hub.queries.ListGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}
	return groups, nil
}

func (c *Client) opGroupsCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.Label == "" {
		return nil, userError("label is required")
	}

	id, err := c.hub.queries.CreateGroup(ctx, req.Label)
	if wsIsUniqueViolation(err) {
		return nil, userError("group label already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	group, err := c.hub.queries.GetGroup(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created group: %w", err)
	}
	slog.Info("admin: group created", "id", group.ID, "label", group.Label, "by", c.userID)
	c.hub.BroadcastAdminEvent("groups.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return group, nil
}

func (c *Client) opGroupsUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID    int64  `json:"id"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}
	if req.Label == "" {
		return nil, userError("label is required")
	}

	if _, err := c.hub.queries.GetGroup(ctx, req.ID); err != nil {
		return nil, userError("group not found")
	}

	err := c.hub.queries.UpdateGroup(ctx, db.UpdateGroupParams{ID: req.ID, Label: req.Label})
	if wsIsUniqueViolation(err) {
		return nil, userError("group label already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update group: %w", err)
	}

	group, err := c.hub.queries.GetGroup(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated group: %w", err)
	}
	slog.Info("admin: group updated", "id", group.ID, "label", group.Label, "by", c.userID)
	c.hub.BroadcastAdminEvent("groups.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return group, nil
}

func (c *Client) opGroupsDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetGroup(ctx, req.ID); err != nil {
		return nil, userError("group not found")
	}

	if err := c.hub.queries.DeleteGroup(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete group: %w", err)
	}
	slog.Info("admin: group deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("groups.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// TAGS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opTagsList(ctx context.Context, _ json.RawMessage) (any, error) {
	tags, err := c.hub.queries.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	return tags, nil
}

func (c *Client) opTagsCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.Label == "" {
		return nil, userError("label is required")
	}

	id, err := c.hub.queries.CreateTag(ctx, req.Label)
	if wsIsUniqueViolation(err) {
		return nil, userError("tag label already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	tag, err := c.hub.queries.GetTag(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created tag: %w", err)
	}
	slog.Info("admin: tag created", "id", tag.ID, "label", tag.Label, "by", c.userID)
	c.hub.BroadcastAdminEvent("tags.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return tag, nil
}

func (c *Client) opTagsUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID    int64  `json:"id"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}
	if req.Label == "" {
		return nil, userError("label is required")
	}

	if _, err := c.hub.queries.GetTag(ctx, req.ID); err != nil {
		return nil, userError("tag not found")
	}

	err := c.hub.queries.UpdateTag(ctx, db.UpdateTagParams{ID: req.ID, Label: req.Label})
	if wsIsUniqueViolation(err) {
		return nil, userError("tag label already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update tag: %w", err)
	}

	tag, err := c.hub.queries.GetTag(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated tag: %w", err)
	}
	slog.Info("admin: tag updated", "id", tag.ID, "label", tag.Label, "by", c.userID)
	c.hub.BroadcastAdminEvent("tags.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return tag, nil
}

func (c *Client) opTagsDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetTag(ctx, req.ID); err != nil {
		return nil, userError("tag not found")
	}

	if err := c.hub.queries.DeleteTag(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete tag: %w", err)
	}
	slog.Info("admin: tag deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("tags.updated", nil)
	c.hub.BroadcastCFG(ctx)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// API KEYS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opAPIKeysList(ctx context.Context, _ json.RawMessage) (any, error) {
	keys, err := c.hub.queries.ListAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	return mapAPIKeys(keys), nil
}

func (c *Client) opAPIKeysCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Key           *string `json:"key"`
		Ident         *string `json:"ident"`
		Disabled      int64   `json:"disabled"`
		SystemsJson   *string `json:"systemsJson"`
		CallRateLimit *int64  `json:"callRateLimit"`
		Order         int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}

	plainKey := uuid.New().String()
	if req.Key != nil && *req.Key != "" {
		plainKey = *req.Key
	}
	hashedKey := auth.HashAPIKey(plainKey)

	id, err := c.hub.queries.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		Key:           hashedKey,
		Ident:         wsPtrToNullStr(req.Ident),
		Disabled:      req.Disabled,
		SystemsJson:   wsPtrToNullStr(req.SystemsJson),
		CallRateLimit: wsPtrToNullInt(req.CallRateLimit),
		Order:         req.Order,
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("API key already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	key, err := c.hub.queries.GetAPIKey(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created API key: %w", err)
	}
	slog.Info("admin: api key created", "id", key.ID, "ident", key.Ident.String, "by", c.userID)
	c.hub.BroadcastAdminEvent("apikeys.updated", nil)

	resp := mapAPIKey(key)
	resp["createdKey"] = plainKey // Return plain key once on creation.
	return resp, nil
}

func (c *Client) opAPIKeysUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID            int64   `json:"id"`
		Key           *string `json:"key"`
		Ident         *string `json:"ident"`
		Disabled      int64   `json:"disabled"`
		SystemsJson   *string `json:"systemsJson"`
		CallRateLimit *int64  `json:"callRateLimit"`
		Order         int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	current, err := c.hub.queries.GetAPIKey(ctx, req.ID)
	if err != nil {
		return nil, userError("API key not found")
	}

	keyHash := current.Key
	if req.Key != nil && *req.Key != "" {
		keyHash = auth.HashAPIKey(*req.Key)
	}

	err = c.hub.queries.UpdateAPIKey(ctx, db.UpdateAPIKeyParams{
		ID:            req.ID,
		Key:           keyHash,
		Ident:         wsPtrToNullStr(req.Ident),
		Disabled:      req.Disabled,
		SystemsJson:   wsPtrToNullStr(req.SystemsJson),
		CallRateLimit: wsPtrToNullInt(req.CallRateLimit),
		Order:         req.Order,
	})
	if wsIsUniqueViolation(err) {
		return nil, userError("API key already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	key, err := c.hub.queries.GetAPIKey(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated API key: %w", err)
	}
	slog.Info("admin: api key updated", "id", key.ID, "ident", key.Ident.String, "by", c.userID)
	c.hub.BroadcastAdminEvent("apikeys.updated", nil)
	return mapAPIKey(key), nil
}

func (c *Client) opAPIKeysDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetAPIKey(ctx, req.ID); err != nil {
		return nil, userError("API key not found")
	}

	if err := c.hub.queries.DeleteAPIKey(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete API key: %w", err)
	}
	slog.Info("admin: api key deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("apikeys.updated", nil)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// DIRMONITORS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opDirMonitorsList(ctx context.Context, _ json.RawMessage) (any, error) {
	dms, err := c.hub.queries.ListDirMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list dirmonitors: %w", err)
	}
	return mapDirMonitors(dms), nil
}

func (c *Client) opDirMonitorsCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Directory   string  `json:"directory"`
		Type        string  `json:"type"`
		Mask        *string `json:"mask"`
		Extension   *string `json:"extension"`
		Frequency   *int64  `json:"frequency"`
		Delay       *int64  `json:"delay"`
		DeleteAfter int64   `json:"deleteAfter"`
		UsePolling  int64   `json:"usePolling"`
		Disabled    int64   `json:"disabled"`
		SystemID    *int64  `json:"systemId"`
		TalkgroupID *int64  `json:"talkgroupId"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.Directory == "" {
		return nil, userError("directory is required")
	}
	if info, statErr := os.Stat(req.Directory); statErr != nil {
		return nil, userError("directory does not exist or is not accessible: " + statErr.Error())
	} else if !info.IsDir() {
		return nil, userError("path is not a directory: " + req.Directory)
	}

	id, err := c.hub.queries.CreateDirMonitor(ctx, db.CreateDirMonitorParams{
		Directory:   req.Directory,
		Type:        req.Type,
		Mask:        wsPtrToNullStr(req.Mask),
		Extension:   wsPtrToNullStr(req.Extension),
		Frequency:   wsPtrToNullInt(req.Frequency),
		Delay:       wsPtrToNullInt(req.Delay),
		DeleteAfter: req.DeleteAfter,
		UsePolling:  req.UsePolling,
		Disabled:    req.Disabled,
		SystemID:    wsPtrToNullInt(req.SystemID),
		TalkgroupID: wsPtrToNullInt(req.TalkgroupID),
		Order:       req.Order,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create dirmonitor: %w", err)
	}

	dm, err := c.hub.queries.GetDirMonitor(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created dirmonitor: %w", err)
	}
	if c.hub.deps.DirMonitorReload != nil {
		c.hub.deps.DirMonitorReload.Reload()
	}
	slog.Info("admin: dirmonitor created", "id", dm.ID, "dir", dm.Directory, "by", c.userID)
	c.hub.BroadcastAdminEvent("dirmonitors.updated", nil)
	return mapDirMonitor(dm), nil
}

func (c *Client) opDirMonitorsUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID          int64   `json:"id"`
		Directory   string  `json:"directory"`
		Type        string  `json:"type"`
		Mask        *string `json:"mask"`
		Extension   *string `json:"extension"`
		Frequency   *int64  `json:"frequency"`
		Delay       *int64  `json:"delay"`
		DeleteAfter int64   `json:"deleteAfter"`
		UsePolling  int64   `json:"usePolling"`
		Disabled    int64   `json:"disabled"`
		SystemID    *int64  `json:"systemId"`
		TalkgroupID *int64  `json:"talkgroupId"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}
	if req.Directory == "" {
		return nil, userError("directory is required")
	}
	if info, statErr := os.Stat(req.Directory); statErr != nil {
		return nil, userError("directory does not exist or is not accessible: " + statErr.Error())
	} else if !info.IsDir() {
		return nil, userError("path is not a directory: " + req.Directory)
	}

	if _, err := c.hub.queries.GetDirMonitor(ctx, req.ID); err != nil {
		return nil, userError("dirmonitor not found")
	}

	if err := c.hub.queries.UpdateDirMonitor(ctx, db.UpdateDirMonitorParams{
		ID:          req.ID,
		Directory:   req.Directory,
		Type:        req.Type,
		Mask:        wsPtrToNullStr(req.Mask),
		Extension:   wsPtrToNullStr(req.Extension),
		Frequency:   wsPtrToNullInt(req.Frequency),
		Delay:       wsPtrToNullInt(req.Delay),
		DeleteAfter: req.DeleteAfter,
		UsePolling:  req.UsePolling,
		Disabled:    req.Disabled,
		SystemID:    wsPtrToNullInt(req.SystemID),
		TalkgroupID: wsPtrToNullInt(req.TalkgroupID),
		Order:       req.Order,
	}); err != nil {
		return nil, fmt.Errorf("failed to update dirmonitor: %w", err)
	}

	dm, err := c.hub.queries.GetDirMonitor(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated dirmonitor: %w", err)
	}
	if c.hub.deps.DirMonitorReload != nil {
		c.hub.deps.DirMonitorReload.Reload()
	}
	slog.Info("admin: dirmonitor updated", "id", dm.ID, "dir", dm.Directory, "by", c.userID)
	c.hub.BroadcastAdminEvent("dirmonitors.updated", nil)
	return mapDirMonitor(dm), nil
}

func (c *Client) opDirMonitorsDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetDirMonitor(ctx, req.ID); err != nil {
		return nil, userError("dirmonitor not found")
	}

	if err := c.hub.queries.DeleteDirMonitor(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete dirmonitor: %w", err)
	}
	if c.hub.deps.DirMonitorReload != nil {
		c.hub.deps.DirMonitorReload.Reload()
	}
	slog.Info("admin: dirmonitor deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("dirmonitors.updated", nil)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// DOWNSTREAMS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opDownstreamsList(ctx context.Context, _ json.RawMessage) (any, error) {
	ds, err := c.hub.queries.ListDownstreams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list downstreams: %w", err)
	}
	return mapDownstreams(ds), nil
}

func (c *Client) opDownstreamsCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Url         string  `json:"url"`
		ApiKey      string  `json:"apiKey"`
		SystemsJson *string `json:"systemsJson"`
		Disabled    int64   `json:"disabled"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.Url == "" {
		return nil, userError("url is required")
	}
	if !wsValidHTTPURL(req.Url) {
		return nil, userError("url must use http or https scheme")
	}

	apiKey := req.ApiKey
	if c.hub.deps.EncryptionKey != "" && apiKey != "" {
		enc, err := auth.EncryptString(apiKey, c.hub.deps.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt downstream API key: %w", err)
		}
		apiKey = enc
	}

	id, err := c.hub.queries.CreateDownstream(ctx, db.CreateDownstreamParams{
		Url:         req.Url,
		ApiKey:      apiKey,
		SystemsJson: wsPtrToNullStr(req.SystemsJson),
		Disabled:    req.Disabled,
		Order:       req.Order,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create downstream: %w", err)
	}

	ds, err := c.hub.queries.GetDownstream(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created downstream: %w", err)
	}
	if c.hub.deps.DownstreamReload != nil {
		c.hub.deps.DownstreamReload.Reload()
	}
	slog.Info("admin: downstream created", "id", ds.ID, "url", ds.Url, "by", c.userID)
	c.hub.BroadcastAdminEvent("downstreams.updated", nil)
	return mapDownstream(ds), nil
}

func (c *Client) opDownstreamsUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID          int64   `json:"id"`
		Url         string  `json:"url"`
		ApiKey      string  `json:"apiKey"`
		SystemsJson *string `json:"systemsJson"`
		Disabled    int64   `json:"disabled"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}
	if req.Url != "" && !wsValidHTTPURL(req.Url) {
		return nil, userError("url must use http or https scheme")
	}

	existing, err := c.hub.queries.GetDownstream(ctx, req.ID)
	if err != nil {
		return nil, userError("downstream not found")
	}

	// Preserve existing API key if none provided (key is never sent to clients).
	apiKey := existing.ApiKey
	if req.ApiKey != "" {
		if c.hub.deps.EncryptionKey != "" {
			enc, err := auth.EncryptString(req.ApiKey, c.hub.deps.EncryptionKey)
			if err != nil {
				return nil, fmt.Errorf("encrypt downstream API key: %w", err)
			}
			apiKey = enc
		} else {
			apiKey = req.ApiKey
		}
	}

	if err := c.hub.queries.UpdateDownstream(ctx, db.UpdateDownstreamParams{
		ID:          req.ID,
		Url:         req.Url,
		ApiKey:      apiKey,
		SystemsJson: wsPtrToNullStr(req.SystemsJson),
		Disabled:    req.Disabled,
		Order:       req.Order,
	}); err != nil {
		return nil, fmt.Errorf("failed to update downstream: %w", err)
	}

	ds, err := c.hub.queries.GetDownstream(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated downstream: %w", err)
	}
	if c.hub.deps.DownstreamReload != nil {
		c.hub.deps.DownstreamReload.Reload()
	}
	slog.Info("admin: downstream updated", "id", ds.ID, "url", ds.Url, "by", c.userID)
	c.hub.BroadcastAdminEvent("downstreams.updated", nil)
	return mapDownstream(ds), nil
}

func (c *Client) opDownstreamsDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetDownstream(ctx, req.ID); err != nil {
		return nil, userError("downstream not found")
	}

	if err := c.hub.queries.DeleteDownstream(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete downstream: %w", err)
	}
	if c.hub.deps.DownstreamReload != nil {
		c.hub.deps.DownstreamReload.Reload()
	}
	slog.Info("admin: downstream deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("downstreams.updated", nil)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// WEBHOOKS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opWebhooksList(ctx context.Context, _ json.RawMessage) (any, error) {
	whs, err := c.hub.queries.ListWebhooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}
	return mapWebhooks(whs), nil
}

func (c *Client) opWebhooksCreate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Url         string  `json:"url"`
		Type        string  `json:"type"`
		Secret      *string `json:"secret"`
		SystemsJson *string `json:"systemsJson"`
		Disabled    int64   `json:"disabled"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.Url == "" {
		return nil, userError("url is required")
	}
	if !wsValidHTTPURL(req.Url) {
		return nil, userError("url must use http or https scheme")
	}

	id, err := c.hub.queries.CreateWebhook(ctx, db.CreateWebhookParams{
		Url:         req.Url,
		Type:        req.Type,
		Secret:      wsPtrToNullStr(req.Secret),
		SystemsJson: wsPtrToNullStr(req.SystemsJson),
		Disabled:    req.Disabled,
		Order:       req.Order,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	wh, err := c.hub.queries.GetWebhook(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created webhook: %w", err)
	}
	slog.Info("admin: webhook created", "id", wh.ID, "url", wh.Url, "by", c.userID)
	c.hub.BroadcastAdminEvent("webhooks.updated", nil)
	return mapWebhook(wh), nil
}

func (c *Client) opWebhooksUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID          int64   `json:"id"`
		Url         string  `json:"url"`
		Type        string  `json:"type"`
		Secret      *string `json:"secret"`
		SystemsJson *string `json:"systemsJson"`
		Disabled    int64   `json:"disabled"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}
	if req.Url != "" && !wsValidHTTPURL(req.Url) {
		return nil, userError("url must use http or https scheme")
	}

	if _, err := c.hub.queries.GetWebhook(ctx, req.ID); err != nil {
		return nil, userError("webhook not found")
	}

	if err := c.hub.queries.UpdateWebhook(ctx, db.UpdateWebhookParams{
		ID:          req.ID,
		Url:         req.Url,
		Type:        req.Type,
		Secret:      wsPtrToNullStr(req.Secret),
		SystemsJson: wsPtrToNullStr(req.SystemsJson),
		Disabled:    req.Disabled,
		Order:       req.Order,
	}); err != nil {
		return nil, fmt.Errorf("failed to update webhook: %w", err)
	}

	wh, err := c.hub.queries.GetWebhook(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated webhook: %w", err)
	}
	slog.Info("admin: webhook updated", "id", wh.ID, "url", wh.Url, "by", c.userID)
	c.hub.BroadcastAdminEvent("webhooks.updated", nil)
	return mapWebhook(wh), nil
}

func (c *Client) opWebhooksDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if _, err := c.hub.queries.GetWebhook(ctx, req.ID); err != nil {
		return nil, userError("webhook not found")
	}

	if err := c.hub.queries.DeleteWebhook(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete webhook: %w", err)
	}
	slog.Info("admin: webhook deleted", "id", req.ID, "by", c.userID)
	c.hub.BroadcastAdminEvent("webhooks.updated", nil)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// SHARED LINKS
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opSharedLinksList(ctx context.Context, _ json.RawMessage) (any, error) {
	rows, err := c.hub.queries.ListSharedLinks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list shared links: %w", err)
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapSharedLink(r))
	}
	return items, nil
}

func (c *Client) opSharedLinksDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, userError("id is required")
	}

	if err := c.hub.queries.DeleteSharedLink(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete shared link: %w", err)
	}
	c.hub.BroadcastAdminEvent("shared-links.updated", nil)
	return map[string]bool{"deleted": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// CONFIG
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opConfigGet(ctx context.Context, _ json.RawMessage) (any, error) {
	settings, err := c.hub.queries.ListSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}

	settingsList := make([]map[string]string, len(settings))
	for i, s := range settings {
		val := s.Value
		if SensitiveSettingKeys[s.Key] && c.hub.deps.EncryptionKey != "" {
			if plain, err := auth.DecryptString(val, c.hub.deps.EncryptionKey); err == nil {
				val = plain
			}
		}
		settingsList[i] = map[string]string{"key": s.Key, "value": val}
	}

	return map[string]any{
		"settings": settingsList,
		"capabilities": map[string]bool{
			"ffmpeg":  c.hub.deps.FFmpegAvailable,
			"fdkAac":  c.hub.deps.FDKAACAvailable,
			"whisper": c.hub.deps.WhisperAvailable,
		},
	}, nil
}

func (c *Client) opConfigUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	var body struct {
		Settings []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(params, &body); err != nil {
		return nil, userError("invalid request body")
	}
	settings := body.Settings

	// Validate all keys first.
	for _, s := range settings {
		if !wsAllowedSettingKeys[s.Key] {
			return nil, userError("unknown setting key: " + s.Key)
		}
		if s.Key == "logLevel" {
			if _, ok := logging.ParseLevel(s.Value); !ok {
				return nil, userError("invalid logLevel; expected debug, info, warn, or error")
			}
		}
		if s.Key == "audioEncodingPreset" {
			if !audio.IsValidEncodingPreset(s.Value) {
				return nil, userError("invalid audioEncodingPreset value")
			}
			if audio.IsHEEncodingPreset(s.Value) && !c.hub.deps.FDKAACAvailable {
				return nil, userError("selected HE-AAC preset requires libfdk_aac support in ffmpeg")
			}
		}
		if s.Key == "audioConversion" {
			if v, err := strconv.Atoi(s.Value); err == nil && v != 0 && !c.hub.deps.FFmpegAvailable {
				return nil, userError("ffmpeg is not installed — install it and restart the service to enable audio conversion")
			}
		}
	}

	sqlDB := c.hub.deps.SQLDB
	if sqlDB == nil {
		return nil, fmt.Errorf("transaction support not available")
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := c.hub.queries.WithTx(tx)
	for _, s := range settings {
		val := s.Value
		if SensitiveSettingKeys[s.Key] && c.hub.deps.EncryptionKey != "" && val != "" {
			enc, err := auth.EncryptString(val, c.hub.deps.EncryptionKey)
			if err != nil {
				return nil, fmt.Errorf("encrypt setting %q: %w", s.Key, err)
			}
			val = enc
		}
		if err := qtx.UpsertSetting(ctx, db.UpsertSettingParams{Key: s.Key, Value: val}); err != nil {
			return nil, fmt.Errorf("failed to save config: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit config: %w", err)
	}

	// Log each changed setting, redacting sensitive keys.
	for _, s := range settings {
		v := s.Value
		if s.Key == "vapidPrivateKey" {
			v = "[REDACTED]"
		}
		slog.Info("admin: config updated", "key", s.Key, "value", v, "by", c.userID)
	}

	// Apply log level change at runtime.
	for _, s := range settings {
		if s.Key == "logLevel" {
			if err := logging.SetLevel(s.Value); err != nil {
				slog.Warn("invalid logLevel setting, keeping previous runtime level", "value", s.Value, "error", err)
			}
			break
		}
	}

	// Hot-reload transcription if any transcription setting changed.
	if c.hub.deps.TranscriberReload != nil {
		transcriptionKeys := map[string]bool{
			"transcriptionEnabled":  true,
			"transcriptionUrl":      true,
			"transcriptionModel":    true,
			"transcriptionLanguage": true,
			"transcriptionDiarize":  true,
		}
		needsReload := false
		for _, s := range settings {
			if transcriptionKeys[s.Key] {
				needsReload = true
				break
			}
		}
		if needsReload {
			// Read current settings from DB (just committed).
			tEnabled, _ := c.hub.queries.GetSetting(ctx, "transcriptionEnabled")
			tURL, _ := c.hub.queries.GetSetting(ctx, "transcriptionUrl")
			tModel, _ := c.hub.queries.GetSetting(ctx, "transcriptionModel")
			tLang, _ := c.hub.queries.GetSetting(ctx, "transcriptionLanguage")
			tDiarize, _ := c.hub.queries.GetSetting(ctx, "transcriptionDiarize")

			ok := c.hub.deps.TranscriberReload.Reload(
				tEnabled.Value == "true",
				tURL.Value,
				tModel.Value,
				tLang.Value,
				tDiarize.Value == "true",
			)
			c.hub.deps.WhisperAvailable = ok && tEnabled.Value == "true"
		}
	}

	// Broadcast updated config to all WS clients using the safe,
	// curated CFG builder (excludes secrets like VAPID keys).
	c.hub.BroadcastCFG(ctx)

	c.hub.BroadcastAdminEvent("config.updated", nil)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// FILESYSTEM
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opFSDirectories(_ context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Path string `json:"path"`
	}
	if params != nil {
		_ = json.Unmarshal(params, &req)
	}
	if req.Path == "" {
		req.Path = "/"
	}

	clean := filepath.Clean(req.Path)
	if !filepath.IsAbs(clean) {
		return nil, userError("path must be absolute")
	}

	info, err := os.Stat(clean)
	if err != nil {
		return nil, userError("directory does not exist or is not accessible: " + err.Error())
	}
	if !info.IsDir() {
		return nil, userError("path is not a directory: " + clean)
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		return nil, userError("failed to read directory: " + err.Error())
	}

	type dirEntry struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}

	dirs := make([]dirEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if clean == "/" && wsHiddenTopLevelDirs[name] {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, dirEntry{Name: name, Path: filepath.Join(clean, name)})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	var parent *string
	if clean != "/" {
		p := filepath.Dir(clean)
		parent = &p
	}

	return map[string]any{
		"path":        clean,
		"parent":      parent,
		"directories": dirs,
	}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// EXPORT
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opExportConfig(ctx context.Context, _ json.RawMessage) (any, error) {
	settings, err := c.hub.queries.ListSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export settings: %w", err)
	}
	users, err := c.hub.queries.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export users: %w", err)
	}
	systems, err := c.hub.queries.ListSystems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export systems: %w", err)
	}
	talkgroups, err := c.hub.queries.ListAllTalkgroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export talkgroups: %w", err)
	}
	units, err := c.hub.queries.ListAllUnits(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export units: %w", err)
	}
	groups, err := c.hub.queries.ListGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export groups: %w", err)
	}
	tags, err := c.hub.queries.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export tags: %w", err)
	}
	apiKeys, err := c.hub.queries.ListAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export api keys: %w", err)
	}
	dirmonitors, err := c.hub.queries.ListDirMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export dirmonitors: %w", err)
	}
	downstreams, err := c.hub.queries.ListDownstreams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export downstreams: %w", err)
	}
	webhooks, err := c.hub.queries.ListWebhooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export webhooks: %w", err)
	}

	// Export all fields — use snake_case keys to match db struct JSON tags.
	// API keys include the hashed key so import can restore authentication.
	// Downstream API keys and webhook secrets are included for full backup.
	// The exported JSON file should be treated as sensitive.
	exportAPIKeys := make([]map[string]any, len(apiKeys))
	for i, k := range apiKeys {
		exportAPIKeys[i] = map[string]any{
			"id":              k.ID,
			"key":             k.Key,
			"ident":           wsNullStr(k.Ident),
			"disabled":        k.Disabled,
			"systems_json":    wsNullStr(k.SystemsJson),
			"call_rate_limit": wsNullInt(k.CallRateLimit),
			"order":           k.Order,
		}
	}
	exportDownstreams := make([]map[string]any, len(downstreams))
	for i, d := range downstreams {
		exportDownstreams[i] = map[string]any{
			"id":           d.ID,
			"url":          d.Url,
			"api_key":      d.ApiKey,
			"systems_json": wsNullStr(d.SystemsJson),
			"disabled":     d.Disabled,
			"order":        d.Order,
		}
	}
	exportWebhooks := make([]map[string]any, len(webhooks))
	for i, w := range webhooks {
		exportWebhooks[i] = map[string]any{
			"id":           w.ID,
			"url":          w.Url,
			"type":         w.Type,
			"secret":       wsNullStr(w.Secret),
			"systems_json": wsNullStr(w.SystemsJson),
			"disabled":     w.Disabled,
			"order":        w.Order,
		}
	}

	return map[string]any{
		"settings":    settings,
		"users":       users,
		"systems":     systems,
		"talkgroups":  talkgroups,
		"units":       units,
		"groups":      groups,
		"tags":        tags,
		"apiKeys":     exportAPIKeys,
		"dirmonitors": dirmonitors,
		"downstreams": exportDownstreams,
		"webhooks":    exportWebhooks,
	}, nil
}

func (c *Client) opExportTalkgroups(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		SystemID *int64 `json:"systemId"`
	}
	if params != nil {
		_ = json.Unmarshal(params, &req)
	}

	var talkgroups []db.Talkgroup
	if req.SystemID != nil {
		rows, err := c.hub.queries.ListTalkgroupsBySystem(ctx, *req.SystemID)
		if err != nil {
			return nil, fmt.Errorf("failed to list talkgroups: %w", err)
		}
		talkgroups = rows
	} else {
		rows, err := c.hub.queries.ListAllTalkgroups(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list talkgroups: %w", err)
		}
		talkgroups = rows
	}

	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"talkgroup_id", "label", "name", "tag_id", "group_id", "frequency", "led", "order"})
	for _, tg := range talkgroups {
		freq := ""
		if tg.Frequency.Valid {
			freq = strconv.FormatInt(tg.Frequency.Int64, 10)
		}
		groupID := ""
		if tg.GroupID.Valid {
			groupID = strconv.FormatInt(tg.GroupID.Int64, 10)
		}
		tagID := ""
		if tg.TagID.Valid {
			tagID = strconv.FormatInt(tg.TagID.Int64, 10)
		}
		_ = w.Write([]string{
			strconv.FormatInt(tg.TalkgroupID, 10),
			tg.Label.String,
			tg.Name.String,
			tagID,
			groupID,
			freq,
			tg.Led.String,
			strconv.FormatInt(tg.Order, 10),
		})
	}
	w.Flush()

	return map[string]string{"csv": buf.String()}, nil
}

func (c *Client) opExportUnits(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		SystemID *int64 `json:"systemId"`
	}
	if params != nil {
		_ = json.Unmarshal(params, &req)
	}

	var units []db.Unit
	if req.SystemID != nil {
		rows, err := c.hub.queries.ListUnitsBySystem(ctx, *req.SystemID)
		if err != nil {
			return nil, fmt.Errorf("failed to list units: %w", err)
		}
		units = rows
	} else {
		rows, err := c.hub.queries.ListAllUnits(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list units: %w", err)
		}
		units = rows
	}

	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"unit_id", "label", "order"})
	for _, u := range units {
		_ = w.Write([]string{
			strconv.FormatInt(u.UnitID, 10),
			u.Label.String,
			strconv.FormatInt(u.Order, 10),
		})
	}
	w.Flush()

	return map[string]string{"csv": buf.String()}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// IMPORT
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opImportConfig(ctx context.Context, params json.RawMessage) (any, error) {
	var data struct {
		Settings    []db.Setting    `json:"settings"`
		Groups      []db.Group      `json:"groups"`
		Tags        []db.Tag        `json:"tags"`
		Systems     []db.System     `json:"systems"`
		Talkgroups  []db.Talkgroup  `json:"talkgroups"`
		Units       []db.Unit       `json:"units"`
		APIKeys     []db.ApiKey     `json:"apiKeys"`
		DirMonitors []db.Dirmonitor `json:"dirmonitors"`
		Downstreams []db.Downstream `json:"downstreams"`
		Webhooks    []db.Webhook    `json:"webhooks"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		return nil, userError("invalid JSON body")
	}

	// Validate encrypted values: reject if no key configured, or if the wrong key is configured.
	encKey := c.hub.deps.EncryptionKey
	for _, s := range data.Settings {
		if SensitiveSettingKeys[s.Key] && auth.IsEncrypted(s.Value) {
			if encKey == "" {
				return nil, userError("backup contains encrypted settings but no encryption key is configured — set --encryption-key before importing")
			}
			if _, err := auth.DecryptString(s.Value, encKey); err != nil {
				return nil, userError("backup contains encrypted settings that cannot be decrypted with the current encryption key — check that --encryption-key matches the key used when the backup was created")
			}
		}
	}
	for _, d := range data.Downstreams {
		if auth.IsEncrypted(d.ApiKey) {
			if encKey == "" {
				return nil, userError("backup contains encrypted downstream API keys but no encryption key is configured — set --encryption-key before importing")
			}
			if _, err := auth.DecryptString(d.ApiKey, encKey); err != nil {
				return nil, userError("backup contains encrypted downstream API keys that cannot be decrypted with the current encryption key — check that --encryption-key matches the key used when the backup was created")
			}
		}
	}

	sqlDB := c.hub.deps.SQLDB
	if sqlDB == nil {
		return nil, fmt.Errorf("transaction support not available")
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := c.hub.queries.WithTx(tx)

	// Settings
	for _, s := range data.Settings {
		if !wsAllowedSettingKeys[s.Key] {
			slog.Warn("import config: skipping unknown setting key", "key", s.Key)
			continue
		}
		if err := qtx.UpsertSetting(ctx, db.UpsertSettingParams(s)); err != nil {
			return nil, fmt.Errorf("failed to import settings: %w", err)
		}
	}

	// Groups
	for _, g := range data.Groups {
		if _, err := qtx.CreateGroup(ctx, g.Label); err != nil && !wsIsUniqueViolation(err) {
			return nil, fmt.Errorf("failed to import groups: %w", err)
		}
	}

	// Tags
	for _, t := range data.Tags {
		if _, err := qtx.CreateTag(ctx, t.Label); err != nil && !wsIsUniqueViolation(err) {
			return nil, fmt.Errorf("failed to import tags: %w", err)
		}
	}

	// Systems
	for _, s := range data.Systems {
		if _, err := qtx.CreateSystem(ctx, db.CreateSystemParams{
			SystemID:               s.SystemID,
			Label:                  s.Label,
			AutoPopulateTalkgroups: s.AutoPopulateTalkgroups,
			BlacklistsJson:         s.BlacklistsJson,
			Led:                    s.Led,
			Order:                  s.Order,
		}); err != nil && !wsIsUniqueViolation(err) {
			return nil, fmt.Errorf("failed to import systems: %w", err)
		}
	}

	// Talkgroups
	for _, tg := range data.Talkgroups {
		if err := qtx.UpsertTalkgroup(ctx, db.UpsertTalkgroupParams{
			SystemID:    tg.SystemID,
			TalkgroupID: tg.TalkgroupID,
			Label:       tg.Label,
			Name:        tg.Name,
			Frequency:   tg.Frequency,
			Led:         tg.Led,
			GroupID:     tg.GroupID,
			TagID:       tg.TagID,
			Order:       tg.Order,
		}); err != nil {
			return nil, fmt.Errorf("failed to import talkgroups: %w", err)
		}
	}

	// Units
	for _, u := range data.Units {
		if err := qtx.UpsertUnit(ctx, db.UpsertUnitParams{
			SystemID: u.SystemID,
			UnitID:   u.UnitID,
			Label:    u.Label,
			Order:    u.Order,
		}); err != nil {
			return nil, fmt.Errorf("failed to import units: %w", err)
		}
	}

	// API Keys
	for _, k := range data.APIKeys {
		if _, err := qtx.CreateAPIKey(ctx, db.CreateAPIKeyParams{
			Key:         k.Key,
			Ident:       k.Ident,
			Disabled:    k.Disabled,
			SystemsJson: k.SystemsJson,
			Order:       k.Order,
		}); err != nil && !wsIsUniqueViolation(err) {
			return nil, fmt.Errorf("failed to import api keys: %w", err)
		}
	}

	// DirMonitors
	for _, d := range data.DirMonitors {
		if _, err := qtx.CreateDirMonitor(ctx, db.CreateDirMonitorParams{
			Directory:   d.Directory,
			Type:        d.Type,
			Mask:        d.Mask,
			Extension:   d.Extension,
			Frequency:   d.Frequency,
			Delay:       d.Delay,
			DeleteAfter: d.DeleteAfter,
			UsePolling:  d.UsePolling,
			Disabled:    d.Disabled,
			SystemID:    d.SystemID,
			TalkgroupID: d.TalkgroupID,
			Order:       d.Order,
		}); err != nil && !wsIsUniqueViolation(err) {
			return nil, fmt.Errorf("failed to import dirmonitors: %w", err)
		}
	}

	// Downstreams
	for _, d := range data.Downstreams {
		if !wsValidHTTPURL(d.Url) {
			slog.Warn("import config: skipping downstream with invalid URL", "url", d.Url)
			continue
		}
		if _, err := qtx.CreateDownstream(ctx, db.CreateDownstreamParams{
			Url:         d.Url,
			ApiKey:      d.ApiKey,
			SystemsJson: d.SystemsJson,
			Disabled:    d.Disabled,
			Order:       d.Order,
		}); err != nil && !wsIsUniqueViolation(err) {
			return nil, fmt.Errorf("failed to import downstreams: %w", err)
		}
	}

	// Webhooks
	for _, w := range data.Webhooks {
		if !wsValidHTTPURL(w.Url) {
			slog.Warn("import config: skipping webhook with invalid URL", "url", w.Url)
			continue
		}
		if _, err := qtx.CreateWebhook(ctx, db.CreateWebhookParams{
			Url:         w.Url,
			Type:        w.Type,
			Secret:      w.Secret,
			SystemsJson: w.SystemsJson,
			Disabled:    w.Disabled,
			Order:       w.Order,
		}); err != nil && !wsIsUniqueViolation(err) {
			return nil, fmt.Errorf("failed to import webhooks: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit import: %w", err)
	}

	slog.Info("config imported successfully via WS", "by", c.userID)
	return map[string]bool{"ok": true}, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// RADIOREFERENCE
// ══════════════════════════════════════════════════════════════════════════════

func (c *Client) opRadioReferenceApply(ctx context.Context, params json.RawMessage) (any, error) {
	type rrCandidate struct {
		Row         int     `json:"row"`
		TalkgroupID int64   `json:"talkgroupId"`
		Label       *string `json:"label,omitempty"`
		Name        *string `json:"name,omitempty"`
		Group       *string `json:"group,omitempty"`
		Tag         *string `json:"tag,omitempty"`
		Led         *string `json:"led,omitempty"`
		Order       *int64  `json:"order,omitempty"`
	}

	var req struct {
		SystemID       int64         `json:"systemId"`
		Candidates     []rrCandidate `json:"candidates"`
		MergeMode      string        `json:"mergeMode"`
		SelectedFields []string      `json:"selectedFields"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.SystemID <= 0 {
		return nil, userError("systemId is required")
	}
	if len(req.Candidates) == 0 {
		return nil, userError("candidates are required")
	}
	if len(req.Candidates) > 100_000 {
		return nil, userError("too many candidates")
	}
	if req.MergeMode == "" {
		req.MergeMode = "fill_missing"
	}
	if req.MergeMode != "fill_missing" && req.MergeMode != "overwrite_selected" {
		return nil, userError("mergeMode must be 'fill_missing' or 'overwrite_selected'")
	}
	if _, err := c.hub.queries.GetSystem(ctx, req.SystemID); err != nil {
		return nil, userError("system not found")
	}

	// Sanitize selected fields.
	rrUpdatable := map[string]bool{"label": true, "name": true, "group": true, "tag": true, "led": true, "order": true}
	selected := make([]string, 0, len(req.SelectedFields))
	for _, f := range req.SelectedFields {
		v := strings.ToLower(strings.TrimSpace(f))
		if rrUpdatable[v] {
			selected = append(selected, v)
		}
	}

	type rowErr struct {
		Row    int    `json:"row"`
		Reason string `json:"reason"`
	}
	resp := map[string]any{
		"processed": 0,
		"matched":   0,
		"updated":   0,
		"skipped":   0,
		"errors":    0,
		"rowErrors": []rowErr{},
	}
	processed, matched, updated, skippedCount, errCount := 0, 0, 0, 0, 0
	rowErrors := make([]rowErr, 0)

	for _, candidate := range req.Candidates {
		processed++

		tg, tgErr := c.hub.queries.GetTalkgroupBySystemAndTGID(ctx, db.GetTalkgroupBySystemAndTGIDParams{
			SystemID:    req.SystemID,
			TalkgroupID: candidate.TalkgroupID,
		})
		if tgErr != nil {
			if errors.Is(tgErr, sql.ErrNoRows) {
				skippedCount++
				rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "talkgroup not found in selected system"})
				continue
			}
			errCount++
			rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
			continue
		}
		matched++

		p := db.UpdateTalkgroupParams{
			ID:          tg.ID,
			TalkgroupID: tg.TalkgroupID,
			Label:       tg.Label,
			Name:        tg.Name,
			Frequency:   tg.Frequency,
			Led:         tg.Led,
			GroupID:     tg.GroupID,
			TagID:       tg.TagID,
			Order:       tg.Order,
		}

		// Determine which fields to apply.
		allow := map[string]bool{}
		if req.MergeMode == "overwrite_selected" {
			for _, f := range selected {
				allow[f] = true
			}
		}

		applyFields := make([]string, 0, 6)
		check := func(field string, hasCand bool, targetEmpty bool) {
			if !hasCand {
				return
			}
			if req.MergeMode == "overwrite_selected" {
				if allow[field] {
					applyFields = append(applyFields, field)
				}
				return
			}
			if targetEmpty {
				applyFields = append(applyFields, field)
			}
		}
		check("label", candidate.Label != nil, !tg.Label.Valid || strings.TrimSpace(tg.Label.String) == "")
		check("name", candidate.Name != nil, !tg.Name.Valid || strings.TrimSpace(tg.Name.String) == "")
		check("group", candidate.Group != nil, !tg.GroupID.Valid)
		check("tag", candidate.Tag != nil, !tg.TagID.Valid)
		check("led", candidate.Led != nil, !tg.Led.Valid || strings.TrimSpace(tg.Led.String) == "")
		check("order", candidate.Order != nil, tg.Order == 0)

		if len(applyFields) == 0 {
			skippedCount++
			continue
		}

		// Apply field updates.
		applyErr := false
		for _, field := range applyFields {
			switch field {
			case "label":
				if candidate.Label != nil {
					p.Label = sql.NullString{String: *candidate.Label, Valid: true}
				}
			case "name":
				if candidate.Name != nil {
					p.Name = sql.NullString{String: *candidate.Name, Valid: true}
				}
			case "group":
				if candidate.Group != nil {
					g, err := c.hub.queries.GetGroupByLabel(ctx, *candidate.Group)
					if err != nil {
						if errors.Is(err, sql.ErrNoRows) {
							newID, createErr := c.hub.queries.CreateGroup(ctx, *candidate.Group)
							if createErr != nil {
								errCount++
								rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
								applyErr = true
								break
							}
							p.GroupID = sql.NullInt64{Int64: newID, Valid: true}
						} else {
							errCount++
							rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
							applyErr = true
							break
						}
					} else {
						p.GroupID = sql.NullInt64{Int64: g.ID, Valid: true}
					}
				}
			case "tag":
				if candidate.Tag != nil {
					t, err := c.hub.queries.GetTagByLabel(ctx, *candidate.Tag)
					if err != nil {
						if errors.Is(err, sql.ErrNoRows) {
							newID, createErr := c.hub.queries.CreateTag(ctx, *candidate.Tag)
							if createErr != nil {
								errCount++
								rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
								applyErr = true
								break
							}
							p.TagID = sql.NullInt64{Int64: newID, Valid: true}
						} else {
							errCount++
							rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
							applyErr = true
							break
						}
					} else {
						p.TagID = sql.NullInt64{Int64: t.ID, Valid: true}
					}
				}
			case "led":
				if candidate.Led != nil {
					p.Led = sql.NullString{String: *candidate.Led, Valid: true}
				}
			case "order":
				if candidate.Order != nil {
					p.Order = *candidate.Order
				}
			}
		}
		if applyErr {
			continue
		}

		if err := c.hub.queries.UpdateTalkgroup(ctx, p); err != nil {
			errCount++
			rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
			continue
		}
		updated++
	}

	resp["processed"] = processed
	resp["matched"] = matched
	resp["updated"] = updated
	resp["skipped"] = skippedCount
	resp["errors"] = errCount
	resp["rowErrors"] = rowErrors
	return resp, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// TRANSCRIPTION MODEL MANAGEMENT
// ══════════════════════════════════════════════════════════════════════════════

// transcriptionBaseURL reads the transcriptionUrl setting from DB.
func (c *Client) transcriptionBaseURL(ctx context.Context) (string, error) {
	s, err := c.hub.queries.GetSetting(ctx, "transcriptionUrl")
	if err == nil && s.Value != "" && wsValidHTTPURL(s.Value) {
		return strings.TrimRight(s.Value, "/"), nil
	}
	// Fall back to the live manager's URL (e.g. when DB setting was just saved
	// but the query above fails due to timing).
	if tr := c.hub.deps.TranscriberReload; tr != nil {
		if u := tr.BaseURL(); u != "" {
			return strings.TrimRight(u, "/"), nil
		}
	}
	return "", userError("transcriptionUrl setting is not configured")
}

func (c *Client) opTranscriptionStatus(ctx context.Context, _ json.RawMessage) (any, error) {
	// Read settings from DB.
	getVal := func(key string) string {
		s, err := c.hub.queries.GetSetting(ctx, key)
		if err != nil {
			return ""
		}
		return s.Value
	}

	enabled := getVal("transcriptionEnabled") == "true"
	baseURL := getVal("transcriptionUrl")
	model := getVal("transcriptionModel")
	language := getVal("transcriptionLanguage")
	diarize := getVal("transcriptionDiarize") == "true"
	liveDisplay := getVal("liveTranscriptDisplay") == "true"

	// Check live connection to go-whisper.
	connected := false
	if baseURL != "" && wsValidHTTPURL(baseURL) {
		trimmed := strings.TrimRight(baseURL, "/")
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, trimmed+"/api/whisper/model", nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				connected = resp.StatusCode >= 200 && resp.StatusCode < 400
			}
		}
	}

	return map[string]any{
		"enabled":     enabled,
		"url":         baseURL,
		"model":       model,
		"language":    language,
		"diarize":     diarize,
		"liveDisplay": liveDisplay,
		"connected":   connected,
	}, nil
}

func (c *Client) opTranscriptionModels(ctx context.Context, _ json.RawMessage) (any, error) {
	baseURL, err := c.transcriptionBaseURL(ctx)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/api/whisper/model", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("go-whisper unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go-whisper returned status %d", resp.StatusCode)
	}

	var result json.RawMessage
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from go-whisper: %w", err)
	}
	return result, nil
}

func (c *Client) opTranscriptionDownload(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.Model == "" {
		return nil, userError("model name is required")
	}

	// go-whisper expects model names with .bin extension
	model := req.Model
	if !strings.HasSuffix(model, ".bin") {
		model += ".bin"
	}

	// tdrz (tinydiarize) models live in a different HuggingFace repo.
	// go-whisper's store accepts a full URL as the model path for non-default repos.
	if strings.Contains(model, "tdrz") {
		model = "https://huggingface.co/akashmjn/tinydiarize-whisper.cpp/resolve/main/ggml-" + strings.TrimPrefix(model, "ggml-")
	}

	baseURL, err := c.transcriptionBaseURL(ctx)
	if err != nil {
		return nil, err
	}

	reqBody, _ := json.Marshal(map[string]string{"model": model})

	// Model downloads can take a long time (500MB+).
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/api/whisper/model", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("go-whisper unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		slog.Warn("go-whisper model download failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("go-whisper returned status %d", resp.StatusCode)
	}

	var result json.RawMessage
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from go-whisper: %w", err)
	}
	return result, nil
}

func (c *Client) opTranscriptionDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, userError("invalid request body")
	}
	if req.ID == "" {
		return nil, userError("model id is required")
	}

	// Sanitise: model ID should be alphanumeric + hyphens/dots/underscores only.
	for _, ch := range req.ID {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || ch == '_') {
			return nil, userError("invalid model id")
		}
	}

	baseURL, err := c.transcriptionBaseURL(ctx)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, baseURL+"/api/whisper/model/"+req.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("go-whisper unreachable: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go-whisper returned status %d", resp.StatusCode)
	}

	return map[string]any{"deleted": true}, nil
}

func (c *Client) opTranscriptionStats(ctx context.Context, _ json.RawMessage) (any, error) {
	// DB aggregate stats — "recent" = last 24 hours.
	since := time.Now().Add(-24 * time.Hour).Unix()
	stats, err := c.hub.queries.TranscriptionStats(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("query transcription stats: %w", err)
	}

	byLang, err := c.hub.queries.TranscriptionsByLanguage(ctx)
	if err != nil {
		return nil, fmt.Errorf("query transcriptions by language: %w", err)
	}

	byModel, err := c.hub.queries.TranscriptionsByModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("query transcriptions by model: %w", err)
	}

	// Pool stats (live).
	queueDepth := 0
	poolEnabled := false
	if tr := c.hub.deps.TranscriberReload; tr != nil {
		poolEnabled = tr.Enabled()
		queueDepth = tr.QueueDepth()
	}

	// Convert interface{} values from COALESCE/AVG to int64.
	toInt64 := func(v interface{}) int64 {
		switch n := v.(type) {
		case int64:
			return n
		case float64:
			return int64(n)
		default:
			return 0
		}
	}

	langBreakdown := make([]map[string]any, 0, len(byLang))
	for _, l := range byLang {
		langBreakdown = append(langBreakdown, map[string]any{
			"language": l.Lang,
			"count":    l.Cnt,
		})
	}

	modelBreakdown := make([]map[string]any, 0, len(byModel))
	for _, m := range byModel {
		modelBreakdown = append(modelBreakdown, map[string]any{
			"model": m.ModelName,
			"count": m.Cnt,
		})
	}

	return map[string]any{
		"total":         stats.Total,
		"recent24h":     stats.RecentCount,
		"avgDurationMs": toInt64(stats.AvgDurationMs),
		"minDurationMs": toInt64(stats.MinDurationMs),
		"maxDurationMs": toInt64(stats.MaxDurationMs),
		"queueDepth":    queueDepth,
		"poolEnabled":   poolEnabled,
		"byLanguage":    langBreakdown,
		"byModel":       modelBreakdown,
	}, nil
}
