<template>
  <div class="cache-manage-page">
    <div class="page-header">
      <h2 class="title">缓存管理</h2>
    </div>

    <el-row :gutter="20">
      <!-- 缓存池配置 -->
      <el-col :xs="24" :lg="12">
        <div class="card">
          <div class="card-header">
            <span class="title">缓存池配置</span>
          </div>

          <el-form
            :model="cacheSettingsForm"
            label-width="160px"
            v-loading="cacheSettingsLoading"
          >
            <el-form-item label="关键词缓存过期时间">
              <el-input-number
                v-model="cacheSettingsForm.keyword_cache_ttl"
                :min="3600"
                :max="604800"
                :step="3600"
              />
              <span class="form-tip">秒（默认86400秒=1天）</span>
            </el-form-item>
            <el-form-item label="图片缓存过期时间">
              <el-input-number
                v-model="cacheSettingsForm.image_cache_ttl"
                :min="3600"
                :max="604800"
                :step="3600"
              />
              <span class="form-tip">秒（默认86400秒=1天）</span>
            </el-form-item>
            <el-form-item label="启用缓存压缩">
              <el-switch v-model="cacheSettingsForm.cache_compress_enabled" />
            </el-form-item>
            <el-form-item label="压缩级别" v-if="cacheSettingsForm.cache_compress_enabled">
              <el-slider
                v-model="cacheSettingsForm.cache_compress_level"
                :min="1"
                :max="9"
                :step="1"
                show-stops
                style="width: 200px"
              />
              <span class="form-tip">1-9，越高压缩率越好但更耗CPU</span>
            </el-form-item>

            <el-divider content-position="left">文件缓存配置</el-divider>

            <el-form-item label="启用文件缓存">
              <el-switch v-model="cacheSettingsForm.file_cache_enabled" />
              <el-text type="info" size="small" style="margin-left: 12px;">
                启用后HTML将缓存到本地文件而非Redis，大幅降低内存占用
              </el-text>
            </el-form-item>

            <template v-if="cacheSettingsForm.file_cache_enabled">
              <el-form-item label="缓存目录">
                <el-input v-model="cacheSettingsForm.file_cache_dir" placeholder="./html_cache" style="width: 250px;" />
                <el-text type="info" size="small" style="margin-left: 12px;">
                  HTML文件存储目录
                </el-text>
              </el-form-item>

              <el-form-item label="最大缓存大小(GB)">
                <el-slider
                  v-model="cacheSettingsForm.file_cache_max_size_gb"
                  :min="1"
                  :max="100"
                  show-input
                  style="width: 300px"
                />
              </el-form-item>

              <el-form-item label="Nginx直服模式">
                <el-switch v-model="cacheSettingsForm.file_cache_nginx_mode" />
                <el-text type="info" size="small" style="margin-left: 12px;">
                  启用后不压缩存储，由Nginx直接serve文件
                </el-text>
              </el-form-item>
            </template>

            <el-divider content-position="left">内存池大小</el-divider>

            <el-form-item label="关键词池大小">
              <el-input-number
                v-model="cacheSettingsForm.keyword_pool_size"
                :min="0"
                :max="10000000"
                :step="10000"
              />
              <span class="form-tip">0=不限制，默认50万</span>
            </el-form-item>
            <el-form-item label="图片池大小">
              <el-input-number
                v-model="cacheSettingsForm.image_pool_size"
                :min="0"
                :max="10000000"
                :step="10000"
              />
              <span class="form-tip">0=不限制，默认50万</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saveCacheSettingsLoading" @click="handleSaveCacheSettings">
                保存配置
              </el-button>
              <el-button type="success" :loading="applyCacheSettingsLoading" @click="handleApplyCacheSettings">
                立即应用
              </el-button>
            </el-form-item>
          </el-form>
        </div>

        <!-- 数据处理状态 -->
        <div class="card" style="margin-top: 20px;">
          <div class="card-header">
            <span class="title">数据处理</span>
            <el-button size="small" @click="loadProcessorStats">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>

          <!-- 告警提示条 -->
          <el-alert
            v-if="contentPoolAlert.level === 'critical' || contentPoolAlert.level === 'exhausted'"
            :title="contentPoolAlert.message"
            :type="contentPoolAlert.level === 'exhausted' ? 'error' : 'warning'"
            show-icon
            :closable="false"
            style="margin-bottom: 16px;"
          />

          <div class="cache-stats" v-loading="processorLoading">
            <!-- Worker状态 -->
            <div class="cache-item">
              <div class="cache-label">处理程序</div>
              <el-tag :type="processorStats.running ? 'success' : 'danger'" size="small">
                {{ processorStats.running ? '运行中' : '已停止' }}
              </el-tag>
            </div>
            <!-- 待处理队列 -->
            <div class="cache-item">
              <div class="cache-label">待处理队列</div>
              <div class="cache-value">{{ processorStats.queue_size }} 篇</div>
              <el-button
                size="small"
                type="danger"
                plain
                @click="handleClearQueue"
                :loading="clearQueueLoading"
                :disabled="processorStats.queue_size === 0"
              >
                清空队列
              </el-button>
            </div>
            <!-- 段落池状态 -->
            <div class="cache-item">
              <div class="cache-label">段落池</div>
              <el-tag :type="getAlertTagType(contentPoolAlert.level)" size="small">
                {{ getAlertLabel(contentPoolAlert.level) }}
              </el-tag>
              <div class="cache-values">
                <div class="cache-value">可用: {{ contentPoolAlert.pool_size }} 条</div>
                <div class="cache-total">已用: {{ contentPoolAlert.used_size }} 条</div>
              </div>
              <el-button
                size="small"
                type="warning"
                plain
                @click="handleResetContentPool"
                :loading="resetPoolLoading"
              >
                重置
              </el-button>
            </div>
          </div>
        </div>
      </el-col>

      <!-- 缓存状态和操作 -->
      <el-col :xs="24" :lg="12">
        <!-- 缓存状态 -->
        <div class="card">
          <div class="card-header">
            <span class="title">缓存状态</span>
            <el-button size="small" @click="loadCacheStats">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>

          <div class="cache-stats" v-loading="cacheLoading">
            <!-- HTML文件缓存 -->
            <div class="cache-item">
              <el-tooltip content="已生成的 HTML 静态文件缓存" placement="top">
                <div class="cache-label">HTML文件缓存</div>
              </el-tooltip>
              <el-tooltip content="已缓存的页面数量" placement="top">
                <div class="cache-value">{{ cacheStats.html_cache_entries || 0 }} 页</div>
              </el-tooltip>
              <el-tooltip content="页面缓存占用的文件空间" placement="top">
                <div class="cache-memory">{{ formatMemory(cacheStats.html_cache_memory_mb || 0) }}</div>
              </el-tooltip>
              <el-button size="small" type="danger" plain @click="handleClearHtmlCache" :loading="clearHtmlCacheLoading">
                清理
              </el-button>
            </div>
            <!-- 关键词缓存 -->
            <div class="cache-section">
              <div class="section-header">
                <span class="section-title">关键词缓存</span>
                <el-button size="small" type="danger" plain @click="handleClearKeywordCache" :loading="clearKeywordCacheLoading">
                  清理
                </el-button>
              </div>

              <!-- 数据源 -->
              <div class="cache-block">
                <div class="block-header">
                  <el-tooltip content="从数据库加载到内存的原始数据" placement="top">
                    <span class="block-label">数据源</span>
                  </el-tooltip>
                  <el-tag :type="cacheStats.keyword_group_stats?.loaded ? 'success' : 'danger'" size="small">
                    {{ cacheStats.keyword_group_stats?.loaded ? '已加载' : '未加载' }}
                  </el-tag>
                </div>
                <div class="block-stats">
                  <el-tooltip content="数据源中的总记录数" placement="top">
                    <span>总数: {{ formatNumber(cacheStats.keyword_group_stats?.total ?? 0) }}</span>
                  </el-tooltip>
                  <el-tooltip content="尚未被消费到缓存池的记录数" placement="top">
                    <span>剩余: {{ formatNumber(cacheStats.keyword_group_stats?.remaining ?? 0) }} ({{ getSourcePercentage(cacheStats.keyword_group_stats) }}%)</span>
                  </el-tooltip>
                  <el-tooltip content="数据源占用的内存大小" placement="top">
                    <span>内存: {{ formatMemory(cacheStats.keyword_group_stats?.memory_mb || 0) }}</span>
                  </el-tooltip>
                </div>
                <el-progress
                  :percentage="getSourcePercentage(cacheStats.keyword_group_stats)"
                  :color="getProgressColor(getSourcePercentage(cacheStats.keyword_group_stats))"
                  :stroke-width="10"
                />
              </div>

              <div class="flow-indicator">↓ 消费</div>

              <!-- 缓存池 -->
              <div class="cache-block">
                <div class="block-header">
                  <el-tooltip content="供页面生成时快速取用的缓存队列" placement="top">
                    <span class="block-label">缓存池</span>
                  </el-tooltip>
                  <el-tag :type="keywordPoolStats.running ? 'success' : 'warning'" size="small">
                    {{ keywordPoolStats.running ? '运行中' : '已停止' }}
                  </el-tag>
                </div>
                <div class="block-stats">
                  <el-tooltip content="缓存池的最大容量" placement="top">
                    <span>缓存: {{ formatNumber(keywordPoolStats.cache_size) }}</span>
                  </el-tooltip>
                  <el-tooltip content="缓存池中剩余可用的数量" placement="top">
                    <span>剩余: {{ formatNumber(keywordPoolStats.remaining) }}</span>
                  </el-tooltip>
                  <el-tooltip content="当剩余低于此值时自动从数据源补充" placement="top">
                    <span>低水位: {{ formatNumber(keywordPoolStats.low_watermark) }}</span>
                  </el-tooltip>
                </div>
                <el-progress
                  :percentage="getPoolPercentage(keywordPoolStats)"
                  :color="getProgressColor(getPoolPercentage(keywordPoolStats))"
                  :stroke-width="10"
                />
                <el-tooltip content="累计已被页面生成消费的总数量" placement="top">
                  <div class="block-footer">已消费: {{ formatNumber(keywordPoolStats.total_consumed) }}</div>
                </el-tooltip>
              </div>
            </div>

            <!-- 图片缓存 -->
            <div class="cache-section">
              <div class="section-header">
                <span class="section-title">图片缓存</span>
                <el-button size="small" type="danger" plain @click="handleClearImageCache" :loading="clearImageCacheLoading">
                  清理
                </el-button>
              </div>

              <!-- 数据源 -->
              <div class="cache-block">
                <div class="block-header">
                  <el-tooltip content="从数据库加载到内存的原始数据" placement="top">
                    <span class="block-label">数据源</span>
                  </el-tooltip>
                  <el-tag :type="cacheStats.image_group_stats?.loaded ? 'success' : 'danger'" size="small">
                    {{ cacheStats.image_group_stats?.loaded ? '已加载' : '未加载' }}
                  </el-tag>
                </div>
                <div class="block-stats">
                  <el-tooltip content="数据源中的总记录数" placement="top">
                    <span>总数: {{ formatNumber(cacheStats.image_group_stats?.total ?? 0) }}</span>
                  </el-tooltip>
                  <el-tooltip content="尚未被消费到缓存池的记录数" placement="top">
                    <span>剩余: {{ formatNumber(cacheStats.image_group_stats?.remaining ?? 0) }} ({{ getSourcePercentage(cacheStats.image_group_stats) }}%)</span>
                  </el-tooltip>
                  <el-tooltip content="数据源占用的内存大小" placement="top">
                    <span>内存: {{ formatMemory(cacheStats.image_group_stats?.memory_mb || 0) }}</span>
                  </el-tooltip>
                </div>
                <el-progress
                  :percentage="getSourcePercentage(cacheStats.image_group_stats)"
                  :color="getProgressColor(getSourcePercentage(cacheStats.image_group_stats))"
                  :stroke-width="10"
                />
              </div>

              <div class="flow-indicator">↓ 消费</div>

              <!-- 缓存池 -->
              <div class="cache-block">
                <div class="block-header">
                  <el-tooltip content="供页面生成时快速取用的缓存队列" placement="top">
                    <span class="block-label">缓存池</span>
                  </el-tooltip>
                  <el-tag :type="imagePoolStats.running ? 'success' : 'warning'" size="small">
                    {{ imagePoolStats.running ? '运行中' : '已停止' }}
                  </el-tag>
                </div>
                <div class="block-stats">
                  <el-tooltip content="缓存池的最大容量" placement="top">
                    <span>缓存: {{ formatNumber(imagePoolStats.cache_size) }}</span>
                  </el-tooltip>
                  <el-tooltip content="缓存池中剩余可用的数量" placement="top">
                    <span>剩余: {{ formatNumber(imagePoolStats.remaining) }}</span>
                  </el-tooltip>
                  <el-tooltip content="当剩余低于此值时自动从数据源补充" placement="top">
                    <span>低水位: {{ formatNumber(imagePoolStats.low_watermark) }}</span>
                  </el-tooltip>
                </div>
                <el-progress
                  :percentage="getPoolPercentage(imagePoolStats)"
                  :color="getProgressColor(getPoolPercentage(imagePoolStats))"
                  :stroke-width="10"
                />
                <el-tooltip content="累计已被页面生成消费的总数量" placement="top">
                  <div class="block-footer">已消费: {{ formatNumber(imagePoolStats.total_consumed) }}</div>
                </el-tooltip>
              </div>
            </div>
          </div>

          <el-divider />

          <div class="cache-actions">
            <el-button type="warning" @click="handleClearAllCache" :loading="clearCacheLoading">
              <el-icon><Delete /></el-icon>
              清理全部缓存
            </el-button>
            <el-button type="primary" @click="handleReloadGroups" :loading="reloadLoading">
              <el-icon><Refresh /></el-icon>
              重载分组
            </el-button>
          </div>
        </div>

      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { clearCache, getCacheStats, getCacheSettings, updateCacheSettings, applyCacheSettings, getGeneratorWorkerStatus, getGeneratorQueueStats, clearGeneratorQueue, getContentPoolAlert, resetContentPool, getCachePoolsStats } from '@/api/settings'
