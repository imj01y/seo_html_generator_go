<template>
  <div class="page-container">
    <!-- 页面头部 -->
    <div class="page-header">
      <h2>数据抓取管理</h2>
      <el-button type="primary" @click="handleCreate">
        <el-icon><Plus /></el-icon>
        新建项目
      </el-button>
    </div>

    <!-- 统计卡片 -->
    <el-row :gutter="16" class="stats-cards">
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-value">{{ totalCount }}</div>
          <div class="stat-label">总项目数</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-value">{{ enabledCount }}</div>
          <div class="stat-label">已启用</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-value running">{{ runningCount }}</div>
          <div class="stat-label">运行中</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-value">{{ formatNumber(totalItems) }}</div>
          <div class="stat-label">累计抓取</div>
        </div>
      </el-col>
    </el-row>

    <!-- 搜索栏 -->
    <div class="search-bar">
      <el-input
        v-model="searchText"
        placeholder="搜索项目名称"
        clearable
        style="width: 200px"
        @keyup.enter="handleSearch"
      />
      <el-select v-model="searchStatus" placeholder="状态" clearable style="width: 120px">
        <el-option label="全部" value="" />
        <el-option label="空闲" value="idle" />
        <el-option label="运行中" value="running" />
        <el-option label="错误" value="error" />
      </el-select>
      <el-button type="primary" @click="handleSearch">搜索</el-button>
      <el-button @click="handleReset">重置</el-button>
    </div>

    <!-- 数据表格 -->
    <el-table :data="projects" v-loading="loading" border stripe>
      <el-table-column prop="name" label="名称" width="180">
        <template #default="{ row }">
          <el-link type="primary" @click="handleEdit(row)">{{ row.name }}</el-link>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
      <el-table-column prop="entry_file" label="入口文件" width="120" />
      <el-table-column prop="status" label="状态" width="120" align="center">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" effect="light" class="status-tag">
            <el-icon v-if="row.status === 'running'" class="is-loading"><Loading /></el-icon>
            {{ getStatusText(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="enabled" label="启用" width="80" align="center">
        <template #default="{ row }">
          <el-switch
            :model-value="row.enabled === 1"
            :loading="row.toggling"
            @change="handleToggle(row)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="last_run_at" label="上次运行" width="120">
        <template #default="{ row }">
          {{ formatRelativeTime(row.last_run_at) }}
        </template>
      </el-table-column>
      <el-table-column label="本次/总量" width="120" align="right">
        <template #default="{ row }">
          {{ row.last_run_items || 0 }} / {{ formatNumber(row.total_items) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="380" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.status !== 'running'"
            type="primary"
            size="small"
            @click="handleRun(row)"
          >
            运行
          </el-button>
          <el-button
            v-else
            type="warning"
            size="small"
            @click="handleStop(row)"
          >
            停止
          </el-button>
          <el-button
            v-if="row.status === 'running'"
            type="primary"
            link
            @click="handleViewLogs(row)"
          >
            查看日志
          </el-button>
          <el-button size="small" @click="handleConfig(row)">配置</el-button>
          <el-button size="small" @click="handleEditCode(row)">编辑代码</el-button>
          <el-button
            v-if="row.status !== 'running'"
            type="warning"
            size="small"
            @click="confirmResetProject(row)"
          >
            重置
          </el-button>
          <el-button type="danger" size="small" @click="confirmDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="fetchProjects"
        @current-change="fetchProjects"
      />
    </div>

    <!-- 日志抽屉 -->
    <el-drawer
      v-model="logDrawerVisible"
      :title="`执行日志 - ${currentProject?.name}`"
      direction="rtl"
      size="50%"
    >
      <LogViewer :logs="executionLogs" :show-filters="true" @clear="executionLogs = []" />
    </el-drawer>

    <!-- 配置弹窗 -->
    <el-dialog
      v-model="configDialogVisible"
      :title="configProject ? `配置 - ${configProject.name}` : '项目配置'"
      width="600px"
    >
      <el-form
        v-if="configProject"
        ref="configFormRef"
        :model="configForm"
        label-position="top"
      >
        <el-form-item label="项目名称" prop="name" required>
          <el-input v-model="configForm.name" placeholder="请输入项目名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="configForm.description"
            type="textarea"
            :rows="2"
            placeholder="项目描述（可选）"
          />
        </el-form-item>
        <el-form-item label="入口文件" prop="entry_file">
          <el-select v-model="configForm.entry_file" style="width: 100%">
            <el-option
              v-for="file in configFiles"
              :key="file.path || file.filename"
              :label="file.path || file.filename"
              :value="(file.path || file.filename || '').replace(/^\//, '')"
            />
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="并发数" prop="concurrency">
              <el-input-number
                v-model="configForm.concurrency"
                :min="1"
                :max="10"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="输出分组" prop="output_group_id">
              <el-select v-model="configForm.output_group_id" style="width: 100%">
                <el-option
                  v-for="group in articleGroups"
                  :key="group.id"
                  :label="group.name"
                  :value="group.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="调度规则" prop="schedule">
          <ScheduleBuilder v-model="configForm.schedule" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="configDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="configSaving" @click="handleSaveConfig">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Loading } from '@element-plus/icons-vue'
import {
  getProjects,
  toggleProject,
  runProject,
  stopProject,
  deleteProject,
  resetProject,
  updateProject,
  getProjectFiles,
  subscribeProjectLogs,
  type SpiderProject,
  type ProjectFile
} from '@/api/spiderProjects'
import { getArticleGroups } from '@/api/articles'
import LogViewer from '@/components/LogViewer.vue'
import ScheduleBuilder from '@/components/ScheduleBuilder.vue'

const router = useRouter()
const route = useRoute()

// 数据状态
const projects = ref<SpiderProject[]>([])
const loading = ref(false)
const total = ref(0)
const currentPage = ref(Number(route.query.page) || 1)
const pageSize = ref(Number(route.query.pageSize) || 20)

// 同步分页状态到 URL
const updateUrlQuery = () => {
  router.replace({
    query: {
      ...route.query,
      page: currentPage.value.toString(),
      pageSize: pageSize.value.toString()
    }
  })
}

// 监听分页变化，同步到 URL
watch([currentPage, pageSize], () => {
  updateUrlQuery()
})

// 搜索条件
const searchText = ref('')
const searchStatus = ref('')

// 日志抽屉
const logDrawerVisible = ref(false)
const currentProject = ref<SpiderProject | null>(null)
const executionLogs = ref<{ time: string; level: string; message: string }[]>([])
let unsubscribeLogs: (() => void) | null = null

// 配置弹窗
const configDialogVisible = ref(false)
const configProject = ref<SpiderProject | null>(null)
const configFiles = ref<ProjectFile[]>([])
const configSaving = ref(false)
const configFormRef = ref()
const configForm = ref({
  name: '',
  description: '',
  entry_file: '',
  concurrency: 3,
  output_group_id: 1,
  schedule: ''
})

// 文章分组
const articleGroups = ref<{ id: number; name: string }[]>([])

// 加载文章分组
async function loadArticleGroups() {
  try {
    const res = await getArticleGroups()
    articleGroups.value = res || []
  } catch {
    articleGroups.value = [{ id: 1, name: '默认文章分组' }]
  }
}

// 计算统计数据
const totalCount = computed(() => total.value)
const enabledCount = computed(() => projects.value.filter(p => p.enabled === 1).length)
const runningCount = computed(() => projects.value.filter(p => p.status === 'running').length)
const totalItems = computed(() => projects.value.reduce((sum, p) => sum + p.total_items, 0))

// 加载项目列表
async function fetchProjects() {
  loading.value = true
  try {
    const { items, total: t } = await getProjects({
      search: searchText.value || undefined,
      status: searchStatus.value || undefined,
      page: currentPage.value,
      page_size: pageSize.value
    })
    projects.value = items
    total.value = t
  } catch (e: any) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

// 搜索
function handleSearch() {
  currentPage.value = 1
  fetchProjects()
}

function handleReset() {
  searchText.value = ''
  searchStatus.value = ''
  currentPage.value = 1
  fetchProjects()
}

// 创建项目
function handleCreate() {
  router.push('/spiders/projects/create')
}

// 编辑项目（跳转到代码页）
function handleEdit(row: SpiderProject) {
  router.push(`/spiders/projects/${row.id}/code`)
}

// 编辑代码
function handleEditCode(row: SpiderProject) {
  router.push(`/spiders/projects/${row.id}/code`)
}

// 打开配置弹窗
async function handleConfig(row: SpiderProject) {
  configProject.value = row
  configForm.value = {
    name: row.name,
    description: row.description || '',
    entry_file: row.entry_file,
    concurrency: row.concurrency,
    output_group_id: row.output_group_id,
    schedule: row.schedule || ''
  }

  // 加载文件列表
  try {
    configFiles.value = await getProjectFiles(row.id)
  } catch {
    configFiles.value = []
  }

  configDialogVisible.value = true
}

// 保存配置
async function handleSaveConfig() {
  if (!configProject.value) return

  if (!configForm.value.name.trim()) {
    ElMessage.warning('请输入项目名称')
    return
  }

  configSaving.value = true
  try {
    await updateProject(configProject.value.id, {
      name: configForm.value.name,
      description: configForm.value.description || undefined,
      entry_file: configForm.value.entry_file,
      concurrency: configForm.value.concurrency,
      output_group_id: configForm.value.output_group_id,
      schedule: configForm.value.schedule || undefined
    })

    ElMessage.success('保存成功')
    configDialogVisible.value = false
    fetchProjects()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    configSaving.value = false
  }
}

// 切换启用状态
async function handleToggle(row: SpiderProject) {
  row.toggling = true
  try {
    const res = await toggleProject(row.id)
    row.enabled = res.enabled
    ElMessage.success(res.message || '操作成功')
  } catch (e: any) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    row.toggling = false
  }
}

// 运行项目
async function handleRun(row: SpiderProject) {
  try {
    await runProject(row.id)
    row.status = 'running'
    ElMessage.success('任务已启动')

    // 打开日志抽屉
    currentProject.value = row
    executionLogs.value = []
    logDrawerVisible.value = true

    // 订阅日志
    unsubscribeLogs?.()
    unsubscribeLogs = subscribeProjectLogs(
      row.id,
      (level, message) => {
        executionLogs.value.push({
          time: new Date().toLocaleTimeString(),
          level,
          message
        })
      },
      () => {
        row.status = 'idle'
        fetchProjects()
      },
      (error) => {
        ElMessage.error(error)
      }
    )
  } catch (e: any) {
    ElMessage.error(e.message || '启动失败')
  }
}

// 查看日志
function handleViewLogs(row: SpiderProject) {
  // 如果是同一个项目且已有日志，直接打开抽屉
  if (currentProject.value?.id === row.id) {
    logDrawerVisible.value = true
    return
  }

  // 如果是不同项目，先取消之前的订阅
  unsubscribeLogs?.()

  // 设置当前项目并清空日志
  currentProject.value = row
  executionLogs.value = []
  logDrawerVisible.value = true

  // 重新订阅日志
  unsubscribeLogs = subscribeProjectLogs(
    row.id,
    (level, message) => {
      executionLogs.value.push({
        time: new Date().toLocaleTimeString(),
        level,
        message
      })
    },
    () => {
      row.status = 'idle'
      fetchProjects()
    },
    (error) => {
      ElMessage.error(error)
    }
  )
}

// 停止项目
async function handleStop(row: SpiderProject) {
  try {
    await stopProject(row.id)
    row.status = 'idle'
    ElMessage.success('已发送停止信号')
  } catch (e: any) {
    ElMessage.error(e.message || '停止失败')
  }
}

// 确认重置项目队列
async function confirmResetProject(row: SpiderProject) {
  try {
    await ElMessageBox.confirm(
      `确定要重置项目「${row.name}」吗？\n\n此操作将清空：\n• 待抓取队列\n• 已抓取URL记录（去重队列）\n• 统计数据\n\n重置后项目将从头开始抓取。`,
      '重置确认',
      {
        confirmButtonText: '确定重置',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await handleResetProject(row)
  } catch {
    // 用户取消
  }
}

// 重置项目队列
async function handleResetProject(row: SpiderProject) {
  try {
    const res = await resetProject(row.id)
    ElMessage.success(res.message || '重置成功')
  } catch (e: any) {
    ElMessage.error(e.message || '重置失败')
  }
}

// 确认删除
async function confirmDelete(row: SpiderProject) {
  try {
    await ElMessageBox.confirm(
      `确定要删除项目「${row.name}」吗？`,
      '删除确认',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await handleDelete(row)
  } catch {
    // 用户取消
  }
}

// 删除项目
async function handleDelete(row: SpiderProject) {
  try {
    await deleteProject(row.id)
    ElMessage.success('删除成功')
    fetchProjects()
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}

// 辅助函数
function getStatusType(status: string) {
  switch (status) {
    case 'running': return 'primary'
    case 'error': return 'danger'
    default: return 'info'
  }
}

function getStatusText(status: string) {
  switch (status) {
    case 'running': return '运行中'
    case 'error': return '错误'
    default: return '空闲'
  }
}

function formatRelativeTime(dateStr: string | null) {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(diff / 86400000)

  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  if (hours < 24) return `${hours}小时前`
  return `${days}天前`
}

function formatNumber(num: number) {
  if (num >= 10000) {
    return (num / 10000).toFixed(1) + '万'
  }
  return num.toString()
}

// 生命周期
onMounted(() => {
  fetchProjects()
  loadArticleGroups()
})

onUnmounted(() => {
  unsubscribeLogs?.()
})
</script>

<style scoped>
.page-container {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.stats-cards {
  margin-bottom: 20px;
}

.stat-card {
  background: #fff;
  border-radius: 4px;
  padding: 16px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.1);
  text-align: center;
}

.stat-value {
  font-size: 24px;
  font-weight: bold;
  color: #303133;
}

.stat-value.running {
  color: #409EFF;
}

.stat-label {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.search-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  align-items: center;
}

.pagination-container {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.status-tag :deep(.el-tag__content) {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

/* 固定列背景色与普通列保持一致 */
:deep(.el-table__fixed-right-patch),
:deep(.el-table__fixed-right) {
  background: transparent !important;
}

:deep(.el-table td.el-table-fixed-column--right) {
  background: #fff !important;
}

:deep(.el-table__row--striped td.el-table-fixed-column--right) {
  background: #fafafa !important;
}

:deep(.el-table__row:hover td.el-table-fixed-column--right) {
  background: var(--el-table-row-hover-bg-color) !important;
}
</style>
