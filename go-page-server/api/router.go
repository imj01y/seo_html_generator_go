// Package api provides the admin API routes and handlers
package api

import (
	"context"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"go-page-server/config"
	"go-page-server/core"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// startTime 记录服务启动时间
var startTime = time.Now()

// Dependencies holds all dependencies required by the API handlers
type Dependencies struct {
	DB               *sqlx.DB
	Config           *config.Config
	TemplateAnalyzer *core.TemplateAnalyzer
	TemplateFuncs    *core.TemplateFuncsManager
	DataPoolManager  *core.DataPoolManager
	Scheduler        *core.Scheduler
	TemplateCache    *core.TemplateCache
	Monitor          *core.Monitor
}

// SetupRouter configures all API routes
func SetupRouter(r *gin.Engine, deps *Dependencies) {
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

	// Admin API group
	admin := r.Group("/api/admin")

	// Pool management routes
	pool := admin.Group("/pool")
	{
		pool.GET("/stats", poolStatsHandler(deps))
		pool.GET("/presets", poolPresetsHandler(deps))
		pool.GET("/preset/:name", poolPresetByNameHandler(deps))
		pool.POST("/preset/:name", poolPresetByNameHandler(deps))
		pool.POST("/resize", poolResizeHandler(deps))
		pool.POST("/warmup", poolWarmupHandler(deps))
		pool.POST("/clear", poolClearHandler(deps))
		pool.POST("/pause", poolPauseHandler(deps))
		pool.POST("/resume", poolResumeHandler(deps))
	}

	// Template analysis routes
	template := admin.Group("/template")
	{
		template.GET("/analysis", templateAnalysisListHandler(deps))
		template.GET("/analysis/:id", templateAnalysisByIDHandler(deps))
		template.POST("/analyze/:id", templateAnalyzeHandler(deps))
		template.GET("/pool-config", templatePoolConfigHandler(deps))
	}

	// Data pool routes
	data := admin.Group("/data")
	{
		data.GET("/stats", dataStatsHandler(deps))
		data.GET("/seo", dataSEOHandler(deps))
		data.GET("/recommendations", dataRecommendationsHandler(deps))
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
			Key            string `json:"key"`
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
		deps.TemplateFuncs.WarmupPools(0.5)

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

// poolWarmupRequest 池预热请求
type poolWarmupRequest struct {
	Percent float64 `json:"percent"`
}

// poolWarmupHandler POST /warmup - 预热池
func poolWarmupHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req poolWarmupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			// 使用默认值 0.5
			req.Percent = 0.5
		}

		// 验证百分比范围
		if req.Percent <= 0 || req.Percent > 1 {
			req.Percent = 0.5
		}

		if deps.TemplateFuncs == nil {
			core.FailWithCode(c, core.ErrPoolInvalid)
			return
		}

		deps.TemplateFuncs.WarmupPools(req.Percent)

		core.Success(c, gin.H{
			"message": "池预热已启动",
			"percent": req.Percent,
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

// poolPauseHandler POST /pause - 暂停池补充
func poolPauseHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateFuncs == nil {
			core.FailWithCode(c, core.ErrPoolInvalid)
			return
		}

		deps.TemplateFuncs.PausePools()

		core.Success(c, gin.H{
			"message": "池补充已暂停",
		})
	}
}

// poolResumeHandler POST /resume - 恢复池补充
func poolResumeHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateFuncs == nil {
			core.FailWithCode(c, core.ErrPoolInvalid)
			return
		}

		deps.TemplateFuncs.ResumePools()

		core.Success(c, gin.H{
			"message": "池补充已恢复",
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

// templatePoolConfigHandler GET /pool-config - 获取推荐的池配置
func templatePoolConfigHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.TemplateAnalyzer == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		config := deps.TemplateAnalyzer.CalculatePoolSize()
		memoryEstimate := core.EstimateMemoryUsage(config)
		memoryHuman := core.FormatMemorySize(memoryEstimate)

		core.Success(c, gin.H{
			"config":          config,
			"memory_estimate": memoryEstimate,
			"memory_human":    memoryHuman,
		})
	}
}

// ============ Data Pool Handlers ============

// dataStatsHandler GET /stats - 获取数据池详细统计
func dataStatsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.DataPoolManager == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		stats := deps.DataPoolManager.GetDetailedStats()
		core.Success(c, stats)
	}
}

// dataSEOHandler GET /seo - 获取 SEO 分析结果
func dataSEOHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.DataPoolManager == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}
		if deps.TemplateAnalyzer == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		analysis := deps.DataPoolManager.AnalyzeSEO(deps.TemplateAnalyzer)
		core.Success(c, analysis)
	}
}

// dataRecommendationsHandler GET /recommendations - 获取数据池优化建议
func dataRecommendationsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.DataPoolManager == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}
		if deps.TemplateAnalyzer == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		recommendations := deps.DataPoolManager.GetRecommendations(deps.TemplateAnalyzer)
		core.Success(c, recommendations)
	}
}

// dataRefreshRequest 数据刷新请求
type dataRefreshRequest struct {
	Pool string `json:"pool" binding:"required,oneof=all keywords images titles contents"`
}

// dataRefreshHandler POST /refresh - 刷新数据池
func dataRefreshHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.DataPoolManager == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		var req dataRefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, err.Error())
			return
		}

		// 创建带超时的上下文
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// 执行刷新
		if err := deps.DataPoolManager.Refresh(ctx, req.Pool); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}

		// 获取最新统计并返回
		stats := deps.DataPoolManager.GetStats()
		core.Success(c, gin.H{
			"message": "数据池刷新成功",
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
				"go_version":   runtime.Version(),
				"num_goroutine": runtime.NumGoroutine(),
				"num_cpu":      runtime.NumCPU(),
				"gomaxprocs":   runtime.GOMAXPROCS(0),
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
		if deps.DataPoolManager != nil {
			info["data"] = deps.DataPoolManager.GetStats()
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
		if deps.DataPoolManager != nil {
			dataStats := deps.DataPoolManager.GetStats()
			if dataStats.Keywords == 0 && dataStats.Images == 0 {
				checks["data_pool"] = gin.H{
					"status":  "degraded",
					"message": "数据池为空（关键词和图片数量为 0）",
				}
				status = "degraded"
			} else {
				checks["data_pool"] = gin.H{
					"status":  "healthy",
					"message": "数据池正常运行",
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