import type { ContentPoolAlert, CachePoolStats } from '@/api/settings'
import { reloadKeywordGroup, clearKeywordCache } from '@/api/keywords'
import { reloadImageGroup, clearImageCache } from '@/api/images'
import type { CacheStats } from '@/types'

const cacheLoading = ref(false)
const clearCacheLoading = ref(false)
const clearHtmlCacheLoading = ref(false)
const clearKeywordCacheLoading = ref(false)
const clearImageCacheLoading = ref(false)
const reloadLoading = ref(false)
const cacheSettingsLoading = ref(false)
const saveCacheSettingsLoading = ref(false)
const applyCacheSettingsLoading = ref(false)
const processorLoading = ref(false)
const clearQueueLoading = ref(false)
const resetPoolLoading = ref(false)

const processorStats = reactive({
  running: false,
  queue_size: 0
})

const contentPoolAlert = reactive<ContentPoolAlert>({
  level: 'unknown',
  message: '',
  pool_size: 0,
  used_size: 0,
  total: 0,
  updated_at: ''
})

// CachePool 缓存池状态
const defaultPoolStats: CachePoolStats = {
  cache_size: 0,
  cursor: 0,
  remaining: 0,
  low_watermark: 0,
  target_size: 0,
  total_consumed: 0,
  total_refilled: 0,
  total_merged: 0,
  running: false
}

