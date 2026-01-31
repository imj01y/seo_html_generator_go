<template>
  <div class="processor-manage-page">
    <div class="page-header">
      <h2 class="title">数据加工管理</h2>
      <el-button size="small" @click="loadAll" :loading="loading">
        <el-icon><Refresh /></el-icon>
        刷新
      </el-button>
    </div>

    <!-- 状态卡片 -->
    <el-row :gutter="16" class="status-cards">
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-card">
          <div class="stat-label">待处理队列</div>
          <div class="stat-value">{{ status.queue_pending }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-card warning">
          <div class="stat-label">重试队列</div>
          <div class="stat-value">{{ status.queue_retry }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-card danger">
          <div class="stat-label">死信队列</div>
          <div class="stat-value">{{ status.queue_dead }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-card success">
          <div class="stat-label">今日处理</div>
          <div class="stat-value">{{ status.processed_today }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-card info">
          <div class="stat-label">处理速度</div>
          <div class="stat-value">{{ status.speed.toFixed(1) }} <small>条/秒</small></div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-card" :class="status.running ? 'success' : 'danger'">
          <div class="stat-label">运行状态</div>
          <div class="stat-value">
            <el-tag :type="status.running ? 'success' : 'danger'" size="large">
              {{ status.running ? '运行中' : '已停止' }}
            </el-tag>
          </div>
        </div>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <!-- 操作和配置 -->
      <el-col :xs="24" :lg="12">
        <!-- 操作按钮 -->
        <div class="card">
          <div class="card-header">
            <span class="title">操作</span>
          </div>
          <div class="action-buttons">
            <el-button
              type="success"
              :loading="startLoading"
              :disabled="status.running"
              @click="handleStart"
            >
              <el-icon><VideoPlay /></el-icon>
              启动
            </el-button>
            <el-button
              type="danger"
              :loading="stopLoading"
              :disabled="!status.running"
              @click="handleStop"
            >
              <el-icon><VideoPause /></el-icon>
              停止
            </el-button>
            <el-button
              type="warning"
              :loading="retryLoading"
              :disabled="status.queue_retry === 0"
              @click="handleRetryAll"
            >
              <el-icon><RefreshRight /></el-icon>
              重试失败 ({{ status.queue_retry }})
            </el-button>
            <el-button
              type="info"
              :loading="clearDeadLoading"
              :disabled="status.queue_dead === 0"
              @click="handleClearDead"
            >
              <el-icon><Delete /></el-icon>
              清空死信 ({{ status.queue_dead }})
            </el-button>
          </div>
        </div>

        <!-- 配置表单 -->
        <div class="card" style="margin-top: 20px;">
          <div class="card-header">
            <span class="title">配置</span>
          </div>
          <el-form
            :model="configForm"
            label-width="140px"
            v-loading="configLoading"
          >
            <el-form-item label="启用数据加工">
              <el-switch v-model="configForm.enabled" />
              <span class="form-tip">关闭后 Worker 启动时不会自动处理</span>
            </el-form-item>
            <el-form-item label="并发 Worker 数">
              <el-input-number
                v-model="configForm.concurrency"
                :min="1"
                :max="10"
                :step="1"
              />
              <span class="form-tip">同时处理文章的协程数量</span>
            </el-form-item>
            <el-form-item label="最大重试次数">
              <el-input-number
                v-model="configForm.retry_max"
                :min="0"
                :max="10"
                :step="1"
              />
              <span class="form-tip">超过后放入死信队列</span>
            </el-form-item>
            <el-form-item label="段落最小长度">
              <el-input-number
                v-model="configForm.min_paragraph_length"
                :min="1"
                :max="500"
                :step="10"
              />
              <span class="form-tip">字符，过短的段落将被过滤</span>
            </el-form-item>
            <el-form-item label="批量写入大小">
              <el-input-number
                v-model="configForm.batch_size"
                :min="1"
                :max="200"
                :step="10"
              />
              <span class="form-tip">每批写入数据库的记录数</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saveConfigLoading" @click="handleSaveConfig">
                保存配置
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-col>

      <!-- 统计信息 -->
      <el-col :xs="24" :lg="12">
        <div class="card">
          <div class="card-header">
            <span class="title">统计信息</span>
          </div>
          <div class="stats-list" v-loading="statsLoading">
            <div class="stats-item">
              <span class="label">累计处理文章</span>
              <span class="value">{{ formatNumber(stats.total_processed) }}</span>
            </div>
            <div class="stats-item">
              <span class="label">累计失败</span>
              <span class="value danger">{{ formatNumber(stats.total_failed) }}</span>
            </div>
            <div class="stats-item">
              <span class="label">累计重试</span>
              <span class="value warning">{{ formatNumber(stats.total_retried) }}</span>
            </div>
            <div class="stats-item">
              <span class="label">成功率</span>
              <span class="value" :class="stats.success_rate >= 95 ? 'success' : stats.success_rate >= 80 ? 'warning' : 'danger'">
                {{ stats.success_rate.toFixed(2) }}%
              </span>
            </div>
            <el-divider />
            <div class="stats-item">
              <span class="label">生成标题数</span>
              <span class="value">{{ formatNumber(stats.titles_generated) }}</span>
            </div>
            <div class="stats-item">
              <span class="label">生成段落数</span>
              <span class="value">{{ formatNumber(stats.contents_generated) }}</span>
            </div>
            <div class="stats-item">
              <span class="label">平均处理时间</span>
              <span class="value">{{ stats.avg_processing_ms.toFixed(0) }} ms</span>
            </div>
          </div>
        </div>

        <!-- 最近错误 -->
        <div class="card" style="margin-top: 20px;" v-if="status.last_error">
          <div class="card-header">
            <span class="title">最近错误</span>
          </div>
          <el-alert
            :title="status.last_error"
            type="error"
            :closable="false"
            show-icon
          />
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  getProcessorConfig,
  updateProcessorConfig,
  getProcessorStatus,
  startProcessor,
  stopProcessor,
  retryAllFailed,
  clearDeadQueue,
  getProcessorStats,
  type ProcessorConfig,
  type ProcessorStatus,
  type ProcessorStats
} from '@/api/processor'

// 加载状态
const loading = ref(false)
const configLoading = ref(false)
const saveConfigLoading = ref(false)
const statsLoading = ref(false)
const startLoading = ref(false)
const stopLoading = ref(false)
const retryLoading = ref(false)
const clearDeadLoading = ref(false)

// 数据
const status = reactive<ProcessorStatus>({
  running: false,
  workers: 0,
  queue_pending: 0,
  queue_retry: 0,
  queue_dead: 0,
  processed_total: 0,
  processed_today: 0,
  speed: 0,
  last_error: null
})

const configForm = reactive<ProcessorConfig>({
  enabled: true,
  concurrency: 3,
  retry_max: 3,
  min_paragraph_length: 20,
  batch_size: 50
})

const stats = reactive<ProcessorStats>({
  total_processed: 0,
  total_failed: 0,
  total_retried: 0,
  success_rate: 0,
  avg_processing_ms: 0,
  titles_generated: 0,
  contents_generated: 0
})

// 自动刷新定时器
let refreshTimer: number | null = null

// 格式化数字
const formatNumber = (num: number): string => {
  if (num >= 10000) return (num / 10000).toFixed(1) + '万'
  return num.toLocaleString()
}

// 加载状态
const loadStatus = async () => {
  try {
    const data = await getProcessorStatus()
    Object.assign(status, data)
  } catch (e) {
    console.error('加载状态失败:', e)
  }
}

// 加载配置
const loadConfig = async () => {
  configLoading.value = true
  try {
    const data = await getProcessorConfig()
    Object.assign(configForm, data)
  } catch (e) {
    console.error('加载配置失败:', e)
  } finally {
    configLoading.value = false
  }
}

// 加载统计
const loadStats = async () => {
  statsLoading.value = true
  try {
    const data = await getProcessorStats()
    Object.assign(stats, data)
  } catch (e) {
    console.error('加载统计失败:', e)
  } finally {
    statsLoading.value = false
  }
}

// 加载全部
const loadAll = async () => {
  loading.value = true
  try {
    await Promise.all([loadStatus(), loadConfig(), loadStats()])
  } finally {
    loading.value = false
  }
}

// 保存配置
const handleSaveConfig = async () => {
  saveConfigLoading.value = true
  try {
    await updateProcessorConfig(configForm)
    ElMessage.success('配置已保存')
  } catch (e) {
    ElMessage.error((e as Error).message || '保存失败')
  } finally {
    saveConfigLoading.value = false
  }
}

// 启动
const handleStart = async () => {
  startLoading.value = true
  try {
    await startProcessor()
    ElMessage.success('启动命令已发送')
    setTimeout(loadStatus, 1000)
  } catch (e) {
    ElMessage.error((e as Error).message || '启动失败')
  } finally {
    startLoading.value = false
  }
}

// 停止
const handleStop = async () => {
  stopLoading.value = true
  try {
    await stopProcessor()
    ElMessage.success('停止命令已发送')
    setTimeout(loadStatus, 1000)
  } catch (e) {
    ElMessage.error((e as Error).message || '停止失败')
  } finally {
    stopLoading.value = false
  }
}

// 重试所有失败
const handleRetryAll = async () => {
  try {
    await ElMessageBox.confirm(
      `确定要重试所有 ${status.queue_retry} 个失败任务吗？`,
      '确认',
      { type: 'warning' }
    )
    retryLoading.value = true
    const result = await retryAllFailed()
    ElMessage.success(`已将 ${result.count} 个任务移回待处理队列`)
    loadStatus()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error((e as Error).message || '重试失败')
    }
  } finally {
    retryLoading.value = false
  }
}

// 清空死信队列
const handleClearDead = async () => {
  try {
    await ElMessageBox.confirm(
      `确定要清空死信队列中的 ${status.queue_dead} 个任务吗？此操作不可恢复。`,
      '警告',
      { type: 'warning' }
    )
    clearDeadLoading.value = true
    const result = await clearDeadQueue()
    ElMessage.success(`已清空 ${result.count} 个死信任务`)
    loadStatus()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error((e as Error).message || '清空失败')
    }
  } finally {
    clearDeadLoading.value = false
  }
}

onMounted(() => {
  loadAll()
  // 每 5 秒自动刷新状态
  refreshTimer = window.setInterval(loadStatus, 5000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<style lang="scss" scoped>
.processor-manage-page {
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

  .status-cards {
    margin-bottom: 20px;

    .stat-card {
      background: #fff;
      border-radius: 8px;
      padding: 16px;
      text-align: center;
      box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);
      border-left: 4px solid #409eff;

      &.success { border-left-color: #67c23a; }
      &.warning { border-left-color: #e6a23c; }
      &.danger { border-left-color: #f56c6c; }
      &.info { border-left-color: #909399; }

      .stat-label {
        font-size: 13px;
        color: #909399;
        margin-bottom: 8px;
      }

      .stat-value {
        font-size: 24px;
        font-weight: 600;
        color: #303133;

        small {
          font-size: 12px;
          font-weight: normal;
          color: #909399;
        }
      }
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

  .action-buttons {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
  }

  .form-tip {
    margin-left: 12px;
    color: #909399;
    font-size: 12px;
  }

  .stats-list {
    .stats-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 12px 0;
      border-bottom: 1px solid #f0f0f0;

      &:last-child {
        border-bottom: none;
      }

      .label {
        font-size: 14px;
        color: #606266;
      }

      .value {
        font-size: 18px;
        font-weight: 600;
        color: #303133;

        &.success { color: #67c23a; }
        &.warning { color: #e6a23c; }
        &.danger { color: #f56c6c; }
      }
    }
  }
}
</style>
