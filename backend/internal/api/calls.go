// Package api — call upload (POST /api/call-upload, /api/trunk-recorder-call-upload).
package api

import (
	"database/sql"
	"encoding/json"
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
	maxCallRatePerMin     = 600
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
	if rateLimit > maxCallRatePerMin {
		rateLimit = maxCallRatePerMin
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

	// Optional call metadata fields.
	var siteCol, channelCol, decoderCol sql.NullString
	if v := c.PostForm("site"); v != "" {
		siteCol = sql.NullString{String: v, Valid: true}
	}
	if v := c.PostForm("channel"); v != "" {
		channelCol = sql.NullString{String: v, Valid: true}
	}
	if v := c.PostForm("decoder"); v != "" {
		decoderCol = sql.NullString{String: v, Valid: true}
	}

	// Optional talkgroup metadata for auto-populate / backfill.
	talkgroupLabel := c.PostForm("talkgroupLabel")
	talkgroupTag := c.PostForm("talkgroupTag")

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
		h.hub.BroadcastCFG(ctx)
	}

	// Blacklist check: reject calls to blacklisted talkgroups.
	if isBlacklistedTG(system.BlacklistsJson, talkgroupIDRaw) {
		slog.Info("call upload: talkgroup is blacklisted",
			"system_id", systemIDRaw, "talkgroup_id", talkgroupIDRaw)
		c.JSON(http.StatusOK, gin.H{"message": "blacklisted"})
		return
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
		var tgLabel, tgName sql.NullString
		if talkgroupLabel != "" {
			tgLabel = sql.NullString{String: talkgroupLabel, Valid: true}
		}
		if talkgroupTag != "" {
			tgName = sql.NullString{String: talkgroupTag, Valid: true}
		}
		newID, cerr := h.queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
			SystemID:    system.ID,
			TalkgroupID: talkgroupIDRaw,
			Label:       tgLabel,
			Name:        tgName,
		})
		if cerr != nil {
			slog.Error("failed to auto-create talkgroup", "talkgroup_id", talkgroupIDRaw, "error", cerr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		slog.Info("auto-populated talkgroup", "talkgroup_id", talkgroupIDRaw, "db_id", newID)
		talkgroup = db.Talkgroup{ID: newID, SystemID: system.ID, TalkgroupID: talkgroupIDRaw, Label: tgLabel, Name: tgName}
		h.hub.BroadcastCFG(ctx)
	} else if !talkgroup.Name.Valid && talkgroupTag != "" {
		// Existing talkgroup has no name — backfill from upload metadata.
		talkgroup.Name = sql.NullString{String: talkgroupTag, Valid: true}
		if !talkgroup.Label.Valid && talkgroupLabel != "" {
			talkgroup.Label = sql.NullString{String: talkgroupLabel, Valid: true}
		}
		if uerr := h.queries.UpdateTalkgroup(ctx, db.UpdateTalkgroupParams{
			ID:          talkgroup.ID,
			TalkgroupID: talkgroup.TalkgroupID,
			Label:       talkgroup.Label,
			Name:        talkgroup.Name,
			Frequency:   talkgroup.Frequency,
			Led:         talkgroup.Led,
			GroupID:     talkgroup.GroupID,
			TagID:       talkgroup.TagID,
			Order:       talkgroup.Order,
		}); uerr != nil {
			slog.Warn("failed to backfill talkgroup name from upload",
				"talkgroup_id", talkgroup.TalkgroupID, "error", uerr)
		} else {
			slog.Info("backfilled talkgroup name from upload",
				"talkgroup_id", talkgroup.TalkgroupID, "name", talkgroupTag)
			h.hub.BroadcastCFG(ctx)
		}
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
		Site:            siteCol,
		Channel:         channelCol,
		Decoder:         decoderCol,
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
		if siteCol.Valid {
			calPayload["site"] = siteCol.String
		}
		if channelCol.Valid {
			calPayload["channel"] = channelCol.String
		}
		if decoderCol.Valid {
			calPayload["decoder"] = decoderCol.String
		}
		calMsg, err := ws.NewCALMessage(calPayload)
		if err != nil {
			slog.Error("failed to build CAL message", "error", err)
		} else {
			const maxBroadcastAudioBytes = 20 << 20 // 20 MiB
			var audioBytes []byte
			audioFullPath := filepath.Join(h.processor.RecordingsDir(), relPath)
			if rel, pathErr := filepath.Rel(h.processor.RecordingsDir(), audioFullPath); pathErr != nil || strings.HasPrefix(rel, "..") {
				slog.Error("audio path escapes base directory", "path", relPath)
			} else if fi, statErr := os.Stat(audioFullPath); statErr != nil {
				slog.Warn("failed to stat audio for WS broadcast", "path", rel, "error", statErr)
			} else if fi.Size() > maxBroadcastAudioBytes {
				slog.Warn("audio file too large for inline WS broadcast, sending metadata only",
					"path", rel, "size_bytes", fi.Size(), "max_bytes", maxBroadcastAudioBytes)
			} else if readBytes, readErr := os.ReadFile(audioFullPath); readErr != nil {
				slog.Warn("failed to read audio for WS broadcast", "path", rel, "error", readErr)
			} else {
				audioBytes = readBytes
			}
			h.hub.BroadcastCAL(calMsg, audioBytes, func(cl *ws.Client) bool {
				return cl.CanReceive(system.ID, talkgroup.ID)
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"id": callID})

	// Notify downstream pushers (non-blocking, after response is sent).
	if h.dsNotifier != nil {
		// Resolve labels for downstream consumers.
		var groupLabel, tagLabel string
		if talkgroup.GroupID.Valid {
			if g, err := h.queries.GetGroup(ctx, talkgroup.GroupID.Int64); err == nil {
				groupLabel = g.Label
			}
		}
		if talkgroup.TagID.Valid {
			if t, err := h.queries.GetTag(ctx, talkgroup.TagID.Int64); err == nil {
				tagLabel = t.Label
			}
		}

		h.dsNotifier.Notify(downstream.CallEvent{
			CallID:         callID,
			AudioPath:      relPath,
			AudioName:      filepath.Base(relPath),
			AudioType:      audioType,
			DateTime:       dateTimeUnix,
			SystemID:       system.SystemID,
			System:         system.ID,
			TalkgroupID:    talkgroup.TalkgroupID,
			Talkgroup:      talkgroup.ID,
			Frequency:      frequency.Int64,
			Duration:       duration.Int64,
			Source:         source.Int64,
			Sources:        sourcesJSON.String,
			Frequencies:    frequenciesJSON.String,
			Patches:        patchesJSON.String,
			SystemLabel:    system.Label,
			TalkgroupLabel: talkgroup.Label.String,
			TalkgroupName:  talkgroup.Name.String,
			TalkgroupGroup: groupLabel,
			TalkgroupTag:   tagLabel,
		})
	}
}

// CallSearchResult is a single call in the search response.
type CallSearchResult struct {
	ID             int64  `json:"id"`
	AudioName      string `json:"audioName"`
	AudioType      string `json:"audioType"`
	DateTime       int64  `json:"dateTime"`
	SystemID       int64  `json:"systemId"`
	SystemLabel    string `json:"systemLabel"`
	TalkgroupID    int64  `json:"talkgroupId"`
	TalkgroupLabel string `json:"talkgroupLabel"`
	TalkgroupName  string `json:"talkgroupName"`
	TalkgroupGroup string `json:"talkgroupGroup,omitempty"`
	TalkgroupTag   string `json:"talkgroupTag,omitempty"`
	TalkgroupLed   string `json:"talkgroupLed,omitempty"`
	Frequency      *int64 `json:"frequency,omitempty"`
	Duration       *int64 `json:"duration,omitempty"`
	Source         *int64 `json:"source,omitempty"`
	Site           string `json:"site,omitempty"`
	Channel        string `json:"channel,omitempty"`
	Decoder        string `json:"decoder,omitempty"`
	Transcript     string `json:"transcript,omitempty"`
	Bookmarked     bool   `json:"bookmarked"`
}

// GetCallAudio handles GET /api/calls/:id/audio.
func (h *CallHandler) GetCallAudio(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid call id"})
		return
	}

	// Require authentication or publicAccess for direct audio access.
	// Anonymous users must use /api/shared/:token/audio for shared calls.
	_, hasUser := c.Get("userID")
	if !hasUser && h.getSettingValue(c, "publicAccess") != "true" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	call, err := h.queries.GetCall(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "call not found"})
			return
		}
		slog.Error("failed to get call audio metadata", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	recordingsDir := h.processor.RecordingsDir()
	relPath := filepath.Clean(call.AudioPath)
	if strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		slog.Warn("rejected unsafe audio path", "id", id, "path", call.AudioPath)
		c.JSON(http.StatusNotFound, gin.H{"error": "audio not found"})
		return
	}

	fullPath := filepath.Join(recordingsDir, relPath)
	if rel, relErr := filepath.Rel(recordingsDir, fullPath); relErr != nil || strings.HasPrefix(rel, "..") {
		slog.Warn("audio path escaped recordings dir", "id", id, "path", call.AudioPath)
		c.JSON(http.StatusNotFound, gin.H{"error": "audio not found"})
		return
	}

	if _, err := os.Stat(fullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "audio file not found"})
			return
		}
		slog.Error("failed to stat call audio file", "id", id, "path", fullPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	contentType := call.AudioType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := call.AudioName
	if filename == "" {
		filename = "call"
	}

	c.Header("Content-Disposition", "inline; filename="+strconv.Quote(filename))
	c.Header("Content-Type", contentType)
	c.File(fullPath)
}

