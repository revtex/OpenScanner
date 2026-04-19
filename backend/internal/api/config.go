package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/logging"
	"github.com/openscanner/openscanner/internal/ws"
)

// allowedSettingKeys is the set of known configuration keys that may be
// written via the admin config API (defence-in-depth: prevents arbitrary keys
// from being persisted).
var allowedSettingKeys = map[string]bool{
	"activityDashboard":           true,
	"afsSystems":                  true,
	"apiKeyCallRate":              true,
	"audioConversion":             true,
	"audioEncodingPreset":         true,
	"autoPopulate":                true,
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
	"showListenersCount":          true,
	"sortTalkgroups":              true,
	"tagsToggle":                  true,
	"time12hFormat":               true,
	"transcriptionBinary":         true,
	"transcriptionEnabled":        true,
	"transcriptionLanguage":       true,
	"transcriptionModel":          true,
	"vapidPrivateKey":             true,
	"vapidPublicKey":              true,
	"webhooksEnabled":             true,
}

// GetConfig handles GET /api/admin/config.
// Returns all settings as a JSON object {key: value, ...}.
//
// @Summary      List all settings
// @Description  Returns every configuration key-value pair.
// @Tags         Admin
// @Produce      json
// @Success      200  {array}   settingResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/config [get]
func (h *AdminHandler) GetConfig(c *gin.Context) {
	requestID, _ := c.Get("requestID")
	settings, err := h.queries.ListSettings(c.Request.Context())
	if err != nil {
		slog.Error("failed to list settings", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list settings"})
		return
	}

	slog.Debug("admin: config fetched", "request_id", requestID, "count", len(settings))

	c.JSON(http.StatusOK, gin.H{
		"settings": toSettingResponses(settings),
		"capabilities": gin.H{
			"ffmpeg":  h.ffmpegAvailable,
			"fdkAac":  h.fdkAACAvailable,
			"whisper": h.whisperAvailable,
		},
	})
}

// PutConfig handles PUT /api/admin/config.
// Accepts a JSON object of key/value pairs, upserts each setting.
// Broadcasts CFG to all WS clients after update.
//
// @Summary      Update settings
// @Description  Upserts one or more configuration key-value pairs and broadcasts CFG to WebSocket clients.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        settings  body  []settingResponse  true  "Array of key/value settings"
// @Success      200  {object}  object  "ok: true"
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/config [put]
func (h *AdminHandler) PutConfig(c *gin.Context) {
	requestID, _ := c.Get("requestID")
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
		if s.Key == "logLevel" {
			if _, ok := logging.ParseLevel(s.Value); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid logLevel; expected debug, info, warn, or error"})
				return
			}
		}
		if s.Key == "audioEncodingPreset" {
			if !audio.IsValidEncodingPreset(s.Value) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid audioEncodingPreset value"})
				return
			}
			if audio.IsHEEncodingPreset(s.Value) && !h.fdkAACAvailable {
				c.JSON(http.StatusBadRequest, gin.H{"error": "selected HE-AAC preset requires libfdk_aac support in ffmpeg"})
				return
			}
		}
	}

	// Reject enabling audio conversion when ffmpeg is not installed.
	for _, s := range settings {
		if s.Key == "audioConversion" {
			if v, err := strconv.Atoi(s.Value); err == nil && v != 0 && !h.ffmpegAvailable {
				c.JSON(http.StatusBadRequest, gin.H{"error": "ffmpeg is not installed — install it and restart the service to enable audio conversion"})
				return
			}
		}
	}

	ctx := c.Request.Context()

	// Wrap all upserts in a transaction so the update is atomic.
	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin config transaction", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := h.queries.WithTx(tx)

	for _, s := range settings {
		if err := qtx.UpsertSetting(ctx, db.UpsertSettingParams{Key: s.Key, Value: s.Value}); err != nil {
			slog.Error("failed to upsert setting", "request_id", requestID, "key", s.Key, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit config transaction", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	actorID, _ := c.Get("userID")
	// Log each changed setting (key=value), redacting sensitive keys.
	for _, s := range settings {
		v := s.Value
		if s.Key == "vapidPrivateKey" {
			v = "[REDACTED]"
		}
		slog.Info("admin: config updated", "request_id", requestID, "key", s.Key, "value", v, "by", actorID)
	}

	// Log level change specifically.
	for _, s := range settings {
		if s.Key == "logLevel" {
			slog.Info("admin: log level changed", "request_id", requestID, "level", s.Value, "by", actorID)
			break
		}
	}

	for _, s := range settings {
		if s.Key == "logLevel" {
			if err := logging.SetLevel(s.Value); err != nil {
				slog.Warn("invalid logLevel setting, keeping previous runtime level", "value", s.Value, "error", err)
			}
			break
		}
	}

	// Broadcast updated config to all WS clients.
	allSettings, err := h.queries.ListSettings(ctx)
	if err != nil {
		slog.Warn("failed to reload settings for CFG broadcast", "request_id", requestID, "error", err)
	} else {
		cfgMap := make(map[string]string, len(allSettings))
		for _, s := range allSettings {
			cfgMap[s.Key] = s.Value
		}
		msg, err := ws.NewCFGMessage(cfgMap)
		if err != nil {
			slog.Warn("failed to build CFG message", "request_id", requestID, "error", err)
		} else {
			h.hub.Broadcast(msg, nil)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
	h.hub.BroadcastAdminEvent("config.updated", nil)
}
