# Keyword Grouping Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add keyword grouping support to TemplateFuncsManager, enabling different sites to use different keyword groups with hot-reload capability.

**Architecture:** Refactor TemplateFuncsManager to use atomic.Pointer[KeywordData] for lock-free reads, similar to the existing ImageData pattern. Remove the keywordEmojiPool and generate emoji-decorated keywords in real-time.

**Tech Stack:** Go 1.21+, sync/atomic, sync.Map

---

## Task 1: Add KeywordData struct and atomic pointer

**Files:**
- Modify: `api/internal/service/template_funcs.go:14-45`

**Step 1: Add KeywordData struct after ImageData**

Add this code after line 17 (after ImageData struct):

```go
// KeywordData 关键词数据（不可变，通过原子指针替换）
type KeywordData struct {
	groups    map[int][]string // groupID -> encoded keywords
	rawGroups map[int][]string // groupID -> raw keywords
}
```

**Step 2: Add new fields to TemplateFuncsManager**

Replace the old keyword fields (lines 27-35) with new atomic pointer and index maps. The struct should have:

```go
type TemplateFuncsManager struct {
	// 预生成池
	clsPool    *ObjectPool[string]
	urlPool    *ObjectPool[string]
	numberPool *NumberPool

	// 关键词数据（原子指针，支持无锁读取和热更新）
	keywordData atomic.Pointer[KeywordData]

	// 分组索引（独立管理，避免数据替换时重置）
	keywordGroupIdx    sync.Map // groupID -> *atomic.Int64
	rawKeywordGroupIdx sync.Map // groupID -> *atomic.Int64

	// 图片数据（原子指针，支持无锁读取和热更新）
	imageData atomic.Pointer[ImageData]

	// 分组索引（独立管理，避免数据替换时重置）
	imageGroupIdx sync.Map // groupID -> *atomic.Int64

	encoder      *HTMLEntityEncoder
	emojiManager *EmojiManager // emoji 管理器引用
}
```

**Step 3: Remove old keyword fields**

Delete these lines from the struct:
- `keywordEmojiPool *ObjectPool[string]`
- `keywords   []string`
- `keywordIdx int64`
- `keywordLen int64`
- `rawKeywords   []string`
- `rawKeywordIdx int64`
- `rawKeywordLen int64`

**Step 4: Build to verify syntax**

Run: `cd api && go build ./...`
Expected: Build errors (methods still reference old fields) - this is expected, we'll fix in next tasks

**Step 5: Commit**

```bash
git add api/internal/service/template_funcs.go
git commit -m "feat(keywords): add KeywordData struct and atomic pointer fields"
```

---

## Task 2: Refactor RandomKeyword to support groupID

**Files:**
- Modify: `api/internal/service/template_funcs.go:231-239`

**Step 1: Update RandomKeyword method signature and implementation**

Replace the existing RandomKeyword method:

```go
// RandomKeyword 获取随机关键词（支持分组）
func (m *TemplateFuncsManager) RandomKeyword(groupID int) string {
	data := m.keywordData.Load()
	if data == nil {
		return ""
	}

	keywords := data.groups[groupID]
	if len(keywords) == 0 {
		// 降级到默认分组
		keywords = data.groups[1]
		if len(keywords) == 0 {
			return ""
		}
	}

	// 获取或创建该分组的索引
	idxPtr, _ := m.keywordGroupIdx.LoadOrStore(groupID, &atomic.Int64{})
	idx := idxPtr.(*atomic.Int64).Add(1) - 1

	return keywords[idx%int64(len(keywords))]
}
```

**Step 2: Build to check (will have errors)**

Run: `cd api && go build ./... 2>&1 | head -20`
Expected: Build errors from callers - expected

**Step 3: Commit**

```bash
git add api/internal/service/template_funcs.go
git commit -m "feat(keywords): refactor RandomKeyword to support groupID parameter"
```

---

## Task 3: Refactor RandomKeywordEmoji to support groupID with real-time generation

**Files:**
- Modify: `api/internal/service/template_funcs.go:241-248`

**Step 1: Update RandomKeywordEmoji to use real-time generation**

Replace the existing RandomKeywordEmoji method:

