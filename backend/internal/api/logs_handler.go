package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/logging"
)

const maxLogResults = 10_000

// GetLogs handles GET /api/admin/logs?from=&to=&level=&q=&limit=.
// Reads from the in-memory ring buffer — no database queries.
//
// @Summary      List logs
// @Description  Returns log entries from the in-memory ring buffer, filtered by date range, level, and text search.
// @Tags         Admin
// @Produce      json
// @Param        from   query  int     false  "Start unix timestamp"
// @Param        to     query  int     false  "End unix timestamp (defaults to now)"
// @Param        level  query  string  false  "Log level filter (debug, info, warn, error)"
// @Param        q      query  string  false  "Substring match on message or attributes"
// @Param        limit  query  int     false  "Maximum rows to return (1-10000, default 500)"
// @Success      200  {array}   logEntryResponse
// @Failure      400  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/logs [get]
func (h *AdminHandler) GetLogs(c *gin.Context) {
	var from int64
	if v := c.Query("from"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' parameter"})
			return
		}
		from = parsed
	}

	var to int64
	if v := c.Query("to"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' parameter"})
			return
		}
		to = parsed
	}

	level := strings.ToLower(strings.TrimSpace(c.Query("level")))
	query := strings.TrimSpace(c.Query("q"))

	limit := 500
	if v := c.Query("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'limit' parameter"})
			return
		}
		if parsed < 1 || parsed > maxLogResults {
			c.JSON(http.StatusBadRequest, gin.H{"error": "'limit' must be between 1 and 10000"})
			return
		}
		limit = parsed
	}

	entries := logging.QueryEntries(level, from, to, query, limit)

	// Convert to response format.
	resp := make([]logEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = logEntryResponse{
			DateTime: e.Time.Unix(),
			Level:    e.Level,
			Message:  e.Message,
			Attrs:    e.Attrs,
		}
	}

	if len(resp) >= limit {
		c.Header("X-Truncated", "true")
	}
	c.JSON(http.StatusOK, resp)
}

// GetLogLevel handles GET /api/admin/logs/level — returns the current runtime log level.
//
// @Summary      Get current log level
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  object  "level: debug|info|warn|error"
// @Security     BearerAuth
// @Router       /admin/logs/level [get]
func (h *AdminHandler) GetLogLevel(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"level": logging.GetLevel()})
}

type logEntryResponse struct {
	DateTime int64             `json:"dateTime"`
	Level    string            `json:"level"`
	Message  string            `json:"message"`
	Attrs    map[string]string `json:"attrs,omitempty"`
} // @name LogEntryResponse
