// Package health provides the GET /api/health endpoint.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves the health-check endpoint.
type Handler struct {
	version string
}

// New constructs a Handler.
func New(version string) *Handler {
	return &Handler{version: version}
}

// Get godoc
//
//	@Summary		Health check
//	@Description	Returns server status and version for readiness probes and Docker HEALTHCHECK.
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	object{status=string,version=string}	"Server is healthy"
//	@Router			/health [get]
func (h *Handler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": h.version,
	})
}
