# 数据池管理实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现数据池（关键词、图片、标题、正文）的加载、刷新、SEO 友好度分析功能。

**Architecture:** 数据池为只读随机选取型，不消耗，定期从数据库刷新，支持分站点数据隔离。

**Tech Stack:** Go sync.RWMutex, math/rand/v2

**依赖:** 阶段2（模板分析器）

---

## Task 1: 定义数据池结构

**Files:**
- Create: `go-page-server/core/data_pool.go`

**Step 1: 创建基础数据结构**

```go
package core

import (
	"math/rand/v2"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DataPool 只读数据池（随机选取，不消耗）
type DataPool struct {
	name     string
	items    []string
	mu       sync.RWMutex
	lastLoad time.Time

	// 统计
	totalSelects int64
}

// NewDataPool 创建数据池
func NewDataPool(name string) *DataPool {
	return &DataPool{
		name:  name,
		items: []string{},
	}
}

// Load 加载数据
func (p *DataPool) Load(items []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.items = items
	p.lastLoad = time.Now()

	log.Info().
		Str("pool", p.name).
		Int("count", len(items)).
		Msg("Data pool loaded")
}

// Get 随机获取一个数据（不消耗）
func (p *DataPool) Get() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.items) == 0 {
		return ""
	}

	idx := rand.IntN(len(p.items))
	p.totalSelects++
	return p.items[idx]
}

// GetN 随机获取 N 个数据（可能重复）
func (p *DataPool) GetN(n int) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.items) == 0 || n <= 0 {
		return nil
	}

	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = p.items[rand.IntN(len(p.items))]
	}
	p.totalSelects += int64(n)
	return result
}

// GetUnique 尽量获取不重复的 N 个数据
func (p *DataPool) GetUnique(n int) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.items) == 0 || n <= 0 {
		return nil
	}

	// 如果请求数量大于池大小，只返回池大小数量
	if n > len(p.items) {
		n = len(p.items)
	}

	// Fisher-Yates 部分洗牌
	indices := make([]int, len(p.items))
	for i := range indices {
		indices[i] = i
	}

	result := make([]string, n)
	for i := 0; i < n; i++ {
		j := i + rand.IntN(len(indices)-i)
		indices[i], indices[j] = indices[j], indices[i]
		result[i] = p.items[indices[i]]
	}

	p.totalSelects += int64(n)
	return result
}

// Count 返回数据量
func (p *DataPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.items)
}

// LastLoad 返回最后加载时间
func (p *DataPool) LastLoad() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastLoad
}

// Stats 返回统计信息
func (p *DataPool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"name":          p.name,
		"count":         len(p.items),
		"last_load":     p.lastLoad,
		"total_selects": p.totalSelects,
	}
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/data_pool.go
git commit -m "feat: add read-only data pool with random selection"
```

---

## Task 2: 实现数据池管理器

**Files:**
- Create: `go-page-server/core/data_pool_manager.go`

**Step 1: 创建管理器**

