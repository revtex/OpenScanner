package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

// ── Export-safe types that strip secrets ──

type exportAPIKey struct {
	ID          int64   `json:"id"`
	Ident       *string `json:"ident,omitempty"`
	Disabled    int64   `json:"disabled"`
	SystemsJson *string `json:"systemsJson,omitempty"`
	Order       int64   `json:"order"`
}

type exportDownstream struct {
	ID          int64   `json:"id"`
	Url         string  `json:"url"`
	SystemsJson *string `json:"systemsJson,omitempty"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
}

type exportWebhook struct {
	ID          int64   `json:"id"`
	Url         string  `json:"url"`
	Type        string  `json:"type"`
	SystemsJson *string `json:"systemsJson,omitempty"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
}

// configExport is the full JSON config export structure.
// Sensitive fields (API key hashes, downstream API keys, webhook secrets)
// are deliberately excluded.
type configExport struct {
	Settings    []db.Setting       `json:"settings"`
	Users       []db.User          `json:"users"`
	Systems     []db.System        `json:"systems"`
	Talkgroups  []db.Talkgroup     `json:"talkgroups"`
	Units       []db.Unit          `json:"units"`
	Groups      []db.Group         `json:"groups"`
	Tags        []db.Tag           `json:"tags"`
	APIKeys     []exportAPIKey     `json:"apiKeys"`
	Dirwatches  []db.Dirwatch      `json:"dirwatches"`
	Downstreams []exportDownstream `json:"downstreams"`
	Webhooks    []exportWebhook    `json:"webhooks"`
}

