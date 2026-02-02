# 统一对象池配置设计方案

> 日期：2026-02-03

## 概述

将标题池、cls类名池、url池、关键词表情池的配置统一化，每个池支持相同的 4 个配置项，并通过下拉菜单选择池类型进行配置，简化前端页面。

## 需求

1. 4 个对象池使用统一的配置结构：池大小、生成协程数、生成间隔、补充阈值
2. 前端使用下拉菜单选择要配置的池，而非每个池单独一个配置卡片
3. 正文池保持现有方式（从数据库加载），不纳入统一配置
4. 补充逻辑改为"低于阈值时补充到满"

## 设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 正文池 | 保持数据库加载 | 正文内容长，内存占用大（5000条约25MB） |
| 补充策略 | 补充到满 | 简化配置，移除 RefillBatch 参数 |
| 配置项数量 | 4 个 | 池大小、协程数、间隔、阈值 |
| 前端交互 | 下拉选择池 | 页面更简洁紧凑 |

## 配置范围

**纳入统一配置的池（4个）：**
- 标题池
- cls类名池
- url池
- 关键词表情池

**不纳入的池：**
- 正文池（保持现有配置方式）

## 现有生成逻辑（保持不变）

### cls类名池
```
生成格式：13位随机字符 + 空格 + 32位随机字符
字符集：abcdefghijklmnopqrstuvwxyz0123456789
示例：ab3kz9x2m1pqr 7h2jk9d0f3s5t8w1q4e6r2y7u0p3a5l
```

### url池
```
60% 概率：/?{9位数字}.html
40% 概率：/?{日期}/{5位数字}.html
示例：/?123456789.html 或 /?20260203/12345.html
```

### 关键词表情池
```
1. 从原始关键词中取一个
2. 50% 概率插入 1 个 emoji，50% 概率插入 2 个 emoji
3. 在关键词的随机位置插入
4. HTML 实体编码输出
示例：关😀键词 或 关😀键🎉词
```

### 标题池
```
格式：关键词1 + emoji1 + 关键词2 + emoji2 + 关键词3
从关键词池随机取 3 个，从 emoji 库随机取 2 个不重复的
```

## 数据库设计

**pool_config 表新增字段：**

```sql
-- 标题池（已有字段，保持）
title_pool_size          INT DEFAULT 800000
title_workers            INT DEFAULT 20
title_refill_interval_ms INT DEFAULT 30
title_threshold          DECIMAL(3,2) DEFAULT 0.40

-- cls类名池（新增）
cls_pool_size            INT DEFAULT 800000
cls_workers              INT DEFAULT 20
cls_refill_interval_ms   INT DEFAULT 30
cls_threshold            DECIMAL(3,2) DEFAULT 0.40

-- url池（新增）
url_pool_size            INT DEFAULT 500000
url_workers              INT DEFAULT 16
url_refill_interval_ms   INT DEFAULT 30
url_threshold            DECIMAL(3,2) DEFAULT 0.40

-- 关键词表情池（新增）
keyword_emoji_pool_size          INT DEFAULT 800000
keyword_emoji_workers            INT DEFAULT 20
keyword_emoji_refill_interval_ms INT DEFAULT 30
keyword_emoji_threshold          DECIMAL(3,2) DEFAULT 0.40
```

## 后端改动

### 1. pool_config.go - 扩展配置结构

```go
type CachePoolConfig struct {
    // 正文池（保持现有）
    ContentsSize     int `db:"contents_size" json:"contents_size"`
    Threshold        int `db:"threshold" json:"threshold"`
    RefillIntervalMs int `db:"refill_interval_ms" json:"refill_interval_ms"`

    // 标题池
    TitlePoolSize         int     `db:"title_pool_size" json:"title_pool_size"`
    TitleWorkers          int     `db:"title_workers" json:"title_workers"`
    TitleRefillIntervalMs int     `db:"title_refill_interval_ms" json:"title_refill_interval_ms"`
    TitleThreshold        float64 `db:"title_threshold" json:"title_threshold"`

    // cls类名池
    ClsPoolSize         int     `db:"cls_pool_size" json:"cls_pool_size"`
    ClsWorkers          int     `db:"cls_workers" json:"cls_workers"`
    ClsRefillIntervalMs int     `db:"cls_refill_interval_ms" json:"cls_refill_interval_ms"`
    ClsThreshold        float64 `db:"cls_threshold" json:"cls_threshold"`

    // url池
    UrlPoolSize         int     `db:"url_pool_size" json:"url_pool_size"`
    UrlWorkers          int     `db:"url_workers" json:"url_workers"`
    UrlRefillIntervalMs int     `db:"url_refill_interval_ms" json:"url_refill_interval_ms"`
    UrlThreshold        float64 `db:"url_threshold" json:"url_threshold"`

    // 关键词表情池
    KeywordEmojiPoolSize         int     `db:"keyword_emoji_pool_size" json:"keyword_emoji_pool_size"`
    KeywordEmojiWorkers          int     `db:"keyword_emoji_workers" json:"keyword_emoji_workers"`
    KeywordEmojiRefillIntervalMs int     `db:"keyword_emoji_refill_interval_ms" json:"keyword_emoji_refill_interval_ms"`
    KeywordEmojiThreshold        float64 `db:"keyword_emoji_threshold" json:"keyword_emoji_threshold"`
}
```

