package api

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// SharedLinkListItem is a single row in the admin shared-links listing.
type SharedLinkListItem struct {
	ID             int64  `json:"id"`
	CallID         int64  `json:"callId"`
	Token          string `json:"token"`
	CreatedAt      int64  `json:"createdAt"`
	SharedBy       string `json:"sharedBy"`
	DateTime       int64  `json:"dateTime"`
	Duration       int64  `json:"duration"`
	SystemLabel    string `json:"systemLabel"`
	TalkgroupLabel string `json:"talkgroupLabel"`
	TalkgroupName  string `json:"talkgroupName"`
} // @name SharedLinkListItem

// GetSharedLinks handles GET /api/admin/shared-links.
//
// @Summary      List shared links
// @Description  Returns all shared links with associated call metadata.
// @Tags         Admin
// @Produce      json
// @Success      200  {array}   SharedLinkListItem
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/shared-links [get]
func (h *AdminHandler) GetSharedLinks(c *gin.Context) {
	rows, err := h.queries.ListSharedLinks(c.Request.Context())
	if err != nil {
		slog.Error("failed to list shared links", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	items := make([]SharedLinkListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, SharedLinkListItem{
			ID:             r.ID,
			CallID:         r.CallID,
			Token:          r.Token,
			CreatedAt:      r.CreatedAt,
			SharedBy:       r.SharedBy,
			DateTime:       r.DateTime,
			Duration:       r.Duration.Int64,
			SystemLabel:    r.SystemLabel.String,
			TalkgroupLabel: r.TalkgroupLabel.String,
			TalkgroupName:  r.TalkgroupName.String,
		})
	}

	c.JSON(http.StatusOK, items)
}

// DeleteSharedLinkAdmin handles DELETE /api/admin/shared-links/:id.
//
// @Summary      Delete a shared link
// @Description  Removes a shared link by its ID.
// @Tags         Admin
// @Produce      json
// @Param        id  path  int  true  "Shared link ID"
// @Success      200  {object}  object  "deleted: true"
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/shared-links/{id} [delete]
func (h *AdminHandler) DeleteSharedLinkAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// Verify it exists first.
	if _, err := h.queries.ListSharedLinks(c.Request.Context()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
	}

	if err := h.queries.DeleteSharedLink(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete shared link", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