```go
package core

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DataPoolManager 数据池管理器
type DataPoolManager struct {
	db *sql.DB

	// 全局数据池
	keywords *DataPool
	images   *DataPool
	titles   *DataPool
	contents *DataPool

	// 分站点数据池
	siteKeywords map[int]*DataPool
	siteImages   map[int]*DataPool
	siteTitles   map[int]*DataPool
	siteContents map[int]*DataPool

	mu sync.RWMutex

	// 自动刷新
	refreshInterval time.Duration
	stopChan        chan struct{}
	running         bool
}

// NewDataPoolManager 创建数据池管理器
func NewDataPoolManager(db *sql.DB, refreshInterval time.Duration) *DataPoolManager {
	return &DataPoolManager{
		db:              db,
		keywords:        NewDataPool("keywords"),
		images:          NewDataPool("images"),
		titles:          NewDataPool("titles"),
		contents:        NewDataPool("contents"),
		siteKeywords:    make(map[int]*DataPool),
		siteImages:      make(map[int]*DataPool),
		siteTitles:      make(map[int]*DataPool),
		siteContents:    make(map[int]*DataPool),
		refreshInterval: refreshInterval,
		stopChan:        make(chan struct{}),
	}
}

// LoadAll 加载所有数据池
func (m *DataPoolManager) LoadAll(ctx context.Context) error {
	if err := m.loadKeywords(ctx); err != nil {
		return err
	}
	if err := m.loadImages(ctx); err != nil {
		return err
	}
	if err := m.loadTitles(ctx); err != nil {
		return err
	}
	if err := m.loadContents(ctx); err != nil {
		return err
	}

	log.Info().Msg("All data pools loaded")
	return nil
}

// loadKeywords 加载关键词
func (m *DataPoolManager) loadKeywords(ctx context.Context) error {
	rows, err := m.db.QueryContext(ctx, "SELECT word FROM keywords WHERE status = 1")
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			continue
		}
		items = append(items, word)
	}

	m.keywords.Load(items)
	return nil
}

// loadImages 加载图片
func (m *DataPoolManager) loadImages(ctx context.Context) error {
	rows, err := m.db.QueryContext(ctx, "SELECT url FROM images WHERE status = 1")
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			continue
		}
		items = append(items, url)
	}

	m.images.Load(items)
	return nil
}

// loadTitles 加载标题
func (m *DataPoolManager) loadTitles(ctx context.Context) error {
	rows, err := m.db.QueryContext(ctx, "SELECT title FROM titles WHERE status = 1")
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			continue
		}
		items = append(items, title)
	}

	m.titles.Load(items)
	return nil
}

// loadContents 加载正文
func (m *DataPoolManager) loadContents(ctx context.Context) error {
	rows, err := m.db.QueryContext(ctx, "SELECT content FROM contents WHERE status = 1")
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			continue
		}
		items = append(items, content)
	}

	m.contents.Load(items)
	return nil
}

// LoadSiteData 加载站点专属数据
func (m *DataPoolManager) LoadSiteData(ctx context.Context, siteID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 加载站点关键词
	rows, err := m.db.QueryContext(ctx,
		"SELECT word FROM site_keywords WHERE site_id = ? AND status = 1", siteID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var keywords []string
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			continue
		}
		keywords = append(keywords, word)
	}

	if len(keywords) > 0 {
		pool := NewDataPool("site_keywords_" + string(rune(siteID)))
		pool.Load(keywords)
		m.siteKeywords[siteID] = pool
	}

	return nil
}

// GetKeyword 获取关键词（优先站点专属）
func (m *DataPoolManager) GetKeyword(siteID int) string {
	m.mu.RLock()
	sitePool := m.siteKeywords[siteID]
	m.mu.RUnlock()

	if sitePool != nil && sitePool.Count() > 0 {
		return sitePool.Get()
	}
	return m.keywords.Get()
}

// GetImage 获取图片
func (m *DataPoolManager) GetImage(siteID int) string {
	m.mu.RLock()
	sitePool := m.siteImages[siteID]
	m.mu.RUnlock()

	if sitePool != nil && sitePool.Count() > 0 {
		return sitePool.Get()
	}
	return m.images.Get()
}

// GetTitle 获取标题
func (m *DataPoolManager) GetTitle(siteID int) string {
	m.mu.RLock()
	sitePool := m.siteTitles[siteID]
	m.mu.RUnlock()

	if sitePool != nil && sitePool.Count() > 0 {
		return sitePool.Get()
	}
	return m.titles.Get()
}

// GetContent 获取正文
func (m *DataPoolManager) GetContent(siteID int) string {
	m.mu.RLock()
	sitePool := m.siteContents[siteID]
	m.mu.RUnlock()

	if sitePool != nil && sitePool.Count() > 0 {
		return sitePool.Get()
	}
	return m.contents.Get()
}

// StartAutoRefresh 启动自动刷新
func (m *DataPoolManager) StartAutoRefresh() {
	if m.running {
		return
	}
	m.running = true

	go func() {
		ticker := time.NewTicker(m.refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopChan:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := m.LoadAll(ctx); err != nil {
					log.Error().Err(err).Msg("Failed to refresh data pools")
				}
				cancel()
			}
		}
	}()

	log.Info().
		Dur("interval", m.refreshInterval).
		Msg("Data pool auto-refresh started")
}

// StopAutoRefresh 停止自动刷新
func (m *DataPoolManager) StopAutoRefresh() {
	if !m.running {
		return
	}
	close(m.stopChan)
	m.running = false
	log.Info().Msg("Data pool auto-refresh stopped")
}

// GetStats 获取所有数据池统计
func (m *DataPoolManager) GetStats() DataPoolStats {
	return DataPoolStats{
		KeywordsCount: m.keywords.Count(),
		ImagesCount:   m.images.Count(),
		TitlesCount:   m.titles.Count(),
		ContentsCount: m.contents.Count(),
	}
}

// GetDetailedStats 获取详细统计
func (m *DataPoolManager) GetDetailedStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	siteStats := make(map[int]map[string]int)
	for siteID := range m.siteKeywords {
		siteStats[siteID] = map[string]int{
			"keywords": m.siteKeywords[siteID].Count(),
		}
	}

	return map[string]interface{}{
		"global": map[string]interface{}{
			"keywords": m.keywords.Stats(),
			"images":   m.images.Stats(),
			"titles":   m.titles.Stats(),
			"contents": m.contents.Stats(),
		},
		"sites":   siteStats,
		"running": m.running,
	}
}

// Refresh 立即刷新指定数据池
func (m *DataPoolManager) Refresh(ctx context.Context, poolName string) error {
	switch poolName {
	case "keywords":
		return m.loadKeywords(ctx)
	case "images":
		return m.loadImages(ctx)
	case "titles":
		return m.loadTitles(ctx)
	case "contents":
		return m.loadContents(ctx)
	case "all":
		return m.LoadAll(ctx)
	default:
		return nil
	}
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/data_pool_manager.go
git commit -m "feat: add data pool manager with site isolation"
```

