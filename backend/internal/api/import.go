package api

import (
	"database/sql"
	"encoding/csv"
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
// Accepts a multipart CSV file and a system_id form field.
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

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // allow variable number of fields
	reader.TrimLeadingSpace = true

	imported := 0
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

		// Skip header row: if first field starts with a non-numeric character.
		col0 := strings.TrimSpace(record[0])
		if imported == 0 && len(col0) > 0 && !unicode.IsDigit(rune(col0[0])) {
			continue
		}

		// talkgroup_id is required.
		tgID, err := strconv.ParseInt(col0, 10, 64)
		if err != nil {
			continue // skip rows with invalid talkgroup_id
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
		imported++
		if imported >= maxImportRows {
			slog.Warn("CSV import row limit reached", "limit", maxImportRows)
			break
		}
	}

	slog.Info("talkgroups imported", "system_id", systemID, "count", imported)
	c.JSON(http.StatusOK, gin.H{"imported": imported})
}

// ImportUnits handles POST /api/admin/import/units.
// Accepts a multipart CSV file and a system_id form field.
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

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	imported := 0
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
		if imported == 0 && len(col0) > 0 && !unicode.IsDigit(rune(col0[0])) {
			continue
		}

		unitID, err := strconv.ParseInt(col0, 10, 64)
		if err != nil {
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
		imported++
		if imported >= maxImportRows {
			slog.Warn("CSV import row limit reached", "limit", maxImportRows)
			break
		}
	}

	slog.Info("units imported", "system_id", systemID, "count", imported)
	c.JSON(http.StatusOK, gin.H{"imported": imported})
}
