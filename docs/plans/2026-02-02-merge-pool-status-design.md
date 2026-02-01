# 合并运行状态到缓存管理 设计文档

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标**: 将 Settings 页面的池运行状态合并到 CacheManage 页面的"数据池"卡片中，实现统一的运行状态监控入口。

**架构**: 扩展后端 `GetDataPoolsStats()` 方法返回全部池数据，前端通过现有 WebSocket 自动获取，删除 Settings 页面的冗余状态显示。

---

## 改动范围

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `api/internal/service/pool_manager.go` | 修改 | 扩展 `GetDataPoolsStats()` 返回 5 个池 |
| `web/src/views/settings/Settings.vue` | 删除 | 移除运行状态区块及相关代码 |
| `web/src/views/cache/CacheManage.vue` | 微调 | 卡片标题 `数据` → `数据池` |

---

## 后端改动

### pool_manager.go - GetDataPoolsStats()

扩展返回数据，包含全部 5 个池：

```go
func (m *PoolManager) GetDataPoolsStats() []PoolStatusStats {
    m.mu.RLock()
    lastRefresh := m.lastRefresh
    stopped := m.stopped.Load()
    m.mu.RUnlock()

    status := "running"
    if stopped {
        status = "stopped"
    }

    var lastRefreshPtr *time.Time
    if !lastRefresh.IsZero() {
        lastRefreshPtr = &lastRefresh
    }

    pools := []PoolStatusStats{}

    // 1. 标题池（消费型，汇总所有分组）
    m.mu.RLock()
    var titlesMax, titlesCurrent int
    for _, pool := range m.titles {
        titlesMax += pool.GetMaxSize()
        titlesCurrent += pool.Len()
    }
    m.mu.RUnlock()

    titlesUsed := titlesMax - titlesCurrent
    titlesUtil := 0.0
    if titlesMax > 0 {
        titlesUtil = float64(titlesUsed) / float64(titlesMax) * 100
    }
    pools = append(pools, PoolStatusStats{
        Name:        "标题池",
        Size:        titlesMax,
        Available:   titlesCurrent,
        Used:        titlesUsed,
        Utilization: titlesUtil,
        Status:      status,
        NumWorkers:  1,
        LastRefresh: lastRefreshPtr,
    })

    // 2. 正文池（消费型，汇总所有分组）
    m.mu.RLock()
    var contentsMax, contentsCurrent int
    for _, pool := range m.contents {
        contentsMax += pool.GetMaxSize()
        contentsCurrent += pool.Len()
    }
    m.mu.RUnlock()

    contentsUsed := contentsMax - contentsCurrent
    contentsUtil := 0.0
    if contentsMax > 0 {
        contentsUtil = float64(contentsUsed) / float64(contentsMax) * 100
    }
    pools = append(pools, PoolStatusStats{
        Name:        "正文池",
        Size:        contentsMax,
        Available:   contentsCurrent,
        Used:        contentsUsed,
        Utilization: contentsUtil,
        Status:      status,
        NumWorkers:  1,
        LastRefresh: lastRefreshPtr,
    })

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
        Utilization: 0,
        Status:      status,
        NumWorkers:  1,
        LastRefresh: lastRefreshPtr,
    })

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
        Utilization: 0,
        Status:      status,
        NumWorkers:  1,
        LastRefresh: lastRefreshPtr,
    })

    // 5. 表情库（静态数据）
    emojiCount := m.emojiManager.Count()
    pools = append(pools, PoolStatusStats{
        Name:        "表情库",
        Size:        emojiCount,
        Available:   emojiCount,
        Used:        0,
        Utilization: 0,
        Status:      status,
        NumWorkers:  0,
        LastRefresh: nil,
    })

    return pools
}
```

**Utilization 语义**:
- 消费型池（标题/正文）: `used / size * 100`，表示消耗率
- 复用型池（关键词/图片/表情）: `0`，从不消耗

---

## 前端改动

### Settings.vue - 删除运行状态

**删除模板**（218-239行）:
```vue
<!-- 删除以下内容 -->
<el-divider content-position="left">运行状态</el-divider>
<el-descriptions :column="1" border v-if="cachePoolStats">
  ...
</el-descriptions>
<el-button @click="loadCachePoolStats" ...>刷新状态</el-button>
```

**删除变量**:
```typescript
// 删除
const cachePoolStatsLoading = ref(false)
const cachePoolStats = ref<CachePoolStats | null>(null)
```

**删除函数**:
```typescript
// 删除 loadCachePoolStats
// 删除 formatKeywordsImagesSummary
```

**删除调用**:
```typescript
// onMounted 中删除 loadCachePoolStats()
// handleSaveCachePool 中删除 loadCachePoolStats()
```

**删除 import**:
```typescript
// 删除 getCachePoolStats, formatPoolSummary, CachePoolStats
```

### CacheManage.vue - 调整标题

```vue
<!-- 修改前 -->
<span class="card-title">数据</span>

<!-- 修改后 -->
<span class="card-title">数据池</span>
```

---

## 测试计划

1. **编译验证**: `go build ./...`

2. **缓存管理页面**:
   - 确认"数据池"卡片显示 5 个池
   - 标题池/正文池进度条随消耗变化（颜色动态）
   - 关键词池/图片池/表情库显示绿色（0%）

3. **Settings 页面**:
   - 确认只显示配置表单
   - 保存配置功能正常
