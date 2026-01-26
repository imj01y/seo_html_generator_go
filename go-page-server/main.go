// Package main is the entry point for the Go page server
package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"go-page-server/config"
	"go-page-server/core"
	"go-page-server/database"
	"go-page-server/handlers"
)

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Configure zerolog
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05"
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "2006-01-02 15:04:05",
	})

	// Get project root (parent of go-page-server)
	execPath, err := os.Executable()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get executable path")
	}
	projectRoot := filepath.Dir(filepath.Dir(execPath))

	// For development, use current directory's parent
	if _, err := os.Stat(filepath.Join(projectRoot, "config.yaml")); os.IsNotExist(err) {
		cwd, _ := os.Getwd()
		projectRoot = filepath.Dir(cwd)
		if _, err := os.Stat(filepath.Join(projectRoot, "config.yaml")); os.IsNotExist(err) {
			projectRoot = cwd
			if _, err := os.Stat(filepath.Join(projectRoot, "..", "config.yaml")); err == nil {
				projectRoot = filepath.Dir(projectRoot)
			}
		}
	}

	log.Info().Str("project_root", projectRoot).Msg("Starting Go page server")

	// Load configuration from Python's config.yaml
	configPath := filepath.Join(projectRoot, "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", configPath).Msg("Failed to load configuration")
	}

	log.Info().
		Str("host", cfg.Server.Host).
		Int("port", cfg.Server.Port).
		Bool("debug", cfg.Server.Debug).
		Msg("Configuration loaded")

	// Initialize database connection
	if err := database.Init(&cfg.Database); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer database.Close()

	db := database.GetDB()

	// Initialize encoder
	core.InitEncoder(0.5)

	// Initialize components
	siteCache := core.NewSiteCache(db, 5*time.Minute)
	htmlCache := core.NewHTMLCache(config.GetCacheDir(projectRoot), cfg.Cache.MaxSizeGB)
	dataManager := core.NewDataManager(db, core.GetEncoder())
	funcsManager := core.NewTemplateFuncsManager(core.GetEncoder())

	// Initialize high-concurrency object pools (target: 500 QPS)
	log.Info().Msg("Initializing high-concurrency object pools (target: 500 QPS)...")
	startTime := time.Now()
	funcsManager.InitPools()
	log.Info().Dur("duration", time.Since(startTime)).Msg("Object pools initialized")

	// Load initial data for default group
	ctx := context.Background()
	log.Info().Msg("Loading initial data...")

	if err := dataManager.LoadAllForGroup(ctx, 1); err != nil {
		log.Warn().Err(err).Msg("Failed to load initial data for default group")
	}

	// Also load data into funcsManager for template rendering
	// This connects funcsManager to dataManager for keywords and images
	keywords := dataManager.GetRandomKeywords(1, 50000)
	if len(keywords) > 0 {
		funcsManager.LoadKeywords(keywords) // Note: these are already encoded
		log.Info().Int("count", len(keywords)).Msg("Keywords loaded to funcs manager")
	}

	// Load image URLs into funcsManager
	imageURLs := dataManager.GetImageURLs(1)
	if len(imageURLs) > 0 {
		funcsManager.LoadImageURLs(imageURLs)
		log.Info().Int("count", len(imageURLs)).Msg("Image URLs loaded to funcs manager")
	}

	// Create page handler
	pageHandler := handlers.NewPageHandler(
		db,
		cfg,
		siteCache,
		htmlCache,
		dataManager,
		funcsManager,
	)

	// Create compile handler
	cwd, _ := os.Getwd()
	templatesDir := filepath.Join(cwd, "templates")
	compileHandler := handlers.NewCompileHandler(pageHandler, templatesDir)

	// Setup Gin
	if !cfg.Server.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Middleware
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	// Routes - Page rendering
	r.GET("/page", pageHandler.ServePage)
	r.GET("/health", pageHandler.Health)
	r.GET("/stats", pageHandler.Stats)

	// Routes - Template compilation API
	api := r.Group("/api")
	{
		api.POST("/template/compile", compileHandler.CompileTemplate)
		api.POST("/template/validate", compileHandler.ValidateTemplate)
		api.POST("/template/preview", compileHandler.PreviewTemplate)
		api.GET("/template/compile/status", compileHandler.CompileStatus)
	}

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info().Str("addr", addr).Msg("Server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Wait for interrupt signal (SIGINT, SIGTERM for shutdown, SIGHUP for reload)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-quit
		if sig == syscall.SIGHUP {
			log.Info().Msg("Received SIGHUP, triggering graceful restart...")
			// In production, this would trigger a graceful restart
			// For now, just log the signal
			// The actual restart would be handled by a process manager like systemd or endless
			continue
		}
		break
	}

	log.Info().Msg("Shutting down server...")

	// Stop object pools
	funcsManager.StopPools()
	log.Info().Msg("Object pools stopped")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server stopped")
}

// requestLogger returns a Gin middleware for request logging
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		if query != "" {
			path = path + "?" + query
		}

		log.Debug().
			Int("status", status).
			Dur("latency", latency).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("ip", c.ClientIP()).
			Msg("Request")
	}
}
