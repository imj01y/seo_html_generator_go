# PoolManager 整合实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 DataManager 的 keywords/images 功能整合到 PoolManager 中，统一数据库缓存管理。

**Architecture:** PoolManager 管理 titles、contents（FIFO 消费）和 keywords、images（随机复用）。EmojiManager 和 HTMLEntityEncoder 作为辅助组件被 PoolManager 使用。

**Tech Stack:** Go, sqlx, zerolog

---

## Task 1: 扩展 CachePoolConfig 配置结构

**Files:**
- Modify: `api/internal/service/pool_config.go`

**Step 1: 添加 keywords/images 配置字段**

在 `CachePoolConfig` 结构体中添加新字段：

```go
type CachePoolConfig struct {
    ID               int       `db:"id" json:"id"`
    TitlesSize       int       `db:"titles_size" json:"titles_size"`
    ContentsSize     int       `db:"contents_size" json:"contents_size"`
    Threshold        int       `db:"threshold" json:"threshold"`
    RefillIntervalMs int       `db:"refill_interval_ms" json:"refill_interval_ms"`
    // 新增字段
    KeywordsSize      int `db:"keywords_size" json:"keywords_size"`
    ImagesSize        int `db:"images_size" json:"images_size"`
    RefreshIntervalMs int `db:"refresh_interval_ms" json:"refresh_interval_ms"`
    UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}
```

**Step 2: 更新 DefaultCachePoolConfig**

```go
func DefaultCachePoolConfig() *CachePoolConfig {
    return &CachePoolConfig{
        ID:                1,
        TitlesSize:        5000,
        ContentsSize:      5000,
        Threshold:         1000,
        RefillIntervalMs:  1000,
        KeywordsSize:      50000,
        ImagesSize:        50000,
        RefreshIntervalMs: 300000, // 5 minutes
    }
}
```

**Step 3: 添加 RefreshInterval 方法**

```go
func (c *CachePoolConfig) RefreshInterval() time.Duration {
    return time.Duration(c.RefreshIntervalMs) * time.Millisecond
}
```

**Step 4: 更新 SaveCachePoolConfig**

更新 INSERT/UPDATE 语句包含新字段。

**Step 5: Commit**

```bash
git add api/internal/service/pool_config.go
git commit -m "feat(pool): extend CachePoolConfig with keywords/images settings

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: 添加数据库迁移

**Files:**
- Modify: `migrations/005_pool_config.sql`

**Step 1: 添加新字段的 ALTER 语句**

```sql
-- 添加 keywords/images 配置字段
ALTER TABLE pool_config
ADD COLUMN IF NOT EXISTS keywords_size INT NOT NULL DEFAULT 50000 COMMENT '关键词池大小',
ADD COLUMN IF NOT EXISTS images_size INT NOT NULL DEFAULT 50000 COMMENT '图片池大小',
ADD COLUMN IF NOT EXISTS refresh_interval_ms INT NOT NULL DEFAULT 300000 COMMENT '刷新间隔(毫秒)';
```

**Step 2: Commit**

```bash
git add migrations/005_pool_config.sql
git commit -m "feat(db): add keywords/images config columns to pool_config

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: 扩展 PoolManager 结构体

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 添加 keywords/images 字段和辅助组件**

在 `PoolManager` 结构体中添加：

```go
type PoolManager struct {
    // 消费型池（FIFO，消费后标记）
    titles   map[int]*MemoryPool
    contents map[int]*MemoryPool

    // 复用型数据（随机获取，可重复）
    keywords    map[int][]string // 已编码版本
    rawKeywords map[int][]string // 未编码版本
    images      map[int][]string
    keywordsMu  sync.RWMutex
    imagesMu    sync.RWMutex

    // 辅助组件
    encoder      *HTMLEntityEncoder
    emojiManager *EmojiManager

    // 原有字段保持不变...
    config   *CachePoolConfig
    db       *sqlx.DB
    mu       sync.RWMutex
    ctx      context.Context
    cancel   context.CancelFunc
    updateCh chan UpdateTask
    wg       sync.WaitGroup
    stopped  atomic.Bool
}
```

**Step 2: 更新 NewPoolManager**

