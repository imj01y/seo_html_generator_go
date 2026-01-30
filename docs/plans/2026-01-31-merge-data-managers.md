# 合并 DataManager 和 DataPoolManager 实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 `DataManager` 和 `DataPoolManager` 合并为单一的 `DataManager`，解决数据刷新不同步导致页面渲染为空的问题。

**Architecture:** 保留 `DataManager` 的分组功能（按 group_id 存储数据），吸收 `DataPoolManager` 的自动刷新、统计、SEO 分析功能。删除 `DataPoolManager` 和 `DataPool`，统一使用增强后的 `DataManager`。

**Tech Stack:** Go 1.24, Gin, sqlx

---

## 背景

当前存在两个独立的数据管理器：
- `DataManager` - 被 `PageHandler` 使用，支持分组，但没有自动刷新
- `DataPoolManager` - 有自动刷新和 SEO 分析，但不被页面渲染使用

这导致管理后台添加数据后，刷新 API 只更新了 `DataPoolManager`，而 `PageHandler` 使用的 `DataManager` 从未被刷新，造成页面渲染数据为空。

---

## Task 1: 增强 DataManager - 添加自动刷新功能

**Files:**
- Modify: `api/internal/service/data_manager.go`

**Step 1: 添加自动刷新相关字段和导入**

在 `DataManager` 结构体中添加：

```go
package core

import (
	"context"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// DataManager manages keywords, images, titles, and content data
type DataManager struct {
	db           *sqlx.DB
	keywords     map[int][]string // group_id -> keywords (pre-encoded)
	rawKeywords  map[int][]string // group_id -> keywords (raw, not encoded)
	imageURLs    map[int][]string // group_id -> image URLs
	titles       map[int][]string // group_id -> titles
	contents     map[int][]string // group_id -> contents
	encoder      *HTMLEntityEncoder
	emojiManager *EmojiManager
	mu           sync.RWMutex
	lastReload   time.Time
	reloadMutex  sync.Mutex

	// 自动刷新（从 DataPoolManager 吸收）
	refreshInterval time.Duration
	stopChan        chan struct{}
	running         atomic.Bool
	refreshCount    atomic.Int64
}
```

**Step 2: 修改 NewDataManager 函数**

```go
// NewDataManager creates a new data manager
func NewDataManager(db *sqlx.DB, encoder *HTMLEntityEncoder, refreshInterval time.Duration) *DataManager {
	return &DataManager{
		db:              db,
		keywords:        make(map[int][]string),
		rawKeywords:     make(map[int][]string),
		imageURLs:       make(map[int][]string),
		titles:          make(map[int][]string),
		contents:        make(map[int][]string),
		encoder:         encoder,
		emojiManager:    NewEmojiManager(),
		refreshInterval: refreshInterval,
	}
}
```

**Step 3: 添加自动刷新方法**

在文件末尾添加：

```go
// StartAutoRefresh 启动自动刷新
func (m *DataManager) StartAutoRefresh(groupIDs []int) {
	if m.running.Swap(true) {
		return // 已经在运行
	}

	m.stopChan = make(chan struct{})

	go func() {
		ticker := time.NewTicker(m.refreshInterval)
		defer ticker.Stop()

		log.Info().Dur("interval", m.refreshInterval).Msg("DataManager auto refresh started")

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				for _, gid := range groupIDs {
					if err := m.LoadAllForGroup(ctx, gid); err != nil {
						log.Error().Err(err).Int("group_id", gid).Msg("Auto refresh failed for group")
					}
				}
				m.refreshCount.Add(1)
				log.Info().Int64("count", m.refreshCount.Load()).Msg("DataManager auto refresh completed")
				cancel()
			case <-m.stopChan:
				log.Info().Msg("DataManager auto refresh stopped")
				return
			}
		}
	}()
}

// StopAutoRefresh 停止自动刷新
func (m *DataManager) StopAutoRefresh() {
	if !m.running.Swap(false) {
		return // 已经停止
	}
	close(m.stopChan)
}

// IsAutoRefreshRunning 返回自动刷新是否在运行
func (m *DataManager) IsAutoRefreshRunning() bool {
	return m.running.Load()
}

// GetRefreshCount 返回刷新次数
func (m *DataManager) GetRefreshCount() int64 {
	return m.refreshCount.Load()
}
```

