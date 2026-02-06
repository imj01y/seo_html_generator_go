package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"seo-generator/api/internal/model"
)

// SpiderStatsHandler 爬虫统计处理器
type SpiderStatsHandler struct{}

// GetRealtimeStats 获取实时统计
func (h *SpiderStatsHandler) GetRealtimeStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": models.SpiderStats{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": models.SpiderStats{}})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	ctx := context.Background()
	statsKey := fmt.Sprintf("spider:stats:%d", id)
	statsData, err := redisClient.HGetAll(ctx, statsKey).Result()

	stats := models.SpiderStats{Status: status}
	if err == nil && len(statsData) > 0 {
		stats.Total, _ = strconv.Atoi(statsData["total"])
		stats.Completed, _ = strconv.Atoi(statsData["completed"])
		stats.Failed, _ = strconv.Atoi(statsData["failed"])
		stats.Retried, _ = strconv.Atoi(statsData["retried"])
		stats.Pending, _ = strconv.Atoi(statsData["pending"])
		stats.Processing, _ = strconv.Atoi(statsData["processing"])

		totalDone := stats.Completed + stats.Failed
		if totalDone > 0 {
			stats.SuccessRate = math.Round(float64(stats.Completed)/float64(totalDone)*10000) / 100
		}
	}

	c.JSON(200, gin.H{"success": true, "data": stats})
}

// GetChartStats 获取历史图表数据
func (h *SpiderStatsHandler) GetChartStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	period := c.DefaultQuery("period", "hour")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	var data []models.StatsChartPoint
	err := sqlxDB.Select(&data, `
		SELECT period_start as time, total, completed, failed, retried, avg_speed
		FROM spider_stats_history
		WHERE project_id = ? AND period_type = ?
		ORDER BY period_start DESC
		LIMIT ?
	`, id, period, limit)

	if err != nil || data == nil {
		data = []models.StatsChartPoint{}
	}

	c.JSON(200, gin.H{"success": true, "data": data})
}

// ClearQueue 清空队列
func (h *SpiderStatsHandler) ClearQueue(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，请先停止"})
		return
	}

	ctx := context.Background()
	keys := []string{
		fmt.Sprintf("spider:queue:%d", id),
		fmt.Sprintf("spider:seen:%d", id),
		fmt.Sprintf("spider:stats:%d", id),
	}
	redisClient.Del(ctx, keys...)

	c.JSON(200, gin.H{"success": true, "message": "队列已清空"})
}

// ListFailed 获取失败请求列表
func (h *SpiderStatsHandler) ListFailed(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}, "total": 0})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	where := "project_id = ?"
	args := []interface{}{id}

	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	var total int
	sqlxDB.Get(&total, "SELECT COUNT(*) FROM spider_failed_requests WHERE "+where, args...)

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	var data []models.SpiderFailedRequest
	sqlxDB.Select(&data, `
		SELECT id, project_id, url, method, callback, meta, error_message,
		       retry_count, failed_at, status
		FROM spider_failed_requests
		WHERE `+where+`
		ORDER BY failed_at DESC
		LIMIT ? OFFSET ?
	`, args...)

	c.JSON(200, gin.H{"success": true, "data": data, "total": total})
}

