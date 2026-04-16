// Package main is the entry point for the OpenScanner server.
//
//	@title			OpenScanner API
//	@version		1.0
//	@description	Radio call manager API — real-time audio streaming, call management, and admin CRUD.
//
//	@BasePath	/api
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Paste the value exactly as copied (already includes Bearer prefix)
//
//	@securityDefinitions.apikey	APIKeyAuth
//	@in							header
//	@name						X-API-Key
//	@description				API key for call upload endpoints
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kardianos/service"
	"github.com/openscanner/openscanner/internal/api"
	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/cli"
	"github.com/openscanner/openscanner/internal/config"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/dirmonitor"
	"github.com/openscanner/openscanner/internal/downstream"
	"github.com/openscanner/openscanner/internal/logging"
	"github.com/openscanner/openscanner/internal/seed"
	"github.com/openscanner/openscanner/internal/ws"
	"golang.org/x/crypto/acme/autocert"
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

	// Check for CLI subcommands (login, logout, config-get, etc.) before starting the server.
	if cli.Run() {
		return
	}

	// kardianos/service configuration.
	svcConfig := &service.Config{
		Name:        "openscanner",
		DisplayName: "OpenScanner",
		Description: "OpenScanner Radio Call Manager",
	}

	prg := &program{cfg: cfg}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		slog.Error("failed to create service", "error", err)
		os.Exit(1)
	}

	// Handle service control commands (install/uninstall/start/stop/restart).
	if cfg.Service != "" {
		if err := service.Control(svc, cfg.Service); err != nil {
			slog.Error("service control failed", "action", cfg.Service, "error", err)
			os.Exit(1)
		}
		fmt.Printf("Service action %q completed successfully\n", cfg.Service)
		os.Exit(0)
	}

	// Run the service (works for both foreground and service manager modes).
	if err := svc.Run(); err != nil {
		slog.Error("service run failed", "error", err)
		os.Exit(1)
	}
}

// program implements the kardianos/service.Interface.
type program struct {
	cfg  *config.Config
	stop context.CancelFunc
}

func (p *program) Start(_ service.Service) error {
	go p.run()
	return nil
}

func (p *program) Stop(_ service.Service) error {
	if p.stop != nil {
		p.stop()
	}
	return nil
}