const keywordPoolStats = reactive<CachePoolStats>({ ...defaultPoolStats })
const imagePoolStats = reactive<CachePoolStats>({ ...defaultPoolStats })

// ========== 缓存流卡片辅助函数 ==========

// 格式化大数字
const formatNumber = (num: number): string => {
  if (num >= 10000) return (num / 10000).toFixed(1) + '万'
  return num.toLocaleString()
}

// 格式化内存大小
const formatMemory = (mb: number): string => {
  if (mb >= 1024) return `${(mb / 1024).toFixed(2)} GB`
  if (mb >= 1) return `${mb.toFixed(2)} MB`
  return `${mb.toFixed(3)} MB`
}

// 计算数据源剩余百分比
const getSourcePercentage = (stats: { total: number; remaining: number } | undefined): number => {
  if (!stats || stats.total === 0) return 0
  return Math.round((stats.remaining / stats.total) * 100)
}

// 计算缓存池剩余百分比
const getPoolPercentage = (pool: CachePoolStats): number => {
  if (pool.cache_size === 0) return 0
  return Math.round((pool.remaining / pool.cache_size) * 100)
}

// 进度条颜色（仅用于进度条组件）
const getProgressColor = (pct: number): string => {
  if (pct > 30) return '#409eff'  // 默认蓝
  if (pct > 10) return '#e6a23c'  // 警告橙
  return '#f56c6c'                 // 危险红
}

