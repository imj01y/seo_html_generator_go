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
          <el-row :gutter="16" class="status-cards">
            <!-- 数据卡片 -->
            <el-col :xs="24" :lg="8">
              <div class="status-card">
                <div class="card-header">
                  <span class="card-title">数据池</span>
                  <el-button size="small" @click="handleRefreshData" :loading="poolOperationLoading">
                    刷新
                  </el-button>
                </div>
                <div class="card-content">
                  <PoolStatusCard
                    v-for="pool in dataPoolStats"
                    :key="pool.name"
                    :pool="pool"
                  />
                  <el-empty v-if="dataPoolStats.length === 0" description="连接中..." />
                </div>
              </div>
            </el-col>

            <!-- 对象卡片 -->
            <el-col :xs="24" :lg="8">
              <div class="status-card">
                <div class="card-header">
                  <span class="card-title">对象</span>
                  <div class="card-actions">
                    <el-button size="small" @click="handleWarmup" :loading="poolOperationLoading">
                      预热
                    </el-button>
                    <el-button size="small" @click="handlePause" :loading="poolOperationLoading">
                      暂停
                    </el-button>
                    <el-button size="small" @click="handleResume" :loading="poolOperationLoading">
                      恢复
                    </el-button>
                  </div>
                </div>
                <div class="card-content">
                  <PoolStatusCard
                    v-for="pool in objectPoolStats"
                    :key="pool.name"
                    :pool="pool"
                  />
                  <el-empty v-if="objectPoolStats.length === 0" description="连接中..." />
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

        <!-- 对象池配置 -->
        <el-tab-pane label="对象池配置" name="config">
          <el-row :gutter="16" class="config-cards">
            <!-- 配置卡片 -->
            <el-col :xs="24" :lg="12">
              <div class="config-card" v-loading="configLoading">
                <div class="card-header">
                  <span class="card-title">配置</span>
                  <span class="memory-estimate">预估内存: {{ memoryEstimate.human }}</span>
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
                      <div class="block-title">池大小预估</div>
                      <div class="block-items">
                        <div class="block-item">
                          <span class="item-label">关键词池</span>
                          <span class="item-value">{{ formatNumber(poolSizes.KeywordPoolSize) }}</span>
                        </div>
                        <div class="block-item">
                          <span class="item-label">图片池</span>
                          <span class="item-value">{{ formatNumber(poolSizes.ImagePoolSize) }}</span>
                        </div>
                        <div class="block-item">
                          <span class="item-label">CSS 类名池</span>
                          <span class="item-value">{{ formatNumber(poolSizes.ClsPoolSize) }}</span>
                        </div>
                        <div class="block-item">
                          <span class="item-label">URL 池</span>
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

        <!-- 数据池配置 -->
        <el-tab-pane label="数据池配置" name="dataPool">
          <div class="data-pool-content" v-loading="cachePoolLoading">
            <el-form
              :model="cachePoolForm"
              label-width="140px"
            >
              <el-row :gutter="24">
                <el-col :xs="24" :lg="12">
                  <div class="config-card">
                    <div class="card-header">
                      <span class="card-title">标题/正文池</span>
                    </div>
                    <div class="card-content">
                      <el-form-item label="标题池大小">
                        <el-input-number
                          v-model="cachePoolForm.titles_size"
                          :min="100"
                          :max="100000"
                          :step="1000"
                        />
                        <span class="form-tip">条</span>
                      </el-form-item>
                      <el-form-item label="正文池大小">
                        <el-input-number
                          v-model="cachePoolForm.contents_size"
                          :min="100"
                          :max="100000"
                          :step="1000"
                        />
                        <span class="form-tip">条</span>
                      </el-form-item>
                      <el-form-item label="补充阈值">
                        <el-input-number
                          v-model="cachePoolForm.threshold"
                          :min="10"
                          :max="cachePoolForm.titles_size"
                          :step="100"
                        />
                        <span class="form-tip">低于此值时触发补充</span>
                      </el-form-item>
                      <el-form-item label="检查间隔">
                        <el-input-number
                          v-model="cachePoolForm.refill_interval_ms"
                          :min="100"
                          :max="60000"
                          :step="100"
                        />
                        <span class="form-tip">毫秒</span>
                      </el-form-item>
                    </div>
                  </div>
                </el-col>

                <el-col :xs="24" :lg="12">
                  <div class="config-card">
                    <div class="card-header">
                      <span class="card-title">关键词/图片池</span>
                    </div>
                    <div class="card-content">
                      <el-form-item label="关键词池大小">
                        <el-input-number
                          v-model="cachePoolForm.keywords_size"
                          :min="1000"
                          :max="500000"
                          :step="10000"
                        />
                        <span class="form-tip">条</span>
                      </el-form-item>
                      <el-form-item label="图片池大小">
                        <el-input-number
                          v-model="cachePoolForm.images_size"
                          :min="1000"
                          :max="500000"
                          :step="10000"
                        />
                        <span class="form-tip">条</span>
                      </el-form-item>
                      <el-form-item label="刷新间隔">
                        <el-input-number
                          v-model="cachePoolForm.refresh_interval_ms"
                          :min="60000"
                          :max="3600000"
                          :step="60000"
                        />
                        <span class="form-tip">毫秒（定期重新加载）</span>
                      </el-form-item>
                    </div>
                  </div>
                </el-col>
              </el-row>

              <div class="form-actions">
                <el-button type="primary" :loading="cachePoolSaveLoading" @click="handleSaveCachePool">
                  保存配置
                </el-button>
              </div>
            </el-form>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PoolStatusCard from '@/components/PoolStatusCard.vue'
