package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

var startTime = time.Now()

// ActivityStatsResponse is the JSON payload for GET /api/admin/activity/stats.
type ActivityStatsResponse struct {
	CallsToday      int64 `json:"callsToday"`
	CallsThisWeek   int64 `json:"callsThisWeek"`
	CallsTotal      int64 `json:"callsTotal"`
	ActiveListeners int   `json:"activeListeners"`
	Uptime          int64 `json:"uptime"`
} // @name ActivityStatsResponse

// GetActivityStats handles GET /api/admin/activity/stats.
//
// @Summary      Get activity statistics
// @Description  Returns call counts (today, this week, total), active listeners, and server uptime.
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  ActivityStatsResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/activity/stats [get]
func (h *AdminHandler) GetActivityStats(c *gin.Context) {
	ctx := c.Request.Context()
	requestID, _ := c.Get("requestID")
	now := time.Now()

	// Start of today (midnight local time).
	y, m, d := now.Date()
	todayStart := time.Date(y, m, d, 0, 0, 0, 0, now.Location()).Unix()

	// Start of this week (Monday midnight local time).
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	weekStart := time.Date(y, m, d-int(weekday-time.Monday), 0, 0, 0, 0, now.Location()).Unix()

	stats, err := h.queries.GetActivityStats(ctx, db.GetActivityStatsParams{
		TodayStart: todayStart,
		WeekStart:  weekStart,
	})
	if err != nil {
		slog.Error("failed to get activity stats", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, ActivityStatsResponse{
		CallsToday:      stats.CallsToday,
		CallsThisWeek:   stats.CallsThisWeek,
		CallsTotal:      stats.CallsTotal,
		ActiveListeners: h.hub.ClientCount(),
		Uptime:          int64(time.Since(startTime).Seconds()),
	})
}

// ChartBucket is a single hour bucket in the activity chart.
type ChartBucket struct {
	Hour  int64 `json:"hour"`
	Count int64 `json:"count"`
} // @name ChartBucket

// ActivityChartResponse is the JSON payload for GET /api/admin/activity/chart.
type ActivityChartResponse struct {
	Buckets []ChartBucket `json:"buckets"`
} // @name ActivityChartResponse

// GetActivityChart handles GET /api/admin/activity/chart.
//
// @Summary      Get activity chart data
// @Description  Returns call counts bucketed by hour for the last 24 hours.
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  ActivityChartResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/activity/chart [get]
func (h *AdminHandler) GetActivityChart(c *gin.Context) {
	ctx := c.Request.Context()
	requestID, _ := c.Get("requestID")
	cutoff := time.Now().Add(-24 * time.Hour).Unix()

	rows, err := h.queries.GetCallsPerHour(ctx, cutoff)
	if err != nil {
		slog.Error("failed to get calls per hour", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	buckets := make([]ChartBucket, len(rows))
	for i, r := range rows {
		buckets[i] = ChartBucket{Hour: r.HourBucket, Count: r.CallCount}
	}

	c.JSON(http.StatusOK, ActivityChartResponse{Buckets: buckets})
}

// TopTalkgroup is a single entry in the top-talkgroups response.
type TopTalkgroup struct {
	TalkgroupID    int64  `json:"talkgroupId"`
	TalkgroupLabel string `json:"talkgroupLabel"`
	TalkgroupName  string `json:"talkgroupName"`
	SystemLabel    string `json:"systemLabel"`
	CallCount      int64  `json:"callCount"`
} // @name TopTalkgroup

// TopTalkgroupsResponse is the JSON payload for GET /api/admin/activity/top-talkgroups.
type TopTalkgroupsResponse struct {
	Talkgroups []TopTalkgroup `json:"talkgroups"`
} // @name TopTalkgroupsResponse

// GetTopTalkgroups handles GET /api/admin/activity/top-talkgroups.
//
// @Summary      Get top talkgroups
// @Description  Returns the top 10 talkgroups by call count over the last 24 hours.
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  TopTalkgroupsResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/activity/top-talkgroups [get]
func (h *AdminHandler) GetTopTalkgroups(c *gin.Context) {
	ctx := c.Request.Context()
	requestID, _ := c.Get("requestID")
	cutoff := time.Now().Add(-24 * time.Hour).Unix()

	rows, err := h.queries.GetTopTalkgroups(ctx, db.GetTopTalkgroupsParams{
		DateTime: cutoff,
		Limit:    10,
	})
	if err != nil {
		slog.Error("failed to get top talkgroups", "request_id", requestID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	tgs := make([]TopTalkgroup, len(rows))
	for i, r := range rows {
		tgs[i] = TopTalkgroup{
			TalkgroupID:    r.TalkgroupID.Int64,
			TalkgroupLabel: r.TalkgroupLabel.String,
			TalkgroupName:  r.TalkgroupName.String,
			SystemLabel:    r.SystemLabel.String,
			CallCount:      r.CallCount,
		}
	}

	c.JSON(http.StatusOK, TopTalkgroupsResponse{Talkgroups: tgs})
}
