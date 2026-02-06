# 关键词分组支持设计

## 背景

### 问题描述

当前关键词系统不支持分组功能，所有站点共用同一套关键词。需要实现类似图片分组的功能，让不同站点可以使用不同的关键词分组。

### 现状分析

| 组件 | 当前状态 | 目标状态 |
|------|----------|----------|
| `PoolManager.KeywordPool` | ✅ 支持分组 | 保持 |
| `TemplateFuncsManager` | ❌ 不支持分组 | 需要改造 |
| `RenderData` | ❌ 无 KeywordGroupID | 需要添加 |

### 额外问题

- `RandomKeywordEmoji()` 使用预生成池 `keywordEmojiPool`，每次启动时生成 10 万条
- 预生成池无法支持分组，需要改为实时生成

## 设计目标

1. 支持关键词分组：不同站点可使用不同的关键词分组
2. 支持热更新：添加/删除关键词后，模板渲染立即生效
3. 保持高性能：读取操作无锁，适合高并发渲染场景
4. 统一架构：与图片分组保持一致的实现模式

## 技术方案

### 架构决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 数据源 | 保持 TemplateFuncsManager 独立缓存 | 无锁读取，性能最优 |
| 并发控制 | 原子指针替换 | 避免读写锁开销 |
| emoji 生成 | 实时生成 | 支持分组，避免预生成池问题 |
| 分组传递 | 通过 RenderData | 数据流清晰，每次渲染独立 |

### 数据结构

```go
// 关键词数据（不可变，通过原子指针替换）
type KeywordData struct {
    groups    map[int][]string  // groupID -> encoded keywords
    rawGroups map[int][]string  // groupID -> raw keywords
}

type TemplateFuncsManager struct {
    // ... 其他字段保持不变

    // 关键词数据（原子指针，支持无锁读取）
    keywordData atomic.Pointer[KeywordData]

    // 分组索引（独立管理，避免数据替换时重置）
    keywordGroupIdx    sync.Map  // groupID -> *atomic.Int64
    rawKeywordGroupIdx sync.Map  // groupID -> *atomic.Int64
}
```

**设计要点：**

- `KeywordData` 是不可变的，更新时创建新实例并原子替换指针
- `groups` 存储编码后的关键词（用于 `RandomKeyword`）
- `rawGroups` 存储原始关键词（用于 `RandomKeywordEmoji`）
- 索引独立于数据存储，避免数据替换时索引重置

### 核心方法

#### TemplateFuncsManager

```go
// RandomKeyword 获取随机关键词（修改：增加 groupID 参数）
func (m *TemplateFuncsManager) RandomKeyword(groupID int) string {
    data := m.keywordData.Load()
    if data == nil {
        return ""
    }

    keywords := data.groups[groupID]
    if len(keywords) == 0 {
        return ""
    }

    // 获取或创建该分组的索引
    idxPtr, _ := m.keywordGroupIdx.LoadOrStore(groupID, &atomic.Int64{})
    idx := idxPtr.(*atomic.Int64).Add(1) - 1

    return keywords[idx % int64(len(keywords))]
}

// RandomKeywordEmoji 获取带 emoji 的随机关键词（修改：实时生成）
func (m *TemplateFuncsManager) RandomKeywordEmoji(groupID int) string {
    data := m.keywordData.Load()
    if data == nil {
        return ""
    }

    rawKeywords := data.rawGroups[groupID]
    if len(rawKeywords) == 0 {
        return ""
    }

    // 获取或创建该分组的索引
    idxPtr, _ := m.rawKeywordGroupIdx.LoadOrStore(groupID, &atomic.Int64{})
    idx := idxPtr.(*atomic.Int64).Add(1) - 1

    keyword := rawKeywords[idx % int64(len(rawKeywords))]
    return addRandomEmoji(keyword)  // 实时添加 emoji
}

// LoadKeywordGroup 加载指定分组（初始化时使用）
func (m *TemplateFuncsManager) LoadKeywordGroup(groupID int, keywords, rawKeywords []string)

// AppendKeywords 追加关键词到指定分组（添加关键词时使用）
func (m *TemplateFuncsManager) AppendKeywords(groupID int, keywords, rawKeywords []string)

// ReloadKeywordGroup 重载指定分组（删除后异步调用）
func (m *TemplateFuncsManager) ReloadKeywordGroup(groupID int, keywords, rawKeywords []string)
```

#### PoolManager

```go
// GetKeywordGroupIDs 返回所有关键词分组ID（新增）
func (m *PoolManager) GetKeywordGroupIDs() []int

// GetKeywords 获取指定分组的所有编码关键词（新增）
func (m *PoolManager) GetKeywords(groupID int) []string

// GetAllRawKeywords 获取指定分组的所有原始关键词（新增）
func (m *PoolManager) GetAllRawKeywords(groupID int) []string
```

### 渲染时传递 groupID

#### RenderData 修改

```go
type RenderData struct {
    Title          string
    TitleGenerator func() string
    SiteID         int
    ImageGroupID   int
    KeywordGroupID int   // 新增
    AnalyticsCode  template.HTML
    BaiduPushJS    template.HTML
    ArticleContent template.HTML
}
```

#### fast_renderer.go 修改

```go
case PlaceholderKeyword:
    if data != nil {
        return fm.RandomKeyword(data.KeywordGroupID)
    }
    return fm.RandomKeyword(1)  // 默认分组
case PlaceholderKeywordEmoji:
    if data != nil {
        return fm.RandomKeywordEmoji(data.KeywordGroupID)
    }
    return fm.RandomKeywordEmoji(1)  // 默认分组
```

