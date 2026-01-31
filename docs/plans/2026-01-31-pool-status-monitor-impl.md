# Pool Status Monitor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 PoolConfig.vue 页面新增池运行状态监控区域，展示 Go 对象池和 Python 数据池的实时状态，支持预热、暂停、恢复等操作。

**Architecture:** 前端在现有 PoolConfig.vue 底部新增监控区域，左右分栏展示 Go 对象池（cls、url、keyword_emoji）和 Python 数据池（关键词、图片）。后端需补充缺失字段（last_refresh、num_workers），前端通过 API 获取统计数据并实现操作按钮。

**Tech Stack:** Vue 3 + Element Plus + TypeScript (前端), Go/Gin (后端)

---

## Task 1: 后端 - 补充 ObjectPool 统计字段

**Files:**
- Modify: `api/internal/service/object_pool.go:232-248`

**Step 1: 在 ObjectPool 结构体添加 lastRefresh 字段**

在 `object_pool.go` 的 `ObjectPool` 结构体中添加 `lastRefresh` 字段：

```go
// 在结构体中添加（约第42行 refillCount 后面）
lastRefresh atomic.Int64 // 最后刷新时间戳（Unix纳秒）
```

**Step 2: 在 checkAndRefill 中更新 lastRefresh**

修改 `checkAndRefill` 方法，在补充后更新时间戳：

```go
// 在 refillParallel() 调用后添加
p.lastRefresh.Store(time.Now().UnixNano())
```

**Step 3: 更新 Stats 方法返回新字段**

修改 `Stats()` 方法（约第232行），添加 `last_refresh` 和 `num_workers` 字段：

```go
func (p *ObjectPool[T]) Stats() map[string]interface{} {
	p.mu.RLock()
	size := p.size
	p.mu.RUnlock()

	available := p.Available()
	used := size - available

	// 计算状态
	status := "running"
	if p.stopped.Load() {
		status = "stopped"
	} else if p.paused.Load() {
		status = "paused"
	}

	// 转换 lastRefresh 时间戳
	lastRefreshNano := p.lastRefresh.Load()
	var lastRefresh *time.Time
	if lastRefreshNano > 0 {
		t := time.Unix(0, lastRefreshNano)
		lastRefresh = &t
	}

	return map[string]interface{}{
		"name":            p.name,
		"size":            size,
		"available":       available,
		"used":            used,
		"total_generated": atomic.LoadInt64(&p.totalGenerated),
		"total_consumed":  atomic.LoadInt64(&p.totalConsumed),
		"utilization":     float64(available) / float64(size) * 100,
		"paused":          p.paused.Load(),
		"status":          status,
		"refill_count":    p.refillCount.Load(),
		"num_workers":     p.numWorkers,
		"last_refresh":    lastRefresh,
	}
}
```

**Step 4: 运行测试验证**

Run: `cd api && go test ./internal/service/... -run TestObjectPool -v`
Expected: PASS

**Step 5: Commit**

```bash
git add api/internal/service/object_pool.go
git commit -m "feat(pool): add last_refresh and status fields to ObjectPool stats"
```

---

## Task 2: 后端 - 补充 DataManager 统计字段

**Files:**
- Modify: `api/internal/service/data_manager.go:414-449`

**Step 1: 定义 DataPoolStats 结构体**

在 `data_manager.go` 中添加新的结构体（在 `DataManagerStats` 后面）：

```go
// DataPoolStats 单个数据池的统计
type DataPoolStats struct {
	Name        string     `json:"name"`
	Size        int        `json:"size"`
	Available   int        `json:"available"`
	Used        int        `json:"used"`
	Utilization float64    `json:"utilization"`
	Status      string     `json:"status"`
	NumWorkers  int        `json:"num_workers"`
	LastRefresh *time.Time `json:"last_refresh"`
}
```

**Step 2: 添加 GetDataPoolsStats 方法**

