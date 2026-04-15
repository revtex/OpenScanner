package api

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/radioref"
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

// RRLoginRequest is the request body for RadioReference API login.
type RRLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
} // @name RRLoginRequest

// rrClient returns a RadioReference API client using the configured app key, or nil.
func (h *AdminHandler) rrClient(ctx context.Context) (*radioref.Client, error) {
	s, err := h.queries.GetSetting(ctx, "radioReferenceAppKey")
	if err != nil || strings.TrimSpace(s.Value) == "" {
		return nil, radioref.ErrNoAppKey
	}
	return radioref.NewClient(s.Value), nil
}

// rrSessionCreds extracts RR session credentials from the X-RR-Session header.
func (h *AdminHandler) rrSessionCreds(c *gin.Context) (username, password string, ok bool) {
	token := c.GetHeader("X-RR-Session")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "RadioReference session required — please log in first"})
		return "", "", false
	}
	sess := h.rrSessions.Get(token)
	if sess == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "RadioReference session expired — please log in again"})
		return "", "", false
	}
	return sess.Username, sess.Password, true
}

// RadioReferenceLogin handles POST /api/admin/radioreference/login.
//
//	@Summary      Login to RadioReference
//	@Description  Validate RadioReference credentials for the current admin session. Credentials are not persisted. Returns a session token for subsequent API hierarchy requests.
//	@Tags         Admin - RadioReference
//	@Accept       json
//	@Produce      json
//	@Param        body  body      RRLoginRequest  true  "RadioReference credentials"
//	@Success      200   {object}  object          "Login successful, includes rrSession token"
//	@Failure      400   {object}  ErrorResponse
//	@Failure      401   {object}  ErrorResponse
//	@Failure      501   {object}  ErrorResponse
//	@Security     BearerAuth
//	@Router       /admin/radioreference/login [post]
func (h *AdminHandler) RadioReferenceLogin(c *gin.Context) {
	var req RRLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	client, err := h.rrClient(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RadioReference API key is not configured — set radioReferenceAppKey in admin settings"})
		return
	}

	user, err := client.GetUserData(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "RadioReference login failed: " + err.Error()})
		return
	}

	token, err := h.rrSessions.Create(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rrSession":     token,
		"username":      user.Username,
		"subExpireDate": user.ExpiresAt,
	})
}

// RadioReferenceCountries handles GET /api/admin/radioreference/countries.
//
//	@Summary      List RadioReference countries
//	@Description  Returns the list of countries available in RadioReference.
//	@Tags         Admin - RadioReference
//	@Produce      json
//	@Success      200  {array}   object  "List of countries"
//	@Failure      501  {object}  ErrorResponse
//	@Security     BearerAuth
//	@Router       /admin/radioreference/countries [get]
func (h *AdminHandler) RadioReferenceCountries(c *gin.Context) {
	client, err := h.rrClient(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RadioReference API key is not configured"})
		return
	}
	countries, err := client.GetCountryList(c.Request.Context())
	if err != nil {
		slog.Error("RadioReference API call failed", "action", "getCountryList", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch countries from RadioReference"})
		return
	}
	c.JSON(http.StatusOK, countries)
}

// RadioReferenceStates handles GET /api/admin/radioreference/states.
//
//	@Summary      List RadioReference states
//	@Description  Returns the list of states/provinces for a given country.
//	@Tags         Admin - RadioReference
//	@Produce      json
//	@Param        countryId  query    int     true  "Country ID from RadioReference"
//	@Success      200        {array}  object  "List of states"
//	@Failure      400        {object} ErrorResponse
//	@Failure      501        {object} ErrorResponse
//	@Security     BearerAuth
//	@Router       /admin/radioreference/states [get]
func (h *AdminHandler) RadioReferenceStates(c *gin.Context) {
	client, err := h.rrClient(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RadioReference API key is not configured"})
		return
	}
	username, password, ok := h.rrSessionCreds(c)
	if !ok {
		return
	}
	countryIDStr := c.Query("countryId")
	if countryIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "countryId query parameter is required"})
		return
	}
	countryID, err := strconv.Atoi(countryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid countryId"})
		return
	}
	states, err := client.GetStates(c.Request.Context(), countryID, username, password)
	if err != nil {
		slog.Error("RadioReference API call failed", "action", "getStates", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch states from RadioReference"})
		return
	}
	c.JSON(http.StatusOK, states)
}