#### page.go 修改

```go
// 已有读取逻辑（line 126-128），只需添加到 RenderData
keywordGroupID := 1
if site.KeywordGroupID.Valid {
    keywordGroupID = int(site.KeywordGroupID.Int64)
}

renderData := &core.RenderData{
    // ... 其他字段
    KeywordGroupID: keywordGroupID,  // 新增
}
```

### 触发机制

#### KeywordsHandler 修改

```go
// 构造函数增加 funcsManager 参数
func NewKeywordsHandler(
    db *sqlx.DB,
    poolManager *core.PoolManager,
    funcsManager *core.TemplateFuncsManager,  // 新增
) *KeywordsHandler

// 封装异步重载方法
func (h *KeywordsHandler) asyncReloadKeywordGroup(groupID int) {
    go func() {
        ctx := context.Background()
        // 1. 等待 PoolManager 重载完成
        h.poolManager.ReloadKeywordGroup(ctx, groupID)
        // 2. 获取最新数据
        keywords := h.poolManager.GetKeywords(groupID)
        rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
        // 3. 同步到 TemplateFuncsManager
        h.funcsManager.ReloadKeywordGroup(groupID, keywords, rawKeywords)
    }()
}
```

#### 各操作的同步方式

| 操作 | 方法 | 同步调用 |
|------|------|----------|
| AddKeyword | 添加单个 | `funcsManager.AppendKeywords(groupID, ...)` |
| BatchAddKeywords | 批量添加 | `funcsManager.AppendKeywords(groupID, ...)` |
| DeleteKeyword | 删除单个 | `asyncReloadKeywordGroup(groupID)` |
| DeleteGroup | 删除分组 | `asyncReloadKeywordGroup(groupID)` |
| BatchDelete | 批量删除 | `asyncReloadKeywordGroup(groupID)` |
| Reload | 手动重载 | `asyncReloadKeywordGroup(groupID)` |

### 启动时初始化

```go
// main.go 启动后
poolManager.Start(ctx)

// 初始化 TemplateFuncsManager 的关键词数据
groupIDs := poolManager.GetKeywordGroupIDs()
for _, groupID := range groupIDs {
    keywords := poolManager.GetKeywords(groupID)
    rawKeywords := poolManager.GetAllRawKeywords(groupID)
    if len(keywords) > 0 {
        funcsManager.LoadKeywordGroup(groupID, keywords, rawKeywords)
        log.Info().Int("group_id", groupID).Int("count", len(keywords)).
            Msg("Keyword group loaded to funcs manager")
    }
}
```

### 清理旧代码

#### 删除旧字段

```go
// 删除以下字段：
type TemplateFuncsManager struct {
    // ❌ 删除：
    keywords      []string
    keywordIdx    int64
    keywordLen    int64
    rawKeywords   []string
    rawKeywordIdx int64
    rawKeywordLen int64
    keywordEmojiPool []string
}
```

#### 删除旧方法

```go
// ❌ 删除：LoadKeywords（旧的单分组加载）
func (m *TemplateFuncsManager) LoadKeywords(keywords []string)

// ❌ 删除：emoji 池初始化相关代码
```

## 修改文件清单

| 文件 | 修改内容 |
|------|----------|
| `api/internal/service/template_funcs.go` | 重构关键词数据结构，新增/修改方法，删除旧代码 |
| `api/internal/service/fast_renderer.go` | RenderData 增加 `KeywordGroupID`，修改 `resolvePlaceholder` |
| `api/internal/service/pool_manager.go` | 新增 `GetKeywordGroupIDs()`、`GetKeywords()`、`GetAllRawKeywords()` |
| `api/internal/service/pool/keyword_pool.go` | 新增 `GetKeywords()`、`GetAllRawKeywords()` 方法 |
| `api/internal/handler/keywords.go` | 构造函数增加参数，各操作添加同步 |
| `api/internal/handler/router.go` | 传入 `deps.TemplateFuncs` |
| `api/internal/handler/page.go` | RenderData 添加 `KeywordGroupID` |
| `api/cmd/main.go` | 多分组初始化逻辑 |

## 性能分析

### 读取性能（渲染时）

| 操作 | 复杂度 | 耗时 |
|------|--------|------|
| 原子加载指针 | O(1) | ~10ns |
| map 查找 | O(1) | ~10ns |
| sync.Map 读取 | O(1) | ~20ns |
| 原子增加索引 | O(1) | ~10ns |
| emoji 实时生成 | O(1) | ~100ns |
| **总计** | O(1) | **~50-150ns** |

### 更新性能（管理操作）

| 操作 | 场景 | 耗时 |
|------|------|------|
| 追加关键词 | 添加少量 | < 1μs |
| 重载分组 | 100万条数据 | 10-30ms（异步，不阻塞） |

## 测试要点

1. **单元测试**
   - `RandomKeyword(groupID)` 正确返回对应分组的关键词
   - `RandomKeywordEmoji(groupID)` 正确返回带 emoji 的关键词
   - `AppendKeywords` 后立即可读取到新关键词
   - `ReloadKeywordGroup` 后数据正确更新
   - 并发读写不 panic

2. **集成测试**
   - 添加关键词后，页面渲染能获取到新关键词
   - 删除关键词后，页面渲染不再返回已删除关键词
   - 不同站点使用不同分组的关键词
