# 运行状态页面展示站点缓存和模板缓存

## 背景

运行状态页面的 HTML 缓存栏内容较少，有空间容纳更多缓存信息。站点配置缓存和模板缓存目前无法在页面上查看状态。

## 设计

### 后端

#### 1. `site_cache.go` — `GetStats()` 增加内存计算

在 `GetStats()` 中遍历 cache 计算内存，无需增量追踪（数据量小，遍历开销可忽略）：

```go
func (sc *SiteCache) GetStats() map[string]interface{} {
    count := 0
    var memoryBytes int64
    sc.cache.Range(func(key, value interface{}) bool {
        count++
        if site, ok := value.(*models.Site); ok && site != nil {
            memoryBytes += siteMemorySize(site)
        }
        return true
    })
    return map[string]interface{}{
        "item_count":   count,
        "memory_bytes": memoryBytes,
    }
}
```

`siteMemorySize` 计算：结构体固定开销 + 各字符串字段 `len()`。

#### 2. `template_cache.go` — 同样模式

`templateMemorySize` 重点计算 `len(Content)`（模板 HTML 内容，占绝大部分内存）。

#### 3. `cache.go` handler — 扩展 `GetCacheStats` 响应

```go
func (h *CacheHandler) GetCacheStats(c *gin.Context) {
    stats := h.htmlCache.GetStats()
    stats["site_cache"] = h.siteCache.GetStats()
    stats["template_cache"] = h.templateCache.GetStats()
    stats["compiled_cache"] = h.templateRenderer.GetCacheStats()
    c.JSON(http.StatusOK, stats)
}
```

### 前端

#### 4. HTML 缓存栏扩展

将 "HTML缓存" 栏改为展示所有缓存统计：

- HTML缓存：缓存页数 + 占用空间（已有）
- 站点缓存：条目数 + 内存占用
- 模板缓存：条目数 + 内存占用
- 编译缓存：条目数（无内存）

#### 5. 总内存汇总

`totalCacheMemory` computed 中加入站点缓存和模板缓存的 `memory_bytes`。
