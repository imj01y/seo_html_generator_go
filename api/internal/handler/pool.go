// api/internal/handler/pool.go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	core "seo-generator/api/internal/service"
)

// PoolHandler handles pool-related API requests
type PoolHandler struct {
	db          *sqlx.DB
	poolManager *core.PoolManager
}

// NewPoolHandler creates a new pool handler
func NewPoolHandler(db *sqlx.DB, poolManager *core.PoolManager) *PoolHandler {
	return &PoolHandler{
		db:          db,
		poolManager: poolManager,
	}
}

// GetConfig returns current pool configuration
func (h *PoolHandler) GetConfig(c *gin.Context) {
	config, err := core.LoadCachePoolConfig(c.Request.Context(), h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}

// UpdateConfig updates pool configuration
func (h *PoolHandler) UpdateConfig(c *gin.Context) {
	var req struct {
		TitlesSize            int `json:"titles_size"`
		ContentsSize          int `json:"contents_size"`
		Threshold             int `json:"threshold"`
		RefillIntervalMs      int `json:"refill_interval_ms"`
		KeywordsSize          int `json:"keywords_size"`
		ImagesSize            int `json:"images_size"`
		RefreshIntervalMs     int `json:"refresh_interval_ms"`
		// 标题池
		TitlePoolSize         int     `json:"title_pool_size"`
		TitleWorkers          int     `json:"title_workers"`
		TitleRefillIntervalMs int     `json:"title_refill_interval_ms"`
		TitleThreshold        float64 `json:"title_threshold"`
		// cls类名池
		ClsPoolSize         int     `json:"cls_pool_size"`
		ClsWorkers          int     `json:"cls_workers"`
		ClsRefillIntervalMs int     `json:"cls_refill_interval_ms"`
		ClsThreshold        float64 `json:"cls_threshold"`
		// url池
		UrlPoolSize         int     `json:"url_pool_size"`
		UrlWorkers          int     `json:"url_workers"`
		UrlRefillIntervalMs int     `json:"url_refill_interval_ms"`
		UrlThreshold        float64 `json:"url_threshold"`
		// 关键词表情池
		KeywordEmojiPoolSize         int     `json:"keyword_emoji_pool_size"`
		KeywordEmojiWorkers          int     `json:"keyword_emoji_workers"`
		KeywordEmojiRefillIntervalMs int     `json:"keyword_emoji_refill_interval_ms"`
		KeywordEmojiThreshold        float64 `json:"keyword_emoji_threshold"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate
	if req.TitlesSize < 100 || req.TitlesSize > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "titles_size must be between 100 and 100000"})
		return
	}
	if req.ContentsSize < 100 || req.ContentsSize > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contents_size must be between 100 and 100000"})
		return
	}
	if req.Threshold < 10 || req.Threshold > req.TitlesSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "threshold must be between 10 and titles_size"})
		return
	}
	if req.RefillIntervalMs < 100 || req.RefillIntervalMs > 60000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refill_interval_ms must be between 100 and 60000"})
		return
	}

	// Validate title config
	if req.TitlePoolSize < 100000 || req.TitlePoolSize > 2000000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_pool_size must be between 100000 and 2000000"})
		return
	}
	if req.TitleWorkers < 1 || req.TitleWorkers > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_workers must be between 1 and 50"})
		return
	}
	if req.TitleRefillIntervalMs < 10 || req.TitleRefillIntervalMs > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_refill_interval_ms must be between 10 and 1000"})
		return
	}
	if req.TitleThreshold < 0.1 || req.TitleThreshold > 0.9 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_threshold must be between 0.1 and 0.9"})
		return
	}

	// Validate cls config
	if req.ClsPoolSize < 100000 || req.ClsPoolSize > 2000000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cls_pool_size must be between 100000 and 2000000"})
		return
	}
	if req.ClsWorkers < 1 || req.ClsWorkers > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cls_workers must be between 1 and 50"})
		return
	}
	if req.ClsRefillIntervalMs < 10 || req.ClsRefillIntervalMs > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cls_refill_interval_ms must be between 10 and 1000"})
		return
	}
	if req.ClsThreshold < 0.1 || req.ClsThreshold > 0.9 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cls_threshold must be between 0.1 and 0.9"})
		return
	}

	// Validate url config
	if req.UrlPoolSize < 100000 || req.UrlPoolSize > 2000000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url_pool_size must be between 100000 and 2000000"})
		return
	}
	if req.UrlWorkers < 1 || req.UrlWorkers > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url_workers must be between 1 and 50"})
		return
	}
	if req.UrlRefillIntervalMs < 10 || req.UrlRefillIntervalMs > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url_refill_interval_ms must be between 10 and 1000"})
		return
	}
	if req.UrlThreshold < 0.1 || req.UrlThreshold > 0.9 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url_threshold must be between 0.1 and 0.9"})
		return
	}

	// Validate keyword_emoji config
	if req.KeywordEmojiPoolSize < 100000 || req.KeywordEmojiPoolSize > 2000000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keyword_emoji_pool_size must be between 100000 and 2000000"})
		return
	}
	if req.KeywordEmojiWorkers < 1 || req.KeywordEmojiWorkers > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keyword_emoji_workers must be between 1 and 50"})
		return
	}
	if req.KeywordEmojiRefillIntervalMs < 10 || req.KeywordEmojiRefillIntervalMs > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keyword_emoji_refill_interval_ms must be between 10 and 1000"})
		return
	}
	if req.KeywordEmojiThreshold < 0.1 || req.KeywordEmojiThreshold > 0.9 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keyword_emoji_threshold must be between 0.1 and 0.9"})
		return
	}

	config := &core.CachePoolConfig{
		TitlesSize:            req.TitlesSize,
		ContentsSize:          req.ContentsSize,
		Threshold:             req.Threshold,
		RefillIntervalMs:      req.RefillIntervalMs,
		KeywordsSize:          req.KeywordsSize,
		ImagesSize:            req.ImagesSize,
		RefreshIntervalMs:     req.RefreshIntervalMs,
		// 标题池
		TitlePoolSize:         req.TitlePoolSize,
		TitleWorkers:          req.TitleWorkers,
		TitleRefillIntervalMs: req.TitleRefillIntervalMs,
		TitleThreshold:        req.TitleThreshold,
		// cls类名池
		ClsPoolSize:         req.ClsPoolSize,
		ClsWorkers:          req.ClsWorkers,
		ClsRefillIntervalMs: req.ClsRefillIntervalMs,
		ClsThreshold:        req.ClsThreshold,
		// url池
		UrlPoolSize:         req.UrlPoolSize,
		UrlWorkers:          req.UrlWorkers,
		UrlRefillIntervalMs: req.UrlRefillIntervalMs,
		UrlThreshold:        req.UrlThreshold,
		// 关键词表情池
		KeywordEmojiPoolSize:         req.KeywordEmojiPoolSize,
		KeywordEmojiWorkers:          req.KeywordEmojiWorkers,
		KeywordEmojiRefillIntervalMs: req.KeywordEmojiRefillIntervalMs,
		KeywordEmojiThreshold:        req.KeywordEmojiThreshold,
	}

	// Save to DB
	if err := core.SaveCachePoolConfig(c.Request.Context(), h.db, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reload pool manager
	if err := h.poolManager.Reload(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Config saved but reload failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  config,
	})
}

// GetStats returns pool statistics
func (h *PoolHandler) GetStats(c *gin.Context) {
	stats := h.poolManager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// Reload triggers a configuration reload
func (h *PoolHandler) Reload(c *gin.Context) {
	if err := h.poolManager.Reload(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
