<template>
  <div class="cache-manage-page">
    <div class="page-header">
      <h2 class="title">缓存管理</h2>
      <el-button size="small" @click="handleRefreshAll" :loading="refreshAllLoading">
        刷新全部
      </el-button>
    </div>

    <div class="main-tabs-container">
      <el-tabs v-model="mainTab" class="main-tabs">
        <!-- 运行状态 -->
        <el-tab-pane label="运行状态" name="status">
          <div v-if="totalCacheMemory > 0" class="total-memory-bar">
            总缓存内存: {{ formatMemorySize(totalCacheMemory) }}
          </div>
          <el-row :gutter="16" class="status-cards">
            <!-- 复用型缓存卡片 -->
            <el-col :xs="24" :lg="8">
              <div class="status-card">
                <div class="card-header">
                  <span class="card-title">复用型缓存</span>
                  <el-button size="small" @click="handleRefreshData" :loading="poolOperationLoading">
                    刷新
                  </el-button>
                </div>
                <div class="card-content">
                  <PoolStatusCard
                    v-for="pool in reusablePoolStats"
                    :key="pool.name"
                    :pool="pool"
                    @reload="handlePoolReload(pool.name)"
                    @reload-group="(groupId: number) => handlePoolReloadGroup(pool.name, groupId)"
                  />
                  <el-empty v-if="reusablePoolStats.length === 0" description="连接中..." />
                </div>
              </div>
            </el-col>

            <!-- 消费型缓存卡片 -->
            <el-col :xs="24" :lg="8">
              <div class="status-card">
                <div class="card-header">
                  <span class="card-title">消费型缓存</span>
                </div>
                <div class="card-content">
                  <PoolStatusCard
                    v-for="pool in consumablePoolStats"
                    :key="pool.name"
                    :pool="pool"
                    @reload="handlePoolReload(pool.name)"
                    @reload-group="(groupId: number) => handlePoolReloadGroup(pool.name, groupId)"
                  />
                  <el-empty v-if="consumablePoolStats.length === 0" description="连接中..." />
                </div>
              </div>
            </el-col>

            <!-- HTML缓存卡片 -->
            <el-col :xs="24" :lg="8">
              <div class="status-card" v-loading="cacheLoading">
                <div class="card-header">
                  <span class="card-title">
                    HTML缓存
                    <el-tag v-if="cacheStats.scanning" size="small" type="warning" style="margin-left: 8px;">统计中...</el-tag>
                    <el-tag v-else-if="!cacheStats.initialized" size="small" type="info" style="margin-left: 8px;">初始化中</el-tag>
                  </span>
                  <div class="card-actions">
                    <el-button size="small" type="primary" plain @click="handleRecalculate" :loading="recalculateLoading" :disabled="cacheStats.scanning">
                      重新计算
                    </el-button>
                    <el-button size="small" type="danger" plain @click="handleClearHtmlCache" :loading="clearHtmlCacheLoading" :disabled="!cacheStats.html_cache_entries">
                      清理
                    </el-button>
                  </div>
                </div>
                <div class="card-content">
                  <div class="cache-info">
                    <div class="cache-stat">
                      <span class="stat-label">缓存页数</span>
                      <span class="stat-value">{{ cacheStats.html_cache_entries || 0 }} 页</span>
                    </div>
                    <div class="cache-stat">
                      <span class="stat-label">占用空间</span>
                      <span class="stat-value">{{ formatMemoryMB(cacheStats.html_cache_memory_mb || 0) }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- 快速配置 -->
        <el-tab-pane label="快速配置" name="config">
          <el-row :gutter="16" class="config-cards">
            <!-- 配置卡片 -->
            <el-col :xs="24" :lg="12">
              <div class="config-card" v-loading="configLoading">
                <div class="card-header">
                  <span class="card-title">配置</span>
                  <span class="memory-estimate">
                    预估内存: {{ memoryEstimate.human }}
                    <el-tooltip placement="top" :show-after="100">
                      <template #content>
                        <div style="line-height: 1.8">
                          <div>预估内存 = (CSS类名池 × 20B + 内链池 × 100B + 关键词表情池 × 60B) × 1.2</div>
                          <div style="margin-top: 4px; color: #a0a0a0; font-size: 12px">1.2 为 20% 额外开销系数</div>
                        </div>
                      </template>
                      <el-icon class="formula-tip"><QuestionFilled /></el-icon>
                    </el-tooltip>
                  </span>
                </div>
                <div class="card-content">
                  <div class="config-content">
                    <div class="config-row">
                      <span class="row-label">预设等级</span>
                      <el-radio-group v-model="configForm.preset" @change="handlePresetChange">
                        <el-radio-button
                          v-for="preset in presets"
                          :key="preset.key"
                          :value="preset.key"
                        >
                          {{ preset.name }} ({{ preset.concurrency }})
                        </el-radio-button>
                        <el-radio-button value="custom">自定义</el-radio-button>
                      </el-radio-group>
                    </div>

                    <div v-if="configForm.preset === 'custom'" class="config-row">
                      <span class="row-label">自定义并发</span>
                      <el-input-number
                        v-model="configForm.concurrency"
                        :min="10"
                        :max="10000"
                        :step="50"
                        size="default"
                        @change="calculateEstimate"
                      />
                      <span class="row-tip">范围: 10 - 10000</span>
                    </div>

                    <div class="config-row">
                      <span class="row-label">缓冲时间</span>
                      <el-input-number
                        v-model="configForm.buffer_seconds"
                        :min="5"
                        :max="30"
                        size="default"
                        @change="calculateEstimate"
                      />
                      <span class="row-tip">秒 (5-30)</span>
                      <el-button type="primary" :loading="configSaving" @click="handleSaveConfig" class="apply-btn">
                        应用配置
                      </el-button>
                    </div>
                  </div>
                </div>
              </div>
            </el-col>

            <!-- 预估详情卡片 -->
            <el-col :xs="24" :lg="12">
              <div class="config-card" v-loading="configLoading">
                <div class="card-header">
                  <span class="card-title">预估详情</span>
                </div>
                <div class="card-content">
                  <div class="details-content">
                    <div class="detail-block">
                      <div class="block-title">
                        模板基准
                        <span class="source-template" v-if="templateStats.source_template">
                          ({{ templateStats.source_template }})
                        </span>
                      </div>
                      <div class="block-items">
                        <div class="block-item">
                          <span class="item-label">单页关键词</span>
                          <span class="item-value">{{ templateStats.max_keyword }}</span>
                        </div>
                        <div class="block-item">
                          <span class="item-label">单页图片</span>
                          <span class="item-value">{{ templateStats.max_image }}</span>
                        </div>
                        <div class="block-item">
                          <span class="item-label">单页内链</span>
                          <span class="item-value">{{ templateStats.max_url }}</span>
                        </div>
                        <div class="block-item">
                          <span class="item-label">单页 CSS 类</span>
                          <span class="item-value">{{ templateStats.max_cls }}</span>
                        </div>
                      </div>
                    </div>

                    <div class="detail-block">
                      <div class="block-title">
                        池大小预估
                        <el-tooltip
                          placement="top"
                          :show-after="100"
                        >
                          <template #content>
                            <div style="line-height: 1.8">
                              <div>池大小 = 单页调用次数 × 并发数 × 缓冲时间(秒)</div>
                              <div style="margin-top: 4px; color: #a0a0a0; font-size: 12px">
                                单页调用次数来自模板分析
                              </div>
                            </div>
                          </template>
                          <el-icon class="formula-tip"><QuestionFilled /></el-icon>
                        </el-tooltip>
                      </div>
                      <div class="block-items">
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
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- 高级配置 -->
        <el-tab-pane label="高级配置" name="dataPool">
          <div class="data-pool-content" v-loading="cachePoolLoading">
            <el-form
              :model="cachePoolForm"
              label-width="140px"
            >
              <el-row :gutter="24">
                <el-col :xs="24" :lg="12">
                  <div class="config-card">
                    <div class="card-content">
                      <el-form-item label="选择缓存">
                        <el-select v-model="selectedPool" style="width: 100%">
                          <el-option
                            v-for="opt in poolOptions"
                            :key="opt.value"
                            :label="opt.label"
                            :value="opt.value"
                          />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="缓存大小">
                        <el-input-number
                          v-model="currentPoolSize"
                          :min="100000"
                          :max="2000000"
                          :step="100000"
                        />
                        <span class="form-tip">条</span>
                      </el-form-item>
                      <el-form-item label="生成协程数">
                        <el-input-number
                          v-model="currentWorkers"
                          :min="1"
                          :max="50"
                          :step="1"
                        />
                        <span class="form-tip">个</span>
                      </el-form-item>
                      <el-form-item label="生成间隔">
                        <el-input-number
                          v-model="currentRefillIntervalMs"
                          :min="10"
                          :max="1000"
                          :step="10"
                        />
                        <span class="form-tip">毫秒</span>
                      </el-form-item>
                      <el-form-item label="补充阈值">
                        <el-input-number
                          v-model="currentThreshold"
                          :min="0.1"
                          :max="0.9"
                          :step="0.1"
                          :precision="2"
                        />
                        <span class="form-tip">(0.1-0.9)</span>
                      </el-form-item>
                      <div class="card-footer">
                        <el-button type="primary" :loading="cachePoolSaveLoading" @click="handleSaveCachePool">
                          保存配置
                        </el-button>
                        <div v-if="configUpdatedAt" class="config-updated-at">
                          上次保存时间：{{ configUpdatedAt }}
                        </div>
                      </div>
                    </div>
                  </div>
                </el-col>

              </el-row>
            </el-form>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, watch, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { QuestionFilled } from '@element-plus/icons-vue'
import PoolStatusCard from '@/components/PoolStatusCard.vue'
import { clearCache, getCacheStats, recalculateCacheStats } from '@/api/settings'
import { formatMemoryMB } from '@/utils/format'
import { getCachePoolConfig, updateCachePoolConfig, refreshDataPool, type CachePoolConfig } from '@/api/cache-pool'
import {
  getPoolConfig,
  updatePoolConfig,
  getPresets,
  formatMemorySize,
  type PoolPreset,
  type PoolSizes,
  type TemplateStats,
  type MemoryEstimate,
  type PoolStats
} from '@/api/pool-config'

// ========== Tab 状态 ==========
const mainTab = ref('status')
const refreshAllLoading = ref(false)

// ========== 缓存状态 ==========
const cacheLoading = ref(false)
const clearHtmlCacheLoading = ref(false)
const recalculateLoading = ref(false)

const cacheStats = reactive({
  html_cache_entries: 0,
  html_cache_memory_mb: 0,
  initialized: false,
  scanning: false,
  last_scan_at: null as string | null
})

// formatMemoryMB 从 @/utils/format 导入

const loadCacheStats = async () => {
  cacheLoading.value = true
  try {
    const stats = await getCacheStats()
    cacheStats.html_cache_entries = stats.html_cache_entries || 0
    cacheStats.html_cache_memory_mb = stats.html_cache_memory_mb || 0
    cacheStats.initialized = stats.initialized ?? false
    cacheStats.scanning = stats.scanning ?? false
    cacheStats.last_scan_at = stats.last_scan_at || null
  } finally {
    cacheLoading.value = false
  }
}

const handleClearHtmlCache = () => {
  ElMessageBox.confirm('确定要清理所有HTML文件缓存吗？此操作将删除全部已缓存的HTML静态文件，不可恢复。', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    clearHtmlCacheLoading.value = true
    try {
      const result = await clearCache()
      ElMessage.success(result.message)
      loadCacheStats()
    } finally {
      clearHtmlCacheLoading.value = false
    }
  })
}

