// Package api contains Gin route handlers for OpenScanner.
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/middleware"
	"github.com/openscanner/openscanner/internal/ws"
)

// Deps holds the dependencies required to register all API routes.
type Deps struct {
	Queries     *db.Queries
	RateLimiter *auth.RateLimiter
	Processor   *audio.Processor
	Hub         *ws.Hub
	Version     string
}

// RegisterRoutes wires all API routes onto the Gin engine.
func RegisterRoutes(r *gin.Engine, deps Deps) {
	setupHandler := NewSetupHandler(deps.Queries)
	authHandler := NewAuthHandler(deps.Queries, deps.RateLimiter)
	callHandler := NewCallHandler(deps.Queries, deps.Processor, deps.Hub)

	// Global middleware applied to every request.
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())

	api := r.Group("/api")

	// Health check — unauthenticated.
	RegisterHealth(api, deps.Version)

	// First-run setup — unauthenticated.
	api.GET("/setup/status", setupHandler.GetSetupStatus)
	api.POST("/setup", setupHandler.PostSetup)

	// Auth — login is unauthenticated; the rest require a valid JWT.
	api.POST("/auth/login", middleware.RateLimit(deps.RateLimiter), authHandler.PostLogin)

	authRequired := api.Group("/auth")
	authRequired.Use(middleware.JWTAuth())
	{
		authRequired.POST("/logout", authHandler.PostLogout)
		authRequired.PUT("/password", authHandler.PutPassword)
		authRequired.GET("/me", authHandler.GetMe)
	}

	// Call upload — API key auth.
	upload := r.Group("/")
	upload.Use(middleware.APIKeyAuth(deps.Queries))
	{
		upload.POST("/api/call-upload", callHandler.PostCallUpload)
		upload.POST("/api/trunk-recorder-call-upload", callHandler.PostCallUpload)
	}

	// WebSocket endpoints.
	r.GET("/ws", gin.WrapF(ws.HandleListenerWS(deps.Hub, deps.Queries)))
	r.GET("/api/admin/ws", gin.WrapF(ws.HandleAdminWS(deps.Hub, deps.Queries)))

	// Serve frontend (Phase 12 — placeholder)
	// r.NoRoute(func(c *gin.Context) { c.File("...") })
}