```go
// RandomKeywordEmoji 获取带 emoji 的随机关键词（支持分组，实时生成）
func (m *TemplateFuncsManager) RandomKeywordEmoji(groupID int) string {
	data := m.keywordData.Load()
	if data == nil {
		return ""
	}

	rawKeywords := data.rawGroups[groupID]
	if len(rawKeywords) == 0 {
		// 降级到默认分组
		rawKeywords = data.rawGroups[1]
		if len(rawKeywords) == 0 {
			return ""
		}
	}

	// 获取或创建该分组的索引
	idxPtr, _ := m.rawKeywordGroupIdx.LoadOrStore(groupID, &atomic.Int64{})
	idx := idxPtr.(*atomic.Int64).Add(1) - 1

	keyword := rawKeywords[idx%int64(len(rawKeywords))]
	return m.generateKeywordWithEmojiFromRaw(keyword)
}

// generateKeywordWithEmojiFromRaw 从原始关键词生成带 emoji 的版本
func (m *TemplateFuncsManager) generateKeywordWithEmojiFromRaw(keyword string) string {
	// 如果 emojiManager 为 nil，直接返回编码后的关键词
	if m.emojiManager == nil {
		return m.encoder.EncodeText(keyword)
	}

	// 随机决定插入 1 或 2 个 emoji（50% 概率）
	emojiCount := 1
	if rand.Float64() < 0.5 {
		emojiCount = 2
	}

	// 转换为 rune 切片处理中文
	runes := []rune(keyword)
	runeLen := len(runes)
	if runeLen == 0 {
		return m.encoder.EncodeText(keyword)
	}

	// 插入 emoji
	exclude := make(map[string]bool)
	for i := 0; i < emojiCount; i++ {
		pos := rand.IntN(runeLen + 1) // 0 到 len，包含首尾
		emoji := m.emojiManager.GetRandomExclude(exclude)
		if emoji != "" {
			exclude[emoji] = true
			// 在位置插入
			newRunes := make([]rune, 0, len(runes)+len([]rune(emoji)))
			newRunes = append(newRunes, runes[:pos]...)
			newRunes = append(newRunes, []rune(emoji)...)
			newRunes = append(newRunes, runes[pos:]...)
			runes = newRunes
			runeLen = len(runes)
		}
	}

	// 编码并返回
	return m.encoder.EncodeText(string(runes))
}
```

**Step 2: Commit**

```bash
git add api/internal/service/template_funcs.go
git commit -m "feat(keywords): refactor RandomKeywordEmoji with real-time generation"
```

---

## Task 4: Add keyword group management methods

**Files:**
- Modify: `api/internal/service/template_funcs.go` (add after image group methods, around line 580)

**Step 1: Add LoadKeywordGroup method**

```go
// ============ 关键词分组管理方法 ============

// LoadKeywordGroup 加载指定分组的关键词（初始化时使用）
func (m *TemplateFuncsManager) LoadKeywordGroup(groupID int, keywords, rawKeywords []string) {
	for {
		old := m.keywordData.Load()

		var newGroups map[int][]string
		var newRawGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
			newRawGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups)+1)
			newRawGroups = make(map[int][]string, len(old.rawGroups)+1)
			for k, v := range old.groups {
				newGroups[k] = v
			}
			for k, v := range old.rawGroups {
				newRawGroups[k] = v
			}
		}

		// 复制数据避免外部修改
		copiedKeywords := make([]string, len(keywords))
		copy(copiedKeywords, keywords)
		newGroups[groupID] = copiedKeywords

		copiedRaw := make([]string, len(rawKeywords))
		copy(copiedRaw, rawKeywords)
		newRawGroups[groupID] = copiedRaw

		newData := &KeywordData{groups: newGroups, rawGroups: newRawGroups}
		if m.keywordData.CompareAndSwap(old, newData) {
			return
		}
	}
}
```

**Step 2: Add AppendKeywords method**