const handleRecalculate = async () => {
  recalculateLoading.value = true
  try {
    const result = await recalculateCacheStats()
    cacheStats.html_cache_entries = result.total_entries || 0
    cacheStats.html_cache_memory_mb = result.total_size_mb || 0
    cacheStats.initialized = true
    cacheStats.scanning = false
    ElMessage.success(`${result.message}，耗时 ${result.duration_ms}ms`)
  } catch (e) {
    ElMessage.error('重新计算失败')
  } finally {
    recalculateLoading.value = false
  }
}

// ========== 并发配置 ==========
const configLoading = ref(false)
const configSaving = ref(false)

const presets = ref<PoolPreset[]>(getPresets())

const configForm = reactive({
  preset: 'medium',
  concurrency: 200,
  buffer_seconds: 10
})

const templateStats = reactive<TemplateStats>({
  max_cls: 0,
  max_url: 0,
  max_keyword_emoji: 0,
  max_keyword: 0,
  max_image: 0,
  max_content: 0,
  source_template: ''
})

const poolSizes = reactive<PoolSizes>({
  ClsPoolSize: 0,
  URLPoolSize: 0,
  KeywordEmojiPoolSize: 0
})

const memoryEstimate = reactive<MemoryEstimate>({
  bytes: 0,
  human: '0 MB'
})

