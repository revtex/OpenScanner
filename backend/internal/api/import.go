package api

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

// tgColumnMap maps logical field names to their CSV column index.
// A value of -1 means the column is not present.
type tgColumnMap struct {
	talkgroupID int
	label       int
	name        int
	tagID       int // OpenScanner integer FK
	groupID     int // OpenScanner integer FK
	tagName     int // rdio-scanner text name (resolved to FK)
	groupName   int // rdio-scanner text name (resolved to FK)
	frequency   int
	led         int
	order       int
}

// detectTgColumns inspects a header row and returns a column map.
// Returns nil if the row doesn't look like a header (first cell is a digit).
func detectTgColumns(header []string) *tgColumnMap {
	if len(header) == 0 {
		return nil
	}
	first := strings.TrimSpace(header[0])
	if len(first) > 0 && unicode.IsDigit(rune(first[0])) {
		return nil // not a header row
	}

	m := &tgColumnMap{
		talkgroupID: -1, label: -1, name: -1,
		tagID: -1, groupID: -1, tagName: -1, groupName: -1,
		frequency: -1, led: -1, order: -1,
	}

	for i, raw := range header {
		col := strings.ToLower(strings.TrimSpace(raw))
		switch col {
		// OpenScanner + rdio-scanner: decimal talkgroup ID
		case "talkgroup_id", "dec", "decimal":
			m.talkgroupID = i
		// OpenScanner label, rdio-scanner alpha_tag
		case "label", "alpha_tag", "alpha tag":
			m.label = i
		// OpenScanner name, rdio-scanner description
		case "name", "description":
			m.name = i
		// OpenScanner integer FK columns
		case "tag_id":
			m.tagID = i
		case "group_id":
			m.groupID = i
		// rdio-scanner text name columns
		case "tag", "category":
			m.tagName = i
		case "group", "service_type":
			m.groupName = i
		case "frequency", "freq":
			m.frequency = i
		case "led", "led_color", "color":
			m.led = i
		case "order", "priority":
			m.order = i
			// skip unknown columns (e.g. "hex")
		}
	}
	return m
}

// defaultTgColumns returns the positional column map matching
// OpenScanner's native CSV format (no header present).
func defaultTgColumns() *tgColumnMap {
	return &tgColumnMap{
		talkgroupID: 0, label: 1, name: 2,
		tagID: 3, groupID: 4, tagName: -1, groupName: -1,
		frequency: 5, led: 6, order: 7,
	}
}

