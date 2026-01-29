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

	// Find project root directory
	projectRoot := findProjectRoot()
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

	// Initialize components (permanent caching mode for 500 concurrent requests)
	// 缓存目录直接从 config.yaml 的 cache.dir 读取
	cacheDir := config.GetCacheDir(projectRoot, cfg.Cache.Dir)
	log.Info().Str("cache_dir", cacheDir).Msg("Cache directory from config.yaml")

	siteCache := core.NewSiteCache(db)
	templateCache := core.NewTemplateCache(db)
	htmlCache := core.NewHTMLCache(cacheDir, cfg.Cache.MaxSizeGB)
	dataManager := core.NewDataManager(db, core.GetEncoder())
	funcsManager := core.NewTemplateFuncsManager(core.GetEncoder())

	// Load all sites into cache at startup
	ctx := context.Background()
	log.Info().Msg("Loading all sites into cache...")
	if err := siteCache.LoadAll(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to load sites into cache")
	}

	// Initialize template analyzer
	log.Info().Msg("Initializing template analyzer...")
	templateAnalyzer := core.NewTemplateAnalyzer()
	templateAnalyzer.SetTargetQPS(500)
	templateAnalyzer.SetSafetyFactor(1.5)

	// Set analyzer on template cache (before loading templates)
	templateCache.SetAnalyzer(templateAnalyzer)

	// Load all templates into cache at startup
	log.Info().Msg("Loading all templates into cache...")
	if err := templateCache.LoadAll(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to load templates into cache")
	}

	// Initialize high-concurrency object pools (target: 500 QPS)
	log.Info().Msg("Initializing high-concurrency object pools (target: 500 QPS)...")
	startTime := time.Now()
	funcsManager.InitPools()
	log.Info().Dur("duration", time.Since(startTime)).Msg("Object pools initialized")

	// Set up template analyzer callback for pool size recommendations
	templateAnalyzer.OnConfigChanged(func(config *core.PoolSizeConfig) {
		log.Info().
			Int("cls_pool", config.ClsPoolSize).
			Int("url_pool", config.URLPoolSize).
			Int("keyword_emoji_pool", config.KeywordEmojiPoolSize).
			Int("number_pool", config.NumberPoolSize).
			Msg("Template analyzer recommends pool sizes")
		// 注意：实际的池大小调整需要在 object_pool 中实现动态扩容功能
		// 当前仅记录推荐值，用于监控和手动调整
	})

	// Initialize data pool manager
	log.Info().Msg("Initializing data pool manager...")
	dataPoolManager := core.NewDataPoolManager(db.DB, 5*time.Minute)

	// Load all data pools
	loadCtx, loadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := dataPoolManager.LoadAll(loadCtx); err != nil {
		log.Warn().Err(err).Msg("Failed to load some data pools")
	}
	loadCancel()

	// Start auto-refresh
	dataPoolManager.StartAutoRefresh()

	// Initialize scheduler
	log.Info().Msg("Initializing scheduler...")
	scheduler := core.NewScheduler(db)

	// Register task handlers
	core.RegisterAllHandlers(scheduler, dataPoolManager, templateCache, htmlCache, siteCache)

	// Start scheduler
	schedCtx := context.Background()
	if err := scheduler.Start(schedCtx); err != nil {
		log.Warn().Err(err).Msg("Failed to start scheduler (tables may not exist)")
	}

	// SEO analysis
	seoAnalysis := dataPoolManager.AnalyzeSEO(templateAnalyzer)
	if seoAnalysis != nil {
		log.Info().
			Str("status", string(seoAnalysis.OverallRating)).
			Msg("Data pool SEO analysis completed")

		// Log recommendations
		recommendations := dataPoolManager.GetRecommendations(templateAnalyzer)
		for poolName, rec := range recommendations {
			if rec.Status != core.SEORatingExcellent {
				log.Info().
					Str("pool", poolName).
					Int("current", rec.CurrentCount).
					Int("calls_per_page", rec.CallsPerPage).
					Float64("repeat_rate", rec.RepeatRate).
					Str("status", string(rec.Status)).
					Str("action", rec.Action).
					Msg("Pool recommendation")
			}
		}
	}

	// Load emojis from data/emojis.json
	emojisPath := filepath.Join(projectRoot, "data", "emojis.json")
	emojiManager := core.NewEmojiManager()
	if err := emojiManager.LoadFromFile(emojisPath); err != nil {
		log.Warn().Err(err).Str("path", emojisPath).Msg("Failed to load emojis")
	} else {
		log.Info().Int("count", emojiManager.Count()).Msg("Emojis loaded")
	}

	// Also load emojis into dataManager for backward compatibility
	if err := dataManager.LoadEmojis(emojisPath); err != nil {
		log.Warn().Err(err).Str("path", emojisPath).Msg("Failed to load emojis to data manager")
	}

	// Set emoji manager on funcsManager for keyword+emoji generation
	funcsManager.SetEmojiManager(emojiManager)

	// Load initial data for default group
	log.Info().Msg("Loading initial data...")

	if err := dataManager.LoadAllForGroup(ctx, 1); err != nil {
		log.Warn().Err(err).Msg("Failed to load initial data for default group")
	}

	// Also load data into funcsManager for template rendering
	// This connects funcsManager to dataManager for keywords and images
	// Use raw keywords (not encoded) - LoadKeywords will encode them
	rawKeywords := dataManager.GetRawKeywords(1, 50000)
	if len(rawKeywords) > 0 {
		funcsManager.LoadKeywords(rawKeywords)
		log.Info().Int("count", len(rawKeywords)).Msg("Keywords loaded to funcs manager")

		// Initialize keyword+emoji pool (requires keywords and emojiManager)
		log.Info().Msg("Initializing keyword emoji pool...")
		funcsManager.InitKeywordEmojiPool()
		log.Info().Msg("Keyword emoji pool initialized")
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
		templateCache,
		htmlCache,
		dataManager,
		funcsManager,
	)

	// Create compile handler
	cwd, _ := os.Getwd()
	templatesDir := filepath.Join(cwd, "templates")
	compileHandler := handlers.NewCompileHandler(pageHandler, templatesDir)

	// Create cache handler
	cacheHandler := handlers.NewCacheHandler(
		htmlCache,
		pageHandler.GetTemplateRenderer(),
		siteCache,
		templateCache,
		projectRoot,
	)

	// Create log handler (for Nginx Lua cache hit logging)
	logHandler := handlers.NewLogHandler(db)

	// Setup Gin
	if !cfg.Server.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Middleware
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	// CORS middleware for cross-origin requests from admin panel
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

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

		// Cache management routes
		api.POST("/cache/clear", cacheHandler.ClearAllCache)
		api.POST("/cache/clear/:domain", cacheHandler.ClearDomainCache)
		api.POST("/cache/template/clear", cacheHandler.ClearTemplateCache)

		// Cache reload routes (for permanent cache updates)
		api.POST("/cache/site/reload", cacheHandler.ReloadAllSites)
		api.POST("/cache/site/reload/:domain", cacheHandler.ReloadSite)
		api.POST("/cache/template/reload", cacheHandler.ReloadAllTemplates)
		api.POST("/cache/template/reload/:name", cacheHandler.ReloadTemplate)

		// Cache config routes (for dynamic config reload)
		api.POST("/cache/config/reload", cacheHandler.ReloadCacheConfig)

		// Log routes (for Nginx Lua cache hit logging)
		api.GET("/log/spider", logHandler.LogSpiderVisit)
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

	// Stop scheduler
	scheduler.Stop()
	log.Info().Msg("Scheduler stopped")

	// Stop data pool auto-refresh
	dataPoolManager.StopAutoRefresh()
	log.Info().Msg("Data pool auto-refresh stopped")

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

// findProjectRoot 查找项目根目录（包含 config.yaml 的目录）
// 搜索顺序：可执行文件的父目录 -> 当前目录的父目录 -> 当前目录 -> 当前目录的上级
func findProjectRoot() string {
	configFile := "config.yaml"

	// 尝试从可执行文件路径推断
	if execPath, err := os.Executable(); err == nil {
		candidate := filepath.Dir(filepath.Dir(execPath))
		if fileExists(filepath.Join(candidate, configFile)) {
			return candidate
		}
	}

	// 尝试当前工作目录及其父目录
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Dir(cwd), // 父目录
		cwd,               // 当前目录
	}

	for _, candidate := range candidates {
		if fileExists(filepath.Join(candidate, configFile)) {
			return candidate
		}
	}

	// 最后尝试当前目录的上级
	parent := filepath.Dir(cwd)
	if fileExists(filepath.Join(parent, configFile)) {
		return parent
	}

	// 默认返回当前目录
	return cwd
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
