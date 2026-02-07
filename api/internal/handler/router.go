// Package api provides the admin API routes and handlers
package api

import (
	"context"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	core "seo-generator/api/internal/service"
	"seo-generator/api/pkg/config"
)

// startTime 记录服务启动时间
var startTime = time.Now()

// Dependencies holds all dependencies required by the API handlers
type Dependencies struct {
	DB               *sqlx.DB
	Redis            *redis.Client
	Config           *config.Config
	TemplateAnalyzer *core.TemplateAnalyzer
	TemplateFuncs    *core.TemplateFuncsManager
	Scheduler        *core.Scheduler
	TemplateCache    *core.TemplateCache
	Monitor          *core.Monitor
	PoolManager      *core.PoolManager
	SystemStats      *core.SystemStatsCollector
	SiteCache        *core.SiteCache
}

// SetupRouter configures all API routes
func SetupRouter(r *gin.Engine, deps *Dependencies) {
	// 全局依赖注入中间件：将 db、redis、config 和 scheduler 注入到 context 中
	// 供使用 c.Get("db")、c.Get("redis")、c.Get("config") 和 c.Get("scheduler") 的 Handler 使用
	r.Use(DependencyInjectionMiddleware(deps.DB, deps.Redis, deps.Config, deps.Scheduler))

	// 双轨认证中间件（JWT 或 API Token），用于外部可调用的添加接口
	dualAuth := DualAuthMiddleware(deps.Config.Auth.SecretKey, deps.DB)

	// Auth routes (public - no middleware required)
	authGroup := r.Group("/api/auth")
	{
		authHandler := NewAuthHandler(
			deps.Config.Auth.SecretKey,
			deps.Config.Auth.AccessTokenExpireMinutes,
			deps.DB,
		)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/logout", authHandler.Logout)

		// Protected auth routes (require JWT)
		authProtected := authGroup.Group("")
		authProtected.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
		{
			authProtected.GET("/profile", authHandler.Profile)
			authProtected.POST("/change-password", authHandler.ChangePassword)
		}
	}

	// Dashboard routes (require JWT)
	dashboardHandler := NewDashboardHandler(deps.DB, deps.Monitor)
	dashboardGroup := r.Group("/api/dashboard")
	dashboardGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		dashboardGroup.GET("/stats", dashboardHandler.Stats)
		dashboardGroup.GET("/spider-visits", dashboardHandler.SpiderVisits)
		dashboardGroup.GET("/cache-stats", dashboardHandler.CacheStats)
	}

	// Logs routes (require JWT)
	logsHandler := NewLogsHandler(deps.DB)
	logsGroup := r.Group("/api/logs")
	logsGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		logsGroup.GET("/history", logsHandler.History)
		logsGroup.GET("/stats", logsHandler.Stats)
		logsGroup.DELETE("/clear", logsHandler.Clear)
	}

	// Templates routes (require JWT)
	templatesHandler := NewTemplatesHandler(deps.DB, deps.TemplateAnalyzer)
	templatesGroup := r.Group("/api/templates")
	templatesGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		templatesGroup.GET("", templatesHandler.List)
		templatesGroup.GET("/options", templatesHandler.Options)
		templatesGroup.GET("/:id", templatesHandler.Get)
		templatesGroup.GET("/:id/sites", templatesHandler.GetSites)
		templatesGroup.POST("", templatesHandler.Create)
		templatesGroup.PUT("/:id", templatesHandler.Update)
		templatesGroup.DELETE("/:id", templatesHandler.Delete)
	}

	// Keywords routes (require JWT)
	keywordsHandler := NewKeywordsHandler(deps.DB, deps.PoolManager, deps.TemplateFuncs)
	keywordsGroup := r.Group("/api/keywords")
	keywordsGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		// 分组管理
		keywordsGroup.GET("/groups", keywordsHandler.ListGroups)
		keywordsGroup.POST("/groups", keywordsHandler.CreateGroup)
		keywordsGroup.PUT("/groups/:id", keywordsHandler.UpdateGroup)
		keywordsGroup.DELETE("/groups/:id", keywordsHandler.DeleteGroup)

		// 关键词 CRUD
		keywordsGroup.GET("/list", keywordsHandler.List)
		keywordsGroup.PUT("/:id", keywordsHandler.Update)
		keywordsGroup.DELETE("/:id", keywordsHandler.Delete)

		// 批量操作
		keywordsGroup.DELETE("/batch", keywordsHandler.BatchDelete)
		keywordsGroup.DELETE("/delete-all", keywordsHandler.DeleteAll)
		keywordsGroup.PUT("/batch/status", keywordsHandler.BatchUpdateStatus)
		keywordsGroup.PUT("/batch/move", keywordsHandler.BatchMove)

		// 上传
		keywordsGroup.POST("/upload", keywordsHandler.Upload)

		// 辅助功能
		keywordsGroup.POST("/reload", keywordsHandler.Reload)
	}

	// Keywords 添加接口（支持 JWT 或 API Token 双轨认证）
	keywordsDual := r.Group("/api/keywords")
	keywordsDual.Use(dualAuth)
	{
		keywordsDual.POST("/add", keywordsHandler.Add)
		keywordsDual.POST("/batch", keywordsHandler.BatchAdd)
	}

	// Images routes (require JWT)
	imagesHandler := NewImagesHandler(deps.DB, deps.PoolManager, deps.TemplateFuncs)
	imagesGroup := r.Group("/api/images")
	imagesGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		// 分组管理
		imagesGroup.GET("/groups", imagesHandler.ListGroups)
		imagesGroup.POST("/groups", imagesHandler.CreateGroup)
		imagesGroup.PUT("/groups/:id", imagesHandler.UpdateGroup)
		imagesGroup.DELETE("/groups/:id", imagesHandler.DeleteGroup)

		// 图片URL管理
		imagesGroup.GET("/urls/list", imagesHandler.ListURLs)
		imagesGroup.POST("/upload", imagesHandler.Upload)
		imagesGroup.PUT("/urls/:id", imagesHandler.UpdateURL)
		imagesGroup.DELETE("/urls/:id", imagesHandler.DeleteURL)

		// 批量操作
		imagesGroup.DELETE("/batch", imagesHandler.BatchDelete)
		imagesGroup.DELETE("/delete-all", imagesHandler.DeleteAll)
		imagesGroup.PUT("/batch/status", imagesHandler.BatchUpdateStatus)
		imagesGroup.PUT("/batch/move", imagesHandler.BatchMove)

		// 辅助功能
		imagesGroup.POST("/urls/reload", imagesHandler.Reload)
	}

	// Images 添加接口（支持 JWT 或 API Token 双轨认证）
	imagesDual := r.Group("/api/images")
	imagesDual.Use(dualAuth)
	{
		imagesDual.POST("/urls/add", imagesHandler.AddURL)
		imagesDual.POST("/urls/batch", imagesHandler.BatchAddURLs)
	}

	// Articles routes (require JWT)
	articlesHandler := NewArticlesHandler(deps.DB, deps.Redis)
	articlesGroup := r.Group("/api/articles")
	articlesGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		// 分组管理
		articlesGroup.GET("/groups", articlesHandler.ListGroups)
		articlesGroup.POST("/groups", articlesHandler.CreateGroup)
		articlesGroup.PUT("/groups/:id", articlesHandler.UpdateGroup)
		articlesGroup.DELETE("/groups/:id", articlesHandler.DeleteGroup)

		// 文章 CRUD
		articlesGroup.GET("/list", articlesHandler.List)
		articlesGroup.GET("/:id", articlesHandler.Get)
		articlesGroup.PUT("/:id", articlesHandler.Update)
		articlesGroup.DELETE("/:id", articlesHandler.Delete)

		// 批量操作
		articlesGroup.DELETE("/batch/delete", articlesHandler.BatchDelete)
		articlesGroup.DELETE("/delete-all", articlesHandler.DeleteAll)
		articlesGroup.PUT("/batch/status", articlesHandler.BatchUpdateStatus)
		articlesGroup.PUT("/batch/move", articlesHandler.BatchMove)
	}

	// Articles 添加接口（支持 JWT 或 API Token 双轨认证）
	articlesDual := r.Group("/api/articles")
	articlesDual.Use(dualAuth)
	{
		articlesDual.POST("/add", articlesHandler.Add)
		articlesDual.POST("/batch", articlesHandler.BatchAdd)
	}

	// Sites routes (require JWT)
	sitesHandler := NewSitesHandler(deps.DB, deps.SiteCache)
	sitesGroup := r.Group("/api/sites")
	sitesGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		sitesGroup.GET("", sitesHandler.List)
		sitesGroup.POST("", sitesHandler.Create)
		sitesGroup.GET("/:id", sitesHandler.Get)
		sitesGroup.PUT("/:id", sitesHandler.Update)
		sitesGroup.DELETE("/:id", sitesHandler.Delete)
		sitesGroup.DELETE("/batch/delete", sitesHandler.BatchDelete)
		sitesGroup.PUT("/batch/status", sitesHandler.BatchUpdateStatus)
	}

	// Site Groups routes (require JWT)
	siteGroupsGroup := r.Group("/api/site-groups")
	siteGroupsGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		siteGroupsGroup.GET("", sitesHandler.ListGroups)
		siteGroupsGroup.POST("", sitesHandler.CreateGroup)
		siteGroupsGroup.GET("/:id", sitesHandler.GetGroup)
		siteGroupsGroup.GET("/:id/options", sitesHandler.GetGroupOptions)
		siteGroupsGroup.PUT("/:id", sitesHandler.UpdateGroup)
		siteGroupsGroup.DELETE("/:id", sitesHandler.DeleteGroup)
	}

	// Groups options route (require JWT)
	r.GET("/api/groups/options", AuthMiddleware(deps.Config.Auth.SecretKey), sitesHandler.GetAllGroupOptions)

	// Spider Projects routes (require JWT)
	spiderProjectsHandler := &SpiderProjectsHandler{}
	spiderFilesHandler := &SpiderFilesHandler{}
	spiderExecutionHandler := &SpiderExecutionHandler{}
	spiderProjectStatsHandler := &SpiderStatsHandler{}
	spiderRoutes := r.Group("/api/spider-projects")
	spiderRoutes.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		spiderRoutes.GET("", spiderProjectsHandler.List)
		spiderRoutes.POST("", spiderProjectsHandler.Create)
		spiderRoutes.GET("/templates", spiderProjectsHandler.GetCodeTemplates)
		spiderRoutes.GET("/:id", spiderProjectsHandler.Get)
		spiderRoutes.PUT("/:id", spiderProjectsHandler.Update)
		spiderRoutes.DELETE("/:id", spiderProjectsHandler.Delete)
		spiderRoutes.POST("/:id/toggle", spiderProjectsHandler.Toggle)

		// 文件管理 - 支持树形结构
		spiderRoutes.GET("/:id/files", spiderFilesHandler.ListFiles)           // ?tree=true 返回树形结构
		spiderRoutes.GET("/:id/files/*path", spiderFilesHandler.GetFile)       // 获取文件内容
		spiderRoutes.POST("/:id/files", spiderFilesHandler.CreateItem)         // 根目录创建
		spiderRoutes.POST("/:id/files/*path", spiderFilesHandler.CreateItem)   // 指定目录创建
		spiderRoutes.PUT("/:id/files/*path", spiderFilesHandler.UpdateFile)    // 更新文件
		spiderRoutes.DELETE("/:id/files/*path", spiderFilesHandler.DeleteFile) // 删除文件/目录
		spiderRoutes.PATCH("/:id/files/*path", spiderFilesHandler.MoveItem)    // 移动/重命名

		// 任务控制
		spiderRoutes.POST("/:id/run", spiderExecutionHandler.Run)
		spiderRoutes.POST("/:id/test", spiderExecutionHandler.Test)
		spiderRoutes.POST("/:id/test/stop", spiderExecutionHandler.TestStop)
		spiderRoutes.POST("/:id/stop", spiderExecutionHandler.Stop)
		spiderRoutes.POST("/:id/pause", spiderExecutionHandler.Pause)
		spiderRoutes.POST("/:id/resume", spiderExecutionHandler.Resume)

		// 统计
		spiderRoutes.GET("/:id/stats/realtime", spiderProjectStatsHandler.GetRealtimeStats)
		spiderRoutes.GET("/:id/stats/chart", spiderProjectStatsHandler.GetChartStats)

		// 队列管理
		spiderRoutes.POST("/:id/queue/clear", spiderProjectStatsHandler.ClearQueue)

		// 失败请求
		spiderRoutes.GET("/:id/failed", spiderProjectStatsHandler.ListFailed)
		spiderRoutes.GET("/:id/failed/stats", spiderProjectStatsHandler.GetFailedStats)
		spiderRoutes.POST("/:id/failed/retry-all", spiderProjectStatsHandler.RetryAllFailed)
		spiderRoutes.POST("/:id/failed/:fid/retry", spiderProjectStatsHandler.RetryOneFailed)
		spiderRoutes.POST("/:id/failed/:fid/ignore", spiderProjectStatsHandler.IgnoreFailed)
		spiderRoutes.DELETE("/:id/failed/:fid", spiderProjectStatsHandler.DeleteFailed)
	}

	// Spider Stats routes (require JWT)
	spiderStatsHandler := &SpiderStatsHandler{}
	statsRoutes := r.Group("/api/spider-stats")
	statsRoutes.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		statsRoutes.GET("/overview", spiderStatsHandler.GetOverview)
		statsRoutes.GET("/chart", spiderStatsHandler.GetChart)
		statsRoutes.GET("/scheduled", spiderStatsHandler.GetScheduled)
		statsRoutes.GET("/by-project", spiderStatsHandler.GetByProject)
	}

	// Pool config routes (require JWT) - 使用 PoolConfigHandler
	poolConfigHandler := NewPoolConfigHandler(deps.DB, deps.Redis, deps.TemplateAnalyzer)
	poolConfigGroup := r.Group("/api/pool-config")
	poolConfigGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		poolConfigGroup.GET("", poolConfigHandler.GetConfig)
		poolConfigGroup.PUT("", poolConfigHandler.UpdateConfig)
	}

	// Cache Pool routes (require JWT) - 标题和正文缓存池配置
	if deps.PoolManager != nil {
		cachePoolHandler := NewPoolHandler(deps.DB, deps.PoolManager, deps.TemplateFuncs)
		cachePoolGroup := r.Group("/api/cache-pool")
		cachePoolGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
		{
			cachePoolGroup.GET("/config", cachePoolHandler.GetConfig)
			cachePoolGroup.PUT("/config", cachePoolHandler.UpdateConfig)
			cachePoolGroup.GET("/stats", cachePoolHandler.GetStats)
			cachePoolGroup.POST("/reload", cachePoolHandler.Reload)
		}
	}

	// Settings routes (require JWT)
	settingsHandler := &SettingsHandler{}
	settingsRoutes := r.Group("/api/settings")
	settingsRoutes.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		settingsRoutes.GET("", settingsHandler.Get)
		settingsRoutes.GET("/cache", settingsHandler.GetCacheSettings)
		settingsRoutes.PUT("/cache", settingsHandler.UpdateCacheSettings)
		settingsRoutes.POST("/cache/apply", settingsHandler.ApplyCacheSettings)
		settingsRoutes.GET("/database", settingsHandler.GetDatabaseStatus)
		settingsRoutes.GET("/api-token", settingsHandler.GetAPIToken)
		settingsRoutes.PUT("/api-token", settingsHandler.UpdateAPIToken)
		settingsRoutes.POST("/api-token/generate", settingsHandler.GenerateAPIToken)
	}

	// Spider Detector routes (require JWT)
	spiderDetectorHandler := &SpiderDetectorHandler{}
	spiderDetectorRoutes := r.Group("/api/spiders")
	spiderDetectorRoutes.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		spiderDetectorRoutes.GET("/config", spiderDetectorHandler.GetSpiderConfig)
		spiderDetectorRoutes.POST("/test", spiderDetectorHandler.TestSpiderDetection)
		spiderDetectorRoutes.GET("/logs", spiderDetectorHandler.GetSpiderLogs)
		spiderDetectorRoutes.GET("/stats", spiderDetectorHandler.GetSpiderStats)
		spiderDetectorRoutes.GET("/daily-stats", spiderDetectorHandler.GetSpiderDailyStats)
		spiderDetectorRoutes.GET("/hourly-stats", spiderDetectorHandler.GetSpiderHourlyStats)
		spiderDetectorRoutes.DELETE("/logs/clear", spiderDetectorHandler.ClearSpiderLogs)
		spiderDetectorRoutes.GET("/trend", spiderDetectorHandler.GetSpiderTrend)
	}

	// Processor routes (数据加工，require JWT)
	processorHandler := &ProcessorHandler{}
	processorRoutes := r.Group("/api/processor")
	processorRoutes.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		processorRoutes.GET("/config", processorHandler.GetConfig)
		processorRoutes.PUT("/config", processorHandler.UpdateConfig)
		processorRoutes.POST("/start", processorHandler.Start)
		processorRoutes.POST("/stop", processorHandler.Stop)
		processorRoutes.POST("/retry-all", processorHandler.RetryAll)
		processorRoutes.DELETE("/dead-queue", processorHandler.ClearDeadQueue)
	}

	// Content Worker Files routes (内容处理代码编辑器，require JWT)
	contentWorkerHandler := NewContentWorkerFilesHandler("/project/content_worker")
	contentWorkerRoutes := r.Group("/api/content-worker")
	contentWorkerRoutes.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		// 目录树（用于移动弹窗）
		contentWorkerRoutes.GET("/files", func(c *gin.Context) {
			if c.Query("tree") == "true" {
				contentWorkerHandler.GetTree(c)
			} else {
				contentWorkerHandler.ListDir(c)
			}
		})
		// 文件/目录操作
		contentWorkerRoutes.GET("/files/*path", contentWorkerHandler.ListDir)
		contentWorkerRoutes.POST("/files/*path", contentWorkerHandler.Create)
		contentWorkerRoutes.PUT("/files/*path", contentWorkerHandler.Save)
		contentWorkerRoutes.DELETE("/files/*path", contentWorkerHandler.Delete)
		contentWorkerRoutes.PATCH("/files/*path", contentWorkerHandler.Move)

		// 上传下载
		contentWorkerRoutes.POST("/upload/*path", contentWorkerHandler.Upload)
		contentWorkerRoutes.GET("/download/*path", contentWorkerHandler.Download)

	}

	// WebSocket routes (不需要认证)
	wsHandler := NewWebSocketHandler(deps.TemplateFuncs, deps.PoolManager, deps.SystemStats)
	r.GET("/ws/spider-logs/:id", wsHandler.SpiderLogs)
	r.GET("/ws/spider-stats/:id", wsHandler.SpiderStats)
	r.GET("/ws/worker-restart", wsHandler.WorkerRestart)
	r.GET("/ws/worker-logs", wsHandler.WorkerLogs)
	r.GET("/ws/processor-logs", wsHandler.ProcessorLogs)
	r.GET("/ws/processor-status", wsHandler.ProcessorStatus)
	r.GET("/ws/pool-status", wsHandler.PoolStatus)
	r.GET("/api/logs/ws", wsHandler.SystemLogs)
	r.GET("/ws/system-stats", wsHandler.SystemStats)

	// Admin API group (require JWT)
	admin := r.Group("/api/admin")
	admin.Use(AuthMiddleware(deps.Config.Auth.SecretKey))

	// Pool management routes
	pool := admin.Group("/pool")
	{
		pool.GET("/stats", poolStatsHandler(deps))
		pool.GET("/presets", poolPresetsHandler(deps))
		pool.GET("/preset/:name", poolPresetByNameHandler(deps))
		pool.POST("/preset/:name", poolPresetByNameHandler(deps))
		pool.POST("/resize", poolResizeHandler(deps))
		pool.POST("/clear", poolClearHandler(deps))
	}

	// Template analysis routes
	template := admin.Group("/template")
	{
		template.GET("/analysis", templateAnalysisListHandler(deps))
		template.GET("/analysis/:id", templateAnalysisByIDHandler(deps))
		template.POST("/analyze/:id", templateAnalyzeHandler(deps))
	}

	// Data pool routes
	data := admin.Group("/data")
	{
		data.GET("/stats", dataStatsHandler(deps))
		data.POST("/refresh", dataRefreshHandler(deps))
	}

	// Task management routes
	task := admin.Group("/task")
	{
		task.GET("/list", taskListHandler(deps))
		task.GET("/:id", taskByIDHandler(deps))
		task.GET("/:id/logs", taskLogsHandler(deps))
		task.POST("/:id/trigger", taskTriggerHandler(deps))
		task.POST("/:id/enable", taskEnableHandler(deps))
		task.POST("/:id/disable", taskDisableHandler(deps))
	}

	// System info routes
	system := admin.Group("/system")
	{
		system.GET("/info", systemInfoHandler(deps))
		system.GET("/health", systemHealthHandler(deps))
		system.GET("/metrics", metricsHandler(deps))
		system.GET("/metrics/history", metricsHistoryHandler(deps))
		system.GET("/alerts", alertsHandler(deps))
		system.GET("/monitor", monitorStatsHandler(deps))
	}
}

