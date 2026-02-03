# 数据处理状态 WebSocket 实时推送设计

## 概述

将数据处理页面的队列状态监控从 HTTP 轮询 + Redis Pub/Sub 混合模式改为纯 WebSocket 定时推送模式。

## 当前问题

- WebSocket 使用 Redis Pub/Sub 订阅模式，连接时收不到历史数据
- 前端在多处仍需调用 HTTP API（初始加载、操作后刷新）
- 状态更新依赖 Python Worker 的发布，Worker 未运行时无数据

## 解决方案

采用后端主动定时推送模式（参考现有的 PoolStatus、SystemStats 实现）：
- 连接时立即发送一次初始状态
- 每秒定时查询 Redis 并推送
- 前端完全移除 HTTP 状态查询

## 文件改动

### 后端

| 文件 | 改动 |
|------|------|
| `api/internal/handler/websocket.go` | 修改 `ProcessorStatus` 为定时推送，新增 `sendProcessorStatus` |
| `api/internal/handler/processor.go` | 移除 `GetStatus` 方法 |
| `api/internal/handler/router.go` | 移除 `GET /api/processor/status` 路由 |

### 前端

| 文件 | 改动 |
|------|------|
| `web/src/views/processor/ProcessorManage.vue` | 移除 HTTP 调用逻辑 |
| `web/src/api/processor.ts` | 移除 `getProcessorStatus` 函数 |

## 实现细节

### 后端 WebSocket 改动

```go
// websocket.go - 修改 ProcessorStatus 方法
func (h *WebSocketHandler) ProcessorStatus(c *gin.Context) {
    // 1. 获取 Redis 客户端
    // 2. 升级 WebSocket 连接
    // 3. 启动 goroutine 监听客户端断开
    // 4. 立即发送一次初始状态
    // 5. 每秒定时推送
}

// 新增 sendProcessorStatus 辅助函数
func (h *WebSocketHandler) sendProcessorStatus(conn *websocket.Conn, redisClient *redis.Client) error {
    // 查询 Redis 队列长度（pending、retry、dead）
    // 查询 processor:status Hash
    // 构建 JSON 并发送
}
```

### 前端改动

移除的代码：
- `loadStatus()` 函数
- `loadAll()` 中的 `loadStatus()` 调用
- `onopen` 中的 `loadStatus()` 调用
- 操作后的 `setTimeout(loadStatus, 1000)` 调用
- `getProcessorStatus` 导入

### 数据流

```
之前：WebSocket 连接 → HTTP 获取初始状态 → 等待 Redis Pub/Sub
之后：WebSocket 连接 → 后端立即推送 → 每秒定时推送
```

## 推送数据格式

```json
{
  "running": true,
  "workers": 4,
  "queue_pending": 100,
  "queue_retry": 5,
  "queue_dead": 2,
  "processed_total": 10000,
  "processed_today": 500,
  "speed": 2.5,
  "last_error": null
}
```

与现有 HTTP API 响应格式一致，前端无需修改数据处理逻辑。
