package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境应该限制
	},
}

// WebSocketHandler WebSocket 处理器
type WebSocketHandler struct{}

// SystemLogs 系统日志 WebSocket
// GET /api/logs/ws
func (h *WebSocketHandler) SystemLogs(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	// 升级为 WebSocket 连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 订阅系统日志频道
	channel := "system:logs"
	pubsub := redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	// 监听客户端断开
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// 接收并转发日志
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			if msg == nil {
				return
			}
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// SpiderStats 爬虫统计实时推送
// GET /ws/spider-stats/:id
func (h *WebSocketHandler) SpiderStats(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	projectID := c.Param("id")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 订阅统计频道
	channel := "spider:stats:project_" + projectID
	pubsub := redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	// 监听客户端断开
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// 转发统计更新
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			if msg == nil {
				return
			}
			conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
		case <-ctx.Done():
			return
		}
	}
}

// SpiderLogs 爬虫日志 WebSocket
// 支持 query 参数 type=test|project，默认 project
// GET /ws/spider-logs/:id?type=test
func (h *WebSocketHandler) SpiderLogs(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	projectID := c.Param("id")
	logType := c.DefaultQuery("type", "project") // test 或 project

	// 升级为 WebSocket 连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 订阅 Redis 日志频道（格式：spider:logs:test_1 或 spider:logs:project_1）
	channel := "spider:logs:" + logType + "_" + projectID
	pubsub := redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	// 监听客户端断开
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// 接收并转发日志
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			if msg == nil {
				return
			}
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
