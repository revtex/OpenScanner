// Package routes wires all OpenScanner HTTP and WebSocket routes onto a Gin engine.
//
// It owns the top-level route registration and middleware ordering, and delegates
// per-feature handling to the handler subpackages (auth, calls, bookmarks, share,
// setup, health, and admin/*).
package routes

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
	authhandler "github.com/openscanner/openscanner/internal/handler/auth"
	"github.com/openscanner/openscanner/internal/handler/admin/imports"
	"github.com/openscanner/openscanner/internal/handler/admin/radioreference"
	"github.com/openscanner/openscanner/internal/handler/admin/transcriptions"
	"github.com/openscanner/openscanner/internal/handler/bookmarks"
	"github.com/openscanner/openscanner/internal/handler/calls"
	"github.com/openscanner/openscanner/internal/handler/health"
	"github.com/openscanner/openscanner/internal/handler/setup"
	"github.com/openscanner/openscanner/internal/handler/share"
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
	Transcriber        audio.Transcriber // nil when transcription is disabled
	Version            string
	FFmpegAvailable    bool
	FDKAACAvailable    bool
	WhisperAvailable   bool
}

// RegisterRoutes wires all API routes onto the Gin engine.
func RegisterRoutes(r *gin.Engine, deps Deps) {
	healthHandler := health.New(deps.Version)
	setupHandler := setup.New(deps.Queries)
	// Avoid the typed-nil interface footgun: only promote deps.Hub into the
	// WSDisconnecter interface when the concrete pointer is non-nil. This
	// keeps tests that pass Deps{} (no Hub) from triggering nil-pointer
	// dereferences on logout / token revocation paths.
	var disconnecter authhandler.WSDisconnecter
	if deps.Hub != nil {
		disconnecter = deps.Hub
	}
	authH := authhandler.New(deps.Queries, deps.RateLimiter, disconnecter)
	callHandler := calls.New(deps.Queries, deps.Processor, deps.Hub, deps.DownstreamNotifier, deps.Transcriber)
	shareHandler := share.New(deps.Queries, deps.Processor)
	bookmarkHandler := bookmarks.New(deps.Queries)
	importsHandler := imports.New(deps.Queries, deps.Hub)
	rrHandler := radioreference.New(deps.Queries)
	transcriptionsHandler := transcriptions.New(deps.Queries, deps.WhisperAvailable)

	// Global middleware applied to every request.
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())

	api := r.Group("/api")

	// Health check — unauthenticated.
	api.GET("/health", healthHandler.Get)

	// First-run setup — unauthenticated.
	api.GET("/setup/status", setupHandler.GetSetupStatus)
	api.POST("/setup", middleware.MaxBodySize(1<<20), setupHandler.PostSetup)

	// Auth — login and refresh are unauthenticated; the rest require a valid JWT.
	api.POST("/auth/login", middleware.MaxBodySize(1<<20), middleware.RateLimit(deps.RateLimiter), authH.PostLogin)
	api.POST("/auth/refresh", middleware.MaxBodySize(1<<20), middleware.RateLimit(deps.RateLimiter), authH.PostRefresh)

	authRequired := api.Group("/auth")
	authRequired.Use(middleware.JWTAuth())
	{
		authRequired.POST("/logout", authH.PostLogout)
		authRequired.PUT("/password", authH.PutPassword)
		authRequired.GET("/me", authH.GetMe)
		authRequired.GET("/tg-selection", authH.GetTGSelection)
		authRequired.PUT("/tg-selection", authH.PutTGSelection)
	}

	// Call search — public access with optional auth for bookmarks.
	api.GET("/calls", middleware.OptionalJWTAuth(), callHandler.GetCalls)
	api.GET("/calls/:id/audio", middleware.OptionalJWTOrSessionAuth(), callHandler.GetCallAudio)
	api.GET("/calls/:id/transcript", middleware.OptionalJWTAuth(), callHandler.GetCallTranscript)

	// Shared calls — token-based public access (no auth required).
	// Rate-limited to 30 req/min per IP to prevent bandwidth exhaustion.
	sharedRateLimit := middleware.RateLimitByIP(30)
	api.GET("/shared/:token", sharedRateLimit, shareHandler.GetSharedCallByToken)
	api.GET("/shared/:token/audio", sharedRateLimit, shareHandler.GetSharedCallAudio)

	// Share management — JWT required.
	api.POST("/calls/:id/share", middleware.JWTAuth(), shareHandler.PostShareCall)
	api.DELETE("/calls/:id/share", middleware.JWTAuth(), shareHandler.DeleteShareCall)
	api.GET("/calls/:id/share", middleware.JWTAuth(), shareHandler.GetCallShare)

	// Bookmarks — JWT required.
	bookmarksGroup := api.Group("/bookmarks")
	bookmarksGroup.Use(middleware.JWTAuth())
	{
		bookmarksGroup.GET("", bookmarkHandler.GetBookmarkIDs)
		bookmarksGroup.GET("/calls", bookmarkHandler.GetBookmarkCalls)
		bookmarksGroup.POST("", bookmarkHandler.PostToggleBookmark)
	}

	// Call upload — API key auth.
	// Pre-auth body-size cap (50 MiB) prevents unauthenticated clients from
	// consuming memory/bandwidth before the API key is validated.
	upload := r.Group("/")
	upload.Use(middleware.MaxBodySize(50<<20), middleware.APIKeyAuth(deps.Queries))
	{
		upload.POST("/api/call-upload", callHandler.PostCallUpload)
		upload.POST("/api/trunk-recorder-call-upload", callHandler.PostCallUpload)
	}

	// Admin routes — JWT + admin role required.
	admin := api.Group("/admin")
	admin.Use(middleware.JWTAuth(), middleware.RequireAdmin(), middleware.MaxBodySize(2<<20)) // 2 MiB JSON body limit
	{
		// Import (file uploads — must stay REST)
		admin.POST("/import/talkgroups", importsHandler.ImportTalkgroups)
		admin.POST("/import/units", importsHandler.ImportUnits)
		admin.POST("/import/groups", importsHandler.ImportGroups)
		admin.POST("/import/tags", importsHandler.ImportTags)

		// RadioReference CSV preview (file upload — must stay REST)
		admin.POST("/radioreference/preview/csv", rrHandler.PreviewCSV)

		// Transcription status
		admin.GET("/transcriptions/status", transcriptionsHandler.GetStatus)

		// Swagger: issue a short-lived HTTP-only cookie so Swagger UI
		// can be opened in a new browser tab without exposing the JWT.
		admin.POST("/docs/session", authhandler.PostDocsSession)
	}

	// Swagger API documentation — protected by the HTTP-only cookie
	// set via POST /api/admin/docs/session above.
	swaggerDocs := api.Group("/admin/docs")
	swaggerDocs.Use(middleware.SwaggerCookieAuth())
	{
		swaggerDocs.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// WebSocket endpoints.
	// /api/ws is the canonical OpenScanner listener route. /ws is a temporary
	// compatibility alias that delegates to the same handler so existing
	// rdio-scanner-shaped clients keep working during the legacy-API transition.
	listenerWS := gin.WrapF(ws.HandleListenerWS(deps.Hub, deps.Queries))
	r.GET("/api/ws", listenerWS)
	r.GET("/ws", listenerWS)
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
