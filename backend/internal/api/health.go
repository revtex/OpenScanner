// Package api — health check endpoint for readiness probes and Docker HEALTHCHECK.
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterHealth registers GET /health on the given router group.
// Response: {"status": "ok", "version": "<version>"}
func RegisterHealth(rg *gin.RouterGroup, version string) {
	rg.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": version,
		})
	})
}
