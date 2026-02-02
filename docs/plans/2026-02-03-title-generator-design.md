# 标题动态生成设计方案

> 日期：2026-02-03

## 概述

将标题生成方式从数据库加载改为动态生成，从关键词内存缓存中随机提取关键词和 emoji 组合生成标题。

## 需求

1. 从 Go 的关键词内存缓存中随机提取关键词、随机提取 emoji
2. 标题生成格式：`关键词1 + emoji1 + 关键词2 + emoji2 + 关键词3`
3. 后台协程负责生成标题并存入标题内存缓存池中
4. 标题池大小和生成线程数由用户在管理后台-缓存管理中配置
5. Go 渲染模板时从标题池中提取消费

## 设计决策

| 决策点 | 选择 |
|--------|------|
| 分组方式 | 按 groupID 分组，每个分组独立标题池 |
| 关键词版本 | 使用已编码版本（HTML 实体编码） |
| 数据库标题 | 完全弃用，只用动态生成 |
| 池实现方式 | Channel（Go 原生，简单可靠） |

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────┐
│                   TitleGenerator                     │
├─────────────────────────────────────────────────────┤
│ 输入：                                               │
│   - PoolManager.keywords[groupID] (已编码关键词)     │
│   - PoolManager.emojiManager (emoji库)              │
│                                                     │
│ 输出：                                               │
│   - pools map[int]*TitlePool (groupID -> 标题池)    │
│                                                     │
│ 生成格式：                                           │
│   关键词1 + emoji1 + 关键词2 + emoji2 + 关键词3      │
└─────────────────────────────────────────────────────┘
```

### 数据结构

```go
// pool_config.go 扩展
type CachePoolConfig struct {
    // ... 现有字段 ...

    // 标题生成配置（新增）
    TitlePoolSize         int `db:"title_pool_size"`          // 标题池大小，默认 5000
    TitleWorkers          int `db:"title_workers"`            // 生成协程数，默认 2
    TitleRefillIntervalMs int `db:"title_refill_interval_ms"` // 生成间隔(ms)，默认 500
    TitleThreshold        int `db:"title_threshold"`          // 补充阈值，默认 1000
}

// title_generator.go 新建
type TitleGenerator struct {
    pools       map[int]*TitlePool  // groupID -> 标题池
    poolManager *PoolManager        // 引用，获取关键词和emoji
    config      *CachePoolConfig
    mu          sync.RWMutex

    ctx         context.Context
    cancel      context.CancelFunc
    wg          sync.WaitGroup
}

type TitlePool struct {
    ch      chan string  // 带缓冲的 channel
    groupID int
}
```

### 生成逻辑

```go
func (g *TitleGenerator) generateTitle(groupID int) string {
    // 1. 获取 3 个随机编码关键词
    keywords := g.poolManager.GetRandomKeywords(groupID, 3)

    // 2. 获取 2 个随机 emoji（不重复）
    emoji1 := g.poolManager.GetRandomEmoji()
    emoji2 := g.poolManager.GetRandomEmojiExclude(map[string]bool{emoji1: true})

    // 3. 拼接：关键词1 + emoji1 + 关键词2 + emoji2 + 关键词3
    return keywords[0] + emoji1 + keywords[1] + emoji2 + keywords[2]
}
```

### 协程管理

```go
func (g *TitleGenerator) Start() {
    // 为每个 keyword group 启动 N 个生成协程
    for _, groupID := range keywordGroupIDs {
        pool := g.getOrCreatePool(groupID)

        for i := 0; i < g.config.TitleWorkers; i++ {
            g.wg.Add(1)
            go g.refillWorker(groupID, pool)
        }
    }
}

func (g *TitleGenerator) refillWorker(groupID int, pool *TitlePool) {
    defer g.wg.Done()
    ticker := time.NewTicker(time.Duration(g.config.TitleRefillIntervalMs) * time.Millisecond)

    for {
        select {
        case <-g.ctx.Done():
            return
        case <-ticker.C:
            // 检查是否需要补充
            if len(pool.ch) < g.config.TitleThreshold {
                g.fillPool(groupID, pool)
            }
        }
    }
}
```

### 系统集成

**PoolManager 集成**：

```go
// pool_manager.go 修改

type PoolManager struct {
    // ... 现有字段 ...

    titleGenerator *TitleGenerator  // 新增：标题生成器
}

// 修改 Pop 方法，titles 类型走生成器
func (m *PoolManager) Pop(poolType string, groupID int) (string, error) {
    if poolType == "titles" {
        return m.titleGenerator.Pop(groupID)
    }
    // contents 保持原有逻辑...
}
```

**TitleGenerator.Pop 方法**：

```go
func (g *TitleGenerator) Pop(groupID int) (string, error) {
    pool := g.getOrCreatePool(groupID)

    select {
    case title := <-pool.ch:
        return title, nil
    default:
        // 池空，同步生成一个返回
        return g.generateTitle(groupID), nil
    }
}
```

**启动顺序**（在 `PoolManager.Start` 中）：

```
1. 加载配置
2. 加载关键词到内存  ← 必须先完成
3. 加载 emoji 文件   ← 必须先完成
4. 启动 TitleGenerator  ← 依赖上面两步
5. 启动其他后台任务
```

## 数据库变更

```sql
ALTER TABLE pool_config ADD COLUMN title_pool_size INT DEFAULT 5000;
ALTER TABLE pool_config ADD COLUMN title_workers INT DEFAULT 2;
ALTER TABLE pool_config ADD COLUMN title_refill_interval_ms INT DEFAULT 500;
ALTER TABLE pool_config ADD COLUMN title_threshold INT DEFAULT 1000;
```

## 前端变更

在 `CacheManage.vue` 数据池配置 tab 中新增标题池配置卡片：

```
┌─────────────────────────────────────────┐
│ 标题池配置                               │
├─────────────────────────────────────────┤
│ 标题池大小      [5000    ] 条            │
│ 生成协程数      [2       ] 个            │
│ 生成间隔        [500     ] 毫秒          │
│ 补充阈值        [1000    ] 低于时触发补充 │
└─────────────────────────────────────────┘
```

## 代码清理

1. `pool_manager.go` 中删除 titles 相关的数据库加载逻辑
2. 删除 `validTables` 中的 `"titles"` 条目（不再需要数据库更新）
3. 移除 titles 的 MemoryPool 使用，改为 TitleGenerator

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/internal/service/title_generator.go` | 新建 | 标题生成器核心逻辑 |
| `api/internal/service/pool_manager.go` | 修改 | 集成 TitleGenerator，删除 titles 数据库逻辑 |
| `api/internal/service/pool_config.go` | 修改 | 新增 4 个配置字段 |
| `api/internal/service/memory_pool.go` | 修改 | 从 validTables 移除 titles |
| `api/internal/handler/pool_config.go` | 修改 | API 支持新字段 |
| `web/src/api/cache-pool.ts` | 修改 | 新增字段类型定义 |
| `web/src/views/cache/CacheManage.vue` | 修改 | 新增标题池配置卡片 |
| `migrations/xxx_add_title_config.sql` | 新增 | 数据库迁移脚本 |

## 配置默认值

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| title_pool_size | 5000 | 每个分组的预生成标题数量 |
| title_workers | 2 | 后台生成协程数量 |
| title_refill_interval_ms | 500 | 检查补充间隔 |
| title_threshold | 1000 | 低于此值触发补充 |
