package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

// configExport is the full JSON config export structure.
type configExport struct {
	Settings    []db.Setting    `json:"settings"`
	Users       []db.User       `json:"users"`
	Systems     []db.System     `json:"systems"`
	Talkgroups  []db.Talkgroup  `json:"talkgroups"`
	Units       []db.Unit       `json:"units"`
	Groups      []db.Group      `json:"groups"`
	Tags        []db.Tag        `json:"tags"`
	APIKeys     []db.ApiKey     `json:"apiKeys"`
	Accesses    []db.Access     `json:"accesses"`
	Dirwatches  []db.Dirwatch   `json:"dirwatches"`
	Downstreams []db.Downstream `json:"downstreams"`
	Webhooks    []db.Webhook    `json:"webhooks"`
}

// ExportConfig handles GET /api/admin/export/config.
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
	accesses, err := h.queries.ListAccesses(ctx)
	if err != nil {
		slog.Error("export: failed to list accesses", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export accesses"})
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

	export := configExport{
		Settings:    settings,
		Users:       users,
		Systems:     systems,
		Talkgroups:  talkgroups,
		Units:       units,
		Groups:      groups,
		Tags:        tags,
		APIKeys:     apiKeys,
		Accesses:    accesses,
		Dirwatches:  dirwatches,
		Downstreams: downstreams,
		Webhooks:    webhooks,
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
	Accesses    []db.Access     `json:"accesses"`
	Dirwatches  []db.Dirwatch   `json:"dirwatches"`
	Downstreams []db.Downstream `json:"downstreams"`
	Webhooks    []db.Webhook    `json:"webhooks"`
}

// ImportConfig handles POST /api/admin/import/config.
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

	// Accesses
	for _, a := range data.Accesses {
		if _, err := qtx.CreateAccess(ctx, db.CreateAccessParams{
			Code:        a.Code,
			Ident:       a.Ident,
			Expiration:  a.Expiration,
			Limit:       a.Limit,
			SystemsJson: a.SystemsJson,
			Order:       a.Order,
		}); err != nil {
			if !isUniqueViolation(err) {
				slog.Error("import config: failed to create access", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import accesses"})
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
