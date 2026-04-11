// Package dirwatch uses fsnotify to watch directories for new audio files.
package dirwatch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/downstream"
	"github.com/openscanner/openscanner/internal/ws"
)

// DownstreamNotifier is the interface used to notify the downstream pusher.
type DownstreamNotifier interface {
	Notify(event downstream.CallEvent)
}

// Service manages all active DirWatch watchers.
type Service struct {
	queries    *db.Queries
	processor  *audio.Processor
	hub        *ws.Hub
	dsNotifier DownstreamNotifier
	mu         sync.Mutex
	reloadMu   sync.Mutex // serialises Reload calls to prevent duplicate goroutine spawning
	appCtx     context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewService creates a DirWatch service.
func NewService(queries *db.Queries, processor *audio.Processor, hub *ws.Hub, dsNotifier DownstreamNotifier) *Service {
	return &Service{
		queries:    queries,
		processor:  processor,
		hub:        hub,
		dsNotifier: dsNotifier,
	}
}

// Start loads all active dirwatch configs from the DB and starts a watcher
// goroutine for each one. The watchers run until ctx is cancelled or Reload
// is called.
func (s *Service) Start(ctx context.Context) {
	childCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	s.appCtx = ctx // remember the long-lived context for Reload
	s.cancel = cancel
	s.mu.Unlock()

	dirwatches, err := s.queries.ListActiveDirwatches(childCtx)
	if err != nil {
		slog.Error("dirwatch: failed to load configs from DB", "error", err)
		cancel()
		return
	}

	for _, dw := range dirwatches {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runDirwatch(childCtx, dw)
		}()
	}
}

// Reload stops all running watchers and restarts them fresh from the DB.
// This should be called after admin CRUD changes to dirwatch configs.
// Reload is serialised via reloadMu to prevent concurrent calls from spawning
// duplicate watcher goroutines. It reuses the application-lifetime context
// from the initial Start call rather than accepting a request-scoped context.
func (s *Service) Reload() {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	s.mu.Lock()
	appCtx := s.appCtx
	s.mu.Unlock()

	if appCtx == nil {
		return
	}
	s.stop()
	s.Start(appCtx)
}

// stop cancels all running watcher goroutines and waits for them to exit.
func (s *Service) stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// runDirwatch dispatches to the appropriate watch strategy for a single entry.
func (s *Service) runDirwatch(ctx context.Context, dw db.Dirwatch) {
	slog.Info("dirwatch: starting watcher",
		"id", dw.ID,
		"dir", dw.Directory,
		"type", dw.Type,
		"polling", dw.UsePolling == 1,
	)

	if dw.UsePolling == 1 {
		s.runWithPolling(ctx, dw)
	} else {
		s.runWithFsnotify(ctx, dw)
	}

	slog.Info("dirwatch: watcher stopped", "id", dw.ID)
}

// runWithFsnotify watches using kernel inotify/kqueue Create events.
func (s *Service) runWithFsnotify(ctx context.Context, dw db.Dirwatch) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("dirwatch: failed to create fsnotify watcher", "id", dw.ID, "error", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(dw.Directory); err != nil {
		slog.Error("dirwatch: failed to add directory to watcher",
			"id", dw.ID, "dir", dw.Directory, "error", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				s.handleFile(ctx, dw, event.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("dirwatch: fsnotify error", "id", dw.ID, "error", err)
		}
	}
}

// minPollIntervalMs is the floor for the polling delay to prevent tight CPU loops.
const minPollIntervalMs = 500

// runWithPolling scans the directory on a regular ticker.
func (s *Service) runWithPolling(ctx context.Context, dw db.Dirwatch) {
	delayMs := int64(2000)
	if dw.Delay.Valid && dw.Delay.Int64 > 0 {
		delayMs = dw.Delay.Int64
	}
	// SECURITY: enforce a minimum polling interval to prevent tight CPU loops.
	if delayMs < minPollIntervalMs {
		slog.Warn("dirwatch: poll delay below minimum, clamping",
			"id", dw.ID, "requested_ms", delayMs, "min_ms", minPollIntervalMs)
		delayMs = minPollIntervalMs
	}

	ticker := time.NewTicker(time.Duration(delayMs) * time.Millisecond)
	defer ticker.Stop()

	seen := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			entries, err := os.ReadDir(dw.Directory)
			if err != nil {
				slog.Warn("dirwatch: failed to read directory",
					"id", dw.ID, "dir", dw.Directory, "error", err)
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				path := filepath.Join(dw.Directory, entry.Name())
				if seen[path] {
					continue
				}
				seen[path] = true
				s.handleFile(ctx, dw, path)
			}
			// Prune stale entries from the seen map to prevent unbounded memory
			// growth in long-running deployments (e.g. delete_after=1 leaves
			// entries for already-deleted files).
			current := make(map[string]bool, len(entries))
			for _, entry := range entries {
				if !entry.IsDir() {
					current[filepath.Join(dw.Directory, entry.Name())] = true
				}
			}
			for path := range seen {
				if !current[path] {
					delete(seen, path)
				}
			}
		}
	}
}