**Step 4: 添加 Refresh 方法（用于手动刷新指定池）**

```go
// Refresh 刷新指定分组的指定数据池
func (m *DataManager) Refresh(ctx context.Context, groupID int, poolName string) error {
	switch poolName {
	case "keywords":
		_, err := m.LoadKeywords(ctx, groupID, 50000)
		return err
	case "images":
		_, err := m.LoadImageURLs(ctx, groupID, 50000)
		return err
	case "titles":
		_, err := m.LoadTitles(ctx, groupID, 10000)
		return err
	case "contents":
		_, err := m.LoadContents(ctx, groupID, 5000)
		return err
	case "all":
		return m.LoadAllForGroup(ctx, groupID)
	default:
		return fmt.Errorf("unknown pool name: %s", poolName)
	}
}

// RefreshAll 刷新所有分组的所有数据
func (m *DataManager) RefreshAll(ctx context.Context, groupIDs []int) error {
	var errs []error
	for _, gid := range groupIDs {
		if err := m.LoadAllForGroup(ctx, gid); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to refresh %d groups", len(errs))
	}
	return nil
}
```

**Step 5: 添加需要的 import**

确保文件顶部有 `"fmt"` 导入。

**Step 6: 验证编译**

Run: `cd E:\j\模板\seo_html_generator\api && go build ./...`
Expected: 编译成功（可能有 unused 警告，后续步骤会修复）

**Step 7: Commit**

```bash
git add api/internal/service/data_manager.go
git commit -m "feat(data-manager): add auto-refresh capability

- Add refreshInterval, stopChan, running, refreshCount fields
- Add StartAutoRefresh/StopAutoRefresh methods
- Add Refresh/RefreshAll methods for manual refresh
- Update NewDataManager to accept refreshInterval parameter"
```

---

## Task 2: 增强 DataManager - 添加统计功能

**Files:**
- Modify: `api/internal/service/data_manager.go`

**Step 1: 添加 DataManagerStats 结构体**

在文件顶部（DataManager 结构体之前）添加：

```go
// DataManagerStats 数据管理器统计
type DataManagerStats struct {
	Keywords     int       `json:"keywords"`
	Images       int       `json:"images"`
	Titles       int       `json:"titles"`
	Contents     int       `json:"contents"`
	GroupCount   int       `json:"group_count"`
	LastRefresh  time.Time `json:"last_refresh"`
	AutoRefresh  bool      `json:"auto_refresh"`
	RefreshCount int64     `json:"refresh_count"`
}
```

**Step 2: 添加 GetPoolStats 方法**

```go
// GetPoolStats 返回数据池统计（兼容 DataPoolManager 接口）
func (m *DataManager) GetPoolStats() DataManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalKeywords, totalImages, totalTitles, totalContents int
	groupIDs := make(map[int]bool)

	for gid, items := range m.keywords {
		totalKeywords += len(items)
		groupIDs[gid] = true
	}
	for gid, items := range m.imageURLs {
		totalImages += len(items)
		groupIDs[gid] = true
	}
	for gid, items := range m.titles {
		totalTitles += len(items)
		groupIDs[gid] = true
	}
	for gid, items := range m.contents {
		totalContents += len(items)
		groupIDs[gid] = true
	}

	return DataManagerStats{
		Keywords:     totalKeywords,
		Images:       totalImages,
		Titles:       totalTitles,
		Contents:     totalContents,
		GroupCount:   len(groupIDs),
		LastRefresh:  m.lastReload,
		AutoRefresh:  m.running.Load(),
		RefreshCount: m.refreshCount.Load(),
	}
}
```

**Step 3: 添加 GetDetailedStats 方法**

