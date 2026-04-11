package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/ws"
)

// allowedSettingKeys is the set of known configuration keys that may be
// written via the admin config API (defence-in-depth: prevents arbitrary keys
// from being persisted).
var allowedSettingKeys = map[string]bool{
	"audioConversion":             true,
	"autoPopulate":                true,
	"branding":                    true,
	"disableDuplicateDetection":   true,
	"duplicateDetectionTimeFrame": true,
	"email":                       true,
	"maxClients":                  true,
	"pruneDays":                   true,
	"publicAccess":                true,
	"apiKeyCallRate":              true,
	"keypadBeeps":                 true,
	"dimmerDelay":                 true,
	"playbackGoesLive":            true,
	"showListenersCount":          true,
	"sortTalkgroups":              true,
	"tagsToggle":                  true,
	"searchPatchedTalkgroups":     true,
	"audioTranscription":          true,
	"whisperModel":                true,
}

// GetConfig handles GET /api/admin/config.
// Returns all settings as a JSON object {key: value, ...}.
func (h *AdminHandler) GetConfig(c *gin.Context) {
	settings, err := h.queries.ListSettings(c.Request.Context())
	if err != nil {
		slog.Error("failed to list settings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list settings"})
		return
	}

	c.JSON(http.StatusOK, toSettingResponses(settings))
}

// PutConfig handles PUT /api/admin/config.
// Accepts a JSON object of key/value pairs, upserts each setting.
// Broadcasts CFG to all WS clients after update.
func (h *AdminHandler) PutConfig(c *gin.Context) {
	var settings []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Validate all keys before writing anything (defence-in-depth: avoid
	// partial writes if a disallowed key appears mid-slice).
	for _, s := range settings {
		if !allowedSettingKeys[s.Key] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown setting key: " + s.Key})
			return
		}
	}

	ctx := c.Request.Context()

	// Wrap all upserts in a transaction so the update is atomic.
	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin config transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := h.queries.WithTx(tx)

	for _, s := range settings {
		if err := qtx.UpsertSetting(ctx, db.UpsertSettingParams{Key: s.Key, Value: s.Value}); err != nil {
			slog.Error("failed to upsert setting", "key", s.Key, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit config transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	// Broadcast updated config to all WS clients.
	allSettings, err := h.queries.ListSettings(ctx)
	if err != nil {
		slog.Warn("failed to reload settings for CFG broadcast", "error", err)
	} else {
		cfgMap := make(map[string]string, len(allSettings))
		for _, s := range allSettings {
			cfgMap[s.Key] = s.Value
		}
		msg, err := ws.NewCFGMessage(cfgMap)
		if err != nil {
			slog.Warn("failed to build CFG message", "error", err)
		} else {
			h.hub.Broadcast(msg, nil)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
