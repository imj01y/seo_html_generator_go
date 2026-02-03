# 系统资源实时监控实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在仪表盘页面增加系统资源（CPU、内存、硬盘、网络、负载）实时监控卡片

**Architecture:** 后端使用 gopsutil 库采集系统资源，通过 WebSocket 每秒推送；前端新增 SystemStatsCard 组件接收并展示

**Tech Stack:** Go 1.24 + gopsutil/v3 + gorilla/websocket | Vue 3 + TypeScript + Element Plus

---

## Task 1: 添加 gopsutil 依赖

**Files:**
- Modify: `api/go.mod`

**Step 1: 安装 gopsutil 依赖**

Run:
```bash
cd api && go get github.com/shirou/gopsutil/v3
```

Expected: go.mod 中新增 gopsutil 依赖

**Step 2: 验证依赖安装**

Run:
```bash
cd api && go mod tidy && grep gopsutil go.mod
```

Expected: 输出包含 `github.com/shirou/gopsutil/v3`

**Step 3: Commit**

```bash
git add api/go.mod api/go.sum
git commit -m "chore(api): 添加 gopsutil 系统监控依赖"
```

---

## Task 2: 创建 SystemStatsCollector

**Files:**
- Create: `api/internal/service/system_stats.go`

**Step 1: 创建数据结构和采集器**

```go
package core

import (
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// CPUStats CPU 统计
type CPUStats struct {
	UsagePercent float64 `json:"usage_percent"`
	Cores        int     `json:"cores"`
}

// MemoryStats 内存统计
type MemoryStats struct {
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// LoadStats 系统负载统计
type LoadStats struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// NetworkStats 网络统计
type NetworkStats struct {
	BytesSentPerSec uint64 `json:"bytes_sent_per_sec"`
	BytesRecvPerSec uint64 `json:"bytes_recv_per_sec"`
}

// DiskStats 磁盘统计
type DiskStats struct {
	Path         string  `json:"path"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// SystemStats 系统统计汇总
type SystemStats struct {
	CPU     CPUStats      `json:"cpu"`
	Memory  MemoryStats   `json:"memory"`
	Load    LoadStats     `json:"load"`
	Network NetworkStats  `json:"network"`
	Disks   []DiskStats   `json:"disks"`
}

// SystemStatsCollector 系统统计采集器
type SystemStatsCollector struct {
	lastNetBytesSent uint64
	lastNetBytesRecv uint64
	lastCollectAt    time.Time
	mu               sync.Mutex
}

// NewSystemStatsCollector 创建系统统计采集器
func NewSystemStatsCollector() *SystemStatsCollector {
	return &SystemStatsCollector{}
}