```go
// GetDetailedStats 返回详细统计信息
func (m *DataManager) GetDetailedStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make(map[int]map[string]int)
	groupIDs := make(map[int]bool)

	for gid := range m.keywords {
		groupIDs[gid] = true
	}
	for gid := range m.imageURLs {
		groupIDs[gid] = true
	}
	for gid := range m.titles {
		groupIDs[gid] = true
	}
	for gid := range m.contents {
		groupIDs[gid] = true
	}

	for gid := range groupIDs {
		groups[gid] = map[string]int{
			"keywords": len(m.keywords[gid]),
			"images":   len(m.imageURLs[gid]),
			"titles":   len(m.titles[gid]),
			"contents": len(m.contents[gid]),
		}
	}

	return map[string]interface{}{
		"groups":        groups,
		"last_refresh":  m.lastReload,
		"auto_refresh":  m.running.Load(),
		"refresh_count": m.refreshCount.Load(),
	}
}
```

**Step 4: 验证编译**

Run: `cd E:\j\模板\seo_html_generator\api && go build ./...`
Expected: 编译成功

**Step 5: Commit**

```bash
git add api/internal/service/data_manager.go
git commit -m "feat(data-manager): add statistics methods

- Add DataManagerStats struct for pool statistics
- Add GetPoolStats method (compatible with old DataPoolManager interface)
- Add GetDetailedStats method for detailed group-level stats"
```

---

## Task 3: 更新 main.go - 移除 DataPoolManager

**Files:**
- Modify: `api/cmd/main.go`

**Step 1: 修改 dataManager 创建，传入 refreshInterval**

找到：
```go
dataManager := core.NewDataManager(db, core.GetEncoder())
```

改为：
```go
dataManager := core.NewDataManager(db, core.GetEncoder(), 5*time.Minute)
```

**Step 2: 删除 dataPoolManager 相关代码**

删除以下代码块（约 142-188 行附近）：
```go
// Initialize data pool manager
log.Info().Msg("Initializing data pool manager...")
dataPoolManager := core.NewDataPoolManager(db.DB, 5*time.Minute)

// Load all data pools
loadCtx, loadCancel := context.WithTimeout(context.Background(), 30*time.Second)
if err := dataPoolManager.LoadAll(loadCtx); err != nil {
    log.Warn().Err(err).Msg("Failed to load some data pools")
}
loadCancel()

// Start auto-refresh
dataPoolManager.StartAutoRefresh()
```

以及 SEO analysis 部分（约 168-188 行）：
```go
// SEO analysis
seoAnalysis := dataPoolManager.AnalyzeSEO(templateAnalyzer)
... (整个 SEO analysis 块)
```

**Step 3: 在数据加载后启动自动刷新**

在数据加载完成后（约 259 行 `Msg("Initial data loaded for all groups")` 之后）添加：

```go
// 启动自动刷新
allGroupIDs := make([]int, 0)
for _, gid := range keywordGroupIDs {
	allGroupIDs = append(allGroupIDs, gid)
}
dataManager.StartAutoRefresh(allGroupIDs)
log.Info().Int("groups", len(allGroupIDs)).Msg("DataManager auto refresh started")
```

**Step 4: 修改 Dependencies 结构**

找到：
```go
deps := &api.Dependencies{
    ...
    DataPoolManager:  dataPoolManager,
    ...
}
```

改为：
```go
deps := &api.Dependencies{
    ...
    DataManager:      dataManager,
    ...
}
```

**Step 5: 修改关闭逻辑**

找到：
```go
// Stop data pool auto-refresh
dataPoolManager.StopAutoRefresh()
log.Info().Msg("Data pool auto-refresh stopped")
```

改为：
```go
// Stop data manager auto-refresh
dataManager.StopAutoRefresh()
log.Info().Msg("DataManager auto-refresh stopped")
```

**Step 6: 验证编译**

Run: `cd E:\j\模板\seo_html_generator\api && go build ./...`
Expected: 编译失败（因为 Dependencies 和 router.go 还在引用 DataPoolManager）

**Step 7: Commit（暂不提交，等 Task 4 完成后一起提交）**

---

## Task 4: 更新 router.go - 使用 DataManager 替换 DataPoolManager

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 修改 Dependencies 结构体**

