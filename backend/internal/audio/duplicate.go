// Package audio — duplicate call detection logic.
package audio

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/openscanner/openscanner/internal/db"
)

// IsDuplicate returns true if a call for the same system+talkgroup exists
// within windowMs milliseconds of dateTime.
// A windowMs of 0 or less disables detection (always returns false).
// systemID and talkgroupID are the DB row IDs (FKs used in the calls table).
func IsDuplicate(ctx context.Context, queries *db.Queries, systemID, talkgroupID int64, dateTime time.Time, windowMs int64) (bool, error) {
	slog.Debug("audio: duplicate check", "system", systemID, "talkgroup", talkgroupID, "window_ms", windowMs)
	if windowMs <= 0 {
		return false, nil
	}

	// First, enforce idempotency for re-imports: same system + talkgroup +
	// timestamp is always a duplicate regardless of the rolling window size.
	hasAtTimestamp, err := queries.HasCallAtTimestamp(ctx, db.HasCallAtTimestampParams{
		SystemID:    systemID,
		TalkgroupID: sql.NullInt64{Int64: talkgroupID, Valid: true},
		DateTime:    dateTime.Unix(),
	})
	if err != nil {
		return false, err
	}
	if hasAtTimestamp != 0 {
		return true, nil
	}

	// Check for any call in the time-range window around the new call's timestamp.
	// This correctly handles out-of-order ingest (e.g. replaying old recordings).
	windowSec := windowMs / 1000
	if windowSec < 1 {
		windowSec = 1
	}
	tsSec := dateTime.Unix()

	found, err := queries.HasCallInTimeRange(ctx, db.HasCallInTimeRangeParams{
		SystemID:    systemID,
		TalkgroupID: sql.NullInt64{Int64: talkgroupID, Valid: true},
		DateTime:    tsSec - windowSec,
		DateTime_2:  tsSec + windowSec,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	isDup := found != 0
	slog.Debug("audio: duplicate check result", "system", systemID, "talkgroup", talkgroupID, "is_duplicate", isDup)
	return isDup, nil
}
