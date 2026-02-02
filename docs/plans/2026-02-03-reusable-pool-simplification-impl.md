# 复用型缓存池简化 实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将关键词池、图片池、表情库从"类生产消费模式"简化为"纯复用型缓存"，移除定时刷新和大小限制。

**Architecture:** 后端 PoolManager 移除 refreshLoop 和 LIMIT 限制，新增 Append/Reload 方法；Handler 层注入 PoolManager 依赖，在 CRUD 操作时触发缓存更新；前端根据 pool_type 字段区分显示消费型和复用型池。

**Tech Stack:** Go (Gin, sqlx), Vue 3 (TypeScript, Element Plus)

---

## Task 1: 扩展 PoolStatusStats 结构体

**Files:**
- Modify: `api/internal/service/pool_manager.go:67-77`

**Step 1: 添加 PoolGroupInfo 结构体**

在 `PoolStatusStats` 结构体之前添加：

```go
// PoolGroupInfo 分组详情
type PoolGroupInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}
```

**Step 2: 扩展 PoolStatusStats 结构体**

修改 `PoolStatusStats` 结构体，添加新字段：

```go
// PoolStatusStats 数据池运行状态统计（用于前端显示）
type PoolStatusStats struct {
	Name        string     `json:"name"`
	Size        int        `json:"size"`
	Available   int        `json:"available"`
	Used        int        `json:"used"`
	Utilization float64    `json:"utilization"`
	Status      string     `json:"status"`
	NumWorkers  int        `json:"num_workers"`
	LastRefresh *time.Time `json:"last_refresh"`
	// 新增字段（复用型池使用）
	PoolType string          `json:"pool_type"`         // "consumable" | "reusable" | "static"
	Groups   []PoolGroupInfo `json:"groups,omitempty"`  // 分组详情（复用型池）
	Source   string          `json:"source,omitempty"`  // 数据来源（表情库）
}
```

**Step 3: 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 4: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): 扩展 PoolStatusStats 结构体支持复用型池"
```

---

## Task 2: 移除定时刷新和大小限制

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 移除常量定义**

删除以下常量（约第 26-28 行）：

```go
// 删除这些行
defaultKeywordsSize     = 50000
defaultImagesSize       = 50000
defaultRefreshIntervalMs = 300000 // 5 minutes
```

**Step 2: 修改 Start 方法，移除 refreshLoop 启动**

在 `Start` 方法中（约第 146-149 行），将：

```go
m.wg.Add(3)
go m.refillLoop()
go m.updateWorker()
go m.refreshLoop(keywordGroupIDs, imageGroupIDs)
```

修改为：

```go
m.wg.Add(2)
go m.refillLoop()
go m.updateWorker()
// refreshLoop 已移除，复用型池不需要定时刷新
```

**Step 3: 删除 refreshLoop 方法**

删除整个 `refreshLoop` 方法（约第 637-666 行）。

**Step 4: 修改 LoadKeywords 方法，移除 LIMIT**

将 LoadKeywords 方法中的查询（约第 450 行）：

```go
query := `SELECT keyword FROM keywords WHERE group_id = ? AND status = 1 ORDER BY RAND() LIMIT ?`

