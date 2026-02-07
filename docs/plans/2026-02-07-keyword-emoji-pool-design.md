# 关键词表情对象池设计

## 目标

将 `random_keyword_emoji()` 从实时生成改为对象池预生成模式，消除每次渲染时的内存分配和锁开销。
默认模板中该函数在 `range(1800)` 循环内调用，每页约 1802 次，实时生成耗时约 0.9-1.8ms，
改为对象池后降至约 0.018ms（50-100 倍提升）。

## 设计方案

完全对标 TitleGenerator 的 channel + worker 模式，创建 KeywordEmojiGenerator。

### 核心结构

```go
// KeywordEmojiPool 单分组的关键词表情池
type KeywordEmojiPool struct {
    ch            chan string
    groupID       int
    memoryBytes   atomic.Int64
    consumedCount atomic.Int64
}

// KeywordEmojiGenerator 关键词表情生成器（对标 TitleGenerator）
type KeywordEmojiGenerator struct {
    pools       map[int]*KeywordEmojiPool
    poolManager *PoolManager
    config      *CachePoolConfig
    encoder     *HTMLEntityEncoder
    emojiManager *EmojiManager
    mu          sync.RWMutex
    ctx         context.Context
    cancel      context.CancelFunc
    wg          sync.WaitGroup
    stopped     atomic.Bool
}
```

### 方法清单（完全对标 TitleGenerator）

| 方法 | 功能 |
|------|------|
| generateKeywordEmoji(groupID) | 取原始关键词 + 随机插入1-2个emoji + HTML编码 |
| getOrCreatePool(groupID) | 获取或创建分组池 |
| Pop(groupID) | 从channel取值，空则同步生成 |
| fillPool(groupID, pool) | 填充到满 |
| refillWorker(groupID, pool) | 后台协程定时补充 |
| Start(groupIDs) | 初始化并启动所有分组 |
| Stop() | 停止所有worker |
| Reload(config) | 配置变更时重启 |
| ForceReload() | 强制重载所有分组 |
| ReloadGroup(groupID) | 排空并重填单个分组 |
| SyncGroups(groupIDs) | 关键词分组变化时同步 |
| GetTotalStats() | 汇总统计 |
| GetGroupStats() | 分组详情统计 |

### 配置

复用 pool_config 表中已有的 keyword_emoji_* 字段：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| keyword_emoji_pool_size | 800000 | 每分组池大小 |
| keyword_emoji_workers | 20 | 生成协程数 |
| keyword_emoji_refill_interval_ms | 30 | 补充检查间隔 |
| keyword_emoji_threshold | 0.40 | 低水位触发阈值 |

### 生成逻辑

复用现有 generateKeywordWithEmojiFromRaw 的逻辑：
1. 从 PoolManager 获取指定分组的原始关键词
2. 随机决定插入 1 或 2 个 emoji（50%概率）
3. 在关键词中随机位置插入 emoji
4. HTML 实体编码（防爬虫）
5. 返回编码后的字符串

## 集成变更

### PoolManager（pool_manager.go）

- 新增字段 `keywordEmojiGenerator *KeywordEmojiGenerator`
- InitAndStart: 创建并启动（紧跟 titleGenerator 之后）
- Stop: 关闭 keywordEmojiGenerator
- Reload: 传递新配置
- ReloadKeywordGroup: 调用 keywordEmojiGenerator.SyncGroups()
- RefreshData:
  - "keywords" 分支追加 keywordEmojiGenerator.SyncGroups()
  - "all" 分支追加 keywordEmojiGenerator.SyncGroups()
  - 新增 "keyword_emojis" 分支调用 ForceReload()
- GetDataPoolsStats: 新增第6个池 "关键词表情"，PoolType = "consumable"
- 新增 GetKeywordEmojiGenerator() 方法

### TemplateFuncsManager（template_funcs.go）

- 新增 keywordEmojiGenerator 引用
- RandomKeywordEmoji: 改为调用 keywordEmojiGenerator.Pop(groupID)，不再实时生成

### router.go

- dataRefreshRequest oneof 加 keyword_emojis
- dataRefreshHandler switch 加 "keyword_emojis" 分支（对标 "titles"）

### 前端（CacheManage.vue）

- handlePoolReload poolMap 加 '关键词表情': 'keyword_emojis'
- handlePoolReloadGroup poolMap 加 '关键词表情': 'keyword_emojis'
- 消费型缓存卡片自动展示（pool_type = "consumable" 会被 filter）

### pool.go（配置验证）

- 新增 keyword_emoji 池配置的验证范围