// ========== 段落池辅助函数 ==========

const getAlertTagType = (level: string): 'success' | 'warning' | 'danger' | 'info' => {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    normal: 'success',
    warning: 'warning',
    critical: 'danger',
    exhausted: 'danger'
  }
  return map[level] || 'info'
}

const getAlertLabel = (level: string): string => {
  const map: Record<string, string> = {
    normal: '正常',
    warning: '预警',
    critical: '严重',
    exhausted: '枯竭',
    unknown: '未知'
  }
  return map[level] || '未知'
}

const cacheSettingsForm = reactive({
  keyword_cache_ttl: 86400,
  image_cache_ttl: 86400,
  cache_compress_enabled: true,
  cache_compress_level: 6,
  keyword_pool_size: 500000,
  image_pool_size: 500000,
  // 文件缓存配置
  file_cache_enabled: false,
  file_cache_dir: './html_cache',
  file_cache_max_size_gb: 50,
  file_cache_nginx_mode: true
})

const cacheStats = reactive<CacheStats>({
  keyword_cache_size: 0,
  image_cache_size: 0,
  keyword_group_stats: { total: 0, cursor: 0, remaining: 0, loaded: false, memory_mb: 0 },
  image_group_stats: { total: 0, cursor: 0, remaining: 0, loaded: false, memory_mb: 0 },
  html_cache_entries: 0,
  html_cache_memory_mb: 0
})