// ============ Pool Management Handlers ============

// poolStatsHandler GET /stats - 获取池统计信息
func poolStatsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateFuncs == nil {
			core.FailWithCode(c, core.ErrPoolInvalid)
			return
		}
		stats := deps.TemplateFuncs.GetPoolStats()
		core.Success(c, stats)
	}
}

// poolPresetsHandler GET /presets - 获取所有预设配置
func poolPresetsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateAnalyzer == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		presets := core.GetAllPoolPresets()
		maxStats := deps.TemplateAnalyzer.GetMaxStats()

		// 为每个预设计算池大小和内存估算
		type presetDetail struct {
			core.PoolPreset
			Key            string               `json:"key"`
			PoolSizes      *core.PoolSizeConfig `json:"pool_sizes"`
			MemoryEstimate int64                `json:"memory_estimate"`
			MemoryHuman    string               `json:"memory_human"`
		}

		result := make([]presetDetail, 0, len(presets))
		for key, preset := range presets {
			poolSizes := core.CalculatePoolSizes(preset, *maxStats)
			memoryEstimate := core.EstimateMemoryUsage(poolSizes)

			result = append(result, presetDetail{
				PoolPreset:     preset,
				Key:            key,
				PoolSizes:      poolSizes,
				MemoryEstimate: memoryEstimate,
				MemoryHuman:    core.FormatMemorySize(memoryEstimate),
			})
		}

		core.Success(c, result)
	}
}

