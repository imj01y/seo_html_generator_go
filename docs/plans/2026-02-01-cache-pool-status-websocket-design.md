# 缓存管理页面池状态 WebSocket 实时推送设计

## 概述

将缓存管理页面"运行状态" tab 中的数据池和对象池改为 WebSocket 实时推送，实现 1 秒级别的状态更新。

## 需求

1. **数据池 + 对象池**：WebSocket 实时推送（1秒/次）
2. **HTML 缓存**：保持现有 HTTP API 按需刷新
3. **Tab 切换行为**：进入"运行状态" tab 时建立连接，离开时断开
4. **推送方式**：API 服务器直接推送（不经过 Redis）

## 技术方案

### 后端

#### 新增 WebSocket 端点

**路径**: `GET /ws/pool-status`

**实现逻辑**:
1. 升级 HTTP 连接为 WebSocket
2. 启动 goroutine，每秒执行：
   - 获取对象池状态 (`TemplateFuncs.GetPoolStats()`)
   - 获取数据池状态 (`DataManager.GetDataPoolsStats()`)
   - 序列化为 JSON 并推送
3. 监听客户端断开，停止推送

**代码位置**: `api/internal/handler/websocket.go`

#### 消息格式

```json
{
  "type": "pool_status",
  "timestamp": "2026-02-01T12:00:00.000Z",
  "object_pools": {
    "cls": {
      "name": "CSS 类名池",
      "size": 1000,
      "available": 800,
      "used": 200,
      "utilization": 20.0,
      "status": "running",
      "paused": false
    },
    "url": { ... },
    "keyword_emoji": { ... }
  },
  "data_pools": [
    {
      "name": "关键词池",
      "size": 5000,
      "available": 4500,
      "used": 500,
      "utilization": 10.0,
      "status": "running",
      "last_refresh": "2026-02-01T11:55:00Z"
    },
    { ... }
  ]
}
```

#### 依赖注入

WebSocket handler 需要访问 `TemplateFuncs` 和 `DataManager`，需要修改 `WebSocketHandler` 结构体：

```go
type WebSocketHandler struct {
    templateFuncs *core.TemplateFuncsManager
    dataManager   *core.DataManager
}
```

### 前端

#### WebSocket 连接管理

**文件**: `web/src/views/cache/CacheManage.vue`

**逻辑**:
1. 使用 `watch` 监听 `mainTab` 变化
2. 当 `mainTab === 'status'` 时建立 WebSocket 连接
3. 当 `mainTab !== 'status'` 时断开连接
4. 组件 `onUnmounted` 时确保断开连接

#### 状态更新

- 收到 WebSocket 消息后直接更新 `objectPoolStats` 和 `dataPoolStats`
- 移除 `poolStatusLoading` 状态（数据持续更新，无需 loading）
- 保留操作按钮（预热、暂停、恢复、刷新数据）

#### UI 调整

1. 移除"刷新全部"按钮中对池状态的刷新（只刷新 HTML 缓存）
2. 添加连接状态指示器（可选，显示 WebSocket 连接状态）
3. 数据池/对象池卡片移除 loading 状态

### 路由注册

在 `router.go` 中添加新端点：

```go
r.GET("/ws/pool-status", wsHandler.PoolStatus)
```

## 实现步骤

### 步骤 1: 后端 - 修改 WebSocketHandler

1. 修改 `WebSocketHandler` 结构体，添加依赖字段
2. 修改 `SetupRouter`，创建带依赖的 `WebSocketHandler`
3. 实现 `PoolStatus` 方法

### 步骤 2: 后端 - 注册路由

1. 在 `SetupRouter` 中注册 `/ws/pool-status` 端点

### 步骤 3: 前端 - WebSocket 集成

1. 添加 WebSocket 连接管理逻辑
2. 添加 `watch` 监听 tab 切换
3. 更新数据绑定逻辑

### 步骤 4: 前端 - UI 调整

1. 调整"刷新全部"按钮行为
2. 移除池状态卡片的 loading 状态
3. （可选）添加连接状态指示

## 文件变更清单

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `api/internal/handler/websocket.go` | 修改 | 添加 PoolStatus 方法，修改结构体 |
| `api/internal/handler/router.go` | 修改 | 注册新端点，修改 wsHandler 创建方式 |
| `web/src/views/cache/CacheManage.vue` | 修改 | 添加 WebSocket 连接管理 |

## 兼容性

- 现有 HTTP API (`/api/admin/pool/stats`, `/api/admin/data/stats`) 保持不变
- 操作 API（预热、暂停、恢复、刷新）保持不变
- HTML 缓存相关功能不受影响
