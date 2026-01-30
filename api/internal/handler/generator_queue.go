package api

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// GeneratorQueueHandler 生成器队列处理器
type GeneratorQueueHandler struct{}

// GetQueueStats 获取待处理队列统计信息
func (h *GeneratorQueueHandler) GetQueueStats(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis 未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	groupID := c.DefaultQuery("group_id", "1")

	ctx := context.Background()
	queueKey := "pending:articles:" + groupID
	queueSize, err := redisClient.LLen(ctx, queueKey).Result()
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":    true,
		"group_id":   groupID,
		"queue_size": queueSize,
	})
}

// PushToQueue 批量推送文章到待处理队列
func (h *GeneratorQueueHandler) PushToQueue(c *gin.Context) {
	db, dbExists := c.Get("db")
	rdb, redisExists := c.Get("redis")
	if !dbExists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	if !redisExists {
		c.JSON(500, gin.H{"success": false, "message": "Redis 未连接"})
		return
	}

	sqlxDB := db.(*sqlx.DB)
	redisClient := rdb.(*redis.Client)

	groupID := c.DefaultQuery("group_id", "1")
	limitStr := c.DefaultQuery("limit", "1000")
	statusStr := c.DefaultQuery("status", "1")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100000 {
		limit = 1000
	}
	status, _ := strconv.Atoi(statusStr)

	ctx := context.Background()
	queueKey := "pending:articles:" + groupID

	// 获取已在队列中的 ID
	existingItems, _ := redisClient.LRange(ctx, queueKey, 0, -1).Result()
	existingIDs := make(map[string]bool)
	for _, item := range existingItems {
		existingIDs[item] = true
	}

	// 查询待推送的文章
	var articles []struct {
		ID int `db:"id"`
	}
	sqlxDB.Select(&articles, `
		SELECT id FROM original_articles
		WHERE group_id = ? AND status = ?
		ORDER BY id DESC
		LIMIT ?
	`, groupID, status, limit)

	if len(articles) == 0 {
		c.JSON(200, gin.H{"success": true, "pushed": 0, "skipped": 0, "message": "没有待处理的文章"})
		return
	}

	// 筛选新 ID
	var newIDs []string
	skipped := 0
	for _, a := range articles {
		idStr := strconv.Itoa(a.ID)
		if existingIDs[idStr] {
			skipped++
		} else {
			newIDs = append(newIDs, idStr)
		}
	}

	if len(newIDs) == 0 {
		c.JSON(200, gin.H{"success": true, "pushed": 0, "skipped": skipped, "message": "所有文章已在队列中"})
		return
	}

	// 批量推送
	pipe := redisClient.Pipeline()
	for _, id := range newIDs {
		pipe.LPush(ctx, queueKey, id)
	}
	pipe.Exec(ctx)

	c.JSON(200, gin.H{
		"success": true,
		"pushed":  len(newIDs),
		"skipped": skipped,
		"message": "已推送 " + strconv.Itoa(len(newIDs)) + " 篇文章到队列",
	})
}

// ClearQueue 清空待处理队列
func (h *GeneratorQueueHandler) ClearQueue(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis 未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	groupID := c.DefaultQuery("group_id", "1")

	ctx := context.Background()
	queueKey := "pending:articles:" + groupID

	queueSize, _ := redisClient.LLen(ctx, queueKey).Result()
	redisClient.Del(ctx, queueKey)

	c.JSON(200, gin.H{
		"success": true,
		"cleared": queueSize,
		"message": "已清空队列，共删除 " + strconv.FormatInt(queueSize, 10) + " 条",
	})
}

// GetGeneratorStats 获取生成器统计信息
func (h *GeneratorQueueHandler) GetGeneratorStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var titlesCount, contentsCount, articlesCount int

	sqlxDB.Get(&titlesCount, "SELECT COUNT(*) FROM titles")
	sqlxDB.Get(&contentsCount, "SELECT COUNT(*) FROM contents")
	sqlxDB.Get(&articlesCount, "SELECT COUNT(*) FROM original_articles WHERE status = 1")

	c.JSON(200, gin.H{
		"success":                 true,
		"titles_count":            titlesCount,
		"contents_count":          contentsCount,
		"original_articles_count": articlesCount,
	})
}

// GetWorkerStatus 获取 Worker 运行状态（通过 Redis）
func (h *GeneratorQueueHandler) GetWorkerStatus(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(200, gin.H{
			"success":            true,
			"running":            false,
			"worker_initialized": false,
			"message":            "Redis 未连接，无法检查 Worker 状态",
		})
		return
	}
	redisClient := rdb.(*redis.Client)

	ctx := context.Background()
	// Worker 可以在 Redis 中设置心跳 key
	heartbeat, err := redisClient.Get(ctx, "generator:worker:heartbeat").Result()

	running := err == nil && heartbeat != ""

	c.JSON(200, gin.H{
		"success":            true,
		"running":            running,
		"worker_initialized": running,
		"last_heartbeat":     heartbeat,
	})
}
