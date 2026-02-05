# 图片热更新与分组支持实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 TemplateFuncsManager 支持图片热更新和多分组，解决页面渲染时图片 src 为空的问题。

**Architecture:** 使用原子指针替换实现无锁读取，添加时用追加模式，删除时异步重载。通过 RenderData 传递 imageGroupID 到模板渲染。

**Tech Stack:** Go 1.21+, atomic.Pointer, sync.Map

---

## Task 1: 重构 TemplateFuncsManager 图片数据结构

**Files:**
- Modify: `api/internal/service/template_funcs.go`

**Step 1: 添加新的数据结构**

在文件顶部 import 区域后添加：

```go
// ImageData 图片数据（不可变，通过原子指针替换）
type ImageData struct {
	groups map[int][]string // groupID -> urls
}
```

**Step 2: 修改 TemplateFuncsManager 结构体**

找到 `type TemplateFuncsManager struct`，删除旧的图片字段，添加新字段：

删除：
```go
imageURLs []string
imageLen  int64
imageIdx  int64
```

添加：
```go
// 图片数据（原子指针，支持无锁读取和热更新）
imageData atomic.Pointer[ImageData]

// 分组索引（独立管理，避免数据替换时重置）
imageGroupIdx sync.Map // groupID -> *atomic.Int64
```

**Step 3: 确保 import 包含必要的包**

```go
import (
	"sync"
	"sync/atomic"
	// ... 其他已有的 import
)
```

**Step 4: 编译验证**

Run: `go build ./...`
Expected: 编译失败（因为旧方法还在引用已删除的字段），这是预期的

**Step 5: Commit 结构变更**

```bash
git add api/internal/service/template_funcs.go
git commit -m "refactor(template_funcs): 重构图片数据结构支持多分组"
```

---

## Task 2: 实现 TemplateFuncsManager 图片相关方法

**Files:**
- Modify: `api/internal/service/template_funcs.go`

**Step 1: 修改 RandomImage 方法**

找到 `func (m *TemplateFuncsManager) RandomImage() string`，替换为：

```go
// RandomImage 获取随机图片URL（支持分组）
func (m *TemplateFuncsManager) RandomImage(groupID int) string {
	data := m.imageData.Load()
	if data == nil {
		return ""
	}

	urls := data.groups[groupID]
	if len(urls) == 0 {
		// 降级到默认分组
		urls = data.groups[1]
		if len(urls) == 0 {
			return ""
		}
	}

	// 获取或创建该分组的索引
	idxPtr, _ := m.imageGroupIdx.LoadOrStore(groupID, &atomic.Int64{})
	idx := idxPtr.(*atomic.Int64).Add(1) - 1

	return urls[idx%int64(len(urls))]
}
```

**Step 2: 删除旧的 LoadImageURLs 方法**

找到并删除整个方法：
```go
// LoadImageURLs 加载图片URL
func (m *TemplateFuncsManager) LoadImageURLs(urls []string) int {
	// ... 整个方法
}
```

**Step 3: 添加新的图片管理方法**

在文件末尾添加：

