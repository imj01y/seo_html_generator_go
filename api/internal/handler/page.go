// Package handlers contains HTTP request handlers
package api

import (
	"context"
	"database/sql"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"seo-generator/api/internal/model"
	core "seo-generator/api/internal/service"
	"seo-generator/api/pkg/config"
)

// PageHandler handles /page requests
type PageHandler struct {
	db               *sqlx.DB
	cfg              *config.Config
	spiderDetector   *core.SpiderDetector
	siteCache        *core.SiteCache
	templateCache    *core.TemplateCache
	htmlCache        *core.HTMLCache
	templateRenderer *core.TemplateRenderer
	funcsManager     *core.TemplateFuncsManager
	poolManager      *core.PoolManager
}

// NewPageHandler creates a new page handler
func NewPageHandler(
	db *sqlx.DB,
	cfg *config.Config,
	siteCache *core.SiteCache,
	templateCache *core.TemplateCache,
	htmlCache *core.HTMLCache,
	funcsManager *core.TemplateFuncsManager,
	poolManager *core.PoolManager,
) *PageHandler {
	return &PageHandler{
		db:               db,
		cfg:              cfg,
		spiderDetector:   core.GetSpiderDetector(),
		siteCache:        siteCache,
		templateCache:    templateCache,
		htmlCache:        htmlCache,
		templateRenderer: core.NewTemplateRenderer(funcsManager),
		funcsManager:     funcsManager,
		poolManager:      poolManager,
	}
}

// ServePage handles the /page endpoint
func (h *PageHandler) ServePage(c *gin.Context) {
	startTime := time.Now()

	// Get query parameters
	ua := c.Query("ua")
	path := c.Query("path")
	domain := c.Query("domain")

	// Validate required parameters
	if ua == "" || path == "" || domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters: ua, path, domain"})
		return
	}

	clientIP := getClientIP(c)

	// Spider detection
	t1 := time.Now()
	detection := h.spiderDetector.Detect(ua)
	spiderTime := time.Since(t1)

	// Non-spider handling
	if !detection.IsSpider {
		if h.cfg.SpiderDetector.Return404ForNonSpider {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<html><body>Hello</body></html>"))
		return
	}

	// Get site config
	t3 := time.Now()
	ctx := context.Background()
	site, err := h.siteCache.Get(ctx, domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain).Msg("Failed to get site config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if site == nil {
		log.Warn().Str("domain", domain).Msg("Domain not registered")
		c.JSON(http.StatusForbidden, gin.H{"error": "Domain not registered"})
		return
	}
	siteTime := time.Since(t3)

	// Get template content from cache (no DB query)
	t4 := time.Now()
	templateName := site.Template
	if templateName == "" {
		templateName = "download_site"
	}

	// Use templateCache for fast lookup
	templateData := h.templateCache.Get(templateName, site.SiteGroupID)
	if templateData == nil || templateData.Content == "" {
		// Fallback to DB query for newly added templates
		var err error
		templateData, err = h.templateCache.GetWithFallback(ctx, templateName, site.SiteGroupID)
		if err != nil || templateData == nil || templateData.Content == "" {
			log.Error().Err(err).Str("template", templateName).Msg("Template not found or empty")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Template not found"})
			return
		}
	}

	// Get keyword group ID
	keywordGroupID := 1
	if site.KeywordGroupID.Valid {
		keywordGroupID = int(site.KeywordGroupID.Int64)
	}

	// Get article group ID
	articleGroupID := 1
	if site.ArticleGroupID.Valid {
		articleGroupID = int(site.ArticleGroupID.Int64)
	}

	// Get image group ID
	imageGroupID := 1
	if site.ImageGroupID.Valid {
		imageGroupID = int(site.ImageGroupID.Int64)
	}

	// Get title and content from pool
	var title, content string
	title, err = h.poolManager.Pop("titles", keywordGroupID)
	if err != nil {
		log.Warn().Err(err).Int("group", keywordGroupID).Msg("Failed to get title from pool")
	}
	content, err = h.poolManager.Pop("contents", articleGroupID)
	if err != nil {
		log.Warn().Err(err).Int("group", articleGroupID).Msg("Failed to get content from pool")
	}
	// 获取关键词用于标题生成（使用关键词分组）
	titleKeywords := h.poolManager.GetRandomKeywords(keywordGroupID, 3)
	fetchTime := time.Since(t4)

	// Build article content using fetched title and content
	articleContent := core.BuildArticleContentFromSingle(title, content)


	// Prepare render data
	analyticsCode := getNullString(site.Analytics)
	baiduPushJS := ""
	if baiduToken := getNullString(site.BaiduToken); baiduToken != "" {
		baiduPushJS = generateBaiduPushJS(baiduToken)
	}

	// 创建标题生成器闭包，同一页面多次调用返回相同标题
	var cachedTitle string
	titleGenerator := func() string {
		if cachedTitle == "" {
			kws := h.poolManager.GetRandomKeywords(keywordGroupID, 3)
			cachedTitle = h.generateTitle(kws)
		}
		return cachedTitle
	}

	renderData := &core.RenderData{
		Title:          h.generateTitle(titleKeywords), // 兼容静态用途
		TitleGenerator: titleGenerator,                 // 动态生成器
		SiteID:         site.ID,
		KeywordGroupID: keywordGroupID,
		ImageGroupID:   imageGroupID,
		AnalyticsCode:  template.HTML(analyticsCode),
		BaiduPushJS:    template.HTML(baiduPushJS),
		ArticleContent: template.HTML(articleContent),
	}

	// Render template
	t5 := time.Now()
	html, err := h.templateRenderer.Render(templateData.Content, templateName, renderData, content)
	if err != nil {
		log.Error().Err(err).Str("template", templateName).Msg("Failed to render template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Render failed"})
		return
	}
	renderTime := time.Since(t5)

	// Cache the result asynchronously
	go func() {
		if err := h.htmlCache.Set(domain, path, html); err != nil {
			log.Warn().Err(err).Str("domain", domain).Str("path", path).Msg("Failed to cache HTML")
		}
	}()

	elapsed := time.Since(startTime)

	log.Info().
		Str("domain", domain).
		Str("path", path).
		Str("spider", detection.SpiderType).
		Dur("elapsed", elapsed).
		Msg("Page generated")

	log.Debug().
		Dur("spider_time", spiderTime).
		Dur("site_time", siteTime).
		Dur("fetch_time", fetchTime).
		Dur("render_time", renderTime).
		Dur("total", elapsed).
		Msg("Performance metrics")

	// Log spider visit asynchronously
	go h.logSpiderVisit(detection, clientIP, ua, domain, path, false, int(elapsed.Milliseconds()), 200)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// generateTitle 生成 SEO 优化的页面标题
