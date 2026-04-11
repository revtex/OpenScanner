// Package api — call upload (POST /api/call-upload, /api/trunk-recorder-call-upload).
package api

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/downstream"
	"github.com/openscanner/openscanner/internal/ws"
)

const (
	defaultCallRatePerMin = 60
	rateWindowDuration    = time.Minute
)

// apiKeyLimiter is a per-API-key sliding-window rate limiter.
type apiKeyLimiter struct {
	mu          sync.Mutex
	windowStart time.Time
	count       int
	rateLimit   int
}

func (l *apiKeyLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.windowStart) >= rateWindowDuration {
		l.windowStart = now
		l.count = 0
	}
	if l.count >= l.rateLimit {
		return false
	}
	l.count++
	return true
}

// CallHandler handles call upload endpoints.
type CallHandler struct {
	queries    *db.Queries
	processor  *audio.Processor
	hub        *ws.Hub
	dsNotifier DownstreamNotifier
	mu         sync.Mutex
	limiters   map[int64]*apiKeyLimiter
}

// NewCallHandler creates a CallHandler.
func NewCallHandler(queries *db.Queries, processor *audio.Processor, hub *ws.Hub, dsNotifier DownstreamNotifier) *CallHandler {
	return &CallHandler{
		queries:    queries,
		processor:  processor,
		hub:        hub,
		dsNotifier: dsNotifier,
		limiters:   make(map[int64]*apiKeyLimiter),
	}
}

func (h *CallHandler) getLimiter(apiKeyID int64, rateLimit int) *apiKeyLimiter {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Periodic cleanup: remove limiters whose window has expired to bound
	// memory growth (one entry per unique API key ID).
	if len(h.limiters) > 100 {
		now := time.Now()
		for id, l := range h.limiters {
			l.mu.Lock()
			stale := now.Sub(l.windowStart) >= 2*rateWindowDuration
			l.mu.Unlock()
			if stale {
				delete(h.limiters, id)
			}
		}
	}

	l, ok := h.limiters[apiKeyID]
	if !ok {
		l = &apiKeyLimiter{
			windowStart: time.Now(),
			rateLimit:   rateLimit,
		}
		h.limiters[apiKeyID] = l
	}
	return l
}

// getSettingValue fetches a setting value from the DB, returning "" on error.
func (h *CallHandler) getSettingValue(c *gin.Context, key string) string {
	s, err := h.queries.GetSetting(c.Request.Context(), key)
	if err != nil {
		return ""
	}
	return s.Value
}