const loadCacheStats = async () => {
  cacheLoading.value = true
  try {
    const [stats, poolsStats] = await Promise.all([
      getCacheStats(),
      getCachePoolsStats().catch(() => ({ keyword_pool: null, image_pool: null }))
    ])
    Object.assign(cacheStats, stats)

    // 更新缓存池状态
    if (poolsStats.keyword_pool) {
      Object.assign(keywordPoolStats, poolsStats.keyword_pool)
    }
    if (poolsStats.image_pool) {
      Object.assign(imagePoolStats, poolsStats.image_pool)
    }
  } finally {
    cacheLoading.value = false
  }
}

const loadCacheSettings = async () => {
  cacheSettingsLoading.value = true
  try {
    const settings = await getCacheSettings()

    const settingKeys = [
      'keyword_cache_ttl',
      'image_cache_ttl',
      'cache_compress_enabled',
      'cache_compress_level',
      'keyword_pool_size',
      'image_pool_size',
      // 文件缓存配置
      'file_cache_enabled',
      'file_cache_dir',
      'file_cache_max_size_gb',
      'file_cache_nginx_mode'
    ] as const

    for (const key of settingKeys) {
      if (settings[key]) {
        (cacheSettingsForm as Record<string, unknown>)[key] = settings[key].value
      }
    }
  } catch (e) {
    console.warn('加载缓存配置失败:', e)
  } finally {
    cacheSettingsLoading.value = false
  }
}

