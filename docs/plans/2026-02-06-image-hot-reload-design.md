# 图片热更新与分组支持设计

## 背景

### 问题描述

页面渲染时 `img` 标签的 `src` 值为空，但缓存管理页面能看到图片已加载。

### 根本原因

系统中存在两个独立的图片数据通道：

| 组件 | 用途 | 热更新 |
|------|------|--------|
| `PoolManager.ImagePool` | 缓存管理、API | ✅ 支持 |
| `TemplateFuncsManager.imageURLs` | 模板渲染 | ❌ 不支持 |

`TemplateFuncsManager` 只在服务启动时加载一次图片数据，后续通过管理后台添加的图片无法同步到模板渲染。

### 额外问题

- 当前只加载 `groupID=1` 的图片
- 站点配置的 `image_group_id` 字段未被使用
- 不同站点无法使用不同的图片分组

## 设计目标

1. 支持图片热更新：添加/删除图片后，模板渲染立即生效
2. 支持多分组：不同站点可使用不同的图片分组
3. 保持高性能：读取操作无锁，适合高并发渲染场景

## 技术方案

### 架构决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 数据源 | 保持 TemplateFuncsManager 独立缓存 | 无锁读取，性能最优 |
| 并发控制 | 原子指针替换 | 避免读写锁开销 |
| 添加操作 | 追加模式 | O(1) 性能 |
| 删除操作 | 异步全量重载 | 避免阻塞管理员操作 |
| 分组传递 | 通过 RenderData | 数据流清晰，每次渲染独立 |

### 数据结构

```go
// 图片数据（不可变，通过原子指针替换）
type ImageData struct {
    groups map[int][]string  // groupID -> urls
}

type TemplateFuncsManager struct {
    // ... 其他字段保持不变

    // 图片数据（原子指针，支持无锁读取）
    imageData atomic.Pointer[ImageData]

    // 分组索引（独立管理，避免数据替换时重置）
    groupIdx sync.Map  // groupID -> *atomic.Int64
}
```

**设计要点：**

- `ImageData` 是不可变的，更新时创建新实例并原子替换指针
- `groupIdx` 独立于数据存储，避免数据替换时索引重置
- 更新单个分组时，其他分组的 slice 共享引用，只复制 map 结构

### 核心方法

#### TemplateFuncsManager

```go
// RandomImage 获取随机图片（修改：增加 groupID 参数）
func (m *TemplateFuncsManager) RandomImage(groupID int) string {
    data := m.imageData.Load()
    if data == nil {
        return ""
    }

    urls := data.groups[groupID]
    if len(urls) == 0 {
        return ""
    }

    // 获取或创建该分组的索引
    idxPtr, _ := m.groupIdx.LoadOrStore(groupID, &atomic.Int64{})
    idx := idxPtr.(*atomic.Int64).Add(1) - 1

    return urls[idx % int64(len(urls))]
}

// LoadImageGroup 加载指定分组（初始化时使用）
func (m *TemplateFuncsManager) LoadImageGroup(groupID int, urls []string)

// AppendImages 追加图片到指定分组（添加图片时使用）
// 使用 CAS 循环确保并发安全，只复制 map 结构，slice 数据共享
func (m *TemplateFuncsManager) AppendImages(groupID int, urls []string)

// ReloadGroup 重载指定分组（删除后异步调用）
func (m *TemplateFuncsManager) ReloadGroup(groupID int, urls []string)
```

#### PoolManager

```go
// GetImageGroupIDs 返回所有图片分组ID（新增）
func (m *PoolManager) GetImageGroupIDs() []int {
    groups := m.poolManager.GetImagePool().GetAllGroups()
    ids := make([]int, 0, len(groups))
    for gid := range groups {
        ids = append(ids, gid)
    }
    return ids
}
```

### 触发机制

#### ImagesHandler 修改

```go
// 构造函数增加 funcsManager 参数
func NewImagesHandler(
    db *sqlx.DB,
    poolManager *core.PoolManager,
    funcsManager *core.TemplateFuncsManager,  // 新增
) *ImagesHandler

// 封装异步重载方法
func (h *ImagesHandler) asyncReloadGroup(groupID int) {
    go func() {
        ctx := context.Background()
        // 1. 等待 PoolManager 重载完成
        h.poolManager.ReloadImageGroup(ctx, groupID)
        // 2. 获取最新数据
        urls := h.poolManager.GetImages(groupID)
        // 3. 同步到 TemplateFuncsManager
        h.funcsManager.ReloadGroup(groupID, urls)
    }()
}
```

