package api

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/ws"
)

const maxImportRows = 100_000 // CSV import safety limit.

// AdminHandler handles admin CRUD endpoints.
type AdminHandler struct {
	queries          *db.Queries
	hub              *ws.Hub
	sqlDB            *sql.DB
	dwReload         DirMonitorReloader
	dsReload         DownstreamReloader
	recordingsDir    string
	ffmpegAvailable  bool
	fdkAACAvailable  bool
	whisperAvailable bool
}

// transcriptionStatusResponse is the JSON shape returned by GetTranscriptionStatus.
type transcriptionStatusResponse struct {
	Enabled             bool   `json:"enabled"`
	URL                 string `json:"url"`
	Model               string `json:"model"`
	Diarize             bool   `json:"diarize"`
	TotalTranscriptions int64  `json:"totalTranscriptions"`
	WhisperAvailable    bool   `json:"whisperAvailable"`
}

// GetTranscriptionStatus handles GET /api/admin/transcriptions/status.
// Returns the current transcription configuration and statistics.
//
//	@Summary		Get transcription status
//	@Description	Returns transcription settings, total count, and whisper availability.
//	@Tags			Admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	transcriptionStatusResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/admin/transcriptions/status [get]
func (h *AdminHandler) GetTranscriptionStatus(c *gin.Context) {
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

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(queries *db.Queries, hub *ws.Hub, sqlDB *sql.DB, dwReload DirMonitorReloader, dsReload DownstreamReloader, recordingsDir ...string) *AdminHandler {
	rd := "."
	if len(recordingsDir) > 0 && strings.TrimSpace(recordingsDir[0]) != "" {
		rd = recordingsDir[0]
	}
	return &AdminHandler{queries: queries, hub: hub, sqlDB: sqlDB, dwReload: dwReload, dsReload: dsReload, recordingsDir: rd}
}
