package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	core "seo-generator/api/internal/service"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境应该限制
	},
}

// WebSocketHandler WebSocket 处理器
type WebSocketHandler struct {
	templateFuncs *core.TemplateFuncsManager
	poolManager   *core.PoolManager
	systemStats   *core.SystemStatsCollector
}

// NewWebSocketHandler 创建 WebSocket 处理器
func NewWebSocketHandler(templateFuncs *core.TemplateFuncsManager, poolManager *core.PoolManager, systemStats *core.SystemStatsCollector) *WebSocketHandler {
	return &WebSocketHandler{
		templateFuncs: templateFuncs,
		poolManager:   poolManager,
		systemStats:   systemStats,
	}
}

// subscribeAndForward 订阅 Redis 频道并转发消息到 WebSocket
// 这是一个通用的辅助函数，用于简化 Redis Pub/Sub 到 WebSocket 的转发逻辑
func subscribeAndForward(conn *websocket.Conn, redisClient *redis.Client, channel string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// 接收并转发消息
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			if msg == nil {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// SystemLogs 系统日志 WebSocket
// GET /api/logs/ws
func (h *WebSocketHandler) SystemLogs(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	subscribeAndForward(conn, redisClient, "system:logs")
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

	subscribeAndForward(conn, redisClient, "spider:stats:project_"+projectID)
}

// WorkerRestart Worker 重启 WebSocket
// 实时推送 pip install 和 docker restart 的日志，重启后自动监听 15 秒容器日志
// GET /ws/worker-restart
func (h *WebSocketHandler) WorkerRestart(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	containerName := "seo-generator-worker"

	// 发送日志的辅助函数
	sendLog := func(logType, message string) {
		data, _ := json.Marshal(map[string]string{
			"type": logType,
			"data": message,
		})
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// 发送完成信号
	sendDone := func(success bool, message string) {
		data, _ := json.Marshal(map[string]interface{}{
			"type":    "done",
			"success": success,
			"data":    message,
		})
		conn.WriteMessage(websocket.TextMessage, data)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 步骤 1: 安装依赖
	sendLog("info", "> 正在安装依赖...")

	pipCmd := exec.CommandContext(ctx,
		"docker", "exec", containerName,
		"pip", "install", "-r", "/app/requirements.txt",
	)

	// 获取 stdout 和 stderr 管道
	stdout, err := pipCmd.StdoutPipe()
	if err != nil {
		sendDone(false, "无法创建输出管道: "+err.Error())
		return
	}
	stderr, err := pipCmd.StderrPipe()
	if err != nil {
		sendDone(false, "无法创建错误管道: "+err.Error())
		return
	}

	if err := pipCmd.Start(); err != nil {
		sendDone(false, "启动命令失败: "+err.Error())
		return
	}

	// 实时读取 stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			sendLog("stdout", scanner.Text())
		}
	}()

	// 实时读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			sendLog("stderr", scanner.Text())
		}
	}()

	if err := pipCmd.Wait(); err != nil {
		sendDone(false, "安装依赖失败: "+err.Error())
		return
	}

	sendLog("info", "> 依赖安装完成")

	// 步骤 2: 重启容器
	sendLog("info", "> 正在重启容器...")

	restartCmd := exec.CommandContext(ctx,
		"docker", "restart", containerName,
	)
	restartOutput, err := restartCmd.CombinedOutput()
	if err != nil {
		sendDone(false, "重启容器失败: "+err.Error())
		return
	}

	if len(restartOutput) > 0 {
		sendLog("stdout", string(restartOutput))
	}

	sendLog("info", "> 容器已重启，正在监听启动日志...")

	// 步骤 3: 监听容器日志 15 秒
	logCtx, logCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer logCancel()

	// 使用 docker logs -f --since 来获取重启后的日志
	logCmd := exec.CommandContext(logCtx,
		"docker", "logs", "-f", "--since", "5s", containerName,
	)

	logStdout, _ := logCmd.StdoutPipe()
	logStderr, _ := logCmd.StderrPipe()

	if err := logCmd.Start(); err != nil {
		sendLog("stderr", "无法获取容器日志: "+err.Error())
	} else {
		// 实时读取日志
		go func() {
			scanner := bufio.NewScanner(logStdout)
			for scanner.Scan() {
				sendLog("stdout", scanner.Text())
			}
		}()
		go func() {
			scanner := bufio.NewScanner(logStderr)
			for scanner.Scan() {
				sendLog("stderr", scanner.Text())
			}
		}()

		// 等待超时或命令结束
		logCmd.Wait()
	}

	sendLog("info", "> 日志监听结束（15秒）")
	sendDone(true, "Worker 重启完成")
}

