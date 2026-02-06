// Package main is the entry point for the Go page server
package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // pprof for CPU profiling
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"seo-generator/api/internal/di"
	api "seo-generator/api/internal/handler"
	models "seo-generator/api/internal/model"
	database "seo-generator/api/internal/repository"
	core "seo-generator/api/internal/service"
	"seo-generator/api/pkg/config"
)

func main() {
	// Note: rand.Seed is deprecated in Go 1.20+, global random is auto-seeded

	// Configure logger using core.SetupLogger
	logConfig := core.DefaultLogConfig()
	logConfig.Format = "console" // 开发时用 console，生产用 json
	logConfig.Output = "stdout"
	if err := core.SetupLogger(logConfig); err != nil {
		// Fallback: print to stderr and continue
		fmt.Fprintf(os.Stderr, "Failed to setup logger: %v\n", err)
	}

	// Start pprof server for CPU profiling
	go func() {
		pprofPort := os.Getenv("PPROF_PORT")
		if pprofPort == "" {
			pprofPort = "6060"
		}
		log.Info().Str("port", pprofPort).Msg("Starting pprof server")
		if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
			log.Error().Err(err).Msg("pprof server failed")
		}
	}()

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

	// Initialize Redis connection (optional)
	var redisClient *redis.Client
	if cfg.Redis.Enabled {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Warn().Err(err).Msg("Failed to connect to Redis, some features will be disabled")
			redisClient = nil
		} else {
			log.Info().
				Str("host", cfg.Redis.Host).
				Int("port", cfg.Redis.Port).
				Msg("Redis connected")
		}
		cancel()
	} else {
		log.Info().Msg("Redis is disabled in configuration")
	}

	// Create dependency injection container
	container := di.NewContainer(db, cfg)
	if redisClient != nil {
		container.SetRedis(redisClient)
	}
	defer container.Close()
	log.Info().Msg("Dependency injection container initialized")

	// Initialize encoder
	core.InitEncoder(0.5)

	// Initialize components (permanent caching mode for 500 concurrent requests)
	// 缓存目录直接从 config.yaml 的 cache.dir 读取
	cacheDir := config.GetCacheDir(projectRoot, cfg.Cache.Dir)
	log.Info().Str("cache_dir", cacheDir).Msg("Cache directory from config.yaml")

	siteCache := core.NewSiteCache(db)
	templateCache := core.NewTemplateCache(db)
	htmlCache := core.NewHTMLCache(cacheDir, cfg.Cache.MaxSizeGB)
	funcsManager := core.NewTemplateFuncsManager(core.GetEncoder())

	// Initialize pool manager for titles and contents (in-memory cache)
	poolManager := core.NewPoolManager(db)
	poolCtx := context.Background()
	if err := poolManager.Start(poolCtx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start PoolManager")
	}
	log.Info().Msg("PoolManager initialized")

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
	funcsManager.InitPools(poolManager.GetConfig())
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

	// Initialize scheduler
	log.Info().Msg("Initializing scheduler...")
	scheduler := core.NewScheduler(db)

	// Register task handlers
	core.RegisterAllHandlers(scheduler, poolManager, templateCache, db, redisClient)

	// Start scheduler
	schedCtx := context.Background()
	if err := scheduler.Start(schedCtx); err != nil {
		log.Warn().Err(err).Msg("Failed to start scheduler (tables may not exist)")
	}

	// Load emojis from data/emojis.json into PoolManager
	emojisPath := filepath.Join(projectRoot, "data", "emojis.json")
	if err := poolManager.LoadEmojis(emojisPath); err != nil {
		log.Warn().Err(err).Str("path", emojisPath).Msg("Failed to load emojis")
	} else {
		log.Info().Int("count", poolManager.GetEmojiCount()).Msg("Emojis loaded to PoolManager")
	}

	// Also create a separate emojiManager for funcsManager (used in template rendering)
	emojiManager := core.NewEmojiManager()
	if err := emojiManager.LoadFromFile(emojisPath); err != nil {
		log.Warn().Err(err).Str("path", emojisPath).Msg("Failed to load emojis for funcsManager")
	}
	funcsManager.SetEmojiManager(emojiManager)

	// Note: keywords/images are now loaded by PoolManager.Start()
	// 初始化 TemplateFuncsManager 的关键词数据
	keywordGroupIDs := poolManager.GetKeywordGroupIDs()
	totalKeywords := 0
	for _, groupID := range keywordGroupIDs {
		keywords := poolManager.GetKeywords(groupID)
		rawKeywords := poolManager.GetAllRawKeywords(groupID)
		if len(keywords) > 0 {
			funcsManager.LoadKeywordGroup(groupID, keywords, rawKeywords)
			totalKeywords += len(keywords)
			log.Info().Int("group_id", groupID).Int("count", len(keywords)).
				Msg("Keyword group loaded to funcs manager")
		}
	}
	log.Info().Int("groups", len(keywordGroupIDs)).Int("total_keywords", totalKeywords).
		Msg("All keyword groups loaded to funcs manager")

	// Load all image groups into funcsManager
	imageGroupIDs := poolManager.GetImageGroupIDs()
	totalImages := 0
	for _, groupID := range imageGroupIDs {
		urls := poolManager.GetImages(groupID)
		if len(urls) > 0 {
			funcsManager.LoadImageGroup(groupID, urls)
			totalImages += len(urls)
			log.Info().Int("group_id", groupID).Int("count", len(urls)).
				Msg("Image group loaded to funcs manager")
		}
	}
	log.Info().Int("groups", len(imageGroupIDs)).Int("total_images", totalImages).
		Msg("All image groups loaded to funcs manager")

	// Create page handler
	pageHandler := api.NewPageHandler(
		db,
		cfg,
		siteCache,
		templateCache,
		htmlCache,
		funcsManager,
		poolManager,
	)

	// === 异步模板预热 ===
	go func() {
		log.Info().Msg("Starting async template warmup...")
		warmupStart := time.Now()
		warmupCount := 0

		templateCache.Range(func(tmpl *models.Template) bool {
			// 构造最小化渲染数据
			dummyData := &core.RenderData{
				Title:  "warmup",
				SiteID: 1,
			}
			// 触发模板编译和快速渲染器初始化
			_, err := pageHandler.GetTemplateRenderer().Render(
				tmpl.Content, tmpl.Name, dummyData, "")
			if err != nil {
				log.Warn().
					Err(err).
					Str("template", tmpl.Name).
					Msg("Template warmup failed")
			} else {
				warmupCount++
			}
			return true // 继续遍历
		})

		log.Info().
			Int("count", warmupCount).
			Dur("duration", time.Since(warmupStart)).
			Msg("Async template warmup completed")
	}()

	// Create cache handler
	cacheHandler := api.NewCacheHandler(
		htmlCache,
		pageHandler.GetTemplateRenderer(),
		siteCache,
		templateCache,
		projectRoot,
	)

	// Create log handler (for Nginx Lua cache hit logging)
	logHandler := api.NewLogHandler(db)

	// Setup Gin
	if !cfg.Server.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Middleware - 使用 core 包的中间件
	r.Use(core.RequestLogger()) // 使用 core.RequestLogger 替代本地 requestLogger
	r.Use(core.Recovery())      // 使用 core.Recovery 替代 gin.Recovery

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

	// Routes - API
	apiGroup := r.Group("/api")
	{
		// Cache management routes
		apiGroup.POST("/cache/clear", cacheHandler.ClearAllCache)
		apiGroup.POST("/cache/clear/:domain", cacheHandler.ClearDomainCache)
		apiGroup.POST("/cache/template/clear", cacheHandler.ClearTemplateCache)

		// Cache reload routes (for permanent cache updates)
		apiGroup.POST("/cache/site/reload", cacheHandler.ReloadAllSites)
		apiGroup.POST("/cache/site/reload/:domain", cacheHandler.ReloadSite)
		apiGroup.POST("/cache/template/reload", cacheHandler.ReloadAllTemplates)
		apiGroup.POST("/cache/template/reload/:name", cacheHandler.ReloadTemplate)

		// Cache config routes (for dynamic config reload)
		apiGroup.POST("/cache/config/reload", cacheHandler.ReloadCacheConfig)

		// Cache stats routes
		apiGroup.GET("/cache/stats", cacheHandler.GetCacheStats)
		apiGroup.POST("/cache/stats/recalculate", cacheHandler.RecalculateCacheStats)

		// Log routes (for Nginx Lua cache hit logging)
		apiGroup.GET("/log/spider", logHandler.LogSpiderVisit)
	}

	// 初始化监控服务
	log.Info().Msg("Initializing monitor service...")
	monitor := core.NewMonitor(10*time.Second, 360) // 10秒采集一次，保留1小时历史
	monitor.Start()

	// 初始化系统统计采集器
	log.Info().Msg("Initializing system stats collector...")
	systemStats := core.NewSystemStatsCollector()

	// Configure Admin API routes
	deps := &api.Dependencies{
		DB:               db,
		Redis:            redisClient,
		Config:           cfg,
		TemplateAnalyzer: templateAnalyzer,
		TemplateFuncs:    funcsManager,
		Scheduler:        scheduler,
		TemplateCache:    templateCache,
		Monitor:          monitor,
		PoolManager:      poolManager,
		SystemStats:      systemStats,
		SiteCache:        siteCache,
	}
	api.SetupRouter(r, deps)

	// Initialize and start StatsArchiver (requires Redis)
	var statsArchiver *core.StatsArchiver
	if redisClient != nil {
		statsArchiver = core.NewStatsArchiver(db, redisClient)
		archiverCtx, archiverCancel := context.WithCancel(context.Background())
		go statsArchiver.Start(archiverCtx)
		defer archiverCancel()
		log.Info().Msg("StatsArchiver initialized and started")
	} else {
		log.Info().Msg("StatsArchiver skipped (Redis not available)")
	}

	// Initialize and start SpiderLogsArchiver
	spiderLogsArchiver := core.NewSpiderLogsArchiver(db)
	spiderLogsArchiverCtx, spiderLogsArchiverCancel := context.WithCancel(context.Background())
	go spiderLogsArchiver.Start(spiderLogsArchiverCtx)
	defer spiderLogsArchiverCancel()
	log.Info().Msg("SpiderLogsArchiver initialized and started")

	// Initialize and start PoolReloader for hot-reload of pool configurations (requires Redis)
	var poolReloader *core.PoolReloader
	if redisClient != nil && funcsManager != nil {
		poolReloader = core.NewPoolReloader(redisClient, funcsManager)
		poolReloader.Start()
		defer poolReloader.Stop()
		log.Info().Msg("PoolReloader initialized and started")
	} else {
		log.Info().Msg("PoolReloader skipped (Redis or TemplateFuncsManager not available)")
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

	// Close Redis connection
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close Redis connection")
		} else {
			log.Info().Msg("Redis connection closed")
		}
	}

	// Stop StatsArchiver
	if statsArchiver != nil {
		statsArchiver.Stop()
		log.Info().Msg("StatsArchiver stopped")
	}

	// Stop SpiderLogsArchiver
	spiderLogsArchiver.Stop()
	log.Info().Msg("SpiderLogsArchiver stopped")

	// 停止监控服务
	monitor.Stop()
	log.Info().Msg("Monitor stopped")

	// Stop pool manager
	poolManager.Stop()
	log.Info().Msg("PoolManager stopped")

	// Stop object pools
	funcsManager.StopPools()
	log.Info().Msg("Object pools stopped")

	// Stop scheduler
	scheduler.Stop()
	log.Info().Msg("Scheduler stopped")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server stopped")
}

// findProjectRoot 查找项目根目录（包含 config.yaml 的目录）
// 搜索顺序：可执行文件的父目录 -> 当前目录的父目录 -> 当前目录
func findProjectRoot() string {
	const configFile = "config.yaml"
	cwd, _ := os.Getwd()

	// 构建候选路径列表
	candidates := []string{
		filepath.Dir(cwd), // 父目录
		cwd,               // 当前目录
	}

	// 尝试从可执行文件路径推断
	if execPath, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Dir(filepath.Dir(execPath))}, candidates...)
	}

	// 遍历候选路径
	for _, candidate := range candidates {
		if fileExists(filepath.Join(candidate, configFile)) {
			return candidate
		}
	}

	// 默认返回当前目录
	return cwd
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