import { clearCache, getCacheStats, recalculateCacheStats } from '@/api/settings'
import { formatMemoryMB } from '@/utils/format'
import { getCachePoolConfig, updateCachePoolConfig, type CachePoolConfig } from '@/api/cache-pool'
import {
  getPoolConfig,
  updatePoolConfig,
  getPresets,
  formatMemorySize,
  warmupPool,
  pausePool,
  resumePool,
  refreshDataPool,
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
  KeywordEmojiPoolSize: 0,
  KeywordPoolSize: 0,
  ImagePoolSize: 0
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

  poolSizes.KeywordPoolSize = templateStats.max_keyword * concurrency * buffer
  poolSizes.ImagePoolSize = templateStats.max_image * concurrency * buffer
  poolSizes.ClsPoolSize = templateStats.max_cls * concurrency * buffer
  poolSizes.URLPoolSize = templateStats.max_url * concurrency * buffer
  poolSizes.KeywordEmojiPoolSize = templateStats.max_keyword_emoji * concurrency * buffer

  const keywordBytes = poolSizes.KeywordPoolSize * 50
  const imageBytes = poolSizes.ImagePoolSize * 150
  const clsBytes = poolSizes.ClsPoolSize * 20
  const urlBytes = poolSizes.URLPoolSize * 100
  const totalBytes = (keywordBytes + imageBytes + clsBytes + urlBytes) * 1.2

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
    Object.assign(poolSizes, res.calculated)
    Object.assign(memoryEstimate, res.memory)
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
const cachePoolForm = reactive<CachePoolConfig>({
  titles_size: 5000,
  contents_size: 5000,
  threshold: 1000,
  refill_interval_ms: 1000,
  keywords_size: 50000,
  images_size: 50000,
  refresh_interval_ms: 300000
})

const loadCachePoolConfig = async () => {
  cachePoolLoading.value = true
  try {
    const config = await getCachePoolConfig()
    cachePoolForm.titles_size = config.titles_size
    cachePoolForm.contents_size = config.contents_size
    cachePoolForm.threshold = config.threshold
    cachePoolForm.refill_interval_ms = config.refill_interval_ms
    cachePoolForm.keywords_size = config.keywords_size
    cachePoolForm.images_size = config.images_size
    cachePoolForm.refresh_interval_ms = config.refresh_interval_ms
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
          objectPools.push({ ...data.object_pools.cls, name: 'CSS 类名池' })
        }
        if (data.object_pools?.url) {
          objectPools.push({ ...data.object_pools.url, name: 'URL 池' })
        }
        if (data.object_pools?.keyword_emoji) {
          objectPools.push({ ...data.object_pools.keyword_emoji, name: '关键词表情池' })
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

const handleWarmup = async () => {
  poolOperationLoading.value = true
  try {
    await warmupPool(0.5)
    ElMessage.success('预热已启动')
  } catch (e) {
    ElMessage.error((e as Error).message || '预热失败')
  } finally {
    poolOperationLoading.value = false
  }
}

const handlePause = async () => {
  poolOperationLoading.value = true
  try {
    await pausePool()
    ElMessage.success('已暂停补充')
  } catch (e) {
    ElMessage.error((e as Error).message || '暂停失败')
  } finally {
    poolOperationLoading.value = false
  }
}

const handleResume = async () => {
  poolOperationLoading.value = true
  try {
    await resumePool()
    ElMessage.success('已恢复补充')
  } catch (e) {
    ElMessage.error((e as Error).message || '恢复失败')
  } finally {
    poolOperationLoading.value = false
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

    .form-actions {
      margin-top: 24px;
      text-align: center;
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
