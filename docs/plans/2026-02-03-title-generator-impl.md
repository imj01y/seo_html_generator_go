# 标题动态生成实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将标题从数据库加载改为从关键词+emoji动态生成，支持后台配置池大小和协程数。

**Architecture:** TitleGenerator 组件从 PoolManager 的关键词缓存和 emoji 库中随机提取数据，拼接生成标题并存入 channel 池。渲染时从池中消费，池空时同步生成。

**Tech Stack:** Go 1.24, Gin, Channel, sync.RWMutex

---

## Task 1: 扩展配置结构

**Files:**
- Modify: `api/internal/service/pool_config.go:12-24`

**Step 1: 添加标题生成配置字段**

在 `CachePoolConfig` 结构体中添加新字段：

```go
// CachePoolConfig holds cache pool configuration for titles, contents, keywords, and images
type CachePoolConfig struct {
	ID               int       `db:"id" json:"id"`
	TitlesSize       int       `db:"titles_size" json:"titles_size"`
	ContentsSize     int       `db:"contents_size" json:"contents_size"`
	Threshold        int       `db:"threshold" json:"threshold"`
	RefillIntervalMs int       `db:"refill_interval_ms" json:"refill_interval_ms"`
	// keywords/images 配置
	KeywordsSize      int `db:"keywords_size" json:"keywords_size"`
	ImagesSize        int `db:"images_size" json:"images_size"`
	RefreshIntervalMs int `db:"refresh_interval_ms" json:"refresh_interval_ms"`
	// 标题生成配置（新增）
	TitlePoolSize         int `db:"title_pool_size" json:"title_pool_size"`
	TitleWorkers          int `db:"title_workers" json:"title_workers"`
	TitleRefillIntervalMs int `db:"title_refill_interval_ms" json:"title_refill_interval_ms"`
	TitleThreshold        int `db:"title_threshold" json:"title_threshold"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}
```

**Step 2: 更新默认配置**

修改 `DefaultCachePoolConfig` 函数：

```go
// DefaultCachePoolConfig returns default configuration
func DefaultCachePoolConfig() *CachePoolConfig {
	return &CachePoolConfig{
		ID:                    1,
		TitlesSize:            5000,
		ContentsSize:          5000,
		Threshold:             1000,
		RefillIntervalMs:      1000,
		KeywordsSize:          50000,
		ImagesSize:            50000,
		RefreshIntervalMs:     300000, // 5 minutes
		TitlePoolSize:         5000,
		TitleWorkers:          2,
		TitleRefillIntervalMs: 500,
		TitleThreshold:        1000,
	}
}
```

**Step 3: 更新保存函数**

修改 `SaveCachePoolConfig` 函数的 SQL：

```go
// SaveCachePoolConfig saves configuration to database
func SaveCachePoolConfig(ctx context.Context, db *sqlx.DB, config *CachePoolConfig) error {
	query := `
		INSERT INTO pool_config (id, titles_size, contents_size, threshold, refill_interval_ms,
			keywords_size, images_size, refresh_interval_ms,
			title_pool_size, title_workers, title_refill_interval_ms, title_threshold)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			titles_size = VALUES(titles_size),
			contents_size = VALUES(contents_size),
			threshold = VALUES(threshold),
			refill_interval_ms = VALUES(refill_interval_ms),
			keywords_size = VALUES(keywords_size),
			images_size = VALUES(images_size),
			refresh_interval_ms = VALUES(refresh_interval_ms),
			title_pool_size = VALUES(title_pool_size),
			title_workers = VALUES(title_workers),
			title_refill_interval_ms = VALUES(title_refill_interval_ms),
			title_threshold = VALUES(title_threshold)
	`
	_, err := db.ExecContext(ctx, query,
		config.TitlesSize,
		config.ContentsSize,
		config.Threshold,
		config.RefillIntervalMs,
		config.KeywordsSize,
		config.ImagesSize,
		config.RefreshIntervalMs,
		config.TitlePoolSize,
		config.TitleWorkers,
		config.TitleRefillIntervalMs,
		config.TitleThreshold,
	)
	return err
}
```

**Step 4: 添加辅助方法**

```go
// TitleRefillInterval returns the title refill interval as time.Duration
func (c *CachePoolConfig) TitleRefillInterval() time.Duration {
	return time.Duration(c.TitleRefillIntervalMs) * time.Millisecond
}
```

**Step 5: Commit**

```bash
git add api/internal/service/pool_config.go
git commit -m "feat(pool): 添加标题生成配置字段"
```

---

## Task 2: 创建 TitleGenerator

**Files:**
- Create: `api/internal/service/title_generator.go`

**Step 1: 创建文件并定义结构**

```go
// api/internal/service/title_generator.go
package core

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// TitlePool 标题池（基于 channel）
type TitlePool struct {
	ch      chan string
	groupID int
}