// WorkerLogs Worker 实时日志 WebSocket
// 持续监听容器日志直到客户端断开
// GET /ws/worker-logs
func (h *WebSocketHandler) WorkerLogs(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	containerName := "seo-generator-worker"

	// 发送日志的辅助函数
	sendLog := func(logType, message string) {
		data, _ := json.Marshal(map[string]string{
			"type": logType,
			"data": message,
		})
		conn.WriteMessage(websocket.TextMessage, data)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	sendLog("info", "> 开始监听容器日志...")

	// 使用 docker logs -f --tail 获取最近日志并持续监听
	logCmd := exec.CommandContext(ctx,
		"docker", "logs", "-f", "--tail", "50", containerName,
	)

	logStdout, err := logCmd.StdoutPipe()
	if err != nil {
		sendLog("stderr", "无法创建输出管道: "+err.Error())
		return
	}
	logStderr, err := logCmd.StderrPipe()
	if err != nil {
		sendLog("stderr", "无法创建错误管道: "+err.Error())
		return
	}

	if err := logCmd.Start(); err != nil {
		sendLog("stderr", "无法获取容器日志: "+err.Error())
		return
	}

	// 实时读取 stdout
	go func() {
		scanner := bufio.NewScanner(logStdout)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				sendLog("stdout", scanner.Text())
			}
		}
	}()

	// 实时读取 stderr
	go func() {
		scanner := bufio.NewScanner(logStderr)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				sendLog("stderr", scanner.Text())
			}
		}
	}()

	// 等待上下文取消或命令结束
	<-ctx.Done()
	logCmd.Process.Kill()
	sendLog("info", "> 日志监听已停止")
}

// ProcessorLogs 数据处理日志 WebSocket
// 订阅 processor:logs 频道，推送数据加工任务的实时日志
// GET /ws/processor-logs
func (h *WebSocketHandler) ProcessorLogs(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	subscribeAndForward(conn, redisClient, "processor:logs")
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
	logType := c.DefaultQuery("type", "project")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 订阅 Redis 日志频道（格式：spider:logs:test_1 或 spider:logs:project_1）
	channel := "spider:logs:" + logType + "_" + projectID
	subscribeAndForward(conn, redisClient, channel)
}

// ProcessorStatus 数据处理状态实时推送
// GET /ws/processor-status
func (h *WebSocketHandler) ProcessorStatus(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	subscribeAndForward(conn, redisClient, "processor:status:realtime")
}

// PoolStatus 池状态实时推送
// 每秒推送一次对象池和数据池的状态
// GET /ws/pool-status
func (h *WebSocketHandler) PoolStatus(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// 每秒推送一次状态
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 立即发送一次初始状态
	h.sendPoolStatus(conn)

	for {
		select {
		case <-ticker.C:
			if err := h.sendPoolStatus(conn); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// sendPoolStatus 发送池状态消息
func (h *WebSocketHandler) sendPoolStatus(conn *websocket.Conn) error {
	// 构建状态消息
	msg := map[string]interface{}{
		"type":      "pool_status",
		"timestamp": time.Now().Format(time.RFC3339Nano),
	}

	// 获取对象池状态
	if h.templateFuncs != nil {
		msg["object_pools"] = h.templateFuncs.GetPoolStats()
	} else {
		msg["object_pools"] = map[string]interface{}{}
	}

	// 获取数据池状态
	if h.poolManager != nil {
		msg["data_pools"] = h.poolManager.GetDataPoolsStats()
	} else {
		msg["data_pools"] = []interface{}{}
	}

	// 序列化并发送
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

// SystemStats 系统资源实时推送
// GET /ws/system-stats
func (h *WebSocketHandler) SystemStats(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 立即发送一次初始状态
	h.sendSystemStats(conn)

	for {
		select {
		case <-ticker.C:
			if err := h.sendSystemStats(conn); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// sendSystemStats 发送系统统计消息
func (h *WebSocketHandler) sendSystemStats(conn *websocket.Conn) error {
	if h.systemStats == nil {
		return nil
	}

	stats, err := h.systemStats.Collect()
	if err != nil {
		return err
	}

	msg := map[string]interface{}{
		"type":      "system_stats",
		"timestamp": time.Now().Format(time.RFC3339Nano),
		"cpu":       stats.CPU,
		"memory":    stats.Memory,
		"load":      stats.Load,
		"network":   stats.Network,
		"disks":     stats.Disks,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}
