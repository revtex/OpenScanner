// Package api — call upload (POST /api/call-upload, /api/trunk-recorder-call-upload).
package api

import (
	"context"
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
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/downstream"
	"github.com/openscanner/openscanner/internal/ws"
)

const (
	defaultCallRatePerMin = 60
	maxCallRatePerMin     = 600
	rateWindowDuration    = time.Minute
	shareRatePerMin       = 10
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
	queries       *db.Queries
	processor     *audio.Processor
	hub           *ws.Hub
	dsNotifier    DownstreamNotifier
	transcriber   audio.Transcriber // nil when transcription is disabled
	mu            sync.Mutex
	limiters      map[int64]*apiKeyLimiter
	shareMu       sync.Mutex
	shareLimiters map[int64]*apiKeyLimiter
}

// NewCallHandler creates a CallHandler.
func NewCallHandler(queries *db.Queries, processor *audio.Processor, hub *ws.Hub, dsNotifier DownstreamNotifier, transcriber audio.Transcriber) *CallHandler {
	return &CallHandler{
		queries:       queries,
		processor:     processor,
		hub:           hub,
		dsNotifier:    dsNotifier,
		transcriber:   transcriber,
		limiters:      make(map[int64]*apiKeyLimiter),
		shareLimiters: make(map[int64]*apiKeyLimiter),
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
	} else {
		l.mu.Lock()
		l.rateLimit = rateLimit
		l.mu.Unlock()
	}
	return l
}

// getShareLimiter returns a per-user rate limiter for share creation.
func (h *CallHandler) getShareLimiter(userID int64) *apiKeyLimiter {
	h.shareMu.Lock()
	defer h.shareMu.Unlock()

	if len(h.shareLimiters) > 100 {
		now := time.Now()
		for id, l := range h.shareLimiters {
			l.mu.Lock()
			stale := now.Sub(l.windowStart) >= 2*rateWindowDuration
			l.mu.Unlock()
			if stale {
				delete(h.shareLimiters, id)
			}
		}
	}

	l, ok := h.shareLimiters[userID]
	if !ok {
		l = &apiKeyLimiter{
			windowStart: time.Now(),
			rateLimit:   shareRatePerMin,
		}
		h.shareLimiters[userID] = l
	}
	return l
}

// systemGrant mirrors ws.systemGrant for grant-based filtering in REST handlers.
type systemGrant struct {
	ID         int64   `json:"id"`
	Talkgroups []int64 `json:"talkgroups,omitempty"`
}

// loadUserGrants returns the parsed grants for the authenticated user. Returns
// nil (allow-all) for admins, unauthenticated users, or users with no grants.
func (h *CallHandler) loadUserGrants(c *gin.Context) []systemGrant {
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	if roleStr == auth.RoleAdmin {
		return nil
	}
	userIDVal, exists := c.Get("userID")
	if !exists {
		return nil
	}
	uid, _ := userIDVal.(int64)
	user, err := h.queries.GetUser(c.Request.Context(), uid)
	if err != nil {
		return nil
	}
	if !user.SystemsJson.Valid || user.SystemsJson.String == "" {
		return nil
	}
	var grants []systemGrant
	if err := json.Unmarshal([]byte(user.SystemsJson.String), &grants); err != nil {
		slog.Warn("failed to parse user grants", "user_id", uid, "error", err)
		return nil
	}
	if len(grants) == 0 {
		return nil
	}
	return grants
}

