# PoolManager 整合设计

## 概述

将 DataManager 的功能整合到 PoolManager 中，统一管理 titles、contents、keywords、images 四种数据库缓存池。

NumberPool 保持独立，专门管理代码生成的随机数。

## 设计决策

### PoolManager（数据库数据缓存）

| 数据类型 | 获取模式 | 消费后处理 | 原因 |
|---------|---------|-----------|------|
| titles | FIFO 顺序消费 | 标记 status=0 | 内容不重复 |
| contents | FIFO 顺序消费 | 标记 status=0 | 内容不重复 |
| keywords | 随机获取 | 不标记 | 允许重复使用 |
| images | 随机获取 | 不标记 | 允许重复使用 |

### NumberPool（独立，代码生成数据）

| 数据类型 | 获取模式 | 补充方式 | 原因 |
|---------|---------|---------|------|
| numbers | 随机消费 | 低水位自动补充 | 预生成避免高并发卡顿 |

NumberPool 使用 ObjectPool[int] 作为底层实现，两者职责分离：
- **ObjectPool[T]**：通用对象池（泛型），负责预生成 + 低水位补充
- **NumberPool**：定义 14 种范围，提供 Get(min, max) 路由

## 新架构

```
┌─────────────────────────────────────────────────────────────┐
│                      PoolManager                             │
│                   (数据库数据缓存)                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────┐    ┌─────────────────┐                │
│  │ MemoryPool      │    │ MemoryPool      │                │
│  │ (titles)        │    │ (contents)      │                │
│  │ - FIFO Pop      │    │ - FIFO Pop      │                │
│  │ - 标记 status=0 │    │ - 标记 status=0 │                │
│  └─────────────────┘    └─────────────────┘                │
│                                                             │
│  ┌─────────────────┐    ┌─────────────────┐                │
│  │ []string 切片   │    │ []string 切片   │                │
│  │ (keywords)      │    │ (images)        │                │
│  │ - 随机获取      │    │ - 随机获取      │                │
│  │ - 可重复使用    │    │ - 可重复使用    │                │
│  │ - 定时刷新      │    │ - 定时刷新      │                │
│  └─────────────────┘    └─────────────────┘                │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                 后台任务                              │   │
│  │  - refillLoop: 补充 titles/contents                  │   │
│  │  - refreshLoop: 刷新 keywords/images                 │   │
│  │  - updateWorker: 异步更新 status=0                   │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                 辅助组件                              │   │
│  │  - EmojiManager: Emoji 管理                          │   │
│  │  - HTMLEntityEncoder: HTML 编码（用于 keywords）     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                      NumberPool                              │
│                   (代码生成数据)                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ map[string]*ObjectPool[int]                          │   │
│  │ - 14种预定义范围 (0-9, 1-10, 10-99, ...)            │   │
│  │ - 预生成 200000 个/范围                              │   │
│  │ - 低水位 40% 时后台补充 (8个worker)                  │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 数据结构

### MemoryPool (已有，用于 titles/contents)

```go
type MemoryPool struct {
    items    []PoolItem      // FIFO 队列
    mu       sync.RWMutex
    groupID  int
    poolType string
    maxSize  int
}
```

### keywords/images 存储（简单切片，无需新结构）

keywords 和 images 只需要：
- 存储数据
- 随机获取（不移除）
- 定时刷新

直接使用 `map[int][]string` 即可，无需创建新的 Pool 结构。

### 扩展的 PoolManager

```go
type PoolManager struct {
    // 消费型池（FIFO，消费后标记）- 使用 MemoryPool
    titles   map[int]*MemoryPool
    contents map[int]*MemoryPool

    // 复用型数据（随机获取，可重复）- 直接使用切片
    keywords    map[int][]string      // 已编码版本
    rawKeywords map[int][]string      // 未编码版本
    images      map[int][]string
    keywordsMu  sync.RWMutex          // keywords 专用锁
    imagesMu    sync.RWMutex          // images 专用锁

    // 配置
    config   *PoolManagerConfig
    db       *sqlx.DB

    // 辅助组件
    encoder      *HTMLEntityEncoder
    emojiManager *EmojiManager

    // 后台任务
    updateCh chan UpdateTask
    ctx      context.Context
    cancel   context.CancelFunc
    wg       sync.WaitGroup
    stopped  atomic.Bool
    mu       sync.RWMutex
}
```

### 扩展的配置

```go
type PoolManagerConfig struct {
    // 消费型池配置（titles/contents）
    TitlesSize       int           // 标题池大小，默认 5000
    ContentsSize     int           // 正文池大小，默认 5000
    Threshold        int           // 补充阈值，默认 1000
    RefillIntervalMs int           // 补充检查间隔，默认 1000ms

    // 复用型池配置（keywords/images）
    KeywordsSize      int          // 关键词池大小，默认 50000
    ImagesSize        int          // 图片池大小，默认 50000
    RefreshIntervalMs int          // 刷新间隔，默认 300000ms (5分钟)
}
```

注：NumberPool 的配置保持在代码中硬编码（14 种范围、池大小 200000、低水位 40%），无需数据库配置。

## API 变更

### 新增方法

```go
// 关键词获取
func (m *PoolManager) GetRandomKeywords(groupID int, count int) []string
func (m *PoolManager) GetRawKeywords(groupID int, count int) []string