const formatNumber = (num: number): string => {
  return num.toLocaleString()
}

const handlePresetChange = (preset: string | number | boolean | undefined) => {
  if (typeof preset !== 'string') return
  if (preset !== 'custom') {
    const selectedPreset = presets.value.find(p => p.key === preset)
    if (selectedPreset) {
      configForm.concurrency = selectedPreset.concurrency
    }
  }
  calculateEstimate()
}

const calculateEstimate = () => {
  const concurrency = configForm.preset === 'custom'
    ? configForm.concurrency
    : (presets.value.find(p => p.key === configForm.preset)?.concurrency || 200)
  const buffer = configForm.buffer_seconds

  poolSizes.ClsPoolSize = templateStats.max_cls * concurrency * buffer
  poolSizes.URLPoolSize = templateStats.max_url * concurrency * buffer
  poolSizes.KeywordEmojiPoolSize = templateStats.max_keyword_emoji * concurrency * buffer

  const clsBytes = poolSizes.ClsPoolSize * 20
  const urlBytes = poolSizes.URLPoolSize * 100
  const keywordEmojiBytes = poolSizes.KeywordEmojiPoolSize * 60
  const totalBytes = (clsBytes + urlBytes + keywordEmojiBytes) * 1.2

  memoryEstimate.bytes = totalBytes
  memoryEstimate.human = formatMemorySize(totalBytes)
}

