package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	core "seo-generator/api/internal/service"
)

// DashboardHandler 仪表盘 handler
type DashboardHandler struct {
	db      *sqlx.DB
	monitor *core.Monitor
}

// NewDashboardHandler 创建 DashboardHandler
func NewDashboardHandler(db *sqlx.DB, monitor *core.Monitor) *DashboardHandler {
	return &DashboardHandler{db: db, monitor: monitor}
}

// Stats 获取仪表盘统计数据
// GET /api/dashboard/stats
func (h *DashboardHandler) Stats(c *gin.Context) {
	stats := make(map[string]interface{})

	if h.db == nil {
		core.Success(c, stats)
		return
	}

	// 站点数量
	var siteCount int
	if err := h.db.Get(&siteCount, "SELECT COUNT(*) FROM sites WHERE status = 1"); err != nil {
		log.Warn().Err(err).Msg("Failed to count sites")
	}
	stats["site_count"] = siteCount

	// 关键词数量
	var keywordCount int
	if err := h.db.Get(&keywordCount, "SELECT COUNT(*) FROM keywords WHERE status = 1"); err != nil {
		log.Warn().Err(err).Msg("Failed to count keywords")
	}
	stats["keyword_count"] = keywordCount

	// 图片数量
	var imageCount int
	if err := h.db.Get(&imageCount, "SELECT COUNT(*) FROM images WHERE status = 1"); err != nil {
		log.Warn().Err(err).Msg("Failed to count images")
	}
	stats["image_count"] = imageCount

	// 文章数量
	var articleCount int
	if err := h.db.Get(&articleCount, "SELECT COUNT(*) FROM original_articles WHERE status = 1"); err != nil {
		log.Warn().Err(err).Msg("Failed to count articles")
	}
	stats["article_count"] = articleCount

	// 模板数量
	var templateCount int
	if err := h.db.Get(&templateCount, "SELECT COUNT(*) FROM templates"); err != nil {
		log.Warn().Err(err).Msg("Failed to count templates")
	}
	stats["template_count"] = templateCount

	core.Success(c, stats)
}

// SpiderVisits 获取蜘蛛访问统计
// GET /api/dashboard/spider-visits
func (h *DashboardHandler) SpiderVisits(c *gin.Context) {
	var total int
	byType := make(map[string]int)

	if h.db != nil {
		// 总访问次数
		h.db.Get(&total, "SELECT COUNT(*) FROM spider_logs")

		// 按蜘蛛类型统计
		var typeStats []struct {
			SpiderType string `db:"spider_type"`
			Count      int    `db:"count"`
		}
		err := h.db.Select(&typeStats, `
			SELECT spider_type, COUNT(*) as count
			FROM spider_logs
			GROUP BY spider_type
			ORDER BY count DESC
		`)
		if err == nil {
			for _, ts := range typeStats {
				byType[ts.SpiderType] = ts.Count
			}
		}
	}

	// 返回前端期望的格式: { total, by_type }
	core.Success(c, gin.H{
		"total":   total,
		"by_type": byType,
	})
}

// CacheStats 获取缓存统计
// GET /api/dashboard/cache-stats
func (h *DashboardHandler) CacheStats(c *gin.Context) {
	if h.monitor != nil {
		snapshot := h.monitor.GetCurrentSnapshot()
		core.Success(c, gin.H{
			"cache_hits":   snapshot.CacheHits,
			"cache_misses": snapshot.CacheMisses,
			"hit_rate":     snapshot.CacheHitRate,
		})
		return
	}

	core.Success(c, gin.H{
		"cache_hits":   0,
		"cache_misses": 0,
		"hit_rate":     0.0,
	})
}