```go
func NewPoolManager(db *sqlx.DB) *PoolManager {
    ctx, cancel := context.WithCancel(context.Background())
    return &PoolManager{
        titles:       make(map[int]*MemoryPool),
        contents:     make(map[int]*MemoryPool),
        keywords:     make(map[int][]string),
        rawKeywords:  make(map[int][]string),
        images:       make(map[int][]string),
        encoder:      GetEncoder(),
        emojiManager: NewEmojiManager(),
        config:       DefaultCachePoolConfig(),
        db:           db,
        ctx:          ctx,
        cancel:       cancel,
        updateCh:     make(chan UpdateTask, 1000),
    }
}
```

**Step 3: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): add keywords/images fields to PoolManager

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: 添加 keywords 加载和获取方法

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 添加 LoadKeywords 方法**

```go
// LoadKeywords loads keywords for a group from the database
func (m *PoolManager) LoadKeywords(ctx context.Context, groupID int) (int, error) {
    query := `SELECT keyword FROM keywords WHERE group_id = ? AND status = 1 ORDER BY RAND() LIMIT ?`

    var keywords []string
    if err := m.db.SelectContext(ctx, &keywords, query, groupID, m.config.KeywordsSize); err != nil {
        return 0, err
    }

    // Store raw keywords
    rawCopy := make([]string, len(keywords))
    copy(rawCopy, keywords)

    // Pre-encode keywords
    encoded := make([]string, len(keywords))
    for i, kw := range keywords {
        encoded[i] = m.encoder.EncodeText(kw)
    }

    m.keywordsMu.Lock()
    m.keywords[groupID] = encoded
    m.rawKeywords[groupID] = rawCopy
    m.keywordsMu.Unlock()

    log.Info().Int("group_id", groupID).Int("count", len(encoded)).Msg("Keywords loaded")
    return len(encoded), nil
}
```

**Step 2: 添加 GetRandomKeywords 方法**

```go
// GetRandomKeywords returns random pre-encoded keywords
func (m *PoolManager) GetRandomKeywords(groupID int, count int) []string {
    m.keywordsMu.RLock()
    items := m.keywords[groupID]
    if len(items) == 0 {
        items = m.keywords[1] // fallback to default group
    }
    m.keywordsMu.RUnlock()

    return getRandomItems(items, count)
}

// GetRawKeywords returns raw (not encoded) keywords
func (m *PoolManager) GetRawKeywords(groupID int, count int) []string {
    m.keywordsMu.RLock()
    items := m.rawKeywords[groupID]
    if len(items) == 0 {
        items = m.rawKeywords[1]
    }
    m.keywordsMu.RUnlock()

    return getRandomItems(items, count)
}
```

**Step 3: 添加 getRandomItems 辅助函数**

```go
// getRandomItems 从切片中随机选取指定数量的元素
func getRandomItems(items []string, count int) []string {
    n := len(items)
    if n == 0 || count == 0 {
        return nil
    }
    if count > n {
        count = n
    }

    swapped := make(map[int]int, count)
    result := make([]string, count)

    for i := 0; i < count; i++ {
        j := i + rand.IntN(n-i)
        vi, oki := swapped[i]
        if !oki {
            vi = i
        }
        vj, okj := swapped[j]
        if !okj {
            vj = j
        }
        swapped[i] = vj
        swapped[j] = vi
        result[i] = items[vj]
    }
    return result
}
```

**Step 4: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): add keywords loading and retrieval methods

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: 添加 images 加载和获取方法

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 添加 LoadImages 方法**

```go
// LoadImages loads image URLs for a group from the database
func (m *PoolManager) LoadImages(ctx context.Context, groupID int) (int, error) {
    query := `SELECT url FROM images WHERE group_id = ? AND status = 1 ORDER BY RAND() LIMIT ?`

    var urls []string
    if err := m.db.SelectContext(ctx, &urls, query, groupID, m.config.ImagesSize); err != nil {
        return 0, err
    }

    m.imagesMu.Lock()
    m.images[groupID] = urls
    m.imagesMu.Unlock()

    log.Info().Int("group_id", groupID).Int("count", len(urls)).Msg("Images loaded")
    return len(urls), nil
}
```

**Step 2: 添加 GetRandomImage 和 GetImages 方法**