```go
// ============ 图片分组管理方法 ============

// LoadImageGroup 加载指定分组的图片（初始化时使用）
func (m *TemplateFuncsManager) LoadImageGroup(groupID int, urls []string) {
	for {
		old := m.imageData.Load()

		var newGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups)+1)
			for k, v := range old.groups {
				newGroups[k] = v
			}
		}

		// 复制 urls 避免外部修改
		copied := make([]string, len(urls))
		copy(copied, urls)
		newGroups[groupID] = copied

		newData := &ImageData{groups: newGroups}
		if m.imageData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// AppendImages 追加图片到指定分组（添加图片时使用）
func (m *TemplateFuncsManager) AppendImages(groupID int, urls []string) {
	if len(urls) == 0 {
		return
	}

	for {
		old := m.imageData.Load()

		var newGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups)+1)
			for k, v := range old.groups {
				newGroups[k] = v
			}
		}

		// 追加到目标分组（显式复制避免并发问题）
		oldUrls := newGroups[groupID]
		newUrls := make([]string, len(oldUrls)+len(urls))
		copy(newUrls, oldUrls)
		copy(newUrls[len(oldUrls):], urls)
		newGroups[groupID] = newUrls

		newData := &ImageData{groups: newGroups}
		if m.imageData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// ReloadImageGroup 重载指定分组（删除后异步调用）
func (m *TemplateFuncsManager) ReloadImageGroup(groupID int, urls []string) {
	for {
		old := m.imageData.Load()

		var newGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups))
			for k, v := range old.groups {
				newGroups[k] = v
			}
		}

		// 复制 urls 避免外部修改
		if len(urls) > 0 {
			copied := make([]string, len(urls))
			copy(copied, urls)
			newGroups[groupID] = copied
		} else {
			delete(newGroups, groupID)
		}

		newData := &ImageData{groups: newGroups}
		if m.imageData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// GetImageStats 获取图片统计信息
func (m *TemplateFuncsManager) GetImageStats() map[int]int {
	data := m.imageData.Load()
	if data == nil {
		return make(map[int]int)
	}

	stats := make(map[int]int, len(data.groups))
	for gid, urls := range data.groups {
		stats[gid] = len(urls)
	}
	return stats
}
```

**Step 4: 编译验证**

Run: `go build ./...`
Expected: 编译失败（调用处签名不匹配），预期的

**Step 5: Commit**

```bash
git add api/internal/service/template_funcs.go
git commit -m "feat(template_funcs): 实现图片热更新和分组管理方法"
```

---

## Task 3: 修改 RenderData 添加 ImageGroupID

**Files:**
- Modify: `api/internal/service/fast_renderer.go`

**Step 1: 找到 RenderData 结构体并添加字段**

找到 `type RenderData struct`，添加 `ImageGroupID` 字段：

```go
type RenderData struct {
	Title          string
	TitleGenerator func() string
	SiteID         int
	ImageGroupID   int // 新增：图片分组ID
	AnalyticsCode  template.HTML
	BaiduPushJS    template.HTML
	ArticleContent template.HTML
	Content        string
}
```

**Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译通过或失败（取决于其他文件），继续下一步

**Step 3: Commit**

```bash
git add api/internal/service/fast_renderer.go
git commit -m "feat(render_data): 添加 ImageGroupID 字段"
```

---

## Task 4: 修改 resolvePlaceholder 使用 ImageGroupID

**Files:**
- Modify: `api/internal/service/fast_renderer.go`

**Step 1: 修改 resolvePlaceholder 函数中的 PlaceholderImage 分支**

找到 `case PlaceholderImage:`，修改为：

```go
case PlaceholderImage:
	if data != nil {
		return fm.RandomImage(data.ImageGroupID)
	}
	return fm.RandomImage(1) // 默认分组
```

**Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译通过

**Step 3: Commit**

```bash
git add api/internal/service/fast_renderer.go
git commit -m "feat(fast_renderer): 渲染时使用站点配置的图片分组"
```

---

## Task 5: 修改 page.go 设置 ImageGroupID

**Files:**
- Modify: `api/internal/handler/page.go`

**Step 1: 添加获取 imageGroupID 的代码**

在 `page.go` 中找到获取 `keywordGroupID` 和 `articleGroupID` 的代码块附近（约 125-135 行），在其后添加：

```go
// Get image group ID
imageGroupID := 1
if site.ImageGroupID.Valid {
	imageGroupID = int(site.ImageGroupID.Int64)
}
```

**Step 2: 修改 renderData 初始化**

找到 `renderData := &core.RenderData{`，添加 `ImageGroupID` 字段：

```go
renderData := &core.RenderData{
	Title:          h.generateTitle(titleKeywords),
	TitleGenerator: titleGenerator,
	SiteID:         site.ID,
	ImageGroupID:   imageGroupID, // 新增
	AnalyticsCode:  template.HTML(analyticsCode),
	BaiduPushJS:    template.HTML(baiduPushJS),
	ArticleContent: template.HTML(articleContent),
}
```

**Step 3: 编译验证**

Run: `go build ./...`
Expected: 编译通过

**Step 4: Commit**

