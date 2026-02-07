# 快速配置重构：预设配置改为管控消耗型池

## 背景

快速配置的预设（低/中/高/极高）根据并发数和缓冲时间计算池容量。当前计算了关键词、图片、CSS类名、内链四个池的大小。

但关键词和图片是**复用型缓存**（数据不消耗，反复使用），不需要根据并发设置上限容量。快速配置应该只管控**消耗型池**：关键词表情、CSS类名、内链。

## 变更内容

### 1. 前端 `web/src/api/pool-config.ts`

`PoolSizes` 接口移除 `KeywordPoolSize`、`ImagePoolSize`：

```typescript
export interface PoolSizes {
  ClsPoolSize: number
  URLPoolSize: number
  KeywordEmojiPoolSize: number
}
```

### 2. 前端 `web/src/views/cache/CacheManage.vue`

#### 2.1 `poolSizes` reactive 对象

移除 `KeywordPoolSize`、`ImagePoolSize`：

```typescript
const poolSizes = reactive<PoolSizes>({
  ClsPoolSize: 0,
  URLPoolSize: 0,
  KeywordEmojiPoolSize: 0
})
```

#### 2.2 `calculateEstimate()` 函数

移除 keyword/image 计算，内存预估改为 cls + url + keywordEmoji：

```typescript
const calculateEstimate = () => {
  const concurrency = ...
  const buffer = configForm.buffer_seconds

  poolSizes.ClsPoolSize = templateStats.max_cls * concurrency * buffer
  poolSizes.URLPoolSize = templateStats.max_url * concurrency * buffer
  poolSizes.KeywordEmojiPoolSize = templateStats.max_keyword_emoji * concurrency * buffer

  // 内存预估：与后端 EstimateMemoryUsage 对齐
  const clsBytes = poolSizes.ClsPoolSize * 20        // AvgClsSize
  const urlBytes = poolSizes.URLPoolSize * 100       // AvgURLSize
  const keywordEmojiBytes = poolSizes.KeywordEmojiPoolSize * 60  // AvgKeywordEmojiSize
  const totalBytes = (clsBytes + urlBytes + keywordEmojiBytes) * 1.2

  memoryEstimate.bytes = totalBytes
  memoryEstimate.human = formatMemorySize(totalBytes)
}
```

#### 2.3 "预估详情"卡片 UI

**模板基准**：保持不变（单页关键词、单页图片、单页内链、单页CSS类）

**池大小预估**：关键词/图片 → 关键词表情

```html
<!-- 改为三项 -->
<div class="block-item">
  <span class="item-label">关键词表情</span>
  <span class="item-value">{{ formatNumber(poolSizes.KeywordEmojiPoolSize) }}</span>
</div>
<div class="block-item">
  <span class="item-label">CSS 类名</span>
  <span class="item-value">{{ formatNumber(poolSizes.ClsPoolSize) }}</span>
</div>
<div class="block-item">
  <span class="item-label">内链</span>
  <span class="item-value">{{ formatNumber(poolSizes.URLPoolSize) }}</span>
</div>
```

### 3. 后端 `api/internal/handler/pool_config.go`

`UpdateConfig` 的 Redis 消息移除 `keyword_pool_size` 和 `image_pool_size`（reloader 不消费）：

```go
reloadMsg := map[string]interface{}{
    "action":         "reload",
    "concurrency":    concurrency,
    "buffer_seconds": req.BufferSeconds,
    "sizes": map[string]int{
        "cls_pool_size":           sizes.ClsPoolSize,
        "url_pool_size":           sizes.URLPoolSize,
        "keyword_emoji_pool_size": sizes.KeywordEmojiPoolSize,
    },
}
```

## 不变更

| 文件 | 原因 |
|------|------|
| `pool_presets.go` | `CalculatePoolSizes` 和 `EstimateMemoryUsage` 已只处理 cls/url/keywordEmoji/number |
| `pool_reloader.go` | 消费端已正确（只解析 cls/url/keywordEmoji/number） |
| `PoolSizeConfig` 结构体 | 已正确 |
| `settings.go` | 独立系统设置，不属于快速配置 |
