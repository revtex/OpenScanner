// Package audio — duplicate call detection logic.
package audio

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/openscanner/openscanner/internal/db"
)

// IsDuplicate returns true if a call for the same system+talkgroup exists
// within windowMs milliseconds of dateTime.
// A windowMs of 0 or less disables detection (always returns false).
// systemID and talkgroupID are the DB row IDs (FKs used in the calls table).
func IsDuplicate(ctx context.Context, queries *db.Queries, systemID, talkgroupID int64, dateTime time.Time, windowMs int64) (bool, error) {
	if windowMs <= 0 {
		return false, nil
	}

	lastCall, err := queries.GetLastCallForTalkgroup(ctx, db.GetLastCallForTalkgroupParams{
		SystemID:    systemID,
		TalkgroupID: sql.NullInt64{Int64: talkgroupID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	diffMs := math.Abs(float64(dateTime.Unix()-lastCall.DateTime)) * 1000
	return diffMs < float64(windowMs), nil
}