// isGranted checks whether a call with the given system/talkgroup passes the
// grant filter. A nil grant list means everything is allowed.
func isGranted(grants []systemGrant, systemID, talkgroupID int64) bool {
	if grants == nil {
		return true
	}
	for _, g := range grants {
		if g.ID != systemID {
			continue
		}
		if len(g.Talkgroups) == 0 {
			return true
		}
		for _, tg := range g.Talkgroups {
			if tg == talkgroupID {
				return true
			}
		}
	}
	return false
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
//
//	@Summary		Upload a call recording
//	@Description	Ingest a radio call with audio and metadata. Requires a valid API key.
//	@Tags			Upload
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		APIKeyAuth
//	@Param			audio			formData	file	true	"Audio file"
//	@Param			dateTime		formData	int		true	"Unix timestamp of the call"
//	@Param			systemId		formData	int		true	"Radio system ID"
//	@Param			talkgroupId		formData	int		true	"Talkgroup ID"
//	@Param			source			formData	int		false	"Source unit ID"
//	@Param			frequency		formData	int		false	"Frequency in Hz"
//	@Param			duration		formData	number	false	"Call duration in seconds"
//	@Param			talkgroupLabel	formData	string	false	"Talkgroup label for auto-populate"
//	@Param			talkgroupTag	formData	string	false	"Talkgroup tag name"
//	@Param			talkgroupGroup	formData	string	false	"Talkgroup group name"
//	@Param			talkgroupName	formData	string	false	"Talkgroup display name"
//	@Param			systemLabel		formData	string	false	"System label"
//	@Param			patches			formData	string	false	"JSON array of patched talkgroup IDs"
//	@Param			audioName		formData	string	false	"Original audio file name"
//	@Param			audioType		formData	string	false	"Audio MIME type"
//	@Param			site			formData	string	false	"Site identifier"
//	@Param			channel			formData	string	false	"Channel identifier"
//	@Param			decoder			formData	string	false	"Decoder software name"
//	@Param			errorCount		formData	int		false	"Decoding error count"
//	@Param			spikeCount		formData	int		false	"Signal spike count"
//	@Success		200	{object}	object{id=int64}			"Call ingested successfully"
//	@Failure		400	{object}	ErrorResponse			"Bad request"
//	@Failure		401	{object}	ErrorResponse			"API key required"
//	@Failure		429	{object}	ErrorResponse			"Rate limit exceeded"
//	@Failure		500	{object}	ErrorResponse			"Internal server error"
//	@Router			/call-upload [post]
//	@Router			/trunk-recorder-call-upload [post]
func (h *CallHandler) PostCallUpload(c *gin.Context) {
	slog.Debug("call-upload: request received", "ip", c.ClientIP())
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
	apiKeyRateOverride := false
	if apiKeyRateVal, ok := c.Get("apiKeyCallRate"); ok {
		if apiKeyRate, ok := apiKeyRateVal.(int64); ok && apiKeyRate > 0 {
			rateLimit = int(apiKeyRate)
			apiKeyRateOverride = true
		}
	}
	if rStr := h.getSettingValue(c, "apiKeyCallRate"); rStr != "" {
		if r, err := strconv.Atoi(rStr); err == nil && r > 0 && !apiKeyRateOverride {
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

	slog.Debug("call-upload: rate limit passed", "api_key_id", apiKeyID)

	// SDRTrunk and other rdio-scanner-compatible clients may send a POST with
	// partial data to verify the API key. rdio-scanner responds with plain-text
	// "Incomplete call data: <reason>" (status 417) which SDRTrunk treats as a
	// successful connection test. We replicate that behavior: parse all fields
	// first, then return the same message format for missing required fields.
	dateTimeStr := c.PostForm("dateTime")
	systemIDStr := c.PostForm("systemId")
	if systemIDStr == "" {
		systemIDStr = c.PostForm("system")
	}
	talkgroupIDStr := c.PostForm("talkgroupId")
	if talkgroupIDStr == "" {
		talkgroupIDStr = c.PostForm("talkgroup")
	}
	_, audioErr := c.FormFile("audio")

	// Check for test=1 explicitly (Trunk Recorder).
	if c.PostForm("test") == "1" {
		c.String(http.StatusOK, "Incomplete call data: no talkgroup\n")
		return
	}

	// rdio-scanner's IsValid() checks all fields WITHOUT early returns and
	// overwrites the error each time, so the LAST failing check wins.
	// SDRTrunk sends system=<id> but no audio/dateTime/talkgroup, so the last
	// error is always "no talkgroup" — which SDRTrunk explicitly checks for.
	// We replicate this behavior: collect the last error, then return it.
	var incompleteReason string
	if audioErr != nil {
		incompleteReason = "no audio"
	}
	if dateTimeStr == "" {
		incompleteReason = "no datetime"
	}
	if systemIDStr == "" {
		incompleteReason = "no system"
	}
	if talkgroupIDStr == "" {
		incompleteReason = "no talkgroup"
	}
	if incompleteReason != "" {
		slog.Warn("call-upload: incomplete data",
			"reason", incompleteReason,
			"api_key_id", apiKeyID,
		)
		c.String(http.StatusExpectationFailed, "Incomplete call data: %s\n", incompleteReason)
		return
	}

	// Parse dateTime.
	// Try unix timestamp first (Trunk Recorder, SDRTrunk), then ISO 8601 (voxcall).
	var dateTimeUnix int64
	if n, err := strconv.ParseInt(dateTimeStr, 10, 64); err == nil {
		dateTimeUnix = n
	} else if t, err := time.Parse(time.RFC3339Nano, dateTimeStr); err == nil {
		dateTimeUnix = t.Unix()
	} else if t, err := time.Parse(time.RFC3339, dateTimeStr); err == nil {
		dateTimeUnix = t.Unix()
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dateTime: expected unix timestamp or ISO 8601"})
		return
	}
	callTime := time.Unix(dateTimeUnix, 0)

	// Trunk Recorder's rdioscanner_uploader plugin sends "system" and
	// "talkgroup" while our canonical field names are "systemId" and
	// "talkgroupId". Accept both for backward compatibility.
	// (Already parsed above for the connectivity check.)
	systemIDRaw, err := strconv.ParseInt(systemIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid systemId"})
		return
	}

	talkgroupIDRaw, err := strconv.ParseInt(talkgroupIDStr, 10, 64)
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

	var errorCount, spikeCount sql.NullInt64
	if v := c.PostForm("errorCount"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			errorCount = sql.NullInt64{Int64: n, Valid: true}
		}
	}
	if v := c.PostForm("spikeCount"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			spikeCount = sql.NullInt64{Int64: n, Valid: true}
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

	// Trunk-recorder's rdio-scanner uploader embeds unit IDs inside the
	// "sources" JSON array rather than sending a top-level "source" field.
	// Extract the first source unit ID when not explicitly provided.
	if !source.Valid && sourcesJSON.Valid {
		source = extractPrimarySource(sourcesJSON.String)
	}

	// Similarly, error and spike counts are per-segment inside the
	// "frequencies" JSON array. Aggregate them when no top-level values
	// were provided.
	if !errorCount.Valid && !spikeCount.Valid && frequenciesJSON.Valid {
		errorCount, spikeCount = aggregateErrorSpikeCounts(frequenciesJSON.String)
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
	talkgroupGroup := c.PostForm("talkgroupGroup")
	talkgroupName := c.PostForm("talkgroupName")

	var talkerAliasCol sql.NullString
	if v := c.PostForm("talkerAlias"); v != "" {
		talkerAliasCol = sql.NullString{String: v, Valid: true}
	}

	// Trunk-recorder embeds OTA aliases in the sources JSON "tag" field
	// rather than sending a top-level "talkerAlias". Extract from the
	// first source entry when not explicitly provided.
	if !talkerAliasCol.Valid && sourcesJSON.Valid {
		talkerAliasCol = extractPrimarySourceTag(sourcesJSON.String)
	}

	ctx := c.Request.Context()
	autoPopulateSystems := h.getSettingValue(c, "autoPopulateSystems") == "true"

	slog.Debug("call-upload: resolving system and talkgroup",
		"system_id", systemIDRaw, "talkgroup_id", talkgroupIDRaw)

	// Resolve system by its radio system_id.
	system, err := h.queries.GetSystemBySystemID(ctx, systemIDRaw)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to query system", "system_id", systemIDRaw, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if !autoPopulateSystems {
			c.JSON(http.StatusBadRequest, gin.H{"error": "system not found"})
			return
		}
		label := strconv.FormatInt(systemIDRaw, 10)
		// SDRTrunk and other uploaders send systemLabel with a human-readable name.
		if sl := c.PostForm("systemLabel"); sl != "" {
			label = sl
		}
		newID, cerr := h.queries.CreateSystem(ctx, db.CreateSystemParams{
			SystemID:               systemIDRaw,
			Label:                  label,
			AutoPopulateTalkgroups: 1,
		})
		if cerr != nil {
			slog.Error("failed to auto-create system", "system_id", systemIDRaw, "error", cerr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		slog.Info("auto-populated system", "system_id", systemIDRaw, "label", label, "db_id", newID)
		system = db.System{ID: newID, SystemID: systemIDRaw, Label: label, AutoPopulateTalkgroups: 1}
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
		if system.AutoPopulateTalkgroups == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "talkgroup not found"})
			return
		}
		var tgLabel, tgName sql.NullString
		if talkgroupLabel != "" {
			tgLabel = sql.NullString{String: talkgroupLabel, Valid: true}
		}
		if talkgroupName != "" {
			tgName = sql.NullString{String: talkgroupName, Valid: true}
		}
		// Resolve group from talkgroupGroup (e.g. SDRTrunk sends this).
		var groupID sql.NullInt64
		if talkgroupGroup != "" {
			groupID = resolveGroupID(ctx, h.queries, talkgroupGroup)
		}
		// Resolve tag from talkgroupTag (e.g. "Law Dispatch", "Fire-Tac").
		var tagID sql.NullInt64
		if talkgroupTag != "" {
			tagID = resolveTagID(ctx, h.queries, talkgroupTag)
		}
		newID, cerr := h.queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
			SystemID:    system.ID,
			TalkgroupID: talkgroupIDRaw,
			Label:       tgLabel,
			Name:        tgName,
			GroupID:     groupID,
			TagID:       tagID,
		})
		if cerr != nil {
			slog.Error("failed to auto-create talkgroup", "talkgroup_id", talkgroupIDRaw, "error", cerr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		slog.Info("auto-populated talkgroup", "system_id", system.SystemID, "talkgroup_id", talkgroupIDRaw, "label", tgLabel.String, "db_id", newID)
		talkgroup = db.Talkgroup{ID: newID, SystemID: system.ID, TalkgroupID: talkgroupIDRaw, Label: tgLabel, Name: tgName, GroupID: groupID, TagID: tagID}
		h.hub.BroadcastCFG(ctx)
	} else if needsBackfill(talkgroup, talkgroupLabel, talkgroupName, talkgroupTag, talkgroupGroup) {
		// Existing talkgroup has empty fields — backfill from upload metadata.
		if !talkgroup.Label.Valid && talkgroupLabel != "" {
			talkgroup.Label = sql.NullString{String: talkgroupLabel, Valid: true}
		}
		if !talkgroup.Name.Valid && talkgroupName != "" {
			talkgroup.Name = sql.NullString{String: talkgroupName, Valid: true}
		}
		if !talkgroup.GroupID.Valid && talkgroupGroup != "" {
			talkgroup.GroupID = resolveGroupID(ctx, h.queries, talkgroupGroup)
		}
		if !talkgroup.TagID.Valid && talkgroupTag != "" {
			talkgroup.TagID = resolveTagID(ctx, h.queries, talkgroupTag)
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
			slog.Warn("failed to backfill talkgroup from upload",
				"talkgroup_id", talkgroup.TalkgroupID, "error", uerr)
		} else {
			slog.Info("backfilled talkgroup from upload",
				"talkgroup_id", talkgroup.TalkgroupID)
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
			c.JSON(http.StatusOK, gin.H{"message": "duplicate call rejected"})
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
	convMode := audio.ConversionDisabled
	if mStr := h.getSettingValue(c, "audioConversion"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil {
			convMode = audio.ConversionMode(m)
		}
	}

	// Resolve encoding preset from settings.
	convPreset := audio.ParseEncodingPreset(h.getSettingValue(c, "audioEncodingPreset"))

	// Store audio file (conversion handled inside Processor.Store).
	relPath, err := h.processor.Store(ctx, fh, convMode, convPreset)
	if err != nil {
		slog.Error("failed to store audio file",
			"system_id", systemIDRaw,
			"talkgroup_id", talkgroupIDRaw,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store audio"})
		return
	}

	slog.Debug("call-upload: audio stored", "path", relPath, "mode", convMode)

	// If the recorder didn't supply a duration, probe the stored file.
	if !duration.Valid {
		absPath := filepath.Join(h.processor.RecordingsDir(), relPath)
		if d := audio.ProbeDuration(ctx, absPath); d > 0 {
			duration = sql.NullInt64{Int64: d, Valid: true}
		}
	}

	// Determine audio MIME type.
	// When conversion is enabled the output format depends on the encoding
	// preset (M4A for AAC presets, MP3 for MP3 presets).
	// Otherwise validate the client-supplied Content-Type against an allowlist
	// to prevent attacker-controlled MIME types from reaching the database.
	var audioType string
	if convMode != audio.ConversionDisabled {
		audioType = audio.OutputMIME(convPreset)
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
		ErrorCount:      errorCount,
		SpikeCount:      spikeCount,
		TalkerAlias:     talkerAliasCol,
	})
	if err != nil {
		slog.Error("failed to insert call", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	slog.Info("call ingested", "id", callID)

	slog.Debug("call-upload: db record inserted",
		"call_id", callID,
		"system", systemIDRaw,
		"talkgroup", talkgroupIDRaw,
		"audio_path", relPath,
	)

	// Extract unit tags from sources JSON and upsert into units table.
	// Sources format: [{"pos":0,"src":12345,"tag":"Unit Name"}, ...]
	if sourcesJSON.Valid {
		upsertUnitsFromSources(ctx, h.queries, system.ID, sourcesJSON.String)
	}

	// Map talkerAlias to the source unit as a label (e.g. P25 radios broadcasting a name).
	if source.Valid && talkerAliasCol.Valid {
		if err := h.queries.UpsertUnit(ctx, db.UpsertUnitParams{
			SystemID: system.ID,
			UnitID:   source.Int64,
			Label:    sql.NullString{String: talkerAliasCol.String, Valid: true},
		}); err != nil {
			slog.Warn("failed to upsert unit from talkerAlias",
				"unit_id", source.Int64, "talkerAlias", talkerAliasCol.String, "error", err)
		}
	}

	// Broadcast to WebSocket listeners.
	if h.hub != nil {
		// Read audio file for inline embedding in the CAL JSON frame.
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
		if errorCount.Valid {
			calPayload["errorCount"] = errorCount.Int64
		}
		if spikeCount.Valid {
			calPayload["spikeCount"] = spikeCount.Int64
		}
		if talkerAliasCol.Valid {
			calPayload["talkerAlias"] = talkerAliasCol.String
		}
		if sourcesJSON.Valid {
			calPayload["sources"] = sourcesJSON.String
		}
		if frequenciesJSON.Valid {
			calPayload["frequencies"] = frequenciesJSON.String
		}
		calMsg, err := ws.NewCALMessage(calPayload, audioBytes)
		if err != nil {
			slog.Error("failed to build CAL message", "error", err)
		} else {
			h.hub.BroadcastCAL(calMsg, func(cl *ws.Client) bool {
				return cl.CanReceive(system.ID, talkgroup.ID)
			})
			slog.Debug("call-upload: ws broadcast sent", "call_id", callID)
		}
	}

	slog.Info("call-upload: complete",
		"call_id", callID,
		"system_id", systemIDRaw,
		"talkgroup_id", talkgroupIDRaw,
		"duration_ms", duration.Int64,
		"duration_valid", duration.Valid,
		"audio_path", relPath,
		"api_key_id", apiKeyID,
	)

	c.JSON(http.StatusOK, gin.H{"id": callID, "message": "Call imported successfully."})

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
			TalkerAlias:    talkerAliasCol.String,
		})
		slog.Debug("call-upload: downstream notify queued", "call_id", callID)
	}

	// Enqueue transcription (non-blocking, after response is sent).
	if h.transcriber != nil {
		absPath := filepath.Join(h.processor.RecordingsDir(), relPath)
		if err := h.transcriber.Submit(ctx, audio.TranscriptionJob{
			CallID:    callID,
			AudioPath: absPath,
		}); err != nil {
			slog.Warn("call-upload: failed to enqueue transcription", "call_id", callID, "error", err)
		}
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
	ErrorCount     *int64 `json:"errorCount,omitempty"`
	SpikeCount     *int64 `json:"spikeCount,omitempty"`
	TalkerAlias    string `json:"talkerAlias,omitempty"`
	Transcript     string `json:"transcript,omitempty"`
	Bookmarked     bool   `json:"bookmarked"`
} // @name CallSearchResult

// GetCallAudio handles GET /api/calls/:id/audio.
//
//	@Summary		Get call audio file
//	@Description	Stream the audio file for a specific call. Authentication is optional when the publicAccess setting is enabled; otherwise a valid JWT is required.
//	@Tags			Calls
//	@Security		BearerAuth
//	@Produce		application/octet-stream
//	@Param			id	path	int	true	"Call ID"
//	@Success		200	{file}	binary			"Audio file"
//	@Failure		400	{object}	ErrorResponse	"Invalid call ID"
//	@Failure		401	{object}	ErrorResponse	"Authentication required"
//	@Failure		404	{object}	ErrorResponse	"Call or audio not found"
//	@Failure		500	{object}	ErrorResponse	"Internal server error"
//	@Router			/calls/{id}/audio [get]
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

	// Enforce per-user grants for non-admin listeners.
	if grants := h.loadUserGrants(c); !isGranted(grants, call.SystemID, call.TalkgroupID.Int64) {
		c.JSON(http.StatusNotFound, gin.H{"error": "call not found"})
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

	c.Header("Content-Disposition", contentDisposition("inline", filename))
	c.Header("Content-Type", contentType)
	c.File(fullPath)
}

// CallSearchResponse is the response for GET /api/calls.
type CallSearchResponse struct {
	Calls []CallSearchResult `json:"calls"`
	Total int64              `json:"total"`
} // @name CallSearchResponse

// GetCalls handles GET /api/calls — paginated call archive search.
//
//	@Summary		Search calls
//	@Description	Paginated search of the call archive with optional filters. Authentication is optional when the publicAccess setting is enabled; otherwise a valid JWT is required.
//	@Tags			Calls
//	@Security		BearerAuth
//	@Produce		json
//	@Param			system_ids		query	string	false	"CSV system DB IDs (e.g. 1,2,3)"
//	@Param			talkgroup_ids	query	string	false	"CSV talkgroup DB IDs (e.g. 10,11)"
//	@Param			groups			query	string	false	"CSV group labels (e.g. Police,Fire)"
//	@Param			tags				query	string	false	"CSV tag labels (e.g. Law,EMS)"
//	@Param			system_id		query	int		false	"Legacy single system DB ID"
//	@Param			talkgroup_id	query	int		false	"Legacy single talkgroup DB ID"
//	@Param			group			query	string	false	"Legacy single group label"
//	@Param			tag				query	string	false	"Legacy single tag label"
//	@Param			date_from		query	int		false	"Unix timestamp lower bound"
//	@Param			date_to			query	int		false	"Unix timestamp upper bound"
//	@Param			sort			query	string	false	"Sort order: asc or desc"	Enums(asc, desc)	default(desc)
//	@Param			page			query	int		false	"Page number (1-based)"		default(1)
//	@Param			limit			query	int		false	"Results per page (max 100)"	default(25)
//	@Param			bookmarked_only	query	bool	false	"Show only bookmarked calls"
//	@Param			transcript		query	string	false	"Filter by transcript text (partial match)"
//	@Success		200	{object}	CallSearchResponse	"Paginated call results"
//	@Failure		400	{object}	ErrorResponse		"Invalid query parameter"
//	@Failure		500	{object}	ErrorResponse		"Internal server error"
//	@Router			/calls [get]
func (h *CallHandler) GetCalls(c *gin.Context) {
	ctx := c.Request.Context()

	parseCSVInt64 := func(raw string) ([]int64, error) {
		if strings.TrimSpace(raw) == "" {
			return nil, nil
		}
		parts := strings.Split(raw, ",")
		vals := make([]int64, 0, len(parts))
		seen := make(map[int64]struct{})
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			n, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, err
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			vals = append(vals, n)
		}
		if len(vals) == 0 {
			return nil, nil
		}
		return vals, nil
	}

	parseCSVStrings := func(raw string) []string {
		if strings.TrimSpace(raw) == "" {
			return nil
		}
		parts := strings.Split(raw, ",")
		vals := make([]string, 0, len(parts))
		seen := make(map[string]struct{})
		for _, part := range parts {
			v := strings.TrimSpace(part)
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			vals = append(vals, v)
		}
		if len(vals) == 0 {
			return nil
		}
		return vals
	}

	toCSVFilter := func(vals []int64) interface{} {
		if len(vals) == 0 {
			return nil
		}
		parts := make([]string, 0, len(vals))
		for _, v := range vals {
			parts = append(parts, strconv.FormatInt(v, 10))
		}
		return strings.Join(parts, ",")
	}

	// Parse multi-select IDs (new CSV params) with single-select fallback.
	rawSystemIDs := c.Query("system_ids")
	if rawSystemIDs == "" {
		rawSystemIDs = c.Query("system_id")
	}
	rawTalkgroupIDs := c.Query("talkgroup_ids")
	if rawTalkgroupIDs == "" {
		rawTalkgroupIDs = c.Query("talkgroup_id")
	}

	systemIDs, err := parseCSVInt64(rawSystemIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid system_ids"})
		return
	}
	talkgroupIDs, err := parseCSVInt64(rawTalkgroupIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid talkgroup_ids"})
		return
	}

	// Parse multi-select labels (new CSV params) with single-select fallback.
	rawGroups := c.Query("groups")
	if rawGroups == "" {
		rawGroups = c.Query("group")
	}
	rawTags := c.Query("tags")
	if rawTags == "" {
		rawTags = c.Query("tag")
	}
	groupLabels := parseCSVStrings(rawGroups)
	tagLabels := parseCSVStrings(rawTags)

	groupIDs := make([]int64, 0, len(groupLabels))
	for _, label := range groupLabels {
		g, err := h.queries.GetGroupByLabel(ctx, label)
		if err == nil {
			groupIDs = append(groupIDs, g.ID)
		}
	}
	if len(groupLabels) > 0 && len(groupIDs) == 0 {
		c.JSON(http.StatusOK, CallSearchResponse{Calls: []CallSearchResult{}, Total: 0})
		return
	}

	tagIDs := make([]int64, 0, len(tagLabels))
	for _, label := range tagLabels {
		t, err := h.queries.GetTagByLabel(ctx, label)
		if err == nil {
			tagIDs = append(tagIDs, t.ID)
		}
	}
	if len(tagLabels) > 0 && len(tagIDs) == 0 {
		c.JSON(http.StatusOK, CallSearchResponse{Calls: []CallSearchResult{}, Total: 0})
		return
	}

	systemIDsCSV := toCSVFilter(systemIDs)
	talkgroupIDsCSV := toCSVFilter(talkgroupIDs)
	groupIDsCSV := toCSVFilter(groupIDs)
	tagIDsCSV := toCSVFilter(tagIDs)

	var transcript interface{}
	if v := c.Query("transcript"); v != "" {
		transcript = v
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

	// Resolve bookmarked_only filter: requires authenticated user.
	var bookmarkUserID interface{}
	if c.Query("bookmarked_only") == "true" {
		if userIDVal, exists := c.Get("userID"); exists {
			if uid, ok := userIDVal.(int64); ok {
				bookmarkUserID = uid
			}
		}
	}

	// Count total matching calls.
	total, err := h.queries.CountCallsFiltered(ctx, db.CountCallsFilteredParams{
		SystemIdsCsv:    systemIDsCSV,
		TalkgroupIdsCsv: talkgroupIDsCSV,
		GroupIdsCsv:     groupIDsCSV,
		TagIdsCsv:       tagIDsCSV,
		DateFrom:        dateFrom,
		DateTo:          dateTo,
		BookmarkUserID:  bookmarkUserID,
		Transcript:      transcript,
	})
	if err != nil {
		slog.Error("failed to count calls", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Fetch calls page.
	var calls []db.Call
	listParams := db.ListCallsParams{
		SystemIdsCsv:    systemIDsCSV,
		TalkgroupIdsCsv: talkgroupIDsCSV,
		GroupIdsCsv:     groupIDsCSV,
		TagIdsCsv:       tagIDsCSV,
		DateFrom:        dateFrom,
		DateTo:          dateTo,
		BookmarkUserID:  bookmarkUserID,
		Transcript:      transcript,
		PageOffset:      sql.NullInt64{Int64: offset, Valid: true},
		PageSize:        sql.NullInt64{Int64: limit, Valid: true},
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

	// Enforce per-user grants — filter out calls the listener is not
	// authorised to see. Admins and unauthenticated public-access users
	// have nil grants (allow-all).
	grants := h.loadUserGrants(c)
	if grants != nil {
		allowed := calls[:0]
		for _, call := range calls {
			if isGranted(grants, call.SystemID, call.TalkgroupID.Int64) {
				allowed = append(allowed, call)
			}
		}
		calls = allowed
		// Adjust total to reflect grant-scoped count. The SQL count does not
		// know about grants, so cap it to the filtered result set size when
		// the filter actually removed rows. This is an approximation; an
		// exact count would require SQL-level grant filtering.
		if int64(len(calls)) < limit {
			total = offset + int64(len(calls))
		}
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
		if call.Source.Valid {
			r.Source = &call.Source.Int64
		}
		if call.ErrorCount.Valid {
			r.ErrorCount = &call.ErrorCount.Int64
		}
		if call.SpikeCount.Valid {
			r.SpikeCount = &call.SpikeCount.Int64
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
		if call.TalkerAlias.Valid {
			r.TalkerAlias = call.TalkerAlias.String
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
			r.SystemID = sys.SystemID
			r.SystemLabel = sys.Label
		}

		// Join talkgroup details (cached).
		if call.TalkgroupID.Valid {
			tg, ok := tgCache[call.TalkgroupID.Int64]
			if !ok {
				var terr error
				tg, terr = h.queries.GetTalkgroup(ctx, call.TalkgroupID.Int64)
				if terr == nil {
					tgCache[call.TalkgroupID.Int64] = tg
				}
			}
			if ok || tgCache[call.TalkgroupID.Int64].ID != 0 {
				r.TalkgroupID = tg.TalkgroupID
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

// resolveGroupID looks up an existing group by label or creates one if it
// doesn't exist. Returns a valid sql.NullInt64 with the group's DB ID, or
// an invalid NullInt64 if the operation fails.
func resolveGroupID(ctx context.Context, q db.Querier, label string) sql.NullInt64 {
	g, err := q.GetGroupByLabel(ctx, label)
	if err == nil {
		return sql.NullInt64{Int64: g.ID, Valid: true}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("failed to look up group by label", "label", label, "error", err)
		return sql.NullInt64{}
	}
	newID, cerr := q.CreateGroup(ctx, label)
	if cerr != nil {
		slog.Warn("failed to auto-create group", "label", label, "error", cerr)
		return sql.NullInt64{}
	}
	slog.Info("auto-populated group from upload", "label", label, "db_id", newID)
	return sql.NullInt64{Int64: newID, Valid: true}
}

// resolveTagID looks up an existing tag by label or creates one if it
// doesn't exist. Returns a valid sql.NullInt64 with the tag's DB ID, or
// an invalid NullInt64 if the operation fails.
func resolveTagID(ctx context.Context, q db.Querier, label string) sql.NullInt64 {
	t, err := q.GetTagByLabel(ctx, label)
	if err == nil {
		return sql.NullInt64{Int64: t.ID, Valid: true}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("failed to look up tag by label", "label", label, "error", err)
		return sql.NullInt64{}
	}
	newID, cerr := q.CreateTag(ctx, label)
	if cerr != nil {
		slog.Warn("failed to auto-create tag", "label", label, "error", cerr)
		return sql.NullInt64{}
	}
	slog.Info("auto-populated tag from upload", "label", label, "db_id", newID)
	return sql.NullInt64{Int64: newID, Valid: true}
}

// needsBackfill returns true if at least one talkgroup field is empty and a
// corresponding value was provided in the upload metadata.
func needsBackfill(tg db.Talkgroup, label, name, tag, group string) bool {
	if !tg.Label.Valid && label != "" {
		return true
	}
	if !tg.Name.Valid && name != "" {
		return true
	}
	if !tg.TagID.Valid && tag != "" {
		return true
	}
	if !tg.GroupID.Valid && group != "" {
		return true
	}
	return false
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

// upsertUnitsFromSources parses the sources JSON array and upserts any units
// that include a "tag" (label) into the units table.
// Sources format: [{"pos":0,"src":12345,"tag":"Unit Name"}, ...]
// Entries without "src" or "tag" are silently skipped.
func upsertUnitsFromSources(ctx context.Context, q *db.Queries, systemDBID int64, raw string) {
	var sources []map[string]any
	if err := json.Unmarshal([]byte(raw), &sources); err != nil {
		return
	}
	for _, entry := range sources {
		srcVal, ok := entry["src"]
		if !ok {
			continue
		}
		srcFloat, ok := srcVal.(float64)
		if !ok || srcFloat <= 0 {
			continue
		}
		tagVal, ok := entry["tag"]
		if !ok {
			continue
		}
		tag, ok := tagVal.(string)
		if !ok || tag == "" {
			continue
		}
		if err := q.UpsertUnit(ctx, db.UpsertUnitParams{
			SystemID: systemDBID,
			UnitID:   int64(srcFloat),
			Label:    sql.NullString{String: tag, Valid: true},
		}); err != nil {
			slog.Warn("failed to upsert unit from sources",
				"unit_id", int64(srcFloat), "tag", tag, "error", err)
		}
	}
}

// extractPrimarySource returns the "src" value from the first entry in a
// sources JSON array. Trunk-recorder sends unit IDs only inside this array
// (e.g. [{"pos":0,"src":12345}, ...]) and does not set a top-level "source".
func extractPrimarySource(raw string) sql.NullInt64 {
	var sources []map[string]any
	if err := json.Unmarshal([]byte(raw), &sources); err != nil || len(sources) == 0 {
		return sql.NullInt64{}
	}
	srcVal, ok := sources[0]["src"]
	if !ok {
		return sql.NullInt64{}
	}
	srcFloat, ok := srcVal.(float64)
	if !ok || srcFloat <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(srcFloat), Valid: true}
}

// extractPrimarySourceTag returns the "tag" value from the first source entry
// that has a non-empty tag. Trunk-recorder sends OTA aliases (talker alias)
// inside the sources JSON rather than as a top-level "talkerAlias" field.
func extractPrimarySourceTag(raw string) sql.NullString {
	var sources []map[string]any
	if err := json.Unmarshal([]byte(raw), &sources); err != nil {
		return sql.NullString{}
	}
	for _, entry := range sources {
		tagVal, ok := entry["tag"]
		if !ok {
			continue
		}
		tag, ok := tagVal.(string)
		if !ok || tag == "" {
			continue
		}
		return sql.NullString{String: tag, Valid: true}
	}
	return sql.NullString{}
}

// aggregateErrorSpikeCounts sums errorCount and spikeCount from all entries
// in a frequencies JSON array. Trunk-recorder sends per-segment values inside
// this array (e.g. [{"errorCount":2,"spikeCount":0}, ...]) rather than
// providing aggregate top-level fields.
func aggregateErrorSpikeCounts(raw string) (sql.NullInt64, sql.NullInt64) {
	var freqs []map[string]any
	if err := json.Unmarshal([]byte(raw), &freqs); err != nil || len(freqs) == 0 {
		return sql.NullInt64{}, sql.NullInt64{}
	}
	var totalErrors, totalSpikes int64
	var found bool
	for _, entry := range freqs {
		if v, ok := entry["errorCount"]; ok {
			if f, ok := v.(float64); ok {
				totalErrors += int64(f)
				found = true
			}
		}
		// trunk-recorder also uses "error_count" in its call JSON.
		if v, ok := entry["error_count"]; ok {
			if f, ok := v.(float64); ok {
				totalErrors += int64(f)
				found = true
			}
		}
		if v, ok := entry["spikeCount"]; ok {
			if f, ok := v.(float64); ok {
				totalSpikes += int64(f)
				found = true
			}
		}
		if v, ok := entry["spike_count"]; ok {
			if f, ok := v.(float64); ok {
				totalSpikes += int64(f)
				found = true
			}
		}
	}
	if !found {
		return sql.NullInt64{}, sql.NullInt64{}
	}
	return sql.NullInt64{Int64: totalErrors, Valid: true},
		sql.NullInt64{Int64: totalSpikes, Valid: true}
}

// transcriptResponse is the JSON shape returned by GetCallTranscript.
type transcriptResponse struct {
	Text     string                       `json:"text"`
	Segments []audio.TranscriptionSegment `json:"segments"`
	Language string                       `json:"language"`
	Model    string                       `json:"model"`
} // @name TranscriptResponse

// GetCallTranscript handles GET /api/calls/:id/transcript.
// Returns the transcription for a call if one exists.
//
//	@Summary		Get call transcript
//	@Description	Returns the transcription text, segments, language and model for a call. Authentication is optional when the publicAccess setting is enabled; otherwise a valid JWT is required.
//	@Tags			Calls
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Call ID"
//	@Success		200	{object}	transcriptResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/calls/{id}/transcript [get]
func (h *CallHandler) GetCallTranscript(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid call id"})
		return
	}

	// Require authentication or publicAccess.
	_, hasUser := c.Get("userID")
	if !hasUser && h.getSettingValue(c, "publicAccess") != "true" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	trx, err := h.queries.GetTranscriptionByCallID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "transcript not found"})
			return
		}
		slog.Error("failed to get transcript", "callID", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var segments []audio.TranscriptionSegment
	if trx.Segments.Valid && trx.Segments.String != "" {
		if err := json.Unmarshal([]byte(trx.Segments.String), &segments); err != nil {
			slog.Warn("failed to parse transcript segments", "callID", id, "error", err)
		}
	}
	if segments == nil {
		segments = []audio.TranscriptionSegment{}
	}

	c.JSON(http.StatusOK, transcriptResponse{
		Text:     trx.Text,
		Segments: segments,
		Language: trx.Language.String,
		Model:    trx.Model.String,
	})
}
