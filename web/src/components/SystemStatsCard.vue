<template>
  <div class="system-stats-card" v-if="stats">
    <div class="card-header">
      <span class="title">系统资源</span>
    </div>
    <div class="card-body">
      <!-- 左侧：核心指标 -->
      <div class="left-panel">
        <!-- CPU -->
        <div class="stat-row">
          <span class="stat-label">CPU</span>
          <el-progress
            :percentage="stats.cpu.usage_percent"
            :stroke-width="20"
            :color="getProgressColor(stats.cpu.usage_percent)"
            :text-inside="true"
            :format="(p: number) => p.toFixed(2) + '%'"
            class="stat-progress"
          />
          <span class="stat-extra">{{ stats.cpu.cores }}核</span>
        </div>
        <!-- 内存 -->
        <div class="stat-row">
          <span class="stat-label">内存</span>
          <el-progress
            :percentage="stats.memory.usage_percent"
            :stroke-width="20"
            :color="getProgressColor(stats.memory.usage_percent)"
            :text-inside="true"
            :format="(p: number) => p.toFixed(2) + '%'"
            class="stat-progress"
          />
          <span class="stat-extra">{{ formatMemoryGB(stats.memory.used_bytes) }}/{{ formatMemoryGB(stats.memory.total_bytes) }}G</span>
        </div>
        <!-- 负载 -->
        <div class="stat-row">
          <span class="stat-label">负载</span>
          <span class="stat-load">
            {{ stats.load.load1.toFixed(2) }} / {{ stats.load.load5.toFixed(2) }} / {{ stats.load.load15.toFixed(2) }}
          </span>
        </div>
        <!-- 网络 -->
        <div class="stat-row">
          <span class="stat-label">网络</span>
          <span class="stat-network">
            <span class="upload">↑ {{ formatSpeed(stats.network.bytes_sent_per_sec) }}</span>
            <span class="download">↓ {{ formatSpeed(stats.network.bytes_recv_per_sec) }}</span>
          </span>
        </div>
      </div>
      <!-- 右侧：磁盘 -->
      <div class="right-panel">
        <div class="disk-list">
          <div
            class="disk-item"
            v-for="disk in stats.disks"
            :key="disk.path"
            :title="`${formatDiskSize(disk.used_bytes)} / ${formatDiskSize(disk.total_bytes)}`"
          >
            <el-progress
              type="circle"
              :percentage="disk.usage_percent"
              :width="80"
              :stroke-width="8"
              :color="getProgressColor(disk.usage_percent)"
              :format="(p: number) => p.toFixed(1) + '%'"
            />
            <span class="disk-path">{{ disk.path }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
  <div class="system-stats-card loading" v-else>
    <el-skeleton :rows="4" animated />
  </div>
</template>

<script setup lang="ts">
import type { SystemStats } from '@/types/system-stats'

defineProps<{
  stats: SystemStats | null
}>()

// 根据百分比返回进度条颜色
function getProgressColor(percent: number): string {
  if (percent >= 90) return '#f56c6c'
  if (percent >= 70) return '#e6a23c'
  return '#67c23a'
}

// 格式化内存（字节转GB，保留2位小数）
function formatMemoryGB(bytes: number): string {
  return (bytes / (1024 * 1024 * 1024)).toFixed(2)
}

// 格式化网络速率
function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec >= 1024 * 1024 * 1024) {
    return (bytesPerSec / (1024 * 1024 * 1024)).toFixed(1) + ' GB/s'
  }
  if (bytesPerSec >= 1024 * 1024) {
    return (bytesPerSec / (1024 * 1024)).toFixed(1) + ' MB/s'
  }
  if (bytesPerSec >= 1024) {
    return (bytesPerSec / 1024).toFixed(1) + ' KB/s'
  }
  return bytesPerSec + ' B/s'
}

// 格式化磁盘大小
function formatDiskSize(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024 * 1024) {
    return Math.round(bytes / (1024 * 1024 * 1024 * 1024)) + 'TB'
  }
  return Math.round(bytes / (1024 * 1024 * 1024)) + 'GB'
}
</script>

<style lang="scss" scoped>
.system-stats-card {
  background-color: #fff;
  border-radius: 8px;
  padding: 16px 20px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);

  &.loading {
    min-height: 160px;
  }

  .card-header {
    margin-bottom: 16px;

    .title {
      font-size: 16px;
      font-weight: 600;
      color: #303133;
    }
  }

  .card-body {
    display: flex;
    gap: 24px;
  }

  .left-panel {
    flex: 1;
    min-width: 0;
  }

  .right-panel {
    flex: 1;
    min-width: 0;
    border-left: 1px solid #ebeef5;
    padding-left: 24px;
  }

  .stat-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;

    &:last-child {
      margin-bottom: 0;
    }

    .stat-label {
      width: 36px;
      font-size: 14px;
      color: #606266;
      flex-shrink: 0;
    }

    .stat-progress {
      width: 220px;
      flex-shrink: 0;
    }

    .stat-extra {
      width: 100px;
      text-align: right;
      font-size: 13px;
      color: #909399;
      flex-shrink: 0;
    }

    .stat-load {
      flex: 1;
      font-size: 14px;
      color: #303133;
    }

    .stat-network {
      flex: 1;
      display: flex;
      gap: 16px;
      font-size: 14px;

      .upload {
        color: #e6a23c;
      }

      .download {
        color: #67c23a;
      }
    }
  }

  .disk-list {
    display: flex;
    flex-wrap: wrap;
    gap: 16px;

    .disk-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 8px;
      cursor: default;

      .disk-path {
        font-size: 13px;
        color: #606266;
        max-width: 80px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        text-align: center;
      }
    }
  }
}
</style>