#### 各操作的同步方式

| 操作 | 方法 | 同步调用 |
|------|------|----------|
| AddURL | 添加单张 | `funcsManager.AppendImages(groupID, []string{url})` |
| BatchAddURLs | 批量添加 | `funcsManager.AppendImages(groupID, urls)` |
| Upload | 上传图片 | `asyncReloadGroup(groupID)` |
| DeleteURL | 删除单张 | `asyncReloadGroup(groupID)` |
| DeleteGroup | 删除分组 | `asyncReloadGroup(groupID)` |
| BatchDelete | 批量删除 | `asyncReloadGroup(groupID)` |
| DeleteAll | 删除全部 | `asyncReloadGroup(groupID)` |
| Reload | 手动重载 | `asyncReloadGroup(groupID)` |

### 渲染时传递 groupID

#### RenderData 修改

```go
type RenderData struct {
    Title          string
    TitleGenerator func() string
    SiteID         int
    ImageGroupID   int   // 新增
    AnalyticsCode  template.HTML
    BaiduPushJS    template.HTML
    ArticleContent template.HTML
}
```

#### page.go 修改

```go
// 获取图片分组 ID
imageGroupID := 1
if site.ImageGroupID.Valid {
    imageGroupID = int(site.ImageGroupID.Int64)
}

renderData := &core.RenderData{
    // ... 其他字段
    ImageGroupID: imageGroupID,  // 新增
}
```

#### fast_renderer.go 修改

```go
case PlaceholderImage:
    return fm.RandomImage(data.ImageGroupID)  // 修改：传入 groupID
```

### 启动时初始化

```go
// main.go 启动后
poolManager.Start(ctx)

// 初始化 TemplateFuncsManager 的图片数据
groupIDs := poolManager.GetImageGroupIDs()
for _, groupID := range groupIDs {
    urls := poolManager.GetImages(groupID)
    if len(urls) > 0 {
        funcsManager.LoadImageGroup(groupID, urls)
        log.Info().Int("group_id", groupID).Int("count", len(urls)).
            Msg("Image group loaded to funcs manager")
    }
}
```

## 修改文件清单

| 文件 | 修改内容 |
|------|----------|
| `api/internal/service/template_funcs.go` | 重构图片数据结构，新增/修改方法 |
| `api/internal/service/pool_manager.go` | 新增 `GetImageGroupIDs()` |
| `api/internal/service/render_data.go` | RenderData 增加 `ImageGroupID` |
| `api/internal/service/fast_renderer.go:112` | `RandomImage(data.ImageGroupID)` |
| `api/internal/handler/images.go` | 构造函数增加参数，各操作添加同步 |
| `api/internal/handler/router.go:128` | 传入 `deps.TemplateFuncs` |
| `api/internal/handler/page.go` | 读取 site.ImageGroupID |
| `api/cmd/main.go` | 多分组初始化逻辑 |
| `api/internal/service/template_renderer.go` | 删除死代码 `templateRenderContext`（可选） |

## 性能分析

### 读取性能（渲染时）

| 操作 | 复杂度 | 耗时 |
|------|--------|------|
| 原子加载指针 | O(1) | ~10ns |
| map 查找 | O(1) | ~10ns |
| sync.Map 读取 | O(1) | ~20ns |
| 原子增加索引 | O(1) | ~10ns |
| **总计** | O(1) | **~50ns** |

### 更新性能（管理操作）

| 操作 | 场景 | 耗时 |
|------|------|------|
| 追加图片 | 添加少量图片 | < 1μs |
| 重载分组 | 100万条数据 | 10-30ms（异步，不阻塞） |

### 内存占用

- 每个图片 URL 约 100 字节
- 100 万图片 ≈ 100MB
- 更新时短暂翻倍（新旧数据共存，旧数据很快被 GC 回收）

## 测试要点

1. **单元测试**
   - `RandomImage(groupID)` 正确返回对应分组的图片
   - `AppendImages` 后立即可读取到新图片
   - `ReloadGroup` 后数据正确更新
   - 并发读写不 panic

2. **集成测试**
   - 添加图片后，页面渲染能获取到新图片
   - 删除图片后，页面渲染不再返回已删除图片
   - 不同站点使用不同分组的图片

3. **性能测试**
   - 高并发渲染时无锁竞争
   - 100 万数据时更新性能可接受
