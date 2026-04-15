// Package api contains Gin route handlers for OpenScanner.
package api

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/openscanner/openscanner/docs" // swagger generated docs

	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/downstream"
	"github.com/openscanner/openscanner/internal/middleware"
	"github.com/openscanner/openscanner/internal/static"
	"github.com/openscanner/openscanner/internal/ws"
)

// DirMonitorReloader is implemented by dirmonitor.Service to trigger a config reload.
type DirMonitorReloader interface {
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
	DirMonitorReloader DirMonitorReloader
	DownstreamReloader DownstreamReloader
	DownstreamNotifier DownstreamNotifier
	Version            string
	FFmpegAvailable    bool
	WhisperAvailable   bool
}

// RegisterRoutes wires all API routes onto the Gin engine.
func RegisterRoutes(r *gin.Engine, deps Deps) {
	setupHandler := NewSetupHandler(deps.Queries)
	authHandler := NewAuthHandler(deps.Queries, deps.RateLimiter)
	callHandler := NewCallHandler(deps.Queries, deps.Processor, deps.Hub, deps.DownstreamNotifier)
	bookmarkHandler := &BookmarkHandler{queries: deps.Queries}
	recordingsDir := "."
	if deps.Processor != nil {
		recordingsDir = deps.Processor.RecordingsDir()
	}
	adminHandler := NewAdminHandler(deps.Queries, deps.Hub, deps.SQLDB, deps.DirMonitorReloader, deps.DownstreamReloader, recordingsDir)
	adminHandler.ffmpegAvailable = deps.FFmpegAvailable
	adminHandler.whisperAvailable = deps.WhisperAvailable

	// Global middleware applied to every request.
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())

	api := r.Group("/api")

	// Health check — unauthenticated.
	RegisterHealth(api, deps.Version)

	// First-run setup — unauthenticated.
	api.GET("/setup/status", setupHandler.GetSetupStatus)
	api.POST("/setup", setupHandler.PostSetup)

	// Auth — login is unauthenticated; the rest require a valid JWT.
	api.POST("/auth/login", middleware.MaxBodySize(1<<20), middleware.RateLimit(deps.RateLimiter), authHandler.PostLogin)

	authRequired := api.Group("/auth")
	authRequired.Use(middleware.JWTAuth())
	{
		authRequired.POST("/logout", authHandler.PostLogout)
		authRequired.PUT("/password", authHandler.PutPassword)
		authRequired.GET("/me", authHandler.GetMe)
	}

	// Call search — public access with optional auth for bookmarks.
	api.GET("/calls", middleware.OptionalJWTAuth(), callHandler.GetCalls)
	api.GET("/calls/:id/audio", middleware.OptionalJWTAuth(), callHandler.GetCallAudio)

	// Shared calls — token-based public access (no auth required).
	api.GET("/shared/:token", callHandler.GetSharedCallByToken)
	api.GET("/shared/:token/audio", callHandler.GetSharedCallAudio)

	// Share management — JWT required.
	api.POST("/calls/:id/share", middleware.JWTAuth(), callHandler.PostShareCall)
	api.DELETE("/calls/:id/share", middleware.JWTAuth(), callHandler.DeleteShareCall)
	api.GET("/calls/:id/share", middleware.JWTAuth(), callHandler.GetCallShare)

	// Bookmarks — JWT required.
	bookmarks := api.Group("/bookmarks")
	bookmarks.Use(middleware.JWTAuth())
	{
		bookmarks.GET("", bookmarkHandler.GetBookmarkIDs)
		bookmarks.GET("/calls", bookmarkHandler.GetBookmarkCalls)
		bookmarks.POST("", bookmarkHandler.PostToggleBookmark)
	}

	// Call upload — API key auth.
	upload := r.Group("/")
	upload.Use(middleware.APIKeyAuth(deps.Queries))
	{
		upload.POST("/api/call-upload", callHandler.PostCallUpload)
		upload.POST("/api/trunk-recorder-call-upload", callHandler.PostCallUpload)
	}

	// Admin CRUD — JWT + admin role required.
	admin := api.Group("/admin")
	admin.Use(middleware.JWTAuth(), middleware.RequireAdmin(), middleware.MaxBodySize(2<<20)) // 2 MiB JSON body limit
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
		admin.PUT("/apikeys/reorder", adminHandler.ReorderAPIKeys)
		admin.PUT("/apikeys/:id", adminHandler.UpdateAPIKey)
		admin.DELETE("/apikeys/:id", adminHandler.DeleteAPIKey)

		// DirMonitors
		admin.GET("/fs/directories", adminHandler.ListServerDirectories)
		admin.GET("/dirmonitors", adminHandler.ListDirMonitors)
		admin.POST("/dirmonitors", adminHandler.CreateDirMonitor)
		admin.PUT("/dirmonitors/:id", adminHandler.UpdateDirMonitor)
		admin.DELETE("/dirmonitors/:id", adminHandler.DeleteDirMonitor)

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
		admin.GET("/tools/audio-missing", adminHandler.GetMissingAudioCalls)
		admin.POST("/tools/audio-missing/cleanup", adminHandler.CleanupMissingAudioCalls)

		// Import / Export
		admin.POST("/import/talkgroups", adminHandler.ImportTalkgroups)
		admin.POST("/import/units", adminHandler.ImportUnits)
		admin.GET("/export/talkgroups", adminHandler.ExportTalkgroups)
		admin.GET("/export/units", adminHandler.ExportUnits)
		admin.GET("/export/config", adminHandler.ExportConfig)
		admin.POST("/import/config", adminHandler.ImportConfig)

		// RadioReference
		admin.POST("/radioreference/login", adminHandler.RadioReferenceLogin)
		admin.GET("/radioreference/countries", adminHandler.RadioReferenceCountries)
		admin.GET("/radioreference/states", adminHandler.RadioReferenceStates)
		admin.GET("/radioreference/counties", adminHandler.RadioReferenceCounties)
		admin.GET("/radioreference/systems", adminHandler.RadioReferenceSystems)
		admin.POST("/radioreference/preview/csv", adminHandler.RadioReferencePreviewCSV)
		admin.POST("/radioreference/preview/api", adminHandler.RadioReferencePreviewAPI)
		admin.POST("/radioreference/apply", adminHandler.RadioReferenceApply)

		// Logs
		admin.GET("/logs", adminHandler.GetLogs)

		// Activity
		admin.GET("/activity/stats", adminHandler.GetActivityStats)
		admin.GET("/activity/chart", adminHandler.GetActivityChart)
		admin.GET("/activity/top-talkgroups", adminHandler.GetTopTalkgroups)

		// Shared Links
		admin.GET("/shared-links", adminHandler.GetSharedLinks)
		admin.DELETE("/shared-links/:id", adminHandler.DeleteSharedLinkAdmin)

		// Swagger: issue a short-lived HTTP-only cookie so Swagger UI
		// can be opened in a new browser tab without exposing the JWT.
		admin.POST("/docs/session", func(c *gin.Context) {
			secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
			auth.SetSwaggerCookie(c, secure)
			c.JSON(200, gin.H{"ok": true})
		})
	}

	// Swagger API documentation — protected by the HTTP-only cookie
	// set via POST /api/admin/docs/session above.
	swaggerDocs := api.Group("/admin/docs")
	swaggerDocs.Use(middleware.SwaggerCookieAuth())
	{
		swaggerDocs.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// WebSocket endpoints.
	r.GET("/ws", gin.WrapF(ws.HandleListenerWS(deps.Hub, deps.Queries)))
	r.GET("/api/admin/ws", gin.WrapF(ws.HandleAdminWS(deps.Hub, deps.Queries)))

	// Serve embedded frontend (SPA mode).
	serveFrontend(r)
}

// serveFrontend serves the embedded frontend dist/ as a SPA.
// Non-API, non-WS routes serve static files; unmatched paths fall back to index.html.
func serveFrontend(r *gin.Engine) {
	distFS, err := fs.Sub(static.DistFS, "dist")
	if err != nil {
		slog.Warn("embedded frontend not available", "error", err)
		return
	}

	// Check if the embedded FS has an index.html (i.e. a real build was embedded).
	if _, err := fs.Stat(distFS, "index.html"); err != nil {
		slog.Warn("no embedded frontend found — run frontend build and rebuild backend to embed")
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Don't serve frontend for API or WS paths.
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Try to serve the exact file (JS, CSS, images, etc.).
		if f, err := distFS.Open(strings.TrimPrefix(path, "/")); err == nil {
			f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: serve index.html for client-side routing.
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