// Collect 采集系统统计
func (c *SystemStatsCollector) Collect() (*SystemStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := &SystemStats{}

	// 采集 CPU
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		stats.CPU.UsagePercent = cpuPercent[0]
	}
	stats.CPU.Cores = runtime.NumCPU()

	// 采集内存
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		stats.Memory.TotalBytes = memInfo.Total
		stats.Memory.UsedBytes = memInfo.Used
		stats.Memory.UsagePercent = memInfo.UsedPercent
	}

	// 采集负载 (Windows 不支持，返回 0)
	loadInfo, err := load.Avg()
	if err == nil && loadInfo != nil {
		stats.Load.Load1 = loadInfo.Load1
		stats.Load.Load5 = loadInfo.Load5
		stats.Load.Load15 = loadInfo.Load15
	}

	// 采集网络
	netIO, err := net.IOCounters(false)
	if err == nil && len(netIO) > 0 {
		now := time.Now()
		if !c.lastCollectAt.IsZero() {
			elapsed := now.Sub(c.lastCollectAt).Seconds()
			if elapsed > 0 {
				stats.Network.BytesSentPerSec = uint64(float64(netIO[0].BytesSent-c.lastNetBytesSent) / elapsed)
				stats.Network.BytesRecvPerSec = uint64(float64(netIO[0].BytesRecv-c.lastNetBytesRecv) / elapsed)
			}
		}
		c.lastNetBytesSent = netIO[0].BytesSent
		c.lastNetBytesRecv = netIO[0].BytesRecv
		c.lastCollectAt = now
	}

	// 采集磁盘
	partitions, err := disk.Partitions(false)
	if err == nil {
		for _, p := range partitions {
			usage, err := disk.Usage(p.Mountpoint)
			if err == nil {
				stats.Disks = append(stats.Disks, DiskStats{
					Path:         p.Mountpoint,
					TotalBytes:   usage.Total,
					UsedBytes:    usage.Used,
					UsagePercent: usage.UsedPercent,
				})
			}
		}
	}

	return stats, nil
}
```

**Step 2: 验证编译**

Run:
```bash
cd api && go build ./...
```

Expected: 编译成功，无错误

**Step 3: Commit**

```bash
git add api/internal/service/system_stats.go
git commit -m "feat(api): 添加 SystemStatsCollector 系统资源采集器"
```

---

## Task 3: 添加 WebSocket 端点

**Files:**
- Modify: `api/internal/handler/websocket.go`

**Step 1: 在 WebSocketHandler 结构体中添加字段**

在 `websocket.go` 的 `WebSocketHandler` 结构体中添加 `systemStats` 字段：

```go
// WebSocketHandler WebSocket 处理器
type WebSocketHandler struct {
	templateFuncs *core.TemplateFuncsManager
	poolManager   *core.PoolManager
	systemStats   *core.SystemStatsCollector  // 新增
}

// NewWebSocketHandler 创建 WebSocket 处理器
func NewWebSocketHandler(templateFuncs *core.TemplateFuncsManager, poolManager *core.PoolManager, systemStats *core.SystemStatsCollector) *WebSocketHandler {
	return &WebSocketHandler{
		templateFuncs: templateFuncs,
		poolManager:   poolManager,
		systemStats:   systemStats,  // 新增
	}
}
```

**Step 2: 添加 SystemStats WebSocket 处理方法**

在 `websocket.go` 文件末尾添加：

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
```

**Step 3: 验证编译**

Run:
```bash
cd api && go build ./...
```

Expected: 编译失败（router.go 调用 NewWebSocketHandler 参数不匹配）

---

## Task 4: 更新路由注册

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 在 Dependencies 中添加 SystemStatsCollector**

在 `router.go` 的 `Dependencies` 结构体中添加：

```go
// Dependencies holds all dependencies required by the API handlers
type Dependencies struct {
	DB               *sqlx.DB
	Redis            *redis.Client
	Config           *config.Config
	TemplateAnalyzer *core.TemplateAnalyzer
	TemplateFuncs    *core.TemplateFuncsManager
	Scheduler        *core.Scheduler
	TemplateCache    *core.TemplateCache
	Monitor          *core.Monitor
	PoolManager      *core.PoolManager
	SystemStats      *core.SystemStatsCollector  // 新增
}
```

**Step 2: 更新 NewWebSocketHandler 调用**

找到 `SetupRouter` 函数中的这一行（约第 362 行）：

```go
wsHandler := NewWebSocketHandler(deps.TemplateFuncs, deps.PoolManager)
```

修改为：

```go
wsHandler := NewWebSocketHandler(deps.TemplateFuncs, deps.PoolManager, deps.SystemStats)
```

**Step 3: 注册新的 WebSocket 路由**

在 WebSocket routes 区块（约第 369 行后）添加：

```go
r.GET("/ws/system-stats", wsHandler.SystemStats)
```

**Step 4: 验证编译**

Run:
```bash
cd api && go build ./...
```

Expected: 编译成功

**Step 5: Commit**

```bash
git add api/internal/handler/websocket.go api/internal/handler/router.go
git commit -m "feat(api): 添加 /ws/system-stats WebSocket 端点"
```

---

