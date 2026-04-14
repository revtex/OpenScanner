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

// ImportTalkgroups handles POST /api/admin/import/talkgroups.
// Accepts a multipart CSV file, a system_id form field, and an optional mode field.
//
// @Summary      Import talkgroups from CSV
// @Description  Accepts a multipart CSV file with talkgroup data and a system_id form field. Columns: talkgroup_id, label, name, tag_id, group_id, frequency, led, order. Header rows are auto-skipped. Use mode=overwrite (default) to update existing talkgroups or mode=skip to leave existing talkgroups unchanged.
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

	var inserted, updated, skipped int
	headerSkipped := false
	for {
		record, err := reader.Read()
		if err == io.EOF {
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

		// talkgroup_id is required.
		tgID, err := strconv.ParseInt(col0, 10, 64)
		if err != nil {
			continue // skip rows with invalid talkgroup_id
		}

		// Check if talkgroup already exists.
		_, existsErr := h.queries.GetTalkgroupBySystemAndTGID(ctx, db.GetTalkgroupBySystemAndTGIDParams{
			SystemID:    systemID,
			TalkgroupID: tgID,
		})
		exists := !errors.Is(existsErr, sql.ErrNoRows)

		if exists && mode == "skip" {
			skipped++
			if inserted+updated+skipped >= maxImportRows {
				break
			}
			continue
		}

		params := db.UpsertTalkgroupParams{
			SystemID:    systemID,
			TalkgroupID: tgID,
		}

		if len(record) > 1 && strings.TrimSpace(record[1]) != "" {
			params.Label = sql.NullString{String: strings.TrimSpace(record[1]), Valid: true}
		}
		if len(record) > 2 && strings.TrimSpace(record[2]) != "" {
			params.Name = sql.NullString{String: strings.TrimSpace(record[2]), Valid: true}
		}
		if len(record) > 3 && strings.TrimSpace(record[3]) != "" {
			if v, err := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64); err == nil {
				params.TagID = sql.NullInt64{Int64: v, Valid: true}
			}
		}
		if len(record) > 4 && strings.TrimSpace(record[4]) != "" {
			if v, err := strconv.ParseInt(strings.TrimSpace(record[4]), 10, 64); err == nil {
				params.GroupID = sql.NullInt64{Int64: v, Valid: true}
			}
		}
		if len(record) > 5 && strings.TrimSpace(record[5]) != "" {
			if v, err := strconv.ParseInt(strings.TrimSpace(record[5]), 10, 64); err == nil {
				params.Frequency = sql.NullInt64{Int64: v, Valid: true}
			}
		}
		if len(record) > 6 && strings.TrimSpace(record[6]) != "" {
			params.Led = sql.NullString{String: strings.TrimSpace(record[6]), Valid: true}
		}
		if len(record) > 7 && strings.TrimSpace(record[7]) != "" {
			if v, err := strconv.ParseInt(strings.TrimSpace(record[7]), 10, 64); err == nil {
				params.Order = v
			}
		}

		if err := h.queries.UpsertTalkgroup(ctx, params); err != nil {
			slog.Error("failed to upsert talkgroup", "talkgroup_id", tgID, "error", err)
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

	slog.Info("talkgroups imported", "system_id", systemID,
		"inserted", inserted, "updated", updated, "skipped", skipped)
	c.JSON(http.StatusOK, gin.H{
		"inserted": inserted,
		"updated":  updated,
		"skipped":  skipped,
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

	var inserted, updated, skipped int
	headerSkipped := false
	for {
		record, err := reader.Read()
		if err == io.EOF {
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
		"inserted", inserted, "updated", updated, "skipped", skipped)
	c.JSON(http.StatusOK, gin.H{
		"inserted": inserted,
		"updated":  updated,
		"skipped":  skipped,
	})
}