// TitleGenerator 动态标题生成器
type TitleGenerator struct {
	pools       map[int]*TitlePool // groupID -> 标题池
	poolManager *PoolManager       // 引用，获取关键词和emoji
	config      *CachePoolConfig
	mu          sync.RWMutex

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	stopped atomic.Bool
}

// NewTitleGenerator 创建标题生成器
func NewTitleGenerator(pm *PoolManager, config *CachePoolConfig) *TitleGenerator {
	ctx, cancel := context.WithCancel(context.Background())
	return &TitleGenerator{
		pools:       make(map[int]*TitlePool),
		poolManager: pm,
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
	}
}
```

**Step 2: 实现核心生成逻辑**

```go
// generateTitle 生成单个标题
// 格式：关键词1 + emoji1 + 关键词2 + emoji2 + 关键词3
func (g *TitleGenerator) generateTitle(groupID int) string {
	// 获取 3 个随机编码关键词
	keywords := g.poolManager.GetRandomKeywords(groupID, 3)
	if len(keywords) < 3 {
		// 关键词不足，返回空或部分拼接
		if len(keywords) == 0 {
			return ""
		}
		// 尽可能拼接
		result := keywords[0]
		if len(keywords) > 1 {
			result += g.poolManager.GetRandomEmoji() + keywords[1]
		}
		return result
	}

	// 获取 2 个不重复的 emoji
	emoji1 := g.poolManager.GetRandomEmoji()
	emoji2 := g.poolManager.GetRandomEmojiExclude(map[string]bool{emoji1: true})

	// 拼接：关键词1 + emoji1 + 关键词2 + emoji2 + 关键词3
	return keywords[0] + emoji1 + keywords[1] + emoji2 + keywords[2]
}
```

**Step 3: 实现池管理方法**

```go
// getOrCreatePool 获取或创建指定 groupID 的标题池
func (g *TitleGenerator) getOrCreatePool(groupID int) *TitlePool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if pool, exists := g.pools[groupID]; exists {
		return pool
	}

	pool := &TitlePool{
		ch:      make(chan string, g.config.TitlePoolSize),
		groupID: groupID,
	}
	g.pools[groupID] = pool
	log.Debug().Int("group_id", groupID).Int("size", g.config.TitlePoolSize).Msg("Created title pool")
	return pool
}

// Pop 从标题池获取一个标题
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

**Step 4: 实现后台填充逻辑**

```go
// fillPool 填充标题池
func (g *TitleGenerator) fillPool(groupID int, pool *TitlePool) {
	need := g.config.TitlePoolSize - len(pool.ch)
	if need <= 0 {
		return
	}

	filled := 0
	for i := 0; i < need; i++ {
		title := g.generateTitle(groupID)
		if title == "" {
			continue
		}
		select {
		case pool.ch <- title:
			filled++
		default:
			// 池满，停止
			break
		}
	}

	if filled > 0 {
		log.Debug().
			Int("group_id", groupID).
			Int("filled", filled).
			Int("total", len(pool.ch)).
			Msg("Title pool filled")
	}
}

// refillWorker 后台填充协程
func (g *TitleGenerator) refillWorker(groupID int, pool *TitlePool) {
	defer g.wg.Done()

	ticker := time.NewTicker(g.config.TitleRefillInterval())
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			if g.stopped.Load() {
				return
			}
			// 检查是否需要补充
			if len(pool.ch) < g.config.TitleThreshold {
				g.fillPool(groupID, pool)
			}
		}
	}
}
```

**Step 5: 实现启动和停止方法**

