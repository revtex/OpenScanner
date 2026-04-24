package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/openscanner/openscanner/internal/db"
)

// WebhooksList returns all webhooks.
func (o *Operations) WebhooksList(ctx context.Context, _ json.RawMessage, _ int64) (any, error) {
	whs, err := o.Queries.ListWebhooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}
	return mapWebhooks(whs), nil
}

// WebhooksCreate creates a new webhook.
func (o *Operations) WebhooksCreate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		Url         string  `json:"url"`
		Type        string  `json:"type"`
		Secret      *string `json:"secret"`
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

	id, err := o.Queries.CreateWebhook(ctx, db.CreateWebhookParams{
		Url:         req.Url,
		Type:        req.Type,
		Secret:      ptrToNullStr(req.Secret),
		SystemsJson: ptrToNullStr(req.SystemsJson),
		Disabled:    req.Disabled,
		Order:       req.Order,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	wh, err := o.Queries.GetWebhook(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created webhook: %w", err)
	}
	slog.Info("admin: webhook created", "id", wh.ID, "url", wh.Url, "by", callerID)
	o.broadcastAdminEvent("webhooks.updated", nil)
	return mapWebhook(wh), nil
}

// WebhooksUpdate updates an existing webhook.
func (o *Operations) WebhooksUpdate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID          int64   `json:"id"`
		Url         string  `json:"url"`
		Type        string  `json:"type"`
		Secret      *string `json:"secret"`
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

	if _, err := o.Queries.GetWebhook(ctx, req.ID); err != nil {
		return nil, UserError("webhook not found")
	}

	if err := o.Queries.UpdateWebhook(ctx, db.UpdateWebhookParams{
		ID:          req.ID,
		Url:         req.Url,
		Type:        req.Type,
		Secret:      ptrToNullStr(req.Secret),
		SystemsJson: ptrToNullStr(req.SystemsJson),
		Disabled:    req.Disabled,
		Order:       req.Order,
	}); err != nil {
		return nil, fmt.Errorf("failed to update webhook: %w", err)
	}

	wh, err := o.Queries.GetWebhook(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated webhook: %w", err)
	}
	slog.Info("admin: webhook updated", "id", wh.ID, "url", wh.Url, "by", callerID)
	o.broadcastAdminEvent("webhooks.updated", nil)
	return mapWebhook(wh), nil
}

// WebhooksDelete deletes a webhook.
func (o *Operations) WebhooksDelete(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetWebhook(ctx, req.ID); err != nil {
		return nil, UserError("webhook not found")
	}

	if err := o.Queries.DeleteWebhook(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete webhook: %w", err)
	}
	slog.Info("admin: webhook deleted", "id", req.ID, "by", callerID)
	o.broadcastAdminEvent("webhooks.updated", nil)
	return map[string]bool{"ok": true}, nil
}
