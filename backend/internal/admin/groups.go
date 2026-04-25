package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/openscanner/openscanner/internal/db"
)

// GroupsList returns all groups.
func (o *Operations) GroupsList(ctx context.Context, _ json.RawMessage, _ int64) (any, error) {
	groups, err := o.Queries.ListGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}
	return groups, nil
}

// GroupsCreate creates a new group.
func (o *Operations) GroupsCreate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.Label == "" {
		return nil, UserError("label is required")
	}

	id, err := o.Queries.CreateGroup(ctx, req.Label)
	if isUniqueViolation(err) {
		return nil, UserError("group label already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	group, err := o.Queries.GetGroup(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created group: %w", err)
	}
	slog.Info("admin: group created", "id", group.ID, "label", group.Label, "by", callerID)
	o.broadcastAdminEvent("groups.updated", nil)
	o.broadcastCFG(ctx)
	return group, nil
}

// GroupsUpdate updates an existing group.
func (o *Operations) GroupsUpdate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
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

	if _, err := o.Queries.GetGroup(ctx, req.ID); err != nil {
		return nil, UserError("group not found")
	}

	err := o.Queries.UpdateGroup(ctx, db.UpdateGroupParams{ID: req.ID, Label: req.Label})
	if isUniqueViolation(err) {
		return nil, UserError("group label already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update group: %w", err)
	}

	group, err := o.Queries.GetGroup(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated group: %w", err)
	}
	slog.Info("admin: group updated", "id", group.ID, "label", group.Label, "by", callerID)
	o.broadcastAdminEvent("groups.updated", nil)
	o.broadcastCFG(ctx)
	return group, nil
}

// GroupsDelete deletes a group.
func (o *Operations) GroupsDelete(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetGroup(ctx, req.ID); err != nil {
		return nil, UserError("group not found")
	}

	if err := o.Queries.DeleteGroup(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete group: %w", err)
	}
	slog.Info("admin: group deleted", "id", req.ID, "by", callerID)
	o.broadcastAdminEvent("groups.updated", nil)
	o.broadcastCFG(ctx)
	return map[string]bool{"ok": true}, nil
}
