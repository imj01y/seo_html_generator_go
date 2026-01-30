# 爬虫实时统计设计

## 概述

将爬虫项目页面的"累计抓取"统计从手动刷新改为实时更新。每次抓取成功一条数据，前端统计数字自动 +1。

## 技术方案

复用现有 WebSocket 基础设施，通过 Redis Pub/Sub 实现实时推送。

### 数据流

```
Python Worker (每抓取成功1条)
    ↓ 更新 Redis 计数
    ↓ 发布到 Redis 频道: spider:stats:project_{id}
    ↓
Go API (WebSocket 订阅)
    ↓ 转发给前端
    ↓
Vue 前端 (实时更新统计卡片)
```

## 改动清单

| 组件 | 文件 | 改动 |
|------|------|------|
| Python Worker | `worker/core/workers/command_listener.py` | 每抓取成功1条，发布统计更新到 Redis |
| Go API | `api/internal/handler/websocket.go` | 新增 `SpiderStats` WebSocket 端点 |
| Go API | `api/internal/handler/router.go` | 注册新路由 `/ws/spider-stats/:id` |
| Vue 前端 | `web/src/api/spiderProjects.ts` | 新增 `subscribeProjectStats()` |
| Vue 前端 | `web/src/views/spiders/ProjectList.vue` | 运行时订阅统计，实时更新显示 |

## 详细设计

### 1. Python Worker

**文件：** `worker/core/workers/command_listener.py`

在 `run_project()` 方法中，每成功处理一条数据后：

```python
# 更新 Redis 实时计数
stats_key = f"spider:{project_id}:stats"
await self.rdb.hincrby(stats_key, "completed", 1)

# 发布统计更新（前端订阅）
stats_msg = {
    "type": "stats",
    "project_id": project_id,
    "items_count": items_count,
    "timestamp": datetime.now().isoformat()
}
await self.rdb.publish(
    f"spider:stats:project_{project_id}",
    json.dumps(stats_msg, ensure_ascii=False)
)
```

**消息格式：**
```json
{
  "type": "stats",
  "project_id": 1,
  "items_count": 42,
  "timestamp": "2026-01-31T10:30:15"
}
```

### 2. Go 后端 WebSocket 端点

**文件：** `api/internal/handler/websocket.go`

新增 `SpiderStats` 方法：

```go
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
```

**路由注册（router.go）：**
```go
ws.GET("/spider-stats/:id", wsHandler.SpiderStats)
```

### 3. 前端 API 层

**文件：** `web/src/api/spiderProjects.ts`

新增 `subscribeProjectStats` 函数：

```typescript
/**
 * 订阅项目统计更新
 */
export function subscribeProjectStats(
  projectId: number,
  onStats: (itemsCount: number) => void,
  onError?: (error: string) => void
): () => void {
  const wsUrl = buildWsUrl(`/ws/spider-stats/${projectId}`)
  let ws: WebSocket | null = null

  try {
    ws = new WebSocket(wsUrl)
  } catch (e) {
    onError?.(`WebSocket 创建失败: ${e}`)
    return () => {}
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'stats') {
        onStats(msg.items_count)
      }
    } catch {
      // 忽略解析错误
    }
  }

  ws.onerror = () => {
    onError?.('统计 WebSocket 连接失败')
  }

  return () => {
    closeWebSocket(ws)
  }
}
```

### 4. 前端 Vue 组件

**文件：** `web/src/views/spiders/ProjectList.vue`

改动点：

1. 引入 `subscribeProjectStats`
2. 新增订阅管理变量 `unsubscribeStats`
3. 在 `handleRun` 中订阅统计更新，收到消息时 `row.total_items += 1`
4. 任务结束或组件卸载时取消订阅

## 用户体验

- 用户点击"运行"后，统计卡片的"累计抓取"数字会随着抓取进度实时跳动
- 表格中对应项目的"本次/总量"也会实时更新
- 无需手动刷新页面