```go
// AppendKeywords 追加关键词到指定分组（添加关键词时使用）
func (m *TemplateFuncsManager) AppendKeywords(groupID int, keywords, rawKeywords []string) {
	if len(keywords) == 0 {
		return
	}

	for {
		old := m.keywordData.Load()

		var newGroups map[int][]string
		var newRawGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
			newRawGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups)+1)
			newRawGroups = make(map[int][]string, len(old.rawGroups)+1)
			for k, v := range old.groups {
				newGroups[k] = v
			}
			for k, v := range old.rawGroups {
				newRawGroups[k] = v
			}
		}

		// 追加到目标分组（显式复制避免并发问题）
		oldKeywords := newGroups[groupID]
		newKeywords := make([]string, len(oldKeywords)+len(keywords))
		copy(newKeywords, oldKeywords)
		copy(newKeywords[len(oldKeywords):], keywords)
		newGroups[groupID] = newKeywords

		oldRaw := newRawGroups[groupID]
		newRaw := make([]string, len(oldRaw)+len(rawKeywords))
		copy(newRaw, oldRaw)
		copy(newRaw[len(oldRaw):], rawKeywords)
		newRawGroups[groupID] = newRaw

		newData := &KeywordData{groups: newGroups, rawGroups: newRawGroups}
		if m.keywordData.CompareAndSwap(old, newData) {
			return
		}
	}
}
```

**Step 3: Add ReloadKeywordGroup method**

```go
// ReloadKeywordGroup 重载指定分组（删除后异步调用）
func (m *TemplateFuncsManager) ReloadKeywordGroup(groupID int, keywords, rawKeywords []string) {
	for {
		old := m.keywordData.Load()

		var newGroups map[int][]string
		var newRawGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
			newRawGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups))
			newRawGroups = make(map[int][]string, len(old.rawGroups))
			for k, v := range old.groups {
				newGroups[k] = v
			}
			for k, v := range old.rawGroups {
				newRawGroups[k] = v
			}
		}

		// 替换或删除分组
		if len(keywords) > 0 {
			copiedKeywords := make([]string, len(keywords))
			copy(copiedKeywords, keywords)
			newGroups[groupID] = copiedKeywords

			copiedRaw := make([]string, len(rawKeywords))
			copy(copiedRaw, rawKeywords)
			newRawGroups[groupID] = copiedRaw
		} else {
			delete(newGroups, groupID)
			delete(newRawGroups, groupID)
		}

		newData := &KeywordData{groups: newGroups, rawGroups: newRawGroups}
		if m.keywordData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// GetKeywordStats 获取关键词统计信息
func (m *TemplateFuncsManager) GetKeywordStats() map[int]int {
	data := m.keywordData.Load()
	if data == nil {
		return make(map[int]int)
	}

	stats := make(map[int]int, len(data.groups))
	for gid, keywords := range data.groups {
		stats[gid] = len(keywords)
	}
	return stats
}
```

**Step 4: Commit**

```bash
git add api/internal/service/template_funcs.go
git commit -m "feat(keywords): add keyword group management methods"
```

---

## Task 5: Remove old keyword-related code

**Files:**
- Modify: `api/internal/service/template_funcs.go`

**Step 1: Remove InitKeywordEmojiPool method** (lines 98-114)

Delete the entire `InitKeywordEmojiPool` function.

**Step 2: Remove old generateKeywordWithEmoji method** (lines 116-163)

Delete the old `generateKeywordWithEmoji` function (the new one is `generateKeywordWithEmojiFromRaw`).

**Step 3: Remove old LoadKeywords method** (lines 181-209)

Delete the entire `LoadKeywords` function.

**Step 4: Update StopPools to remove keywordEmojiPool reference** (lines 165-179)

Remove these lines from StopPools:
```go
if m.keywordEmojiPool != nil {
    m.keywordEmojiPool.Stop()
}
```

**Step 5: Update ReloadPools to remove keywordEmojiPool reference**

Remove keywordEmojiPool handling from ReloadPools method.

**Step 6: Update ClearPools to remove keywordEmojiPool reference**

Remove keywordEmojiPool handling from ClearPools method.

**Step 7: Update GetStats to use new keyword stats**

Replace old keyword stats with:
```go
"keywords_count": m.GetKeywordStats(),
```

Remove references to `keywordEmojiPool` stats.

**Step 8: Update GetPoolStats to remove keywordEmojiPool**

Remove keywordEmojiPool from GetPoolStats method.

**Step 9: Build to verify**

Run: `cd api && go build ./...`
Expected: Build errors from callers - expected at this stage

**Step 10: Commit**

```bash
git add api/internal/service/template_funcs.go
git commit -m "refactor(keywords): remove old keyword pool and methods"
```

