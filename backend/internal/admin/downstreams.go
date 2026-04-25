package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// DownstreamsList returns all downstreams.
func (o *Operations) DownstreamsList(ctx context.Context, _ json.RawMessage, _ int64) (any, error) {
	ds, err := o.Queries.ListDownstreams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list downstreams: %w", err)
	}
	return mapDownstreams(ds), nil
}

// DownstreamsCreate creates a new downstream and triggers a reload.
func (o *Operations) DownstreamsCreate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		Url         string  `json:"url"`
		ApiKey      string  `json:"apiKey"`
		SystemsJson *string `json:"systemsJson"`
		Disabled    int64   `json:"disabled"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.Url == "" {
		return nil, UserError("url is required")
	}
	if !validHTTPURL(req.Url) {
		return nil, UserError("url must use http or https scheme")
	}

	apiKey := req.ApiKey
	if o.Deps.EncryptionKey != "" && apiKey != "" {
		enc, err := auth.EncryptString(apiKey, o.Deps.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt downstream API key: %w", err)
		}
		apiKey = enc
	}

	id, err := o.Queries.CreateDownstream(ctx, db.CreateDownstreamParams{
		Url:         req.Url,
		ApiKey:      apiKey,
		SystemsJson: ptrToNullStr(req.SystemsJson),
		Disabled:    req.Disabled,
		Order:       req.Order,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create downstream: %w", err)
	}

	ds, err := o.Queries.GetDownstream(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created downstream: %w", err)
	}
	if o.Deps.DownstreamReload != nil {
		o.Deps.DownstreamReload.Reload()
	}
	slog.Info("admin: downstream created", "id", ds.ID, "url", ds.Url, "by", callerID)
	o.broadcastAdminEvent("downstreams.updated", nil)
	return mapDownstream(ds), nil
}

// DownstreamsUpdate updates a downstream. Blank apiKey preserves the current one.
func (o *Operations) DownstreamsUpdate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID          int64   `json:"id"`
		Url         string  `json:"url"`
		ApiKey      string  `json:"apiKey"`
		SystemsJson *string `json:"systemsJson"`
		Disabled    int64   `json:"disabled"`
		Order       int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}
	if req.Url != "" && !validHTTPURL(req.Url) {
		return nil, UserError("url must use http or https scheme")
	}

	existing, err := o.Queries.GetDownstream(ctx, req.ID)
	if err != nil {
		return nil, UserError("downstream not found")
	}

	// Preserve existing API key if none provided (key is never sent to clients).
	apiKey := existing.ApiKey
	if req.ApiKey != "" {
		if o.Deps.EncryptionKey != "" {
			enc, err := auth.EncryptString(req.ApiKey, o.Deps.EncryptionKey)
			if err != nil {
				return nil, fmt.Errorf("encrypt downstream API key: %w", err)
			}
			apiKey = enc
		} else {
			apiKey = req.ApiKey
		}
	}

	if err := o.Queries.UpdateDownstream(ctx, db.UpdateDownstreamParams{
		ID:          req.ID,
		Url:         req.Url,
		ApiKey:      apiKey,
		SystemsJson: ptrToNullStr(req.SystemsJson),
		Disabled:    req.Disabled,
		Order:       req.Order,
	}); err != nil {
		return nil, fmt.Errorf("failed to update downstream: %w", err)
	}

	ds, err := o.Queries.GetDownstream(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated downstream: %w", err)
	}
	if o.Deps.DownstreamReload != nil {
		o.Deps.DownstreamReload.Reload()
	}
	slog.Info("admin: downstream updated", "id", ds.ID, "url", ds.Url, "by", callerID)
	o.broadcastAdminEvent("downstreams.updated", nil)
	return mapDownstream(ds), nil
}

// DownstreamsDelete deletes a downstream and triggers a reload.
func (o *Operations) DownstreamsDelete(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetDownstream(ctx, req.ID); err != nil {
		return nil, UserError("downstream not found")
	}

	if err := o.Queries.DeleteDownstream(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete downstream: %w", err)
	}
	if o.Deps.DownstreamReload != nil {
		o.Deps.DownstreamReload.Reload()
	}
	slog.Info("admin: downstream deleted", "id", req.ID, "by", callerID)
	o.broadcastAdminEvent("downstreams.updated", nil)
	return map[string]bool{"ok": true}, nil
}