// poolPresetByNameHandler GET/POST /preset/:name - 获取或应用预设
func poolPresetByNameHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateAnalyzer == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		name := c.Param("name")

		preset, ok := core.GetPoolPreset(name)
		if !ok {
			core.FailWithCode(c, core.ErrNotFound)
			return
		}

		maxStats := deps.TemplateAnalyzer.GetMaxStats()
		poolSizes := core.CalculatePoolSizes(preset, *maxStats)
		memoryEstimate := core.EstimateMemoryUsage(poolSizes)

		if c.Request.Method == http.MethodGet {
			// GET: 返回预设详情
			core.Success(c, gin.H{
				"preset":          preset,
				"pool_sizes":      poolSizes,
				"memory_estimate": memoryEstimate,
				"memory_human":    core.FormatMemorySize(memoryEstimate),
			})
			return
		}

		// POST: 应用预设
		if deps.TemplateFuncs == nil {
			core.FailWithCode(c, core.ErrPoolInvalid)
			return
		}

		deps.TemplateFuncs.ResizePools(poolSizes)

		core.Success(c, gin.H{
			"message":         "预设已应用",
			"preset":          preset,
			"pool_sizes":      poolSizes,
			"memory_estimate": memoryEstimate,
			"memory_human":    core.FormatMemorySize(memoryEstimate),
		})
	}
}