在 `DataManager` 中添加新方法：

```go
// GetDataPoolsStats 返回数据池统计（与 Go 对象池格式一致）
func (m *DataManager) GetDataPoolsStats() []DataPoolStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 计算状态
	status := "running"
	if !m.running.Load() {
		status = "stopped"
	}

	// 获取 lastRefresh 指针
	var lastRefresh *time.Time
	if !m.lastReload.IsZero() {
		lastRefresh = &m.lastReload
	}

	// 计算关键词池总数
	var totalKeywords int
	for _, items := range m.keywords {
		totalKeywords += len(items)
	}

	// 计算图片池总数
	var totalImages int
	for _, items := range m.imageURLs {
		totalImages += len(items)
	}

	pools := []DataPoolStats{
		{
			Name:        "关键词缓存池",
			Size:        totalKeywords,
			Available:   totalKeywords,
			Used:        0,
			Utilization: 100.0,
			Status:      status,
			NumWorkers:  1, // 单线程加载
			LastRefresh: lastRefresh,
		},
		{
			Name:        "图片缓存池",
			Size:        totalImages,
			Available:   totalImages,
			Used:        0,
			Utilization: 100.0,
			Status:      status,
			NumWorkers:  1, // 单线程加载
			LastRefresh: lastRefresh,
		},
	}

	return pools
}
```

**Step 3: 运行测试验证**

Run: `cd api && go test ./internal/service/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add api/internal/service/data_manager.go
git commit -m "feat(data): add GetDataPoolsStats method for unified pool stats"
```

---

## Task 3: 后端 - 添加 /api/admin/data/stats 路由

**Files:**
- Modify: `api/internal/handler/router.go:421-428`

**Step 1: 修改 dataStatsHandler 返回格式**

修改 `dataStatsHandler` 函数（约第789行），返回与对象池一致的格式：

```go
// dataStatsHandler GET /stats - 获取数据池详细统计
func dataStatsHandler(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.DataManager == nil {
			core.FailWithCode(c, core.ErrInternalServer)
			return
		}

		// 返回与对象池格式一致的统计
		pools := deps.DataManager.GetDataPoolsStats()
		core.Success(c, gin.H{
			"pools": pools,
		})
	}
}
```

**Step 2: 运行测试验证**

Run: `cd api && go test ./internal/handler/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "feat(api): update /admin/data/stats to return unified pool stats format"
```

---

## Task 4: 前端 - 添加 API 封装函数

**Files:**
- Modify: `web/src/api/pool-config.ts`

**Step 1: 添加类型定义**

在 `pool-config.ts` 文件末尾添加类型定义：

```typescript
// 池状态类型
export type PoolStatus = 'running' | 'paused' | 'stopped'

// 单个池的统计
export interface PoolStats {
  name: string
  size: number
  available: number
  used: number
  utilization: number
  status: PoolStatus
  num_workers: number
  last_refresh: string | null
  // Go 对象池特有字段
  total_generated?: number
  total_consumed?: number
  paused?: boolean
  refill_count?: number
}

// Go 对象池统计响应
export interface ObjectPoolStatsResponse {
  cls: PoolStats
  url: PoolStats
  keyword_emoji?: PoolStats
}

// Python 数据池统计响应
export interface DataPoolStatsResponse {
  pools: PoolStats[]
}
```

**Step 2: 添加 API 函数**

在类型定义后添加 API 函数：

