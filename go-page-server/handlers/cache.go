// Package handlers contains HTTP request handlers
package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// CacheHandler 缓存管理处理器
type CacheHandler struct {
	htmlCache        *core.HTMLCache
	templateRenderer *core.TemplateRenderer
	siteCache        *core.SiteCache
	templateCache    *core.TemplateCache
}

// NewCacheHandler 创建缓存管理处理器
func NewCacheHandler(
	htmlCache *core.HTMLCache,
	templateRenderer *core.TemplateRenderer,
	siteCache *core.SiteCache,
	templateCache *core.TemplateCache,
) *CacheHandler {
	return &CacheHandler{
		htmlCache:        htmlCache,
		templateRenderer: templateRenderer,
		siteCache:        siteCache,
		templateCache:    templateCache,
	}
}

// ClearTemplateCache 清除模板缓存（模板内容更新时使用）
// POST /api/cache/template/clear
func (h *CacheHandler) ClearTemplateCache(c *gin.Context) {
	// 清除模板编译缓存
	h.templateRenderer.ClearCache()

	// 同时清除HTML缓存，因为模板变化后HTML需要重新生成
	htmlCount, err := h.htmlCache.Clear("")
	if err != nil {
		log.Error().Err(err).Msg("Failed to clear HTML cache")
	}

	log.Info().Int("html_cleared", htmlCount).Msg("Template cache cleared")

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"html_cleared": htmlCount,
		"message":      "模板缓存已清除",
	})
}

// ClearAllCache 清除所有缓存
// POST /api/cache/clear
func (h *CacheHandler) ClearAllCache(c *gin.Context) {
	htmlCount, err := h.htmlCache.Clear("")
	if err != nil {
		log.Error().Err(err).Msg("Failed to clear HTML cache")
	}

	h.templateRenderer.ClearCache()
	h.siteCache.InvalidateAll()
	if h.templateCache != nil {
		h.templateCache.InvalidateAll()
	}

	log.Info().Int("html_cleared", htmlCount).Msg("All caches cleared")

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"html_cleared": htmlCount,
		"message":      "所有缓存已清除",
	})
}

// ClearDomainCache 清除指定域名的缓存
// POST /api/cache/clear/:domain
func (h *CacheHandler) ClearDomainCache(c *gin.Context) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "域名不能为空",
		})
		return
	}

	htmlCount, err := h.htmlCache.Clear(domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain).Msg("Failed to clear domain cache")
	}

	h.siteCache.Invalidate(domain)

	log.Info().Str("domain", domain).Int("html_cleared", htmlCount).Msg("Domain cache cleared")

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"domain":       domain,
		"html_cleared": htmlCount,
		"message":      "域名缓存已清除",
	})
}

// ReloadAllSites 重新加载所有站点缓存
// POST /api/cache/site/reload
func (h *CacheHandler) ReloadAllSites(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.siteCache.ReloadAll(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to reload all sites")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	stats := h.siteCache.GetStats()
	log.Info().Interface("stats", stats).Msg("All sites cache reloaded")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
		"message": "所有站点缓存已重新加载",
	})
}

// ReloadSite 重新加载指定站点缓存
// POST /api/cache/site/reload/:domain
func (h *CacheHandler) ReloadSite(c *gin.Context) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "域名不能为空",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.siteCache.Reload(ctx, domain); err != nil {
		log.Error().Err(err).Str("domain", domain).Msg("Failed to reload site")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Info().Str("domain", domain).Msg("Site cache reloaded")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"domain":  domain,
		"message": "站点缓存已重新加载",
	})
}

// ReloadAllTemplates 重新加载所有模板缓存
// POST /api/cache/template/reload
func (h *CacheHandler) ReloadAllTemplates(c *gin.Context) {
	if h.templateCache == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Template cache not initialized",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.templateCache.ReloadAll(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to reload all templates")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Also clear the compiled template cache
	h.templateRenderer.ClearCache()

	stats := h.templateCache.GetStats()
	log.Info().Interface("stats", stats).Msg("All templates cache reloaded")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
		"message": "所有模板缓存已重新加载",
	})
}

// ReloadTemplate 重新加载指定模板缓存
// POST /api/cache/template/reload/:name
func (h *CacheHandler) ReloadTemplate(c *gin.Context) {
	if h.templateCache == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Template cache not initialized",
		})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "模板名称不能为空",
		})
		return
	}

	// Optional site_group_id parameter
	siteGroupIDStr := c.Query("site_group_id")
	var siteGroupID int
	if siteGroupIDStr != "" {
		var err error
		siteGroupID, err = strconv.Atoi(siteGroupIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "无效的 site_group_id",
			})
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	if siteGroupID > 0 {
		// Reload specific version
		err = h.templateCache.Reload(ctx, name, siteGroupID)
	} else {
		// Reload all versions of this template
		err = h.templateCache.ReloadByName(ctx, name)
	}

	if err != nil {
		log.Error().Err(err).Str("name", name).Int("site_group_id", siteGroupID).Msg("Failed to reload template")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Also clear the compiled template cache for this template
	h.templateRenderer.ClearCache()

	log.Info().Str("name", name).Int("site_group_id", siteGroupID).Msg("Template cache reloaded")

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"name":          name,
		"site_group_id": siteGroupID,
		"message":       "模板缓存已重新加载",
	})
}
