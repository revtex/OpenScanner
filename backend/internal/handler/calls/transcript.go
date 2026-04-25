package calls

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/handler/shared"
)

// transcriptResponse is the JSON shape returned by GetCallTranscript.
type transcriptResponse struct {
	Text     string                       `json:"text"`
	Segments []audio.TranscriptionSegment `json:"segments"`
	Language string                       `json:"language"`
	Model    string                       `json:"model"`
} // @name TranscriptResponse

// GetCallTranscript handles GET /api/calls/:id/transcript.
// Returns the transcription for a call if one exists.
//
//	@Summary		Get call transcript
//	@Description	Returns the transcription text, segments, language and model for a call. Authentication is optional when the publicAccess setting is enabled; otherwise a valid JWT is required.
//	@Tags			Calls
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Call ID"
//	@Success		200	{object}	transcriptResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/calls/{id}/transcript [get]
func (h *Handler) GetCallTranscript(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid call id"})
		return
	}

	// Require authentication or publicAccess.
	_, hasUser := c.Get("userID")
	if !hasUser && shared.GetSettingValue(c, h.queries, "publicAccess") != "true" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	trx, err := h.queries.GetTranscriptionByCallID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "transcript not found"})
			return
		}
		slog.Error("failed to get transcript", "call_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var segments []audio.TranscriptionSegment
	if trx.Segments.Valid && trx.Segments.String != "" {
		if err := json.Unmarshal([]byte(trx.Segments.String), &segments); err != nil {
			slog.Warn("failed to parse transcript segments", "call_id", id, "error", err)
		}
	}
	if segments == nil {
		segments = []audio.TranscriptionSegment{}
	}

	c.JSON(http.StatusOK, transcriptResponse{
		Text:     trx.Text,
		Segments: segments,
		Language: trx.Language.String,
		Model:    trx.Model.String,
	})
}
