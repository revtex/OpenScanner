// Package api contains Gin route handlers for OpenScanner.
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/middleware"
)

// Deps holds the dependencies required to register all API routes.
type Deps struct {
	Queries     *db.Queries
	RateLimiter *auth.RateLimiter
	Version     string
}

// RegisterRoutes wires all API routes onto the Gin engine.
func RegisterRoutes(r *gin.Engine, deps Deps) {
	setupHandler := NewSetupHandler(deps.Queries)
	authHandler := NewAuthHandler(deps.Queries, deps.RateLimiter)

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
	api.POST("/auth/login", authHandler.PostLogin)

	authRequired := api.Group("/auth")
	authRequired.Use(middleware.JWTAuth())
	{
		authRequired.POST("/logout", authHandler.PostLogout)
		authRequired.PUT("/password", authHandler.PutPassword)
		authRequired.GET("/me", authHandler.GetMe)
	}

	// Serve frontend (Phase 12 — placeholder)
	// r.NoRoute(func(c *gin.Context) { c.File("...") })
}
