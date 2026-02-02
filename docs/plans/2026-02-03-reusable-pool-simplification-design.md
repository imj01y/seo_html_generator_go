# 复用型缓存池简化设计方案

## 概述

将关键词池、图片池、表情库从"类生产消费模式"简化为"纯复用型缓存"，移除不必要的复杂度。

## 目标

- 启动时全量加载，不限制大小
- 新增时追加到内存，删除时重载分组
- 移除定时刷新机制
- 前端显示分组详情和重载按钮

## 设计决策

| 决策项 | 选择 | 理由 |
|--------|------|------|
| 刷新策略 | 启动加载 + 事件驱动更新 | 避免不必要的定时刷新 |
| 内存策略 | 全量加载，不限大小 | 接受 GB 级内存占用 |
| 分组设计 | 关键词/图片分组，表情库不分组 | 符合业务需求 |
| 新增处理 | 追加到内存 | 性能最优 |
| 删除处理 | 重载对应分组 | 保证数据一致性 |
| 随机算法 | Fisher-Yates 部分洗牌 | 批量获取不重复，O(k) 复杂度 |

## 架构变更

### 之前
```
数据库 → 随机抽样(限5万) → 内存 → 每5分钟定时刷新
```

### 之后
```
数据库 → 全量加载 → 内存 → 事件驱动更新(新增追加/删除重载)
```

## 后端变更

### 1. PoolManager (`api/internal/service/pool_manager.go`)

#### 移除
```go
// 删除这些常量
defaultKeywordsSize     = 50000
defaultImagesSize       = 50000
defaultRefreshIntervalMs = 300000

// 删除 refreshLoop 方法及其调用
```

#### 修改加载方法
```go
// LoadKeywords - 移除 LIMIT，全量加载
func (m *PoolManager) LoadKeywords(ctx context.Context, groupID int) (int, error) {
    query := `SELECT keyword FROM keywords WHERE group_id = ? AND status = 1`
    // 移除 ORDER BY RAND() LIMIT ?
}

// LoadImages - 同样移除 LIMIT
func (m *PoolManager) LoadImages(ctx context.Context, groupID int) (int, error) {
    query := `SELECT url FROM images WHERE group_id = ? AND status = 1`
}
```

#### 新增方法
```go
// AppendKeywords 追加关键词到内存（新增时调用）
func (m *PoolManager) AppendKeywords(groupID int, keywords []string) {
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
}

// AppendImages 追加图片到内存（新增时调用）
func (m *PoolManager) AppendImages(groupID int, urls []string) {
    m.imagesMu.Lock()
    defer m.imagesMu.Unlock()

    if m.images[groupID] == nil {
        m.images[groupID] = []string{}
    }
    m.images[groupID] = append(m.images[groupID], urls...)
}

// ReloadKeywordGroup 重载指定分组（删除时调用）
func (m *PoolManager) ReloadKeywordGroup(ctx context.Context, groupID int) error {
    _, err := m.LoadKeywords(ctx, groupID)
    return err
}

// ReloadImageGroup 重载指定分组（删除时调用）
func (m *PoolManager) ReloadImageGroup(ctx context.Context, groupID int) error {
    _, err := m.LoadImages(ctx, groupID)
    return err
}

// ReloadEmojis 重载表情库
func (m *PoolManager) ReloadEmojis(path string) error {
    return m.emojiManager.LoadFromFile(path)
}
```

#### 扩展 PoolStatusStats 结构
```go
type PoolStatusStats struct {
    // 现有字段保留（兼容消费型池）
    Name        string     `json:"name"`
    Size        int        `json:"size"`
    Available   int        `json:"available"`
    Used        int        `json:"used"`
    Utilization float64    `json:"utilization"`
    Status      string     `json:"status"`
    NumWorkers  int        `json:"num_workers"`
    LastRefresh *time.Time `json:"last_refresh"`

    // 新增字段（复用型池使用）
    PoolType    string          `json:"pool_type"`              // "consumable" | "reusable" | "static"
    Groups      []PoolGroupInfo `json:"groups,omitempty"`       // 分组详情（复用型池）
    Source      string          `json:"source,omitempty"`       // 数据来源（表情库）
}

type PoolGroupInfo struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Count int    `json:"count"`
}
```

