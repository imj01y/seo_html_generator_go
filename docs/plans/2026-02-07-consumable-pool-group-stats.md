# 消费型缓存池分组统计 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为消费型缓存池（标题/正文）在缓存管理页面添加可展开的分组详情，显示每个分组的容量、可用、已用、内存、利用率进度条。

**Architecture:** 扩展后端 `PoolGroupInfo` 结构体，增加消费型池需要的统计字段（size/available/used/utilization/memory_bytes），用 `omitempty` 保持与复用型池的 JSON 兼容。在 `TitleGenerator` 新增 `GetGroupStats()` 方法返回按分组统计，在 `GetDataPoolsStats()` 中为标题和正文填充 `Groups` 字段。前端 `PoolStatusCard.vue` 在消费型池的聚合统计下方增加可折叠的分组详情面板，每个分组显示小进度条和关键指标。

**Tech Stack:** Go (Gin), Vue 3 (Element Plus), TypeScript, WebSocket

---

### Task 1: 扩展后端 PoolGroupInfo 结构体

**Files:**
- Modify: `api/internal/service/pool_manager.go:54-59`

**Step 1: 修改 PoolGroupInfo 结构体**

在 `api/internal/service/pool_manager.go:54-59`，将 `PoolGroupInfo` 从：

```go
type PoolGroupInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}
```

改为：

```go
// PoolGroupInfo 分组详情
type PoolGroupInfo struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Count       int     `json:"count"`
	Size        int     `json:"size,omitempty"`
	Available   int     `json:"available,omitempty"`
	Used        int     `json:"used,omitempty"`
	Utilization float64 `json:"utilization,omitempty"`
	MemoryBytes int64   `json:"memory_bytes,omitempty"`
}
```

说明：复用型池（关键词/图片）只填 `ID/Name/Count`，新增字段因 `omitempty` 不会出现在 JSON 中。消费型池（标题/正文）填满所有字段。

**Step 2: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): extend PoolGroupInfo with consumable pool stats fields"
```

---

### Task 2: TitleGenerator 新增按分组统计方法

**Files:**
- Modify: `api/internal/service/title_generator.go` (在 `GetTotalStats()` 之后，约 line 277)

**Step 1: 添加 GetGroupStats 方法**

在 `title_generator.go` 的 `GetTotalStats()` 方法（line 277）之后，添加：

```go
// GetGroupStats 获取按分组的统计信息（用于前端分组详情展示）
func (g *TitleGenerator) GetGroupStats() []PoolGroupInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	groups := make([]PoolGroupInfo, 0, len(g.pools))
	for gid, pool := range g.pools {
		current := len(pool.ch)
		maxSize := g.config.TitlePoolSize
		consumed := int(pool.consumedCount.Load())
		util := 0.0
		if maxSize > 0 {
			util = float64(current) / float64(maxSize) * 100
		}
		groups = append(groups, PoolGroupInfo{
			ID:          gid,
			Count:       current,
			Size:        maxSize,
			Available:   current,
			Used:        consumed,
			Utilization: util,
			MemoryBytes: pool.memoryBytes.Load(),
		})
	}
	return groups
}
```

注意：此方法返回的 `PoolGroupInfo` 类型在 `pool_manager.go` 中定义，同属 `core` 包，可直接引用。`Name` 字段留空，由调用方（`GetDataPoolsStats`）填充。

**Step 2: Commit**

```bash
git add api/internal/service/title_generator.go
git commit -m "feat(title): add GetGroupStats method for per-group statistics"
```

---

### Task 3: GetDataPoolsStats 为标题/正文填充分组详情

**Files:**
- Modify: `api/internal/service/pool_manager.go:635-706` (`GetDataPoolsStats` 方法)

**Step 1: 添加分组名称获取辅助方法**

在 `pool_manager.go` 的 `getImageGroupNames()` 方法（约 line 631）之后，添加：

```go
// getContentGroupNames 获取正文/标题分组名称映射
// 标题和正文的 group_id 对应 keyword_groups 表的 id
func (m *PoolManager) getContentGroupNames() map[int]string {
	return m.getKeywordGroupNames()
}
```

**Step 2: 修改标题池部分（line 653-675），在 append 前获取并填充分组详情**

将 `GetDataPoolsStats()` 中标题池部分从：

```go
	// 1. 标题池（改用 TitleGenerator 统计）
	var titlesCurrent, titlesMax int
	var titlesMemory, titlesConsumed int64
	if m.titleGenerator != nil {
		titlesCurrent, titlesMax, titlesMemory, titlesConsumed = m.titleGenerator.GetTotalStats()
	}
	titlesUsed := int(titlesConsumed) // 已用 = 实际被消费的数量
	titlesUtil := 0.0
	if titlesMax > 0 {
		titlesUtil = float64(titlesCurrent) / float64(titlesMax) * 100
	}
	pools = append(pools, PoolStatusStats{
		Name:        "标题",
		Size:        titlesMax,
		Available:   titlesCurrent,
		Used:        titlesUsed,
		Utilization: titlesUtil,
		Status:      status,
		NumWorkers:  m.config.TitleWorkers,
		LastRefresh: lastRefreshPtr,
		MemoryBytes: titlesMemory,
		PoolType:    "consumable",
	})