```go
// Start 启动标题生成器
func (g *TitleGenerator) Start(groupIDs []int) {
	log.Info().
		Ints("group_ids", groupIDs).
		Int("pool_size", g.config.TitlePoolSize).
		Int("workers", g.config.TitleWorkers).
		Msg("Starting TitleGenerator")

	for _, groupID := range groupIDs {
		pool := g.getOrCreatePool(groupID)

		// 初始填充
		g.fillPool(groupID, pool)

		// 启动 N 个填充协程
		for i := 0; i < g.config.TitleWorkers; i++ {
			g.wg.Add(1)
			go g.refillWorker(groupID, pool)
		}
	}
}

// Stop 停止标题生成器
func (g *TitleGenerator) Stop() {
	g.stopped.Store(true)
	g.cancel()
	g.wg.Wait()
	log.Info().Msg("TitleGenerator stopped")
}

// Reload 重载配置
func (g *TitleGenerator) Reload(config *CachePoolConfig) {
	g.mu.Lock()
	oldConfig := g.config
	g.config = config
	g.mu.Unlock()

	// 如果池大小变化，需要重建池
	if config.TitlePoolSize != oldConfig.TitlePoolSize {
		log.Info().
			Int("old_size", oldConfig.TitlePoolSize).
			Int("new_size", config.TitlePoolSize).
			Msg("Title pool size changed, pools will be recreated on next access")
		// 清空旧池，下次访问时会创建新大小的池
		g.mu.Lock()
		g.pools = make(map[int]*TitlePool)
		g.mu.Unlock()
	}
}
```

**Step 6: 实现统计方法**

```go
// GetStats 获取标题池统计
func (g *TitleGenerator) GetStats() map[int]map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := make(map[int]map[string]int)
	for groupID, pool := range g.pools {
		stats[groupID] = map[string]int{
			"current":   len(pool.ch),
			"max_size":  g.config.TitlePoolSize,
			"threshold": g.config.TitleThreshold,
		}
	}
	return stats
}

// GetTotalStats 获取汇总统计
func (g *TitleGenerator) GetTotalStats() (current, maxSize int) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, pool := range g.pools {
		current += len(pool.ch)
		maxSize += g.config.TitlePoolSize
	}
	return
}
```

**Step 7: Commit**

```bash
git add api/internal/service/title_generator.go
git commit -m "feat(pool): 创建 TitleGenerator 动态标题生成器"
```

---

## Task 3: 集成到 PoolManager

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 添加 TitleGenerator 字段**

在 `PoolManager` 结构体中添加：

```go
type PoolManager struct {
	// 消费型池（FIFO，消费后标记）
	titles   map[int]*MemoryPool // groupID -> pool （保留但不再使用）
	contents map[int]*MemoryPool // groupID -> pool

	// 标题生成器（新增）
	titleGenerator *TitleGenerator

	// ... 其他字段保持不变 ...
}
```

**Step 2: 修改 NewPoolManager**

```go
// NewPoolManager creates a new pool manager
func NewPoolManager(db *sqlx.DB) *PoolManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &PoolManager{
		titles:       make(map[int]*MemoryPool), // 保留用于 contents 复用
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
		// titleGenerator 在 Start 中初始化
	}
}
```

**Step 3: 修改 Start 方法**

在 `Start` 方法中，关键词和 emoji 加载完成后初始化 TitleGenerator：