// poolResizeRequest 池调整大小请求
type poolResizeRequest struct {
	ClsSize          int `json:"cls_size"`
	URLSize          int `json:"url_size"`
	KeywordEmojiSize int `json:"keyword_emoji_size"`
}

// poolResizeHandler POST /resize - 调整池大小
func poolResizeHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req poolResizeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, err.Error())
			return
		}

		if deps.TemplateFuncs == nil {
			core.FailWithCode(c, core.ErrPoolInvalid)
			return
		}

		config := &core.PoolSizeConfig{
			ClsPoolSize:          req.ClsSize,
			URLPoolSize:          req.URLSize,
			KeywordEmojiPoolSize: req.KeywordEmojiSize,
		}

		deps.TemplateFuncs.ResizePools(config)

		core.Success(c, gin.H{
			"message":    "池大小已调整",
			"pool_sizes": config,
		})
	}
}

// poolClearHandler POST /clear - 清空池
func poolClearHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateFuncs == nil {
			core.FailWithCode(c, core.ErrPoolInvalid)
			return
		}

		deps.TemplateFuncs.ClearPools()

		core.Success(c, gin.H{
			"message": "池已清空",
		})
	}
}

// ============ Template Analysis Handlers ============

// templateAnalysisListHandler GET /analysis - 获取所有模板分析结果
func templateAnalysisListHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateAnalyzer == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		templates := deps.TemplateAnalyzer.GetAllAnalyses()
		maxStats := deps.TemplateAnalyzer.GetMaxStats()
		stats := deps.TemplateAnalyzer.GetStats()

		core.Success(c, gin.H{
			"templates": templates,
			"max_stats": maxStats,
			"stats":     stats,
		})
	}
}

