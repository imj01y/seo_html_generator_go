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