```

改为：

```go
	// 1. 标题池（改用 TitleGenerator 统计）
	groupNames := m.getContentGroupNames()

	var titlesCurrent, titlesMax int
	var titlesMemory, titlesConsumed int64
	var titleGroups []PoolGroupInfo
	if m.titleGenerator != nil {
		titlesCurrent, titlesMax, titlesMemory, titlesConsumed = m.titleGenerator.GetTotalStats()
		titleGroups = m.titleGenerator.GetGroupStats()
		// 填充分组名称
		for i := range titleGroups {
			if name, ok := groupNames[titleGroups[i].ID]; ok {
				titleGroups[i].Name = name
			} else {
				titleGroups[i].Name = fmt.Sprintf("分组%d", titleGroups[i].ID)
			}
		}
	}
	titlesUsed := int(titlesConsumed)
	titlesUtil := 0.0
	if titlesMax > 0 {
		titlesUtil = float64(titlesCurrent) / float64(titlesMax) * 100
	}
	pools = append(pools, PoolStatusStats{
		Name:        "标题",
		Size:        titlesMax,
		Available:   titlesCurrent,
		Used:        titlesUsed,
		Utilization: titlesUtil,
		Status:      status,
		NumWorkers:  m.config.TitleWorkers,
		LastRefresh: lastRefreshPtr,
		MemoryBytes: titlesMemory,
		PoolType:    "consumable",
		Groups:      titleGroups,
	})
```

**Step 3: 修改正文池部分（line 677-706），在遍历时收集分组详情**

将正文池部分从：

```go
	// 2. 正文池（消费型，汇总所有分组）
	m.mu.RLock()
	var contentsMax, contentsCurrent int
	var contentsMemory int64
	var contentsConsumed int64
	for _, pool := range m.contents {
		contentsMax += pool.GetMaxSize()
		contentsCurrent += pool.Len()
		contentsMemory += pool.MemoryBytes()
		contentsConsumed += pool.ConsumedCount()
	}
	m.mu.RUnlock()

	contentsUsed := int(contentsConsumed) // 已用 = 实际被消费的数量
	contentsUtil := 0.0
	if contentsMax > 0 {
		contentsUtil = float64(contentsCurrent) / float64(contentsMax) * 100
	}
	pools = append(pools, PoolStatusStats{
		Name:        "正文",
		Size:        contentsMax,
		Available:   contentsCurrent,
		Used:        contentsUsed,
		Utilization: contentsUtil,
		Status:      status,
		NumWorkers:  1,
		LastRefresh: lastRefreshPtr,
		MemoryBytes: contentsMemory,
		PoolType:    "consumable",
	})
```

改为：

```go
	// 2. 正文池（消费型，汇总所有分组 + 分组详情）
	m.mu.RLock()
	var contentsMax, contentsCurrent int
	var contentsMemory int64
	var contentsConsumed int64
	contentGroups := make([]PoolGroupInfo, 0, len(m.contents))
	for gid, pool := range m.contents {
		current := pool.Len()
		maxSize := pool.GetMaxSize()
		consumed := int(pool.ConsumedCount())
		mem := pool.MemoryBytes()

		contentsMax += maxSize
		contentsCurrent += current
		contentsMemory += mem
		contentsConsumed += pool.ConsumedCount()

		util := 0.0
		if maxSize > 0 {
			util = float64(current) / float64(maxSize) * 100
		}
		name := groupNames[gid]
		if name == "" {
			name = fmt.Sprintf("分组%d", gid)
		}
		contentGroups = append(contentGroups, PoolGroupInfo{
			ID:          gid,
			Name:        name,
			Count:       current,
			Size:        maxSize,
			Available:   current,
			Used:        consumed,
			Utilization: util,
			MemoryBytes: mem,
		})
	}
	m.mu.RUnlock()

	contentsUsed := int(contentsConsumed)
	contentsUtil := 0.0
	if contentsMax > 0 {
		contentsUtil = float64(contentsCurrent) / float64(contentsMax) * 100
	}
	pools = append(pools, PoolStatusStats{
		Name:        "正文",
		Size:        contentsMax,
		Available:   contentsCurrent,
		Used:        contentsUsed,
		Utilization: contentsUtil,
		Status:      status,
		NumWorkers:  1,
		LastRefresh: lastRefreshPtr,
		MemoryBytes: contentsMemory,
		PoolType:    "consumable",
		Groups:      contentGroups,
	})
```

注意：`groupNames` 变量在 Step 2 中已获取（标题池部分开头），正文池部分直接复用。

**Step 4: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): populate Groups field for consumable pools in GetDataPoolsStats"
```

---

### Task 4: 更新前端 TypeScript 类型

**Files:**
- Modify: `web/src/api/cache-pool.ts:13-17`

**Step 1: 扩展 PoolGroupInfo 接口**