```go
// Start starts the pool manager
func (m *PoolManager) Start(ctx context.Context) error {
	// Load config from DB
	config, err := LoadCachePoolConfig(ctx, m.db)
	if err != nil {
		return fmt.Errorf("failed to load pool config: %w", err)
	}
	m.config = config

	// Discover groups for contents only (titles now use TitleGenerator)
	groupIDs, err := m.discoverGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover groups: %w", err)
	}

	for _, gid := range groupIDs {
		// 只为 contents 创建池，titles 使用 TitleGenerator
		m.getOrCreatePool("contents", gid)
	}

	// Initial fill for contents only
	m.checkAndRefillAll()

	// Discover and load keywords/images
	keywordGroupIDs, _ := m.discoverKeywordGroups(ctx)
	imageGroupIDs, _ := m.discoverImageGroups(ctx)

	for _, gid := range keywordGroupIDs {
		if _, err := m.LoadKeywords(ctx, gid); err != nil {
			log.Warn().Err(err).Int("group", gid).Msg("Failed to load keywords")
		}
	}
	for _, gid := range imageGroupIDs {
		if _, err := m.LoadImages(ctx, gid); err != nil {
			log.Warn().Err(err).Int("group", gid).Msg("Failed to load images")
		}
	}

	// 初始化并启动 TitleGenerator（必须在关键词加载完成后）
	m.titleGenerator = NewTitleGenerator(m, m.config)
	m.titleGenerator.Start(keywordGroupIDs)

	// Set initial lastRefresh time
	m.mu.Lock()
	m.lastRefresh = time.Now()
	m.mu.Unlock()

	// Start background workers
	m.wg.Add(3)
	go m.refillLoop()
	go m.updateWorker()
	go m.refreshLoop(keywordGroupIDs, imageGroupIDs)

	log.Info().
		Int("content_groups", len(groupIDs)).
		Int("keyword_groups", len(keywordGroupIDs)).
		Int("image_groups", len(imageGroupIDs)).
		Int("title_pool_size", m.config.TitlePoolSize).
		Int("contents_size", m.config.ContentsSize).
		Msg("PoolManager started")

	return nil
}
```

**Step 4: 修改 Stop 方法**

```go
// Stop stops the pool manager gracefully
func (m *PoolManager) Stop() {
	m.stopped.Store(true)
	m.cancel()
	close(m.updateCh)
	if m.titleGenerator != nil {
		m.titleGenerator.Stop()
	}
	m.wg.Wait()
	log.Info().Msg("PoolManager stopped")
}
```

**Step 5: 修改 Pop 方法**

```go
// Pop retrieves an item from the pool
func (m *PoolManager) Pop(poolType string, groupID int) (string, error) {
	// titles 使用 TitleGenerator
	if poolType == "titles" {
		if m.titleGenerator == nil {
			return "", ErrCachePoolEmpty
		}
		return m.titleGenerator.Pop(groupID)
	}

	// contents 保持原有逻辑
	if err := validatePoolType(poolType); err != nil {
		return "", err
	}

	pool := m.getOrCreatePool(poolType, groupID)
	item, ok := pool.Pop()
	if !ok {
		// Try to refill and pop again
		m.refillPool(pool)
		item, ok = pool.Pop()
		if !ok {
			return "", ErrCachePoolEmpty
		}
	}

	// Async update status
	if !m.stopped.Load() {
		select {
		case m.updateCh <- UpdateTask{Table: poolType, ID: item.ID}:
		default:
			log.Warn().Str("table", poolType).Int64("id", item.ID).Msg("Update channel full")
		}
	}

	return item.Text, nil
}
```

**Step 6: 修改 Reload 方法**

```go
// Reload reloads configuration from database
func (m *PoolManager) Reload(ctx context.Context) error {
	config, err := LoadCachePoolConfig(ctx, m.db)
	if err != nil {
		return err
	}

	m.mu.Lock()
	oldConfig := m.config
	m.config = config

	// Resize contents pools if needed (titles now use TitleGenerator)
	if config.ContentsSize != oldConfig.ContentsSize {
		for _, pool := range m.contents {
			pool.Resize(config.ContentsSize)
		}
	}
	m.mu.Unlock()

	// Reload TitleGenerator config
	if m.titleGenerator != nil {
		m.titleGenerator.Reload(config)
	}

	log.Info().
		Int("title_pool_size", config.TitlePoolSize).
		Int("title_workers", config.TitleWorkers).
		Int("contents_size", config.ContentsSize).
		Int("threshold", config.Threshold).
		Msg("PoolManager config reloaded")

	return nil
}
```

**Step 7: 修改 GetDataPoolsStats 方法**

更新标题池统计部分：

```go
// GetDataPoolsStats 返回数据池运行状态统计
func (m *PoolManager) GetDataPoolsStats() []PoolStatusStats {
	// ... 前面代码保持不变 ...

	pools := []PoolStatusStats{}

	// 1. 标题池（改用 TitleGenerator 统计）
	var titlesCurrent, titlesMax int
	if m.titleGenerator != nil {
		titlesCurrent, titlesMax = m.titleGenerator.GetTotalStats()
	}
	titlesUsed := titlesMax - titlesCurrent
	titlesUtil := 0.0
	if titlesMax > 0 {
		titlesUtil = float64(titlesCurrent) / float64(titlesMax) * 100
	}
	pools = append(pools, PoolStatusStats{
		Name:        "标题池",
		Size:        titlesMax,
		Available:   titlesCurrent,
		Used:        titlesUsed,
		Utilization: titlesUtil,
		Status:      status,
		NumWorkers:  m.config.TitleWorkers,
		LastRefresh: lastRefreshPtr,
	})

	// 2. 正文池（保持不变）
	// ... 后续代码保持不变 ...
}
```