---

## Task 3: 集成 SEO 分析

**Files:**
- Modify: `go-page-server/core/data_pool_manager.go`

**Step 1: 添加 SEO 分析方法**

在 `data_pool_manager.go` 末尾追加：

```go
// AnalyzeSEO 分析 SEO 友好度
func (m *DataPoolManager) AnalyzeSEO(analyzer *TemplateAnalyzer) []DataPoolSEOAnalysis {
	stats := m.GetStats()
	return analyzer.AnalyzeSEOFriendliness(stats)
}

// GetRecommendations 获取数据池优化建议
func (m *DataPoolManager) GetRecommendations(analyzer *TemplateAnalyzer) map[string]PoolRecommendation {
	stats := m.GetStats()
	maxStats, _ := analyzer.GetMaxStats()

	recommendations := make(map[string]PoolRecommendation)

	recommendations["keywords"] = getRecommendation(
		"关键词",
		stats.KeywordsCount,
		maxStats.RandomKeyword,
	)

	recommendations["images"] = getRecommendation(
		"图片",
		stats.ImagesCount,
		maxStats.RandomImage,
	)

	recommendations["titles"] = getRecommendation(
		"标题",
		stats.TitlesCount,
		maxStats.RandomTitle,
	)

	recommendations["contents"] = getRecommendation(
		"正文",
		stats.ContentsCount,
		maxStats.RandomContent+maxStats.ContentPinyin,
	)

	return recommendations
}

// PoolRecommendation 数据池建议
type PoolRecommendation struct {
	DataType       string    `json:"data_type"`
	CurrentCount   int       `json:"current_count"`
	CallsPerPage   int       `json:"calls_per_page"`
	RecommendedMin int       `json:"recommended_min"`
	RepeatRate     float64   `json:"repeat_rate"`
	Status         SEORating `json:"status"`
	Action         string    `json:"action"`
}

func getRecommendation(dataType string, currentCount, callsPerPage int) PoolRecommendation {
	rec := PoolRecommendation{
		DataType:     dataType,
		CurrentCount: currentCount,
		CallsPerPage: callsPerPage,
	}

	if callsPerPage == 0 {
		rec.Status = SEORatingExcellent
		rec.Action = "无需操作（模板未使用）"
		return rec
	}

	// 目标重复率 5%
	rec.RecommendedMin = GetRecommendedPoolSize(callsPerPage, 5)
	rec.RepeatRate = float64(callsPerPage) / float64(currentCount) * 100
	if rec.RepeatRate > 100 {
		rec.RepeatRate = 100
	}

	switch {
	case rec.RepeatRate < 5:
		rec.Status = SEORatingExcellent
		rec.Action = "无需操作"
	case rec.RepeatRate < 15:
		rec.Status = SEORatingGood
		rec.Action = "可选优化"
	case rec.RepeatRate < 30:
		rec.Status = SEORatingFair
		rec.Action = fmt.Sprintf("建议增加到 %d 条", rec.RecommendedMin)
	default:
		rec.Status = SEORatingPoor
		rec.Action = fmt.Sprintf("强烈建议增加到 %d 条", rec.RecommendedMin)
	}

	return rec
}
```

需要添加 `fmt` 导入。

**Step 2: Commit**

```bash
git add go-page-server/core/data_pool_manager.go
git commit -m "feat: add SEO analysis to data pool manager"
```

---

## Task 4: 添加测试

**Files:**
- Create: `go-page-server/core/data_pool_test.go`

**Step 1: 创建测试文件**

