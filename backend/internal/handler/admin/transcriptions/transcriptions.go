// Package transcriptions provides the admin transcription status endpoint.
package transcriptions

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

// transcriptionStatusResponse is the JSON shape returned by GetStatus.
type transcriptionStatusResponse struct {
	Enabled             bool   `json:"enabled"`
	URL                 string `json:"url"`
	Model               string `json:"model"`
	Diarize             bool   `json:"diarize"`
	TotalTranscriptions int64  `json:"totalTranscriptions"`
	WhisperAvailable    bool   `json:"whisperAvailable"`
} // @name TranscriptionStatusResponse

// Handler serves the transcription status endpoint.
type Handler struct {
	queries          *db.Queries
	whisperAvailable bool
}

// New constructs a Handler.
func New(queries *db.Queries, whisperAvailable bool) *Handler {
	return &Handler{queries: queries, whisperAvailable: whisperAvailable}
}

// GetStatus handles GET /api/admin/transcriptions/status.
//
//	@Summary		Get transcription status
//	@Description	Returns transcription settings, total count, and whisper availability.
//	@Tags			Admin,v1-Admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	transcriptionStatusResponse
//	@Failure		500	{object}	shared.ErrorResponse
//	@Router			/admin/transcriptions/status [get]
//	@Router			/v1/admin/transcriptions/status [get]
func (h *Handler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()

	getSetting := func(key string) string {
		s, err := h.queries.GetSetting(ctx, key)
		if err != nil {
			return ""
		}
		return s.Value
	}

	count, err := h.queries.CountTranscriptions(ctx)
	if err != nil {
		slog.Error("failed to count transcriptions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, transcriptionStatusResponse{
		Enabled:             getSetting("transcriptionEnabled") == "true",
		URL:                 getSetting("transcriptionUrl"),
		Model:               getSetting("transcriptionModel"),
		Diarize:             getSetting("transcriptionDiarize") == "true",
		TotalTranscriptions: count,
		WhisperAvailable:    h.whisperAvailable,
	})
}
