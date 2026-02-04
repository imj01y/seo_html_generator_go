package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	models "seo-generator/api/internal/model"
	core "seo-generator/api/internal/service"
)

// SpiderDetectorHandler 蜘蛛检测处理器
type SpiderDetectorHandler struct{}

// GetSpiderConfig 获取蜘蛛检测配置
// GET /api/spiders/config
func (h *SpiderDetectorHandler) GetSpiderConfig(c *gin.Context) {
	detector := core.GetSpiderDetector()
	if detector == nil {
		core.Success(c, gin.H{
			"spiders": []interface{}{},
			"enabled": false,
		})
		return
	}

	// 获取所有蜘蛛类型
	types := detector.GetAllSpiderTypes()
	spiders := make([]gin.H, 0, len(types))

	for _, spiderType := range types {
		info := detector.GetSpiderInfo(spiderType)
		if info != nil {
			spiders = append(spiders, gin.H{
				"type":        spiderType,
				"name":        info.Name,
				"dns_domains": info.DNSDomains,
			})
		}
	}

	// 获取缓存统计
	stats := detector.GetStats()

	core.Success(c, gin.H{
		"spiders": spiders,
		"enabled": true,
		"stats":   stats,
	})
}

// TestSpiderDetection 测试蜘蛛检测
// POST /api/spiders/test
func (h *SpiderDetectorHandler) TestSpiderDetection(c *gin.Context) {
	var req struct {
		UserAgent string `json:"user_agent" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请提供 user_agent")
		return
	}

	detector := core.GetSpiderDetector()
	if detector == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "蜘蛛检测器未初始化")
		return
	}

	result := detector.Detect(req.UserAgent)
	core.Success(c, result)
}

// GetSpiderLogs 获取蜘蛛访问日志
// GET /api/spiders/logs
func (h *SpiderDetectorHandler) GetSpiderLogs(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		core.Success(c, gin.H{"items": []interface{}{}, "total": 0})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	spiderType := c.Query("spider_type")
	domain := c.Query("domain")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

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

	var total int
	countQuery := "SELECT COUNT(*) FROM spider_logs WHERE " + where
	sqlxDB.Get(&total, countQuery, args...)

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	var logs []models.SpiderLog
	query := `
		SELECT id, spider_type, ip, ua, domain, path, dns_ok, resp_time, cache_hit, status, created_at
		FROM spider_logs
		WHERE ` + where + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	sqlxDB.Select(&logs, query, args...)

	if logs == nil {
		logs = []models.SpiderLog{}
	}

	// 前端期望 items 而不是 data
	core.Success(c, gin.H{
		"items":     logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetSpiderStats 获取蜘蛛统计概览
// GET /api/spiders/stats
func (h *SpiderDetectorHandler) GetSpiderStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		core.Success(c, gin.H{
			"total":   0,
			"by_type": map[string]int{},
		})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var total int
	sqlxDB.Get(&total, "SELECT COUNT(*) FROM spider_logs")

	var typeStats []struct {
		SpiderType string `db:"spider_type"`
		Count      int    `db:"count"`
	}
	sqlxDB.Select(&typeStats, `
		SELECT spider_type, COUNT(*) as count
		FROM spider_logs
		GROUP BY spider_type
		ORDER BY count DESC
	`)

	byType := make(map[string]int)
	for _, ts := range typeStats {
		byType[ts.SpiderType] = ts.Count
	}

	core.Success(c, gin.H{
		"total":   total,
		"by_type": byType,
	})
}

// GetSpiderDailyStats 获取每日统计
// GET /api/spiders/daily-stats
func (h *SpiderDetectorHandler) GetSpiderDailyStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		core.Success(c, gin.H{"days": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days < 1 || days > 90 {
		days = 7
	}

	spiderType := c.Query("spider_type")

	where := "created_at >= DATE_SUB(NOW(), INTERVAL ? DAY)"
	args := []interface{}{days}

	if spiderType != "" {
		where += " AND spider_type = ?"
		args = append(args, spiderType)
	}

	var stats []struct {
		Date  string `db:"date" json:"date"`
		Total int    `db:"total" json:"total"`
	}

	query := `
		SELECT DATE(created_at) as date, COUNT(*) as total
		FROM spider_logs
		WHERE ` + where + `
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	sqlxDB.Select(&stats, query, args...)

	if stats == nil {
		stats = []struct {
			Date  string `db:"date" json:"date"`
			Total int    `db:"total" json:"total"`
		}{}
	}

	// 前端期望 days 而不是 data，字段 total 而不是 count
	core.Success(c, gin.H{"days": stats})
}

// GetSpiderHourlyStats 获取每小时统计
// GET /api/spiders/hourly-stats
func (h *SpiderDetectorHandler) GetSpiderHourlyStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		core.Success(c, gin.H{"hours": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	hoursParam, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if hoursParam < 1 || hoursParam > 168 {
		hoursParam = 24
	}

	spiderType := c.Query("spider_type")

	where := "created_at >= DATE_SUB(NOW(), INTERVAL ? HOUR)"
	args := []interface{}{hoursParam}

	if spiderType != "" {
		where += " AND spider_type = ?"
		args = append(args, spiderType)
	}

	var stats []struct {
		Hour  int `db:"hour" json:"hour"`
		Total int `db:"total" json:"total"`
	}

	// 前端期望 hour 是小时数字 (0-23)，total 是数量
	query := `
		SELECT HOUR(created_at) as hour, COUNT(*) as total
		FROM spider_logs
		WHERE ` + where + `
		GROUP BY HOUR(created_at)
		ORDER BY hour ASC
	`
	sqlxDB.Select(&stats, query, args...)

	if stats == nil {
		stats = []struct {
			Hour  int `db:"hour" json:"hour"`
			Total int `db:"total" json:"total"`
		}{}
	}

	// 前端期望 hours 而不是 data
	core.Success(c, gin.H{"hours": stats})
}

// ClearSpiderLogs 清空蜘蛛日志
// DELETE /api/spiders/logs/clear
func (h *SpiderDetectorHandler) ClearSpiderLogs(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未连接")
		return
	}
	sqlxDB := db.(*sqlx.DB)

	spiderType := c.Query("spider_type")
	beforeDays, _ := strconv.Atoi(c.Query("before_days"))

	where := "1=1"
	args := []interface{}{}

	if spiderType != "" {
		where += " AND spider_type = ?"
		args = append(args, spiderType)
	}
	if beforeDays > 0 {
		where += " AND created_at < DATE_SUB(NOW(), INTERVAL ? DAY)"
		args = append(args, beforeDays)
	}

	result, err := sqlxDB.Exec("DELETE FROM spider_logs WHERE "+where, args...)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "清空日志失败: "+err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	core.Success(c, gin.H{
		"message": "日志已清空",
		"deleted": affected,
	})
}
