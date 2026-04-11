// Package main is the entry point for the OpenScanner server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/api"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/config"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/dirwatch"
	"github.com/openscanner/openscanner/internal/seed"
	"github.com/openscanner/openscanner/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	if cfg.ShowVersion {
		fmt.Printf("openscanner %s\n", config.Version)
		os.Exit(0)
	}

	if cfg.ConfigSave {
		if err := cfg.SaveINI(); err != nil {
			slog.Error("failed to save configuration", "error", err)
			os.Exit(1)
		}
		fmt.Printf("Configuration saved to %s\n", cfg.ConfigFile)
		os.Exit(0)
	}

	// Configure structured logging.
	var handler slog.Handler
	if os.Getenv("OPENSCANNER_ENV") == "development" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
		gin.SetMode(gin.ReleaseMode)
	}
	slog.SetDefault(slog.New(handler))

	slog.Info("starting OpenScanner",
		"version", config.Version,
		"listen", cfg.Listen,
		"db_file", cfg.DBFile,
		"base_dir", cfg.BaseDir,
	)

	// Open database (runs migrations automatically).
	sqlDB, err := db.Open(cfg.DBFile)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	// Seed default data.
	if err := seed.Seed(context.Background(), sqlDB); err != nil {
		slog.Error("failed to seed database", "error", err)
		os.Exit(1)
	}

	// Set up Gin router with registered routes.
	router := gin.New()
	router.MaxMultipartMemory = 50 << 20 // 50 MiB limit for multipart uploads
	router.Use(gin.Recovery())

	// Create the shutdown context early so it can be passed to long-lived components
	// (e.g. RateLimiter cleanup goroutine) to enable clean shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	queries := db.New(sqlDB)
	rateLimiter := auth.NewRateLimiter(ctx)

	// Set up bounded FFmpeg worker pool and audio processor.
	pool := audio.NewWorkerPool(ctx)
	processor := audio.NewProcessor(cfg.BaseDir, pool)

	// Start background call pruner.
	go audio.PruneLoop(ctx, queries, cfg.BaseDir)

	// Create and start WebSocket hub.
	hub := ws.NewHub(queries, config.Version)
	go hub.Run(ctx)

	// Start DirWatch service.
	dwService := dirwatch.NewService(queries, processor, hub)
	dwService.Start(ctx)

	api.RegisterRoutes(router, api.Deps{
		Queries:          queries,
		RateLimiter:      rateLimiter,
		Processor:        processor,
		Hub:              hub,
		SQLDB:            sqlDB,
		DirwatchReloader: dwService,
		Version:          config.Version,
	})

	// Create HTTP server.
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Channel to signal fatal server errors to the main goroutine.
	serverErr := make(chan error, 1)

	// Start server in a goroutine.
	go func() {
		slog.Info("HTTP server listening", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Block until signal or server error.
	select {
	case <-ctx.Done():
		slog.Info("shutting down server...")
	case err := <-serverErr:
		slog.Error("server error", "error", err)
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("server stopped")
}
