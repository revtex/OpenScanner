package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/openscanner/openscanner/internal/db"
)

// TalkgroupsList returns all talkgroups.
func (o *Operations) TalkgroupsList(ctx context.Context, _ json.RawMessage, _ int64) (any, error) {
	tgs, err := o.Queries.ListAllTalkgroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list talkgroups: %w", err)
	}
	return mapTalkgroups(tgs), nil
}

// TalkgroupsCreate creates a new talkgroup.
func (o *Operations) TalkgroupsCreate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		SystemID    int64   `json:"systemId"`
		TalkgroupID int64   `json:"talkgroupId"`
		Label       *string `json:"label"`
		Name        *string `json:"name"`
		Frequency   *int64  `json:"frequency"`
		Led         *string `json:"led"`
		GroupID     *int64  `json:"groupId"`
		TagID       *int64  `json:"tagId"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}

	id, err := o.Queries.CreateTalkgroup(ctx, db.CreateTalkgroupParams{
		SystemID:    req.SystemID,
		TalkgroupID: req.TalkgroupID,
		Label:       ptrToNullStr(req.Label),
		Name:        ptrToNullStr(req.Name),
		Frequency:   ptrToNullInt(req.Frequency),
		Led:         ptrToNullStr(req.Led),
		GroupID:     ptrToNullInt(req.GroupID),
		TagID:       ptrToNullInt(req.TagID),
		Order:       req.Order,
	})
	if isUniqueViolation(err) {
		return nil, UserError("talkgroup already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create talkgroup: %w", err)
	}

	tg, err := o.Queries.GetTalkgroup(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created talkgroup: %w", err)
	}
	slog.Info("admin: talkgroup created", "id", tg.ID, "talkgroup_id", tg.TalkgroupID, "by", callerID)
	o.broadcastAdminEvent("talkgroups.updated", nil)
	o.broadcastCFG(ctx)
	return mapTalkgroup(tg), nil
}

// TalkgroupsUpdate updates an existing talkgroup.
func (o *Operations) TalkgroupsUpdate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID          int64   `json:"id"`
		TalkgroupID int64   `json:"talkgroupId"`
		Label       *string `json:"label"`
		Name        *string `json:"name"`
		Frequency   *int64  `json:"frequency"`
		Led         *string `json:"led"`
		GroupID     *int64  `json:"groupId"`
		TagID       *int64  `json:"tagId"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetTalkgroup(ctx, req.ID); err != nil {
		return nil, UserError("talkgroup not found")
	}

	err := o.Queries.UpdateTalkgroup(ctx, db.UpdateTalkgroupParams{
		ID:          req.ID,
		TalkgroupID: req.TalkgroupID,
		Label:       ptrToNullStr(req.Label),
		Name:        ptrToNullStr(req.Name),
		Frequency:   ptrToNullInt(req.Frequency),
		Led:         ptrToNullStr(req.Led),
		GroupID:     ptrToNullInt(req.GroupID),
		TagID:       ptrToNullInt(req.TagID),
		Order:       req.Order,
	})
	if isUniqueViolation(err) {
		return nil, UserError("talkgroup already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update talkgroup: %w", err)
	}

	tg, err := o.Queries.GetTalkgroup(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated talkgroup: %w", err)
	}
	slog.Info("admin: talkgroup updated", "id", tg.ID, "talkgroup_id", tg.TalkgroupID, "by", callerID)
	o.broadcastAdminEvent("talkgroups.updated", nil)
	o.broadcastCFG(ctx)
	return mapTalkgroup(tg), nil
}

// TalkgroupsDelete deletes a talkgroup.
func (o *Operations) TalkgroupsDelete(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetTalkgroup(ctx, req.ID); err != nil {
		return nil, UserError("talkgroup not found")
	}

	if err := o.Queries.DeleteTalkgroup(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete talkgroup: %w", err)
	}
	slog.Info("admin: talkgroup deleted", "id", req.ID, "by", callerID)
	o.broadcastAdminEvent("talkgroups.updated", nil)
	o.broadcastCFG(ctx)
	return map[string]bool{"ok": true}, nil
}
