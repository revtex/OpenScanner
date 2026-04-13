package api

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

type missingAudioCall struct {
	ID        int64  `json:"id"`
	DateTime  int64  `json:"dateTime"`
	AudioPath string `json:"audioPath"`
	AudioName string `json:"audioName"`
	Reason    string `json:"reason"`
}

type missingAudioResponse struct {
	RecordingsDir string             `json:"recordingsDir"`
	Limit         int64              `json:"limit"`
	Offset        int64              `json:"offset"`
	TotalCalls    int64              `json:"totalCalls"`
	Checked       int                `json:"checked"`
	Missing       []missingAudioCall `json:"missing"`
}

type cleanupMissingAudioRequest struct {
	Confirm bool    `json:"confirm"`
	CallIDs []int64 `json:"callIds"`
}

type cleanupMissingAudioResponse struct {
	Requested int                `json:"requested"`
	Deleted   int                `json:"deleted"`
	Skipped   []missingAudioCall `json:"skipped"`
}

func missingAudioReason(recordingsDir, audioPath string) string {
	relPath := filepath.Clean(audioPath)
	if relPath == "." || strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		return "invalid relative path"
	}
	fullPath := filepath.Join(recordingsDir, relPath)
	if rel, relErr := filepath.Rel(recordingsDir, fullPath); relErr != nil || strings.HasPrefix(rel, "..") {
		return "path escapes recordings directory"
	}
	if st, statErr := os.Stat(fullPath); statErr != nil || st.IsDir() {
		return "file not found"
	}
	return ""
}

// GetMissingAudioCalls handles GET /api/admin/tools/audio-missing.
// It checks archived calls in a page window and returns entries whose audio
// file does not exist under the configured recordings directory.
//
// @Summary      List calls with missing audio files
// @Description  Paginates through stored calls and returns those whose audio file is missing from disk.
// @Tags         Admin
// @Produce      json
// @Param        limit   query  int  false  "Page size (1–1000, default 200)"
// @Param        offset  query  int  false  "Page offset (default 0)"
// @Success      200  {object}  missingAudioResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/tools/audio-missing [get]
func (h *AdminHandler) GetMissingAudioCalls(c *gin.Context) {
	limit := int64(200)
	if v := c.Query("limit"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		if n > 1000 {
			n = 1000
		}
		limit = n
	}

	offset := int64(0)
	if v := c.Query("offset"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset"})
			return
		}
		offset = n
	}

	ctx := c.Request.Context()
	total, err := h.queries.CountCalls(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count calls"})
		return
	}

	calls, err := h.queries.ListCalls(ctx, db.ListCallsParams{
		SystemID:    nil,
		TalkgroupID: nil,
		DateFrom:    nil,
		DateTo:      nil,
		PageOffset:  sql.NullInt64{Int64: offset, Valid: true},
		PageSize:    sql.NullInt64{Int64: limit, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list calls"})
		return
	}

	missing := make([]missingAudioCall, 0)
	recordingsDir := h.recordingsDir
	if strings.TrimSpace(recordingsDir) == "" {
		recordingsDir = "."
	}

	for _, call := range calls {
		reason := missingAudioReason(recordingsDir, call.AudioPath)
		if reason != "" {
			missing = append(missing, missingAudioCall{
				ID:        call.ID,
				DateTime:  call.DateTime,
				AudioPath: call.AudioPath,
				AudioName: call.AudioName,
				Reason:    reason,
			})
		}
	}

	c.JSON(http.StatusOK, missingAudioResponse{
		RecordingsDir: recordingsDir,
		Limit:         limit,
		Offset:        offset,
		TotalCalls:    total,
		Checked:       len(calls),
		Missing:       missing,
	})
}

// CleanupMissingAudioCalls handles POST /api/admin/tools/audio-missing/cleanup.
// It deletes call rows only when the call is still missing on disk at delete time.
//
// @Summary      Clean up calls with missing audio
// @Description  Deletes call database rows for the given call IDs, but only if their audio file is still missing on disk at delete time. Requires confirm=true.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        body  body  cleanupMissingAudioRequest  true  "Call IDs to clean up"
// @Success      200  {object}  cleanupMissingAudioResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/tools/audio-missing/cleanup [post]
func (h *AdminHandler) CleanupMissingAudioCalls(c *gin.Context) {
	var req cleanupMissingAudioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if !req.Confirm {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirm must be true"})
		return
	}
	if len(req.CallIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "callIds is required"})
		return
	}
	if len(req.CallIDs) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many callIds"})
		return
	}

	ctx := c.Request.Context()
	recordingsDir := h.recordingsDir
	if strings.TrimSpace(recordingsDir) == "" {
		recordingsDir = "."
	}

	deleted := 0
	skipped := make([]missingAudioCall, 0)
	for _, callID := range req.CallIDs {
		call, err := h.queries.GetCall(ctx, callID)
		if err != nil {
			skipped = append(skipped, missingAudioCall{ID: callID, Reason: "call not found"})
			continue
		}
		reason := missingAudioReason(recordingsDir, call.AudioPath)
		if reason == "" {
			skipped = append(skipped, missingAudioCall{
				ID:        call.ID,
				DateTime:  call.DateTime,
				AudioPath: call.AudioPath,
				AudioName: call.AudioName,
				Reason:    "file now exists",
			})
			continue
		}
		if err := h.queries.DeleteCall(ctx, call.ID); err != nil {
			skipped = append(skipped, missingAudioCall{
				ID:        call.ID,
				DateTime:  call.DateTime,
				AudioPath: call.AudioPath,
				AudioName: call.AudioName,
				Reason:    "delete failed",
			})
			continue
		}
		deleted++
	}

	c.JSON(http.StatusOK, cleanupMissingAudioResponse{
		Requested: len(req.CallIDs),
		Deleted:   deleted,
		Skipped:   skipped,
	})
}