```go
// GetRandomImage returns a random image URL
func (m *PoolManager) GetRandomImage(groupID int) string {
    m.imagesMu.RLock()
    items := m.images[groupID]
    if len(items) == 0 {
        items = m.images[1]
    }
    m.imagesMu.RUnlock()

    if len(items) == 0 {
        return ""
    }
    return items[rand.IntN(len(items))]
}

// GetImages returns all image URLs for a group
func (m *PoolManager) GetImages(groupID int) []string {
    m.imagesMu.RLock()
    urls := m.images[groupID]
    m.imagesMu.RUnlock()

    if len(urls) == 0 {
        return nil
    }

    result := make([]string, len(urls))
    copy(result, urls)
    return result
}
```

**Step 3: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): add images loading and retrieval methods

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: 添加 Emoji 相关方法

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 添加 LoadEmojis 方法**

```go
// LoadEmojis loads emojis from a JSON file
func (m *PoolManager) LoadEmojis(path string) error {
    return m.emojiManager.LoadFromFile(path)
}
```

**Step 2: 添加 GetRandomEmoji 方法**

```go
// GetRandomEmoji returns a random emoji
func (m *PoolManager) GetRandomEmoji() string {
    return m.emojiManager.GetRandom()
}

// GetRandomEmojiExclude returns a random emoji not in the exclude set
func (m *PoolManager) GetRandomEmojiExclude(exclude map[string]bool) string {
    return m.emojiManager.GetRandomExclude(exclude)
}

// GetEmojiCount returns the number of loaded emojis
func (m *PoolManager) GetEmojiCount() int {
    return m.emojiManager.Count()
}
```

**Step 3: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): add emoji methods to PoolManager

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: 添加 keywords/images 刷新循环

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 修改 Start 方法启动刷新循环**

在 `Start` 方法中添加：

```go
// 发现 keywords 和 images 分组
keywordGroupIDs, _ := m.discoverKeywordGroups(ctx)
imageGroupIDs, _ := m.discoverImageGroups(ctx)

// 初始加载 keywords 和 images
for _, gid := range keywordGroupIDs {
    m.LoadKeywords(ctx, gid)
}
for _, gid := range imageGroupIDs {
    m.LoadImages(ctx, gid)
}

// 启动刷新循环
m.wg.Add(1)
go m.refreshLoop(keywordGroupIDs, imageGroupIDs)
```

**Step 2: 添加 discoverKeywordGroups 和 discoverImageGroups**

```go
func (m *PoolManager) discoverKeywordGroups(ctx context.Context) ([]int, error) {
    query := `SELECT DISTINCT id FROM keyword_groups`
    var ids []int
    if err := m.db.SelectContext(ctx, &ids, query); err != nil {
        return []int{1}, nil
    }
    if len(ids) == 0 {
        return []int{1}, nil
    }
    return ids, nil
}

func (m *PoolManager) discoverImageGroups(ctx context.Context) ([]int, error) {
    query := `SELECT DISTINCT id FROM image_groups`
    var ids []int
    if err := m.db.SelectContext(ctx, &ids, query); err != nil {
        return []int{1}, nil
    }
    if len(ids) == 0 {
        return []int{1}, nil
    }
    return ids, nil
}
```

**Step 3: 添加 refreshLoop**

```go
func (m *PoolManager) refreshLoop(keywordGroupIDs, imageGroupIDs []int) {
    defer m.wg.Done()

    ticker := time.NewTicker(m.config.RefreshInterval())
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            ctx := context.Background()
            for _, gid := range keywordGroupIDs {
                if _, err := m.LoadKeywords(ctx, gid); err != nil {
                    log.Warn().Err(err).Int("group", gid).Msg("Failed to refresh keywords")
                }
            }
            for _, gid := range imageGroupIDs {
                if _, err := m.LoadImages(ctx, gid); err != nil {
                    log.Warn().Err(err).Int("group", gid).Msg("Failed to refresh images")
                }
            }
            log.Debug().Msg("Keywords and images refreshed")
        case <-m.ctx.Done():
            return
        }
    }
}
```

**Step 4: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): add keywords/images refresh loop

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: 更新 GetStats 方法

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 扩展 GetStats 返回 keywords/images 统计**

