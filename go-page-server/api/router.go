// Package api provides the admin API routes and handlers
package api

import (
	"net/http"

	"go-page-server/core"

	"github.com/gin-gonic/gin"
)

// Dependencies holds all dependencies required by the API handlers
type Dependencies struct {
	TemplateAnalyzer *core.TemplateAnalyzer
	TemplateFuncs    *core.TemplateFuncsManager
	DataPoolManager  *core.DataPoolManager
	Scheduler        *core.Scheduler
	TemplateCache    *core.TemplateCache
}

// SetupRouter configures all API routes
func SetupRouter(r *gin.Engine, deps *Dependencies) {
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

// ============ Template Analysis Handlers (placeholders) ============

func templateAnalysisListHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.3
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func templateAnalysisByIDHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.3
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func templateAnalyzeHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.3
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func templatePoolConfigHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.3
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

// ============ Data Pool Handlers (placeholders) ============

func dataStatsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.4
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func dataSEOHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.4
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func dataRecommendationsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.4
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func dataRefreshHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.4
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

// ============ Task Management Handlers (placeholders) ============

func taskListHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.5
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func taskByIDHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.5
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func taskLogsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.5
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func taskTriggerHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.5
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func taskEnableHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.5
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func taskDisableHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.5
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

// ============ System Info Handlers (placeholders) ============

func systemInfoHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.6
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}

func systemHealthHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement in Task 7.6
		c.JSON(200, gin.H{"message": "not implemented"})
	}
}