// GetFailedStats 获取失败统计
func (h *SpiderStatsHandler) GetFailedStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": map[string]int{"pending": 0, "retried": 0, "ignored": 0, "total": 0}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var stats struct {
		Pending int `db:"pending"`
		Retried int `db:"retried"`
		Ignored int `db:"ignored"`
	}

	sqlxDB.Get(&stats, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending,
			COALESCE(SUM(CASE WHEN status = 'retried' THEN 1 ELSE 0 END), 0) as retried,
			COALESCE(SUM(CASE WHEN status = 'ignored' THEN 1 ELSE 0 END), 0) as ignored
		FROM spider_failed_requests WHERE project_id = ?
	`, id)

	c.JSON(200, gin.H{
		"success": true,
		"data": map[string]int{
			"pending": stats.Pending,
			"retried": stats.Retried,
			"ignored": stats.Ignored,
			"total":   stats.Pending + stats.Retried + stats.Ignored,
		},
	})
}

// RetryAllFailed 重试所有失败请求
func (h *SpiderStatsHandler) RetryAllFailed(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	var failed []models.SpiderFailedRequest
	sqlxDB.Select(&failed, `
		SELECT id, url, method, callback, meta
		FROM spider_failed_requests
		WHERE project_id = ? AND status = 'pending'
	`, id)

	ctx := context.Background()
	queueKey := fmt.Sprintf("spider:queue:%d", id)

	count := 0
	for _, f := range failed {
		reqData, _ := json.Marshal(map[string]interface{}{
			"url":      f.URL,
			"method":   f.Method,
			"callback": f.Callback,
			"meta":     f.Meta,
		})
		redisClient.LPush(ctx, queueKey, reqData)
		sqlxDB.Exec("UPDATE spider_failed_requests SET status = 'retried' WHERE id = ?", f.ID)
		count++
	}

	c.JSON(200, gin.H{"success": true, "message": fmt.Sprintf("已重试 %d 个失败请求", count), "count": count})
}

// RetryOneFailed 重试单个失败请求
func (h *SpiderStatsHandler) RetryOneFailed(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	projectID, _ := strconv.Atoi(c.Param("id"))
	failedID, _ := strconv.Atoi(c.Param("fid"))

	var f models.SpiderFailedRequest
	err := sqlxDB.Get(&f, `
		SELECT id, url, method, callback, meta
		FROM spider_failed_requests
		WHERE id = ? AND project_id = ? AND status = 'pending'
	`, failedID, projectID)

	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "失败请求不存在或状态不正确"})
		return
	}

	ctx := context.Background()
	queueKey := fmt.Sprintf("spider:queue:%d", projectID)

	reqData, _ := json.Marshal(map[string]interface{}{
		"url":      f.URL,
		"method":   f.Method,
		"callback": f.Callback,
		"meta":     f.Meta,
	})
	redisClient.LPush(ctx, queueKey, reqData)
	sqlxDB.Exec("UPDATE spider_failed_requests SET status = 'retried' WHERE id = ?", failedID)

	c.JSON(200, gin.H{"success": true, "message": "已重试"})
}

// IgnoreFailed 忽略失败请求
func (h *SpiderStatsHandler) IgnoreFailed(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	failedID, _ := strconv.Atoi(c.Param("fid"))

	result, _ := sqlxDB.Exec("UPDATE spider_failed_requests SET status = 'ignored' WHERE id = ?", failedID)
	affected, _ := result.RowsAffected()

	if affected == 0 {
		c.JSON(404, gin.H{"success": false, "message": "失败请求不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "已忽略"})
}

// DeleteFailed 删除失败请求
func (h *SpiderStatsHandler) DeleteFailed(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	failedID, _ := strconv.Atoi(c.Param("fid"))

	result, _ := sqlxDB.Exec("DELETE FROM spider_failed_requests WHERE id = ?", failedID)
	affected, _ := result.RowsAffected()

	if affected == 0 {
		c.JSON(404, gin.H{"success": false, "message": "失败请求不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "已删除"})
}

// GetOverview 获取统计概览（从 Redis 读取实时数据）
func (h *SpiderStatsHandler) GetOverview(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": map[string]interface{}{
			"total": 0, "completed": 0, "failed": 0, "retried": 0, "success_rate": 0, "avg_speed": 0,
		}})
		return
	}
	redisClient := rdb.(*redis.Client)

	projectIDStr := c.Query("project_id")
	ctx := context.Background()

	var total, completed, failed, retried int64

	if projectIDStr != "" && projectIDStr != "0" {
		// 单个项目统计
		projectID, _ := strconv.Atoi(projectIDStr)
		statsKey := fmt.Sprintf("spider:%d:stats", projectID)
		statsData, err := redisClient.HGetAll(ctx, statsKey).Result()
		if err == nil && len(statsData) > 0 {
			total, _ = strconv.ParseInt(statsData["total"], 10, 64)
			completed, _ = strconv.ParseInt(statsData["completed"], 10, 64)
			failed, _ = strconv.ParseInt(statsData["failed"], 10, 64)
			retried, _ = strconv.ParseInt(statsData["retried"], 10, 64)
		}
	} else {
		// 全部项目统计：扫描所有 spider:*:stats 键
		iter := redisClient.Scan(ctx, 0, "spider:*:stats", 100).Iterator()
		for iter.Next(ctx) {
			key := iter.Val()
			// 排除 archived 和 test 键
			if strings.Contains(key, ":archived") || strings.HasPrefix(key, "test_spider:") {
				continue
			}
			statsData, err := redisClient.HGetAll(ctx, key).Result()
			if err == nil {
				t, _ := strconv.ParseInt(statsData["total"], 10, 64)
				comp, _ := strconv.ParseInt(statsData["completed"], 10, 64)
				f, _ := strconv.ParseInt(statsData["failed"], 10, 64)
				r, _ := strconv.ParseInt(statsData["retried"], 10, 64)
				total += t
				completed += comp
				failed += f
				retried += r
			}
		}
		// 检查迭代器错误
		if err := iter.Err(); err != nil {
			c.JSON(500, gin.H{"success": false, "message": "Redis 扫描失败"})
			return
		}
	}

	var successRate float64
	totalDone := completed + failed
	if totalDone > 0 {
		successRate = math.Round(float64(completed)/float64(totalDone)*10000) / 100
	}

	c.JSON(200, gin.H{"success": true, "data": gin.H{
		"total":        total,
		"completed":    completed,
		"failed":       failed,
		"retried":      retried,
		"success_rate": successRate,
		"avg_speed":    0, // 实时统计不计算速度
	}})
}

// GetChart 获取图表数据
func (h *SpiderStatsHandler) GetChart(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	projectIDStr := c.Query("project_id")
	period := c.DefaultQuery("period", "hour")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	// 周期回退顺序
	periodFallback := map[string]string{
		"month": "day",
		"day":   "hour",
		"hour":  "minute",
	}

	// 尝试查询，如果没有数据则回退
	for {
		where := "period_type = ?"
		args := []interface{}{period}

		if projectIDStr != "" && projectIDStr != "0" {
			projectID, _ := strconv.Atoi(projectIDStr)
			where += " AND project_id = ?"
			args = append(args, projectID)
		}

		args = append(args, limit)

		var data []models.StatsChartPoint
		err := sqlxDB.Select(&data, `
			SELECT period_start as time, SUM(total) as total, SUM(completed) as completed,
			       SUM(failed) as failed, SUM(retried) as retried, AVG(avg_speed) as avg_speed
			FROM spider_stats_history
			WHERE `+where+`
			GROUP BY period_start
			ORDER BY period_start DESC
			LIMIT ?
		`, args...)

		if err == nil && len(data) > 0 {
			// 反转为时间正序
			for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
				data[i], data[j] = data[j], data[i]
			}
			c.JSON(200, gin.H{"success": true, "data": data})
			return
		}

		// 回退到更细粒度
		fallback, ok := periodFallback[period]
		if !ok {
			// 已经是最细粒度，返回空
			c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
			return
		}
		period = fallback
	}
}

// GetScheduled 获取已调度项目
func (h *SpiderStatsHandler) GetScheduled(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var projects []struct {
		ID       int     `db:"id" json:"id"`
		Name     string  `db:"name" json:"name"`
		Schedule *string `db:"schedule" json:"schedule"`
		Enabled  int     `db:"enabled" json:"enabled"`
	}

	sqlxDB.Select(&projects, `
		SELECT id, name, schedule, enabled
		FROM spider_projects
		WHERE schedule IS NOT NULL AND schedule != ''
		ORDER BY id
	`)

	c.JSON(200, gin.H{"success": true, "data": projects})
}

// GetByProject 按项目统计（从 Redis 读取实时数据）
func (h *SpiderStatsHandler) GetByProject(c *gin.Context) {
	db, dbExists := c.Get("db")
	rdb, redisExists := c.Get("redis")
	if !dbExists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	if !redisExists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)
	redisClient := rdb.(*redis.Client)
	ctx := context.Background()

	// 获取所有项目
	var projects []struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	if err := sqlxDB.Select(&projects, "SELECT id, name FROM spider_projects ORDER BY id"); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "查询项目列表失败"})
		return
	}

	// 从 Redis 获取每个项目的统计
	result := make([]gin.H, 0, len(projects))
	for _, p := range projects {
		statsKey := fmt.Sprintf("spider:%d:stats", p.ID)
		statsData, err := redisClient.HGetAll(ctx, statsKey).Result()

		var total, completed, failed, retried int64
		if err == nil && len(statsData) > 0 {
			total, _ = strconv.ParseInt(statsData["total"], 10, 64)
			completed, _ = strconv.ParseInt(statsData["completed"], 10, 64)
			failed, _ = strconv.ParseInt(statsData["failed"], 10, 64)
			retried, _ = strconv.ParseInt(statsData["retried"], 10, 64)
		}

		var successRate float64
		totalDone := completed + failed
		if totalDone > 0 {
			successRate = math.Round(float64(completed)/float64(totalDone)*10000) / 100
		}

		result = append(result, gin.H{
			"project_id":   p.ID,
			"project_name": p.Name,
			"total":        total,
			"completed":    completed,
			"failed":       failed,
			"retried":      retried,
			"success_rate": successRate,
		})
	}

	c.JSON(200, gin.H{"success": true, "data": result})
}