// handleFile applies security checks, the extension filter, and calls the
// appropriate parser before handing off to ingestCall.
func (s *Service) handleFile(ctx context.Context, dw db.Dirwatch, filePath string) {
	// Security: resolve symlinks then reject paths that escape the watched directory.
	watchedReal, err := filepath.EvalSymlinks(filepath.Clean(dw.Directory))
	if err != nil {
		slog.Warn("dirwatch: failed to resolve watched directory",
			"dir", dw.Directory, "error", err)
		return
	}
	fileReal, err := filepath.EvalSymlinks(filepath.Clean(filePath))
	if err != nil {
		slog.Warn("dirwatch: failed to resolve file path, rejected",
			"file", filePath, "error", err)
		return
	}
	rel, err := filepath.Rel(watchedReal, fileReal)
	if err != nil || strings.HasPrefix(rel, "..") {
		slog.Warn("dirwatch: file outside watched directory, rejected",
			"file", filePath, "dir", watchedReal)
		return
	}

	// Extension filter: only process files that match dw.Extension when set.
	if dw.Extension.Valid && dw.Extension.String != "" {
		want := strings.ToLower(dw.Extension.String)
		if !strings.HasPrefix(want, ".") {
			want = "." + want
		}
		got := strings.ToLower(filepath.Ext(filePath))
		if got != want {
			return
		}
	}

	parse := parserForType(dw.Type)
	parsed, err := parse(dw, fileReal)
	if err != nil {
		slog.Error("dirwatch: parse error", "id", dw.ID, "file", filePath, "error", err)
		return
	}
	if parsed == nil {
		// Parser signalled skip (e.g. audio not yet arrived for trunk-recorder sidecar).
		return
	}

	if err := s.ingestCall(ctx, dw, parsed); err != nil {
		slog.Error("dirwatch: ingest failed", "id", dw.ID, "file", filePath, "error", err)
	}
}