// CallSearchResponse is the response for GET /api/calls.
type CallSearchResponse struct {
	Calls []CallSearchResult `json:"calls"`
	Total int64              `json:"total"`
}

// GetCalls handles GET /api/calls — paginated call archive search.
func (h *CallHandler) GetCalls(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters.
	var systemID, talkgroupID interface{}
	if v := c.Query("system_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid system_id"})
			return
		}
		systemID = n
	}
	if v := c.Query("talkgroup_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid talkgroup_id"})
			return
		}
		talkgroupID = n
	}

	var dateFrom, dateTo interface{}
	if v := c.Query("date_from"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_from"})
			return
		}
		dateFrom = n
	}
	if v := c.Query("date_to"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_to"})
			return
		}
		dateTo = n
	}

	page := int64(1)
	if v := c.Query("page"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page"})
			return
		}
		page = n
	}

	limit := int64(25)
	if v := c.Query("limit"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		if n > 100 {
			n = 100
		}
		limit = n
	}

	sortOrder := "desc"
	if v := c.Query("sort"); v != "" {
		v = strings.ToLower(v)
		if v != "asc" && v != "desc" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sort must be asc or desc"})
			return
		}
		sortOrder = v
	}

	offset := (page - 1) * limit

	// Count total matching calls.
	total, err := h.queries.CountCallsFiltered(ctx, db.CountCallsFilteredParams{
		SystemID:    systemID,
		TalkgroupID: talkgroupID,
		DateFrom:    dateFrom,
		DateTo:      dateTo,
	})
	if err != nil {
		slog.Error("failed to count calls", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Fetch calls page.
	var calls []db.Call
	listParams := db.ListCallsParams{
		SystemID:    systemID,
		TalkgroupID: talkgroupID,
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		PageOffset:  sql.NullInt64{Int64: offset, Valid: true},
		PageSize:    sql.NullInt64{Int64: limit, Valid: true},
	}
	if sortOrder == "asc" {
		calls, err = h.queries.ListCallsAsc(ctx, db.ListCallsAscParams(listParams))
	} else {
		calls, err = h.queries.ListCalls(ctx, listParams)
	}
	if err != nil {
		slog.Error("failed to list calls", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Build set of bookmarked call IDs for authenticated users.
	bookmarkedIDs := make(map[int64]bool)
	if userIDVal, exists := c.Get("userID"); exists {
		if uid, ok := userIDVal.(int64); ok {
			bookmarks, berr := h.queries.ListBookmarksByUser(ctx, sql.NullInt64{Int64: uid, Valid: true})
			if berr == nil {
				for _, bm := range bookmarks {
					bookmarkedIDs[bm.CallID] = true
				}
			}
		}
	}

	// Pre-cache lookups to avoid N+1 queries.
	systemCache := make(map[int64]db.System)
	tgCache := make(map[int64]db.Talkgroup)
	groupCache := make(map[int64]string)
	tagCache := make(map[int64]string)

	// Build response with joined labels and transcripts.
	results := make([]CallSearchResult, 0, len(calls))
	for _, call := range calls {
		r := CallSearchResult{
			ID:        call.ID,
			AudioName: call.AudioName,
			AudioType: call.AudioType,
			DateTime:  call.DateTime,
			SystemID:  call.SystemID,
		}

		if call.Frequency.Valid {
			r.Frequency = &call.Frequency.Int64
		}
		if call.Duration.Valid {
			r.Duration = &call.Duration.Int64
		}
		if call.Site.Valid {
			r.Site = call.Site.String
		}
		if call.Channel.Valid {
			r.Channel = call.Channel.String
		}
		if call.Decoder.Valid {
			r.Decoder = call.Decoder.String
		}
		if call.Source.Valid {
			r.Source = &call.Source.Int64
		}
		if call.Site.Valid {
			r.Site = call.Site.String
		}
		if call.Channel.Valid {
			r.Channel = call.Channel.String
		}
		if call.Decoder.Valid {
			r.Decoder = call.Decoder.String
		}

		// Join system label (cached).
		sys, ok := systemCache[call.SystemID]
		if !ok {
			var serr error
			sys, serr = h.queries.GetSystem(ctx, call.SystemID)
			if serr == nil {
				systemCache[call.SystemID] = sys
			}
		}
		if ok || systemCache[call.SystemID].ID != 0 {
			r.SystemLabel = sys.Label
		}

		// Join talkgroup details (cached).
		if call.TalkgroupID.Valid {
			r.TalkgroupID = call.TalkgroupID.Int64
			tg, ok := tgCache[call.TalkgroupID.Int64]
			if !ok {
				var terr error
				tg, terr = h.queries.GetTalkgroup(ctx, call.TalkgroupID.Int64)
				if terr == nil {
					tgCache[call.TalkgroupID.Int64] = tg
				}
			}
			if ok || tgCache[call.TalkgroupID.Int64].ID != 0 {
				if tg.Label.Valid {
					r.TalkgroupLabel = tg.Label.String
				}
				if tg.Name.Valid {
					r.TalkgroupName = tg.Name.String
				}
				if tg.Led.Valid {
					r.TalkgroupLed = tg.Led.String
				}
				// Resolve group label (cached).
				if tg.GroupID.Valid {
					grpLabel, ok := groupCache[tg.GroupID.Int64]
					if !ok {
						grp, gerr := h.queries.GetGroup(ctx, tg.GroupID.Int64)
						if gerr == nil {
							groupCache[tg.GroupID.Int64] = grp.Label
							grpLabel = grp.Label
						}
					}
					if ok || grpLabel != "" {
						r.TalkgroupGroup = grpLabel
					}
				}
				// Resolve tag label (cached).
				if tg.TagID.Valid {
					tagLabel, ok := tagCache[tg.TagID.Int64]
					if !ok {
						tag, tgerr := h.queries.GetTag(ctx, tg.TagID.Int64)
						if tgerr == nil {
							tagCache[tg.TagID.Int64] = tag.Label
							tagLabel = tag.Label
						}
					}
					if ok || tagLabel != "" {
						r.TalkgroupTag = tagLabel
					}
				}
			}
		}

		// Join transcript.
		trn, terr := h.queries.GetTranscriptionByCallID(ctx, call.ID)
		if terr == nil {
			r.Transcript = trn.Text
		}

		// Bookmark status.
		r.Bookmarked = bookmarkedIDs[call.ID]

		results = append(results, r)
	}

	c.JSON(http.StatusOK, CallSearchResponse{
		Calls: results,
		Total: total,
	})
}

// isBlacklistedTG checks whether a talkgroup ID appears in a system's blacklist.
// The blacklist is a JSON array of integers stored in blacklists_json.
func isBlacklistedTG(blacklistsJSON sql.NullString, talkgroupID int64) bool {
	if !blacklistsJSON.Valid || strings.TrimSpace(blacklistsJSON.String) == "" {
		return false
	}
	var ids []int64
	if err := json.Unmarshal([]byte(blacklistsJSON.String), &ids); err != nil {
		slog.Warn("failed to parse blacklists_json", "error", err)
		return false
	}
	for _, id := range ids {
		if id == talkgroupID {
			return true
		}
	}
	return false
}
