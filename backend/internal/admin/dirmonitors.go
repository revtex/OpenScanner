package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/openscanner/openscanner/internal/db"
)

// DirMonitorsList returns all dirmonitors.
func (o *Operations) DirMonitorsList(ctx context.Context, _ json.RawMessage, _ int64) (any, error) {
	dms, err := o.Queries.ListDirMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list dirmonitors: %w", err)
	}
	return mapDirMonitors(dms), nil
}

// DirMonitorsCreate creates a new dirmonitor and triggers a reload.
func (o *Operations) DirMonitorsCreate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		Directory   string  `json:"directory"`
		Type        string  `json:"type"`
		Mask        *string `json:"mask"`
		Extension   *string `json:"extension"`
		Frequency   *int64  `json:"frequency"`
		Delay       *int64  `json:"delay"`
		DeleteAfter int64   `json:"deleteAfter"`
		UsePolling  int64   `json:"usePolling"`
		Disabled    int64   `json:"disabled"`
		SystemID    *int64  `json:"systemId"`
		TalkgroupID *int64  `json:"talkgroupId"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.Directory == "" {
		return nil, UserError("directory is required")
	}
	if info, statErr := os.Stat(req.Directory); statErr != nil {
		return nil, UserError("directory does not exist or is not accessible: " + statErr.Error())
	} else if !info.IsDir() {
		return nil, UserError("path is not a directory: " + req.Directory)
	}

	id, err := o.Queries.CreateDirMonitor(ctx, db.CreateDirMonitorParams{
		Directory:   req.Directory,
		Type:        req.Type,
		Mask:        ptrToNullStr(req.Mask),
		Extension:   ptrToNullStr(req.Extension),
		Frequency:   ptrToNullInt(req.Frequency),
		Delay:       ptrToNullInt(req.Delay),
		DeleteAfter: req.DeleteAfter,
		UsePolling:  req.UsePolling,
		Disabled:    req.Disabled,
		SystemID:    ptrToNullInt(req.SystemID),
		TalkgroupID: ptrToNullInt(req.TalkgroupID),
		Order:       req.Order,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create dirmonitor: %w", err)
	}

	dm, err := o.Queries.GetDirMonitor(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created dirmonitor: %w", err)
	}
	if o.Deps.DirMonitorReload != nil {
		o.Deps.DirMonitorReload.Reload()
	}
	slog.Info("admin: dirmonitor created", "id", dm.ID, "dir", dm.Directory, "by", callerID)
	o.broadcastAdminEvent("dirmonitors.updated", nil)
	return mapDirMonitor(dm), nil
}

// DirMonitorsUpdate updates a dirmonitor and triggers a reload.
func (o *Operations) DirMonitorsUpdate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID          int64   `json:"id"`
		Directory   string  `json:"directory"`
		Type        string  `json:"type"`
		Mask        *string `json:"mask"`
		Extension   *string `json:"extension"`
		Frequency   *int64  `json:"frequency"`
		Delay       *int64  `json:"delay"`
		DeleteAfter int64   `json:"deleteAfter"`
		UsePolling  int64   `json:"usePolling"`
		Disabled    int64   `json:"disabled"`
		SystemID    *int64  `json:"systemId"`
		TalkgroupID *int64  `json:"talkgroupId"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}
	if req.Directory == "" {
		return nil, UserError("directory is required")
	}
	if info, statErr := os.Stat(req.Directory); statErr != nil {
		return nil, UserError("directory does not exist or is not accessible: " + statErr.Error())
	} else if !info.IsDir() {
		return nil, UserError("path is not a directory: " + req.Directory)
	}

	if _, err := o.Queries.GetDirMonitor(ctx, req.ID); err != nil {
		return nil, UserError("dirmonitor not found")
	}

	if err := o.Queries.UpdateDirMonitor(ctx, db.UpdateDirMonitorParams{
		ID:          req.ID,
		Directory:   req.Directory,
		Type:        req.Type,
		Mask:        ptrToNullStr(req.Mask),
		Extension:   ptrToNullStr(req.Extension),
		Frequency:   ptrToNullInt(req.Frequency),
		Delay:       ptrToNullInt(req.Delay),
		DeleteAfter: req.DeleteAfter,
		UsePolling:  req.UsePolling,
		Disabled:    req.Disabled,
		SystemID:    ptrToNullInt(req.SystemID),
		TalkgroupID: ptrToNullInt(req.TalkgroupID),
		Order:       req.Order,
	}); err != nil {
		return nil, fmt.Errorf("failed to update dirmonitor: %w", err)
	}

	dm, err := o.Queries.GetDirMonitor(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated dirmonitor: %w", err)
	}
	if o.Deps.DirMonitorReload != nil {
		o.Deps.DirMonitorReload.Reload()
	}
	slog.Info("admin: dirmonitor updated", "id", dm.ID, "dir", dm.Directory, "by", callerID)
	o.broadcastAdminEvent("dirmonitors.updated", nil)
	return mapDirMonitor(dm), nil
}

// DirMonitorsDelete deletes a dirmonitor and triggers a reload.
func (o *Operations) DirMonitorsDelete(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetDirMonitor(ctx, req.ID); err != nil {
		return nil, UserError("dirmonitor not found")
	}

	if err := o.Queries.DeleteDirMonitor(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete dirmonitor: %w", err)
	}
	if o.Deps.DirMonitorReload != nil {
		o.Deps.DirMonitorReload.Reload()
	}
	slog.Info("admin: dirmonitor deleted", "id", req.ID, "by", callerID)
	o.broadcastAdminEvent("dirmonitors.updated", nil)
	return map[string]bool{"ok": true}, nil
}
