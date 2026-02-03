# 系统资源实时监控设计方案

## 概述

在仪表盘页面增加系统资源实时监控功能，监控 CPU、内存、硬盘、网络、系统负载。

## 需求

- 监控目标：Go API 所在主机资源（单机部署）
- 更新频率：1 秒
- 展示形式：仪表盘卡片（左右分栏）
- 磁盘监控：所有分区
- 监控指标：CPU、内存、硬盘、网络 IO、系统负载

## UI 设计

### 布局

在现有统计卡片下方新增系统资源卡片：

```
┌─────────────────────────────────────────────────────────────────────┐
│  [站点数量] [关键词总数] [图片总数] [文章总数]                        │  ← 现有
├─────────────────────────────────────────────────────────────────────┤
│  [系统资源卡片 - 左右分栏]                                           │  ← 新增
├─────────────────────────────────────────────────────────────────────┤
│  [蜘蛛访问趋势图]              [蜘蛛类型分布]                        │  ← 现有
└─────────────────────────────────────────────────────────────────────┘
```

### 系统资源卡片详细设计

```
┌─────────────────────────────────────┬──────────────────────────────────┐
│ CPU      ████████░░░░  45.2%  8核  │  磁盘                            │
│ 内存     ████████░░░░  50.0%  8.25/16.00G│  C: ████░░ 50%  250/500GB  │
│ 负载     1.25 / 1.10 / 0.95        │  D: ███░░░ 40%   400GB/1TB       │
│ 网络     ↑ 1.2 MB/s  ↓ 2.5 MB/s    │  E: █░░░░░ 15%   150GB/1TB       │
└─────────────────────────────────────┴──────────────────────────────────┘
```

### 格式化规则

| 指标 | 格式 | 示例 |
|------|------|------|
| CPU 百分比 | 1 位小数 | `45.2%` |
| 内存容量 | 2 位小数 + G | `8.25/16.00G` |
| 内存百分比 | 1 位小数 | `50.0%` |
| 磁盘容量 | 整数 GB/TB | `250/500GB` |
| 磁盘百分比 | 整数 | `50%` |
| 网络速率 | 1 位小数，自动切换单位 | `1.2 MB/s` |
| 系统负载 | 2 位小数 | `1.25 / 1.10 / 0.95` |

## 技术架构

### 数据流

```
Go 后端 (gopsutil 库)
    ↓ 每秒采集
WebSocket /ws/system-stats
    ↓ 推送 JSON
前端 Dashboard
    ↓ 更新卡片
实时显示
```

### WebSocket 消息格式

```json
{
  "type": "system_stats",
  "timestamp": "2026-02-04T10:00:00.000Z",
  "cpu": {
    "usage_percent": 45.2,
    "cores": 8
  },
  "memory": {
    "total_bytes": 17179869184,
    "used_bytes": 8589934592,
    "usage_percent": 50.0
  },
  "load": {
    "load1": 1.25,
    "load5": 1.10,
    "load15": 0.95
  },
  "network": {
    "bytes_sent_per_sec": 1048576,
    "bytes_recv_per_sec": 2097152
  },
  "disks": [
    {
      "path": "C:",
      "total_bytes": 500000000000,
      "used_bytes": 250000000000,
      "usage_percent": 50.0
    },
    {
      "path": "D:",
      "total_bytes": 1000000000000,
      "used_bytes": 400000000000,
      "usage_percent": 40.0
    }
  ]
}
```

## 后端实现

### 新增文件

**`api/internal/service/system_stats.go`**

```go
package core

import (
    "sync"
    "time"

    "github.com/shirou/gopsutil/v3/cpu"
    "github.com/shirou/gopsutil/v3/disk"
    "github.com/shirou/gopsutil/v3/load"
    "github.com/shirou/gopsutil/v3/mem"
    "github.com/shirou/gopsutil/v3/net"
)

type CPUStats struct {
    UsagePercent float64 `json:"usage_percent"`
    Cores        int     `json:"cores"`
}

type MemoryStats struct {
    TotalBytes   uint64  `json:"total_bytes"`
    UsedBytes    uint64  `json:"used_bytes"`
    UsagePercent float64 `json:"usage_percent"`
}

type LoadStats struct {
    Load1  float64 `json:"load1"`
    Load5  float64 `json:"load5"`
    Load15 float64 `json:"load15"`
}

type NetworkStats struct {
    BytesSentPerSec uint64 `json:"bytes_sent_per_sec"`
    BytesRecvPerSec uint64 `json:"bytes_recv_per_sec"`
}

type DiskStats struct {
    Path         string  `json:"path"`
    TotalBytes   uint64  `json:"total_bytes"`
    UsedBytes    uint64  `json:"used_bytes"`
    UsagePercent float64 `json:"usage_percent"`
}

type SystemStats struct {
    CPU     CPUStats      `json:"cpu"`
    Memory  MemoryStats   `json:"memory"`
    Load    LoadStats     `json:"load"`
    Network NetworkStats  `json:"network"`
    Disks   []DiskStats   `json:"disks"`
}

type SystemStatsCollector struct {
    lastNetIO     net.IOCountersStat
    lastCollectAt time.Time
    mu            sync.Mutex
}

func NewSystemStatsCollector() *SystemStatsCollector {
    return &SystemStatsCollector{}
}

func (c *SystemStatsCollector) Collect() (*SystemStats, error) {
    // 实现采集逻辑
}
```

