package api

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

const rrMergeModeFillMissing = "fill_missing"
const rrMergeModeOverwriteSelected = "overwrite_selected"

var rrUpdatableFields = map[string]bool{
	"label": true,
	"name":  true,
	"group": true,
	"tag":   true,
	"led":   true,
	"order": true,
}

// RRRowError describes a per-row error in RadioReference enrichment.
type RRRowError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
} // @name RRRowError

// RRTalkgroupCandidate is a normalized enrichment candidate from RadioReference.
type RRTalkgroupCandidate struct {
	Row         int     `json:"row"`
	TalkgroupID int64   `json:"talkgroupId"`
	Label       *string `json:"label,omitempty"`
	Name        *string `json:"name,omitempty"`
	Group       *string `json:"group,omitempty"`
	Tag         *string `json:"tag,omitempty"`
	Led         *string `json:"led,omitempty"`
	Order       *int64  `json:"order,omitempty"`
} // @name RRTalkgroupCandidate

// RRPreviewRow is one row in the enrichment preview.
type RRPreviewRow struct {
	RRTalkgroupCandidate
	Matched          bool     `json:"matched"`
	WouldUpdate      bool     `json:"wouldUpdate"`
	WouldUpdateField []string `json:"wouldUpdateFields"`
	SkipReason       string   `json:"skipReason,omitempty"`
} // @name RRPreviewRow

// RRPreviewResponse is the response from a RadioReference CSV preview.
type RRPreviewResponse struct {
	Processed   int            `json:"processed"`
	Matched     int            `json:"matched"`
	WouldUpdate int            `json:"wouldUpdate"`
	Skipped     int            `json:"skipped"`
	Errors      int            `json:"errors"`
	RowErrors   []RRRowError   `json:"rowErrors"`
	Rows        []RRPreviewRow `json:"rows"`
} // @name RRPreviewResponse

// RRApplyRequest is the request body for applying RadioReference enrichment.
type RRApplyRequest struct {
	SystemID       int64                  `json:"systemId"`
	Candidates     []RRTalkgroupCandidate `json:"candidates"`
	MergeMode      string                 `json:"mergeMode"`
	SelectedFields []string               `json:"selectedFields"`
} // @name RRApplyRequest

// RRApplyResponse is the result of applying RadioReference enrichment.
type RRApplyResponse struct {
	Processed int          `json:"processed"`
	Matched   int          `json:"matched"`
	Updated   int          `json:"updated"`
	Skipped   int          `json:"skipped"`
	Errors    int          `json:"errors"`
	RowErrors []RRRowError `json:"rowErrors"`
} // @name RRApplyResponse