func (p *program) run() {
	cfg := p.cfg

	// Apply configured timezone so recorder timestamps are interpreted correctly.
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			slog.Error("invalid timezone", "timezone", cfg.Timezone, "error", err)
			os.Exit(1)
		}
		time.Local = loc
		slog.Info("timezone configured", "timezone", cfg.Timezone)
	}

	// Determine log file path (next to the database file).
	logFilePath := strings.TrimSuffix(cfg.DBFile, ".db") + ".log"

	// Load historical logs into the ring buffer before configuring
	// the new slog handler (so they appear in the admin UI immediately).
	logging.LoadHistoricalLogs(logFilePath)

	// Configure structured logging.
	if os.Getenv("OPENSCANNER_ENV") == "development" {
		logging.Configure(true, logFilePath)
	} else {
		logging.Configure(false, logFilePath)
		gin.SetMode(gin.ReleaseMode)
	}
	defer logging.CloseLogFile()

	// Print a human-readable startup banner.
	listenURL := cfg.Listen
	if listenURL[0] == ':' {
		listenURL = "0.0.0.0" + listenURL
	}
	scheme := "http"
	if cfg.SSLCert != "" || cfg.SSLAutoCert != "" {
		scheme = "https"
	}
	fmt.Fprintf(os.Stdout, "\n"+
		"  ┌───────────────────────────────────┐\n"+
		"  │       O P E N S C A N N E R       │\n"+
		"  └───────────────────────────────────┘\n"+
		"  Version:     %s\n"+
		"  URL:         %s\n"+
		"  Database:    %s\n"+
		"  Recordings:  %s\n\n",
		config.Version,
		scheme+"://"+listenURL,
		cfg.DBFile,
		cfg.RecordingsDir,
	)

	// Startup checks: verify external tool availability.
	checkExternalTools()

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

	queries := db.New(sqlDB)

	persistedLogLevel := ""
	if setting, err := queries.GetSetting(context.Background(), "logLevel"); err == nil {
		persistedLogLevel = setting.Value
		if err := logging.SetLevel(setting.Value); err != nil {
			slog.Warn("invalid persisted log level, keeping default", "value", setting.Value, "error", err)
		}
	}

	publicAccess := ""
	if setting, err := queries.GetSetting(context.Background(), "publicAccess"); err == nil {
		publicAccess = setting.Value
	}

	autoPopulate := ""
	if setting, err := queries.GetSetting(context.Background(), "autoPopulate"); err == nil {
		autoPopulate = setting.Value
	}

	slog.Debug("server: loaded settings from db",
		"log_level", persistedLogLevel,
		"public_access", publicAccess,
		"auto_populate", autoPopulate,
	)

	// Set up Gin router with registered routes.
	router := gin.New()
	router.MaxMultipartMemory = 50 << 20 // 50 MiB limit for multipart uploads
	router.Use(gin.Recovery())

	// Create the shutdown context early so it can be passed to long-lived components
	// (e.g. RateLimiter cleanup goroutine) to enable clean shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	p.stop = stop

	rateLimiter := auth.NewRateLimiter(ctx)

	// Set up bounded FFmpeg worker pool and audio processor.
	hasFFmpeg := audio.CheckFFmpeg()
	pool := audio.NewWorkerPool(ctx)
	processor := audio.NewProcessor(cfg.RecordingsDir, pool)

	// If ffmpeg is not available, force audioConversion to disabled in the DB.
	if !hasFFmpeg {
		if err := queries.UpsertSetting(context.Background(), db.UpsertSettingParams{
			Key: "audioConversion", Value: "0",
		}); err != nil {
			slog.Error("failed to force-disable audioConversion", "error", err)
		}
	}

	// Check whisper availability.
	hasWhisper := checkWhisper()

	// Start background call pruner.
	go audio.PruneLoop(ctx, queries, cfg.RecordingsDir)

	// Create and start WebSocket hub.
	hub := ws.NewHub(queries, config.Version)
	go hub.Run(ctx)

	// Start DirMonitor service.
	dsService := downstream.NewService(queries, processor)
	dsService.Start(ctx)

	dwService := dirmonitor.NewService(queries, processor, hub, dsService)
	dwService.Start(ctx)

	api.RegisterRoutes(router, api.Deps{
		Queries:            queries,
		RateLimiter:        rateLimiter,
		Processor:          processor,
		Hub:                hub,
		SQLDB:              sqlDB,
		DirMonitorReloader: dwService,
		DownstreamReloader: dsService,
		DownstreamNotifier: dsService,
		Version:            config.Version,
		FFmpegAvailable:    hasFFmpeg,
		WhisperAvailable:   hasWhisper,
	})

	// Create HTTP server.
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           router, // may be replaced below when SSL is enabled
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Channel to signal fatal server errors to the main goroutine.
	serverErr := make(chan error, 2)

	// When SSL is configured, replace the HTTP handler with a redirect so that
	// plaintext application traffic is never served to clients.
	sslEnabled := cfg.SSLAutoCert != "" || (cfg.SSLCert != "" && cfg.SSLKey != "")
	if sslEnabled {
		srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
		})
	}

	// Start HTTP server in a goroutine.
	go func() {
		slog.Info("HTTP server listening", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Start TLS server if configured.
	var tlsSrv *http.Server
	if cfg.SSLAutoCert != "" {
		tlsSrv = p.startAutoCertServer(cfg, router, srv, serverErr)
	} else if cfg.SSLCert != "" && cfg.SSLKey != "" {
		tlsSrv = p.startTLSServer(cfg, router, serverErr)
	}

	slog.Info("server: startup complete",
		"version", config.Version,
		"addr", cfg.Listen,
		"ssl_enabled", sslEnabled,
		"db", cfg.DBFile,
		"recordings_dir", cfg.RecordingsDir,
	)

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
		slog.Error("HTTP server shutdown error", "error", err)
	}

	if tlsSrv != nil {
		if err := tlsSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("TLS server shutdown error", "error", err)
		}
	}

	dsService.Stop()
	slog.Info("server: shutdown complete")
}

// startTLSServer starts an HTTPS server with the provided certificate and key files.
func (p *program) startTLSServer(cfg *config.Config, handler http.Handler, errCh chan<- error) *http.Server {
	addr := cfg.SSLListen
	if addr == "" {
		addr = ":443"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("TLS server listening", "addr", addr, "cert", cfg.SSLCert)
		if err := srv.ListenAndServeTLS(cfg.SSLCert, cfg.SSLKey); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	return srv
}

// startAutoCertServer starts an HTTPS server with Let's Encrypt auto-certificate management.
// httpSrv is the already-started HTTP server whose handler is augmented with the
// ACME HTTP-01 challenge responder for certificate issuance without ALPN.
func (p *program) startAutoCertServer(cfg *config.Config, handler http.Handler, httpSrv *http.Server, errCh chan<- error) *http.Server {
	addr := cfg.SSLListen
	if addr == "" {
		addr = ":443"
	}

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cfg.SSLAutoCert),
		Cache:      autocert.DirCache("autocert-cache"),
	}

	// Augment the HTTP listener to handle ACME HTTP-01 challenges.
	// The existing redirect handler is preserved for non-challenge requests.
	httpSrv.Handler = m.HTTPHandler(httpSrv.Handler)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		TLSConfig:         m.TLSConfig(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("TLS server listening (auto-cert)", "addr", addr, "domain", cfg.SSLAutoCert)
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	return srv
}

// checkExternalTools logs warnings if optional external tools are not available.
func checkExternalTools() {
	// Startup-only advisory messages; actual enforcement is in audio.CheckFFmpeg
	// and checkWhisper which return booleans used to gate features.
}

// checkWhisper returns true if a whisper binary is on PATH.
func checkWhisper() bool {
	if _, err := exec.LookPath("whisper"); err != nil {
		slog.Warn("whisper not found on PATH, transcription disabled")
		return false
	}
	return true
}
