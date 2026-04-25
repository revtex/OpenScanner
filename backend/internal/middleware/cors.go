package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS handles Cross-Origin Resource Sharing.
// In production the frontend is served from the same origin so cross-origin
// requests are rejected. The allowed origin is derived from the request's own
// Host header (same-origin only). Preflight requests are handled with 204.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// Default to same-origin: the Origin must match the Host.
		host := c.Request.Host
		// Build the expected origin from the request scheme + host.
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		expected := scheme + "://" + host
		allowed := origin == expected

		isLocalhost := func(h string) bool {
			h = strings.ToLower(h)
			return h == "localhost" || h == "127.0.0.1"
		}

		// Dev/local exception: allow localhost frontend origins on different ports
		// when backend is also running on localhost. Only active when Gin is in
		// debug mode; release builds enforce strict same-origin.
		if !allowed && gin.Mode() == gin.DebugMode {
			if u, err := url.Parse(origin); err == nil {
				if reqHost, reqErr := url.Parse(scheme + "://" + host); reqErr == nil {
					if isLocalhost(u.Hostname()) && isLocalhost(reqHost.Hostname()) {
						allowed = true
					}
				}
			}
		}

		if !allowed {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
		c.Header("Access-Control-Max-Age", "86400")
		c.Header("Vary", "Origin")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