---

## Task 6: Add KeywordGroupID to RenderData and update fast_renderer

**Files:**
- Modify: `api/internal/service/fast_renderer.go:107-110`

**Step 1: Add KeywordGroupID field to RenderData struct**

Find the RenderData struct (around line 195 in fast_renderer.go or render_data.go) and add:

```go
KeywordGroupID int // 关键词分组ID
```

**Step 2: Update resolvePlaceholder for PlaceholderKeyword**

Replace lines 107-110:

```go
case PlaceholderKeyword:
	if data != nil {
		return fm.RandomKeyword(data.KeywordGroupID)
	}
	return fm.RandomKeyword(1)
case PlaceholderKeywordEmoji:
	if data != nil {
		return fm.RandomKeywordEmoji(data.KeywordGroupID)
	}
	return fm.RandomKeywordEmoji(1)
```

**Step 3: Build to verify**

Run: `cd api && go build ./...`
Expected: Build errors from other files - expected

**Step 4: Commit**

```bash
git add api/internal/service/fast_renderer.go
git commit -m "feat(render): add KeywordGroupID to RenderData and update resolvePlaceholder"
```

---

## Task 7: Add PoolManager methods for keyword data access

**Files:**
- Modify: `api/internal/service/pool_manager.go` (around line 470)

**Step 1: Add GetKeywordGroupIDs method**

```go
// GetKeywordGroupIDs 返回所有关键词分组ID
func (m *PoolManager) GetKeywordGroupIDs() []int {
	groups := m.poolManager.GetKeywordPool().GetAllGroups()
	ids := make([]int, 0, len(groups))
	for gid := range groups {
		ids = append(ids, gid)
	}
	return ids
}
```

**Step 2: Add GetKeywords method**

```go
// GetKeywords 获取指定分组的所有编码关键词
func (m *PoolManager) GetKeywords(groupID int) []string {
	return m.poolManager.GetKeywordPool().GetKeywords(groupID)
}

// GetAllRawKeywords 获取指定分组的所有原始关键词
func (m *PoolManager) GetAllRawKeywords(groupID int) []string {
	return m.poolManager.GetKeywordPool().GetAllRawKeywords(groupID)
}
```

**Step 3: Build (will fail - KeywordPool methods don't exist yet)**

Run: `cd api && go build ./... 2>&1 | head -10`
Expected: Build errors - GetKeywords/GetAllRawKeywords undefined

**Step 4: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): add keyword data access methods to PoolManager"
```

---

## Task 8: Add KeywordPool methods for full data access

**Files:**
- Modify: `api/internal/service/pool/keyword_pool.go` (after GetRawKeywords method, around line 247)

**Step 1: Add GetKeywords method**

```go
// GetKeywords 返回指定分组的所有编码关键词
func (p *KeywordPool) GetKeywords(groupID int) []string {
	p.mu.RLock()
	items := p.data[groupID]
	if len(items) == 0 {
		items = p.data[1] // fallback to default group
	}
	// 复制避免外部修改
	result := make([]string, len(items))
	copy(result, items)
	p.mu.RUnlock()
	return result
}