const loadConfig = async () => {
  configLoading.value = true
  try {
    const res = await getPoolConfig()
    configForm.preset = res.config.preset
    configForm.concurrency = res.config.concurrency
    configForm.buffer_seconds = res.config.buffer_seconds
    Object.assign(templateStats, res.template_stats)
    calculateEstimate()
  } catch (e) {
    ElMessage.error((e as Error).message || '加载配置失败')
  } finally {
    configLoading.value = false
  }
}

const handleSaveConfig = async () => {
  configSaving.value = true
  try {
    const concurrency = configForm.preset === 'custom'
      ? configForm.concurrency
      : (presets.value.find(p => p.key === configForm.preset)?.concurrency || 200)

    await updatePoolConfig({
      preset: configForm.preset,
      concurrency: concurrency,
      buffer_seconds: configForm.buffer_seconds
    })

    ElMessage.success('配置已更新并生效')
    await loadConfig()
  } catch (e) {
    ElMessage.error((e as Error).message || '保存失败')
  } finally {
    configSaving.value = false
  }
}

// ========== 数据池配置 ==========
const cachePoolLoading = ref(false)
const cachePoolSaveLoading = ref(false)

const configUpdatedAt = ref('')

// 消费型缓存选择
const poolOptions = [
  { label: '标题', value: 'title' },
  { label: '正文', value: 'content' },
  { label: 'CSS 类名', value: 'cls' },
  { label: '内链', value: 'url' },
  { label: '关键词表情', value: 'keyword_emoji' }
]
const selectedPool = ref('title')

const cachePoolForm = reactive<CachePoolConfig>({
  // 标题池
  title_pool_size: 100000,
  title_workers: 4,
  title_refill_interval_ms: 200,
  title_threshold: 0.3,
  // 正文池
  content_pool_size: 500000,
  content_workers: 10,
  content_refill_interval_ms: 50,
  content_threshold: 0.4,
  // cls类名池
  cls_pool_size: 100000,
  cls_workers: 4,
  cls_refill_interval_ms: 200,
  cls_threshold: 0.3,
  // url池
  url_pool_size: 100000,
  url_workers: 4,
  url_refill_interval_ms: 200,
  url_threshold: 0.3,
  // 关键词表情池
  keyword_emoji_pool_size: 50000,
  keyword_emoji_workers: 2,
  keyword_emoji_refill_interval_ms: 200,
  keyword_emoji_threshold: 0.3
})