// 图片获取
func (m *PoolManager) GetRandomImage(groupID int) string
func (m *PoolManager) GetImages(groupID int) []string

// Emoji 获取
func (m *PoolManager) GetRandomEmoji() string
func (m *PoolManager) GetRandomEmojiExclude(exclude map[string]bool) string

// 刷新
func (m *PoolManager) RefreshKeywords(groupID int) error
func (m *PoolManager) RefreshImages(groupID int) error
```

注：随机数获取继续使用独立的 `NumberPool.Get(min, max)`。

### 保留方法

```go
// 标题/正文获取（已有）
func (m *PoolManager) Pop(poolType string, groupID int) (string, error)

// 配置管理（已有）
func (m *PoolManager) Reload(ctx context.Context) error
func (m *PoolManager) GetStats() map[string]interface{}
func (m *PoolManager) GetConfig() *PoolManagerConfig
```

## 数据库配置表更新

```sql
ALTER TABLE pool_config
ADD COLUMN keywords_size INT NOT NULL DEFAULT 50000 COMMENT '关键词池大小',
ADD COLUMN images_size INT NOT NULL DEFAULT 50000 COMMENT '图片池大小',
ADD COLUMN refresh_interval_ms INT NOT NULL DEFAULT 300000 COMMENT '刷新间隔(毫秒)';
```

## 文件改动清单

### 修改文件

| 文件 | 改动 |
|------|------|
| `api/internal/service/pool_manager.go` | 添加 keywords/images 及相关方法 |
| `api/internal/service/pool_config.go` | 扩展配置结构 |
| `api/internal/handler/pool.go` | 扩展 API |
| `api/cmd/main.go` | 移除 DataManager，使用 PoolManager |
| `api/internal/handler/page.go` | 使用 PoolManager 获取 keywords/images |
| `api/internal/handler/router.go` | 移除 DataManager 依赖 |
| `migrations/005_pool_config.sql` | 添加新字段 |
| `web/src/api/cache-pool.ts` | 扩展配置字段 |
| `web/src/views/settings/Settings.vue` | 扩展 UI |

### 删除文件

| 文件 | 原因 |
|------|------|
| `api/internal/service/data_manager.go` | 功能已整合到 PoolManager |

### 保留文件

| 文件 | 原因 |
|------|------|
| `api/internal/service/number_pool.go` | 独立管理随机数，不整合 |
| `api/internal/service/object_pool.go` | 通用对象池，被 NumberPool 使用 |

### 新增文件

无（功能整合到现有文件）

## 迁移策略

1. 扩展 PoolManager 支持 keywords/images
2. 更新调用方使用 PoolManager
3. 删除 DataManager
4. 更新前端配置界面

## 与原设计对比

| 维度 | 原设计 | 新设计 |
|-----|-------|-------|
| 数据库缓存管理器 | 2个 (PoolManager + DataManager) | 1个 (PoolManager) |
| 随机数管理器 | NumberPool（独立） | NumberPool（保持独立） |
| 代码复杂度 | 分散，职责重叠 | 统一，职责清晰 |
| 配置管理 | 分散配置 | 统一配置表 |
| API 接口 | 两套接口 | 统一接口 |