```go
func (m *PoolManager) GetStats() map[string]interface{} {
    m.mu.RLock()
    titlesStats := make(map[int]map[string]int)
    contentsStats := make(map[int]map[string]int)
    for gid, pool := range m.titles {
        titlesStats[gid] = map[string]int{
            "current":   pool.Len(),
            "max_size":  pool.GetMaxSize(),
            "threshold": m.config.Threshold,
        }
    }
    for gid, pool := range m.contents {
        contentsStats[gid] = map[string]int{
            "current":   pool.Len(),
            "max_size":  pool.GetMaxSize(),
            "threshold": m.config.Threshold,
        }
    }
    m.mu.RUnlock()

    m.keywordsMu.RLock()
    keywordsStats := make(map[int]int)
    for gid, items := range m.keywords {
        keywordsStats[gid] = len(items)
    }
    m.keywordsMu.RUnlock()

    m.imagesMu.RLock()
    imagesStats := make(map[int]int)
    for gid, items := range m.images {
        imagesStats[gid] = len(items)
    }
    m.imagesMu.RUnlock()

    return map[string]interface{}{
        "titles":   titlesStats,
        "contents": contentsStats,
        "keywords": keywordsStats,
        "images":   imagesStats,
        "emojis":   m.emojiManager.Count(),
        "config": map[string]interface{}{
            "titles_size":         m.config.TitlesSize,
            "contents_size":       m.config.ContentsSize,
            "threshold":           m.config.Threshold,
            "refill_interval_ms":  m.config.RefillIntervalMs,
            "keywords_size":       m.config.KeywordsSize,
            "images_size":         m.config.ImagesSize,
            "refresh_interval_ms": m.config.RefreshIntervalMs,
        },
    }
}
```

**Step 2: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): extend GetStats with keywords/images/emojis

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: 更新 page.go 使用 PoolManager

**Files:**
- Modify: `api/internal/handler/page.go`

**Step 1: 移除 dataManager 依赖，只使用 poolManager**

更新 `PageHandler` 结构体，移除 `dataManager` 字段：

```go
type PageHandler struct {
    db               *sqlx.DB
    cfg              *config.Config
    spiderDetector   *core.SpiderDetector
    siteCache        *core.SiteCache
    templateCache    *core.TemplateCache
    htmlCache        *core.HTMLCache
    templateRenderer *core.TemplateRenderer
    funcsManager     *core.TemplateFuncsManager
    poolManager      *core.PoolManager
}
```

**Step 2: 更新 NewPageHandler**

移除 `dataManager` 参数。

**Step 3: 更新 ServePage 中的调用**

将 `h.dataManager.GetRandomKeywords()` 改为 `h.poolManager.GetRandomKeywords()`
将 `h.dataManager.GetRandomEmojiExclude()` 改为 `h.poolManager.GetRandomEmojiExclude()`
将 `h.dataManager.GetRandomContent()` 改为 fallback 逻辑（因为 content 已由 PoolManager.Pop 管理）

**Step 4: 更新 generateTitle 方法**

使用 `h.poolManager.GetRandomEmojiExclude()`

**Step 5: 更新 Stats 方法**

移除 `data_manager` 统计，使用 `pool_manager` 统计。

**Step 6: Commit**

```bash
git add api/internal/handler/page.go
git commit -m "refactor(page): use PoolManager instead of DataManager

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 10: 更新 main.go 移除 DataManager

**Files:**
- Modify: `api/cmd/main.go`

**Step 1: 移除 DataManager 创建和初始化**

删除：
- `dataManager := core.NewDataManager(...)`
- `dataManager.LoadKeywords(...)` 循环
- `dataManager.LoadImageURLs(...)` 循环
- `dataManager.StartAutoRefresh(...)`
- `dataManager.StopAutoRefresh()`
- `dataManager.LoadEmojis(...)`

**Step 2: 在 PoolManager 启动后加载 keywords/images/emojis**

PoolManager.Start() 已经会自动加载，只需加载 emojis：

```go
// Load emojis
emojisPath := filepath.Join(projectRoot, "data", "emojis.json")
if err := poolManager.LoadEmojis(emojisPath); err != nil {
    log.Warn().Err(err).Msg("Failed to load emojis")
}
```

**Step 3: 更新 pageHandler 创建，移除 dataManager 参数**

**Step 4: 更新 Dependencies 结构体，移除 DataManager**

**Step 5: Commit**

```bash
git add api/cmd/main.go
git commit -m "refactor(main): remove DataManager, use PoolManager for all data

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 11: 更新 router.go 移除 DataManager 依赖

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 更新 Dependencies 结构体**

