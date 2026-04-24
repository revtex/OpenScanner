package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/openscanner/openscanner/internal/db"
)

// TagsList returns all tags.
func (o *Operations) TagsList(ctx context.Context, _ json.RawMessage, _ int64) (any, error) {
	tags, err := o.Queries.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	return tags, nil
}

// TagsCreate creates a new tag.
func (o *Operations) TagsCreate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.Label == "" {
		return nil, UserError("label is required")
	}

	id, err := o.Queries.CreateTag(ctx, req.Label)
	if isUniqueViolation(err) {
		return nil, UserError("tag label already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	tag, err := o.Queries.GetTag(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created tag: %w", err)
	}
	slog.Info("admin: tag created", "id", tag.ID, "label", tag.Label, "by", callerID)
	o.broadcastAdminEvent("tags.updated", nil)
	o.broadcastCFG(ctx)
	return tag, nil
}

// TagsUpdate updates an existing tag.
func (o *Operations) TagsUpdate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID    int64  `json:"id"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}
	if req.Label == "" {
		return nil, UserError("label is required")
	}

	if _, err := o.Queries.GetTag(ctx, req.ID); err != nil {
		return nil, UserError("tag not found")
	}

	err := o.Queries.UpdateTag(ctx, db.UpdateTagParams{ID: req.ID, Label: req.Label})
	if isUniqueViolation(err) {
		return nil, UserError("tag label already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update tag: %w", err)
	}

	tag, err := o.Queries.GetTag(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated tag: %w", err)
	}
	slog.Info("admin: tag updated", "id", tag.ID, "label", tag.Label, "by", callerID)
	o.broadcastAdminEvent("tags.updated", nil)
	o.broadcastCFG(ctx)
	return tag, nil
}

// TagsDelete deletes a tag.
func (o *Operations) TagsDelete(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetTag(ctx, req.ID); err != nil {
		return nil, UserError("tag not found")
	}

	if err := o.Queries.DeleteTag(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete tag: %w", err)
	}
	slog.Info("admin: tag deleted", "id", req.ID, "by", callerID)
	o.broadcastAdminEvent("tags.updated", nil)
	o.broadcastCFG(ctx)
	return map[string]bool{"ok": true}, nil
}