#### 修改 GetDataPoolsStats 方法
```go
func (m *PoolManager) GetDataPoolsStats() []PoolStatusStats {
    // ... 现有代码 ...

    // 3. 关键词池（复用型，增加分组详情）
    m.keywordsMu.RLock()
    var totalKeywords int
    keywordGroups := []PoolGroupInfo{}
    for gid, items := range m.keywords {
        count := len(items)
        totalKeywords += count
        keywordGroups = append(keywordGroups, PoolGroupInfo{
            ID:    gid,
            Name:  fmt.Sprintf("分组%d", gid), // 或从数据库查询名称
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

    // 4. 图片池（同样处理）
    // ...

    // 5. 表情库
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

    return pools
}
```

### 2. KeywordsHandler (`api/internal/handler/keywords.go`)

#### 修改结构体
```go
type KeywordsHandler struct {
    db          *sqlx.DB
    poolManager *core.PoolManager  // 新增
}

func NewKeywordsHandler(db *sqlx.DB, poolManager *core.PoolManager) *KeywordsHandler {
    return &KeywordsHandler{
        db:          db,
        poolManager: poolManager,
    }
}
```

#### 修改 Add 方法
```go
func (h *KeywordsHandler) Add(c *gin.Context) {
    // ... 现有代码 ...

    // 成功后追加到缓存
    if affected > 0 && h.poolManager != nil {
        h.poolManager.AppendKeywords(groupID, []string{req.Keyword})
    }

    // ... 返回响应 ...
}
```

#### 修改 BatchAdd 方法
```go
func (h *KeywordsHandler) BatchAdd(c *gin.Context) {
    // ... 现有代码 ...

    // 成功后追加到缓存
    if added > 0 && h.poolManager != nil {
        h.poolManager.AppendKeywords(groupID, addedKeywords)
    }

    // ... 返回响应 ...
}
```

#### 修改 Upload 方法
```go
func (h *KeywordsHandler) Upload(c *gin.Context) {
    // ... 现有代码 ...

    // 成功后追加到缓存
    if added > 0 && h.poolManager != nil {
        h.poolManager.AppendKeywords(groupID, addedKeywords)
    }

    // ... 返回响应 ...
}
```

#### 修改 Delete 方法
```go
func (h *KeywordsHandler) Delete(c *gin.Context) {
    // 先查询要删除的关键词所属分组
    var groupID int
    h.db.Get(&groupID, "SELECT group_id FROM keywords WHERE id = ?", id)

    // ... 现有删除代码 ...

    // 删除后重载分组缓存
    if h.poolManager != nil {
        h.poolManager.ReloadKeywordGroup(c.Request.Context(), groupID)
    }

    // ... 返回响应 ...
}
```

#### 修改 BatchDelete、DeleteAll、DeleteGroup 方法
同样在删除后调用 `ReloadKeywordGroup`。

#### 修改 Reload 方法
```go
func (h *KeywordsHandler) Reload(c *gin.Context) {
    groupID, _ := strconv.Atoi(c.DefaultQuery("group_id", "0"))

    if h.poolManager != nil {
        if groupID > 0 {
            h.poolManager.ReloadKeywordGroup(c.Request.Context(), groupID)
        } else {
            // 重载所有分组
            groupIDs, _ := h.getKeywordGroupIDs()
            for _, gid := range groupIDs {
                h.poolManager.ReloadKeywordGroup(c.Request.Context(), gid)
            }
        }
    }

    var total int64
    h.db.Get(&total, "SELECT COUNT(*) FROM keywords WHERE status = 1")
    core.Success(c, gin.H{"success": true, "total": total})
}
```

### 3. ImagesHandler (`api/internal/handler/images.go`)

同 KeywordsHandler 的改动模式。

### 4. Router (`api/internal/handler/router.go`)