// 当前选中池的配置（为每个字段创建独立的 computed）
const currentPoolSize = computed({
  get: () => cachePoolForm[`${selectedPool.value}_pool_size` as keyof CachePoolConfig] as number,
  set: (val: number) => { (cachePoolForm as Record<string, unknown>)[`${selectedPool.value}_pool_size`] = val }
})
const currentWorkers = computed({
  get: () => cachePoolForm[`${selectedPool.value}_workers` as keyof CachePoolConfig] as number,
  set: (val: number) => { (cachePoolForm as Record<string, unknown>)[`${selectedPool.value}_workers`] = val }
})
const currentRefillIntervalMs = computed({
  get: () => cachePoolForm[`${selectedPool.value}_refill_interval_ms` as keyof CachePoolConfig] as number,
  set: (val: number) => { (cachePoolForm as Record<string, unknown>)[`${selectedPool.value}_refill_interval_ms`] = val }
})
const currentThreshold = computed({
  get: () => cachePoolForm[`${selectedPool.value}_threshold` as keyof CachePoolConfig] as number,
  set: (val: number) => { (cachePoolForm as Record<string, unknown>)[`${selectedPool.value}_threshold`] = val }
})

const loadCachePoolConfig = async () => {
  cachePoolLoading.value = true
  try {
    const config = await getCachePoolConfig()
    // 标题池
    cachePoolForm.title_pool_size = config.title_pool_size || 100000
    cachePoolForm.title_workers = config.title_workers || 4
    cachePoolForm.title_refill_interval_ms = config.title_refill_interval_ms || 200
    cachePoolForm.title_threshold = config.title_threshold || 0.3
    // 正文池
    cachePoolForm.content_pool_size = config.content_pool_size || 500000
    cachePoolForm.content_workers = config.content_workers || 10
    cachePoolForm.content_refill_interval_ms = config.content_refill_interval_ms || 50
    cachePoolForm.content_threshold = config.content_threshold || 0.4
    // cls类名池
    cachePoolForm.cls_pool_size = config.cls_pool_size || 100000
    cachePoolForm.cls_workers = config.cls_workers || 4
    cachePoolForm.cls_refill_interval_ms = config.cls_refill_interval_ms || 200
    cachePoolForm.cls_threshold = config.cls_threshold || 0.3
    // url池
    cachePoolForm.url_pool_size = config.url_pool_size || 100000
    cachePoolForm.url_workers = config.url_workers || 4
    cachePoolForm.url_refill_interval_ms = config.url_refill_interval_ms || 200
    cachePoolForm.url_threshold = config.url_threshold || 0.3
    // 关键词表情池
    cachePoolForm.keyword_emoji_pool_size = config.keyword_emoji_pool_size || 50000
    cachePoolForm.keyword_emoji_workers = config.keyword_emoji_workers || 2
    cachePoolForm.keyword_emoji_refill_interval_ms = config.keyword_emoji_refill_interval_ms || 200
    cachePoolForm.keyword_emoji_threshold = config.keyword_emoji_threshold || 0.3
    // 上次保存时间
    if (config.updated_at) {
      configUpdatedAt.value = new Date(config.updated_at).toLocaleString('zh-CN')
    }
  } catch (e) {
    console.error('Failed to load cache pool config:', e)
  } finally {
    cachePoolLoading.value = false
  }
}

const handleSaveCachePool = async () => {
  cachePoolSaveLoading.value = true
  try {
    await updateCachePoolConfig(cachePoolForm)
    ElMessage.success('数据池配置已保存')
    await loadCachePoolConfig()
  } catch (e) {
    ElMessage.error((e as Error).message || '保存失败')
  } finally {
    cachePoolSaveLoading.value = false
  }
}

// ========== 池运行状态 ==========
const objectPoolStats = ref<PoolStats[]>([])
const dataPoolStats = ref<PoolStats[]>([])
const poolOperationLoading = ref(false)

// 复用型缓存：reusable + static 类型的池
const reusablePoolStats = computed(() =>
  dataPoolStats.value.filter(p => p.pool_type === 'reusable' || p.pool_type === 'static')
)

// 消费型缓存：consumable 类型的池 + 对象池
const consumablePoolStats = computed(() => [
  ...dataPoolStats.value.filter(p => p.pool_type === 'consumable'),
  ...objectPoolStats.value
])