// RadioReferencePreviewCSV handles POST /api/admin/radioreference/preview/csv.
//
//	@Summary      Preview RadioReference CSV enrichment
//	@Description  Upload a RadioReference CSV export and preview which local talkgroups would be enriched. Frequency is never updated. Columns: talkgroup id (decimal/tgid), alpha tag, description, group/category, tag/service type, led, order.
//	@Tags         Admin - RadioReference
//	@Accept       multipart/form-data
//	@Produce      json
//	@Param        system_id  formData  int   true  "Local system ID to match talkgroups against"
//	@Param        file       formData  file  true  "RadioReference CSV file"
//	@Success      200  {object}  RRPreviewResponse
//	@Failure      400  {object}  ErrorResponse
//	@Security     BearerAuth
//	@Router       /admin/radioreference/preview/csv [post]
func (h *AdminHandler) RadioReferencePreviewCSV(c *gin.Context) {
	ctx := c.Request.Context()

	systemID, ok := parseSystemIDForm(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetSystem(ctx, systemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "system not found"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	limited := io.LimitReader(file, 5<<20) // 5 MiB max CSV size
	candidates, rowErrors, err := parseRadioReferenceCSV(limited)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := RRPreviewResponse{RowErrors: rowErrors}
	for _, candidate := range candidates {
		resp.Processed++
		preview := RRPreviewRow{RRTalkgroupCandidate: candidate}

		tg, tgErr := h.queries.GetTalkgroupBySystemAndTGID(ctx, db.GetTalkgroupBySystemAndTGIDParams{
			SystemID:    systemID,
			TalkgroupID: candidate.TalkgroupID,
		})
		if tgErr != nil {
			if errors.Is(tgErr, sql.ErrNoRows) {
				resp.Skipped++
				// Do not add not-found rows to the preview; skipped count is enough.
				continue
			}
			resp.Errors++
			resp.RowErrors = append(resp.RowErrors, RRRowError{Row: candidate.Row, Reason: "database error"})
			continue
		}

		preview.Matched = true
		resp.Matched++
		fields := rrCandidateFieldsForMode(tg, candidate, rrMergeModeFillMissing, nil)
		preview.WouldUpdateField = fields
		preview.WouldUpdate = len(fields) > 0
		if preview.WouldUpdate {
			resp.WouldUpdate++
		}
		resp.Rows = append(resp.Rows, preview)
	}

	resp.Errors += len(rowErrors)
	c.JSON(http.StatusOK, resp)
}

// RadioReferenceApply handles POST /api/admin/radioreference/apply.
//
//	@Summary      Apply RadioReference enrichment
//	@Description  Apply previously previewed RadioReference talkgroup enrichment candidates. Supports fill_missing (default) and overwrite_selected merge modes with per-field toggles. Frequency is never updated.
//	@Tags         Admin - RadioReference
//	@Accept       json
//	@Produce      json
//	@Param        body  body      RRApplyRequest   true  "Candidates and merge options"
//	@Success      200   {object}  RRApplyResponse
//	@Failure      400   {object}  ErrorResponse
//	@Failure      500   {object}  ErrorResponse
//	@Security     BearerAuth
//	@Router       /admin/radioreference/apply [post]
func (h *AdminHandler) RadioReferenceApply(c *gin.Context) {
	ctx := c.Request.Context()

	var req RRApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.SystemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "systemId is required"})
		return
	}
	if len(req.Candidates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "candidates are required"})
		return
	}
	if len(req.Candidates) > maxImportRows {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many candidates"})
		return
	}
	if req.MergeMode == "" {
		req.MergeMode = rrMergeModeFillMissing
	}
	if req.MergeMode != rrMergeModeFillMissing && req.MergeMode != rrMergeModeOverwriteSelected {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mergeMode must be 'fill_missing' or 'overwrite_selected'"})
		return
	}
	if _, err := h.queries.GetSystem(ctx, req.SystemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "system not found"})
		return
	}

	selected := rrSanitizeSelectedFields(req.SelectedFields)
	resp := RRApplyResponse{}
	for _, candidate := range req.Candidates {
		resp.Processed++

		tg, tgErr := h.queries.GetTalkgroupBySystemAndTGID(ctx, db.GetTalkgroupBySystemAndTGIDParams{
			SystemID:    req.SystemID,
			TalkgroupID: candidate.TalkgroupID,
		})
		if tgErr != nil {
			if errors.Is(tgErr, sql.ErrNoRows) {
				resp.Skipped++
				resp.RowErrors = append(resp.RowErrors, RRRowError{Row: candidate.Row, Reason: "talkgroup not found in selected system"})
				continue
			}
			resp.Errors++
			resp.RowErrors = append(resp.RowErrors, RRRowError{Row: candidate.Row, Reason: "database error"})
			continue
		}
		resp.Matched++

		params := db.UpdateTalkgroupParams{
			ID:          tg.ID,
			TalkgroupID: tg.TalkgroupID,
			Label:       tg.Label,
			Name:        tg.Name,
			Frequency:   tg.Frequency, // Keep existing value; RR never updates frequency.
			Led:         tg.Led,
			GroupID:     tg.GroupID,
			TagID:       tg.TagID,
			Order:       tg.Order,
		}

		applyFields := rrCandidateFieldsForMode(tg, candidate, req.MergeMode, selected)
		if len(applyFields) == 0 {
			resp.Skipped++
			continue
		}

		if err := h.applyRRFieldUpdates(ctx, &params, candidate, applyFields); err != nil {
			resp.Errors++
			resp.RowErrors = append(resp.RowErrors, RRRowError{Row: candidate.Row, Reason: err.Error()})
			continue
		}

		if err := h.queries.UpdateTalkgroup(ctx, params); err != nil {
			resp.Errors++
			resp.RowErrors = append(resp.RowErrors, RRRowError{Row: candidate.Row, Reason: "database error"})
			continue
		}
		resp.Updated++
	}

	c.JSON(http.StatusOK, resp)
}

func parseSystemIDForm(c *gin.Context) (int64, bool) {
	systemIDStr := c.PostForm("system_id")
	if systemIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "system_id is required"})
		return 0, false
	}
	systemID, err := strconv.ParseInt(systemIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid system_id"})
		return 0, false
	}
	return systemID, true
}

func parseRadioReferenceCSV(r io.Reader) ([]RRTalkgroupCandidate, []RRRowError, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, errors.New("invalid CSV format")
	}
	if len(records) == 0 {
		return nil, nil, errors.New("CSV is empty")
	}

	header := mapCSVHeaders(records[0])
	tgidIndex, ok := header["talkgroupid"]
	if !ok {
		return nil, nil, errors.New("missing required talkgroup id column")
	}

	labelIndex := rrIndex(header, "label", "alphatag", "alpha")
	nameIndex := rrIndex(header, "name", "description")
	groupIndex := rrIndex(header, "group", "category")
	tagIndex := rrIndex(header, "tag", "servicetype", "service")
	ledIndex := rrIndex(header, "led")
	orderIndex := rrIndex(header, "order")

	candidates := make([]RRTalkgroupCandidate, 0, len(records)-1)
	rowErrors := make([]RRRowError, 0)
	for i := 1; i < len(records); i++ {
		rowNum := i + 1
		record := records[i]
		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue
		}
		if len(candidates) >= maxImportRows {
			break
		}

		tgidRaw := rrCell(record, tgidIndex)
		if tgidRaw == "" {
			rowErrors = append(rowErrors, RRRowError{Row: rowNum, Reason: "missing talkgroup id"})
			continue
		}
		tgid, parseErr := strconv.ParseInt(tgidRaw, 10, 64)
		if parseErr != nil {
			rowErrors = append(rowErrors, RRRowError{Row: rowNum, Reason: "invalid talkgroup id"})
			continue
		}

		candidate := RRTalkgroupCandidate{Row: rowNum, TalkgroupID: tgid}
		if v := rrCell(record, labelIndex); v != "" {
			candidate.Label = rrStringPtr(v)
		}
		if v := rrCell(record, nameIndex); v != "" {
			candidate.Name = rrStringPtr(v)
		}
		if v := rrCell(record, groupIndex); v != "" {
			candidate.Group = rrStringPtr(v)
		}
		if v := rrCell(record, tagIndex); v != "" {
			candidate.Tag = rrStringPtr(v)
		}
		if v := rrCell(record, ledIndex); v != "" {
			candidate.Led = rrStringPtr(v)
		}
		if v := rrCell(record, orderIndex); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				candidate.Order = &n
			}
		}
		candidates = append(candidates, candidate)
	}

	return candidates, rowErrors, nil
}