// GetAllRawKeywords 返回指定分组的所有原始关键词
func (p *KeywordPool) GetAllRawKeywords(groupID int) []string {
	p.mu.RLock()
	items := p.rawData[groupID]
	if len(items) == 0 {
		items = p.rawData[1] // fallback to default group
	}
	// 复制避免外部修改
	result := make([]string, len(items))
	copy(result, items)
	p.mu.RUnlock()
	return result
}
```

**Step 2: Build to verify**

Run: `cd api && go build ./...`
Expected: PASS or minimal errors

**Step 3: Commit**

```bash
git add api/internal/service/pool/keyword_pool.go
git commit -m "feat(pool): add GetKeywords and GetAllRawKeywords to KeywordPool"
```

---

## Task 9: Update KeywordsHandler to sync with TemplateFuncsManager

**Files:**
- Modify: `api/internal/handler/keywords.go`

**Step 1: Add funcsManager field to KeywordsHandler**

Update the struct (lines 20-24):

```go
// KeywordsHandler 关键词管理 handler
type KeywordsHandler struct {
	db           *sqlx.DB
	poolManager  *core.PoolManager
	funcsManager *core.TemplateFuncsManager
}
```

**Step 2: Update NewKeywordsHandler constructor**

```go
// NewKeywordsHandler 创建 KeywordsHandler
func NewKeywordsHandler(db *sqlx.DB, poolManager *core.PoolManager, funcsManager *core.TemplateFuncsManager) *KeywordsHandler {
	return &KeywordsHandler{
		db:           db,
		poolManager:  poolManager,
		funcsManager: funcsManager,
	}
}
```

**Step 3: Add asyncReloadKeywordGroup helper method**

Add after the constructor:

```go
// asyncReloadKeywordGroup 异步重载关键词分组到 TemplateFuncsManager
func (h *KeywordsHandler) asyncReloadKeywordGroup(groupID int) {
	go func() {
		ctx := context.Background()
		// 1. 等待 PoolManager 重载完成
		h.poolManager.ReloadKeywordGroup(ctx, groupID)
		// 2. 获取最新数据
		keywords := h.poolManager.GetKeywords(groupID)
		rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
		// 3. 同步到 TemplateFuncsManager
		if h.funcsManager != nil {
			h.funcsManager.ReloadKeywordGroup(groupID, keywords, rawKeywords)
		}
	}()
}
```

**Step 4: Update DeleteGroup to use asyncReloadKeywordGroup** (around line 314)

Replace:
```go
go h.poolManager.ReloadKeywordGroup(context.Background(), id)
```
With:
```go
h.asyncReloadKeywordGroup(id)
```

**Step 5: Update Delete to use asyncReloadKeywordGroup** (around line 456)

Replace:
```go
go h.poolManager.ReloadKeywordGroup(context.Background(), groupID)
```
With:
```go
h.asyncReloadKeywordGroup(groupID)
```

**Step 6: Update BatchDelete to use asyncReloadKeywordGroup** (around line 505)

Replace the loop:
```go
for _, gid := range groupIDs {
    go h.poolManager.ReloadKeywordGroup(context.Background(), gid)
}
```
With:
```go
for _, gid := range groupIDs {
    h.asyncReloadKeywordGroup(gid)
}
```

**Step 7: Update DeleteAll to use asyncReloadKeywordGroup** (around line 550)

Replace:
```go
go h.poolManager.ReloadKeywordGroup(context.Background(), *req.GroupID)
```
With:
```go
h.asyncReloadKeywordGroup(*req.GroupID)
```

For the "all" case, add after RefreshData:
```go
// 全部删除后，需要重载所有分组到 TemplateFuncsManager
if h.funcsManager != nil {
    groupIDs := h.poolManager.GetKeywordGroupIDs()
    for _, gid := range groupIDs {
        keywords := h.poolManager.GetKeywords(gid)
        rawKeywords := h.poolManager.GetAllRawKeywords(gid)
        h.funcsManager.ReloadKeywordGroup(gid, keywords, rawKeywords)
    }
}
```

**Step 8: Update BatchAdd to sync with TemplateFuncsManager** (around line 699)

After `h.poolManager.AppendKeywords(groupID, addedKeywords)`, add:
```go
// 同步到 TemplateFuncsManager（编码关键词需要重新获取）
if h.funcsManager != nil {
    encodedKeywords := h.poolManager.GetKeywords(groupID)
    rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
    h.funcsManager.ReloadKeywordGroup(groupID, encodedKeywords, rawKeywords)
}
```

**Step 9: Update Add to sync with TemplateFuncsManager** (around line 749)

After `h.poolManager.AppendKeywords(groupID, []string{req.Keyword})`, add:
```go
// 同步到 TemplateFuncsManager
if h.funcsManager != nil {
    encodedKeywords := h.poolManager.GetKeywords(groupID)
    rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
    h.funcsManager.ReloadKeywordGroup(groupID, encodedKeywords, rawKeywords)
}
```

**Step 10: Update Upload to sync with TemplateFuncsManager** (around line 858)

After `h.poolManager.ReloadKeywordGroup(c.Request.Context(), groupID)`, add:
```go
// 同步到 TemplateFuncsManager
if h.funcsManager != nil {
    keywords := h.poolManager.GetKeywords(groupID)
    rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
    h.funcsManager.ReloadKeywordGroup(groupID, keywords, rawKeywords)
}
```

**Step 11: Update Reload to sync with TemplateFuncsManager** (around line 883)

After the poolManager reload calls, add sync to TemplateFuncsManager:
```go
// 同步到 TemplateFuncsManager
if h.funcsManager != nil {
    if groupIDStr != "" {
        groupID, _ := strconv.Atoi(groupIDStr)
        if groupID > 0 {
            keywords := h.poolManager.GetKeywords(groupID)
            rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
            h.funcsManager.ReloadKeywordGroup(groupID, keywords, rawKeywords)
        }
    } else {
        // 重载所有分组
        groupIDs := h.poolManager.GetKeywordGroupIDs()
        for _, gid := range groupIDs {
            keywords := h.poolManager.GetKeywords(gid)
            rawKeywords := h.poolManager.GetAllRawKeywords(gid)
            h.funcsManager.ReloadKeywordGroup(gid, keywords, rawKeywords)
        }
    }
}
```

**Step 12: Build to verify**

Run: `cd api && go build ./...`
Expected: Build error - router.go needs update

**Step 13: Commit**

```bash
git add api/internal/handler/keywords.go
git commit -m "feat(handler): update KeywordsHandler to sync with TemplateFuncsManager"
```

---

## Task 10: Update router.go to pass TemplateFuncs to KeywordsHandler

**Files:**
- Modify: `api/internal/handler/router.go:97`

**Step 1: Update NewKeywordsHandler call**

Change line 97:
```go
keywordsHandler := NewKeywordsHandler(deps.DB, deps.PoolManager)
```
To:
```go
keywordsHandler := NewKeywordsHandler(deps.DB, deps.PoolManager, deps.TemplateFuncs)
```

**Step 2: Build to verify**

Run: `cd api && go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "feat(router): pass TemplateFuncs to KeywordsHandler"
```

---

## Task 11: Update page.go to set KeywordGroupID in RenderData

**Files:**
- Modify: `api/internal/handler/page.go`

**Step 1: Find where RenderData is created and add KeywordGroupID**

The page.go file should already read `keywordGroupID` from site config. Find the RenderData creation and add:

```go
KeywordGroupID: keywordGroupID,
```

**Step 2: Build to verify**

Run: `cd api && go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add api/internal/handler/page.go
git commit -m "feat(page): add KeywordGroupID to RenderData"
```

---

## Task 12: Update main.go to initialize keyword groups

**Files:**
- Modify: `api/cmd/main.go`

**Step 1: Find the image group initialization section and add keyword initialization after it**

After the image group initialization loop, add:

```go
// 初始化 TemplateFuncsManager 的关键词数据
keywordGroupIDs := poolManager.GetKeywordGroupIDs()
for _, groupID := range keywordGroupIDs {
    keywords := poolManager.GetKeywords(groupID)
    rawKeywords := poolManager.GetAllRawKeywords(groupID)
    if len(keywords) > 0 {
        funcsManager.LoadKeywordGroup(groupID, keywords, rawKeywords)
        log.Info().Int("group_id", groupID).Int("count", len(keywords)).
            Msg("Keyword group loaded to funcs manager")
    }
}
```

**Step 2: Build to verify**

Run: `cd api && go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add api/cmd/main.go
git commit -m "feat(main): initialize keyword groups in TemplateFuncsManager"
```

---

## Task 13: Final build and test

**Step 1: Full build**

Run: `cd api && go build ./...`
Expected: PASS

**Step 2: Run tests if any**

Run: `cd api && go test ./... 2>&1 | head -30`

**Step 3: Commit any remaining fixes**

If there are any compilation errors, fix them and commit.

---

## Summary

This implementation adds keyword grouping support with the following features:

1. **KeywordData struct** with atomic pointer for lock-free reads
2. **RandomKeyword(groupID)** and **RandomKeywordEmoji(groupID)** methods
3. **Group management methods**: LoadKeywordGroup, AppendKeywords, ReloadKeywordGroup
4. **PoolManager methods** for data access: GetKeywordGroupIDs, GetKeywords, GetAllRawKeywords
5. **KeywordsHandler** syncs all operations to TemplateFuncsManager
6. **RenderData** includes KeywordGroupID for per-request grouping
7. **Startup initialization** loads all keyword groups

The architecture mirrors the existing image grouping implementation for consistency.