// 总缓存内存（字节）
const totalCacheMemory = computed(() => {
  const poolBytes = [...dataPoolStats.value, ...objectPoolStats.value]
    .reduce((sum, p) => sum + (p.memory_bytes || 0), 0)
  const htmlBytes = (cacheStats.html_cache_memory_mb || 0) * 1024 * 1024
  return poolBytes + htmlBytes
})

// ========== WebSocket 连接管理 ==========
let poolStatusWs: WebSocket | null = null

const connectPoolStatusWs = () => {
  if (poolStatusWs) return

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws/pool-status`

  poolStatusWs = new WebSocket(wsUrl)

  poolStatusWs.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      if (data.type === 'pool_status') {
        // 更新对象池状态
        const objectPools: PoolStats[] = []
        if (data.object_pools?.cls) {
          objectPools.push({ ...data.object_pools.cls, name: 'CSS 类名' })
        }
        if (data.object_pools?.url) {
          objectPools.push({ ...data.object_pools.url, name: '内链' })
        }
        if (data.object_pools?.keyword_emoji) {
          objectPools.push({ ...data.object_pools.keyword_emoji, name: '关键词表情' })
        }
        objectPoolStats.value = objectPools

        // 更新数据池状态
        dataPoolStats.value = data.data_pools || []
      }
    } catch (e) {
      console.error('Failed to parse pool status message:', e)
    }
  }

  poolStatusWs.onerror = (error) => {
    console.error('Pool status WebSocket error:', error)
  }

  poolStatusWs.onclose = () => {
    poolStatusWs = null
  }
}

const disconnectPoolStatusWs = () => {
  if (poolStatusWs) {
    poolStatusWs.close()
    poolStatusWs = null
  }
}

// 监听 tab 切换，自动建立/断开 WebSocket 连接
watch(mainTab, (newTab) => {
  if (newTab === 'status') {
    connectPoolStatusWs()
  } else {
    disconnectPoolStatusWs()
  }
})

const handleRefreshAll = async () => {
  refreshAllLoading.value = true
  try {
    await loadCacheStats()
    ElMessage.success('已刷新缓存数据')
  } finally {
    refreshAllLoading.value = false
  }
}

const handleRefreshData = async () => {
  poolOperationLoading.value = true
  try {
    await refreshDataPool('all')
    ElMessage.success('数据刷新已启动')
  } catch (e) {
    ElMessage.error((e as Error).message || '刷新失败')
  } finally {
    poolOperationLoading.value = false
  }
}

const handlePoolReload = async (poolName: string) => {
  poolOperationLoading.value = true
  try {
    const poolMap: Record<string, string> = {
      '关键词': 'keywords',
      '图片': 'images',
      '表情': 'emojis',
      '标题': 'titles',
      '正文': 'contents',
      '关键词表情': 'keyword_emojis'
    }
    const pool = poolMap[poolName]
    if (pool) {
      await refreshDataPool(pool)
      ElMessage.success(`${poolName}重载成功`)
      // WebSocket 会自动推送池状态更新
    }
  } catch (e) {
    ElMessage.error((e as Error).message || '重载失败')
  } finally {
    poolOperationLoading.value = false
  }
}

const handlePoolReloadGroup = async (poolName: string, groupId: number) => {
  try {
    const poolMap: Record<string, string> = {
      '关键词': 'keywords',
      '图片': 'images',
      '标题': 'titles',
      '正文': 'contents',
      '关键词表情': 'keyword_emojis'
    }
    const pool = poolMap[poolName]
    if (pool) {
      await refreshDataPool(pool, groupId)
      ElMessage.success(`${poolName}分组${groupId}重载成功`)
      // WebSocket 会自动推送池状态更新
    }
  } catch (e) {
    ElMessage.error((e as Error).message || '重载失败')
  }
}

// ========== 初始化 ==========
onMounted(() => {
  loadCacheStats()
  loadConfig()
  loadCachePoolConfig()
  // 初始 tab 是 status，建立 WebSocket 连接
  if (mainTab.value === 'status') {
    connectPoolStatusWs()
  }
})

onUnmounted(() => {
  disconnectPoolStatusWs()
})
</script>

<style lang="scss" scoped>
.cache-manage-page {
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;

    .title {
      font-size: 20px;
      font-weight: 600;
      color: #303133;
    }
  }

  .main-tabs-container {
    background-color: #fff;
    border-radius: 8px;
    padding: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);
  }

  .main-tabs {
    :deep(.el-tabs__header) {
      margin-bottom: 20px;
    }
  }

  .total-memory-bar {
    margin-bottom: 16px;
    padding: 10px 16px;
    background-color: #f0f5ff;
    border-radius: 6px;
    font-size: 14px;
    color: #303133;
    font-weight: 500;
  }

  // 运行状态三卡片
  .status-cards {
    .el-col {
      margin-bottom: 16px;

      @media (min-width: 1200px) {
        margin-bottom: 0;
      }
    }
  }

  .status-card {
    background-color: #f5f7fa;
    border-radius: 8px;
    padding: 16px;
    height: 100%;

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 16px;
      padding-bottom: 12px;
      border-bottom: 1px solid #e4e7ed;
      flex-wrap: wrap;
      gap: 8px;

      .card-title {
        font-size: 15px;
        font-weight: 600;
        color: #303133;
      }

      .card-actions {
        display: flex;
        gap: 6px;
        flex-wrap: wrap;
      }
    }

    .card-content {
      min-height: 120px;
    }
  }

  // HTML 缓存信息
  .cache-info {
    .cache-stat {
      display: flex;
      justify-content: space-between;
      padding: 12px;
      background-color: #fff;
      border-radius: 6px;
      margin-bottom: 8px;

      &:last-child {
        margin-bottom: 0;
      }

      .stat-label {
        font-size: 14px;
        color: #606266;
      }

      .stat-value {
        font-size: 16px;
        font-weight: 600;
        color: #303133;
      }
    }
  }

  // 并发配置卡片
  .config-cards {
    .el-col {
      margin-bottom: 16px;

      @media (min-width: 1200px) {
        margin-bottom: 0;
      }
    }
  }

  .config-card {
    background-color: #f5f7fa;
    border-radius: 8px;
    padding: 16px;
    height: 100%;

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 16px;
      padding-bottom: 12px;
      border-bottom: 1px solid #e4e7ed;

      .card-title {
        font-size: 15px;
        font-weight: 600;
        color: #303133;
      }

      .memory-estimate {
        color: #909399;
        font-size: 14px;
      }
    }
  }

  .config-content {
    .config-row {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 16px;

      &:last-child {
        margin-bottom: 0;
      }

      .row-label {
        width: 80px;
        flex-shrink: 0;
        color: #606266;
        font-size: 14px;
      }

      .row-tip {
        color: #909399;
        font-size: 12px;
      }

      .apply-btn {
        margin-left: auto;
      }
    }

    :deep(.el-radio-group) {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }
  }

  // 数据池配置
  .data-pool-content {
    .el-col {
      margin-bottom: 16px;

      @media (min-width: 1200px) {
        margin-bottom: 0;
      }
    }

    .form-tip {
      margin-left: 12px;
      color: #909399;
      font-size: 12px;
    }

    .card-footer {
      margin-top: 8px;
      position: relative;
      text-align: center;
    }

    .config-updated-at {
      position: absolute;
      right: 0;
      top: 50%;
      transform: translateY(-50%);
      color: #909399;
      font-size: 13px;
    }

    :deep(.el-form-item) {
      margin-bottom: 18px;
    }

    :deep(.el-input-number) {
      width: 180px;
    }
  }

  .details-content {
    .detail-block {
      margin-bottom: 16px;

      &:last-child {
        margin-bottom: 0;
      }

      .block-title {
        font-size: 14px;
        font-weight: 500;
        color: #606266;
        margin-bottom: 12px;

        .source-template {
          font-weight: normal;
          color: #909399;
          font-size: 12px;
        }

        .formula-tip {
          margin-left: 4px;
          color: #909399;
          font-size: 14px;
          cursor: pointer;
          vertical-align: middle;
        }
      }

      .block-items {
        display: flex;
        flex-direction: column;
        gap: 8px;

        .block-item {
          display: flex;
          justify-content: space-between;
          padding: 8px 12px;
          background: #fff;
          border-radius: 4px;

          .item-label {
            color: #606266;
            font-size: 13px;
          }

          .item-value {
            color: #303133;
            font-weight: 500;
            font-size: 13px;
          }
        }
      }
    }
  }
}
</style>