移除 `DataManager` 字段。

**Step 2: 更新所有使用 DataManager 的地方**

改为使用 `PoolManager`。

**Step 3: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "refactor(router): remove DataManager dependency

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 12: 删除 data_manager.go

**Files:**
- Delete: `api/internal/service/data_manager.go`

**Step 1: 删除文件**

```bash
git rm api/internal/service/data_manager.go
```

**Step 2: Commit**

```bash
git commit -m "refactor(pool): remove DataManager, functionality merged into PoolManager

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 13: 更新前端 API 类型

**Files:**
- Modify: `web/src/api/cache-pool.ts`

**Step 1: 扩展 CachePoolConfig 接口**

```typescript
export interface CachePoolConfig {
  id?: number
  titles_size: number
  contents_size: number
  threshold: number
  refill_interval_ms: number
  // 新增字段
  keywords_size: number
  images_size: number
  refresh_interval_ms: number
  updated_at?: string
}
```

**Step 2: Commit**

```bash
git add web/src/api/cache-pool.ts
git commit -m "feat(web): extend CachePoolConfig with keywords/images fields

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 14: 更新前端设置页面

**Files:**
- Modify: `web/src/views/settings/Settings.vue`

**Step 1: 添加 keywords/images 配置表单项**

在缓存池配置 tab 中添加：

```vue
<el-form-item label="关键词池大小">
  <el-input-number
    v-model="cachePoolForm.keywords_size"
    :min="1000"
    :max="100000"
    :step="5000"
  />
  <span class="form-tip">条</span>
</el-form-item>
<el-form-item label="图片池大小">
  <el-input-number
    v-model="cachePoolForm.images_size"
    :min="1000"
    :max="100000"
    :step="5000"
  />
  <span class="form-tip">条</span>
</el-form-item>
<el-form-item label="刷新间隔">
  <el-input-number
    v-model="cachePoolForm.refresh_interval_ms"
    :min="60000"
    :max="3600000"
    :step="60000"
  />
  <span class="form-tip">毫秒（keywords/images 定时刷新）</span>
</el-form-item>
```

**Step 2: 更新 cachePoolForm reactive 对象**

添加新字段的默认值。

**Step 3: 更新 loadCachePoolConfig 方法**

加载新字段。

**Step 4: Commit**

```bash
git add web/src/views/settings/Settings.vue
git commit -m "feat(web): add keywords/images config UI in settings

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 15: 验证和测试

**Step 1: 编译 Go 代码**

```bash
cd api && go build ./...
```

**Step 2: 运行服务验证**

启动服务，检查日志确认：
- PoolManager 正确加载 titles/contents/keywords/images
- 刷新循环正常运行
- /page 请求正常返回

**Step 3: 验证前端**

打开设置页面，确认新配置项显示正常。

**Step 4: 最终提交**

```bash
git add -A
git commit -m "feat(pool): complete PoolManager consolidation

- Merged DataManager functionality into PoolManager
- keywords/images now managed by PoolManager with periodic refresh
- titles/contents continue using FIFO consumption with status marking
- Removed DataManager

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## 文件改动总结

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/internal/service/pool_config.go` | 修改 | 添加 keywords/images 配置字段 |
| `api/internal/service/pool_manager.go` | 修改 | 添加 keywords/images/emoji 管理 |
| `api/internal/handler/page.go` | 修改 | 使用 PoolManager 替代 DataManager |
| `api/cmd/main.go` | 修改 | 移除 DataManager 初始化 |
| `api/internal/handler/router.go` | 修改 | 移除 DataManager 依赖 |
| `api/internal/service/data_manager.go` | 删除 | 功能已整合到 PoolManager |
| `migrations/005_pool_config.sql` | 修改 | 添加新配置字段 |
| `web/src/api/cache-pool.ts` | 修改 | 扩展配置类型 |
| `web/src/views/settings/Settings.vue` | 修改 | 添加配置 UI |