将 `web/src/api/cache-pool.ts:13-17` 从：

```typescript
export interface PoolGroupInfo {
  id: number
  name: string
  count: number
}
```

改为：

```typescript
/** 池分组信息 */
export interface PoolGroupInfo {
  id: number
  name: string
  count: number
  // 消费型池分组扩展字段
  size?: number
  available?: number
  used?: number
  utilization?: number
  memory_bytes?: number
}
```

**Step 2: Commit**

```bash
git add web/src/api/cache-pool.ts
git commit -m "feat(web): extend PoolGroupInfo type with consumable pool fields"
```

---

### Task 5: 前端 PoolStatusCard 消费型池显示分组详情

**Files:**
- Modify: `web/src/components/PoolStatusCard.vue`

**Step 1: 在消费型池模板中添加分组折叠面板**

在 `PoolStatusCard.vue` 的消费型池 `</template>` 结束标签（line 49）之前，即 `</div>` (stats-grid 结束) 之后添加分组折叠面板。

将 line 48-49 处的：

```html
      </div>
    </template>
```

改为：

```html
      </div>

      <!-- 消费型池分组详情 -->
      <el-collapse v-if="pool.groups && pool.groups.length > 1" class="groups-collapse">
        <el-collapse-item :title="`分组详情 (${pool.groups.length} 个分组)`">
          <div class="consumable-groups-list">
            <div v-for="group in pool.groups" :key="group.id" class="consumable-group-item">
              <div class="group-header">
                <span class="group-name">{{ group.name }}</span>
                <span class="group-util">{{ (group.utilization || 0).toFixed(0) }}%</span>
              </div>
              <el-progress
                :percentage="group.utilization || 0"
                :color="getGroupColor(group.utilization || 0)"
                :stroke-width="8"
                :show-text="false"
              />
              <div class="group-stats">
                <span>{{ formatNumber(group.available ?? 0) }}/{{ formatNumber(group.size ?? 0) }}</span>
                <span>已用: {{ formatNumber(group.used ?? 0) }}</span>
                <span>{{ formatBytes(group.memory_bytes) }}</span>
              </div>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </template>
```

注意：`v-if="pool.groups && pool.groups.length > 1"` — 只有多个分组时才显示折叠面板，单分组时聚合统计已经够用。

**Step 2: 在 script 中添加 getGroupColor 方法**

在 `PoolStatusCard.vue` 的 `<script setup>` 部分，在 `progressColor` computed 之后（约 line 129），添加：

```typescript
const getGroupColor = (util: number): string => {
  if (util > 70) return '#67C23A'
  if (util > 30) return '#409EFF'
  if (util > 10) return '#E6A23C'
  return '#F56C6C'
}
```

**Step 3: 在 style 中添加分组详情样式**

在 `PoolStatusCard.vue` 的 `<style>` 部分，在 `.stats-grid` 样式块结束（约 line 255）之后，添加：

```scss
  .consumable-groups-list {
    .consumable-group-item {
      padding: 8px 12px;
      background: var(--el-fill-color-light);
      border-radius: 4px;
      margin-bottom: 6px;

      &:last-child {
        margin-bottom: 0;
      }

      .group-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 4px;

        .group-name {
          font-size: 13px;
          color: var(--el-text-color-regular);
          font-weight: 500;
        }

        .group-util {
          font-size: 12px;
          font-weight: 600;
          color: var(--el-text-color-secondary);
        }
      }

      :deep(.el-progress) {
        margin-bottom: 4px;
      }

      .group-stats {
        display: flex;
        justify-content: space-between;
        font-size: 12px;
        color: var(--el-text-color-secondary);
      }
    }
  }
```

**Step 4: Commit**

```bash
git add web/src/components/PoolStatusCard.vue
git commit -m "feat(web): add collapsible group details for consumable pool cards"
```

---

### Task 6: 验证部署

**Step 1: 构建并启动 Docker 环境**

```bash
docker compose up -d --build api web
```

等待构建完成。

**Step 2: 验证后端 WebSocket 数据**

在浏览器打开管理后台的缓存管理页面，使用浏览器开发者工具 Network -> WS 面板，查看 `/ws/pool-status` 的消息。

验证：
- 标题池的 `groups` 字段不为空，每个分组包含 `id, name, count, size, available, used, utilization, memory_bytes`
- 正文池的 `groups` 字段同样包含完整的分组信息
- 关键词/图片池的 `groups` 字段只有 `id, name, count`（其他字段因 `omitempty` 不出现在 JSON 中）

**Step 3: 验证前端显示**

在缓存管理页面，验证：
- 标题卡片下方出现 "分组详情 (N 个分组)" 折叠面板（仅当多于1个分组时）
- 展开后每个分组显示：名称、利用率百分比、小进度条、可用/容量、已用、内存
- 正文卡片同样显示分组折叠面板
- 关键词/图片卡片的分组详情保持不变（名称 + 数量 + 重载按钮）

**Step 4: Commit（如有修复）**

```bash
git add -A
git commit -m "fix: address issues found during deployment verification"
```