// templateAnalysisByIDHandler GET /analysis/:id - 获取单个模板分析结果
// :id 是站点组 ID，需要查询参数 name
func templateAnalysisByIDHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateAnalyzer == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		idParam := c.Param("id")
		siteGroupID, err := strconv.Atoi(idParam)
		if err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, "无效的站点组 ID")
			return
		}

		name := c.Query("name")
		if name == "" {
			core.FailWithMessage(c, core.ErrInvalidParam, "需要提供模板名称 (name 查询参数)")
			return
		}

		analysis := deps.TemplateAnalyzer.GetAnalysis(name, siteGroupID)
		if analysis == nil {
			core.FailWithCode(c, core.ErrTemplateNotFound)
			return
		}

		core.Success(c, analysis)
	}
}

// templateAnalyzeHandler POST /analyze/:id - 分析指定模板
// :id 是站点组 ID，需要查询参数 name
func templateAnalyzeHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateAnalyzer == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}
		if deps.TemplateCache == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		idParam := c.Param("id")
		siteGroupID, err := strconv.Atoi(idParam)
		if err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, "无效的站点组 ID")
			return
		}

		name := c.Query("name")
		if name == "" {
			core.FailWithMessage(c, core.ErrInvalidParam, "需要提供模板名称 (name 查询参数)")
			return
		}

		// 从缓存获取模板
		tpl := deps.TemplateCache.Get(name, siteGroupID)
		if tpl == nil {
			core.FailWithCode(c, core.ErrTemplateNotFound)
			return
		}

		// 执行分析
		analysis := deps.TemplateAnalyzer.AnalyzeTemplate(tpl.Name, tpl.SiteGroupID, tpl.Content)

		core.Success(c, analysis)
	}
}