var keywords []string
if err := m.db.SelectContext(ctx, &keywords, query, groupID, defaultKeywordsSize); err != nil {
```

修改为：

```go
query := `SELECT keyword FROM keywords WHERE group_id = ? AND status = 1`

var keywords []string
if err := m.db.SelectContext(ctx, &keywords, query, groupID); err != nil {
```

**Step 5: 修改 LoadImages 方法，移除 LIMIT**

将 LoadImages 方法中的查询（约第 536 行）：

```go
query := `SELECT url FROM images WHERE group_id = ? AND status = 1 ORDER BY RAND() LIMIT ?`

var urls []string
if err := m.db.SelectContext(ctx, &urls, query, groupID, defaultImagesSize); err != nil {
```

修改为：

```go
query := `SELECT url FROM images WHERE group_id = ? AND status = 1`

var urls []string
if err := m.db.SelectContext(ctx, &urls, query, groupID); err != nil {
```

**Step 6: 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 7: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "refactor(pool): 移除复用型池的定时刷新和大小限制"
```

---

## Task 3: 新增 Append 和 Reload 方法

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 添加 AppendKeywords 方法**

在 `GetRawKeywords` 方法后添加：

```go
// AppendKeywords 追加关键词到内存（新增时调用）
func (m *PoolManager) AppendKeywords(groupID int, keywords []string) {
	if len(keywords) == 0 {
		return
	}

	m.keywordsMu.Lock()
	defer m.keywordsMu.Unlock()

	if m.keywords[groupID] == nil {
		m.keywords[groupID] = []string{}
		m.rawKeywords[groupID] = []string{}
	}

	// 追加原始关键词
	m.rawKeywords[groupID] = append(m.rawKeywords[groupID], keywords...)

	// 追加编码后的关键词
	for _, kw := range keywords {
		m.keywords[groupID] = append(m.keywords[groupID], m.encoder.EncodeText(kw))
	}

	log.Debug().Int("group_id", groupID).Int("added", len(keywords)).Msg("Keywords appended to cache")
}
```

**Step 2: 添加 AppendImages 方法**

在 `GetImages` 方法后添加：

```go
// AppendImages 追加图片到内存（新增时调用）
func (m *PoolManager) AppendImages(groupID int, urls []string) {
	if len(urls) == 0 {
		return
	}

	m.imagesMu.Lock()
	defer m.imagesMu.Unlock()

	if m.images[groupID] == nil {
		m.images[groupID] = []string{}
	}
	m.images[groupID] = append(m.images[groupID], urls...)

	log.Debug().Int("group_id", groupID).Int("added", len(urls)).Msg("Images appended to cache")
}
```

**Step 3: 添加 ReloadKeywordGroup 方法**

```go
// ReloadKeywordGroup 重载指定分组的关键词缓存（删除时调用）
func (m *PoolManager) ReloadKeywordGroup(ctx context.Context, groupID int) error {
	_, err := m.LoadKeywords(ctx, groupID)
	if err != nil {
		log.Error().Err(err).Int("group_id", groupID).Msg("Failed to reload keyword group")
	}
	return err
}
```

**Step 4: 添加 ReloadImageGroup 方法**

```go
// ReloadImageGroup 重载指定分组的图片缓存（删除时调用）
func (m *PoolManager) ReloadImageGroup(ctx context.Context, groupID int) error {
	_, err := m.LoadImages(ctx, groupID)
	if err != nil {
		log.Error().Err(err).Int("group_id", groupID).Msg("Failed to reload image group")
	}
	return err
}
```

**Step 5: 添加 ReloadEmojis 方法**

```go
// ReloadEmojis 重载表情库
func (m *PoolManager) ReloadEmojis(path string) error {
	return m.emojiManager.LoadFromFile(path)
}
```

**Step 6: 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 7: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): 添加 Append 和 Reload 方法支持事件驱动更新"
```

---

## Task 4: 修改 GetDataPoolsStats 返回分组详情

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 修改关键词池统计部分**

在 `GetDataPoolsStats` 方法中，将关键词池部分（约第 738-754 行）：

```go
// 3. 关键词池（复用型，utilization = 0）
m.keywordsMu.RLock()
var totalKeywords int
for _, items := range m.keywords {
	totalKeywords += len(items)
}
m.keywordsMu.RUnlock()
pools = append(pools, PoolStatusStats{
	Name:        "关键词池",
	Size:        totalKeywords,
	Available:   totalKeywords,
	Used:        0,
	Utilization: 100,
	Status:      status,
	NumWorkers:  1,
	LastRefresh: lastRefreshPtr,
})
```

修改为：

```go
// 3. 关键词池（复用型，增加分组详情）
m.keywordsMu.RLock()
var totalKeywords int
keywordGroups := []PoolGroupInfo{}
for gid, items := range m.keywords {
	count := len(items)
	totalKeywords += count
	keywordGroups = append(keywordGroups, PoolGroupInfo{
		ID:    gid,
		Name:  fmt.Sprintf("分组%d", gid),
		Count: count,
	})
}
m.keywordsMu.RUnlock()
pools = append(pools, PoolStatusStats{
	Name:        "关键词池",
	Size:        totalKeywords,
	Available:   totalKeywords,
	Used:        0,
	Utilization: 100,
	Status:      status,
	NumWorkers:  0,
	LastRefresh: lastRefreshPtr,
	PoolType:    "reusable",
	Groups:      keywordGroups,
})
```

**Step 2: 修改图片池统计部分**

将图片池部分（约第 756-772 行）：

```go
// 4. 图片池（复用型，utilization = 0）
m.imagesMu.RLock()
var totalImages int
for _, items := range m.images {
	totalImages += len(items)
}
m.imagesMu.RUnlock()
pools = append(pools, PoolStatusStats{
	Name:        "图片池",
	Size:        totalImages,
	Available:   totalImages,
	Used:        0,
	Utilization: 100,
	Status:      status,
	NumWorkers:  1,
	LastRefresh: lastRefreshPtr,
})
```

修改为：

```go
// 4. 图片池（复用型，增加分组详情）
m.imagesMu.RLock()
var totalImages int
imageGroups := []PoolGroupInfo{}
for gid, items := range m.images {
	count := len(items)
	totalImages += count
	imageGroups = append(imageGroups, PoolGroupInfo{
		ID:    gid,
		Name:  fmt.Sprintf("分组%d", gid),
		Count: count,
	})
}
m.imagesMu.RUnlock()
pools = append(pools, PoolStatusStats{
	Name:        "图片池",
	Size:        totalImages,
	Available:   totalImages,
	Used:        0,
	Utilization: 100,
	Status:      status,
	NumWorkers:  0,
	LastRefresh: lastRefreshPtr,
	PoolType:    "reusable",
	Groups:      imageGroups,
})
```

**Step 3: 修改表情库统计部分**

将表情库部分（约第 774-786 行）：

```go
// 5. 表情库（静态数据）
emojiCount := m.emojiManager.Count()
pools = append(pools, PoolStatusStats{
	Name:        "表情库",
	Size:        emojiCount,
	Available:   emojiCount,
	Used:        0,
	Utilization: 100,
	Status:      status,
	NumWorkers:  0,
	LastRefresh: nil,
})
```

修改为：

```go
// 5. 表情库（静态数据）
emojiCount := m.emojiManager.Count()
pools = append(pools, PoolStatusStats{
	Name:        "表情库",
	Size:        emojiCount,
	Available:   emojiCount,
	Used:        0,
	Utilization: 100,
	Status:      status,
	NumWorkers:  0,
	LastRefresh: nil,
	PoolType:    "static",
	Source:      "emojis.json",
})
```

**Step 4: 为消费型池添加 PoolType**

修改标题池和正文池部分，添加 `PoolType: "consumable"`：

标题池（约第 702-711 行）添加：
```go
PoolType:    "consumable",
```

正文池（约第 727-736 行）添加：
```go
PoolType:    "consumable",
```

**Step 5: 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 6: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): GetDataPoolsStats 返回分组详情和池类型"
```

---

## Task 5: 修改 KeywordsHandler 注入 PoolManager

**Files:**
- Modify: `api/internal/handler/keywords.go`

**Step 1: 修改结构体定义**

将 KeywordsHandler 结构体（约第 19-22 行）：

```go
// KeywordsHandler 关键词管理 handler
type KeywordsHandler struct {
	db *sqlx.DB
}
```

修改为：

```go
// KeywordsHandler 关键词管理 handler
type KeywordsHandler struct {
	db          *sqlx.DB
	poolManager *core.PoolManager
}
```

**Step 2: 修改构造函数**

将 NewKeywordsHandler 函数（约第 24-27 行）：

```go
// NewKeywordsHandler 创建 KeywordsHandler
func NewKeywordsHandler(db *sqlx.DB) *KeywordsHandler {
	return &KeywordsHandler{db: db}
}
```

修改为：

```go
// NewKeywordsHandler 创建 KeywordsHandler
func NewKeywordsHandler(db *sqlx.DB, poolManager *core.PoolManager) *KeywordsHandler {
	return &KeywordsHandler{
		db:          db,
		poolManager: poolManager,
	}
}
```

**Step 3: 编译验证（预期失败）**

Run: `cd api && go build ./...`
Expected: 编译错误，router.go 调用参数不匹配

**Step 4: Commit（WIP）**

```bash
git add api/internal/handler/keywords.go
git commit -m "wip: KeywordsHandler 注入 PoolManager 依赖"
```

---

## Task 6: 修改 ImagesHandler 注入 PoolManager

**Files:**
- Modify: `api/internal/handler/images.go`

**Step 1: 修改结构体定义**

将 ImagesHandler 结构体（约第 18-21 行）：

```go
// ImagesHandler 图片管理 handler
type ImagesHandler struct {
	db *sqlx.DB
}
```

修改为：

```go
// ImagesHandler 图片管理 handler
type ImagesHandler struct {
	db          *sqlx.DB
	poolManager *core.PoolManager
}
```

**Step 2: 修改构造函数**

将 NewImagesHandler 函数（约第 23-26 行）：

```go
// NewImagesHandler 创建 ImagesHandler
func NewImagesHandler(db *sqlx.DB) *ImagesHandler {
	return &ImagesHandler{db: db}
}
```

修改为：

```go
// NewImagesHandler 创建 ImagesHandler
func NewImagesHandler(db *sqlx.DB, poolManager *core.PoolManager) *ImagesHandler {
	return &ImagesHandler{
		db:          db,
		poolManager: poolManager,
	}
}
```

**Step 3: Commit（WIP）**

```bash
git add api/internal/handler/images.go
git commit -m "wip: ImagesHandler 注入 PoolManager 依赖"
```

---

## Task 7: 修改 Router 初始化 Handler

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 修改 KeywordsHandler 初始化**

将（约第 96 行）：

```go
keywordsHandler := NewKeywordsHandler(deps.DB)
```

修改为：

```go
keywordsHandler := NewKeywordsHandler(deps.DB, deps.PoolManager)
```

**Step 2: 修改 ImagesHandler 初始化**

将（约第 127 行）：

```go
imagesHandler := NewImagesHandler(deps.DB)
```

修改为：

```go
imagesHandler := NewImagesHandler(deps.DB, deps.PoolManager)
```

**Step 3: 扩展 dataRefreshRequest 结构体**

将（约第 760-762 行）：

```go
type dataRefreshRequest struct {
	Pool string `json:"pool" binding:"required,oneof=all keywords images titles contents"`
}
```

修改为：

```go
type dataRefreshRequest struct {
	Pool    string `json:"pool" binding:"required,oneof=all keywords images titles contents emojis"`
	GroupID *int   `json:"group_id"`
}
```

**Step 4: 修改 dataRefreshHandler 支持分组和表情库**

将 dataRefreshHandler 函数（约第 764-793 行）修改为：

```go
// dataRefreshHandler POST /refresh - 刷新数据池
func dataRefreshHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.PoolManager == nil {
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

		switch req.Pool {
		case "keywords":
			if req.GroupID != nil {
				deps.PoolManager.ReloadKeywordGroup(ctx, *req.GroupID)
			} else {
				deps.PoolManager.RefreshData(ctx, "keywords")
			}
		case "images":
			if req.GroupID != nil {
				deps.PoolManager.ReloadImageGroup(ctx, *req.GroupID)
			} else {
				deps.PoolManager.RefreshData(ctx, "images")
			}
		case "emojis":
			deps.PoolManager.ReloadEmojis("data/emojis.json")
		default:
			if err := deps.PoolManager.RefreshData(ctx, req.Pool); err != nil {
				core.FailWithMessage(c, core.ErrInternalServer, err.Error())
				return
			}
		}

		stats := deps.PoolManager.GetPoolStatsSimple()
		core.Success(c, gin.H{
			"success": true,
			"pool":    req.Pool,
			"stats":   stats,
		})
	}
}
```

**Step 5: 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 6: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "feat(router): 更新 Handler 初始化和 dataRefreshHandler 支持分组"
```

---

## Task 8: KeywordsHandler 新增时追加缓存

**Files:**
- Modify: `api/internal/handler/keywords.go`

**Step 1: 修改 Add 方法**

在 Add 方法成功返回之前（约第 696 行之前），添加缓存追加逻辑：

```go
// 成功后追加到缓存
if h.poolManager != nil {
	h.poolManager.AppendKeywords(groupID, []string{req.Keyword})
}

id, _ := result.LastInsertId()
core.Success(c, gin.H{"success": true, "id": id})
```

**Step 2: 修改 BatchAdd 方法**

在 BatchAdd 方法中，需要收集成功添加的关键词。修改循环部分（约第 630-658 行）：

在循环前添加：
```go
addedKeywords := []string{}
```

在循环内，当 affected > 0 时，添加：
```go
addedKeywords = append(addedKeywords, kw)
```

在返回之前添加：
```go
// 成功后追加到缓存
if len(addedKeywords) > 0 && h.poolManager != nil {
	h.poolManager.AppendKeywords(groupID, addedKeywords)
}
```

**Step 3: 修改 Upload 方法**

类似 BatchAdd，在批量插入成功后追加缓存。在返回之前添加：

```go
// 成功后追加到缓存（需要重新加载该分组，因为批量插入难以追踪具体成功的）
if added > 0 && h.poolManager != nil {
	h.poolManager.ReloadKeywordGroup(c.Request.Context(), groupID)
}
```

**Step 4: 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 5: Commit**

```bash
git add api/internal/handler/keywords.go
git commit -m "feat(keywords): 新增关键词时追加到缓存"
```

---

## Task 9: KeywordsHandler 删除时重载缓存

**Files:**
- Modify: `api/internal/handler/keywords.go`

**Step 1: 修改 Delete 方法**

在删除前查询分组，删除后重载缓存。修改 Delete 方法（约第 422-441 行）：

```go
// Delete 删除关键词
// DELETE /api/keywords/:id
func (h *KeywordsHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的关键词 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 先查询要删除的关键词所属分组
	var groupID int
	h.db.Get(&groupID, "SELECT group_id FROM keywords WHERE id = ?", id)

	// 物理删除
	if _, err := h.db.Exec("DELETE FROM keywords WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 删除后重载分组缓存
	if groupID > 0 && h.poolManager != nil {
		go h.poolManager.ReloadKeywordGroup(context.Background(), groupID)
	}

	core.Success(c, gin.H{"success": true})
}
```

**Step 2: 修改 BatchDelete 方法**

在批量删除前查询涉及的分组，删除后重载。在 BatchDelete 方法中：

删除前添加：
```go
// 查询涉及的分组
var groupIDs []int
placeholders := strings.Repeat("?,", len(req.IDs))
placeholders = placeholders[:len(placeholders)-1]
args := make([]interface{}, len(req.IDs))
for i, id := range req.IDs {
	args[i] = id
}
h.db.Select(&groupIDs, fmt.Sprintf("SELECT DISTINCT group_id FROM keywords WHERE id IN (%s)", placeholders), args...)
```

删除后添加：
```go
// 重载涉及的分组缓存
if h.poolManager != nil {
	for _, gid := range groupIDs {
		go h.poolManager.ReloadKeywordGroup(context.Background(), gid)
	}
}
```

**Step 3: 修改 DeleteAll 方法**

在 DeleteAll 方法中，删除后重载分组：

```go
// 删除后重载缓存
if h.poolManager != nil {
	if req.GroupID != nil {
		go h.poolManager.ReloadKeywordGroup(context.Background(), *req.GroupID)
	} else {
		// 全部删除，需要重载所有分组
		go h.poolManager.RefreshData(context.Background(), "keywords")
	}
}
```

**Step 4: 修改 DeleteGroup 方法**

在删除分组后重载缓存（约第 306 行之前）：

```go
// 删除后重载缓存（分组已删除，清除该分组的缓存）
if h.poolManager != nil {
	go h.poolManager.ReloadKeywordGroup(context.Background(), id)
}
```

**Step 5: 修改 Reload 方法**

将 Reload 方法（约第 809-821 行）修改为实际调用 PoolManager：

```go
// Reload 重新加载关键词缓存
// POST /api/keywords/reload
func (h *KeywordsHandler) Reload(c *gin.Context) {
	groupIDStr := c.Query("group_id")

	if h.poolManager != nil {
		if groupIDStr != "" {
			groupID, _ := strconv.Atoi(groupIDStr)
			if groupID > 0 {
				h.poolManager.ReloadKeywordGroup(c.Request.Context(), groupID)
			}
		} else {
			h.poolManager.RefreshData(c.Request.Context(), "keywords")
		}
	}

	var total int64
	if h.db != nil {
		h.db.Get(&total, "SELECT COUNT(*) FROM keywords WHERE status = 1")
	}

	core.Success(c, gin.H{"success": true, "total": total})
}
```

**Step 6: 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 7: Commit**

```bash
git add api/internal/handler/keywords.go
git commit -m "feat(keywords): 删除关键词时重载缓存"
```

---

## Task 10: ImagesHandler 新增和删除时更新缓存

**Files:**
- Modify: `api/internal/handler/images.go`

**Step 1: 修改 AddURL 方法**

在成功返回之前添加缓存追加：

```go
// 成功后追加到缓存
if h.poolManager != nil {
	h.poolManager.AppendImages(groupID, []string{req.URL})
}
```

**Step 2: 修改 BatchAddURLs 方法**

在返回之前添加：

```go
// 成功后重载分组缓存
if added > 0 && h.poolManager != nil {
	h.poolManager.ReloadImageGroup(c.Request.Context(), groupID)
}
```

**Step 3: 修改 Upload 方法**

在返回之前添加：

```go
// 成功后重载分组缓存
if added > 0 && h.poolManager != nil {
	h.poolManager.ReloadImageGroup(c.Request.Context(), groupID)
}
```

**Step 4: 修改 DeleteURL 方法**

删除前查询分组，删除后重载：

```go
// 先查询要删除的图片所属分组
var groupID int
h.db.Get(&groupID, "SELECT group_id FROM images WHERE id = ?", id)

// ... 现有删除代码 ...

// 删除后重载分组缓存
if groupID > 0 && h.poolManager != nil {
	go h.poolManager.ReloadImageGroup(context.Background(), groupID)
}
```

**Step 5: 修改 BatchDelete 方法**

类似 KeywordsHandler，在删除前查询涉及的分组，删除后重载。

**Step 6: 修改 DeleteAll 方法**

删除后重载缓存：

```go
if h.poolManager != nil {
	if req.GroupID != nil {
		go h.poolManager.ReloadImageGroup(context.Background(), *req.GroupID)
	} else {
		go h.poolManager.RefreshData(context.Background(), "images")
	}
}
```

**Step 7: 修改 DeleteGroup 方法**

删除后重载缓存：

```go
if h.poolManager != nil {
	go h.poolManager.ReloadImageGroup(context.Background(), id)
}
```

**Step 8: 修改 Reload 方法**

```go
// Reload 重新加载图片缓存
// POST /api/images/urls/reload
func (h *ImagesHandler) Reload(c *gin.Context) {
	groupIDStr := c.Query("group_id")

	if h.poolManager != nil {
		if groupIDStr != "" {
			groupID, _ := strconv.Atoi(groupIDStr)
			if groupID > 0 {
				h.poolManager.ReloadImageGroup(c.Request.Context(), groupID)
			}
		} else {
			h.poolManager.RefreshData(c.Request.Context(), "images")
		}
	}

	var total int64
	if h.db != nil {
		h.db.Get(&total, "SELECT COUNT(*) FROM images WHERE status = 1")
	}

	core.Success(c, gin.H{"success": true, "total": total})
}
```

**Step 9: 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 10: Commit**

```bash
git add api/internal/handler/images.go
git commit -m "feat(images): 新增/删除图片时更新缓存"
```

---

## Task 11: 前端类型定义更新

**Files:**
- Modify: `web/src/api/pool-config.ts`

**Step 1: 添加 PoolGroupInfo 类型**

在文件中添加：

```typescript
/** 池分组信息 */
export interface PoolGroupInfo {
  id: number
  name: string
  count: number
}
```

**Step 2: 扩展 PoolStats 类型**

修改 PoolStats 接口，添加新字段：

```typescript
/** 池状态 */
export interface PoolStats {
  name: string
  size: number
  available: number
  used: number
  utilization: number
  status: string
  num_workers: number
  last_refresh: string | null
  // 新增字段
  pool_type?: 'consumable' | 'reusable' | 'static'
  groups?: PoolGroupInfo[]
  source?: string
}
```

**Step 3: 修改 refreshDataPool 函数支持分组**

```typescript
/** 刷新数据池 */
export function refreshDataPool(pool: string, groupId?: number): Promise<{ success: boolean }> {
  return request.post('/admin/data/refresh', {
    pool,
    group_id: groupId
  })
}
```

**Step 4: Commit**

```bash
git add web/src/api/pool-config.ts
git commit -m "feat(web): 更新池状态类型定义支持复用型池"
```

---

## Task 12: 前端 PoolStatusCard 组件适配

**Files:**
- Modify: `web/src/components/PoolStatusCard.vue`

**Step 1: 添加新的 props 和 emits**

```typescript
const props = defineProps<{
  pool: PoolStats
}>()

const emit = defineEmits<{
  (e: 'reload'): void
  (e: 'reload-group', groupId: number): void
}>()
```

**Step 2: 添加重载方法**

```typescript
const handleReload = () => {
  emit('reload')
}

const handleReloadGroup = (groupId: number) => {
  emit('reload-group', groupId)
}
```

**Step 3: 修改模板，根据 pool_type 显示不同内容**

将整个 template 替换为：

```vue
<template>
  <div class="pool-status-card">
    <div class="card-header">
      <span class="pool-name">{{ pool.name }}</span>
      <template v-if="pool.pool_type === 'reusable' || pool.pool_type === 'static'">
        <el-button size="small" @click="handleReload">
          重载
        </el-button>
      </template>
      <template v-else>
        <span :class="['status-badge', `status-${pool.status}`]">
          <span class="status-icon">{{ statusIcon }}</span>
          {{ statusText }}
        </span>
      </template>
    </div>

    <!-- 消费型池：显示利用率 -->
    <template v-if="!pool.pool_type || pool.pool_type === 'consumable'">
      <div class="progress-section">
        <el-progress
          :percentage="utilizationPercent"
          :color="progressColor"
          :stroke-width="12"
          :show-text="false"
        />
        <span class="progress-text">{{ utilizationPercent.toFixed(0) }}%</span>
      </div>

      <div class="stats-grid">
        <div class="stat-item">
          <span class="stat-label">容量</span>
          <span class="stat-value">{{ formatNumber(pool.size) }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">可用</span>
          <span class="stat-value">{{ formatNumber(pool.available) }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">已用</span>
          <span class="stat-value">{{ formatNumber(pool.used) }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">线程</span>
          <span class="stat-value">{{ pool.num_workers }}</span>
        </div>
      </div>
    </template>

    <!-- 复用型池：显示总数和分组 -->
    <template v-else-if="pool.pool_type === 'reusable'">
      <div class="reusable-stats">
        <span class="total">总计: {{ formatNumber(pool.size) }} 条</span>
        <span class="groups-count" v-if="pool.groups">({{ pool.groups.length }} 个分组)</span>
      </div>
      <el-collapse v-if="pool.groups && pool.groups.length > 0" class="groups-collapse">
        <el-collapse-item title="分组详情">
          <div class="groups-list">
            <div v-for="group in pool.groups" :key="group.id" class="group-item">
              <span class="group-name">{{ group.name }}</span>
              <span class="group-count">{{ formatNumber(group.count) }}</span>
              <el-button size="small" link @click="handleReloadGroup(group.id)">
                重载
              </el-button>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </template>

    <!-- 静态池：显示总数和来源 -->
    <template v-else-if="pool.pool_type === 'static'">
      <div class="static-stats">
        <div class="stat-row">
          <span class="stat-label">总计</span>
          <span class="stat-value">{{ formatNumber(pool.size) }} 个</span>
        </div>
        <div class="stat-row" v-if="pool.source">
          <span class="stat-label">来源</span>
          <span class="stat-value">{{ pool.source }}</span>
        </div>
      </div>
    </template>

    <div class="last-refresh" v-if="pool.last_refresh">
      最后加载: {{ formatTime(pool.last_refresh) }}
    </div>
  </div>
</template>
```

**Step 4: 添加新样式**

在 style 部分添加：

```scss
.reusable-stats {
  padding: 12px;
  background: #f5f7fa;
  border-radius: 6px;
  margin-bottom: 8px;

  .total {
    font-size: 16px;
    font-weight: 600;
    color: #303133;
  }

  .groups-count {
    margin-left: 8px;
    font-size: 14px;
    color: #909399;
  }
}

.groups-collapse {
  margin-bottom: 8px;

  :deep(.el-collapse-item__header) {
    font-size: 13px;
    color: #606266;
  }
}

.groups-list {
  .group-item {
    display: flex;
    align-items: center;
    padding: 8px 12px;
    background: #fff;
    border-radius: 4px;
    margin-bottom: 4px;

    &:last-child {
      margin-bottom: 0;
    }

    .group-name {
      flex: 1;
      font-size: 13px;
      color: #606266;
    }

    .group-count {
      margin-right: 12px;
      font-size: 13px;
      font-weight: 500;
      color: #303133;
    }
  }
}

.static-stats {
  .stat-row {
    display: flex;
    justify-content: space-between;
    padding: 8px 12px;
    background: #f5f7fa;
    border-radius: 4px;
    margin-bottom: 4px;

    &:last-child {
      margin-bottom: 0;
    }

    .stat-label {
      font-size: 13px;
      color: #909399;
    }

    .stat-value {
      font-size: 14px;
      font-weight: 500;
      color: #303133;
    }
  }
}
```

**Step 5: Commit**

```bash
git add web/src/components/PoolStatusCard.vue
git commit -m "feat(web): PoolStatusCard 适配复用型和静态型池显示"
```

---

## Task 13: 前端 CacheManage 页面适配

**Files:**
- Modify: `web/src/views/cache/CacheManage.vue`

**Step 1: 导入 refreshDataPool**

确保从 pool-config 导入 refreshDataPool：

```typescript
import {
  // ... 现有导入 ...
  refreshDataPool,
} from '@/api/pool-config'
```

**Step 2: 添加重载处理方法**

在 script setup 中添加：

```typescript
const handlePoolReload = async (poolName: string) => {
  poolOperationLoading.value = true
  try {
    const poolMap: Record<string, string> = {
      '关键词池': 'keywords',
      '图片池': 'images',
      '表情库': 'emojis'
    }
    const pool = poolMap[poolName]
    if (pool) {
      await refreshDataPool(pool)
      ElMessage.success(`${poolName}重载成功`)
    }
  } catch (e) {
    ElMessage.error((e as Error).message || '重载失败')
  } finally {
    poolOperationLoading.value = false
  }
}

const handlePoolReloadGroup = async (poolName: string, groupId: number) => {
  poolOperationLoading.value = true
  try {
    const poolMap: Record<string, string> = {
      '关键词池': 'keywords',
      '图片池': 'images'
    }
    const pool = poolMap[poolName]
    if (pool) {
      await refreshDataPool(pool, groupId)
      ElMessage.success(`${poolName}分组${groupId}重载成功`)
    }
  } catch (e) {
    ElMessage.error((e as Error).message || '重载失败')
  } finally {
    poolOperationLoading.value = false
  }
}
```

**Step 3: 修改 PoolStatusCard 调用，添加事件处理**

在模板中，修改 PoolStatusCard 的使用：

```vue
<PoolStatusCard
  v-for="pool in dataPoolStats"
  :key="pool.name"
  :pool="pool"
  @reload="handlePoolReload(pool.name)"
  @reload-group="(groupId) => handlePoolReloadGroup(pool.name, groupId)"
/>
```

**Step 4: Commit**

```bash
git add web/src/views/cache/CacheManage.vue
git commit -m "feat(web): CacheManage 页面支持复用型池重载操作"
```

---

## Task 14: 最终验证和清理

**Step 1: 后端编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 2: 前端编译验证**

Run: `cd web && npm run build`
Expected: 编译成功，无错误

**Step 3: 合并 WIP 提交**

```bash
git rebase -i HEAD~13
# 将 wip 提交 squash 到相应的 feat 提交
```

**Step 4: 最终提交**

```bash
git log --oneline -10
# 确认提交历史清晰
```

---

## 验收标准

1. ✅ 服务启动时全量加载关键词和图片（无 LIMIT 限制）
2. ✅ 移除定时刷新（refreshLoop 已删除）
3. ✅ 新增关键词/图片后，缓存自动追加
4. ✅ 删除关键词/图片后，对应分组缓存重载
5. ✅ 前端正确显示复用型池的分组详情
6. ✅ 手动重载功能正常工作（全部/分组）
7. ✅ 表情库重载功能正常工作