```go
package core

import (
	"testing"
)

func TestDataPool_BasicOperations(t *testing.T) {
	pool := NewDataPool("test")

	// 加载数据
	items := []string{"a", "b", "c", "d", "e"}
	pool.Load(items)

	if pool.Count() != 5 {
		t.Errorf("Expected count 5, got %d", pool.Count())
	}

	// 获取数据
	item := pool.Get()
	if item == "" {
		t.Error("Expected non-empty item")
	}

	// 验证是数据池中的数据
	found := false
	for _, v := range items {
		if v == item {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Item %s not in pool", item)
	}
}

func TestDataPool_GetN(t *testing.T) {
	pool := NewDataPool("test")
	pool.Load([]string{"a", "b", "c"})

	items := pool.GetN(5)
	if len(items) != 5 {
		t.Errorf("Expected 5 items, got %d", len(items))
	}
}

func TestDataPool_GetUnique(t *testing.T) {
	pool := NewDataPool("test")
	pool.Load([]string{"a", "b", "c", "d", "e"})

	items := pool.GetUnique(3)
	if len(items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(items))
	}

	// 检查唯一性
	seen := make(map[string]bool)
	for _, item := range items {
		if seen[item] {
			t.Errorf("Duplicate item: %s", item)
		}
		seen[item] = true
	}
}

func TestDataPool_GetUniqueExceedsSize(t *testing.T) {
	pool := NewDataPool("test")
	pool.Load([]string{"a", "b", "c"})

	// 请求超过池大小
	items := pool.GetUnique(10)
	if len(items) != 3 {
		t.Errorf("Expected 3 items (pool size), got %d", len(items))
	}
}

func TestDataPool_EmptyPool(t *testing.T) {
	pool := NewDataPool("test")

	item := pool.Get()
	if item != "" {
		t.Errorf("Expected empty string from empty pool, got %s", item)
	}

	items := pool.GetN(5)
	if items != nil {
		t.Error("Expected nil from empty pool")
	}
}

func TestDataPool_Stats(t *testing.T) {
	pool := NewDataPool("test")
	pool.Load([]string{"a", "b", "c"})

	// 执行一些操作
	pool.Get()
	pool.GetN(3)

	stats := pool.Stats()

	if stats["name"] != "test" {
		t.Errorf("Expected name=test, got %s", stats["name"])
	}
	if stats["count"].(int) != 3 {
		t.Errorf("Expected count=3, got %d", stats["count"])
	}
	if stats["total_selects"].(int64) != 4 {
		t.Errorf("Expected total_selects=4, got %d", stats["total_selects"])
	}
}

func TestPoolRecommendation(t *testing.T) {
	tests := []struct {
		name         string
		currentCount int
		callsPerPage int
		wantStatus   SEORating
	}{
		{
			name:         "优秀",
			currentCount: 10000,
			callsPerPage: 100,
			wantStatus:   SEORatingExcellent,
		},
		{
			name:         "良好",
			currentCount: 1000,
			callsPerPage: 100,
			wantStatus:   SEORatingGood,
		},
		{
			name:         "一般",
			currentCount: 500,
			callsPerPage: 100,
			wantStatus:   SEORatingFair,
		},
		{
			name:         "差",
			currentCount: 100,
			callsPerPage: 100,
			wantStatus:   SEORatingPoor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := getRecommendation("test", tt.currentCount, tt.callsPerPage)
			if rec.Status != tt.wantStatus {
				t.Errorf("Expected status %s, got %s", tt.wantStatus, rec.Status)
			}
		})
	}
}
```

**Step 2: 运行测试**

```bash
cd go-page-server && go test -v ./core/... -run TestDataPool
```

Expected: PASS

**Step 3: Commit**

```bash
git add go-page-server/core/data_pool_test.go
git commit -m "test: add data pool tests"
```

---

## Task 5: 更新 main.go 初始化

**Files:**
- Modify: `go-page-server/main.go`

**Step 1: 添加数据池管理器初始化**

在 `main()` 函数中添加：

```go
// 初始化数据池管理器
dataPoolManager := core.NewDataPoolManager(db, 5*time.Minute)

// 加载所有数据池
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
if err := dataPoolManager.LoadAll(ctx); err != nil {
	log.Fatal().Err(err).Msg("Failed to load data pools")
}
cancel()

// 启动自动刷新
dataPoolManager.StartAutoRefresh()
defer dataPoolManager.StopAutoRefresh()

// 集成 SEO 分析
seoAnalysis := dataPoolManager.AnalyzeSEO(templateAnalyzer)
for _, analysis := range seoAnalysis {
	log.Info().
		Str("type", analysis.DataType).
		Float64("repeat_rate", analysis.ExpectedRepeatRate).
		Str("rating", string(analysis.Rating)).
		Str("suggestion", analysis.Suggestion).
		Msg("SEO analysis")
}
```

**Step 2: Commit**

```bash
git add go-page-server/main.go
git commit -m "feat: initialize data pool manager in main"
```

---

## 完成检查清单

- [ ] Task 1: 数据池结构
- [ ] Task 2: 数据池管理器
- [ ] Task 3: SEO 分析集成
- [ ] Task 4: 测试覆盖
- [ ] Task 5: main.go 初始化

所有任务完成后运行完整测试：

```bash
cd go-page-server && go test -v ./...
```
