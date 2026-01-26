// Package handlers contains HTTP request handlers
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// CacheHandler 缓存管理处理器
type CacheHandler struct {
	htmlCache        *core.HTMLCache
	templateRenderer *core.TemplateRenderer
	siteCache        *core.SiteCache
}

// NewCacheHandler 创建缓存管理处理器
func NewCacheHandler(
	htmlCache *core.HTMLCache,
	templateRenderer *core.TemplateRenderer,
	siteCache *core.SiteCache,
) *CacheHandler {
	return &CacheHandler{
		htmlCache:        htmlCache,
		templateRenderer: templateRenderer,
		siteCache:        siteCache,
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