func mapCSVHeaders(row []string) map[string]int {
	m := make(map[string]int, len(row))
	for i, h := range row {
		norm := normalizeHeader(h)
		if norm == "" {
			continue
		}
		m[norm] = i
	}
	return m
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	v := b.String()
	switch v {
	case "tgid", "talkgroup", "talkgroupdecimal", "decimal", "dec":
		return "talkgroupid"
	case "alpha", "alphatag":
		return "label"
	}
	return v
}

func rrIndex(headers map[string]int, keys ...string) int {
	for _, key := range keys {
		if idx, ok := headers[key]; ok {
			return idx
		}
	}
	return -1
}

func rrCell(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func rrStringPtr(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func rrSanitizeSelectedFields(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		v := strings.ToLower(strings.TrimSpace(f))
		if !rrUpdatableFields[v] {
			continue
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func rrCandidateFieldsForMode(tg db.Talkgroup, candidate RRTalkgroupCandidate, mergeMode string, selectedFields []string) []string {
	allow := map[string]bool{}
	if mergeMode == rrMergeModeOverwriteSelected {
		for _, f := range selectedFields {
			allow[f] = true
		}
	}

	fields := make([]string, 0, 6)
	check := func(field string, hasCandidate bool, targetEmpty bool) {
		if !hasCandidate {
			return
		}
		if mergeMode == rrMergeModeOverwriteSelected {
			if allow[field] {
				fields = append(fields, field)
			}
			return
		}
		if targetEmpty {
			fields = append(fields, field)
		}
	}

	check("label", candidate.Label != nil, !tg.Label.Valid || strings.TrimSpace(tg.Label.String) == "")
	check("name", candidate.Name != nil, !tg.Name.Valid || strings.TrimSpace(tg.Name.String) == "")
	check("group", candidate.Group != nil, !tg.GroupID.Valid)
	check("tag", candidate.Tag != nil, !tg.TagID.Valid)
	check("led", candidate.Led != nil, !tg.Led.Valid || strings.TrimSpace(tg.Led.String) == "")
	check("order", candidate.Order != nil, tg.Order == 0)
	return fields
}

func (h *AdminHandler) applyRRFieldUpdates(ctx context.Context, params *db.UpdateTalkgroupParams, candidate RRTalkgroupCandidate, fields []string) error {
	for _, field := range fields {
		switch field {
		case "label":
			if candidate.Label != nil {
				params.Label = sql.NullString{String: *candidate.Label, Valid: true}
			}
		case "name":
			if candidate.Name != nil {
				params.Name = sql.NullString{String: *candidate.Name, Valid: true}
			}
		case "group":
			if candidate.Group != nil {
				g, err := h.queries.GetGroupByLabel(ctx, *candidate.Group)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						newID, createErr := h.queries.CreateGroup(ctx, *candidate.Group)
						if createErr != nil {
							return errors.New("database error")
						}
						params.GroupID = sql.NullInt64{Int64: newID, Valid: true}
					} else {
						return errors.New("database error")
					}
				} else {
					params.GroupID = sql.NullInt64{Int64: g.ID, Valid: true}
				}
			}
		case "tag":
			if candidate.Tag != nil {
				t, err := h.queries.GetTagByLabel(ctx, *candidate.Tag)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						newID, createErr := h.queries.CreateTag(ctx, *candidate.Tag)
						if createErr != nil {
							return errors.New("database error")
						}
						params.TagID = sql.NullInt64{Int64: newID, Valid: true}
					} else {
						return errors.New("database error")
					}
				} else {
					params.TagID = sql.NullInt64{Int64: t.ID, Valid: true}
				}
			}
		case "led":
			if candidate.Led != nil {
				params.Led = sql.NullString{String: *candidate.Led, Valid: true}
			}
		case "order":
			if candidate.Order != nil {
				params.Order = *candidate.Order
			}
		}
	}
	return nil
}