// 格式: 关键词1 + Emoji1 + 关键词2 + Emoji2 + 关键词3
func (h *PageHandler) generateTitle(keywords []string) string {
	switch {
	case len(keywords) == 0:
		return "Welcome"
	case len(keywords) < 3:
		return keywords[0]
	}

	usedEmojis := make(map[string]bool, 2)
	var builder strings.Builder
	builder.Grow(100) // 预分配空间

	for i := 0; i < 3; i++ {
		builder.WriteString(keywords[i])
		// 在前两个关键词后添加 Emoji
		if i < 2 {
			if emoji := h.poolManager.GetRandomEmojiExclude(usedEmojis); emoji != "" {
				usedEmojis[emoji] = true
				builder.WriteString(emoji)
			}
		}
	}

	return builder.String()
}

// logSpiderVisit logs spider visit to database asynchronously
func (h *PageHandler) logSpiderVisit(
	detection *models.DetectionResult,
	ip, ua, domain, path string,
	cacheHit bool,
	respTime int,
	status int,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Truncate long values
	if len(ua) > 500 {
		ua = ua[:500]
	}
	if len(path) > 500 {
		path = path[:500]
	}

	spiderType := detection.SpiderType
	if spiderType == "" {
		spiderType = "unknown"
	}

	cacheHitInt := 0
	if cacheHit {
		cacheHitInt = 1
	}

	query := `INSERT INTO spider_logs (spider_type, ip, ua, domain, path, dns_ok, resp_time, cache_hit, status)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	log.Debug().
		Str("spider_type", spiderType).
		Str("ip", ip).
		Str("domain", domain).
		Str("path", path).
		Msg("Inserting spider log")

	_, err := h.db.ExecContext(ctx, query, spiderType, ip, ua, domain, path, 0, respTime, cacheHitInt, status)
	if err != nil {
		log.Error().Err(err).Msg("Failed to log spider visit")
	} else {
		log.Debug().Msg("Spider log inserted successfully")
	}
}

// getNullString 安全获取 sql.NullString 的值
func getNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// getClientIP gets the client's real IP address
func getClientIP(c *gin.Context) string {
	// Try X-Forwarded-For header
	forwardedFor := c.GetHeader("X-Forwarded-For")
	if forwardedFor != "" {
		// Take the first IP (original client)
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}

	// Try X-Real-IP header
	realIP := c.GetHeader("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to connection IP
	return c.ClientIP()
}

// generateBaiduPushJS generates Baidu push JavaScript code
func generateBaiduPushJS(token string) string {
	if token == "" {
		return ""
	}

	return `<script>
(function(){
    var bp = document.createElement('script');
    var curProtocol = window.location.protocol.split(':')[0];
    if (curProtocol === 'https') {
        bp.src = 'https://zz.bdstatic.com/linksubmit/push.js';
    } else {
        bp.src = 'http://push.zhanzhang.baidu.com/push.js';
    }
    var s = document.getElementsByTagName("script")[0];
    s.parentNode.insertBefore(bp, s);
})();
</script>`
}

// Health handles health check endpoint
func (h *PageHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// Stats handles stats endpoint
func (h *PageHandler) Stats(c *gin.Context) {
	stats := gin.H{
		"spider_detector":         h.spiderDetector.GetStats(),
		"site_cache":              h.siteCache.GetStats(),
		"html_cache":              h.htmlCache.GetStats(),
		"pool_manager":            h.poolManager.GetStats(),
		"template_compiled_cache": h.templateRenderer.GetCacheStats(),
	}
	if h.templateCache != nil {
		stats["template_cache"] = h.templateCache.GetStats()
	}
	c.JSON(http.StatusOK, stats)
}

// GetTemplateRenderer returns the template renderer for cache management
func (h *PageHandler) GetTemplateRenderer() *core.TemplateRenderer {
	return h.templateRenderer
}