#### 修改 handler 初始化
```go
// Keywords routes
keywordsHandler := NewKeywordsHandler(deps.DB, deps.PoolManager)

// Images routes
imagesHandler := NewImagesHandler(deps.DB, deps.PoolManager)
```

#### 扩展 dataRefreshRequest
```go
type dataRefreshRequest struct {
    Pool    string `json:"pool" binding:"required,oneof=all keywords images titles contents emojis"`
    GroupID *int   `json:"group_id"`
}
```

#### 修改 dataRefreshHandler
```go
func dataRefreshHandler(deps *Dependencies) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req dataRefreshRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            core.FailWithMessage(c, core.ErrInvalidParam, err.Error())
            return
        }

        ctx := c.Request.Context()

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
            deps.PoolManager.RefreshData(ctx, req.Pool)
        }

        // ... 返回响应 ...
    }
}
```

## 前端变更

### 1. 类型定义

```typescript
// web/src/types/pool.ts
interface PoolGroupInfo {
  id: number
  name: string
  count: number
}

interface PoolStatusStats {
  name: string
  size: number
  available: number
  used: number
  utilization: number
  status: string
  num_workers: number
  last_refresh: string | null
  // 新增
  pool_type: 'consumable' | 'reusable' | 'static'
  groups?: PoolGroupInfo[]
  source?: string
}
```

### 2. PoolStatusCard.vue 修改

```vue
<template>
  <div class="pool-status-card">
    <div class="card-header">
      <span class="pool-name">{{ pool.name }}</span>
      <template v-if="pool.pool_type === 'reusable' || pool.pool_type === 'static'">
        <el-button size="small" @click="handleReload" :loading="reloading">
          重载
        </el-button>
      </template>
      <template v-else>
        <span :class="['status-badge', `status-${pool.status}`]">
          {{ statusText }}
        </span>
      </template>
    </div>

    <!-- 消费型池：显示利用率 -->
    <template v-if="pool.pool_type === 'consumable'">
      <div class="progress-section">
        <el-progress :percentage="utilizationPercent" :color="progressColor" />
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
      <el-collapse v-if="pool.groups && pool.groups.length > 0">
        <el-collapse-item title="分组详情">
          <el-table :data="pool.groups" size="small">
            <el-table-column prop="name" label="分组" />
            <el-table-column prop="count" label="数量">
              <template #default="{ row }">
                {{ formatNumber(row.count) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80">
              <template #default="{ row }">
                <el-button size="small" link @click="handleReloadGroup(row.id)">
                  重载
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-collapse-item>
      </el-collapse>
    </template>

    <!-- 静态池：显示总数和来源 -->
    <template v-else-if="pool.pool_type === 'static'">
      <div class="static-stats">
        <span class="total">总计: {{ formatNumber(pool.size) }} 个</span>
        <span class="source" v-if="pool.source">来源: {{ pool.source }}</span>
      </div>
    </template>

    <div class="last-refresh" v-if="pool.last_refresh">
      最后加载: {{ formatTime(pool.last_refresh) }}
    </div>
  </div>
</template>
```

### 3. API 调用修改

```typescript
// web/src/api/pool-config.ts

// 刷新数据池（支持分组）
export function refreshDataPool(pool: string, groupId?: number): Promise<void> {
  return request.post('/admin/data/refresh', {
    pool,
    group_id: groupId
  })
}
```

## 数据库变更

无数据库结构变更。

## 测试要点

1. 服务启动时全量加载关键词和图片
2. 新增关键词/图片后，缓存自动更新
3. 删除关键词/图片后，对应分组缓存重载
4. 前端正确显示分组详情
5. 手动重载功能正常工作
6. 表情库重载功能正常工作

## 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 千万级数据全量加载耗时 | 启动时间延长 | 异步加载，不阻塞服务启动 |
| 内存占用增加 | 可能达到 GB 级 | 监控内存使用，必要时增加服务器内存 |
| 删除时重载分组性能 | 大分组重载耗时 | 异步重载，不阻塞请求响应 |
