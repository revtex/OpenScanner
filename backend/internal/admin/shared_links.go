package admin

import (
	"context"
	"encoding/json"
	"fmt"
)

// SharedLinksList returns all shared links.
func (o *Operations) SharedLinksList(ctx context.Context, _ json.RawMessage, _ int64) (any, error) {
	rows, err := o.Queries.ListSharedLinks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list shared links: %w", err)
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapSharedLink(r))
	}
	return items, nil
}

// SharedLinksDelete deletes a shared link.
func (o *Operations) SharedLinksDelete(ctx context.Context, params json.RawMessage, _ int64) (any, error) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.ID <= 0 {
		return nil, UserError("id is required")
	}

	if err := o.Queries.DeleteSharedLink(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete shared link: %w", err)
	}
	o.broadcastAdminEvent("shared-links.updated", nil)
	return map[string]bool{"deleted": true}, nil
}
