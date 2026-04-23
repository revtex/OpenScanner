// Package api — health check endpoint for readiness probes and Docker HEALTHCHECK.
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterHealth godoc
//
//	@Summary		Health check
//	@Description	Returns server status and version for readiness probes and Docker HEALTHCHECK.
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	object{status=string,version=string}	"Server is healthy"
//	@Router			/health [get]
func RegisterHealth(rg *gin.RouterGroup, version string) {
	rg.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": version,
		})
	})
}
