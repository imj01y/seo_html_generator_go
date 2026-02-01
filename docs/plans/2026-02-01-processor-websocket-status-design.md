# 数据处理监控面板 WebSocket 实时状态设计

## 概述

将数据处理页面监控面板中的队列状态展示从定时轮询改为 WebSocket 实时推送。

## 整体架构

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Python Worker  │     │    Redis        │     │   Go Backend    │
│                 │     │                 │     │                 │
│  处理完一条任务  │────▶│  PUBLISH        │────▶│  订阅频道       │
│  ↓              │     │  processor:     │     │  ↓              │
│  查询队列长度    │     │  status:realtime│     │  WebSocket 转发  │
│  ↓              │     │                 │     │                 │
│  发布完整状态    │     └─────────────────┘     └────────┬────────┘
└─────────────────┘                                      │
                                                         ▼
                                              ┌─────────────────┐
                                              │   Vue Frontend  │
                                              │                 │
                                              │  WebSocket 连接  │
                                              │  ↓              │
                                              │  更新 reactive   │
                                              │  状态对象        │
                                              └─────────────────┘
```

**数据流：**
1. Worker 处理完一条任务后，查询 Redis 获取 3 个队列的长度
2. 组装完整状态 JSON，PUBLISH 到 `processor:status:realtime` 频道
3. Go 后端通过 `subscribeAndForward` 模式订阅并转发
4. 前端 WebSocket 接收后直接更新 `status` reactive 对象

## 数据结构

**WebSocket 端点：**
```
GET /ws/processor-status
```

**推送的 JSON 结构：**
```json
{
  "running": true,
  "workers": 3,
  "queue_pending": 156,
  "queue_retry": 2,
  "queue_dead": 0,
  "processed_total": 1234,
  "processed_today": 89,
  "speed": 2.5,
  "last_error": null
}
```

**Redis 频道：**
```
processor:status:realtime
```

## 代码改动清单

| 组件 | 文件 | 改动 |
|------|------|------|
| Worker | `generator_worker.py` | 添加 `on_complete` 回调属性，处理完成/失败后调用 |
| Manager | `generator_manager.py` | 1. 新增 `_last_error` 属性<br>2. 新增 `_publish_realtime_status()` 方法<br>3. 创建 Worker 时传入回调 |
| Go 后端 | `websocket.go` | 新增 `ProcessorStatus` 方法 |
| Go 后端 | `router.go` | 新增 `/ws/processor-status` 路由 |
| 前端 | `ProcessorManage.vue` | 1. 使用 `buildWsUrl` 建立 WebSocket<br>2. 添加断线自动重连逻辑<br>3. 连接成功后调用一次 HTTP API 获取初始状态<br>4. 移除定时轮询<br>5. 保留手动刷新按钮 |

## 错误处理

**Python Worker：**

| 场景 | 处理方式 |
|------|----------|
| 任务处理异常 | 捕获异常，记录到 `_last_error`，继续处理下一条 |
| Redis 连接失败 | 发布失败时静默忽略，不影响主流程 |
| 队列查询失败 | 使用默认值 0，记录日志 |

**Go 后端：**

| 场景 | 处理方式 |
|------|----------|
| Redis 未连接 | 返回 500，关闭 WebSocket |
| WebSocket 升级失败 | 直接返回，无需额外处理 |
| 客户端断开 | `subscribeAndForward` 已有处理，自动退出 |

**前端：**

| 场景 | 处理方式 |
|------|----------|
| 连接失败 | 启动重连 |
| 连接断开 | 指数退避重连（1s → 2s → 4s → 8s，最大 30s） |
| 消息解析失败 | 静默忽略，记录 console.error |
| 组件卸载 | 关闭 WebSocket 并清除重连定时器 |

**前端重连逻辑：**
```typescript
let ws: WebSocket | null = null
let reconnectTimer: number | null = null
let reconnectDelay = 1000

onMounted(() => {
  connect()
})

onUnmounted(() => {
  if (reconnectTimer) clearTimeout(reconnectTimer)
  if (ws) ws.close()
})

function connect() {
  ws = new WebSocket(buildWsUrl('/ws/processor-status'))

  ws.onopen = () => {
    reconnectDelay = 1000
    loadStatus()  // 获取初始状态
  }

  ws.onmessage = (e) => {
    try {
      Object.assign(status, JSON.parse(e.data))
    } catch { /* 静默忽略 */ }
  }

  ws.onclose = () => {
    reconnectTimer = setTimeout(connect, reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 2, 30000)
  }
}
```

## 测试验证

**编译验证：**

| 阶段 | 验证项 | 方法 |
|------|--------|------|
| 后端编译 | Go 代码无语法错误 | `go build ./...` |
| 前端编译 | TypeScript 无类型错误 | `npx vue-tsc --noEmit` |
| Worker 语法 | Python 代码无语法错误 | `python -m py_compile generator_manager.py generator_worker.py` |

**功能验证：**

| 场景 | 预期结果 |
|------|----------|
| 打开监控页面 | WebSocket 连接成功，显示当前状态 |
| Worker 处理一条任务 | 前端立即更新队列数量和处理计数 |
| Worker 处理失败 | `last_error` 显示错误信息，重试/死信队列数量变化 |
| 手动点击刷新 | 调用 HTTP API 更新状态 |
| 关闭页面再打开 | 重新建立连接，显示最新状态 |
| 网络断开后恢复 | 自动重连，数据恢复正常 |

## 回滚策略

如果出现问题，可以快速回滚：
- 前端：恢复定时轮询逻辑
- 后端：移除 WebSocket 路由
- Worker：移除发布逻辑

---

*文档创建时间: 2026-02-01*
