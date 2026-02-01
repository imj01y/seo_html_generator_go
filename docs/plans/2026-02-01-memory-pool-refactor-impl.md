# 内存缓存池重构实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将标题和正文缓存池从 Redis 中间层重构为 Go 内存缓存，支持后台动态配置。

**Architecture:** Go PoolManager 管理内存中的 titles/contents 队列，后台 goroutine 每秒检查并从 DB 补充，通过 API 支持配置热更新。

**Tech Stack:** Go (Gin, sqlx), Vue 3 (Element Plus), MySQL

---

## Task 1: 创建数据库迁移脚本

**Files:**
- Create: `migrations/005_pool_config.sql`

**Step 1: 创建迁移文件**

```sql
-- migrations/005_pool_config.sql
-- 缓存池配置表

CREATE TABLE IF NOT EXISTS pool_config (
    id INT PRIMARY KEY DEFAULT 1,
    titles_size INT NOT NULL DEFAULT 5000 COMMENT '标题池大小',
    contents_size INT NOT NULL DEFAULT 5000 COMMENT '正文池大小',
    threshold INT NOT NULL DEFAULT 1000 COMMENT '补充阈值',
    refill_interval_ms INT NOT NULL DEFAULT 1000 COMMENT '检查间隔(毫秒)',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT chk_id CHECK (id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='缓存池配置（单行）';

-- 插入默认配置
INSERT IGNORE INTO pool_config (id, titles_size, contents_size, threshold, refill_interval_ms)
VALUES (1, 5000, 5000, 1000, 1000);
```

**Step 2: Commit**

```bash
git add migrations/005_pool_config.sql
git commit -m "feat(db): add pool_config table migration"
```

---

## Task 2: 创建 MemoryPool 内存池

**Files:**
- Create: `api/internal/service/memory_pool.go`

**Step 1: 创建内存池结构**

```go
// api/internal/service/memory_pool.go
package core

import (
	"sync"
)

// PoolItem represents an item in the pool
type PoolItem struct {
	ID   int64
	Text string
}

// MemoryPool is a thread-safe FIFO queue for pool items
type MemoryPool struct {
	items    []PoolItem
	mu       sync.RWMutex
	groupID  int
	poolType string // "titles" or "contents"
	maxSize  int
}

// NewMemoryPool creates a new memory pool
func NewMemoryPool(groupID int, poolType string, maxSize int) *MemoryPool {
	return &MemoryPool{
		items:    make([]PoolItem, 0, maxSize),
		groupID:  groupID,
		poolType: poolType,
		maxSize:  maxSize,
	}
}

// Pop removes and returns the first item from the pool
func (p *MemoryPool) Pop() (PoolItem, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.items) == 0 {
		return PoolItem{}, false
	}

	item := p.items[0]
	p.items = p.items[1:]
	return item, true
}

// Push adds items to the end of the pool
func (p *MemoryPool) Push(items []PoolItem) {
	if len(items) == 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Respect max size
	available := p.maxSize - len(p.items)
	if available <= 0 {
		return
	}
	if len(items) > available {
		items = items[:available]
	}

	p.items = append(p.items, items...)
}

// Len returns the current number of items in the pool
func (p *MemoryPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.items)
}

// Clear removes all items from the pool
func (p *MemoryPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = p.items[:0]
}

// Resize changes the max size of the pool
func (p *MemoryPool) Resize(newMaxSize int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.maxSize = newMaxSize

	// Truncate if current items exceed new max size
	if len(p.items) > newMaxSize {
		p.items = p.items[:newMaxSize]
	}
}

// GetGroupID returns the group ID
func (p *MemoryPool) GetGroupID() int {
	return p.groupID
}

// GetPoolType returns the pool type
func (p *MemoryPool) GetPoolType() string {
	return p.poolType
}

// GetMaxSize returns the max size
func (p *MemoryPool) GetMaxSize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.maxSize
}
```

**Step 2: 验证编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 3: Commit**

```bash
git add api/internal/service/memory_pool.go
git commit -m "feat(api): add MemoryPool implementation"
```

---

## Task 3: 创建 PoolConfig 配置结构

