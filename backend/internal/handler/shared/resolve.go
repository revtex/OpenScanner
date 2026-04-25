package shared

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/openscanner/openscanner/internal/db"
)

// ResolveGroupID looks up an existing group by label or creates one if it
// doesn't exist. Returns a valid sql.NullInt64 with the group's DB ID, or
// an invalid NullInt64 if the operation fails.
func ResolveGroupID(ctx context.Context, q db.Querier, label string) sql.NullInt64 {
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

// ResolveTagID looks up an existing tag by label or creates one if it
// doesn't exist. Returns a valid sql.NullInt64 with the tag's DB ID, or
// an invalid NullInt64 if the operation fails.
func ResolveTagID(ctx context.Context, q db.Querier, label string) sql.NullInt64 {
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
