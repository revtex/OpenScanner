package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/openscanner/openscanner/internal/db"
)

// SystemsList returns all systems.
func (o *Operations) SystemsList(ctx context.Context, _ json.RawMessage, _ int64) (any, error) {
	systems, err := o.Queries.ListSystems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list systems: %w", err)
	}
	return mapSystems(systems), nil
}

// SystemsCreate creates a new system.
func (o *Operations) SystemsCreate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		SystemID               int64   `json:"systemId"`
		Label                  string  `json:"label"`
		AutoPopulateTalkgroups int64   `json:"autoPopulateTalkgroups"`
		BlacklistsJson         *string `json:"blacklistsJson"`
		Led                    *string `json:"led"`
		Order                  int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}

	id, err := o.Queries.CreateSystem(ctx, db.CreateSystemParams{
		SystemID:               req.SystemID,
		Label:                  req.Label,
		AutoPopulateTalkgroups: req.AutoPopulateTalkgroups,
		BlacklistsJson:         ptrToNullStr(req.BlacklistsJson),
		Led:                    ptrToNullStr(req.Led),
		Order:                  req.Order,
	})
	if isUniqueViolation(err) {
		return nil, UserError("system_id already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create system: %w", err)
	}

	system, err := o.Queries.GetSystem(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created system: %w", err)
	}
	slog.Info("admin: system created", "id", system.ID, "system_id", system.SystemID, "label", system.Label, "by", callerID)
	o.broadcastAdminEvent("systems.updated", nil)
	o.broadcastCFG(ctx)
	return mapSystem(system), nil
}

// SystemsUpdate updates an existing system.
func (o *Operations) SystemsUpdate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID                     int64   `json:"id"`
		SystemID               int64   `json:"systemId"`
		Label                  string  `json:"label"`
		AutoPopulateTalkgroups int64   `json:"autoPopulateTalkgroups"`
		BlacklistsJson         *string `json:"blacklistsJson"`
		Led                    *string `json:"led"`
		Order                  int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetSystem(ctx, req.ID); err != nil {
		return nil, UserError("system not found")
	}

	err := o.Queries.UpdateSystem(ctx, db.UpdateSystemParams{
		ID:                     req.ID,
		SystemID:               req.SystemID,
		Label:                  req.Label,
		AutoPopulateTalkgroups: req.AutoPopulateTalkgroups,
		BlacklistsJson:         ptrToNullStr(req.BlacklistsJson),
		Led:                    ptrToNullStr(req.Led),
		Order:                  req.Order,
	})
	if isUniqueViolation(err) {
		return nil, UserError("system_id already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update system: %w", err)
	}

	system, err := o.Queries.GetSystem(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated system: %w", err)
	}
	slog.Info("admin: system updated", "id", system.ID, "system_id", system.SystemID, "by", callerID)
	o.broadcastAdminEvent("systems.updated", nil)
	o.broadcastCFG(ctx)
	return mapSystem(system), nil
}

// SystemsDelete deletes a system.
func (o *Operations) SystemsDelete(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetSystem(ctx, req.ID); err != nil {
		return nil, UserError("system not found")
	}

	if err := o.Queries.DeleteSystem(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete system: %w", err)
	}
	slog.Info("admin: system deleted", "id", req.ID, "by", callerID)
	o.broadcastAdminEvent("systems.updated", nil)
	o.broadcastCFG(ctx)
	return map[string]bool{"ok": true}, nil
}