// RadioReferenceCounties handles GET /api/admin/radioreference/counties.
//
//	@Summary      List RadioReference counties
//	@Description  Returns the list of counties for a given state.
//	@Tags         Admin - RadioReference
//	@Produce      json
//	@Param        stateId  query    int     true  "State ID from RadioReference"
//	@Success      200      {array}  object  "List of counties"
//	@Failure      400      {object} ErrorResponse
//	@Failure      501      {object} ErrorResponse
//	@Security     BearerAuth
//	@Router       /admin/radioreference/counties [get]
func (h *AdminHandler) RadioReferenceCounties(c *gin.Context) {
	client, err := h.rrClient(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RadioReference API key is not configured"})
		return
	}
	username, password, ok := h.rrSessionCreds(c)
	if !ok {
		return
	}
	stateIDStr := c.Query("stateId")
	if stateIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stateId query parameter is required"})
		return
	}
	stateID, err := strconv.Atoi(stateIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stateId"})
		return
	}
	counties, err := client.GetCounties(c.Request.Context(), stateID, username, password)
	if err != nil {
		slog.Error("RadioReference API call failed", "action", "getCounties", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch counties from RadioReference"})
		return
	}
	c.JSON(http.StatusOK, counties)
}

// RadioReferenceSystems handles GET /api/admin/radioreference/systems.
//
//	@Summary      List RadioReference systems
//	@Description  Returns the list of radio systems for a given county.
//	@Tags         Admin - RadioReference
//	@Produce      json
//	@Param        countyId  query    int     true  "County ID from RadioReference"
//	@Success      200       {array}  object  "List of systems"
//	@Failure      400       {object} ErrorResponse
//	@Failure      501       {object} ErrorResponse
//	@Security     BearerAuth
//	@Router       /admin/radioreference/systems [get]
func (h *AdminHandler) RadioReferenceSystems(c *gin.Context) {
	client, err := h.rrClient(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RadioReference API key is not configured"})
		return
	}
	username, password, ok := h.rrSessionCreds(c)
	if !ok {
		return
	}
	countyIDStr := c.Query("countyId")
	if countyIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "countyId query parameter is required"})
		return
	}
	countyID, err := strconv.Atoi(countyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid countyId"})
		return
	}
	systems, err := client.GetSystems(c.Request.Context(), countyID, username, password)
	if err != nil {
		slog.Error("RadioReference API call failed", "action", "getSystems", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch systems from RadioReference"})
		return
	}
	c.JSON(http.StatusOK, systems)
}

// RRAPIPreviewRequest is the request body for API-based enrichment preview.
type RRAPIPreviewRequest struct {
	RRSystemID int   `json:"rrSystemId"`
	SystemID   int64 `json:"systemId"`
} // @name RRAPIPreviewRequest