// PostCallUpload handles POST /api/call-upload and /api/trunk-recorder-call-upload.
func (h *CallHandler) PostCallUpload(c *gin.Context) {
	// Retrieve API key ID injected by APIKeyAuth middleware.
	apiKeyIDVal, exists := c.Get("apiKeyID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
		return
	}
	apiKeyID, ok := apiKeyIDVal.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Per-API-key rate limiting.
	rateLimit := defaultCallRatePerMin
	if rStr := h.getSettingValue(c, "apiKeyCallRate"); rStr != "" {
		if r, err := strconv.Atoi(rStr); err == nil && r > 0 {
			rateLimit = r
		}
	}
	if !h.getLimiter(apiKeyID, rateLimit).allow() {
		slog.Warn("call upload rate limit exceeded", "api_key_id", apiKeyID)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}

	// Parse required fields.
	dateTimeStr := c.PostForm("dateTime")
	if dateTimeStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dateTime is required"})
		return
	}
	dateTimeUnix, err := strconv.ParseInt(dateTimeStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dateTime"})
		return
	}
	callTime := time.Unix(dateTimeUnix, 0)

	systemIDStr := c.PostForm("systemId")
	if systemIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "systemId is required"})
		return
	}
	systemIDRaw, err := strconv.ParseInt(systemIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid systemId"})
		return
	}

	tgIDStr := c.PostForm("talkgroupId")
	if tgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "talkgroupId is required"})
		return
	}
	talkgroupIDRaw, err := strconv.ParseInt(tgIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid talkgroupId"})
		return
	}

	// Parse optional fields.
	var frequency, duration, source sql.NullInt64
	if v := c.PostForm("frequency"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			frequency = sql.NullInt64{Int64: n, Valid: true}
		}
	}
	if v := c.PostForm("duration"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			duration = sql.NullInt64{Int64: n, Valid: true}
		}
	}
	if v := c.PostForm("source"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			source = sql.NullInt64{Int64: n, Valid: true}
		}
	}

	var sourcesJSON, frequenciesJSON, patchesJSON sql.NullString
	if v := c.PostForm("sources"); v != "" {
		sourcesJSON = sql.NullString{String: v, Valid: true}
	}
	if v := c.PostForm("frequencies"); v != "" {
		frequenciesJSON = sql.NullString{String: v, Valid: true}
	}
	if v := c.PostForm("patches"); v != "" {
		patchesJSON = sql.NullString{String: v, Valid: true}
	}

	ctx := c.Request.Context()
	autoPopulate := h.getSettingValue(c, "autoPopulate") == "true"

	// Resolve system by its radio system_id.
	system, err := h.queries.GetSystemBySystemID(ctx, systemIDRaw)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to query system", "system_id", systemIDRaw, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if !autoPopulate {
			c.JSON(http.StatusBadRequest, gin.H{"error": "system not found"})
			return
		}
		label := strconv.FormatInt(systemIDRaw, 10)
		newID, cerr := h.queries.CreateSystem(ctx, db.CreateSystemParams{
			SystemID:     systemIDRaw,
			Label:        label,
			AutoPopulate: 1,
		})
		if cerr != nil {
			slog.Error("failed to auto-create system", "system_id", systemIDRaw, "error", cerr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		slog.Info("auto-populated system", "system_id", systemIDRaw, "db_id", newID)
		system = db.System{ID: newID, SystemID: systemIDRaw, Label: label, AutoPopulate: 1}
	}

	// Resolve talkgroup by system DB ID + radio talkgroup ID.
	talkgroup, err := h.queries.GetTalkgroupBySystemAndTGID(ctx, db.GetTalkgroupBySystemAndTGIDParams{
		SystemID:    system.ID,
		TalkgroupID: talkgroupIDRaw,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to query talkgroup", "system_id", system.ID, "talkgroup_id", talkgroupIDRaw, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if !autoPopulate {
			c.JSON(http.StatusBadRequest, gin.H{"error": "talkgroup not found"})
			return
		}
		newID, cerr := h.queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
			SystemID:    system.ID,
			TalkgroupID: talkgroupIDRaw,
		})
		if cerr != nil {
			slog.Error("failed to auto-create talkgroup", "talkgroup_id", talkgroupIDRaw, "error", cerr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		slog.Info("auto-populated talkgroup", "talkgroup_id", talkgroupIDRaw, "db_id", newID)
		talkgroup = db.Talkgroup{ID: newID, SystemID: system.ID, TalkgroupID: talkgroupIDRaw}
	}

	// Duplicate detection (system.ID and talkgroup.ID are the FK values in calls).
	if h.getSettingValue(c, "disableDuplicateDetection") != "true" {
		windowMs := int64(2000)
		if v := h.getSettingValue(c, "duplicateDetectionTimeFrame"); v != "" {
			if wm, err := strconv.ParseInt(v, 10, 64); err == nil {
				windowMs = wm
			}
		}
		dup, derr := audio.IsDuplicate(ctx, h.queries, system.ID, talkgroup.ID, callTime, windowMs)
		if derr != nil {
			slog.Error("duplicate detection failed", "error", derr)
			// Non-fatal: proceed with ingest.
		} else if dup {
			slog.Info("duplicate call rejected", "system_id", systemIDRaw, "talkgroup_id", talkgroupIDRaw)
			c.JSON(http.StatusOK, gin.H{"message": "duplicate"})
			return
		}
	}

	// Get uploaded audio file.
	fh, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "audio file is required"})
		return
	}

	// Resolve audio conversion mode from settings.
	convMode := audio.ConversionEnabled
	if mStr := h.getSettingValue(c, "audioConversion"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil {
			convMode = audio.ConversionMode(m)
		}
	}

	// Store audio file (conversion handled inside Processor.Store).
	relPath, err := h.processor.Store(ctx, fh, convMode)
	if err != nil {
		slog.Error("failed to store audio file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store audio"})
		return
	}

	// Determine audio MIME type.
	// When conversion is enabled the output is always AAC.
	// Otherwise validate the client-supplied Content-Type against an allowlist
	// to prevent attacker-controlled MIME types from reaching the database.
	var audioType string
	if convMode != audio.ConversionDisabled {
		audioType = "audio/aac"
	} else {
		switch fh.Header.Get("Content-Type") {
		case "audio/mpeg", "audio/mp3", "audio/wav", "audio/x-wav",
			"audio/ogg", "audio/aac", "audio/m4a", "audio/mp4",
			"audio/x-m4a", "audio/opus":
			audioType = fh.Header.Get("Content-Type")
		default:
			audioType = "application/octet-stream"
		}
	}

	// Insert call record.
	callID, err := h.queries.CreateCall(ctx, db.CreateCallParams{
		AudioPath:       relPath,
		AudioName:       filepath.Base(relPath),
		AudioType:       audioType,
		DateTime:        dateTimeUnix,
		Frequency:       frequency,
		Duration:        duration,
		Source:          source,
		SourcesJson:     sourcesJSON,
		FrequenciesJson: frequenciesJSON,
		PatchesJson:     patchesJSON,
		SystemID:        system.ID,
		TalkgroupID:     sql.NullInt64{Int64: talkgroup.ID, Valid: true},
	})
	if err != nil {
		slog.Error("failed to insert call", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	slog.Info("call ingested", "id", callID)

	// Broadcast to WebSocket listeners.
	if h.hub != nil {
		calPayload := map[string]any{
			"id":          callID,
			"audioName":   filepath.Base(relPath),
			"audioType":   audioType,
			"dateTime":    dateTimeUnix,
			"systemId":    system.SystemID,
			"system":      system.ID,
			"talkgroupId": talkgroup.TalkgroupID,
			"talkgroup":   talkgroup.ID,
		}
		if frequency.Valid {
			calPayload["frequency"] = frequency.Int64
		}
		if duration.Valid {
			calPayload["duration"] = duration.Int64
		}
		if source.Valid {
			calPayload["source"] = source.Int64
		}
		calMsg, err := ws.NewCALMessage(calPayload)
		if err != nil {
			slog.Error("failed to build CAL message", "error", err)
		} else {
			audioFullPath := filepath.Join(h.processor.BaseDir(), relPath)
			if rel, pathErr := filepath.Rel(h.processor.BaseDir(), audioFullPath); pathErr != nil || strings.HasPrefix(rel, "..") {
				slog.Error("audio path escapes base directory", "path", relPath)
			} else {
				audioBytes, readErr := os.ReadFile(audioFullPath)
				if readErr != nil {
					slog.Warn("failed to read audio for WS broadcast", "path", rel, "error", readErr)
				}
				h.hub.BroadcastCAL(calMsg, audioBytes, func(cl *ws.Client) bool {
					return cl.CanReceive(system.ID, talkgroup.ID)
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"id": callID})

	// Notify downstream pushers (non-blocking, after response is sent).
	if h.dsNotifier != nil {
		h.dsNotifier.Notify(downstream.CallEvent{
			CallID:      callID,
			AudioPath:   relPath,
			AudioName:   filepath.Base(relPath),
			AudioType:   audioType,
			DateTime:    dateTimeUnix,
			SystemID:    system.SystemID,
			System:      system.ID,
			TalkgroupID: talkgroup.TalkgroupID,
			Talkgroup:   talkgroup.ID,
			Frequency:   frequency.Int64,
			Duration:    duration.Int64,
			Source:      source.Int64,
			Sources:     sourcesJSON.String,
			Frequencies: frequenciesJSON.String,
			Patches:     patchesJSON.String,
		})
	}
}