**Files:**
- Create: `api/internal/service/pool_config.go`

**Step 1: 创建配置结构**

```go
// api/internal/service/pool_config.go
package core

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// PoolConfig holds pool configuration
type PoolConfig struct {
	ID               int       `db:"id"`
	TitlesSize       int       `db:"titles_size"`
	ContentsSize     int       `db:"contents_size"`
	Threshold        int       `db:"threshold"`
	RefillIntervalMs int       `db:"refill_interval_ms"`
	UpdatedAt        time.Time `db:"updated_at"`
}

// RefillInterval returns the refill interval as time.Duration
func (c *PoolConfig) RefillInterval() time.Duration {
	return time.Duration(c.RefillIntervalMs) * time.Millisecond
}

// DefaultPoolConfig returns default configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		ID:               1,
		TitlesSize:       5000,
		ContentsSize:     5000,
		Threshold:        1000,
		RefillIntervalMs: 1000,
	}
}

// LoadPoolConfig loads configuration from database
func LoadPoolConfig(ctx context.Context, db *sqlx.DB) (*PoolConfig, error) {
	config := &PoolConfig{}
	err := db.GetContext(ctx, config, "SELECT * FROM pool_config WHERE id = 1")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load pool config, using defaults")
		return DefaultPoolConfig(), nil
	}
	return config, nil
}

// SavePoolConfig saves configuration to database
func SavePoolConfig(ctx context.Context, db *sqlx.DB, config *PoolConfig) error {
	query := `
		INSERT INTO pool_config (id, titles_size, contents_size, threshold, refill_interval_ms)
		VALUES (1, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			titles_size = VALUES(titles_size),
			contents_size = VALUES(contents_size),
			threshold = VALUES(threshold),
			refill_interval_ms = VALUES(refill_interval_ms)
	`
	_, err := db.ExecContext(ctx, query,
		config.TitlesSize,
		config.ContentsSize,
		config.Threshold,
		config.RefillIntervalMs,
	)
	return err
}
```

**Step 2: 验证编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 3: Commit**

```bash
git add api/internal/service/pool_config.go
git commit -m "feat(api): add PoolConfig structure and DB operations"
```

---

## Task 4: 创建 PoolManager 管理器

**Files:**
- Create: `api/internal/service/pool_manager.go`

**Step 1: 创建管理器结构**

```go
// api/internal/service/pool_manager.go
package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// PoolManager manages memory pools for titles and contents
type PoolManager struct {
	titles   map[int]*MemoryPool // groupID -> pool
	contents map[int]*MemoryPool // groupID -> pool
	config   *PoolConfig
	db       *sqlx.DB
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	updateCh chan UpdateTask
	wg       sync.WaitGroup
	stopped  atomic.Bool
}

// NewPoolManager creates a new pool manager
func NewPoolManager(db *sqlx.DB) *PoolManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &PoolManager{
		titles:   make(map[int]*MemoryPool),
		contents: make(map[int]*MemoryPool),
		config:   DefaultPoolConfig(),
		db:       db,
		ctx:      ctx,
		cancel:   cancel,
		updateCh: make(chan UpdateTask, 1000),
	}
}

// Start starts the pool manager
func (m *PoolManager) Start(ctx context.Context) error {
	// Load config from DB
	config, err := LoadPoolConfig(ctx, m.db)
	if err != nil {
		return fmt.Errorf("failed to load pool config: %w", err)
	}
	m.config = config

	// Discover and initialize pools for all groups
	groupIDs, err := m.discoverGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover groups: %w", err)
	}

	for _, gid := range groupIDs {
		m.getOrCreatePool("titles", gid)
		m.getOrCreatePool("contents", gid)
	}

	// Initial fill
	m.checkAndRefillAll()

	// Start background workers
	m.wg.Add(2)
	go m.refillLoop()
	go m.updateWorker()

	log.Info().
		Int("groups", len(groupIDs)).
		Int("titles_size", m.config.TitlesSize).
		Int("contents_size", m.config.ContentsSize).
		Msg("PoolManager started")

	return nil
}

// Stop stops the pool manager gracefully
func (m *PoolManager) Stop() {
	m.stopped.Store(true)
	m.cancel()
	close(m.updateCh)
	m.wg.Wait()
	log.Info().Msg("PoolManager stopped")
}

// discoverGroups finds all active group IDs
func (m *PoolManager) discoverGroups(ctx context.Context) ([]int, error) {
	query := `
		SELECT DISTINCT group_id FROM (
			SELECT group_id FROM titles WHERE status = 1
			UNION
			SELECT group_id FROM contents WHERE status = 1
		) t
	`
	var groupIDs []int
	err := m.db.SelectContext(ctx, &groupIDs, query)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return []int{1}, nil // Default to group 1
	}
	return groupIDs, nil
}

// getOrCreatePool gets or creates a pool for the given type and group
func (m *PoolManager) getOrCreatePool(poolType string, groupID int) *MemoryPool {
	m.mu.Lock()
	defer m.mu.Unlock()

	var pools map[int]*MemoryPool
	var maxSize int

	if poolType == "titles" {
		pools = m.titles
		maxSize = m.config.TitlesSize
	} else {
		pools = m.contents
		maxSize = m.config.ContentsSize
	}

	pool, exists := pools[groupID]
	if !exists {
		pool = NewMemoryPool(groupID, poolType, maxSize)
		pools[groupID] = pool
		log.Debug().Str("type", poolType).Int("group", groupID).Msg("Created new pool")
	}

	return pool
}

// Pop retrieves an item from the pool
func (m *PoolManager) Pop(poolType string, groupID int) (string, error) {
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
			return "", ErrPoolEmpty
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

// refillLoop runs the background refill check
func (m *PoolManager) refillLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.RefillInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAndRefillAll()
		case <-m.ctx.Done():
			return
		}
	}
}

// checkAndRefillAll checks and refills all pools
func (m *PoolManager) checkAndRefillAll() {
	m.mu.RLock()
	titlePools := make([]*MemoryPool, 0, len(m.titles))
	contentPools := make([]*MemoryPool, 0, len(m.contents))
	for _, p := range m.titles {
		titlePools = append(titlePools, p)
	}
	for _, p := range m.contents {
		contentPools = append(contentPools, p)
	}
	m.mu.RUnlock()

	for _, pool := range titlePools {
		if pool.Len() < m.config.Threshold {
			m.refillPool(pool)
		}
	}
	for _, pool := range contentPools {
		if pool.Len() < m.config.Threshold {
			m.refillPool(pool)
		}
	}
}

// refillPool refills a single pool from database
func (m *PoolManager) refillPool(pool *MemoryPool) {
	poolType := pool.GetPoolType()
	groupID := pool.GetGroupID()
	currentLen := pool.Len()
	maxSize := pool.GetMaxSize()
	need := maxSize - currentLen

	if need <= 0 {
		return
	}

	column := "title"
	if poolType == "contents" {
		column = "content"
	}

	query := fmt.Sprintf(`
		SELECT id, %s as text FROM %s
		WHERE group_id = ? AND status = 1
		ORDER BY batch_id DESC, id ASC
		LIMIT ?
	`, column, poolType)

	var items []PoolItem
	err := m.db.SelectContext(m.ctx, &items, query, groupID, need)
	if err != nil {
		log.Error().Err(err).Str("type", poolType).Int("group", groupID).Msg("Failed to refill pool")
		return
	}

	if len(items) > 0 {
		pool.Push(items)
		log.Debug().
			Str("type", poolType).
			Int("group", groupID).
			Int("added", len(items)).
			Int("total", pool.Len()).
			Msg("Pool refilled")
	}
}

// updateWorker processes status updates
func (m *PoolManager) updateWorker() {
	defer m.wg.Done()

	for task := range m.updateCh {
		select {
		case <-m.ctx.Done():
			return
		default:
			m.processUpdate(task)
		}
	}
}

// processUpdate updates the status of a consumed item
func (m *PoolManager) processUpdate(task UpdateTask) {
	if err := validatePoolType(task.Table); err != nil {
		return
	}
	query := fmt.Sprintf("UPDATE %s SET status = 0 WHERE id = ?", task.Table)
	_, err := m.db.ExecContext(m.ctx, query, task.ID)
	if err != nil {
		log.Warn().Err(err).Str("table", task.Table).Int64("id", task.ID).Msg("Failed to update status")
	}
}

// Reload reloads configuration from database
func (m *PoolManager) Reload(ctx context.Context) error {
	config, err := LoadPoolConfig(ctx, m.db)
	if err != nil {
		return err
	}

	m.mu.Lock()
	oldConfig := m.config
	m.config = config

	// Resize pools if needed
	if config.TitlesSize != oldConfig.TitlesSize {
		for _, pool := range m.titles {
			pool.Resize(config.TitlesSize)
		}
	}
	if config.ContentsSize != oldConfig.ContentsSize {
		for _, pool := range m.contents {
			pool.Resize(config.ContentsSize)
		}
	}
	m.mu.Unlock()

	log.Info().
		Int("titles_size", config.TitlesSize).
		Int("contents_size", config.ContentsSize).
		Int("threshold", config.Threshold).
		Int("interval_ms", config.RefillIntervalMs).
		Msg("PoolManager config reloaded")

	return nil
}

// GetStats returns pool statistics
func (m *PoolManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

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

	return map[string]interface{}{
		"titles":   titlesStats,
		"contents": contentsStats,
		"config": map[string]interface{}{
			"titles_size":        m.config.TitlesSize,
			"contents_size":      m.config.ContentsSize,
			"threshold":          m.config.Threshold,
			"refill_interval_ms": m.config.RefillIntervalMs,
		},
	}
}
```

**Step 2: 验证编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 3: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(api): add PoolManager with refill loop and config reload"
```

---

## Task 5: 创建 Pool API Handler

**Files:**
- Create: `api/internal/handler/pool.go`

**Step 1: 创建 API Handler**

```go
// api/internal/handler/pool.go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	core "seo-generator/api/internal/service"
)

