package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// APIKeysList returns all API keys.
func (o *Operations) APIKeysList(ctx context.Context, _ json.RawMessage, _ int64) (any, error) {
	keys, err := o.Queries.ListAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	return mapAPIKeys(keys), nil
}

// APIKeysCreate creates a new API key. Returns the plaintext key once.
func (o *Operations) APIKeysCreate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		Key           *string `json:"key"`
		Ident         *string `json:"ident"`
		Disabled      int64   `json:"disabled"`
		SystemsJson   *string `json:"systemsJson"`
		CallRateLimit *int64  `json:"callRateLimit"`
		Order         int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}

	plainKey := uuid.New().String()
	if req.Key != nil && *req.Key != "" {
		plainKey = *req.Key
	}
	hashedKey := auth.HashAPIKey(plainKey)

	id, err := o.Queries.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		Key:           hashedKey,
		Ident:         ptrToNullStr(req.Ident),
		Disabled:      req.Disabled,
		SystemsJson:   ptrToNullStr(req.SystemsJson),
		CallRateLimit: ptrToNullInt(req.CallRateLimit),
		Order:         req.Order,
	})
	if isUniqueViolation(err) {
		return nil, UserError("API key already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	key, err := o.Queries.GetAPIKey(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created API key: %w", err)
	}
	slog.Info("admin: api key created", "id", key.ID, "ident", key.Ident.String, "by", callerID)
	o.broadcastAdminEvent("apikeys.updated", nil)

	resp := mapAPIKey(key)
	resp["createdKey"] = plainKey // Return plain key once on creation.
	return resp, nil
}

// APIKeysUpdate updates an existing API key. A blank key preserves the current one.
func (o *Operations) APIKeysUpdate(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID            int64   `json:"id"`
		Key           *string `json:"key"`
		Ident         *string `json:"ident"`
		Disabled      int64   `json:"disabled"`
		SystemsJson   *string `json:"systemsJson"`
		CallRateLimit *int64  `json:"callRateLimit"`
		Order         int64   `json:"order"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	current, err := o.Queries.GetAPIKey(ctx, req.ID)
	if err != nil {
		return nil, UserError("API key not found")
	}

	keyHash := current.Key
	if req.Key != nil && *req.Key != "" {
		keyHash = auth.HashAPIKey(*req.Key)
	}

	err = o.Queries.UpdateAPIKey(ctx, db.UpdateAPIKeyParams{
		ID:            req.ID,
		Key:           keyHash,
		Ident:         ptrToNullStr(req.Ident),
		Disabled:      req.Disabled,
		SystemsJson:   ptrToNullStr(req.SystemsJson),
		CallRateLimit: ptrToNullInt(req.CallRateLimit),
		Order:         req.Order,
	})
	if isUniqueViolation(err) {
		return nil, UserError("API key already exists")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	key, err := o.Queries.GetAPIKey(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated API key: %w", err)
	}
	slog.Info("admin: api key updated", "id", key.ID, "ident", key.Ident.String, "by", callerID)
	o.broadcastAdminEvent("apikeys.updated", nil)
	return mapAPIKey(key), nil
}

// APIKeysDelete deletes an API key.
func (o *Operations) APIKeysDelete(ctx context.Context, params json.RawMessage, callerID int64) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if _, err := o.Queries.GetAPIKey(ctx, req.ID); err != nil {
		return nil, UserError("API key not found")
	}

	if err := o.Queries.DeleteAPIKey(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete API key: %w", err)
	}
	slog.Info("admin: api key deleted", "id", req.ID, "by", callerID)
	o.broadcastAdminEvent("apikeys.updated", nil)
	return map[string]bool{"ok": true}, nil
}
