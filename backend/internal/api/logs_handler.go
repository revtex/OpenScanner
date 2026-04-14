package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

const maxLogResults = 10_000

// GetLogs handles GET /api/admin/logs?from=&to=&level=.
//
// @Summary      List logs
// @Description  Returns log entries within an optional date range and level filter. Results are capped at 10,000 rows (X-Truncated header set when truncated).
// @Tags         Admin
// @Produce      json
// @Param        from   query  int     false  "Start unix timestamp"
// @Param        to     query  int     false  "End unix timestamp (defaults to now)"
// @Param        level  query  string  false  "Log level filter (e.g. INFO, WARN, ERROR)"
// @Success      200  {array}   logResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/logs [get]
func (h *AdminHandler) GetLogs(c *gin.Context) {
	ctx := c.Request.Context()

	var from int64
	if v := c.Query("from"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' parameter"})
			return
		}
		from = parsed
	}

	to := time.Now().Unix()
	if v := c.Query("to"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' parameter"})
			return
		}
		to = parsed
	}

	level := c.Query("level")

	var logs []db.Log
	var err error
	if level != "" {
		logs, err = h.queries.ListLogsByDateRangeAndLevel(ctx, db.ListLogsByDateRangeAndLevelParams{
			DateTime:   from,
			DateTime_2: to,
			Level:      level,
		})
	} else {
		logs, err = h.queries.ListLogsByDateRange(ctx, db.ListLogsByDateRangeParams{
			DateTime:   from,
			DateTime_2: to,
		})
	}
	if err != nil {
		slog.Error("failed to list logs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list logs"})
		return
	}
	if len(logs) > maxLogResults {
		logs = logs[:maxLogResults]
		c.Header("X-Truncated", "true")
	}
	c.JSON(http.StatusOK, toLogResponses(logs))
}