const getCacheSettingsData = () => ({
  keyword_cache_ttl: cacheSettingsForm.keyword_cache_ttl,
  image_cache_ttl: cacheSettingsForm.image_cache_ttl,
  cache_compress_enabled: cacheSettingsForm.cache_compress_enabled,
  cache_compress_level: cacheSettingsForm.cache_compress_level,
  keyword_pool_size: cacheSettingsForm.keyword_pool_size,
  image_pool_size: cacheSettingsForm.image_pool_size,
  // 文件缓存配置
  file_cache_enabled: cacheSettingsForm.file_cache_enabled,
  file_cache_dir: cacheSettingsForm.file_cache_dir,
  file_cache_max_size_gb: cacheSettingsForm.file_cache_max_size_gb,
  file_cache_nginx_mode: cacheSettingsForm.file_cache_nginx_mode
})

const handleSaveCacheSettings = async () => {
  saveCacheSettingsLoading.value = true
  try {
    await updateCacheSettings(getCacheSettingsData())
    ElMessage.success('缓存配置已保存')
  } catch (e) {
    ElMessage.error((e as Error).message || '保存失败')
  } finally {
    saveCacheSettingsLoading.value = false
  }
}

const handleApplyCacheSettings = async () => {
  applyCacheSettingsLoading.value = true
  try {
    await updateCacheSettings(getCacheSettingsData())
    const result = await applyCacheSettings()
    ElMessage.success(result.message || '配置已应用')
  } catch (e) {
    ElMessage.error((e as Error).message || '应用失败')
  } finally {
    applyCacheSettingsLoading.value = false
  }
}

// 清理页面缓存
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

// 清理关键词缓存
const handleClearKeywordCache = () => {
  ElMessageBox.confirm('确定要清理关键词缓存吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    clearKeywordCacheLoading.value = true
    try {
      const result = await clearKeywordCache()
      ElMessage.success(result.message)
      loadCacheStats()
    } finally {
      clearKeywordCacheLoading.value = false
    }
  })
}

// 清理图片缓存
const handleClearImageCache = () => {
  ElMessageBox.confirm('确定要清理图片缓存吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    clearImageCacheLoading.value = true
    try {
      const result = await clearImageCache()
      ElMessage.success(result.message)
      loadCacheStats()
    } finally {
      clearImageCacheLoading.value = false
    }
  })
}

// 清理全部缓存
const handleClearAllCache = () => {
  ElMessageBox.confirm('确定要清理所有缓存吗？这将导致下次访问时重新加载数据。', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    clearCacheLoading.value = true
    try {
      const results = await Promise.allSettled([
        clearCache(),
        clearKeywordCache(),
        clearImageCache()
      ])

      const successCount = results.filter(r => r.status === 'fulfilled').length
      ElMessage.success(`已清理 ${successCount}/3 种缓存`)
      loadCacheStats()
    } finally {
      clearCacheLoading.value = false
    }
  })
}

const handleReloadGroups = async () => {
  reloadLoading.value = true
  try {
    const [keywordRes, imageRes] = await Promise.all([
      reloadKeywordGroup(),
      reloadImageGroup()
    ])
    ElMessage.success(`重载完成：关键词 ${keywordRes.total} 个，图片 ${imageRes.total} 个`)
    loadCacheStats()
  } finally {
    reloadLoading.value = false
  }
}

// 加载数据处理状态
const loadProcessorStats = async () => {
  processorLoading.value = true
  try {
    const [workerRes, queueRes, alertRes] = await Promise.all([
      getGeneratorWorkerStatus(),
      getGeneratorQueueStats(1),
      getContentPoolAlert()
    ])
    processorStats.running = workerRes.running || false
    processorStats.queue_size = queueRes.queue_size || 0

    if (alertRes.success && alertRes.alert) {
      Object.assign(contentPoolAlert, alertRes.alert)
    }
  } catch (error) {
    console.error('Failed to load processor stats:', error)
  } finally {
    processorLoading.value = false
  }
}

// 重置段落池
const handleResetContentPool = async () => {
  try {
    await ElMessageBox.confirm(
      '确定要重置段落池吗？这将清空已用池并从数据库重新加载所有段落ID。',
      '提示',
      { type: 'warning' }
    )
    resetPoolLoading.value = true
    const res = await resetContentPool()
    if (res.success) {
      ElMessage.success(res.message)
      await loadProcessorStats()
    } else {
      ElMessage.error(res.message || '重置失败')
    }
  } catch (error: unknown) {
    if (error !== 'cancel') {
      ElMessage.error('重置失败')
    }
  } finally {
    resetPoolLoading.value = false
  }
}