// ExportConfig handles GET /api/admin/export/config.
//
// @Summary      Export full configuration
// @Description  Returns the entire server configuration as a JSON download. Sensitive fields (API key hashes, downstream API keys, webhook secrets) are excluded.
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  configExport
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/export/config [get]
func (h *AdminHandler) ExportConfig(c *gin.Context) {
	ctx := c.Request.Context()

	settings, err := h.queries.ListSettings(ctx)
	if err != nil {
		slog.Error("export: failed to list settings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export settings"})
		return
	}
	users, err := h.queries.ListUsers(ctx)
	if err != nil {
		slog.Error("export: failed to list users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export users"})
		return
	}
	systems, err := h.queries.ListSystems(ctx)
	if err != nil {
		slog.Error("export: failed to list systems", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export systems"})
		return
	}
	talkgroups, err := h.queries.ListAllTalkgroups(ctx)
	if err != nil {
		slog.Error("export: failed to list talkgroups", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export talkgroups"})
		return
	}
	units, err := h.queries.ListAllUnits(ctx)
	if err != nil {
		slog.Error("export: failed to list units", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export units"})
		return
	}
	groups, err := h.queries.ListGroups(ctx)
	if err != nil {
		slog.Error("export: failed to list groups", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export groups"})
		return
	}
	tags, err := h.queries.ListTags(ctx)
	if err != nil {
		slog.Error("export: failed to list tags", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export tags"})
		return
	}
	apiKeys, err := h.queries.ListAPIKeys(ctx)
	if err != nil {
		slog.Error("export: failed to list api keys", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export api keys"})
		return
	}
	dirwatches, err := h.queries.ListDirwatches(ctx)
	if err != nil {
		slog.Error("export: failed to list dirwatches", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export dirwatches"})
		return
	}
	downstreams, err := h.queries.ListDownstreams(ctx)
	if err != nil {
		slog.Error("export: failed to list downstreams", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export downstreams"})
		return
	}
	webhooks, err := h.queries.ListWebhooks(ctx)
	if err != nil {
		slog.Error("export: failed to list webhooks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export webhooks"})
		return
	}

	// Sanitize sensitive fields before export.
	safeAPIKeys := make([]exportAPIKey, len(apiKeys))
	for i, k := range apiKeys {
		safeAPIKeys[i] = exportAPIKey{
			ID:          k.ID,
			Ident:       nullStr(k.Ident),
			Disabled:    k.Disabled,
			SystemsJson: nullStr(k.SystemsJson),
			Order:       k.Order,
		}
	}
	safeDownstreams := make([]exportDownstream, len(downstreams))
	for i, d := range downstreams {
		safeDownstreams[i] = exportDownstream{
			ID:          d.ID,
			Url:         d.Url,
			SystemsJson: nullStr(d.SystemsJson),
			Disabled:    d.Disabled,
			Order:       d.Order,
		}
	}
	safeWebhooks := make([]exportWebhook, len(webhooks))
	for i, w := range webhooks {
		safeWebhooks[i] = exportWebhook{
			ID:          w.ID,
			Url:         w.Url,
			Type:        w.Type,
			SystemsJson: nullStr(w.SystemsJson),
			Disabled:    w.Disabled,
			Order:       w.Order,
		}
	}

	export := configExport{
		Settings:    settings,
		Users:       users,
		Systems:     systems,
		Talkgroups:  talkgroups,
		Units:       units,
		Groups:      groups,
		Tags:        tags,
		APIKeys:     safeAPIKeys,
		Dirwatches:  dirwatches,
		Downstreams: safeDownstreams,
		Webhooks:    safeWebhooks,
	}

	c.Header("Content-Disposition", `attachment; filename="openscanner-config.json"`)
	c.JSON(http.StatusOK, export)
}

// configImport is the JSON structure accepted by the config import endpoint.
type configImport struct {
	Settings    []db.Setting    `json:"settings"`
	Groups      []db.Group      `json:"groups"`
	Tags        []db.Tag        `json:"tags"`
	Systems     []db.System     `json:"systems"`
	Talkgroups  []db.Talkgroup  `json:"talkgroups"`
	Units       []db.Unit       `json:"units"`
	APIKeys     []db.ApiKey     `json:"apiKeys"`
	Dirwatches  []db.Dirwatch   `json:"dirwatches"`
	Downstreams []db.Downstream `json:"downstreams"`
	Webhooks    []db.Webhook    `json:"webhooks"`
}

// ImportConfig handles POST /api/admin/import/config.
//
// @Summary      Import full configuration
// @Description  Imports settings, groups, tags, systems, talkgroups, units, API keys, dirwatches, downstreams, and webhooks from a JSON body. Existing records are upserted or skipped on conflict.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        config  body  configImport  true  "Configuration data to import"
// @Success      200  {object}  object  "ok: true"
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/import/config [post]
func (h *AdminHandler) ImportConfig(c *gin.Context) {
	ctx := c.Request.Context()

	var data configImport
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("import config: failed to begin transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := h.queries.WithTx(tx)

	// Settings
	for _, s := range data.Settings {
		if !allowedSettingKeys[s.Key] {
			slog.Warn("import config: skipping unknown setting key", "key", s.Key)
			continue
		}
		if err := qtx.UpsertSetting(ctx, db.UpsertSettingParams(s)); err != nil {
			slog.Error("import config: failed to upsert setting", "key", s.Key, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import settings"})
			return
		}
	}

	// Groups
	for _, g := range data.Groups {
		if _, err := qtx.CreateGroup(ctx, g.Label); err != nil {
			if !isUniqueViolation(err) {
				slog.Error("import config: failed to create group", "label", g.Label, "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import groups"})
				return
			}
		}
	}

	// Tags
	for _, t := range data.Tags {
		if _, err := qtx.CreateTag(ctx, t.Label); err != nil {
			if !isUniqueViolation(err) {
				slog.Error("import config: failed to create tag", "label", t.Label, "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import tags"})
				return
			}
		}
	}

	// Systems
	for _, s := range data.Systems {
		if _, err := qtx.CreateSystem(ctx, db.CreateSystemParams{
			SystemID:       s.SystemID,
			Label:          s.Label,
			AutoPopulate:   s.AutoPopulate,
			BlacklistsJson: s.BlacklistsJson,
			Led:            s.Led,
			Order:          s.Order,
		}); err != nil {
			if !isUniqueViolation(err) {
				slog.Error("import config: failed to create system", "system_id", s.SystemID, "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import systems"})
				return
			}
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
			slog.Error("import config: failed to upsert talkgroup", "talkgroup_id", tg.TalkgroupID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import talkgroups"})
			return
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
			slog.Error("import config: failed to upsert unit", "unit_id", u.UnitID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import units"})
			return
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
		}); err != nil {
			if !isUniqueViolation(err) {
				slog.Error("import config: failed to create api key", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import api keys"})
				return
			}
		}
	}

	// Dirwatches
	for _, d := range data.Dirwatches {
		if _, err := qtx.CreateDirwatch(ctx, db.CreateDirwatchParams{
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
		}); err != nil {
			if !isUniqueViolation(err) {
				slog.Error("import config: failed to create dirwatch", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import dirwatches"})
				return
			}
		}
	}

	// Downstreams
	for _, d := range data.Downstreams {
		if !validHTTPURL(d.Url) {
			slog.Warn("import config: skipping downstream with invalid URL", "url", d.Url)
			continue
		}
		if _, err := qtx.CreateDownstream(ctx, db.CreateDownstreamParams{
			Url:         d.Url,
			ApiKey:      d.ApiKey,
			SystemsJson: d.SystemsJson,
			Disabled:    d.Disabled,
			Order:       d.Order,
		}); err != nil {
			if !isUniqueViolation(err) {
				slog.Error("import config: failed to create downstream", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import downstreams"})
				return
			}
		}
	}

	// Webhooks
	for _, w := range data.Webhooks {
		if !validHTTPURL(w.Url) {
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
		}); err != nil {
			if !isUniqueViolation(err) {
				slog.Error("import config: failed to create webhook", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import webhooks"})
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("import config: failed to commit transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit import"})
		return
	}

	slog.Info("config imported successfully")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