```bash
git add api/internal/handler/page.go
git commit -m "feat(page): 从站点配置读取图片分组ID"
```

---

## Task 6: 添加 PoolManager.GetImageGroupIDs 方法

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 添加新方法**

在 `pool_manager.go` 中 `GetImages` 方法附近添加：

```go
// GetImageGroupIDs 返回所有图片分组ID
func (m *PoolManager) GetImageGroupIDs() []int {
	groups := m.poolManager.GetImagePool().GetAllGroups()
	ids := make([]int, 0, len(groups))
	for gid := range groups {
		ids = append(ids, gid)
	}
	return ids
}
```

**Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译通过

**Step 3: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool_manager): 添加 GetImageGroupIDs 方法"
```

---

## Task 7: 修改 ImagesHandler 支持热更新

**Files:**
- Modify: `api/internal/handler/images.go`

**Step 1: 修改 ImagesHandler 结构体**

找到 `type ImagesHandler struct`，添加 `funcsManager` 字段：

```go
type ImagesHandler struct {
	db          *sqlx.DB
	repo        repository.ImageRepository
	groupRepo   repository.ImageGroupRepository
	poolManager *core.PoolManager
	funcsManager *core.TemplateFuncsManager // 新增
}
```

**Step 2: 修改构造函数**

找到 `func NewImagesHandler`，修改签名和初始化：

```go
func NewImagesHandler(db *sqlx.DB, poolManager *core.PoolManager, funcsManager *core.TemplateFuncsManager) *ImagesHandler {
	return &ImagesHandler{
		db:          db,
		repo:        repository.NewImageRepository(db),
		groupRepo:   repository.NewImageGroupRepository(db),
		poolManager: poolManager,
		funcsManager: funcsManager, // 新增
	}
}
```

**Step 3: 添加异步重载辅助方法**

在构造函数后添加：

```go
// asyncReloadImageGroup 异步重载图片分组到 TemplateFuncsManager
func (h *ImagesHandler) asyncReloadImageGroup(groupID int) {
	go func() {
		ctx := context.Background()
		// 1. 等待 PoolManager 重载完成
		h.poolManager.ReloadImageGroup(ctx, groupID)
		// 2. 获取最新数据
		urls := h.poolManager.GetImages(groupID)
		// 3. 同步到 TemplateFuncsManager
		h.funcsManager.ReloadImageGroup(groupID, urls)
	}()
}
```

**Step 4: 编译验证**

Run: `go build ./...`
Expected: 编译失败（router.go 调用处参数不匹配），预期的

**Step 5: Commit**

```bash
git add api/internal/handler/images.go
git commit -m "feat(images_handler): 添加 funcsManager 依赖和异步重载方法"
```

---

## Task 8: 修改 router.go 传入 funcsManager

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 修改 NewImagesHandler 调用**

找到 `imagesHandler := NewImagesHandler(deps.DB, deps.PoolManager)`，修改为：

```go
imagesHandler := NewImagesHandler(deps.DB, deps.PoolManager, deps.TemplateFuncs)
```

**Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译通过

**Step 3: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "feat(router): 传入 TemplateFuncs 到 ImagesHandler"
```

---

## Task 9: 在图片操作中添加同步调用

**Files:**
- Modify: `api/internal/handler/images.go`

**Step 1: 修改 AddURL 方法**

找到 `h.poolManager.AppendImages(groupID, []string{req.URL})` 这一行，在其后添加：

```go
h.funcsManager.AppendImages(groupID, []string{req.URL})
```

**Step 2: 修改 BatchAddURLs 方法**

找到 `h.poolManager.ReloadImageGroup(c.Request.Context(), groupID)` 这一行，替换为：

```go
h.asyncReloadImageGroup(groupID)
```

**Step 3: 修改 Upload 方法**

找到 `h.poolManager.ReloadImageGroup(c.Request.Context(), groupID)` 这一行，替换为：

```go
h.asyncReloadImageGroup(groupID)
```

**Step 4: 修改 DeleteURL 方法**

找到 `go h.poolManager.ReloadImageGroup(context.Background(), groupID)` 这一行，替换为：

