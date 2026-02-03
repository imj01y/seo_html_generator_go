<template>
  <div class="processor-manage-page">
    <!-- 监控数据卡片 -->
    <div class="monitor-card">
      <!-- 顶部操作栏 -->
      <div class="card-header">
        <div class="action-buttons">
          <el-button
            type="success"
            size="small"
            :loading="startLoading"
            :disabled="status.running"
            @click="handleStart"
          >
            <el-icon><VideoPlay /></el-icon>
            启动
          </el-button>
          <el-button
            type="danger"
            size="small"
            :loading="stopLoading"
            :disabled="!status.running"
            @click="handleStop"
          >
            <el-icon><VideoPause /></el-icon>
            停止
          </el-button>
          <el-button
            type="warning"
            size="small"
            :loading="retryLoading"
            :disabled="status.queue_retry === 0"
            @click="handleRetryAll"
          >
            <el-icon><RefreshRight /></el-icon>
            重试失败 ({{ status.queue_retry }})
          </el-button>
          <el-button
            type="info"
            size="small"
            :loading="clearDeadLoading"
            :disabled="status.queue_dead === 0"
            @click="handleClearDead"
          >
            <el-icon><Delete /></el-icon>
            清空死信 ({{ status.queue_dead }})
          </el-button>
        </div>
      </div>

      <!-- 监控数据内容 -->
      <div class="monitor-content">
        <!-- 队列状态行 -->
        <div class="data-section">
          <div class="section-title">队列状态</div>
          <div class="data-row">
            <div class="data-item">
              <span class="label">待处理</span>
              <span class="value">{{ status.queue_pending }}</span>
            </div>
            <div class="data-item warning">
              <span class="label">重试中</span>
              <span class="value">{{ status.queue_retry }}</span>
            </div>
            <div class="data-item danger">
              <span class="label">死信</span>
              <span class="value">{{ status.queue_dead }}</span>
            </div>
            <div class="data-item success">
              <span class="label">今日处理</span>
              <span class="value">{{ status.processed_today }}</span>
            </div>
            <div class="data-item info">
              <span class="label">处理速度</span>
              <span class="value">{{ status.speed.toFixed(1) }} <small>条/秒</small></span>
            </div>
            <div class="data-item" :class="status.running ? 'success' : 'danger'">
              <span class="label">运行状态</span>
              <span class="value">
                <el-tag :type="status.running ? 'success' : 'danger'" size="small">
                  {{ status.running ? '运行中' : '已停止' }}
                </el-tag>
              </span>
            </div>
          </div>
        </div>

        <!-- 最近错误 -->
        <div class="error-section" v-if="status.last_error">
          <el-alert
            :title="status.last_error"
            type="error"
            :closable="false"
            show-icon
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { VideoPlay, VideoPause, RefreshRight, Delete } from '@element-plus/icons-vue'
import {
  startProcessor,
  stopProcessor,
  retryAllFailed,
  clearDeadQueue,
  type ProcessorStatus
} from '@/api/processor'
import { buildWsUrl } from '@/api/shared'

// 加载状态
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

// WebSocket 连接
let ws: WebSocket | null = null
let reconnectTimer: number | null = null
let reconnectDelay = 1000

// WebSocket 连接
const connectWebSocket = () => {
  ws = new WebSocket(buildWsUrl('/ws/processor-status'))

  ws.onopen = () => {
    console.log('WebSocket connected')
    reconnectDelay = 1000  // 重置重连延迟
  }

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      Object.assign(status, data)
    } catch (e) {
      console.error('Failed to parse WebSocket message:', e)
    }
  }

  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
  }

  ws.onclose = () => {
    console.log('WebSocket closed, reconnecting...')
    ws = null
    // 指数退避重连
    reconnectTimer = window.setTimeout(() => {
      connectWebSocket()
    }, reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 2, 30000)
  }
}

// 启动
const handleStart = async () => {
  startLoading.value = true
  try {
    await startProcessor()
    ElMessage.success('启动命令已发送')
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
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error((e as Error).message || '清空失败')
    }
  } finally {
    clearDeadLoading.value = false
  }
}

onMounted(() => {
  connectWebSocket()
})

onUnmounted(() => {
  // 清理 WebSocket 连接
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (ws) {
    ws.close()
    ws = null
  }
})
</script>

<style lang="scss" scoped>
.processor-manage-page {
  .monitor-card {
    background-color: #fff;
    border-radius: 8px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid #ebeef5;

    .action-buttons {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }
  }

  .monitor-content {
    padding: 20px;
  }

  .data-section {
    &:not(:last-child) {
      margin-bottom: 20px;
      padding-bottom: 20px;
      border-bottom: 1px dashed #ebeef5;
    }

    .section-title {
      font-size: 14px;
      font-weight: 600;
      color: #606266;
      margin-bottom: 12px;
    }

    .data-row {
      display: flex;
      flex-wrap: wrap;
      gap: 32px;

      .data-item {
        display: flex;
        flex-direction: column;
        gap: 4px;

        .label {
          font-size: 12px;
          color: #909399;
        }

        .value {
          font-size: 20px;
          font-weight: 600;
          color: #303133;

          small {
            font-size: 12px;
            font-weight: normal;
            color: #909399;
          }
        }

        &.success .value { color: #67c23a; }
        &.warning .value { color: #e6a23c; }
        &.danger .value { color: #f56c6c; }
        &.info .value { color: #409eff; }
      }
    }
  }

  .error-section {
    margin-top: 16px;
    padding-top: 16px;
    border-top: 1px dashed #ebeef5;
  }
}
</style>