### 修改文件

**`api/internal/handler/websocket.go`**

新增 `SystemStats` 方法：

```go
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

    // 立即发送一次
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

func (h *WebSocketHandler) sendSystemStats(conn *websocket.Conn) error {
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
```

**`api/internal/handler/router.go`**

注册路由：

```go
ws.GET("/system-stats", wsHandler.SystemStats)
```

### 依赖

```bash
go get github.com/shirou/gopsutil/v3
```

## 前端实现

### 新增文件

**`web/src/components/SystemStatsCard.vue`**

系统资源卡片组件，接收 WebSocket 推送的数据并渲染左右分栏布局。

**`web/src/api/system-stats.ts`**

WebSocket 连接管理：

```typescript
let ws: WebSocket | null = null

export function connectSystemStatsWs(onMessage: (data: SystemStats) => void) {
  const wsUrl = `ws://${window.location.host}/ws/system-stats`
  ws = new WebSocket(wsUrl)

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    onMessage(data)
  }
}

export function disconnectSystemStatsWs() {
  if (ws) {
    ws.close()
    ws = null
  }
}
```

**`web/src/types/system-stats.ts`**

类型定义：

```typescript
export interface CPUStats {
  usage_percent: number
  cores: number
}

export interface MemoryStats {
  total_bytes: number
  used_bytes: number
  usage_percent: number
}

export interface LoadStats {
  load1: number
  load5: number
  load15: number
}

export interface NetworkStats {
  bytes_sent_per_sec: number
  bytes_recv_per_sec: number
}

export interface DiskStats {
  path: string
  total_bytes: number
  used_bytes: number
  usage_percent: number
}

export interface SystemStats {
  type: string
  timestamp: string
  cpu: CPUStats
  memory: MemoryStats
  load: LoadStats
  network: NetworkStats
  disks: DiskStats[]
}
```

### 修改文件

**`web/src/views/Dashboard.vue`**

```vue
<template>
  <div class="dashboard">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <!-- 现有的 4 个卡片 -->
    </el-row>

    <!-- 新增：系统资源卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="24">
        <SystemStatsCard :stats="systemStats" />
      </el-col>
    </el-row>

    <!-- 图表区域 -->
    <el-row :gutter="20">
      <!-- 现有的图表 -->
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import SystemStatsCard from '@/components/SystemStatsCard.vue'
import { connectSystemStatsWs, disconnectSystemStatsWs } from '@/api/system-stats'
import type { SystemStats } from '@/types/system-stats'

const systemStats = ref<SystemStats | null>(null)

onMounted(() => {
  connectSystemStatsWs((data) => {
    systemStats.value = data
  })
})

onUnmounted(() => {
  disconnectSystemStatsWs()
})
</script>
```

## 文件清单

### 后端

| 操作 | 文件 |
|------|------|
| 新增 | `api/internal/service/system_stats.go` |
| 修改 | `api/internal/handler/websocket.go` |
| 修改 | `api/internal/handler/router.go` |

### 前端

| 操作 | 文件 |
|------|------|
| 新增 | `web/src/components/SystemStatsCard.vue` |
| 新增 | `web/src/api/system-stats.ts` |
| 新增 | `web/src/types/system-stats.ts` |
| 修改 | `web/src/views/Dashboard.vue` |

## 测试要点

1. WebSocket 连接正常建立和断开
2. 数据每秒更新一次
3. CPU、内存、负载、网络、磁盘数据准确
4. 多磁盘分区正确显示
5. 页面切换时 WebSocket 正确断开
6. 网络速率计算正确（基于差值）
7. 格式化显示符合规则
