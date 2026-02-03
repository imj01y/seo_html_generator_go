package api

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// ProcessorConfig 数据加工配置
type ProcessorConfig struct {
	Enabled            bool `json:"enabled"`
	Concurrency        int  `json:"concurrency"`
	RetryMax           int  `json:"retry_max"`
	MinParagraphLength int  `json:"min_paragraph_length"`
	BatchSize          int  `json:"batch_size"`
}

// ProcessorStatus 数据加工状态
type ProcessorStatus struct {
	Running        bool    `json:"running"`
	Workers        int     `json:"workers"`
	QueuePending   int64   `json:"queue_pending"`
	QueueRetry     int64   `json:"queue_retry"`
	QueueDead      int64   `json:"queue_dead"`
	ProcessedTotal int64   `json:"processed_total"`
	ProcessedToday int64   `json:"processed_today"`
	Speed          float64 `json:"speed"`
	LastError      *string `json:"last_error"`
}

// ProcessorCommand Redis 命令结构
type ProcessorCommand struct {
	Action    string `json:"action"`
	Timestamp int64  `json:"timestamp"`
}

// ProcessorHandler 数据加工处理器
type ProcessorHandler struct{}

// 配置键名常量
const (
	processorEnabledKey            = "processor.enabled"
	processorConcurrencyKey        = "processor.concurrency"
	processorRetryMaxKey           = "processor.retry_max"
	processorMinParagraphLengthKey = "processor.min_paragraph_length"
	processorBatchSizeKey          = "processor.batch_size"
)

// 默认配置
var processorDefaultConfig = ProcessorConfig{
	Enabled:            true,
	Concurrency:        3,
	RetryMax:           3,
	MinParagraphLength: 20,
	BatchSize:          50,
}

// publishProcessorCommand 发布命令到 Redis
func publishProcessorCommand(rdb *redis.Client, action string) error {
	ctx := context.Background()
	cmd := ProcessorCommand{
		Action:    action,
		Timestamp: time.Now().Unix(),
	}
	cmdJSON, _ := json.Marshal(cmd)
	return rdb.Publish(ctx, "processor:commands", cmdJSON).Err()
}

// GetConfig 获取配置
func (h *ProcessorHandler) GetConfig(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	config := loadProcessorConfig(sqlxDB)
	c.JSON(200, gin.H{"success": true, "data": config})
}

// UpdateConfig 更新配置
func (h *ProcessorHandler) UpdateConfig(c *gin.Context) {
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

	var config ProcessorConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 验证参数
	if config.Concurrency < 1 {
		config.Concurrency = 1
	}
	if config.Concurrency > 10 {
		config.Concurrency = 10
	}
	if config.RetryMax < 0 {
		config.RetryMax = 0
	}
	if config.RetryMax > 10 {
		config.RetryMax = 10
	}
	if config.MinParagraphLength < 1 {
		config.MinParagraphLength = 1
	}
	if config.BatchSize < 1 {
		config.BatchSize = 1
	}
	if config.BatchSize > 200 {
		config.BatchSize = 200
	}

	// 保存配置
	saveProcessorSetting(sqlxDB, processorEnabledKey, boolToStr(config.Enabled))
	saveProcessorSetting(sqlxDB, processorConcurrencyKey, intToStr(config.Concurrency))
	saveProcessorSetting(sqlxDB, processorRetryMaxKey, intToStr(config.RetryMax))
	saveProcessorSetting(sqlxDB, processorMinParagraphLengthKey, intToStr(config.MinParagraphLength))
	saveProcessorSetting(sqlxDB, processorBatchSizeKey, intToStr(config.BatchSize))

	// 通知 Worker 重新加载配置
	publishProcessorCommand(redisClient, "reload_config")

	c.JSON(200, gin.H{"success": true, "message": "配置已更新", "data": config})
}

// Start 手动启动
func (h *ProcessorHandler) Start(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	if err := publishProcessorCommand(redisClient, "start"); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "发送命令失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "启动命令已发送"})
}

// Stop 手动停止
func (h *ProcessorHandler) Stop(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	if err := publishProcessorCommand(redisClient, "stop"); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "发送命令失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "停止命令已发送"})
}

// RetryAll 重试所有失败任务
func (h *ProcessorHandler) RetryAll(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	ctx := context.Background()

	// 将重试队列中的所有任务移回待处理队列
	count := 0
	for {
		result, err := redisClient.RPopLPush(ctx, "pending:articles:retry", "pending:articles").Result()
		if err != nil || result == "" {
			break
		}
		// 清除重试计数
		redisClient.Del(ctx, "processor:retry:"+result)
		count++
	}

	c.JSON(200, gin.H{"success": true, "message": "已重试", "count": count})
}

// ClearDeadQueue 清空死信队列
func (h *ProcessorHandler) ClearDeadQueue(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	ctx := context.Background()

	// 获取队列长度
	length, _ := redisClient.LLen(ctx, "pending:articles:dead").Result()

	// 清空队列
	redisClient.Del(ctx, "pending:articles:dead")

	c.JSON(200, gin.H{"success": true, "message": "死信队列已清空", "count": length})
}

// ============================================
// 辅助函数
// ============================================

// loadProcessorConfig 从数据库加载配置
func loadProcessorConfig(db *sqlx.DB) ProcessorConfig {
	config := processorDefaultConfig

	var settings []struct {
		Key   string `db:"setting_key"`
		Value string `db:"setting_value"`
	}

	db.Select(&settings, `
		SELECT setting_key, setting_value FROM system_settings
		WHERE setting_key LIKE 'processor.%'
	`)

	for _, s := range settings {
		switch s.Key {
		case processorEnabledKey:
			config.Enabled = s.Value == "true" || s.Value == "1"
		case processorConcurrencyKey:
			config.Concurrency = strToInt(s.Value)
		case processorRetryMaxKey:
			config.RetryMax = strToInt(s.Value)
		case processorMinParagraphLengthKey:
			config.MinParagraphLength = strToInt(s.Value)
		case processorBatchSizeKey:
			config.BatchSize = strToInt(s.Value)
		}
	}

	return config
}

// saveProcessorSetting 保存单个配置项
func saveProcessorSetting(db *sqlx.DB, key, value string) {
	var existsCount int
	db.Get(&existsCount, "SELECT COUNT(*) FROM system_settings WHERE setting_key = ?", key)

	if existsCount > 0 {
		db.Exec("UPDATE system_settings SET setting_value = ? WHERE setting_key = ?", value, key)
	} else {
		db.Exec(`
			INSERT INTO system_settings (setting_key, setting_value, setting_type, description)
			VALUES (?, ?, 'string', ?)
		`, key, value, "数据加工配置: "+key)
	}
}

// 类型转换辅助函数
func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func intToStr(i int) string {
	return strconv.Itoa(i)
}

func strToInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func strToFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
