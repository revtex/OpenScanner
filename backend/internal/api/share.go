package api

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openscanner/openscanner/internal/db"
)

// ShareResponse is the JSON payload for a shared call viewed via token.
type ShareResponse struct {
	Token          string `json:"token"`
	DateTime       int64  `json:"dateTime"`
	SystemLabel    string `json:"systemLabel"`
	TalkgroupLabel string `json:"talkgroupLabel"`
	TalkgroupName  string `json:"talkgroupName"`
	Frequency      int64  `json:"frequency"`
	Duration       int64  `json:"duration"`
	Source         int64  `json:"source"`
	Transcript     string `json:"transcript,omitempty"`
	AudioURL       string `json:"audioUrl"`
} // @name ShareResponse

// ShareCreateResponse is returned when a call is shared.
type ShareCreateResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
} // @name ShareCreateResponse

// PostShareCall handles POST /api/calls/:id/share.
// Creates a shared_links record for the call and returns the token + URL.
//
// @Summary      Share a call
// @Description  Creates a shared_links record for the call and returns the token + URL.
// @Tags         Sharing
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Call ID"
// @Success      201  {object}  ShareCreateResponse
// @Success      200  {object}  ShareCreateResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /calls/{id}/share [post]
func (h *CallHandler) PostShareCall(c *gin.Context) {
	ctx := c.Request.Context()

	if h.getSettingValue(c, "shareableLinks") != "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "sharing is disabled"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid call id"})
		return
	}

	userIDVal, _ := c.Get("userID")
	userID, _ := userIDVal.(int64)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	// Check if already shared — return existing token.
	existing, err := h.queries.GetSharedLinkByCallID(ctx, id)
	if err == nil {
		c.JSON(http.StatusOK, ShareCreateResponse{
			Token: existing.Token,
			URL:   fmt.Sprintf("/call/%s", existing.Token),
		})
		return
	}

	// Verify call exists.
	if _, err := h.queries.GetCall(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "call not found"})
			return
		}
		slog.Error("failed to get call for sharing", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	token := uuid.New().String()
	sl, err := h.queries.CreateSharedLink(ctx, db.CreateSharedLinkParams{
		CallID: id,
		UserID: userID,
		Token:  token,
	})
	if err != nil {
		slog.Error("failed to create shared link", "call_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, ShareCreateResponse{
		Token: sl.Token,
		URL:   fmt.Sprintf("/call/%s", sl.Token),
	})
}

// DeleteShareCall handles DELETE /api/calls/:id/share.
// Removes the shared_links record. Only the original sharer or an admin can unshare.
//
// @Summary      Unshare a call
// @Description  Removes the shared_links record. Only the original sharer or an admin can unshare.
// @Tags         Sharing
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Call ID"
// @Success      200  {object}  object{shared=bool}
// @Failure      400  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /calls/{id}/share [delete]
func (h *CallHandler) DeleteShareCall(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid call id"})
		return
	}

	userIDVal, _ := c.Get("userID")
	userID, _ := userIDVal.(int64)
	role, _ := c.Get("role")

	sl, err := h.queries.GetSharedLinkByCallID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not shared"})
			return
		}
		slog.Error("failed to get shared link", "call_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if sl.UserID != userID && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	if err := h.queries.DeleteSharedLinkByCallID(ctx, id); err != nil {
		slog.Error("failed to delete shared link", "call_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"shared": false})
}

// GetSharedCallByToken handles GET /api/shared/:token.
// Returns call metadata as JSON for public viewing. No authentication required.
//
// @Summary      Get shared call by token
// @Description  Returns call metadata as JSON for public viewing. No authentication required.
// @Tags         Sharing
// @Produce      json
// @Param        token  path      string  true  "Share token"
// @Failure      400    {object}  ErrorResponse
// @Success      200    {object}  ShareResponse
// @Failure      404    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Router       /shared/{token} [get]
func (h *CallHandler) GetSharedCallByToken(c *gin.Context) {
	ctx := c.Request.Context()
	token := c.Param("token")

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}

	sl, err := h.queries.GetSharedLinkByToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		slog.Error("failed to get shared link by token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	resp := ShareResponse{
		Token:          sl.Token,
		DateTime:       sl.DateTime,
		SystemLabel:    sl.SystemLabel.String,
		TalkgroupLabel: sl.TalkgroupLabel.String,
		TalkgroupName:  sl.TalkgroupName.String,
		Frequency:      sl.Frequency.Int64,
		Duration:       sl.Duration.Int64,
		Source:         sl.Source.Int64,
		AudioURL:       fmt.Sprintf("/api/shared/%s/audio", sl.Token),
	}

	// Fetch transcript if available.
	trx, err := h.queries.GetTranscriptionByCallID(ctx, sl.CallID)
	if err == nil {
		resp.Transcript = trx.Text
	}

	c.JSON(http.StatusOK, resp)
}

// GetSharedCallAudio handles GET /api/shared/:token/audio.
// Serves the audio file for a shared call. No authentication required.
//
// @Summary      Get shared call audio
// @Description  Serves the audio file for a shared call. No authentication required.
// @Tags         Sharing
// @Produce      application/octet-stream
// @Param        token  path      string  true  "Share token"
// @Failure      400    {object}  ErrorResponse
// @Success      200    {file}    binary
// @Failure      404    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Router       /shared/{token}/audio [get]
func (h *CallHandler) GetSharedCallAudio(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}

	sl, err := h.queries.GetSharedLinkByToken(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		slog.Error("failed to get shared link for audio", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	recordingsDir := h.processor.RecordingsDir()
	relPath := filepath.Clean(sl.AudioPath)
	if strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		slog.Warn("rejected unsafe audio path", "token", token, "path", sl.AudioPath)
		c.JSON(http.StatusNotFound, gin.H{"error": "audio not found"})
		return
	}

	fullPath := filepath.Join(recordingsDir, relPath)
	if rel, relErr := filepath.Rel(recordingsDir, fullPath); relErr != nil || strings.HasPrefix(rel, "..") {
		slog.Warn("audio path escaped recordings dir", "token", token, "path", sl.AudioPath)
		c.JSON(http.StatusNotFound, gin.H{"error": "audio not found"})
		return
	}

	if _, err := os.Stat(fullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "audio file not found"})
			return
		}
		slog.Error("failed to stat shared call audio", "token", token, "path", fullPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	contentType := sl.AudioType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := sl.AudioName
	if filename == "" {
		filename = filepath.Base(sl.AudioPath)
	}

	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
	c.Header("Content-Type", contentType)
	c.File(fullPath)
}

// GetCallShare handles GET /api/calls/:id/share (legacy compatibility).
// Returns the share token for a call if it exists, for authenticated users.
//
// @Summary      Get call share status
// @Description  Returns the share token for a call if it exists, for authenticated users.
// @Tags         Sharing
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Call ID"
// @Success      200  {object}  ShareCreateResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /calls/{id}/share [get]
func (h *CallHandler) GetCallShare(c *gin.Context) {
	ctx := c.Request.Context()

	if h.getSettingValue(c, "shareableLinks") != "true" {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid call id"})
		return
	}

	sl, err := h.queries.GetSharedLinkByCallID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not shared"})
			return
		}
		slog.Error("failed to get shared link", "call_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": sl.Token, "shared": true})
}
