<template>
  <div class="pool-status-card">
    <div class="card-header">
      <span class="pool-name">{{ pool.name }}</span>
      <template v-if="pool.pool_type === 'reusable' || pool.pool_type === 'static'">
        <el-button size="small" @click="handleReload">
          重载
        </el-button>
      </template>
      <template v-else>
        <span :class="['status-badge', `status-${pool.status}`]">
          <span class="status-icon">{{ statusIcon }}</span>
          {{ statusText }}
        </span>
      </template>
    </div>

    <!-- 消费型池：显示利用率 -->
    <template v-if="!pool.pool_type || pool.pool_type === 'consumable'">
      <!-- 保持原有的消费型池显示逻辑 -->
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
          <span class="stat-label">内存</span>
          <span class="stat-value">{{ formatBytes(pool.memory_bytes) }}</span>
        </div>
      </div>
    </template>

    <!-- 复用型池：显示总数和分组 -->
    <template v-else-if="pool.pool_type === 'reusable'">
      <div class="reusable-stats">
        <span class="total">总计: {{ formatNumber(pool.size) }} 条</span>
        <span class="groups-count" v-if="pool.groups">({{ pool.groups.length }} 个分组)</span>
        <span class="memory-info">内存: {{ formatBytes(pool.memory_bytes) }}</span>
      </div>
      <el-collapse v-if="pool.groups && pool.groups.length > 0" class="groups-collapse">
        <el-collapse-item title="分组详情">
          <div class="groups-list">
            <div v-for="group in pool.groups" :key="group.id" class="group-item">
              <span class="group-name">{{ group.name }}</span>
              <span class="group-count">{{ formatNumber(group.count) }}</span>
              <el-button size="small" link @click="handleReloadGroup(group.id)">
                重载
              </el-button>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </template>

    <!-- 静态池：显示总数和来源 -->
    <template v-else-if="pool.pool_type === 'static'">
      <div class="static-stats">
        <div class="stat-row">
          <span class="stat-label">总计</span>
          <span class="stat-value">{{ formatNumber(pool.size) }} 个</span>
        </div>
        <div class="stat-row">
          <span class="stat-label">内存</span>
          <span class="stat-value">{{ formatBytes(pool.memory_bytes) }}</span>
        </div>
        <div class="stat-row" v-if="pool.source">
          <span class="stat-label">来源</span>
          <span class="stat-value">{{ pool.source }}</span>
        </div>
      </div>
    </template>

    <div class="last-refresh" v-if="pool.last_refresh">
      最后加载: {{ formatTime(pool.last_refresh) }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PoolStats } from '@/api/cache-pool'

const props = defineProps<{
  pool: PoolStats
}>()

const emit = defineEmits<{
  (e: 'reload'): void
  (e: 'reload-group', groupId: number): void
}>()

const STATUS_CONFIG: Record<string, { icon: string; text: string }> = {
  running: { icon: '●', text: '运行中' },
  paused: { icon: '⏸', text: '已暂停' },
  stopped: { icon: '⏹', text: '已停止' }
}

const statusIcon = computed(() => STATUS_CONFIG[props.pool.status]?.icon ?? '●')
const statusText = computed(() => STATUS_CONFIG[props.pool.status]?.text ?? '未知')

const utilizationPercent = computed(() => {
  return props.pool.utilization || 0
})

const progressColor = computed(() => {
  const util = utilizationPercent.value
  if (util > 70) return '#67C23A'      // 绿色 - 充足
  if (util > 30) return '#409EFF'      // 蓝色 - 正常
  if (util > 10) return '#E6A23C'      // 橙色 - 偏低
  return '#F56C6C'                     // 红色 - 紧张
})

const handleReload = () => {
  emit('reload')
}

const handleReloadGroup = (groupId: number) => {
  emit('reload-group', groupId)
}

const formatNumber = (num: number): string => {
  return num.toLocaleString()
}

const formatBytes = (bytes?: number): string => {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let unitIndex = 0
  let value = bytes
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex++
  }
  return value.toFixed(unitIndex > 0 ? 2 : 0) + ' ' + units[unitIndex]
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

  .reusable-stats {
    padding: 12px;
    background: var(--el-fill-color-light);
    border-radius: 6px;
    margin-bottom: 8px;

    .total {
      font-size: 16px;
      font-weight: 600;
      color: var(--el-text-color-primary);
    }

    .groups-count {
      margin-left: 8px;
      font-size: 14px;
      color: var(--el-text-color-secondary);
    }

    .memory-info {
      margin-left: 12px;
      font-size: 14px;
      color: var(--el-text-color-secondary);
    }
  }

  .groups-collapse {
    margin-bottom: 8px;

    :deep(.el-collapse-item__header) {
      font-size: 13px;
      color: var(--el-text-color-regular);
    }
  }

  .groups-list {
    .group-item {
      display: flex;
      align-items: center;
      padding: 8px 12px;
      background: var(--el-bg-color);
      border-radius: 4px;
      margin-bottom: 4px;

      &:last-child {
        margin-bottom: 0;
      }

      .group-name {
        flex: 1;
        font-size: 13px;
        color: var(--el-text-color-regular);
      }

      .group-count {
        margin-right: 12px;
        font-size: 13px;
        font-weight: 500;
        color: var(--el-text-color-primary);
      }
    }
  }

  .static-stats {
    .stat-row {
      display: flex;
      justify-content: space-between;
      padding: 8px 12px;
      background: var(--el-fill-color-light);
      border-radius: 4px;
      margin-bottom: 4px;

      &:last-child {
        margin-bottom: 0;
      }

      .stat-label {
        font-size: 13px;
        color: var(--el-text-color-secondary);
      }

      .stat-value {
        font-size: 14px;
        font-weight: 500;
        color: var(--el-text-color-primary);
      }
    }
  }
}
</style>
