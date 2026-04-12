// Package api contains Gin route handlers for OpenScanner.
package api

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/downstream"
	"github.com/openscanner/openscanner/internal/middleware"
	"github.com/openscanner/openscanner/internal/ws"
)

// DirwatchReloader is implemented by dirwatch.Service to trigger a config reload.
type DirwatchReloader interface {
	Reload()
}

// DownstreamReloader is implemented by downstream.Service to trigger a config reload.
type DownstreamReloader interface {
	Reload()
}

// DownstreamNotifier sends call events to downstream pushers.
type DownstreamNotifier interface {
	Notify(event downstream.CallEvent)
}

// Deps holds the dependencies required to register all API routes.
type Deps struct {
	Queries            *db.Queries
	RateLimiter        *auth.RateLimiter
	Processor          *audio.Processor
	Hub                *ws.Hub
	SQLDB              *sql.DB
	DirwatchReloader   DirwatchReloader
	DownstreamReloader DownstreamReloader
	DownstreamNotifier DownstreamNotifier
	Version            string
}

// RegisterRoutes wires all API routes onto the Gin engine.
func RegisterRoutes(r *gin.Engine, deps Deps) {
	setupHandler := NewSetupHandler(deps.Queries)
	authHandler := NewAuthHandler(deps.Queries, deps.RateLimiter)
	callHandler := NewCallHandler(deps.Queries, deps.Processor, deps.Hub, deps.DownstreamNotifier)
	adminHandler := NewAdminHandler(deps.Queries, deps.Hub, deps.SQLDB, deps.DirwatchReloader, deps.DownstreamReloader)

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

	// Call search — public access with optional auth for bookmarks.
	api.GET("/calls", middleware.OptionalJWTAuth(), callHandler.GetCalls)

	// Call upload — API key auth.
	upload := r.Group("/")
	upload.Use(middleware.APIKeyAuth(deps.Queries))
	{
		upload.POST("/api/call-upload", callHandler.PostCallUpload)
		upload.POST("/api/trunk-recorder-call-upload", callHandler.PostCallUpload)
	}

	// Admin CRUD — JWT + admin role required.
	admin := api.Group("/admin")
	admin.Use(middleware.JWTAuth(), middleware.RequireAdmin())
	{
		// Users
		admin.GET("/users", adminHandler.ListUsers)
		admin.POST("/users", adminHandler.CreateUser)
		admin.PUT("/users/:id", adminHandler.UpdateUser)
		admin.DELETE("/users/:id", adminHandler.DeleteUser)

		// Systems
		admin.GET("/systems", adminHandler.ListSystems)
		admin.POST("/systems", adminHandler.CreateSystem)
		admin.PUT("/systems/reorder", adminHandler.ReorderSystems)
		admin.PUT("/systems/:id", adminHandler.UpdateSystem)
		admin.DELETE("/systems/:id", adminHandler.DeleteSystem)

		// Talkgroups
		admin.GET("/talkgroups", adminHandler.ListTalkgroups)
		admin.POST("/talkgroups", adminHandler.CreateTalkgroup)
		admin.PUT("/talkgroups/:id", adminHandler.UpdateTalkgroup)
		admin.DELETE("/talkgroups/:id", adminHandler.DeleteTalkgroup)

		// Units
		admin.GET("/units", adminHandler.ListUnits)
		admin.POST("/units", adminHandler.CreateUnit)
		admin.PUT("/units/:id", adminHandler.UpdateUnit)
		admin.DELETE("/units/:id", adminHandler.DeleteUnit)

		// Groups
		admin.GET("/groups", adminHandler.ListGroups)
		admin.POST("/groups", adminHandler.CreateGroup)
		admin.PUT("/groups/:id", adminHandler.UpdateGroup)
		admin.DELETE("/groups/:id", adminHandler.DeleteGroup)

		// Tags
		admin.GET("/tags", adminHandler.ListTags)
		admin.POST("/tags", adminHandler.CreateTag)
		admin.PUT("/tags/:id", adminHandler.UpdateTag)
		admin.DELETE("/tags/:id", adminHandler.DeleteTag)

		// API Keys
		admin.GET("/apikeys", adminHandler.ListAPIKeys)
		admin.POST("/apikeys", adminHandler.CreateAPIKey)
		admin.PUT("/apikeys/:id", adminHandler.UpdateAPIKey)
		admin.DELETE("/apikeys/:id", adminHandler.DeleteAPIKey)

		// Dirwatches
		admin.GET("/dirwatches", adminHandler.ListDirwatches)
		admin.POST("/dirwatches", adminHandler.CreateDirwatch)
		admin.PUT("/dirwatches/:id", adminHandler.UpdateDirwatch)
		admin.DELETE("/dirwatches/:id", adminHandler.DeleteDirwatch)

		// Downstreams
		admin.GET("/downstreams", adminHandler.ListDownstreams)
		admin.POST("/downstreams", adminHandler.CreateDownstream)
		admin.PUT("/downstreams/:id", adminHandler.UpdateDownstream)
		admin.DELETE("/downstreams/:id", adminHandler.DeleteDownstream)

		// Webhooks
		admin.GET("/webhooks", adminHandler.ListWebhooks)
		admin.POST("/webhooks", adminHandler.CreateWebhook)
		admin.PUT("/webhooks/:id", adminHandler.UpdateWebhook)
		admin.DELETE("/webhooks/:id", adminHandler.DeleteWebhook)

		// Config
		admin.GET("/config", adminHandler.GetConfig)
		admin.PUT("/config", adminHandler.PutConfig)

		// Import / Export
		admin.POST("/import/talkgroups", adminHandler.ImportTalkgroups)
		admin.POST("/import/units", adminHandler.ImportUnits)
		admin.GET("/export/config", adminHandler.ExportConfig)
		admin.POST("/import/config", adminHandler.ImportConfig)

		// Logs
		admin.GET("/logs", adminHandler.GetLogs)
	}

	// WebSocket endpoints.
	r.GET("/ws", gin.WrapF(ws.HandleListenerWS(deps.Hub, deps.Queries)))
	r.GET("/api/admin/ws", gin.WrapF(ws.HandleAdminWS(deps.Hub, deps.Queries)))

	// Serve frontend (Phase 12 — placeholder)
	// r.NoRoute(func(c *gin.Context) { c.File("...") })
}
