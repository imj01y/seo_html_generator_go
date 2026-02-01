# 标题和正文内存缓存池重构设计

## 概述

将标题和正文的缓存池从 "Python 补充 + Redis 中间层 + Go 消费" 重构为 "Go 内存缓存 + Go 自补充"，简化架构，减少依赖。

## 设计决策

| 项目 | 决策 |
|-----|------|
| 补充职责 | Go 负责（原 Python） |
| 触发机制 | 后台 goroutine 定时检查（1秒） |
| 缓存方式 | 内存缓存（原 Redis） |
| 数据结构 | slice + mutex（支持动态调整） |
| 部署模式 | 单实例 |
| 配置存储 | 数据库 |
| 配置更新 | 后台调用 Go API 触发 |
| 现有代码 | 删除 Python PoolFiller + Redis 中间层 |

## 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Go API                                │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                 PoolManager                            │  │
│  │  ┌─────────────────┐    ┌─────────────────┐           │  │
│  │  │ titles (slice)  │    │ contents (slice)│           │  │
│  │  │ + mutex         │    │ + mutex         │           │  │
│  │  └────────┬────────┘    └────────┬────────┘           │  │
│  │           │ Pop()                │ Pop()              │  │
│  │           ▼                      ▼                    │  │
│  │  ┌─────────────────────────────────────────────────┐  │  │
│  │  │              Refiller (goroutine)               │  │  │
│  │  │  - 每 1 秒检查队列长度                           │  │  │
│  │  │  - 低于阈值时从 DB 补充                          │  │  │
│  │  │  - 异步更新已消费数据的 status                   │  │  │
│  │  └─────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────┘  │
│           ↑                              ↑                   │
│           │ 查询 status=1               │ UPDATE status=0   │
│           ▼                              ▼                   │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                      MySQL                             │  │
│  │  titles 表 / contents 表 / pool_config 表              │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
           ↑
           │ POST /api/pool/reload
           │
┌──────────┴──────────┐
│   后台管理页面        │
│   修改池配置          │
└─────────────────────┘
```

## 核心数据结构

### MemoryPool 结构

```go
type PoolItem struct {
    ID   int64
    Text string
}

type MemoryPool struct {
    items     []PoolItem
    mu        sync.RWMutex
    groupID   int
    poolType  string  // "titles" 或 "contents"
}

// Pop 从队列头部取出一个元素
func (p *MemoryPool) Pop() (PoolItem, bool)

// Push 向队列尾部添加元素
func (p *MemoryPool) Push(items []PoolItem)

// Len 返回当前队列长度
func (p *MemoryPool) Len() int

// Resize 调整池大小（配置更新时调用）
func (p *MemoryPool) Resize(newSize int)
```

### PoolConfig 配置结构

```go
type PoolConfig struct {
    TitlesSize      int           // 标题池大小，默认 5000
    ContentsSize    int           // 正文池大小，默认 5000
    Threshold       int           // 补充阈值，默认 1000 (20%)
    RefillInterval  time.Duration // 检查间隔，默认 1 秒
}
```

### 数据库配置表

```sql
CREATE TABLE pool_config (
    id INT PRIMARY KEY DEFAULT 1,
    titles_size INT DEFAULT 5000,
    contents_size INT DEFAULT 5000,
    threshold INT DEFAULT 1000,
    refill_interval_ms INT DEFAULT 1000,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

## PoolManager 管理器

```go
type PoolManager struct {
    titles      map[int]*MemoryPool  // groupID -> pool
    contents    map[int]*MemoryPool  // groupID -> pool
    config      *PoolConfig
    db          *sqlx.DB
    mu          sync.RWMutex
    ctx         context.Context
    cancel      context.CancelFunc
    updateCh    chan UpdateTask      // 异步状态更新
    wg          sync.WaitGroup
}

// NewPoolManager 创建管理器
func NewPoolManager(db *sqlx.DB) *PoolManager

// Start 启动后台 goroutine（补充 + 状态更新）
func (m *PoolManager) Start()

// Stop 优雅停止
func (m *PoolManager) Stop()

// Pop 获取一条数据
func (m *PoolManager) Pop(poolType string, groupID int) (string, error)

// Reload 重新加载配置（API 调用触发）
func (m *PoolManager) Reload() error

// GetStats 获取池状态统计
func (m *PoolManager) GetStats() map[string]interface{}
```

### 后台 goroutine 职责

```go
func (m *PoolManager) refillLoop() {
    ticker := time.NewTicker(m.config.RefillInterval)  // 1 秒
    for {
        select {
        case <-ticker.C:
            m.checkAndRefill()
        case <-m.ctx.Done():
            return
        }
    }
}

func (m *PoolManager) checkAndRefill() {
    // 遍历所有分组的 titles 和 contents 池
    // 低于阈值时从 DB 补充
}
```

## API 接口

### 配置重载接口

```
POST /api/pool/reload
Response: { "success": true, "config": {...} }
```

### 池状态查询接口

```
GET /api/pool/stats
Response: {
    "titles": {
        "1": { "size": 5000, "current": 4500, "threshold": 1000 }
    },
    "contents": {
        "1": { "size": 5000, "current": 4800, "threshold": 1000 }
    },
    "config": {
        "titles_size": 5000,
        "contents_size": 5000,
        "threshold": 1000,
        "refill_interval_ms": 1000
    }
}
```

### 后台管理页面

在设置页面添加"缓存池配置"区域：
- 标题池大小输入框
- 正文池大小输入框
- 补充阈值输入框
- 保存按钮（保存到 DB 后调用 `/api/pool/reload`）

## 文件改动清单

### 新增文件

| 文件 | 说明 |
|------|------|
| `api/internal/service/memory_pool.go` | 内存池实现 |
| `api/internal/service/pool_manager.go` | 池管理器 |
| `api/internal/handler/pool.go` | API 接口 |
| `migrations/xxx_add_pool_config.sql` | 配置表迁移 |

### 修改文件

| 文件 | 改动 |
|------|------|
| `api/cmd/main.go` | 初始化 PoolManager 替代 PoolConsumer |
| `api/internal/handler/page.go` | 使用 PoolManager.Pop() |
| `api/internal/handler/router.go` | 添加 /api/pool 路由 |
| `web/src/views/settings/Settings.vue` | 添加池配置 UI |

### 删除文件

| 文件 | 原因 |
|------|------|
| `api/internal/service/pool_consumer.go` | 被 PoolManager 替代 |
| `content_worker/core/pool_filler.py` | 不再需要 Python 补充 |

## 与原设计对比

| 维度 | 原设计 | 新设计 |
|-----|-------|-------|
| 补充职责 | Python PoolFiller | Go PoolManager |
| 缓存层 | Redis List | Go 内存 slice |
| 服务依赖 | Go 依赖 Python + Redis | Go 自给自足 |
| 故障影响 | Python 挂掉需降级 | 无额外故障点 |
| 配置方式 | 代码硬编码 | 数据库 + 后台管理 |
| 动态调整 | 不支持 | 支持热更新 |

## 设计优势

1. **简化架构** - 移除 Python 补充和 Redis 中间层
2. **减少故障点** - Go 自己管理缓存，不依赖外部服务
3. **部署简化** - 单服务即可工作
4. **配置灵活** - 支持后台动态调整池大小
5. **性能更优** - 纯内存访问，无网络往返