## Task 5: 在 main.go 中初始化 SystemStatsCollector

**Files:**
- Modify: `api/cmd/server/main.go` 或项目入口文件

**Step 1: 查找入口文件**

Run:
```bash
find api -name "main.go" | head -5
```

**Step 2: 初始化 SystemStatsCollector**

在创建 Dependencies 的地方，添加：

```go
systemStats := core.NewSystemStatsCollector()
```

并将其添加到 Dependencies 初始化中：

```go
deps := &api.Dependencies{
    // ... 现有字段 ...
    SystemStats: systemStats,
}
```

**Step 3: 验证编译和运行**

Run:
```bash
cd api && go build ./... && echo "Build successful"
```

Expected: 编译成功

**Step 4: Commit**

```bash
git add api/cmd/server/main.go
git commit -m "feat(api): 初始化 SystemStatsCollector"
```

---

## Task 6: 创建前端类型定义

**Files:**
- Create: `web/src/types/system-stats.ts`

**Step 1: 创建类型定义文件**

```typescript
/**
 * 系统资源监控类型定义
 */

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

**Step 2: Commit**

```bash
git add web/src/types/system-stats.ts
git commit -m "feat(web): 添加系统资源监控类型定义"
```

---

## Task 7: 创建 WebSocket API

**Files:**
- Create: `web/src/api/system-stats.ts`

**Step 1: 创建 WebSocket 连接管理**

```typescript
/**
 * 系统资源监控 WebSocket API
 */

import type { SystemStats } from '@/types/system-stats'

let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