// ============ Data Pool Handlers ============

// dataStatsHandler GET /stats - 获取数据池运行状态统计
func dataStatsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.PoolManager == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		// 返回与对象池格式一致的统计
		pools := deps.PoolManager.GetDataPoolsStats()
		core.Success(c, gin.H{
			"pools": pools,
		})
	}
}

// dataRefreshRequest 数据刷新请求
type dataRefreshRequest struct {
	Pool    string `json:"pool" binding:"required,oneof=all keywords images titles contents emojis keyword_emojis"`
	GroupID *int   `json:"group_id"`
}

// dataRefreshHandler POST /refresh - 刷新数据池
func dataRefreshHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.PoolManager == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		var req dataRefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		switch req.Pool {
		case "keywords":
			if req.GroupID != nil {
				deps.PoolManager.ReloadKeywordGroup(ctx, *req.GroupID)
			} else {
				deps.PoolManager.RefreshData(ctx, "keywords")
			}
		case "images":
			if req.GroupID != nil {
				deps.PoolManager.ReloadImageGroup(ctx, *req.GroupID)
			} else {
				deps.PoolManager.RefreshData(ctx, "images")
			}
		case "titles":
			if req.GroupID != nil {
				if tg := deps.PoolManager.GetTitleGenerator(); tg != nil {
					tg.ReloadGroup(*req.GroupID)
				}
			} else {
				deps.PoolManager.RefreshData(ctx, "titles")
			}
		case "contents":
			if req.GroupID != nil {
				deps.PoolManager.ReloadContentGroup(ctx, *req.GroupID)
			} else {
				deps.PoolManager.RefreshData(ctx, "contents")
			}
		case "keyword_emojis":
			if req.GroupID != nil {
				if keg := deps.PoolManager.GetKeywordEmojiGenerator(); keg != nil {
					keg.ReloadGroup(*req.GroupID)
				}
			} else {
				deps.PoolManager.RefreshData(ctx, "keyword_emojis")
			}
		case "emojis":
			deps.PoolManager.ReloadEmojis("data/emojis.json")
		default:
			if err := deps.PoolManager.RefreshData(ctx, req.Pool); err != nil {
				core.FailWithMessage(c, core.ErrInternalServer, err.Error())
				return
			}
		}

		stats := deps.PoolManager.GetPoolStatsSimple()
		core.Success(c, gin.H{
			"success": true,
			"pool":    req.Pool,
			"stats":   stats,
		})
	}
}