// ingestCall runs the full call ingest pipeline for a parsed file, mirroring
// the logic in api/calls.go but without an HTTP context.
func (s *Service) ingestCall(ctx context.Context, dw db.Dirwatch, parsed *ParsedCall) error {
	// Helper to retrieve a setting value from the DB (returns "" on missing/error).
	getSetting := func(key string) string {
		v, err := s.queries.GetSetting(ctx, key)
		if err != nil {
			return ""
		}
		return v.Value
	}

	autoPopulate := getSetting("autoPopulate") == "true"

	// Validate required IDs before touching the DB.
	if parsed.SystemID == 0 {
		return fmt.Errorf("no system ID in parsed call for file %s", parsed.AudioFilePath)
	}
	if parsed.TalkgroupID == 0 {
		return fmt.Errorf("no talkgroup ID in parsed call for file %s", parsed.AudioFilePath)
	}

	// ── Resolve system ──────────────────────────────────────────────────────
	system, err := s.queries.GetSystemBySystemID(ctx, parsed.SystemID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("query system %d: %w", parsed.SystemID, err)
		}
		if !autoPopulate {
			return fmt.Errorf("system %d not found and autoPopulate is disabled", parsed.SystemID)
		}
		label := strconv.FormatInt(parsed.SystemID, 10)
		newID, cerr := s.queries.CreateSystem(ctx, db.CreateSystemParams{
			SystemID:     parsed.SystemID,
			Label:        label,
			AutoPopulate: 1,
		})
		if cerr != nil {
			return fmt.Errorf("auto-create system %d: %w", parsed.SystemID, cerr)
		}
		slog.Info("dirwatch: auto-populated system", "system_id", parsed.SystemID, "db_id", newID)
		system = db.System{ID: newID, SystemID: parsed.SystemID, Label: label, AutoPopulate: 1}
	}

	// ── Resolve talkgroup ───────────────────────────────────────────────────
	talkgroup, err := s.queries.GetTalkgroupBySystemAndTGID(ctx, db.GetTalkgroupBySystemAndTGIDParams{
		SystemID:    system.ID,
		TalkgroupID: parsed.TalkgroupID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("query talkgroup %d: %w", parsed.TalkgroupID, err)
		}
		if !autoPopulate {
			return fmt.Errorf("talkgroup %d not found and autoPopulate is disabled", parsed.TalkgroupID)
		}
		newID, cerr := s.queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
			SystemID:    system.ID,
			TalkgroupID: parsed.TalkgroupID,
		})
		if cerr != nil {
			return fmt.Errorf("auto-create talkgroup %d: %w", parsed.TalkgroupID, cerr)
		}
		slog.Info("dirwatch: auto-populated talkgroup",
			"talkgroup_id", parsed.TalkgroupID, "db_id", newID)
		talkgroup = db.Talkgroup{ID: newID, SystemID: system.ID, TalkgroupID: parsed.TalkgroupID}
	}

	// ── Duplicate detection ─────────────────────────────────────────────────
	if getSetting("disableDuplicateDetection") != "true" {
		windowMs := int64(2000)
		if v := getSetting("duplicateDetectionTimeFrame"); v != "" {
			if wm, err := strconv.ParseInt(v, 10, 64); err == nil {
				windowMs = wm
			}
		}
		dup, derr := audio.IsDuplicate(ctx, s.queries, system.ID, talkgroup.ID, parsed.DateTime, windowMs)
		if derr != nil {
			slog.Error("dirwatch: duplicate detection error", "error", derr)
			// Non-fatal: proceed.
		} else if dup {
			slog.Info("dirwatch: duplicate call rejected",
				"system_id", parsed.SystemID, "talkgroup_id", parsed.TalkgroupID)
			return nil
		}
	}

	// ── Resolve conversion mode ─────────────────────────────────────────────
	convMode := audio.ConversionEnabled
	if mStr := getSetting("audioConversion"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil {
			convMode = audio.ConversionMode(m)
		}
	}

	// ── Store audio ─────────────────────────────────────────────────────────
	relPath, err := s.processor.StoreFile(ctx, parsed.AudioFilePath, convMode)
	if err != nil {
		return fmt.Errorf("store audio: %w", err)
	}

	// Determine MIME type.  When conversion is enabled the output is always AAC.
	var audioType string
	if convMode != audio.ConversionDisabled {
		audioType = "audio/aac"
	} else {
		audioType = mimeFromExt(filepath.Ext(parsed.AudioFilePath))
	}

	// ── Build nullable fields ───────────────────────────────────────────────
	var freq sql.NullInt64
	if parsed.Frequency > 0 {
		freq = sql.NullInt64{Int64: parsed.Frequency, Valid: true}
	}
	var dur sql.NullInt64
	if parsed.Duration > 0 {
		dur = sql.NullInt64{Int64: parsed.Duration, Valid: true}
	}
	var src sql.NullInt64
	if parsed.Source > 0 {
		src = sql.NullInt64{Int64: parsed.Source, Valid: true}
	}
	var srcJSON, freqJSON, patchJSON sql.NullString
	if parsed.SourcesJSON != "" {
		srcJSON = sql.NullString{String: parsed.SourcesJSON, Valid: true}
	}
	if parsed.FreqsJSON != "" {
		freqJSON = sql.NullString{String: parsed.FreqsJSON, Valid: true}
	}
	if parsed.PatchesJSON != "" {
		patchJSON = sql.NullString{String: parsed.PatchesJSON, Valid: true}
	}

	dateTimeUnix := parsed.DateTime.Unix()

	// ── Insert call record ──────────────────────────────────────────────────
	callID, err := s.queries.CreateCall(ctx, db.CreateCallParams{
		AudioPath:       relPath,
		AudioName:       filepath.Base(relPath),
		AudioType:       audioType,
		DateTime:        dateTimeUnix,
		Frequency:       freq,
		Duration:        dur,
		Source:          src,
		SourcesJson:     srcJSON,
		FrequenciesJson: freqJSON,
		PatchesJson:     patchJSON,
		SystemID:        system.ID,
		TalkgroupID:     sql.NullInt64{Int64: talkgroup.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert call record: %w", err)
	}

	slog.Info("dirwatch: call ingested",
		"id", callID,
		"system_id", system.SystemID,
		"talkgroup_id", talkgroup.TalkgroupID,
	)

	// ── Broadcast to WebSocket listeners ────────────────────────────────────
	if s.hub != nil {
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
		if freq.Valid {
			calPayload["frequency"] = freq.Int64
		}
		if dur.Valid {
			calPayload["duration"] = dur.Int64
		}
		if src.Valid {
			calPayload["source"] = src.Int64
		}

		calMsg, err := ws.NewCALMessage(calPayload)
		if err != nil {
			slog.Error("dirwatch: failed to build CAL message", "error", err)
		} else {
			// SECURITY: limit audio read size for WS broadcast to prevent OOM
			// for large audio files. Files exceeding the limit are still stored
			// on disk; the client will fetch them via HTTP.
			const maxBroadcastAudioBytes = 20 << 20 // 20 MiB
			var audioBytes []byte
			audioAbsPath := filepath.Join(s.processor.BaseDir(), relPath)
			if fi, statErr := os.Stat(audioAbsPath); statErr != nil {
				slog.Warn("dirwatch: failed to stat audio for WS broadcast",
					"path", relPath, "error", statErr)
			} else if fi.Size() > maxBroadcastAudioBytes {
				slog.Warn("dirwatch: audio file too large for inline WS broadcast, sending metadata only",
					"path", relPath, "size_bytes", fi.Size(), "max_bytes", maxBroadcastAudioBytes)
			} else if readBytes, readErr := os.ReadFile(audioAbsPath); readErr != nil {
				slog.Warn("dirwatch: failed to read audio for WS broadcast",
					"path", relPath, "error", readErr)
			} else {
				audioBytes = readBytes
			}
			s.hub.BroadcastCAL(calMsg, audioBytes, func(cl *ws.Client) bool {
				return cl.CanReceive(system.ID, talkgroup.ID)
			})
		}
	}

	// ── Notify downstream pushers ──────────────────────────────────────────
	if s.dsNotifier != nil {
		s.dsNotifier.Notify(downstream.CallEvent{
			CallID:      callID,
			AudioPath:   relPath,
			AudioName:   filepath.Base(relPath),
			AudioType:   audioType,
			DateTime:    dateTimeUnix,
			SystemID:    system.SystemID,
			System:      system.ID,
			TalkgroupID: talkgroup.TalkgroupID,
			Talkgroup:   talkgroup.ID,
			Frequency:   freq.Int64,
			Duration:    dur.Int64,
			Source:      src.Int64,
			Sources:     srcJSON.String,
			Frequencies: freqJSON.String,
			Patches:     patchJSON.String,
		})
	}

	// ── Delete source file if configured ────────────────────────────────────
	if dw.DeleteAfter == 1 {
		if err := os.Remove(parsed.AudioFilePath); err != nil && !os.IsNotExist(err) {
			slog.Warn("dirwatch: failed to delete source file",
				"file", parsed.AudioFilePath, "error", err)
		}
	}

	return nil
}

// mimeFromExt maps a file extension (including leading dot) to an audio MIME type.
func mimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".ogg":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".opus":
		return "audio/opus"
	default:
		return "application/octet-stream"
	}
}
