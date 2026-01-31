package api

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// SpiderDetectorHandler 蜘蛛检测处理器
type SpiderDetectorHandler struct{}

// GetSpiderConfig 获取蜘蛛检测配置
func (h *SpiderDetectorHandler) GetSpiderConfig(c *gin.Context) {
	cfg, exists := c.Get("config")
	if !exists {
		c.JSON(200, gin.H{
			"enabled":            true,
			"dns_verify_enabled": false,
			"dns_verify_types":   []string{},
			"dns_timeout":        2.0,
		})
		return
	}

	// 简化返回
	c.JSON(200, gin.H{
		"success": true,
		"config":  cfg,
	})
}

// TestSpiderDetection 测试蜘蛛检测
func (h *SpiderDetectorHandler) TestSpiderDetection(c *gin.Context) {
	userAgent := c.Query("user_agent")
	if userAgent == "" {
		c.JSON(400, gin.H{"success": false, "message": "需要提供 user_agent 参数"})
		return
	}

	// 简单的蜘蛛检测逻辑
	spiderKeywords := map[string]string{
		"baiduspider":    "baidu",
		"googlebot":      "google",
		"bingbot":        "bing",
		"sogou":          "sogou",
		"360spider":      "360",
		"bytespider":     "toutiao",
		"yandexbot":      "yandex",
		"duckduckbot":    "duckduckgo",
	}

	isSpider := false
	spiderType := ""
	spiderName := ""

	lowerUA := userAgent
	for keyword, stype := range spiderKeywords {
		if contains(lowerUA, keyword) {
			isSpider = true
			spiderType = stype
			spiderName = keyword
			break
		}
	}

	c.JSON(200, gin.H{
		"is_spider":   isSpider,
		"spider_type": spiderType,
		"spider_name": spiderName,
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, i, substr) {
			return true
		}
	}
	return false
}