找到：
```go
type Dependencies struct {
    ...
    DataPoolManager  *core.DataPoolManager
    ...
}
```

改为：
```go
type Dependencies struct {
    ...
    DataManager      *core.DataManager
    ...
}
```

**Step 2: 修改 dataStatsHandler**

找到 `dataStatsHandler` 函数，将 `deps.DataPoolManager` 改为 `deps.DataManager`：

```go
func dataStatsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.DataManager == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		stats := deps.DataManager.GetDetailedStats()
		core.Success(c, stats)
	}
}
```

**Step 3: 修改 dataSEOHandler**

暂时移除 SEO 分析功能（或返回空结果），因为这需要 TemplateAnalyzer 配合：

```go
func dataSEOHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 重新实现 SEO 分析功能
		core.Success(c, gin.H{"message": "SEO analysis temporarily disabled during refactoring"})
	}
}
```

**Step 4: 修改 dataRecommendationsHandler**

同样暂时返回空结果：

```go
func dataRecommendationsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 重新实现推荐功能
		core.Success(c, gin.H{"message": "Recommendations temporarily disabled during refactoring"})
	}
}
```

**Step 5: 修改 dataRefreshHandler**

```go
func dataRefreshHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.DataManager == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		var req dataRefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			core.FailWithMessage(c, core.ErrInvalidParam, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// 刷新默认分组（group_id=1）的数据
		if err := deps.DataManager.Refresh(ctx, 1, req.Pool); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}

		stats := deps.DataManager.GetPoolStats()
		core.Success(c, gin.H{
			"success": true,
			"pool":    req.Pool,
			"stats":   stats,
		})
	}
}
```

**Step 6: 修改 systemInfoHandler 中的 DataPoolManager 引用**

找到：
```go
if deps.DataPoolManager != nil {
    info["data"] = deps.DataPoolManager.GetStats()
}
```

改为：
```go
if deps.DataManager != nil {
    info["data"] = deps.DataManager.GetPoolStats()
}
```

**Step 7: 修改 dashboardHandler 中的 DataPoolManager 引用**

找到：
```go
if deps.DataPoolManager != nil {
    dataStats := deps.DataPoolManager.GetStats()
    ...
}
```

改为：
```go
if deps.DataManager != nil {
    dataStats := deps.DataManager.GetPoolStats()
    ...
}
```

**Step 8: 验证编译**

Run: `cd E:\j\模板\seo_html_generator\api && go build ./...`
Expected: 可能还有 task_handlers.go 的错误

---

## Task 5: 更新 task_handlers.go - 使用 DataManager

**Files:**
- Modify: `api/internal/service/task_handlers.go`

**Step 1: 修改 RefreshDataHandler 结构体**

```go
// RefreshDataHandler 刷新数据池处理器
type RefreshDataHandler struct {
	dataManager *DataManager
}

// NewRefreshDataHandler 创建刷新数据池处理器
func NewRefreshDataHandler(manager *DataManager) *RefreshDataHandler {
	return &RefreshDataHandler{
		dataManager: manager,
	}
}
```

**Step 2: 修改 Handle 方法**

```go
// Handle 执行刷新数据池任务
func (h *RefreshDataHandler) Handle(task *ScheduledTask) TaskResult {
	startTime := time.Now()

	params, err := ParseRefreshDataParams(task.Params)
	if err != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("parse params failed: %v", err),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Info().
		Str("pool_name", params.PoolName).
		Int("site_id", params.SiteID).
		Msg("Refreshing data pool")

	// 使用分组 ID（默认为 1）
	groupID := 1
	if params.SiteID > 0 {
		groupID = params.SiteID // 如果指定了 site_id，用作 group_id
	}

	refreshErr := h.dataManager.Refresh(ctx, groupID, params.PoolName)

	if refreshErr != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("refresh failed: %v", refreshErr),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	stats := h.dataManager.GetPoolStats()
	return TaskResult{
		Success:  true,
		Message:  fmt.Sprintf("refreshed %s, keywords=%d, images=%d, titles=%d, contents=%d", params.PoolName, stats.Keywords, stats.Images, stats.Titles, stats.Contents),
		Duration: time.Since(startTime).Milliseconds(),
	}
}
```