// 清空待处理队列
const handleClearQueue = async () => {
  try {
    await ElMessageBox.confirm(
      '确定要清空待处理队列吗？这将删除所有等待处理的文章任务。',
      '警告',
      { type: 'warning' }
    )
    clearQueueLoading.value = true
    const res = await clearGeneratorQueue(1)
    if (res.success) {
      ElMessage.success(`已清空 ${res.cleared} 条待处理任务`)
      await loadProcessorStats()
    }
  } catch (error: unknown) {
    if (error !== 'cancel') {
      ElMessage.error('清空失败')
    }
  } finally {
    clearQueueLoading.value = false
  }
}

onMounted(() => {
  loadCacheStats()
  loadCacheSettings()
  loadProcessorStats()
})
</script>

<style lang="scss" scoped>
.cache-manage-page {
  .mt-20 {
    margin-top: 20px;
  }

  .page-header {
    margin-bottom: 20px;

    .title {
      font-size: 20px;
      font-weight: 600;
      color: #303133;
    }
  }

  .card {
    background-color: #fff;
    border-radius: 8px;
    padding: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 20px;

      .title {
        font-size: 16px;
        font-weight: 600;
        color: #303133;
      }
    }
  }

  .form-tip {
    margin-left: 12px;
    color: #909399;
    font-size: 12px;
  }

  .cache-stats {
    display: flex;
    flex-direction: column;
    gap: 12px;

    .cache-item {
      display: flex;
      align-items: center;
      padding: 16px;
      background-color: #f5f7fa;
      border-radius: 8px;
      gap: 16px;

      .cache-label {
        font-size: 14px;
        color: #606266;
        width: 110px;
        flex-shrink: 0;
        white-space: nowrap;
      }

      .cache-values {
        display: flex;
        flex-direction: column;
        gap: 4px;
        min-width: 120px;

        .cache-value {
          font-size: 14px;
          font-weight: 600;
          color: #303133;
        }

        .cache-total {
          font-size: 12px;
          color: #909399;
        }
      }

      .cache-value {
        font-size: 18px;
        font-weight: 600;
        color: #303133;
        min-width: 80px;
      }

      .cache-memory {
        font-size: 14px;
        color: #909399;
        min-width: 70px;
      }

      .el-button {
        margin-left: auto;
      }

      &.cache-pool-item {
        background-color: #f0f9eb;
        border-left: 3px solid #67c23a;
        margin-left: 20px;

        .cache-label {
          color: #67c23a;
          font-size: 13px;
        }
      }
    }

    // ========== 简洁缓存区块样式 ==========
    .cache-section {
      background: #fff;
      border-radius: 8px;
      padding: 16px;
      margin-bottom: 12px;
      border: 1px solid #e4e7ed;

      .section-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 16px;

        .section-title {
          font-size: 15px;
          font-weight: 600;
          color: #303133;
        }
      }

      .cache-block {
        background: #f5f7fa;
        border-radius: 8px;
        padding: 12px 16px;
        margin-bottom: 8px;

        .block-header {
          display: flex;
          align-items: center;
          gap: 8px;
          margin-bottom: 8px;

          .block-label {
            font-size: 13px;
            font-weight: 600;
            color: #606266;
          }
        }

        .block-stats {
          display: flex;
          flex-wrap: wrap;
          gap: 16px;
          margin-bottom: 8px;
          font-size: 13px;
          color: #606266;

          span {
            white-space: nowrap;
          }
        }

        .block-footer {
          margin-top: 8px;
          font-size: 12px;
          color: #909399;
        }
      }

      .flow-indicator {
        text-align: center;
        color: #909399;
        font-size: 12px;
        padding: 4px 0;
      }
    }
  }

  .cache-actions {
    display: flex;
    gap: 12px;
    justify-content: center;
  }
}
</style>