// RadioReferencePreviewAPI handles POST /api/admin/radioreference/preview/api.
//
//	@Summary      Preview RadioReference API enrichment
//	@Description  Fetch talkgroups from a RadioReference trunked system and preview which local talkgroups would be enriched. Requires a valid RR session (X-RR-Session header). Frequency is never updated.
//	@Tags         Admin - RadioReference
//	@Accept       json
//	@Produce      json
//	@Param        body  body      RRAPIPreviewRequest  true  "RR system ID and local system ID"
//	@Success      200   {object}  RRPreviewResponse
//	@Failure      400   {object}  ErrorResponse
//	@Failure      401   {object}  ErrorResponse
//	@Failure      502   {object}  ErrorResponse
//	@Security     BearerAuth
//	@Router       /admin/radioreference/preview/api [post]
func (h *AdminHandler) RadioReferencePreviewAPI(c *gin.Context) {
	ctx := c.Request.Context()

	var req RRAPIPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.RRSystemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rrSystemId is required"})
		return
	}
	if req.SystemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "systemId is required"})
		return
	}

	client, err := h.rrClient(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RadioReference API key is not configured"})
		return
	}
	username, password, ok := h.rrSessionCreds(c)
	if !ok {
		return
	}

	if _, err := h.queries.GetSystem(ctx, req.SystemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "local system not found"})
		return
	}

	// Fetch talkgroups and categories from RR.
	rrTalkgroups, err := client.GetTrsTalkgroups(ctx, req.RRSystemID, username, password)
	if err != nil {
		slog.Error("RadioReference API call failed", "action", "getTrsTalkgroups", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch talkgroups from RadioReference"})
		return
	}
	rrCats, err := client.GetTrsTalkgroupCats(ctx, req.RRSystemID, username, password)
	if err != nil {
		slog.Error("RadioReference API call failed", "action", "getTrsTalkgroupCats", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch talkgroup categories from RadioReference"})
		return
	}

	// Build category ID → name map.
	catMap := make(map[int]string, len(rrCats))
	for _, cat := range rrCats {
		catMap[cat.CID] = cat.Name
	}

	// Convert RR talkgroups to candidates.
	candidates := make([]RRTalkgroupCandidate, 0, len(rrTalkgroups))
	for i, tg := range rrTalkgroups {
		candidate := RRTalkgroupCandidate{
			Row:         i + 1,
			TalkgroupID: int64(tg.Dec),
		}
		if tg.Alpha != "" {
			candidate.Label = rrStringPtr(tg.Alpha)
		}
		if tg.Descr != "" {
			candidate.Name = rrStringPtr(tg.Descr)
		}
		if catName, ok := catMap[tg.CatID]; ok && catName != "" {
			candidate.Group = rrStringPtr(catName)
		}
		if len(tg.Tags) > 0 && tg.Tags[0].Name != "" {
			candidate.Tag = rrStringPtr(tg.Tags[0].Name)
		}
		if tg.Sort > 0 {
			order := int64(tg.Sort)
			candidate.Order = &order
		}
		candidates = append(candidates, candidate)
	}

	// Build preview using the same engine as CSV mode.
	resp := RRPreviewResponse{RowErrors: make([]RRRowError, 0)}
	for _, candidate := range candidates {
		resp.Processed++
		preview := RRPreviewRow{RRTalkgroupCandidate: candidate}

		tg, tgErr := h.queries.GetTalkgroupBySystemAndTGID(ctx, db.GetTalkgroupBySystemAndTGIDParams{
			SystemID:    req.SystemID,
			TalkgroupID: candidate.TalkgroupID,
		})
		if tgErr != nil {
			if errors.Is(tgErr, sql.ErrNoRows) {
				resp.Skipped++
				preview.SkipReason = "talkgroup not found in selected system"
				resp.Rows = append(resp.Rows, preview)
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

	c.JSON(http.StatusOK, resp)
}

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
				preview.SkipReason = "talkgroup not found in selected system"
				resp.Rows = append(resp.Rows, preview)
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
						return errors.New("group not found: " + *candidate.Group)
					}
					return errors.New("database error")
				}
				params.GroupID = sql.NullInt64{Int64: g.ID, Valid: true}
			}
		case "tag":
			if candidate.Tag != nil {
				t, err := h.queries.GetTagByLabel(ctx, *candidate.Tag)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return errors.New("tag not found: " + *candidate.Tag)
					}
					return errors.New("database error")
				}
				params.TagID = sql.NullInt64{Int64: t.ID, Valid: true}
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