// PoolHandler handles pool-related API requests
type PoolHandler struct {
	db          *sqlx.DB
	poolManager *core.PoolManager
}

// NewPoolHandler creates a new pool handler
func NewPoolHandler(db *sqlx.DB, poolManager *core.PoolManager) *PoolHandler {
	return &PoolHandler{
		db:          db,
		poolManager: poolManager,
	}
}

// GetConfig returns current pool configuration
func (h *PoolHandler) GetConfig(c *gin.Context) {
	config, err := core.LoadPoolConfig(c.Request.Context(), h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}

// UpdateConfig updates pool configuration
func (h *PoolHandler) UpdateConfig(c *gin.Context) {
	var req struct {
		TitlesSize       int `json:"titles_size"`
		ContentsSize     int `json:"contents_size"`
		Threshold        int `json:"threshold"`
		RefillIntervalMs int `json:"refill_interval_ms"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate
	if req.TitlesSize < 100 || req.TitlesSize > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "titles_size must be between 100 and 100000"})
		return
	}
	if req.ContentsSize < 100 || req.ContentsSize > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contents_size must be between 100 and 100000"})
		return
	}
	if req.Threshold < 10 || req.Threshold > req.TitlesSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "threshold must be between 10 and titles_size"})
		return
	}
	if req.RefillIntervalMs < 100 || req.RefillIntervalMs > 60000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refill_interval_ms must be between 100 and 60000"})
		return
	}

	config := &core.PoolConfig{
		TitlesSize:       req.TitlesSize,
		ContentsSize:     req.ContentsSize,
		Threshold:        req.Threshold,
		RefillIntervalMs: req.RefillIntervalMs,
	}

	// Save to DB
	if err := core.SavePoolConfig(c.Request.Context(), h.db, config); err != nil {
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

// GetStats returns pool statistics
func (h *PoolHandler) GetStats(c *gin.Context) {
	stats := h.poolManager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// Reload triggers a configuration reload
func (h *PoolHandler) Reload(c *gin.Context) {
	if err := h.poolManager.Reload(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
```

**Step 2: 验证编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 3: Commit**

```bash
git add api/internal/handler/pool.go
git commit -m "feat(api): add Pool API handler"
```

---

## Task 6: 注册 Pool API 路由

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 在 Dependencies 中添加 PoolManager**

在 `Dependencies` 结构体中添加：

```go
type Dependencies struct {
	// ... 现有字段 ...
	PoolManager      *core.PoolManager
}
```

**Step 2: 在 SetupRouter 中添加路由**

在 `SetupRouter` 函数中添加（在其他路由组之后）：

```go
	// Pool routes (require JWT)
	if deps.PoolManager != nil {
		poolHandler := NewPoolHandler(deps.DB, deps.PoolManager)
		poolGroup := r.Group("/api/pool")
		poolGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
		{
			poolGroup.GET("/config", poolHandler.GetConfig)
			poolGroup.PUT("/config", poolHandler.UpdateConfig)
			poolGroup.GET("/stats", poolHandler.GetStats)
			poolGroup.POST("/reload", poolHandler.Reload)
		}
	}
```

**Step 3: 验证编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 4: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "feat(api): register Pool API routes"
```

---

## Task 7: 修改 main.go 使用 PoolManager

**Files:**
- Modify: `api/cmd/main.go`

**Step 1: 替换 PoolConsumer 为 PoolManager**

找到 PoolConsumer 初始化代码并替换为：

```go
	// Initialize pool manager for titles and contents
	poolManager := core.NewPoolManager(db)
	if err := poolManager.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start PoolManager")
	}
	log.Info().Msg("PoolManager initialized")
```

**Step 2: 更新 Dependencies**

在创建 Dependencies 时添加 PoolManager：

```go
	deps := &api.Dependencies{
		// ... 现有字段 ...
		PoolManager:      poolManager,
	}
```

**Step 3: 更新 PageHandler 创建**

将 `poolConsumer` 参数改为 `poolManager`：

```go
	pageHandler := api.NewPageHandler(
		db,
		cfg,
		siteCache,
		templateCache,
		htmlCache,
		dataManager,
		funcsManager,
		poolManager,  // 改为 poolManager
	)
```

**Step 4: 更新关闭逻辑**

找到 `poolConsumer.Stop()` 并替换为：

```go
	// Stop pool manager
	poolManager.Stop()
	log.Info().Msg("PoolManager stopped")
```

**Step 5: 删除 PoolConsumer 相关代码**

移除所有 `poolConsumer` 相关的初始化和关闭代码。

**Step 6: 验证编译**

Run: `cd api && go build ./cmd/main.go`
Expected: 无错误

**Step 7: Commit**

```bash
git add api/cmd/main.go
git commit -m "feat(api): replace PoolConsumer with PoolManager in main.go"
```

---

## Task 8: 修改 PageHandler 使用 PoolManager

**Files:**
- Modify: `api/internal/handler/page.go`

**Step 1: 修改结构体字段**

将 `poolConsumer *core.PoolConsumer` 改为 `poolManager *core.PoolManager`

**Step 2: 修改构造函数参数**

将参数 `poolConsumer *core.PoolConsumer` 改为 `poolManager *core.PoolManager`

**Step 3: 修改 ServePage 中的调用**

将 `h.poolConsumer.PopWithFallback` 改为 `h.poolManager.Pop`：

```go
	// Get title and content from pool
	var title, content string
	if h.poolManager != nil {
		var err error
		title, err = h.poolManager.Pop("titles", articleGroupID)
		if err != nil {
			log.Warn().Err(err).Int("group", articleGroupID).Msg("Failed to get title from pool")
			titles := h.dataManager.GetRandomTitles(articleGroupID, 1)
			if len(titles) > 0 {
				title = titles[0]
			}
		}
		content, err = h.poolManager.Pop("contents", articleGroupID)
		if err != nil {
			log.Warn().Err(err).Int("group", articleGroupID).Msg("Failed to get content from pool")
			content = h.dataManager.GetRandomContent(articleGroupID)
		}
	} else {
		// Fallback to dataManager
		titles := h.dataManager.GetRandomTitles(articleGroupID, 1)
		if len(titles) > 0 {
			title = titles[0]
		}
		content = h.dataManager.GetRandomContent(articleGroupID)
	}
```

**Step 4: 验证编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 5: Commit**

```bash
git add api/internal/handler/page.go
git commit -m "feat(api): use PoolManager in PageHandler"
```

---

## Task 9: 删除旧的 PoolConsumer

**Files:**
- Delete: `api/internal/service/pool_consumer.go`

**Step 1: 删除文件**

```bash
git rm api/internal/service/pool_consumer.go
```

**Step 2: 验证编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 3: Commit**

```bash
git commit -m "refactor(api): remove deprecated PoolConsumer"
```

---

## Task 10: 删除 Python PoolFiller

**Files:**
- Delete: `content_worker/core/pool_filler.py`
- Modify: `content_worker/core/__init__.py`
- Modify: `content_worker/main.py`

**Step 1: 删除 pool_filler.py**

```bash
git rm content_worker/core/pool_filler.py
```

**Step 2: 从 __init__.py 移除导出**

移除：
```python
from .pool_filler import PoolFiller, PoolFillerManager
```

和 `__all__` 中的：
```python
'PoolFiller',
'PoolFillerManager',
```

**Step 3: 从 main.py 移除 PoolFillerManager 相关代码**

删除所有 `pool_filler_manager` 相关的导入、初始化和清理代码。

**Step 4: 验证语法**

Run: `python -m py_compile content_worker/main.py content_worker/core/__init__.py`
Expected: 无错误

**Step 5: Commit**

```bash
git add content_worker/
git commit -m "refactor(worker): remove PoolFiller (moved to Go)"
```

---

## Task 11: 添加前端 Pool 配置 API

**Files:**
- Create: `web/src/api/pool.ts`

**Step 1: 创建 API 文件**

```typescript
// web/src/api/pool.ts
import request from './request'

export interface PoolConfig {
  titles_size: number
  contents_size: number
  threshold: number
  refill_interval_ms: number
}

export interface PoolStats {
  titles: Record<number, { current: number; max_size: number; threshold: number }>
  contents: Record<number, { current: number; max_size: number; threshold: number }>
  config: PoolConfig
}

// 获取配置
export function getPoolConfig() {
  return request.get<PoolConfig>('/api/pool/config')
}

// 更新配置
export function updatePoolConfig(config: PoolConfig) {
  return request.put<{ success: boolean; config: PoolConfig }>('/api/pool/config', config)
}

// 获取统计
export function getPoolStats() {
  return request.get<PoolStats>('/api/pool/stats')
}

// 重载配置
export function reloadPool() {
  return request.post<{ success: boolean }>('/api/pool/reload')
}
```

**Step 2: Commit**

```bash
git add web/src/api/pool.ts
git commit -m "feat(web): add Pool API client"
```

---

## Task 12: 添加前端 Pool 配置 UI

**Files:**
- Modify: `web/src/views/settings/Settings.vue`

**Step 1: 添加缓存池配置 Tab**

在 `<el-tabs>` 中添加新的 tab-pane：

```vue
      <!-- 缓存池配置 -->
      <el-tab-pane label="缓存池配置" name="pool">
        <div class="tab-content">
          <el-form
            ref="poolFormRef"
            :model="poolForm"
            label-width="140px"
            v-loading="poolLoading"
          >
            <el-form-item label="标题池大小">
              <el-input-number
                v-model="poolForm.titles_size"
                :min="100"
                :max="100000"
                :step="1000"
              />
              <span class="form-tip">条</span>
            </el-form-item>
            <el-form-item label="正文池大小">
              <el-input-number
                v-model="poolForm.contents_size"
                :min="100"
                :max="100000"
                :step="1000"
              />
              <span class="form-tip">条</span>
            </el-form-item>
            <el-form-item label="补充阈值">
              <el-input-number
                v-model="poolForm.threshold"
                :min="10"
                :max="poolForm.titles_size"
                :step="100"
              />
              <span class="form-tip">低于此值时触发补充</span>
            </el-form-item>
            <el-form-item label="检查间隔">
              <el-input-number
                v-model="poolForm.refill_interval_ms"
                :min="100"
                :max="60000"
                :step="100"
              />
              <span class="form-tip">毫秒</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="poolSaveLoading" @click="handleSavePool">
                保存配置
              </el-button>
            </el-form-item>
          </el-form>

          <!-- 池状态统计 -->
          <el-divider content-position="left">运行状态</el-divider>
          <el-descriptions :column="2" border v-if="poolStats">
            <el-descriptions-item label="标题池">
              {{ getPoolSummary(poolStats.titles) }}
            </el-descriptions-item>
            <el-descriptions-item label="正文池">
              {{ getPoolSummary(poolStats.contents) }}
            </el-descriptions-item>
          </el-descriptions>
          <el-button @click="loadPoolStats" :loading="poolStatsLoading" style="margin-top: 10px">
            刷新状态
          </el-button>
        </div>
      </el-tab-pane>
```

**Step 2: 添加相关的 script 代码**

```typescript
import { getPoolConfig, updatePoolConfig, getPoolStats, type PoolConfig, type PoolStats } from '@/api/pool'

// 在 setup 中添加
const poolForm = ref<PoolConfig>({
  titles_size: 5000,
  contents_size: 5000,
  threshold: 1000,
  refill_interval_ms: 1000,
})
const poolLoading = ref(false)
const poolSaveLoading = ref(false)
const poolStats = ref<PoolStats | null>(null)
const poolStatsLoading = ref(false)

const loadPoolConfig = async () => {
  poolLoading.value = true
  try {
    const res = await getPoolConfig()
    poolForm.value = res.data
  } catch (e) {
    console.error('Failed to load pool config', e)
  } finally {
    poolLoading.value = false
  }
}

const handleSavePool = async () => {
  poolSaveLoading.value = true
  try {
    await updatePoolConfig(poolForm.value)
    ElMessage.success('缓存池配置已保存')
    loadPoolStats()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    poolSaveLoading.value = false
  }
}

const loadPoolStats = async () => {
  poolStatsLoading.value = true
  try {
    const res = await getPoolStats()
    poolStats.value = res.data
  } catch (e) {
    console.error('Failed to load pool stats', e)
  } finally {
    poolStatsLoading.value = false
  }
}

const getPoolSummary = (pools: Record<number, { current: number; max_size: number }>) => {
  const entries = Object.entries(pools)
  if (entries.length === 0) return '无数据'
  return entries.map(([gid, p]) => `分组${gid}: ${p.current}/${p.max_size}`).join(', ')
}

// 在 onMounted 中添加
onMounted(() => {
  // ... 现有代码 ...
  loadPoolConfig()
  loadPoolStats()
})
```

**Step 3: Commit**

```bash
git add web/src/views/settings/Settings.vue
git commit -m "feat(web): add Pool config UI in settings page"
```

---

## Task 13: 整体验证

**Step 1: 验证 Go 编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 2: 验证 Python 语法**

Run: `cd content_worker && python -m py_compile main.py core/__init__.py`
Expected: 无输出

**Step 3: 验证前端编译**

Run: `cd web && npm run build`
Expected: 编译成功

**Step 4: 最终提交**

```bash
git add -A
git commit -m "feat: complete memory pool refactor

- Replace Redis-based PoolConsumer with in-memory PoolManager
- Add pool_config table for dynamic configuration
- Add Pool API endpoints for config management
- Add Pool config UI in settings page
- Remove Python PoolFiller (functionality moved to Go)"
```

---

## 文件清单

### 新增文件
- `migrations/005_pool_config.sql` - 配置表迁移
- `api/internal/service/memory_pool.go` - 内存池实现
- `api/internal/service/pool_config.go` - 配置结构
- `api/internal/service/pool_manager.go` - 池管理器
- `api/internal/handler/pool.go` - API Handler
- `web/src/api/pool.ts` - 前端 API

### 修改文件
- `api/internal/handler/router.go` - 添加路由
- `api/cmd/main.go` - 使用 PoolManager
- `api/internal/handler/page.go` - 使用 PoolManager
- `web/src/views/settings/Settings.vue` - 添加配置 UI
- `content_worker/main.py` - 移除 PoolFiller
- `content_worker/core/__init__.py` - 移除导出

### 删除文件
- `api/internal/service/pool_consumer.go` - 被 PoolManager 替代
- `content_worker/core/pool_filler.py` - 不再需要
