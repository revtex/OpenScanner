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

// ToggleBookmarkRequest is the request body for POST /api/bookmarks.
type ToggleBookmarkRequest struct {
	CallID int64 `json:"callId" example:"42"`
} // @name ToggleBookmarkRequest

// ToggleBookmarkResponse is returned after toggling a bookmark.
type ToggleBookmarkResponse struct {
	Bookmarked bool  `json:"bookmarked" example:"true"`
	ID         int64 `json:"id,omitempty" example:"7"`
} // @name ToggleBookmarkResponse

// BookmarkIDsResponse is returned by GET /api/bookmarks.
type BookmarkIDsResponse struct {
	CallIDs []int64 `json:"callIds"`
} // @name BookmarkIDsResponse

// BookmarkCallsResponse is returned by GET /api/bookmarks/calls.
type BookmarkCallsResponse struct {
	Calls []CallSearchResult `json:"calls"`
} // @name BookmarkCallsResponse

// PostToggleBookmark handles POST /api/bookmarks — toggles a bookmark for the authenticated user.
//
//	@Summary		Toggle bookmark on a call
//	@Description	Creates a bookmark if one does not exist for the given call and user, or removes it if it already exists.
//	@Tags			Bookmarks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body	ToggleBookmarkRequest	true	"Call to bookmark"
//	@Success		200	{object}	ToggleBookmarkResponse
//	@Failure		400	{object}	ErrorResponse	"Invalid request body"
//	@Failure		401	{object}	ErrorResponse	"Authentication required"
//	@Failure		500	{object}	ErrorResponse	"Internal server error"
//	@Router			/bookmarks [post]
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

// GetBookmarkIDs handles GET /api/bookmarks — returns call IDs bookmarked by the authenticated user.
//
//	@Summary		List bookmarked call IDs
//	@Description	Returns an array of call IDs that the authenticated user has bookmarked.
//	@Tags			Bookmarks
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	BookmarkIDsResponse
//	@Failure		401	{object}	ErrorResponse	"Authentication required"
//	@Failure		500	{object}	ErrorResponse	"Internal server error"
//	@Router			/bookmarks [get]
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
//
//	@Summary		List bookmarked calls with metadata
//	@Description	Returns full call details for all calls bookmarked by the authenticated user.
//	@Tags			Bookmarks
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	BookmarkCallsResponse
//	@Failure		401	{object}	ErrorResponse	"Authentication required"
//	@Failure		500	{object}	ErrorResponse	"Internal server error"
//	@Router			/bookmarks/calls [get]
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
		}
		if row.SystemRadioID.Valid {
			r.SystemID = row.SystemRadioID.Int64
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
		if row.ErrorCount.Valid {
			r.ErrorCount = &row.ErrorCount.Int64
		}
		if row.SpikeCount.Valid {
			r.SpikeCount = &row.SpikeCount.Int64
		}
		if row.TalkgroupRadioID.Valid {
			r.TalkgroupID = row.TalkgroupRadioID.Int64
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
		if row.TranscriptText.Valid {
			r.Transcript = row.TranscriptText.String
		}
		r.Bookmarked = true
		results = append(results, r)
	}

	c.JSON(http.StatusOK, gin.H{"calls": results})
}