// col returns the trimmed value at index i, or "" if out of range.
func col(record []string, i int) string {
	if i < 0 || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

// ImportTalkgroups handles POST /api/admin/import/talkgroups.
// Accepts a multipart CSV file, a system_id form field, and an optional mode field.
// Supports both OpenScanner and rdio-scanner CSV formats via header detection.
//
// @Summary      Import talkgroups from CSV
// @Description  Accepts a multipart CSV file with talkgroup data and a system_id form field. Supports OpenScanner format (talkgroup_id, label, name, tag_id, group_id, frequency, led, order) and rdio-scanner format (dec, hex, alpha_tag, description, tag, group, priority). Header rows are auto-detected; tag/group names are resolved to IDs automatically. Use mode=overwrite (default) to update existing talkgroups or mode=skip to leave existing talkgroups unchanged.
// @Tags         Admin
// @Accept       multipart/form-data
// @Produce      json
// @Param        system_id  formData  int     true   "System ID to import talkgroups into"
// @Param        file       formData  file    true   "CSV file"
// @Param        mode       formData  string  false  "Duplicate handling: overwrite (default) or skip"
// @Success      200  {object}  object  "inserted, updated, skipped counts"
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/import/talkgroups [post]
func (h *AdminHandler) ImportTalkgroups(c *gin.Context) {
	ctx := c.Request.Context()

	systemIDStr := c.PostForm("system_id")
	if systemIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "system_id is required"})
		return
	}
	systemID, err := strconv.ParseInt(systemIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid system_id"})
		return
	}

	// Verify system exists.
	if _, err := h.queries.GetSystem(ctx, systemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "system not found"})
		return
	}

	mode := c.DefaultPostForm("mode", "overwrite")
	if mode != "overwrite" && mode != "skip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be 'overwrite' or 'skip'"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // allow variable number of fields
	reader.TrimLeadingSpace = true

	// Read the first non-blank row to detect column layout.
	var columns *tgColumnMap
	var firstDataRow []string
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			slog.Warn("talkgroup import: file is empty", "system_id", systemID)
			c.JSON(http.StatusOK, gin.H{
				"inserted": 0, "updated": 0, "skipped": 0, "failed": 0,
				"message": "file is empty",
			})
			return
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid CSV format"})
			return
		}
		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue
		}
		columns = detectTgColumns(record)
		if columns == nil {
			// First row is data, not a header — use default positional layout.
			columns = defaultTgColumns()
			firstDataRow = record
		}
		break
	}

	if columns.talkgroupID < 0 {
		slog.Warn("talkgroup import: no talkgroup_id column detected",
			"system_id", systemID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "CSV header has no talkgroup_id / dec / decimal column; " +
				"cannot import without knowing which column holds the talkgroup ID",
		})
		return
	}

	var inserted, updated, skipped, failed int

	processRow := func(record []string) error {
		tgIDStr := col(record, columns.talkgroupID)
		tgID, err := strconv.ParseInt(tgIDStr, 10, 64)
		if err != nil {
			failed++
			return nil //nolint:nilerr // invalid talkgroup_id: count and skip
		}

		// Check if talkgroup already exists.
		_, existsErr := h.queries.GetTalkgroupBySystemAndTGID(ctx, db.GetTalkgroupBySystemAndTGIDParams{
			SystemID:    systemID,
			TalkgroupID: tgID,
		})
		exists := !errors.Is(existsErr, sql.ErrNoRows)

		if exists && mode == "skip" {
			skipped++
			return nil
		}

		params := db.UpsertTalkgroupParams{
			SystemID:    systemID,
			TalkgroupID: tgID,
		}

		if v := col(record, columns.label); v != "" {
			params.Label = sql.NullString{String: v, Valid: true}
		}
		if v := col(record, columns.name); v != "" {
			params.Name = sql.NullString{String: v, Valid: true}
		}

		// Tag: prefer integer FK (but verify it exists), fall back to name.
		if v := col(record, columns.tagID); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				if _, gerr := h.queries.GetTag(ctx, id); gerr == nil {
					params.TagID = sql.NullInt64{Int64: id, Valid: true}
				} else if name := col(record, columns.tagName); name != "" {
					params.TagID = resolveTagID(ctx, h.queries, name)
				}
			}
		} else if v := col(record, columns.tagName); v != "" {
			params.TagID = resolveTagID(ctx, h.queries, v)
		}

		// Group: prefer integer FK (but verify it exists), fall back to name.
		if v := col(record, columns.groupID); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				if _, gerr := h.queries.GetGroup(ctx, id); gerr == nil {
					params.GroupID = sql.NullInt64{Int64: id, Valid: true}
				} else if name := col(record, columns.groupName); name != "" {
					params.GroupID = resolveGroupID(ctx, h.queries, name)
				}
			}
		} else if v := col(record, columns.groupName); v != "" {
			params.GroupID = resolveGroupID(ctx, h.queries, v)
		}

		if v := col(record, columns.frequency); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				params.Frequency = sql.NullInt64{Int64: id, Valid: true}
			}
		}
		if v := col(record, columns.led); v != "" {
			params.Led = sql.NullString{String: v, Valid: true}
		}
		if v := col(record, columns.order); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				params.Order = id
			}
		}

		if err := h.queries.UpsertTalkgroup(ctx, params); err != nil {
			return err
		}
		if exists {
			updated++
		} else {
			inserted++
		}
		return nil
	}

	// Process the first data row if header detection consumed it.
	if firstDataRow != nil {
		if err := processRow(firstDataRow); err != nil {
			slog.Error("failed to upsert talkgroup", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
	}

	for {
		if inserted+updated+skipped >= maxImportRows {
			slog.Warn("CSV import row limit reached", "limit", maxImportRows)
			break
		}

		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			slog.Error("csv read error", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid CSV format"})
			return
		}
		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue
		}

		if err := processRow(record); err != nil {
			slog.Error("failed to upsert talkgroup", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
	}

	slog.Info("talkgroups imported", "system_id", systemID,
		"inserted", inserted, "updated", updated, "skipped", skipped, "failed", failed)
	c.JSON(http.StatusOK, gin.H{
		"inserted": inserted,
		"updated":  updated,
		"skipped":  skipped,
		"failed":   failed,
	})
}

// ImportUnits handles POST /api/admin/import/units.
// Accepts a multipart CSV file, a system_id form field, and an optional mode field.
//
// @Summary      Import units from CSV
// @Description  Accepts a multipart CSV file with unit data and a system_id form field. Columns: unit_id, label, order. Header rows are auto-skipped. Use mode=overwrite (default) to update existing units or mode=skip to leave existing units unchanged.
// @Tags         Admin
// @Accept       multipart/form-data
// @Produce      json
// @Param        system_id  formData  int     true   "System ID to import units into"
// @Param        file       formData  file    true   "CSV file"
// @Param        mode       formData  string  false  "Duplicate handling: overwrite (default) or skip"
// @Success      200  {object}  object  "inserted, updated, skipped counts"
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/import/units [post]
func (h *AdminHandler) ImportUnits(c *gin.Context) {
	ctx := c.Request.Context()

	systemIDStr := c.PostForm("system_id")
	if systemIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "system_id is required"})
		return
	}
	systemID, err := strconv.ParseInt(systemIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid system_id"})
		return
	}

	// Verify system exists.
	if _, err := h.queries.GetSystem(ctx, systemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "system not found"})
		return
	}

	mode := c.DefaultPostForm("mode", "overwrite")
	if mode != "overwrite" && mode != "skip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be 'overwrite' or 'skip'"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	var inserted, updated, skipped, failed int
	headerSkipped := false
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			slog.Error("csv read error", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid CSV format"})
			return
		}

		// Skip blank lines.
		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue
		}

		// Skip header row.
		col0 := strings.TrimSpace(record[0])
		if !headerSkipped && len(col0) > 0 && !unicode.IsDigit(rune(col0[0])) {
			headerSkipped = true
			continue
		}
		headerSkipped = true

		unitID, err := strconv.ParseInt(col0, 10, 64)
		if err != nil {
			failed++
			continue
		}

		// Check if unit already exists.
		_, existsErr := h.queries.GetUnitBySystemAndUnitID(ctx, db.GetUnitBySystemAndUnitIDParams{
			SystemID: systemID,
			UnitID:   unitID,
		})
		exists := !errors.Is(existsErr, sql.ErrNoRows)

		if exists && mode == "skip" {
			skipped++
			if inserted+updated+skipped >= maxImportRows {
				break
			}
			continue
		}

		params := db.UpsertUnitParams{
			SystemID: systemID,
			UnitID:   unitID,
		}

		if len(record) > 1 && strings.TrimSpace(record[1]) != "" {
			params.Label = sql.NullString{String: strings.TrimSpace(record[1]), Valid: true}
		}
		if len(record) > 2 && strings.TrimSpace(record[2]) != "" {
			if v, err := strconv.ParseInt(strings.TrimSpace(record[2]), 10, 64); err == nil {
				params.Order = v
			}
		}

		if err := h.queries.UpsertUnit(ctx, params); err != nil {
			slog.Error("failed to upsert unit", "unit_id", unitID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if exists {
			updated++
		} else {
			inserted++
		}
		if inserted+updated+skipped >= maxImportRows {
			slog.Warn("CSV import row limit reached", "limit", maxImportRows)
			break
		}
	}

	slog.Info("units imported", "system_id", systemID,
		"inserted", inserted, "updated", updated, "skipped", skipped, "failed", failed)
	c.JSON(http.StatusOK, gin.H{
		"inserted": inserted,
		"updated":  updated,
		"skipped":  skipped,
		"failed":   failed,
	})
}