**Step 8: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): 集成 TitleGenerator 到 PoolManager"
```

---

## Task 4: 更新 memory_pool.go

**Files:**
- Modify: `api/internal/service/memory_pool.go:14-17`

**Step 1: 从 validTables 移除 titles**

```go
// validTables is a whitelist of allowed table names for SQL queries
var validTables = map[string]bool{
	"contents": true,
}
```

**Step 2: Commit**

```bash
git add api/internal/service/memory_pool.go
git commit -m "refactor(pool): 从 validTables 移除 titles"
```

---

## Task 5: 更新 API Handler

**Files:**
- Modify: `api/internal/handler/pool.go`

**Step 1: 更新 UpdateConfig 请求结构**

```go
// UpdateConfig updates pool configuration
func (h *PoolHandler) UpdateConfig(c *gin.Context) {
	var req struct {
		TitlesSize            int `json:"titles_size"`
		ContentsSize          int `json:"contents_size"`
		Threshold             int `json:"threshold"`
		RefillIntervalMs      int `json:"refill_interval_ms"`
		KeywordsSize          int `json:"keywords_size"`
		ImagesSize            int `json:"images_size"`
		RefreshIntervalMs     int `json:"refresh_interval_ms"`
		TitlePoolSize         int `json:"title_pool_size"`
		TitleWorkers          int `json:"title_workers"`
		TitleRefillIntervalMs int `json:"title_refill_interval_ms"`
		TitleThreshold        int `json:"title_threshold"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate contents
	if req.ContentsSize < 100 || req.ContentsSize > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contents_size must be between 100 and 100000"})
		return
	}
	if req.Threshold < 10 || req.Threshold > req.ContentsSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "threshold must be between 10 and contents_size"})
		return
	}
	if req.RefillIntervalMs < 100 || req.RefillIntervalMs > 60000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refill_interval_ms must be between 100 and 60000"})
		return
	}

	// Validate title config
	if req.TitlePoolSize < 100 || req.TitlePoolSize > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_pool_size must be between 100 and 100000"})
		return
	}
	if req.TitleWorkers < 1 || req.TitleWorkers > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_workers must be between 1 and 10"})
		return
	}
	if req.TitleRefillIntervalMs < 100 || req.TitleRefillIntervalMs > 60000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_refill_interval_ms must be between 100 and 60000"})
		return
	}
	if req.TitleThreshold < 10 || req.TitleThreshold > req.TitlePoolSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title_threshold must be between 10 and title_pool_size"})
		return
	}

	config := &core.CachePoolConfig{
		TitlesSize:            req.TitlesSize,
		ContentsSize:          req.ContentsSize,
		Threshold:             req.Threshold,
		RefillIntervalMs:      req.RefillIntervalMs,
		KeywordsSize:          req.KeywordsSize,
		ImagesSize:            req.ImagesSize,
		RefreshIntervalMs:     req.RefreshIntervalMs,
		TitlePoolSize:         req.TitlePoolSize,
		TitleWorkers:          req.TitleWorkers,
		TitleRefillIntervalMs: req.TitleRefillIntervalMs,
		TitleThreshold:        req.TitleThreshold,
	}

	// Save to DB
	if err := core.SaveCachePoolConfig(c.Request.Context(), h.db, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reload pool manager
	if err := h.poolManager.Reload(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Config saved but reload failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  config,
	})
}
```

**Step 2: Commit**

```bash
git add api/internal/handler/pool.go
git commit -m "feat(api): 更新缓存池配置 API 支持标题生成参数"
```

---

## Task 6: 更新前端类型定义

**Files:**
- Modify: `web/src/api/cache-pool.ts`

**Step 1: 添加新字段到类型定义**

```typescript
/** 缓存池配置 */
export interface CachePoolConfig {
  id?: number
  titles_size: number
  contents_size: number
  threshold: number
  refill_interval_ms: number
  keywords_size: number
  images_size: number
  refresh_interval_ms: number
  // 标题生成配置（新增）
  title_pool_size: number
  title_workers: number
  title_refill_interval_ms: number
  title_threshold: number
  updated_at?: string
}
```

**Step 2: Commit**

```bash
git add web/src/api/cache-pool.ts
git commit -m "feat(web): 添加标题生成配置类型定义"
```

---

## Task 7: 更新前端配置页面

**Files:**
- Modify: `web/src/views/cache/CacheManage.vue`

**Step 1: 更新 cachePoolForm 响应式对象**

在 `<script setup>` 中找到 `cachePoolForm` 并添加新字段：

```typescript
const cachePoolForm = reactive<CachePoolConfig>({
  titles_size: 5000,
  contents_size: 5000,
  threshold: 1000,
  refill_interval_ms: 1000,
  keywords_size: 50000,
  images_size: 50000,
  refresh_interval_ms: 300000,
  // 标题生成配置（新增）
  title_pool_size: 5000,
  title_workers: 2,
  title_refill_interval_ms: 500,
  title_threshold: 1000
})
```

**Step 2: 更新 loadCachePoolConfig 函数**

```typescript
const loadCachePoolConfig = async () => {
  cachePoolLoading.value = true
  try {
    const config = await getCachePoolConfig()
    cachePoolForm.titles_size = config.titles_size
    cachePoolForm.contents_size = config.contents_size
    cachePoolForm.threshold = config.threshold
    cachePoolForm.refill_interval_ms = config.refill_interval_ms
    cachePoolForm.keywords_size = config.keywords_size
    cachePoolForm.images_size = config.images_size
    cachePoolForm.refresh_interval_ms = config.refresh_interval_ms
    // 标题生成配置
    cachePoolForm.title_pool_size = config.title_pool_size || 5000
    cachePoolForm.title_workers = config.title_workers || 2
    cachePoolForm.title_refill_interval_ms = config.title_refill_interval_ms || 500
    cachePoolForm.title_threshold = config.title_threshold || 1000
  } catch (e) {
    console.error('Failed to load cache pool config:', e)
  } finally {
    cachePoolLoading.value = false
  }
}
```

**Step 3: 在模板中添加标题池配置卡片**

在 `数据池配置` tab 的 `<el-row>` 中，将原来的"标题/正文池"卡片改为"正文池"，并新增"标题池配置"卡片：

```vue
<!-- 数据池配置 -->
<el-tab-pane label="数据池配置" name="dataPool">
  <div class="data-pool-content" v-loading="cachePoolLoading">
    <el-form
      :model="cachePoolForm"
      label-width="140px"
    >
      <el-row :gutter="24">
        <!-- 标题池配置（新增） -->
        <el-col :xs="24" :lg="12">
          <div class="config-card">
            <div class="card-header">
              <span class="card-title">标题池配置</span>
            </div>
            <div class="card-content">
              <el-form-item label="标题池大小">
                <el-input-number
                  v-model="cachePoolForm.title_pool_size"
                  :min="100"
                  :max="100000"
                  :step="1000"
                />
                <span class="form-tip">条（每个分组）</span>
              </el-form-item>
              <el-form-item label="生成协程数">
                <el-input-number
                  v-model="cachePoolForm.title_workers"
                  :min="1"
                  :max="10"
                  :step="1"
                />
                <span class="form-tip">个（每个分组）</span>
              </el-form-item>
              <el-form-item label="生成间隔">
                <el-input-number
                  v-model="cachePoolForm.title_refill_interval_ms"
                  :min="100"
                  :max="60000"
                  :step="100"
                />
                <span class="form-tip">毫秒</span>
              </el-form-item>
              <el-form-item label="补充阈值">
                <el-input-number
                  v-model="cachePoolForm.title_threshold"
                  :min="10"
                  :max="cachePoolForm.title_pool_size"
                  :step="100"
                />
                <span class="form-tip">低于此值时触发补充</span>
              </el-form-item>
            </div>
          </div>
        </el-col>

        <!-- 正文池（原标题/正文池，移除标题相关） -->
        <el-col :xs="24" :lg="12">
          <div class="config-card">
            <div class="card-header">
              <span class="card-title">正文池</span>
            </div>
            <div class="card-content">
              <el-form-item label="正文池大小">
                <el-input-number
                  v-model="cachePoolForm.contents_size"
                  :min="100"
                  :max="100000"
                  :step="1000"
                />
                <span class="form-tip">条</span>
              </el-form-item>
              <el-form-item label="补充阈值">
                <el-input-number
                  v-model="cachePoolForm.threshold"
                  :min="10"
                  :max="cachePoolForm.contents_size"
                  :step="100"
                />
                <span class="form-tip">低于此值时触发补充</span>
              </el-form-item>
              <el-form-item label="检查间隔">
                <el-input-number
                  v-model="cachePoolForm.refill_interval_ms"
                  :min="100"
                  :max="60000"
                  :step="100"
                />
                <span class="form-tip">毫秒</span>
              </el-form-item>
            </div>
          </div>
        </el-col>
      </el-row>

      <el-row :gutter="24" style="margin-top: 16px;">
        <!-- 关键词/图片池（保持不变） -->
        <el-col :xs="24" :lg="12">
          <div class="config-card">
            <div class="card-header">
              <span class="card-title">关键词/图片池</span>
            </div>
            <div class="card-content">
              <el-form-item label="关键词池大小">
                <el-input-number
                  v-model="cachePoolForm.keywords_size"
                  :min="1000"
                  :max="500000"
                  :step="10000"
                />
                <span class="form-tip">条</span>
              </el-form-item>
              <el-form-item label="图片池大小">
                <el-input-number
                  v-model="cachePoolForm.images_size"
                  :min="1000"
                  :max="500000"
                  :step="10000"
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
                <span class="form-tip">毫秒（定期重新加载）</span>
              </el-form-item>
            </div>
          </div>
        </el-col>
      </el-row>

      <div class="form-actions">
        <el-button type="primary" :loading="cachePoolSaveLoading" @click="handleSaveCachePool">
          保存配置
        </el-button>
      </div>
    </el-form>
  </div>
</el-tab-pane>
```

**Step 4: Commit**

```bash
git add web/src/views/cache/CacheManage.vue
git commit -m "feat(web): 添加标题池配置界面"
```

---

## Task 8: 数据库迁移

**Files:**
- Create: `migrations/001_add_title_config.sql`

**Step 1: 创建迁移文件**

```sql
-- 添加标题生成配置字段到 pool_config 表
ALTER TABLE pool_config
ADD COLUMN title_pool_size INT DEFAULT 5000 AFTER refresh_interval_ms,
ADD COLUMN title_workers INT DEFAULT 2 AFTER title_pool_size,
ADD COLUMN title_refill_interval_ms INT DEFAULT 500 AFTER title_workers,
ADD COLUMN title_threshold INT DEFAULT 1000 AFTER title_refill_interval_ms;

-- 更新现有记录的默认值
UPDATE pool_config SET
  title_pool_size = 5000,
  title_workers = 2,
  title_refill_interval_ms = 500,
  title_threshold = 1000
WHERE id = 1;
```

**Step 2: Commit**

```bash
git add migrations/001_add_title_config.sql
git commit -m "chore(db): 添加标题生成配置字段迁移"
```

---

## Task 9: 最终验证

**Step 1: 编译检查**

```bash
cd api && go build ./...
```

Expected: 编译成功，无错误

**Step 2: 前端编译检查**

```bash
cd web && npm run build
```

Expected: 编译成功，无错误

**Step 3: 执行数据库迁移**

```bash
mysql -u root -p seo_generator < migrations/001_add_title_config.sql
```

**Step 4: 启动服务测试**

1. 启动 Go API 服务
2. 访问管理后台 - 缓存管理 - 数据池配置
3. 确认标题池配置卡片显示正常
4. 修改配置并保存，确认生效
5. 访问一个 SEO 页面，确认标题格式为：`关键词1+emoji1+关键词2+emoji2+关键词3`

**Step 5: 最终提交**

```bash
git add -A
git commit -m "feat: 完成标题动态生成功能

- 新增 TitleGenerator 从关键词+emoji动态生成标题
- 支持配置标题池大小、生成协程数、生成间隔、补充阈值
- 前端缓存管理页面新增标题池配置卡片
- 数据库迁移添加配置字段"
```
