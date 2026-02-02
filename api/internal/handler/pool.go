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
		TitlePoolSize         int `json:"title_pool_size"`
		TitleWorkers          int `json:"title_workers"`
		TitleRefillIntervalMs int `json:"title_refill_interval_ms"`
		TitleThreshold        int `json:"title_threshold"`
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
	if req.TitlePoolSize < 100 || req.TitlePoolSize > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_pool_size must be between 100 and 100000"})
		return
	}
	if req.TitleWorkers < 1 || req.TitleWorkers > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_workers must be between 1 and 10"})
		return
	}
	if req.TitleRefillIntervalMs < 100 || req.TitleRefillIntervalMs > 60000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_refill_interval_ms must be between 100 and 60000"})
		return
	}
	if req.TitleThreshold < 10 || req.TitleThreshold > req.TitlePoolSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_threshold must be between 10 and title_pool_size"})
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
		TitlePoolSize:         req.TitlePoolSize,
		TitleWorkers:          req.TitleWorkers,
		TitleRefillIntervalMs: req.TitleRefillIntervalMs,
		TitleThreshold:        req.TitleThreshold,
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