**Step 3: 修改 RegisterAllHandlers 函数签名**

```go
func RegisterAllHandlers(scheduler *Scheduler, dataManager *DataManager, templateCache *TemplateCache, htmlCache *HTMLCache, siteCache *SiteCache) {
	// Register data refresh handler
	if dataManager != nil {
		scheduler.RegisterHandler(NewRefreshDataHandler(dataManager))
	}
    // ... 其余不变
}
```

**Step 4: 验证编译**

Run: `cd E:\j\模板\seo_html_generator\api && go build ./...`
Expected: 编译成功

**Step 5: Commit**

```bash
git add api/cmd/main.go api/internal/handler/router.go api/internal/service/task_handlers.go api/internal/service/data_manager.go
git commit -m "refactor: replace DataPoolManager with unified DataManager

- Remove DataPoolManager from main.go
- Update Dependencies to use DataManager instead
- Update router handlers to use DataManager
- Update task_handlers to use DataManager
- DataManager now handles auto-refresh, statistics, and manual refresh

BREAKING CHANGE: DataPoolManager has been removed"
```

---

## Task 6: 删除废弃文件

**Files:**
- Delete: `api/internal/service/data_pool_manager.go`
- Delete: `api/internal/service/data_pool.go`

**Step 1: 确认没有其他引用**

Run: `cd E:\j\模板\seo_html_generator && grep -r "DataPoolManager\|DataPool" api/ --include="*.go" | grep -v "_test.go"`

Expected: 应该没有输出（或只有测试文件）

**Step 2: 删除文件**

```bash
rm api/internal/service/data_pool_manager.go
rm api/internal/service/data_pool.go
```

**Step 3: 验证编译**

Run: `cd E:\j\模板\seo_html_generator\api && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add -A
git commit -m "chore: remove deprecated DataPoolManager and DataPool

- Delete data_pool_manager.go (~450 lines)
- Delete data_pool.go (~140 lines)
- All functionality now consolidated in DataManager"
```

---

## Task 7: 验证功能

**Step 1: 启动服务**

Run: `cd E:\j\模板\seo_html_generator && docker-compose up -d`

**Step 2: 检查启动日志**

Run: `docker-compose logs -f api`

Expected: 应该看到类似日志：
```
DataManager auto refresh started
Keywords loaded - group_id=1, count=XXX
```

**Step 3: 测试刷新 API**

```bash
curl -X POST http://127.0.0.1:8009/api/data/refresh \
  -H "Content-Type: application/json" \
  -d '{"pool": "all"}'
```

Expected: 返回成功，包含统计信息

**Step 4: 测试页面渲染**

```bash
curl "http://127.0.0.1:8009/page?ua=Baiduspider&path=/1.html&domain=example.com"
```

Expected: 返回的 HTML 中应该包含关键词、图片、标题、正文内容

**Step 5: 验证数据统计**

```bash
curl http://127.0.0.1:8009/api/data/stats
```

Expected: 返回各分组的数据统计

---

## 回滚计划

如果出现问题，可以通过以下命令回滚：

```bash
git revert HEAD~3  # 回滚最近 3 个提交
```

或者检出之前的版本：

```bash
git checkout HEAD~3 -- api/
```

---

## 总结

| 步骤 | 描述 | 影响文件 |
|-----|------|---------|
| Task 1 | 添加自动刷新功能 | data_manager.go |
| Task 2 | 添加统计功能 | data_manager.go |
| Task 3 | 更新 main.go | main.go |
| Task 4 | 更新 router.go | router.go |
| Task 5 | 更新 task_handlers.go | task_handlers.go |
| Task 6 | 删除废弃文件 | data_pool_manager.go, data_pool.go |
| Task 7 | 验证功能 | - |

**预计删除代码：** ~590 行
**预计新增代码：** ~100 行
**净减少：** ~490 行
