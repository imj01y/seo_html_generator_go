<template>
  <div class="spider-logs">
    <div class="page-header">
      <h2 class="title">蜘蛛日志</h2>
      <el-button type="danger" @click="handleClearLogs">
        <el-icon><Delete /></el-icon>
        清理旧日志
      </el-button>
    </div>

    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="24" :sm="8" :md="6">
        <div class="stat-card">
          <div class="stat-value">{{ stats.total_visits }}</div>
          <div class="stat-label">总访问次数</div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="16" :md="18">
        <div class="stat-card spider-stats">
          <div class="spider-item" v-for="(count, spider) in stats.by_spider" :key="spider">
            <span class="spider-name">{{ spider }}</span>
            <span class="spider-count">{{ count }}</span>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 图表 -->
    <el-row :gutter="20" class="chart-row">
      <el-col :xs="24" :lg="16">
        <div class="card">
          <div class="card-header">
            <span class="title">访问趋势</span>
            <el-radio-group v-model="periodType" size="small" @change="loadChart">
              <el-radio-button label="minute">分钟</el-radio-button>
              <el-radio-button label="hour">小时</el-radio-button>
              <el-radio-button label="day">天</el-radio-button>
              <el-radio-button label="month">月</el-radio-button>
            </el-radio-group>
          </div>
          <div ref="chartRef" class="chart"></div>
        </div>
      </el-col>
      <el-col :xs="24" :lg="8">
        <div class="card">
          <div class="card-header">
            <span class="title">蜘蛛分布</span>
          </div>
          <div ref="pieChartRef" class="chart"></div>
        </div>
      </el-col>
    </el-row>

    <!-- 日志列表 -->
    <div class="card">
      <div class="card-header">
        <span class="title">访问日志</span>
      </div>

      <!-- 筛选 -->
      <el-form :inline="true" class="search-form">
        <el-form-item label="蜘蛛类型">
          <el-select v-model="filter.spider_type" placeholder="全部" clearable @change="loadLogs">
            <el-option label="Baiduspider" value="Baiduspider" />
            <el-option label="Googlebot" value="Googlebot" />
            <el-option label="bingbot" value="bingbot" />
            <el-option label="Sogou" value="Sogou" />
            <el-option label="360Spider" value="360Spider" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态码">
          <el-select v-model="filter.status_code" placeholder="全部" clearable @change="loadLogs">
            <el-option label="200" :value="200" />
            <el-option label="301" :value="301" />
            <el-option label="302" :value="302" />
            <el-option label="404" :value="404" />
            <el-option label="500" :value="500" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            @change="handleDateChange"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadLogs">查询</el-button>
        </el-form-item>
      </el-form>

      <!-- 表格 -->
      <el-table :data="logs" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="spider_type" label="蜘蛛" width="110">
          <template #default="{ row }">
            <el-tag size="small">{{ row.spider_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="domain" label="域名" width="160">
          <template #default="{ row }">
            <el-link v-if="row.domain" :href="`http://${row.domain}`" target="_blank" type="primary">
              {{ row.domain }}
            </el-link>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="path" label="请求路径" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="path-text">{{ row.path }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态码" width="90">
          <template #default="{ row }">
            <el-tag
              :type="getStatusType(row.status)"
              size="small"
            >
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="resp_time" label="响应" width="80">
          <template #default="{ row }">
            {{ row.resp_time }}ms
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="130" />
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        class="pagination"
        @size-change="loadLogs"
        @current-change="loadLogs"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as echarts from 'echarts'
import dayjs from 'dayjs'
import {
  getSpiderLogs,
  getSpiderStats,
  getSpiderTrend,
  clearOldLogs
} from '@/api/spiders'
import type { SpiderLog, SpiderStats } from '@/types'

const loading = ref(false)
const chartRef = ref<HTMLElement>()
const pieChartRef = ref<HTMLElement>()
let chart: echarts.ECharts | null = null
let pieChart: echarts.ECharts | null = null

const periodType = ref<'minute' | 'hour' | 'day' | 'month'>('hour')
const dateRange = ref<[string, string] | null>(null)

const stats = reactive<SpiderStats>({
  total_visits: 0,
  by_spider: {},
  by_site: {},
  by_status: {}
})

const logs = ref<SpiderLog[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const filter = reactive({
  spider_type: '',
  status_code: undefined as number | undefined,
  start_date: '',
  end_date: ''
})

const formatDate = (dateStr: string): string => {
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss')
}

const getStatusType = (code: number): 'success' | 'warning' | 'danger' => {
  if (code >= 200 && code < 300) return 'success'
  if (code >= 300 && code < 400) return 'warning'
  return 'danger'
}

const handleDateChange = (val: [string, string] | null) => {
  if (val) {
    filter.start_date = val[0]
    filter.end_date = val[1]
  } else {
    filter.start_date = ''
    filter.end_date = ''
  }
  loadLogs()
}

const loadStats = async () => {
  try {
    const data = await getSpiderStats()
    Object.assign(stats, data)
  } catch {
    // 错误已处理
  }
}

const loadLogs = async () => {
  loading.value = true
  try {
    const res = await getSpiderLogs({
      page: currentPage.value,
      page_size: pageSize.value,
      spider_type: filter.spider_type || undefined,
      status_code: filter.status_code,
      start_date: filter.start_date || undefined,
      end_date: filter.end_date || undefined
    })
    logs.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

const periodLabels: Record<string, string> = {
  minute: '分钟',
  hour: '小时',
  day: '天',
  month: '月'
}

const loadChart = async () => {
  if (!chartRef.value) return

  chart = chart || echarts.init(chartRef.value)

  try {
    const requestedPeriod = periodType.value
    const response = await getSpiderTrend({ period: periodType.value, limit: 100 })
    const trendData = response.items || []

    // 根据实际返回的 period 更新（可能发生了回退）
    if (response.period !== requestedPeriod) {
      ElMessage.info(`暂无${periodLabels[requestedPeriod]}数据，已回退到${periodLabels[response.period]}`)
      periodType.value = response.period as typeof periodType.value
    }

    // 格式化时间标签
    const formatTime = (timeStr: string): string => {
      const date = new Date(timeStr)
      switch (periodType.value) {
        case 'minute':
          return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
        case 'hour':
          return date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit' })
        case 'day':
          return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
        case 'month':
          return date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit' })
        default:
          return timeStr
      }
    }

    // 根据粒度选择图表类型
    const chartTypeMap: Record<string, 'line' | 'bar'> = {
      'minute': 'line',
      'hour': 'bar',
      'day': 'line',
      'month': 'bar'
    }
    const seriesType = chartTypeMap[periodType.value] || 'line'

    chart.setOption({
      tooltip: { trigger: 'axis' },
      xAxis: {
        type: 'category',
        data: trendData.map(d => formatTime(d.time))
      },
      yAxis: { type: 'value' },
      series: [{
        name: '访问量',
        type: seriesType,
        data: trendData.map(d => d.total),
        smooth: seriesType === 'line',
        areaStyle: seriesType === 'line' ? { opacity: 0.3 } : undefined
      }]
    }, true)
  } catch {
    // 错误已处理
  }
}

const loadPieChart = async () => {
  if (!pieChartRef.value) return

  pieChart = pieChart || echarts.init(pieChartRef.value)

  const pieData = Object.entries(stats.by_spider).map(([name, value]) => ({
    name,
    value
  }))

  pieChart.setOption({
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { orient: 'vertical', right: 10, top: 'center' },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      center: ['40%', '50%'],
      data: pieData
    }]
  })
}

const handleClearLogs = () => {
  ElMessageBox.prompt('请输入要保留的天数（清理该天数之前的日志）', '清理旧日志', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputValue: '30',
    inputPattern: /^\d+$/,
    inputErrorMessage: '请输入有效的天数'
  }).then(async ({ value }) => {
    try {
      const days = parseInt(value)
      const res = await clearOldLogs(days)
      ElMessage.success(`已清理 ${res.deleted} 条日志`)
      loadLogs()
      loadStats()
    } catch (e) {
      ElMessage.warning((e as Error).message || '功能暂未实现')
    }
  })
}

const handleResize = () => {
  chart?.resize()
  pieChart?.resize()
}

onMounted(async () => {
  await loadStats()
  loadLogs()
  loadChart()
  loadPieChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  chart?.dispose()
  pieChart?.dispose()
})
</script>

<style lang="scss" scoped>
.spider-logs {
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

  .stats-row {
    margin-bottom: 20px;
  }

  .stat-card {
    background-color: #fff;
    border-radius: 8px;
    padding: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);

    .stat-value {
      font-size: 32px;
      font-weight: 600;
      color: #303133;
    }

    .stat-label {
      font-size: 14px;
      color: #909399;
      margin-top: 4px;
    }

    &.spider-stats {
      display: flex;
      flex-wrap: wrap;
      gap: 20px;

      .spider-item {
        display: flex;
        flex-direction: column;
        align-items: center;
        min-width: 80px;

        .spider-name {
          font-size: 12px;
          color: #909399;
        }

        .spider-count {
          font-size: 20px;
          font-weight: 600;
          color: #303133;
        }
      }
    }
  }

  .chart-row {
    margin-bottom: 20px;
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

  .path-text {
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: inline-block;
  }

  .text-muted {
    color: #909399;
    font-size: 13px;
  }

  .pagination {
    margin-top: 20px;
    justify-content: flex-end;
  }
}
</style>