```typescript
/**
 * 获取 Go 对象池统计
 */
export const getObjectPoolStats = (): Promise<ObjectPoolStatsResponse> => {
  return request.get('/admin/pool/stats')
}

/**
 * 获取 Python 数据池统计
 */
export const getDataPoolStats = (): Promise<DataPoolStatsResponse> => {
  return request.get('/admin/data/stats')
}

/**
 * 预热对象池
 */
export const warmupPool = (percent?: number): Promise<void> => {
  return request.post('/admin/pool/warmup', { percent: percent || 0.5 })
}

/**
 * 暂停对象池补充
 */
export const pausePool = (): Promise<void> => {
  return request.post('/admin/pool/pause')
}

/**
 * 恢复对象池补充
 */
export const resumePool = (): Promise<void> => {
  return request.post('/admin/pool/resume')
}

/**
 * 刷新数据池
 */
export const refreshDataPool = (pool?: string): Promise<void> => {
  return request.post('/admin/data/refresh', { pool: pool || 'all' })
}
```

**Step 3: Commit**

```bash
git add web/src/api/pool-config.ts
git commit -m "feat(api): add pool stats and control API functions"
```

---

## Task 5: 前端 - 创建池状态卡片组件

**Files:**
- Create: `web/src/components/PoolStatusCard.vue`

**Step 1: 创建组件文件**

创建 `web/src/components/PoolStatusCard.vue`：

```vue
<template>
  <div class="pool-status-card">
    <div class="card-header">
      <span class="pool-name">{{ pool.name }}</span>
      <span :class="['status-badge', `status-${pool.status}`]">
        <span class="status-icon">{{ statusIcon }}</span>
        {{ statusText }}
      </span>
    </div>

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

    <div class="last-refresh">
      最后刷新: {{ formatTime(pool.last_refresh) }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PoolStats } from '@/api/pool-config'

const props = defineProps<{
  pool: PoolStats
}>()

const statusIcon = computed(() => {
  switch (props.pool.status) {
    case 'running': return '●'
    case 'paused': return '⏸'
    case 'stopped': return '⏹'
    default: return '●'
  }
})

const statusText = computed(() => {
  switch (props.pool.status) {
    case 'running': return '运行中'
    case 'paused': return '已暂停'
    case 'stopped': return '已停止'
    default: return '未知'
  }
})

const utilizationPercent = computed(() => {
  return props.pool.utilization || 0
})

const progressColor = computed(() => {
  const util = utilizationPercent.value
  if (util < 30) return '#67C23A'      // 绿色 - 充足
  if (util < 70) return '#409EFF'      // 蓝色 - 正常
  if (util < 90) return '#E6A23C'      // 橙色 - 偏高
  return '#F56C6C'                     // 红色 - 紧张
})

const formatNumber = (num: number): string => {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M'
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'k'
  }
  return num.toString()
}

const formatTime = (time: string | null): string => {
  if (!time) return '-'
  const date = new Date(time)
  const now = new Date()
  const isToday = date.toDateString() === now.toDateString()

  if (isToday) {
    return date.toLocaleTimeString('zh-CN', { hour12: false })
  }
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) +
         ' ' + date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
}
</script>

<style lang="scss" scoped>
.pool-status-card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 12px;

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;

    .pool-name {
      font-size: 14px;
      font-weight: 600;
      color: #303133;
    }

    .status-badge {
      font-size: 12px;
      padding: 2px 8px;
      border-radius: 4px;

      .status-icon {
        margin-right: 4px;
      }

      &.status-running {
        background: #e1f3d8;
        color: #67C23A;
      }
      &.status-paused {
        background: #faecd8;
        color: #E6A23C;
      }
      &.status-stopped {
        background: #fde2e2;
        color: #F56C6C;
      }
    }
  }

  .progress-section {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;

    :deep(.el-progress) {
      flex: 1;
    }

    .progress-text {
      font-size: 14px;
      font-weight: 600;
      color: #606266;
      min-width: 40px;
      text-align: right;
    }
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 8px;
    margin-bottom: 8px;

    .stat-item {
      text-align: center;

      .stat-label {
        display: block;
        font-size: 12px;
        color: #909399;
        margin-bottom: 2px;
      }

      .stat-value {
        display: block;
        font-size: 14px;
        font-weight: 500;
        color: #303133;
      }
    }
  }

  .last-refresh {
    font-size: 12px;
    color: #909399;
    text-align: right;
  }
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/components/PoolStatusCard.vue
git commit -m "feat(component): add PoolStatusCard component for pool status display"
```

