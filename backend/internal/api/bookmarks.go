package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

type BookmarkHandler struct {
	queries *db.Queries
}

func (h *BookmarkHandler) PostToggleBookmark(c *gin.Context) {
	var req struct {
		CallID int64 `json:"callId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(int64)

	nullUserID := sql.NullInt64{Int64: userID, Valid: true}

	_, err := h.queries.GetBookmarkByCallAndUser(c.Request.Context(), db.GetBookmarkByCallAndUserParams{
		CallID: req.CallID,
		UserID: nullUserID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check bookmark"})
			return
		}
		// Bookmark doesn't exist — create it
		id, err := h.queries.CreateBookmark(c.Request.Context(), db.CreateBookmarkParams{
			CallID:    req.CallID,
			UserID:    nullUserID,
			SessionID: sql.NullString{},
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create bookmark"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"bookmarked": true, "id": id})
		return
	}

	// Bookmark exists — delete it
	if err := h.queries.DeleteBookmarkByCallAndUser(c.Request.Context(), db.DeleteBookmarkByCallAndUserParams{
		CallID: req.CallID,
		UserID: nullUserID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete bookmark"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bookmarked": false})
}

func (h *BookmarkHandler) GetBookmarkIDs(c *gin.Context) {
	uid, _ := c.Get("userID")
	userID := uid.(int64)

	callIDs, err := h.queries.ListBookmarkCallIDsByUser(c.Request.Context(), sql.NullInt64{Int64: userID, Valid: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list bookmarks"})
		return
	}
	if callIDs == nil {
		callIDs = []int64{}
	}
	c.JSON(http.StatusOK, gin.H{"callIds": callIDs})
}

// GetBookmarkCalls handles GET /api/bookmarks/calls — returns bookmarked calls with full metadata.
func (h *BookmarkHandler) GetBookmarkCalls(c *gin.Context) {
	uid, _ := c.Get("userID")
	userID := uid.(int64)

	rows, err := h.queries.ListBookmarkCallsByUser(c.Request.Context(), sql.NullInt64{Int64: userID, Valid: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list bookmarked calls"})
		return
	}

	results := make([]CallSearchResult, 0, len(rows))
	for _, row := range rows {
		r := CallSearchResult{
			ID:        row.ID,
			AudioName: row.AudioName,
			AudioType: row.AudioType,
			DateTime:  row.DateTime,
			SystemID:  row.SystemID,
		}
		if row.Frequency.Valid {
			r.Frequency = &row.Frequency.Int64
		}
		if row.Duration.Valid {
			r.Duration = &row.Duration.Int64
		}
		if row.Source.Valid {
			r.Source = &row.Source.Int64
		}
		if row.TalkgroupID.Valid {
			r.TalkgroupID = row.TalkgroupID.Int64
		}
		r.SystemLabel = row.SystemLabel.String
		r.TalkgroupLabel = row.TalkgroupLabel.String
		r.TalkgroupName = row.TalkgroupName.String
		r.TalkgroupLed = row.TalkgroupLed.String
		if row.Site.Valid {
			r.Site = row.Site.String
		}
		if row.Channel.Valid {
			r.Channel = row.Channel.String
		}
		if row.Decoder.Valid {
			r.Decoder = row.Decoder.String
		}
		r.Bookmarked = true
		results = append(results, r)
	}

	c.JSON(http.StatusOK, gin.H{"calls": results})
}
