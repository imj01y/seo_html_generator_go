package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"seo-generator/api/internal/model"
)

// SpiderExecutionHandler 爬虫执行处理器
type SpiderExecutionHandler struct{}

// publishCommand 发布命令到 Redis
func publishCommand(rdb *redis.Client, cmd models.SpiderCommand) error {
	ctx := context.Background()
	cmdJSON, _ := json.Marshal(cmd)
	return rdb.Publish(ctx, "spider:commands", cmdJSON).Err()
}

// Run 运行项目
func (h *SpiderExecutionHandler) Run(c *gin.Context) {
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

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var status string
	err = sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中"})
		return
	}

	sqlxDB.Exec("UPDATE spider_projects SET status = 'running' WHERE id = ?", id)

	cmd := models.SpiderCommand{
		Action:    "run",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	if err := publishCommand(redisClient, cmd); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "发送命令失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "任务已启动"})
}

// Test 测试运行
func (h *SpiderExecutionHandler) Test(c *gin.Context) {
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
	maxItems, _ := strconv.Atoi(c.DefaultQuery("max_items", "0"))

	var existsCount int
	sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if existsCount == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	cmd := models.SpiderCommand{
		Action:    "test",
		ProjectID: id,
		MaxItems:  maxItems,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	sessionID := fmt.Sprintf("test_%d", id)
	c.JSON(200, gin.H{"success": true, "message": "测试已启动", "session_id": sessionID})
}

// TestStop 停止测试
func (h *SpiderExecutionHandler) TestStop(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	cmd := models.SpiderCommand{
		Action:    "test_stop",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	c.JSON(200, gin.H{"success": true, "message": "测试已停止"})
}

// Stop 停止项目
func (h *SpiderExecutionHandler) Stop(c *gin.Context) {
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
	clearQueue := c.Query("clear_queue") == "true"

	cmd := models.SpiderCommand{
		Action:    "stop",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	sqlxDB.Exec("UPDATE spider_projects SET status = 'idle', last_error = ? WHERE id = ?",
		"用户手动停止", id)

	message := "已停止"
	if clearQueue {
		message += "并清空队列"
	}
	c.JSON(200, gin.H{"success": true, "message": message})
}

// Pause 暂停项目
func (h *SpiderExecutionHandler) Pause(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	cmd := models.SpiderCommand{
		Action:    "pause",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	c.JSON(200, gin.H{"success": true, "message": "已暂停"})
}

// Resume 恢复项目
func (h *SpiderExecutionHandler) Resume(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	cmd := models.SpiderCommand{
		Action:    "resume",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	c.JSON(200, gin.H{"success": true, "message": "已恢复"})
}