---

## Task 6: 前端 - 在 PoolConfig.vue 中集成池状态监控

**Files:**
- Modify: `web/src/views/settings/PoolConfig.vue`

**Step 1: 导入依赖和组件**

在 `<script setup>` 部分的导入区域添加：

```typescript
import PoolStatusCard from '@/components/PoolStatusCard.vue'
import {
  getPoolConfig,
  updatePoolConfig,
  getPresets,
  formatMemorySize,
  getObjectPoolStats,
  getDataPoolStats,
  warmupPool,
  pausePool,
  resumePool,
  refreshDataPool,
  type PoolPreset,
  type PoolSizes,
  type TemplateStats,
  type MemoryEstimate,
  type PoolStats,
  type ObjectPoolStatsResponse,
  type DataPoolStatsResponse
} from '@/api/pool-config'
```

**Step 2: 添加响应式状态**

在现有的响应式状态后添加：

```typescript
// 池状态监控
const poolStatusLoading = ref(false)
const objectPoolStats = ref<PoolStats[]>([])
const dataPoolStats = ref<PoolStats[]>([])
const operationLoading = ref(false)
```

**Step 3: 添加加载池状态方法**

在 `loadConfig` 方法后添加：

```typescript
const loadPoolStatus = async () => {
  poolStatusLoading.value = true
  try {
    const [objectRes, dataRes] = await Promise.all([
      getObjectPoolStats(),
      getDataPoolStats()
    ])

    // 转换对象池统计为数组格式
    const objectPools: PoolStats[] = []
    if (objectRes.cls) {
      objectPools.push({ ...objectRes.cls, name: 'CSS 类名池' })
    }
    if (objectRes.url) {
      objectPools.push({ ...objectRes.url, name: 'URL 池' })
    }
    if (objectRes.keyword_emoji) {
      objectPools.push({ ...objectRes.keyword_emoji, name: '关键词表情池' })
    }
    objectPoolStats.value = objectPools

    // 数据池统计
    dataPoolStats.value = dataRes.pools || []
  } catch (e) {
    ElMessage.error((e as Error).message || '加载池状态失败')
  } finally {
    poolStatusLoading.value = false
  }
}

// 操作方法
const handleWarmup = async () => {
  operationLoading.value = true
  try {
    await warmupPool(0.5)
    ElMessage.success('预热已启动')
    await loadPoolStatus()
  } catch (e) {
    ElMessage.error((e as Error).message || '预热失败')
  } finally {
    operationLoading.value = false
  }
}

const handlePause = async () => {
  operationLoading.value = true
  try {
    await pausePool()
    ElMessage.success('已暂停补充')
    await loadPoolStatus()
  } catch (e) {
    ElMessage.error((e as Error).message || '暂停失败')
  } finally {
    operationLoading.value = false
  }
}

const handleResume = async () => {
  operationLoading.value = true
  try {
    await resumePool()
    ElMessage.success('已恢复补充')
    await loadPoolStatus()
  } catch (e) {
    ElMessage.error((e as Error).message || '恢复失败')
  } finally {
    operationLoading.value = false
  }
}

const handleRefreshData = async () => {
  operationLoading.value = true
  try {
    await refreshDataPool('all')
    ElMessage.success('数据刷新已启动')
    await loadPoolStatus()
  } catch (e) {
    ElMessage.error((e as Error).message || '刷新失败')
  } finally {
    operationLoading.value = false
  }
}

const handleRefreshAll = async () => {
  await loadPoolStatus()
  ElMessage.success('状态已刷新')
}
```

**Step 4: 更新 onMounted**

修改 `onMounted`：

```typescript
onMounted(() => {
  loadConfig()
  loadPoolStatus()
})
```