func equalFoldAt(s string, i int, substr string) bool {
	for j := 0; j < len(substr); j++ {
		c1 := s[i+j]
		c2 := substr[j]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// GetSpiderLogs 获取蜘蛛访问日志
func (h *SpiderDetectorHandler) GetSpiderLogs(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	spiderType := c.Query("spider_type")
	domain := c.Query("domain")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	where := "1=1"
	args := []interface{}{}

	if spiderType != "" {
		where += " AND spider_type = ?"
		args = append(args, spiderType)
	}
	if domain != "" {
		where += " AND domain = ?"
		args = append(args, domain)
	}
	if startDate != "" {
		where += " AND DATE(created_at) >= ?"
		args = append(args, startDate)
	}
	if endDate != "" {
		where += " AND DATE(created_at) <= ?"
		args = append(args, endDate)
	}

	var total int
	sqlxDB.Get(&total, "SELECT COUNT(*) FROM spider_logs WHERE "+where, args...)

	args = append(args, pageSize, offset)
	var logs []SpiderLogItem
	err := sqlxDB.Select(&logs, `
		SELECT id, spider_type, ip, ua, domain, path, dns_ok, resp_time, cache_hit, status, created_at
		FROM spider_logs
		WHERE `+where+`
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, args...)

	if err != nil || logs == nil {
		logs = []SpiderLogItem{}
	}

	c.JSON(200, gin.H{
		"items":     logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetSpiderStats 获取蜘蛛详细统计信息
func (h *SpiderDetectorHandler) GetSpiderStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var total, todayTotal int
	sqlxDB.Get(&total, "SELECT COUNT(*) FROM spider_logs")
	sqlxDB.Get(&todayTotal, "SELECT COUNT(*) FROM spider_logs WHERE DATE(created_at) = CURDATE()")

	// 按类型统计
	var byType []struct {
		SpiderType string `db:"spider_type"`
		Count      int    `db:"count"`
	}
	sqlxDB.Select(&byType, `
		SELECT spider_type, COUNT(*) as count
		FROM spider_logs
		GROUP BY spider_type
		ORDER BY count DESC
	`)

	// 按域名统计
	var byDomain []struct {
		Domain string `db:"domain"`
		Count  int    `db:"count"`
	}
	sqlxDB.Select(&byDomain, `
		SELECT domain, COUNT(*) as count
		FROM spider_logs
		GROUP BY domain
		ORDER BY count DESC
		LIMIT 10
	`)

	// 按状态码统计
	var byStatus []struct {
		Status int `db:"status"`
		Count  int `db:"count"`
	}
	sqlxDB.Select(&byStatus, `
		SELECT status, COUNT(*) as count
		FROM spider_logs
		GROUP BY status
		ORDER BY status
	`)

	// 缓存命中统计
	var cacheHitCount, cacheMissCount int
	sqlxDB.Get(&cacheHitCount, "SELECT COUNT(*) FROM spider_logs WHERE cache_hit = 1")
	sqlxDB.Get(&cacheMissCount, "SELECT COUNT(*) FROM spider_logs WHERE cache_hit = 0")

	hitRate := float64(0)
	if cacheHitCount+cacheMissCount > 0 {
		hitRate = float64(cacheHitCount) / float64(cacheHitCount+cacheMissCount) * 100
	}

	// 平均响应时间
	var avgResponse float64
	sqlxDB.Get(&avgResponse, "SELECT COALESCE(AVG(resp_time), 0) FROM spider_logs")

	c.JSON(200, gin.H{
		"total":                total,
		"today_total":          todayTotal,
		"by_type":              byType,
		"by_domain":            byDomain,
		"by_status":            byStatus,
		"cache_hit_rate":       hitRate,
		"cache_hit_count":      cacheHitCount,
		"cache_miss_count":     cacheMissCount,
		"avg_response_time_ms": avgResponse,
	})
}

// GetSpiderDailyStats 获取每日蜘蛛访问统计
func (h *SpiderDetectorHandler) GetSpiderDailyStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days < 1 || days > 30 {
		days = 7
	}

	var stats []struct {
		Date            time.Time `db:"date"`
		SpiderType      string    `db:"spider_type"`
		Count           int       `db:"count"`
		CacheHits       int       `db:"cache_hits"`
		AvgResponseTime float64   `db:"avg_response_time"`
	}

	sqlxDB.Select(&stats, `
		SELECT
			DATE(created_at) as date,
			spider_type,
			COUNT(*) as count,
			SUM(CASE WHEN cache_hit = 1 THEN 1 ELSE 0 END) as cache_hits,
			AVG(resp_time) as avg_response_time
		FROM spider_logs
		WHERE created_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
		GROUP BY DATE(created_at), spider_type
		ORDER BY date DESC, count DESC
	`, days)

	// 按日期组织结果
	result := make(map[string]gin.H)
	for _, row := range stats {
		dateStr := row.Date.Format("2006-01-02")
		if _, exists := result[dateStr]; !exists {
			result[dateStr] = gin.H{
				"date":    dateStr,
				"total":   0,
				"by_type": gin.H{},
			}
		}
		result[dateStr]["total"] = result[dateStr]["total"].(int) + row.Count
		byType := result[dateStr]["by_type"].(gin.H)
		byType[row.SpiderType] = gin.H{
			"count":             row.Count,
			"cache_hits":        row.CacheHits,
			"avg_response_time": row.AvgResponseTime,
		}
	}

	days_list := make([]gin.H, 0, len(result))
	for _, v := range result {
		days_list = append(days_list, v)
	}

	c.JSON(200, gin.H{"days": days_list})
}

// GetSpiderHourlyStats 获取按小时的蜘蛛访问统计
func (h *SpiderDetectorHandler) GetSpiderHourlyStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	date := c.Query("date")
	dateCondition := "DATE(created_at) = CURDATE()"
	args := []interface{}{}
	if date != "" {
		dateCondition = "DATE(created_at) = ?"
		args = append(args, date)
	}

	var stats []struct {
		Hour       int    `db:"hour"`
		SpiderType string `db:"spider_type"`
		Count      int    `db:"count"`
	}

	sqlxDB.Select(&stats, `
		SELECT
			HOUR(created_at) as hour,
			spider_type,
			COUNT(*) as count
		FROM spider_logs
		WHERE `+dateCondition+`
		GROUP BY HOUR(created_at), spider_type
		ORDER BY hour ASC
	`, args...)

	// 初始化 24 小时结果
	result := make([]gin.H, 24)
	for h := 0; h < 24; h++ {
		result[h] = gin.H{
			"hour":    h,
			"total":   0,
			"by_type": gin.H{},
		}
	}

	// 填充数据
	for _, row := range stats {
		if row.Hour >= 0 && row.Hour < 24 {
			result[row.Hour]["total"] = result[row.Hour]["total"].(int) + row.Count
			byType := result[row.Hour]["by_type"].(gin.H)
			byType[row.SpiderType] = row.Count
		}
	}

	c.JSON(200, gin.H{"hours": result})
}

// ClearSpiderLogs 清理旧的蜘蛛日志
func (h *SpiderDetectorHandler) ClearSpiderLogs(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	beforeDays, _ := strconv.Atoi(c.DefaultQuery("before_days", "30"))
	if beforeDays < 1 {
		beforeDays = 30
	}

	result, err := sqlxDB.Exec(
		"DELETE FROM spider_logs WHERE created_at < DATE_SUB(NOW(), INTERVAL ? DAY)",
		beforeDays,
	)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	c.JSON(200, gin.H{
		"success": true,
		"deleted": affected,
		"message": "已清理 " + strconv.Itoa(beforeDays) + " 天前的日志",
	})
}
