// Package audio — background pruning of old calls and their audio files.
package audio

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/openscanner/openscanner/internal/db"
)

// PruneLoop runs pruneOldCalls on a 1-hour tick until ctx is cancelled.
func PruneLoop(ctx context.Context, queries *db.Queries, recordingsDir string) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruneOldCalls(ctx, queries, recordingsDir)
		}
	}
}

// pruneOldCalls deletes audio files and call records older than pruneDays.
// It processes rows in batches of up to 500 (limited by GetCallIDsOlderThan).
func pruneOldCalls(ctx context.Context, queries *db.Queries, recordingsDir string) {
	setting, err := queries.GetSetting(ctx, "pruneDays")
	if err != nil {
		slog.Error("failed to get pruneDays setting", "error", err)
		return
	}
	pruneDays, err := strconv.Atoi(setting.Value)
	if err != nil || pruneDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -pruneDays).Unix()

	for {
		rows, err := queries.GetCallIDsOlderThan(ctx, cutoff)
		if err != nil {
			slog.Error("failed to query old calls for pruning", "error", err)
			return
		}
		if len(rows) == 0 {
			return
		}

		cleanBase := filepath.Clean(recordingsDir)
		for _, row := range rows {
			audioPath := filepath.Join(recordingsDir, row.AudioPath)
			// Defense-in-depth: ensure resolved path stays within recordingsDir.
			if !strings.HasPrefix(filepath.Clean(audioPath), cleanBase+string(filepath.Separator)) {
				slog.Warn("pruner: audio path escapes recordingsDir, skipping file removal", "audio_path", row.AudioPath)
			} else if err := os.Remove(audioPath); err != nil && !os.IsNotExist(err) {
				slog.Warn("failed to remove audio file during prune", "error", err)
			}
			if err := queries.DeleteCallBatch(ctx, row.ID); err != nil {
				slog.Error("failed to delete call during prune", "id", row.ID, "error", err)
			}
			runtime.Gosched()
		}

		// GetCallIDsOlderThan is limited to 500 rows; stop when we get fewer.
		if len(rows) < 500 {
			return
		}
	}
}