**Step 5: 在模板中添加池状态监控区域**

在 `</el-row>` 结束标签后（现有配置卡片区域后面），添加池状态监控区域：

```vue
    <!-- 池运行状态监控 -->
    <div class="card pool-status-section">
      <div class="card-header">
        <span class="title">池运行状态</span>
        <div class="header-actions">
          <el-button size="small" @click="handleRefreshAll" :loading="poolStatusLoading">
            刷新全部
          </el-button>
        </div>
      </div>

      <el-row :gutter="20" v-loading="poolStatusLoading">
        <!-- 左侧: Go 对象池 -->
        <el-col :xs="24" :lg="12">
          <div class="pool-section">
            <div class="section-header">
              <span class="section-title">Go 对象池</span>
              <div class="section-actions">
                <el-button size="small" @click="handleWarmup" :loading="operationLoading">
                  预热
                </el-button>
                <el-button size="small" @click="handlePause" :loading="operationLoading">
                  暂停
                </el-button>
                <el-button size="small" @click="handleResume" :loading="operationLoading">
                  恢复
                </el-button>
              </div>
            </div>
            <div class="pool-cards">
              <PoolStatusCard
                v-for="pool in objectPoolStats"
                :key="pool.name"
                :pool="pool"
              />
              <el-empty v-if="objectPoolStats.length === 0" description="暂无数据" />
            </div>
          </div>
        </el-col>

        <!-- 右侧: Python 数据池 -->
        <el-col :xs="24" :lg="12">
          <div class="pool-section">
            <div class="section-header">
              <span class="section-title">Python 数据池</span>
              <div class="section-actions">
                <el-button size="small" @click="handleRefreshData" :loading="operationLoading">
                  刷新数据
                </el-button>
              </div>
            </div>
            <div class="pool-cards">
              <PoolStatusCard
                v-for="pool in dataPoolStats"
                :key="pool.name"
                :pool="pool"
              />
              <el-empty v-if="dataPoolStats.length === 0" description="暂无数据" />
            </div>
          </div>
        </el-col>
      </el-row>
    </div>
```

**Step 6: 添加样式**

在 `<style>` 部分添加池状态监控区域的样式：

```scss
.pool-status-section {
  margin-top: 20px;

  .header-actions {
    display: flex;
    gap: 8px;
  }

  .pool-section {
    .section-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 16px;
      padding-bottom: 12px;
      border-bottom: 1px solid #ebeef5;

      .section-title {
        font-size: 15px;
        font-weight: 600;
        color: #303133;
      }

      .section-actions {
        display: flex;
        gap: 8px;
      }
    }

    .pool-cards {
      min-height: 200px;
    }
  }
}
```

**Step 7: Commit**

```bash
git add web/src/views/settings/PoolConfig.vue
git commit -m "feat(ui): integrate pool status monitor into PoolConfig page"
```

---

## Task 7: 测试验证

**Step 1: 运行后端测试**

Run: `cd api && go test ./... -v`
Expected: All tests PASS

**Step 2: 运行前端 TypeScript 检查**

Run: `cd web && npm run type-check`
Expected: No errors

**Step 3: 启动开发服务器验证**

Run: `cd web && npm run dev`
Expected: 页面正常显示，池状态监控区域可见

**Step 4: Final Commit**

```bash
git add -A
git commit -m "feat: complete pool status monitor implementation"
```

---

## Summary

完成以上任务后，PoolConfig.vue 页面将新增池运行状态监控区域，具有以下功能：

1. **左侧 Go 对象池**：展示 CSS 类名池、URL 池、关键词表情池的状态
2. **右侧 Python 数据池**：展示关键词缓存池、图片缓存池的状态
3. **每个池卡片**：显示池名称、状态、进度条、容量、可用、已用、线程数、最后刷新时间
4. **操作按钮**：预热、暂停、恢复（Go 对象池）、刷新数据（Python 数据池）、刷新全部