// ============ Task Management Handlers ============

// taskListHandler GET /list - 获取所有任务
func taskListHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Scheduler == nil {
			core.FailWithCode(c, core.ErrSchedulerNotRunning)
			return
		}

		tasks, err := deps.Scheduler.GetTasks(c.Request.Context())
		if err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}

		core.Success(c, gin.H{
			"tasks": tasks,
			"total": len(tasks),
		})
	}
}

// taskByIDHandler GET /:id - 获取单个任务
func taskByIDHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Scheduler == nil {
			core.FailWithCode(c, core.ErrSchedulerNotRunning)
			return
		}

		idParam := c.Param("id")
		taskID, err := strconv.Atoi(idParam)
		if err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, "无效的任务 ID")
			return
		}

		// 从任务列表中查找指定任务
		tasks, err := deps.Scheduler.GetTasks(c.Request.Context())
		if err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}

		for _, task := range tasks {
			if task.ID == int64(taskID) {
				core.Success(c, task)
				return
			}
		}

		core.FailWithCode(c, core.ErrSchedulerTaskNotFound)
	}
}

// taskLogsHandler GET /:id/logs - 获取任务日志
func taskLogsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Scheduler == nil {
			core.FailWithCode(c, core.ErrSchedulerNotRunning)
			return
		}

		idParam := c.Param("id")
		taskID, err := strconv.Atoi(idParam)
		if err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, "无效的任务 ID")
			return
		}

		// 获取 limit 参数，默认 20，最大 100
		limit := 20
		if limitParam := c.Query("limit"); limitParam != "" {
			if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
				limit = l
				if limit > 100 {
					limit = 100
				}
			}
		}

		logs, err := deps.Scheduler.GetTaskLogs(c.Request.Context(), int64(taskID), limit)
		if err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}

		core.Success(c, gin.H{
			"logs":    logs,
			"total":   len(logs),
			"task_id": taskID,
			"limit":   limit,
		})
	}
}

// taskTriggerHandler POST /:id/trigger - 手动触发任务
func taskTriggerHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Scheduler == nil {
			core.FailWithCode(c, core.ErrSchedulerNotRunning)
			return
		}

		idParam := c.Param("id")
		taskID, err := strconv.Atoi(idParam)
		if err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, "无效的任务 ID")
			return
		}

		if err := deps.Scheduler.TriggerTask(int64(taskID)); err != nil {
			core.FailWithMessage(c, core.ErrSchedulerExecFailed, err.Error())
			return
		}

		core.Success(c, gin.H{
			"message": "任务已触发",
			"task_id": taskID,
		})
	}
}

// taskEnableHandler POST /:id/enable - 启用任务
func taskEnableHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Scheduler == nil {
			core.FailWithCode(c, core.ErrSchedulerNotRunning)
			return
		}

		idParam := c.Param("id")
		taskID, err := strconv.Atoi(idParam)
		if err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, "无效的任务 ID")
			return
		}

		if err := deps.Scheduler.EnableTask(c.Request.Context(), int64(taskID)); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}

		core.Success(c, gin.H{
			"message": "任务已启用",
			"task_id": taskID,
		})
	}
}

// taskDisableHandler POST /:id/disable - 禁用任务
func taskDisableHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Scheduler == nil {
			core.FailWithCode(c, core.ErrSchedulerNotRunning)
			return
		}

		idParam := c.Param("id")
		taskID, err := strconv.Atoi(idParam)
		if err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, "无效的任务 ID")
			return
		}

		if err := deps.Scheduler.DisableTask(c.Request.Context(), int64(taskID)); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}

		core.Success(c, gin.H{
			"message": "任务已禁用",
			"task_id": taskID,
		})
	}
}

// ============ System Info Handlers ============