### 2. object_pool.go - 修改补充逻辑

将 `LowWatermark` 改为 `Threshold`，补充逻辑改为"补充到满"：

```go
type PoolConfig struct {
    Name          string
    Size          int
    Threshold     float64       // 低于此比例触发补充
    NumWorkers    int
    CheckInterval time.Duration
}

// checkAndRefill 检查并补充到满
func (p *ObjectPool[T]) checkAndRefill() {
    if p.paused.Load() {
        return
    }

    available := p.Available()
    threshold := int64(float64(p.size) * p.threshold)

    if available < threshold {
        // 补充到满
        need := p.size - available
        p.refillToFull(need)
    }
}
```

### 3. template_funcs.go - 从配置初始化池

```go
func (m *TemplateFuncsManager) InitPools(config *CachePoolConfig) {
    m.clsPool = NewObjectPool[string](PoolConfig{
        Name:          "cls",
        Size:          config.ClsPoolSize,
        Threshold:     config.ClsThreshold,
        NumWorkers:    config.ClsWorkers,
        CheckInterval: time.Duration(config.ClsRefillIntervalMs) * time.Millisecond,
    }, generateRandomCls)

    m.urlPool = NewObjectPool[string](PoolConfig{
        Name:          "url",
        Size:          config.UrlPoolSize,
        Threshold:     config.UrlThreshold,
        NumWorkers:    config.UrlWorkers,
        CheckInterval: time.Duration(config.UrlRefillIntervalMs) * time.Millisecond,
    }, generateRandomURL)

    m.keywordEmojiPool = NewObjectPool[string](PoolConfig{
        Name:          "keyword_emoji",
        Size:          config.KeywordEmojiPoolSize,
        Threshold:     config.KeywordEmojiThreshold,
        NumWorkers:    config.KeywordEmojiWorkers,
        CheckInterval: time.Duration(config.KeywordEmojiRefillIntervalMs) * time.Millisecond,
    }, m.generateKeywordWithEmoji)
}
```

### 4. title_generator.go - 使用统一配置

TitleGenerator 也使用相同的配置结构，从 CachePoolConfig 读取标题池配置。

## 前端改动

### 1. cache-pool.ts - 更新类型定义

```typescript
interface PoolItemConfig {
  pool_size: number
  workers: number
  refill_interval_ms: number
  threshold: number  // 0-1 的小数
}

interface CachePoolConfig {
  // 正文池（保持现有）
  contents_size: number
  threshold: number
  refill_interval_ms: number

  // 对象池配置
  title_pool_size: number
  title_workers: number
  title_refill_interval_ms: number
  title_threshold: number

  cls_pool_size: number
  cls_workers: number
  cls_refill_interval_ms: number
  cls_threshold: number

  url_pool_size: number
  url_workers: number
  url_refill_interval_ms: number
  url_threshold: number

  keyword_emoji_pool_size: number
  keyword_emoji_workers: number
  keyword_emoji_refill_interval_ms: number
  keyword_emoji_threshold: number
}
```

### 2. CacheManage.vue - 数据池配置 tab

**布局：**

```
┌─────────────────────────────────────────────────────┐
│ 数据池配置                                           │
├──────────────────────┬──────────────────────────────┤
│  ┌────────────────┐  │  ┌────────────────────────┐ │
│  │ 正文池         │  │  │ 对象池配置             │ │
│  │                │  │  │                        │ │
│  │ 正文池大小 [ ] │  │  │ 选择池 [▼ 标题池    ] │ │
│  │ 补充阈值   [ ] │  │  │                        │ │
│  │ 检查间隔   [ ] │  │  │ 池大小     [        ] │ │
│  └────────────────┘  │  │ 生成协程数 [        ] │ │
│                      │  │ 生成间隔   [        ] │ │
│                      │  │ 补充阈值   [        ] │ │
│                      │  └────────────────────────┘ │
├──────────────────────┴──────────────────────────────┤
│                    [保存配置]                        │
└─────────────────────────────────────────────────────┘
```

**下拉菜单选项：**
- 标题池
- cls类名池
- url池
- 关键词表情池

**交互逻辑：**
- 切换下拉菜单时，显示对应池的当前配置值
- 修改后点击保存，更新所选池的配置

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/internal/service/pool_config.go` | 修改 | 扩展配置结构，添加 12 个新字段 |
| `api/internal/service/object_pool.go` | 修改 | 补充逻辑改为"补充到满" |
| `api/internal/service/template_funcs.go` | 修改 | InitPools 从配置读取参数 |
| `api/internal/service/title_generator.go` | 修改 | 使用统一配置结构 |
| `api/internal/handler/pool.go` | 修改 | API 支持新字段 |
| `web/src/api/cache-pool.ts` | 修改 | 更新类型定义 |
| `web/src/views/cache/CacheManage.vue` | 修改 | 下拉选择池 + 统一配置表单 |
| `migrations/002_unified_pool_config.sql` | 新增 | 数据库迁移 |

## 默认配置值

| 池 | 池大小 | 协程数 | 间隔(ms) | 阈值 |
|---|---|---|---|---|
| 标题池 | 800,000 | 20 | 30 | 0.4 |
| cls类名池 | 800,000 | 20 | 30 | 0.4 |
| url池 | 500,000 | 16 | 30 | 0.4 |
| 关键词表情池 | 800,000 | 20 | 30 | 0.4 |
