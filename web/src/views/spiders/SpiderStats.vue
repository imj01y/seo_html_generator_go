<template>
  <div class="spider-stats">
    <!-- 顶部筛选栏 -->
    <el-card class="filter-card" shadow="never">
      <div class="filter-bar">
        <div class="filter-left">
          <el-select
            v-model="selectedProjectId"
            placeholder="选择项目"
            clearable
            style="width: 200px"
            @change="handleFilterChange"
          >
            <el-option :value="0" label="全部项目" />
            <el-option
              v-for="project in projects"
              :key="project.id"
              :value="project.id"
              :label="project.name"
            />
          </el-select>
          <el-radio-group v-model="periodType" @change="handleFilterChange">
            <el-radio-button value="minute">分钟</el-radio-button>
            <el-radio-button value="hour">小时</el-radio-button>
            <el-radio-button value="day">天</el-radio-button>
            <el-radio-button value="month">月</el-radio-button>
          </el-radio-group>
        </div>
        <div class="filter-right">
          <el-date-picker
            v-model="dateRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            value-format="YYYY-MM-DDTHH:mm:ss"
            @change="handleFilterChange"
          />
          <el-button :icon="Refresh" @click="loadData">刷新</el-button>
        </div>
      </div>
    </el-card>

    <!-- 统计卡片 -->
    <div class="stats-cards">
      <el-card class="stat-card" shadow="hover">
        <div class="stat-content">
          <div class="stat-icon total">
            <el-icon :size="24"><DataLine /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ formatNumber(overview.total) }}</div>
            <div class="stat-label">总请求</div>
          </div>
        </div>
      </el-card>
      <el-card class="stat-card" shadow="hover">
        <div class="stat-content">
          <div class="stat-icon success">
            <el-icon :size="24"><SuccessFilled /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">
              {{ formatNumber(overview.completed) }}
              <span class="stat-rate success">{{ overview.success_rate }}%</span>
            </div>
            <div class="stat-label">成功</div>
          </div>
        </div>
      </el-card>
      <el-card class="stat-card" shadow="hover">
        <div class="stat-content">
          <div class="stat-icon error">
            <el-icon :size="24"><CircleCloseFilled /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ formatNumber(overview.failed) }}</div>
            <div class="stat-label">失败</div>
          </div>
        </div>
      </el-card>
      <el-card class="stat-card" shadow="hover">
        <div class="stat-content">
          <div class="stat-icon warning">
            <el-icon :size="24"><RefreshRight /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ formatNumber(overview.retried) }}</div>
            <div class="stat-label">重试</div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 图表区域 -->
    <div class="charts-row">
      <!-- 趋势图 -->
      <el-card class="chart-card trend-card" shadow="never">
        <template #header>
          <span>请求趋势</span>
        </template>
        <div ref="trendChartRef" class="chart-container" />
      </el-card>

      <!-- 饼图 -->
      <el-card class="chart-card pie-card" shadow="never">
        <template #header>
          <span>状态分布</span>
        </template>
        <div ref="pieChartRef" class="chart-container" />
      </el-card>
    </div>

    <!-- 项目明细表（仅全部项目视图） -->
    <el-card v-if="selectedProjectId === 0" class="table-card" shadow="never">
      <template #header>
        <span>各项目统计</span>
      </template>
      <el-table :data="projectStats" stripe>
        <el-table-column prop="project_name" label="项目名称" min-width="150" />
        <el-table-column prop="total" label="总数" width="100" align="right">
          <template #default="{ row }">{{ formatNumber(row.total) }}</template>
        </el-table-column>
        <el-table-column prop="completed" label="成功" width="100" align="right">
          <template #default="{ row }">
            <span class="text-success">{{ formatNumber(row.completed) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="failed" label="失败" width="100" align="right">
          <template #default="{ row }">
            <span class="text-danger">{{ formatNumber(row.failed) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="retried" label="重试" width="100" align="right">
          <template #default="{ row }">
            <span class="text-warning">{{ formatNumber(row.retried) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="success_rate" label="成功率" width="100" align="right">
          <template #default="{ row }">
            <el-tag :type="getSuccessRateType(row.success_rate)" size="small">
              {{ row.success_rate }}%
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { Refresh, DataLine, SuccessFilled, CircleCloseFilled, RefreshRight } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { getProjects, getStatsOverview, getChartStats, getStatsByProject } from '@/api/spiderProjects'
import type { SpiderProject } from '@/api/spiderProjects'
import type { SpiderStatsOverview, SpiderChartDataPoint, SpiderStatsByProject } from '@/types'

// 筛选条件
const selectedProjectId = ref<number>(0)
const periodType = ref<string>('hour')
const dateRange = ref<[string, string] | null>(null)

// 数据
const projects = ref<SpiderProject[]>([])
const overview = ref<SpiderStatsOverview>({
  total: 0,
  completed: 0,
  failed: 0,
  retried: 0,
  success_rate: 0,
  avg_speed: 0,
})
const chartData = ref<SpiderChartDataPoint[]>([])
const projectStats = ref<SpiderStatsByProject[]>([])

// 图表引用
const trendChartRef = ref<HTMLElement | null>(null)
const pieChartRef = ref<HTMLElement | null>(null)
let trendChart: echarts.ECharts | null = null
let pieChart: echarts.ECharts | null = null

// 格式化数字
const formatNumber = (num: number): string => {
  if (num >= 10000) {
    return (num / 10000).toFixed(1) + 'w'
  }
  return num.toLocaleString()
}

// 获取成功率标签类型
const getSuccessRateType = (rate: number): 'success' | 'warning' | 'danger' => {
  if (rate >= 95) return 'success'
  if (rate >= 80) return 'warning'
  return 'danger'
}

// 加载项目列表
const loadProjects = async () => {
  try {
    const res = await getProjects({ page_size: 100 })
    projects.value = res.items
  } catch (error) {
    console.error('Failed to load projects:', error)
  }
}

// 加载统计数据
const loadData = async () => {
  const params: Record<string, any> = {
    project_id: selectedProjectId.value || undefined,
    period: periodType.value,
  }

  if (dateRange.value) {
    params.start = dateRange.value[0]
    params.end = dateRange.value[1]
  }

  try {
    // 并行加载数据
    const [overviewRes, chartRes] = await Promise.all([
      getStatsOverview(params),
      getChartStats({ ...params, limit: 100 }),
    ])

    overview.value = overviewRes
    chartData.value = chartRes

    // 如果选择全部项目，加载按项目统计
    if (selectedProjectId.value === 0) {
      projectStats.value = await getStatsByProject({
        period: periodType.value,
        start: dateRange.value?.[0],
        end: dateRange.value?.[1],
      })
    }

    // 更新图表
    await nextTick()
    updateTrendChart()
    updatePieChart()
  } catch (error) {
    console.error('Failed to load stats:', error)
  }
}

// 处理筛选条件变化
const handleFilterChange = () => {
  loadData()
}

// 更新趋势图
const updateTrendChart = () => {
  if (!trendChartRef.value) return

  trendChart = trendChart || echarts.init(trendChartRef.value)

  const times = chartData.value.map(d => {
    const date = new Date(d.time)
    if (periodType.value === 'minute') {
      return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    } else if (periodType.value === 'hour') {
      return date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit' })
    } else if (periodType.value === 'day') {
      return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
    } else {
      return date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit' })
    }
  })

  const option: echarts.EChartsOption = {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross' },
    },
    legend: {
      data: ['成功', '失败', '重试'],
      bottom: 0,
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '15%',
      top: '10%',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      data: times,
      axisLabel: {
        rotate: 30,
        fontSize: 10,
      },
    },
    yAxis: {
      type: 'value',
    },
    series: [
      {
        name: '成功',
        type: 'line',
        smooth: true,
        data: chartData.value.map(d => d.completed),
        itemStyle: { color: '#67c23a' },
        areaStyle: { color: 'rgba(103, 194, 58, 0.1)' },
      },
      {
        name: '失败',
        type: 'line',
        smooth: true,
        data: chartData.value.map(d => d.failed),
        itemStyle: { color: '#f56c6c' },
        areaStyle: { color: 'rgba(245, 108, 108, 0.1)' },
      },
      {
        name: '重试',
        type: 'line',
        smooth: true,
        data: chartData.value.map(d => d.retried),
        itemStyle: { color: '#e6a23c' },
        areaStyle: { color: 'rgba(230, 162, 60, 0.1)' },
      },
    ],
  }

  trendChart.setOption(option)
}

// 更新饼图
const updatePieChart = () => {
  if (!pieChartRef.value) return

  pieChart = pieChart || echarts.init(pieChartRef.value)

  const data = [
    { value: overview.value.completed, name: '成功', itemStyle: { color: '#67c23a' } },
    { value: overview.value.failed, name: '失败', itemStyle: { color: '#f56c6c' } },
    { value: overview.value.retried, name: '重试', itemStyle: { color: '#e6a23c' } },
  ].filter(d => d.value > 0)

  const option: echarts.EChartsOption = {
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c} ({d}%)',
    },
    legend: {
      orient: 'vertical',
      right: '10%',
      top: 'center',
    },
    series: [
      {
        type: 'pie',
        radius: ['40%', '70%'],
        center: ['35%', '50%'],
        avoidLabelOverlap: false,
        label: {
          show: false,
          position: 'center',
        },
        emphasis: {
          label: {
            show: true,
            fontSize: 16,
            fontWeight: 'bold',
          },
        },
        labelLine: {
          show: false,
        },
        data,
      },
    ],
  }

  pieChart.setOption(option)
}

// 窗口大小变化时重绘图表
const handleResize = () => {
  trendChart?.resize()
  pieChart?.resize()
}

onMounted(async () => {
  await loadProjects()
  await loadData()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  trendChart?.dispose()
  pieChart?.dispose()
})
</script>

<style lang="scss" scoped>
.spider-stats {
  .filter-card {
    margin-bottom: 16px;

    .filter-bar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      flex-wrap: wrap;
      gap: 12px;

      .filter-left {
        display: flex;
        align-items: center;
        gap: 12px;
      }

      .filter-right {
        display: flex;
        align-items: center;
        gap: 12px;
      }
    }
  }

  .stats-cards {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
    margin-bottom: 16px;

    @media (max-width: 1200px) {
      grid-template-columns: repeat(2, 1fr);
    }

    @media (max-width: 600px) {
      grid-template-columns: 1fr;
    }

    .stat-card {
      .stat-content {
        display: flex;
        align-items: center;
        gap: 16px;

        .stat-icon {
          width: 48px;
          height: 48px;
          border-radius: 8px;
          display: flex;
          align-items: center;
          justify-content: center;
          color: #fff;

          &.total {
            background: linear-gradient(135deg, #409eff 0%, #66b1ff 100%);
          }

          &.success {
            background: linear-gradient(135deg, #67c23a 0%, #85ce61 100%);
          }

          &.error {
            background: linear-gradient(135deg, #f56c6c 0%, #f78989 100%);
          }

          &.warning {
            background: linear-gradient(135deg, #e6a23c 0%, #ebb563 100%);
          }
        }

        .stat-info {
          .stat-value {
            font-size: 24px;
            font-weight: 600;
            color: #303133;
            line-height: 1.2;

            .stat-rate {
              font-size: 12px;
              font-weight: normal;
              margin-left: 4px;

              &.success {
                color: #67c23a;
              }
            }
          }

          .stat-label {
            font-size: 14px;
            color: #909399;
            margin-top: 4px;
          }
        }
      }
    }
  }

  .charts-row {
    display: grid;
    grid-template-columns: 2fr 1fr;
    gap: 16px;
    margin-bottom: 16px;

    @media (max-width: 1200px) {
      grid-template-columns: 1fr;
    }

    .chart-card {
      .chart-container {
        height: 300px;
      }
    }
  }

  .table-card {
    .text-success {
      color: #67c23a;
    }

    .text-danger {
      color: #f56c6c;
    }

    .text-warning {
      color: #e6a23c;
    }
  }
}
</style>