// systemInfoHandler GET /info - 获取系统信息
func systemInfoHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取内存统计
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		// 计算运行时长
		uptime := time.Since(startTime)

		// 构建响应数据
		info := gin.H{
			"runtime": gin.H{
				"go_version":    runtime.Version(),
				"num_goroutine": runtime.NumGoroutine(),
				"num_cpu":       runtime.NumCPU(),
				"gomaxprocs":    runtime.GOMAXPROCS(0),
			},
			"memory": gin.H{
				"alloc":       core.FormatMemorySize(int64(mem.Alloc)),
				"total_alloc": core.FormatMemorySize(int64(mem.TotalAlloc)),
				"sys":         core.FormatMemorySize(int64(mem.Sys)),
				"heap_alloc":  core.FormatMemorySize(int64(mem.HeapAlloc)),
				"heap_sys":    core.FormatMemorySize(int64(mem.HeapSys)),
				"gc_cycles":   mem.NumGC,
			},
			"uptime": gin.H{
				"start_time": startTime.Format(time.RFC3339),
				"duration":   uptime.String(),
				"seconds":    int64(uptime.Seconds()),
			},
		}

		// 获取对象池统计
		if deps.TemplateFuncs != nil {
			info["pools"] = deps.TemplateFuncs.GetPoolStats()
		}

		// 获取数据池统计
		if deps.PoolManager != nil {
			info["data"] = deps.PoolManager.GetPoolStatsSimple()
		}

		// 获取模板分析统计
		if deps.TemplateAnalyzer != nil {
			info["templates"] = deps.TemplateAnalyzer.GetStats()
		}

		core.Success(c, info)
	}
}

// systemHealthHandler GET /health - 获取系统健康状态
func systemHealthHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := "healthy"
		checks := make(map[string]gin.H)

		// 检查对象池
		if deps.TemplateFuncs != nil {
			poolStats := deps.TemplateFuncs.GetPoolStats()
			if poolStats == nil {
				checks["object_pool"] = gin.H{
					"status":  "unhealthy",
					"message": "对象池统计为空",
				}
				status = "degraded"
			} else {
				checks["object_pool"] = gin.H{
					"status":  "healthy",
					"message": "对象池正常运行",
				}
			}
		} else {
			checks["object_pool"] = gin.H{
				"status":  "unhealthy",
				"message": "对象池管理器未初始化",
			}
			status = "degraded"
		}

		// 检查数据池
		if deps.PoolManager != nil {
			dataStats := deps.PoolManager.GetPoolStatsSimple()
			if dataStats.Keywords == 0 && dataStats.Images == 0 {
				checks["data_pool"] = gin.H{
					"status":  "degraded",
					"message": "数据池为空（关键词和图片数量为 0）",
				}
				status = "degraded"
			} else {
				checks["data_pool"] = gin.H{
					"status":   "healthy",
					"message":  "数据池正常运行",
					"keywords": dataStats.Keywords,
					"images":   dataStats.Images,
				}
			}
		} else {
			checks["data_pool"] = gin.H{
				"status":  "unhealthy",
				"message": "数据池管理器未初始化",
			}
			status = "degraded"
		}

		// 检查模板分析器
		if deps.TemplateAnalyzer != nil {
			templateStats := deps.TemplateAnalyzer.GetStats()
			templatesAnalyzed, ok := templateStats["templates_analyzed"].(int)
			if !ok || templatesAnalyzed == 0 {
				checks["templates"] = gin.H{
					"status":  "degraded",
					"message": "未分析任何模板",
				}
				status = "degraded"
			} else {
				checks["templates"] = gin.H{
					"status":  "healthy",
					"message": "模板分析器正常运行",
					"count":   templatesAnalyzed,
				}
			}
		} else {
			checks["templates"] = gin.H{
				"status":  "unhealthy",
				"message": "模板分析器未初始化",
			}
			status = "degraded"
		}

		core.Success(c, gin.H{
			"status":  status,
			"checks":  checks,
			"time":    time.Now().Format(time.RFC3339),
			"version": "1.0.0",
		})
	}
}

// ============ Monitor Handlers ============

// metricsHandler GET /metrics - 获取实时指标
func metricsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Monitor == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		snapshot := deps.Monitor.GetCurrentSnapshot()
		core.Success(c, snapshot)
	}
}

// metricsHistoryHandler GET /metrics/history - 获取历史指标
func metricsHistoryHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Monitor == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		// 获取 limit 参数，默认 60
		limit := 60
		if limitParam := c.Query("limit"); limitParam != "" {
			if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
				limit = l
			}
		}

		history := deps.Monitor.GetHistory(limit)
		core.Success(c, gin.H{
			"history": history,
			"total":   len(history),
			"limit":   limit,
		})
	}
}

// alertsHandler GET /alerts - 获取告警列表
func alertsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Monitor == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		// 获取 limit 参数，默认 50
		limit := 50
		if limitParam := c.Query("limit"); limitParam != "" {
			if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
				limit = l
			}
		}

		// 检查是否只获取未解决的告警
		var alerts []core.Alert
		if c.Query("unresolved") == "true" {
			alerts = deps.Monitor.GetUnresolvedAlerts()
			// 如果需要限制数量
			if limit > 0 && len(alerts) > limit {
				alerts = alerts[:limit]
			}
		} else {
			alerts = deps.Monitor.GetAlerts(limit)
		}

		core.Success(c, gin.H{
			"alerts": alerts,
			"total":  len(alerts),
			"limit":  limit,
		})
	}
}

// monitorStatsHandler GET /monitor - 获取监控统计
func monitorStatsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Monitor == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		stats := deps.Monitor.GetStats()
		core.Success(c, stats)
	}
}