export function connectSystemStatsWs(onMessage: (data: SystemStats) => void): void {
  // 清理之前的连接
  disconnectSystemStatsWs()

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws/system-stats`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    console.log('[SystemStats] WebSocket connected')
  }

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data) as SystemStats
      onMessage(data)
    } catch (e) {
      console.error('[SystemStats] Failed to parse message:', e)
    }
  }

  ws.onerror = (error) => {
    console.error('[SystemStats] WebSocket error:', error)
  }

  ws.onclose = () => {
    console.log('[SystemStats] WebSocket closed')
    ws = null
  }
}

export function disconnectSystemStatsWs(): void {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (ws) {
    ws.close()
    ws = null
  }
}
```

**Step 2: Commit**

```bash
git add web/src/api/system-stats.ts
git commit -m "feat(web): 添加系统资源监控 WebSocket API"
```

---

## Task 8: 创建 SystemStatsCard 组件

**Files:**
- Create: `web/src/components/SystemStatsCard.vue`

**Step 1: 创建组件**

```vue
<template>
  <div class="system-stats-card" v-if="stats">
    <div class="card-header">
      <span class="title">系统资源</span>
    </div>
    <div class="card-body">
      <!-- 左侧：核心指标 -->
      <div class="left-panel">
        <!-- CPU -->
        <div class="stat-row">
          <span class="stat-label">CPU</span>
          <el-progress
            :percentage="stats.cpu.usage_percent"
            :stroke-width="12"
            :color="getProgressColor(stats.cpu.usage_percent)"
            class="stat-progress"
          />
          <span class="stat-value">{{ stats.cpu.usage_percent.toFixed(1) }}%</span>
          <span class="stat-extra">{{ stats.cpu.cores }}核</span>
        </div>
        <!-- 内存 -->
        <div class="stat-row">
          <span class="stat-label">内存</span>
          <el-progress
            :percentage="stats.memory.usage_percent"
            :stroke-width="12"
            :color="getProgressColor(stats.memory.usage_percent)"
            class="stat-progress"
          />
          <span class="stat-value">{{ stats.memory.usage_percent.toFixed(1) }}%</span>
          <span class="stat-extra">{{ formatMemoryGB(stats.memory.used_bytes) }}/{{ formatMemoryGB(stats.memory.total_bytes) }}G</span>
        </div>
        <!-- 负载 -->
        <div class="stat-row">
          <span class="stat-label">负载</span>
          <span class="stat-load">
            {{ stats.load.load1.toFixed(2) }} / {{ stats.load.load5.toFixed(2) }} / {{ stats.load.load15.toFixed(2) }}
          </span>
        </div>
        <!-- 网络 -->
        <div class="stat-row">
          <span class="stat-label">网络</span>
          <span class="stat-network">
            <span class="upload">↑ {{ formatSpeed(stats.network.bytes_sent_per_sec) }}</span>
            <span class="download">↓ {{ formatSpeed(stats.network.bytes_recv_per_sec) }}</span>
          </span>
        </div>
      </div>
      <!-- 右侧：磁盘 -->
      <div class="right-panel">
        <div class="panel-title">磁盘</div>
        <div class="disk-list">
          <div class="disk-row" v-for="disk in stats.disks" :key="disk.path">
            <span class="disk-path">{{ disk.path }}</span>
            <el-progress
              :percentage="disk.usage_percent"
              :stroke-width="10"
              :color="getProgressColor(disk.usage_percent)"
              class="disk-progress"
            />
            <span class="disk-percent">{{ Math.round(disk.usage_percent) }}%</span>
            <span class="disk-size">{{ formatDiskSize(disk.used_bytes) }}/{{ formatDiskSize(disk.total_bytes) }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
  <div class="system-stats-card loading" v-else>
    <el-skeleton :rows="4" animated />
  </div>
</template>

<script setup lang="ts">
import type { SystemStats } from '@/types/system-stats'

defineProps<{
  stats: SystemStats | null
}>()

// 根据百分比返回进度条颜色
function getProgressColor(percent: number): string {
  if (percent >= 90) return '#f56c6c'
  if (percent >= 70) return '#e6a23c'
  return '#67c23a'
}

// 格式化内存（字节转GB，保留2位小数）
function formatMemoryGB(bytes: number): string {
  return (bytes / (1024 * 1024 * 1024)).toFixed(2)
}

// 格式化网络速率
function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec >= 1024 * 1024 * 1024) {
    return (bytesPerSec / (1024 * 1024 * 1024)).toFixed(1) + ' GB/s'
  }
  if (bytesPerSec >= 1024 * 1024) {
    return (bytesPerSec / (1024 * 1024)).toFixed(1) + ' MB/s'
  }
  if (bytesPerSec >= 1024) {
    return (bytesPerSec / 1024).toFixed(1) + ' KB/s'
  }
  return bytesPerSec + ' B/s'
}

// 格式化磁盘大小
function formatDiskSize(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024 * 1024) {
    return Math.round(bytes / (1024 * 1024 * 1024 * 1024)) + 'TB'
  }
  return Math.round(bytes / (1024 * 1024 * 1024)) + 'GB'
}
</script>

<style lang="scss" scoped>
.system-stats-card {
  background-color: #fff;
  border-radius: 8px;
  padding: 16px 20px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);

  &.loading {
    min-height: 160px;
  }

  .card-header {
    margin-bottom: 16px;

    .title {
      font-size: 16px;
      font-weight: 600;
      color: #303133;
    }
  }

  .card-body {
    display: flex;
    gap: 24px;
  }

  .left-panel {
    flex: 1;
    min-width: 0;
  }

  .right-panel {
    flex: 1;
    min-width: 0;
    border-left: 1px solid #ebeef5;
    padding-left: 24px;

    .panel-title {
      font-size: 14px;
      font-weight: 500;
      color: #606266;
      margin-bottom: 12px;
    }
  }

  .stat-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;

    &:last-child {
      margin-bottom: 0;
    }

    .stat-label {
      width: 36px;
      font-size: 14px;
      color: #606266;
      flex-shrink: 0;
    }

    .stat-progress {
      flex: 1;
      min-width: 0;
    }

    .stat-value {
      width: 50px;
      text-align: right;
      font-size: 14px;
      font-weight: 500;
      color: #303133;
      flex-shrink: 0;
    }

    .stat-extra {
      width: 100px;
      text-align: right;
      font-size: 13px;
      color: #909399;
      flex-shrink: 0;
    }

    .stat-load {
      flex: 1;
      font-size: 14px;
      color: #303133;
    }

    .stat-network {
      flex: 1;
      display: flex;
      gap: 16px;
      font-size: 14px;

      .upload {
        color: #e6a23c;
      }

      .download {
        color: #67c23a;
      }
    }
  }

  .disk-list {
    .disk-row {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 8px;

      &:last-child {
        margin-bottom: 0;
      }

      .disk-path {
        width: 40px;
        font-size: 13px;
        color: #606266;
        flex-shrink: 0;
      }

      .disk-progress {
        flex: 1;
        min-width: 0;
      }

      .disk-percent {
        width: 36px;
        text-align: right;
        font-size: 13px;
        font-weight: 500;
        color: #303133;
        flex-shrink: 0;
      }

      .disk-size {
        width: 90px;
        text-align: right;
        font-size: 12px;
        color: #909399;
        flex-shrink: 0;
      }
    }
  }
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/components/SystemStatsCard.vue
git commit -m "feat(web): 添加 SystemStatsCard 组件"
```

---

## Task 9: 集成到 Dashboard

**Files:**
- Modify: `web/src/views/Dashboard.vue`

**Step 1: 导入组件和 API**

在 `<script setup>` 区块顶部添加导入：

```typescript
import SystemStatsCard from '@/components/SystemStatsCard.vue'
import { connectSystemStatsWs, disconnectSystemStatsWs } from '@/api/system-stats'
import type { SystemStats } from '@/types/system-stats'
```

**Step 2: 添加响应式状态**

在 `<script setup>` 中添加：

```typescript
const systemStats = ref<SystemStats | null>(null)
```

**Step 3: 连接 WebSocket**

在 `onMounted` 中添加：

```typescript
connectSystemStatsWs((data) => {
  systemStats.value = data
})
```

在 `onUnmounted` 中添加：

```typescript
disconnectSystemStatsWs()
```

**Step 4: 在模板中添加卡片**

在统计卡片 `</el-row>` 后，图表区域 `<el-row>` 前添加：

```vue
<!-- 系统资源卡片 -->
<el-row :gutter="20" class="stats-row">
  <el-col :span="24">
    <SystemStatsCard :stats="systemStats" />
  </el-col>
</el-row>
```

**Step 5: 验证前端编译**

Run:
```bash
cd web && npm run build
```

Expected: 编译成功

**Step 6: Commit**

```bash
git add web/src/views/Dashboard.vue
git commit -m "feat(web): 在仪表盘集成系统资源监控卡片"
```

---

## Task 10: 端到端测试

**Step 1: 启动后端**

Run:
```bash
cd api && go run ./cmd/server
```

**Step 2: 启动前端**

Run:
```bash
cd web && npm run dev
```

**Step 3: 手动测试**

1. 打开浏览器访问仪表盘页面
2. 确认系统资源卡片显示
3. 确认 CPU、内存、负载、网络、磁盘数据每秒更新
4. 打开浏览器开发者工具，确认 WebSocket 连接正常
5. 切换到其他页面，确认 WebSocket 断开

**Step 4: 最终提交**

```bash
git add -A
git commit -m "feat: 完成系统资源实时监控功能"
```

---

## 文件清单

| 操作 | 文件 |
|------|------|
| Create | `api/internal/service/system_stats.go` |
| Modify | `api/internal/handler/websocket.go` |
| Modify | `api/internal/handler/router.go` |
| Modify | `api/cmd/server/main.go` |
| Create | `web/src/types/system-stats.ts` |
| Create | `web/src/api/system-stats.ts` |
| Create | `web/src/components/SystemStatsCard.vue` |
| Modify | `web/src/views/Dashboard.vue` |

## 依赖

- Go: `github.com/shirou/gopsutil/v3`