```go
h.asyncReloadImageGroup(groupID)
```

**Step 5: 修改 DeleteGroup 方法**

找到 `go h.poolManager.ReloadImageGroup(context.Background(), id)` 这一行，替换为：

```go
h.asyncReloadImageGroup(id)
```

**Step 6: 修改 BatchDelete 方法**

找到循环中的 `go h.poolManager.ReloadImageGroup(context.Background(), gid)`，替换为：

```go
h.asyncReloadImageGroup(gid)
```

**Step 7: 修改 DeleteAll 方法**

找到 `go h.poolManager.ReloadImageGroup(context.Background(), *req.GroupID)`，替换为：

```go
h.asyncReloadImageGroup(*req.GroupID)
```

**Step 8: 修改 Reload 方法**

找到 `h.poolManager.ReloadImageGroup(c.Request.Context(), groupID)`，替换为：

```go
h.asyncReloadImageGroup(groupID)
```

**Step 9: 编译验证**

Run: `go build ./...`
Expected: 编译通过

**Step 10: Commit**

```bash
git add api/internal/handler/images.go
git commit -m "feat(images_handler): 图片操作时同步到 TemplateFuncsManager"
```

---

## Task 10: 修改 main.go 初始化逻辑

**Files:**
- Modify: `api/cmd/main.go`

**Step 1: 找到旧的图片初始化代码并替换**

找到这段代码：
```go
// Load image URLs into funcsManager
imageURLs := poolManager.GetImages(1)
if len(imageURLs) > 0 {
	funcsManager.LoadImageURLs(imageURLs)
	log.Info().Int("count", len(imageURLs)).Msg("Image URLs loaded to funcs manager")
}
```

替换为：
```go
// Load all image groups into funcsManager
imageGroupIDs := poolManager.GetImageGroupIDs()
totalImages := 0
for _, groupID := range imageGroupIDs {
	urls := poolManager.GetImages(groupID)
	if len(urls) > 0 {
		funcsManager.LoadImageGroup(groupID, urls)
		totalImages += len(urls)
		log.Info().Int("group_id", groupID).Int("count", len(urls)).
			Msg("Image group loaded to funcs manager")
	}
}
log.Info().Int("groups", len(imageGroupIDs)).Int("total_images", totalImages).
	Msg("All image groups loaded to funcs manager")
```

**Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译通过

**Step 3: Commit**

```bash
git add api/cmd/main.go
git commit -m "feat(main): 启动时加载所有图片分组到 TemplateFuncsManager"
```

---

## Task 11: 清理死代码（可选）

**Files:**
- Modify: `api/internal/service/template_renderer.go`

**Step 1: 删除未使用的 templateRenderContext**

找到并删除整个 `templateRenderContext` 结构体及其所有方法（约第 163-218 行）：

```go
// templateRenderContext is the context passed to templates
type templateRenderContext struct {
	// ... 整个结构体和所有方法
}
```

**Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译通过

**Step 3: Commit**

```bash
git add api/internal/service/template_renderer.go
git commit -m "refactor(template_renderer): 删除未使用的 templateRenderContext"
```

---

## Task 12: 最终验证

**Step 1: 完整编译**

Run: `go build ./...`
Expected: 编译成功，无错误

**Step 2: 运行测试（如果有）**

Run: `go test ./...`
Expected: 所有测试通过

**Step 3: 代码格式化**

Run: `go fmt ./...`

**Step 4: 最终 Commit**

```bash
git add -A
git commit -m "chore: 代码格式化和清理"
```

---

## 验收测试（手动）

1. 启动服务（Docker 或本地）
2. 访问管理后台，添加几张图片到分组 1
3. 访问 `http://127.0.0.1:8009/page?ua=Baiduspider&path=/16.html&domain=example.com`
4. 检查页面中的 `<img>` 标签 `src` 属性是否有值
5. 再添加几张图片，刷新页面，确认新图片能出现

---

## 回滚方案

如果出现问题，可以通过以下命令回滚：

```bash
git checkout main
git branch -D feature/image-hot-reload
git worktree remove .worktrees/image-hot-reload
```
