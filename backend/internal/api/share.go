package api

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ShareResponse is the JSON payload for a shared call.
type ShareResponse struct {
	ID             int64  `json:"id"`
	DateTime       int64  `json:"dateTime"`
	SystemLabel    string `json:"systemLabel"`
	TalkgroupLabel string `json:"talkgroupLabel"`
	TalkgroupName  string `json:"talkgroupName"`
	Frequency      int64  `json:"frequency"`
	Duration       int64  `json:"duration"`
	Source         int64  `json:"source"`
	Transcript     string `json:"transcript,omitempty"`
	AudioURL       string `json:"audioUrl"`
}

// GetCallShare handles GET /api/calls/:id/share.
// Returns call metadata as JSON for public viewing when shareableLinks is enabled.
func (h *CallHandler) GetCallShare(c *gin.Context) {
	ctx := c.Request.Context()

	// Check if shareable links are enabled.
	if h.getSettingValue(c, "shareableLinks") != "true" {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid call id"})
		return
	}

	call, err := h.queries.GetCall(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		slog.Error("failed to get call for share", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	resp := ShareResponse{
		ID:             call.ID,
		DateTime:       call.DateTime,
		SystemLabel:    call.SystemLabel.String,
		TalkgroupLabel: call.TalkgroupLabel.String,
		TalkgroupName:  call.TalkgroupName.String,
		Frequency:      call.Frequency.Int64,
		Duration:       call.Duration.Int64,
		Source:         call.Source.Int64,
		AudioURL:       fmt.Sprintf("/api/calls/%d/audio", call.ID),
	}

	// Fetch transcript if available.
	trx, err := h.queries.GetTranscriptionByCallID(ctx, id)
	if err == nil {
		resp.Transcript = trx.Text
	}

	c.JSON(http.StatusOK, resp)
}
