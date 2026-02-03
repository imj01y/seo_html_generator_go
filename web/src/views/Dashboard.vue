<template>
  <div class="dashboard">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="12" :sm="12" :md="6">
        <div class="stat-card cursor-pointer">
          <div class="stat-icon" style="background-color: #409eff">
            <el-icon :size="28"><Monitor /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.sites_count }}</div>
            <div class="stat-label">站点数量</div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <div class="stat-card cursor-pointer">
          <div class="stat-icon" style="background-color: #67c23a">
            <el-icon :size="28"><Key /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">
              {{ formatNumber(stats.keywords_total) }}
              <span class="stat-memory">{{ formatMemoryMB(stats.keyword_group_stats?.memory_mb || 0) }}</span>
            </div>
            <div class="stat-label">关键词总数</div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <div class="stat-card cursor-pointer">
          <div class="stat-icon" style="background-color: #e6a23c">
            <el-icon :size="28"><Picture /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">
              {{ formatNumber(stats.images_total) }}
              <span class="stat-memory">{{ formatMemoryMB(stats.image_group_stats?.memory_mb || 0) }}</span>
            </div>
            <div class="stat-label">图片总数</div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <div class="stat-card cursor-pointer">
          <div class="stat-icon" style="background-color: #f56c6c">
            <el-icon :size="28"><Document /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ formatNumber(stats.articles_total) }}</div>
            <div class="stat-label">文章总数</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 系统资源卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="24">
        <SystemStatsCard :stats="systemStats" />
      </el-col>
    </el-row>

    <!-- 图表区域 -->
    <el-row :gutter="20">
      <el-col :xs="24" :lg="16">
        <div class="card">
          <div class="card-header">
            <span class="title">蜘蛛访问趋势（近7天）</span>
          </div>
          <div ref="trendChartRef" class="chart"></div>
        </div>
      </el-col>
      <el-col :xs="24" :lg="8">
        <div class="card">
          <div class="card-header">
            <span class="title">蜘蛛类型分布</span>
          </div>
          <div ref="pieChartRef" class="chart"></div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import * as echarts from 'echarts'
import { getDashboardStats, getSpiderStats } from '@/api/dashboard'
import { getDailyStats } from '@/api/spiders'
import { formatMemoryMB, formatNumber } from '@/utils/format'
import type { DashboardStats } from '@/types'
import SystemStatsCard from '@/components/SystemStatsCard.vue'
import { connectSystemStatsWs, disconnectSystemStatsWs } from '@/api/system-stats'
import type { SystemStats } from '@/types/system-stats'

const stats = reactive<DashboardStats>({
  sites_count: 0,
  keywords_total: 0,
  images_total: 0,
  articles_total: 0,
  keyword_group_stats: { total: 0, cursor: 0, remaining: 0, loaded: false, memory_mb: 0 },
  image_group_stats: { total: 0, cursor: 0, remaining: 0, loaded: false, memory_mb: 0 }
})

const systemStats = ref<SystemStats | null>(null)

const trendChartRef = ref<HTMLElement>()
const pieChartRef = ref<HTMLElement>()
let trendChart: echarts.ECharts | null = null
let pieChart: echarts.ECharts | null = null

// formatNumber 和 formatMemoryMB 从 @/utils/format 导入

const loadStats = async () => {
  try {
    const data = await getDashboardStats()
    Object.assign(stats, data)
  } catch {
    // 错误已处理
  }
}

const loadTrendChart = async () => {
  try {
    const dailyStats = await getDailyStats(7)
    if (!trendChartRef.value) return

    trendChart = echarts.init(trendChartRef.value)

    const dates = dailyStats.map(d => d.date)
    const totals = dailyStats.map(d => d.total)

    const series: echarts.SeriesOption[] = [
      {
        name: '总访问',
        type: 'line',
        data: totals,
        smooth: true,
        areaStyle: { opacity: 0.3 }
      }
    ]

    trendChart.setOption({
      tooltip: { trigger: 'axis' },
      legend: { data: ['总访问'] },
      grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
      xAxis: { type: 'category', data: dates },
      yAxis: { type: 'value' },
      series
    })
  } catch {
    // 错误已处理
  }
}

const loadPieChart = async () => {
  try {
    const spiderStats = await getSpiderStats()
    if (!pieChartRef.value) return

    pieChart = echarts.init(pieChartRef.value)

    const pieData = Object.entries(spiderStats.by_spider).map(([name, value]) => ({
      name,
      value
    }))

    pieChart.setOption({
      tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
      legend: { orient: 'vertical', right: 10, top: 'center' },
      series: [
        {
          type: 'pie',
          radius: ['40%', '70%'],
          center: ['40%', '50%'],
          data: pieData,
          emphasis: {
            itemStyle: {
              shadowBlur: 10,
              shadowOffsetX: 0,
              shadowColor: 'rgba(0, 0, 0, 0.5)'
            }
          }
        }
      ]
    })
  } catch {
    // 错误已处理
  }
}

const handleResize = () => {
  trendChart?.resize()
  pieChart?.resize()
}

onMounted(() => {
  loadStats()
  loadTrendChart()
  loadPieChart()
  window.addEventListener('resize', handleResize)
  connectSystemStatsWs((data) => {
    systemStats.value = data
  })
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  trendChart?.dispose()
  pieChart?.dispose()
  disconnectSystemStatsWs()
})
</script>

<style lang="scss" scoped>
.dashboard {
  .stats-row {
    margin-bottom: 20px;
  }

  .stat-card {
    display: flex;
    align-items: center;
    padding: 20px;
    background-color: #fff;
    border-radius: 8px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);
    transition: transform 0.2s ease, box-shadow 0.2s ease;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);
    }

    .stat-icon {
      width: 60px;
      height: 60px;
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      color: #fff;
    }

    .stat-info {
      margin-left: 16px;

      .stat-value {
        font-size: 28px;
        font-weight: 600;
        color: #303133;

        .stat-memory {
          font-size: 14px;
          font-weight: 400;
          color: #909399;
          margin-left: 8px;
        }
      }

      .stat-label {
        font-size: 14px;
        color: #909399;
        margin-top: 4px;
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
      margin-bottom: 16px;

      .title {
        font-size: 16px;
        font-weight: 600;
        color: #303133;
      }
    }

    .chart {
      height: 300px;
    }
  }
}
</style>
