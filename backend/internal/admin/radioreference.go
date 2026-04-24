package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/openscanner/openscanner/internal/db"
)

// RadioReferenceApply merges RadioReference-sourced talkgroup metadata into
// the local DB, using either fill-missing or overwrite-selected semantics.
func (o *Operations) RadioReferenceApply(ctx context.Context, params json.RawMessage, _ int64) (any, error) {
	type rrCandidate struct {
		Row         int     `json:"row"`
		TalkgroupID int64   `json:"talkgroupId"`
		Label       *string `json:"label,omitempty"`
		Name        *string `json:"name,omitempty"`
		Group       *string `json:"group,omitempty"`
		Tag         *string `json:"tag,omitempty"`
		Led         *string `json:"led,omitempty"`
		Order       *int64  `json:"order,omitempty"`
	}

	var req struct {
		SystemID       int64         `json:"systemId"`
		Candidates     []rrCandidate `json:"candidates"`
		MergeMode      string        `json:"mergeMode"`
		SelectedFields []string      `json:"selectedFields"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, UserError("invalid request body")
	}
	if req.SystemID <= 0 {
		return nil, UserError("systemId is required")
	}
	if len(req.Candidates) == 0 {
		return nil, UserError("candidates are required")
	}
	if len(req.Candidates) > 100_000 {
		return nil, UserError("too many candidates")
	}
	if req.MergeMode == "" {
		req.MergeMode = "fill_missing"
	}
	if req.MergeMode != "fill_missing" && req.MergeMode != "overwrite_selected" {
		return nil, UserError("mergeMode must be 'fill_missing' or 'overwrite_selected'")
	}
	if _, err := o.Queries.GetSystem(ctx, req.SystemID); err != nil {
		return nil, UserError("system not found")
	}

	// Sanitize selected fields.
	rrUpdatable := map[string]bool{"label": true, "name": true, "group": true, "tag": true, "led": true, "order": true}
	selected := make([]string, 0, len(req.SelectedFields))
	for _, f := range req.SelectedFields {
		v := strings.ToLower(strings.TrimSpace(f))
		if rrUpdatable[v] {
			selected = append(selected, v)
		}
	}

	type rowErr struct {
		Row    int    `json:"row"`
		Reason string `json:"reason"`
	}
	resp := map[string]any{
		"processed": 0,
		"matched":   0,
		"updated":   0,
		"skipped":   0,
		"errors":    0,
		"rowErrors": []rowErr{},
	}
	processed, matched, updated, skippedCount, errCount := 0, 0, 0, 0, 0
	rowErrors := make([]rowErr, 0)

	for _, candidate := range req.Candidates {
		processed++

		tg, tgErr := o.Queries.GetTalkgroupBySystemAndTGID(ctx, db.GetTalkgroupBySystemAndTGIDParams{
			SystemID:    req.SystemID,
			TalkgroupID: candidate.TalkgroupID,
		})
		if tgErr != nil {
			if errors.Is(tgErr, sql.ErrNoRows) {
				skippedCount++
				rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "talkgroup not found in selected system"})
				continue
			}
			errCount++
			rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
			continue
		}
		matched++

		p := db.UpdateTalkgroupParams{
			ID:          tg.ID,
			TalkgroupID: tg.TalkgroupID,
			Label:       tg.Label,
			Name:        tg.Name,
			Frequency:   tg.Frequency,
			Led:         tg.Led,
			GroupID:     tg.GroupID,
			TagID:       tg.TagID,
			Order:       tg.Order,
		}

		// Determine which fields to apply.
		allow := map[string]bool{}
		if req.MergeMode == "overwrite_selected" {
			for _, f := range selected {
				allow[f] = true
			}
		}

		applyFields := make([]string, 0, 6)
		check := func(field string, hasCand bool, targetEmpty bool) {
			if !hasCand {
				return
			}
			if req.MergeMode == "overwrite_selected" {
				if allow[field] {
					applyFields = append(applyFields, field)
				}
				return
			}
			if targetEmpty {
				applyFields = append(applyFields, field)
			}
		}
		check("label", candidate.Label != nil, !tg.Label.Valid || strings.TrimSpace(tg.Label.String) == "")
		check("name", candidate.Name != nil, !tg.Name.Valid || strings.TrimSpace(tg.Name.String) == "")
		check("group", candidate.Group != nil, !tg.GroupID.Valid)
		check("tag", candidate.Tag != nil, !tg.TagID.Valid)
		check("led", candidate.Led != nil, !tg.Led.Valid || strings.TrimSpace(tg.Led.String) == "")
		check("order", candidate.Order != nil, tg.Order == 0)

		if len(applyFields) == 0 {
			skippedCount++
			continue
		}

		// Apply field updates.
		applyErr := false
		for _, field := range applyFields {
			switch field {
			case "label":
				if candidate.Label != nil {
					p.Label = sql.NullString{String: *candidate.Label, Valid: true}
				}
			case "name":
				if candidate.Name != nil {
					p.Name = sql.NullString{String: *candidate.Name, Valid: true}
				}
			case "group":
				if candidate.Group != nil {
					g, err := o.Queries.GetGroupByLabel(ctx, *candidate.Group)
					if err != nil {
						if errors.Is(err, sql.ErrNoRows) {
							newID, createErr := o.Queries.CreateGroup(ctx, *candidate.Group)
							if createErr != nil {
								errCount++
								rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
								applyErr = true
								break
							}
							p.GroupID = sql.NullInt64{Int64: newID, Valid: true}
						} else {
							errCount++
							rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
							applyErr = true
							break
						}
					} else {
						p.GroupID = sql.NullInt64{Int64: g.ID, Valid: true}
					}
				}
			case "tag":
				if candidate.Tag != nil {
					t, err := o.Queries.GetTagByLabel(ctx, *candidate.Tag)
					if err != nil {
						if errors.Is(err, sql.ErrNoRows) {
							newID, createErr := o.Queries.CreateTag(ctx, *candidate.Tag)
							if createErr != nil {
								errCount++
								rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
								applyErr = true
								break
							}
							p.TagID = sql.NullInt64{Int64: newID, Valid: true}
						} else {
							errCount++
							rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
							applyErr = true
							break
						}
					} else {
						p.TagID = sql.NullInt64{Int64: t.ID, Valid: true}
					}
				}
			case "led":
				if candidate.Led != nil {
					p.Led = sql.NullString{String: *candidate.Led, Valid: true}
				}
			case "order":
				if candidate.Order != nil {
					p.Order = *candidate.Order
				}
			}
		}
		if applyErr {
			continue
		}

		if err := o.Queries.UpdateTalkgroup(ctx, p); err != nil {
			errCount++
			rowErrors = append(rowErrors, rowErr{Row: candidate.Row, Reason: "database error"})
			continue
		}
		updated++
	}

	resp["processed"] = processed
	resp["matched"] = matched
	resp["updated"] = updated
	resp["skipped"] = skippedCount
	resp["errors"] = errCount
	resp["rowErrors"] = rowErrors
	return resp, nil
}
