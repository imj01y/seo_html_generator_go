<template>
  <div class="list-page template-list">
    <div class="page-header">
      <h2 class="title">模板管理</h2>
      <el-button type="primary" @click="handleAdd">
        <el-icon><Plus /></el-icon>
        新增模板
      </el-button>
    </div>

    <div class="card">
      <!-- 搜索栏 -->
      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="searchStatus" placeholder="全部" clearable @change="loadTemplates">
            <el-option label="启用" :value="1" />
            <el-option label="禁用" :value="0" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadTemplates">搜索</el-button>
        </el-form-item>
      </el-form>

      <!-- 表格 -->
      <el-table :data="templates" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="模板标识" width="140" />
        <el-table-column prop="display_name" label="显示名称" min-width="120" />
        <el-table-column prop="description" label="描述" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ row.description || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="sites_count" label="绑定站点" width="100">
          <template #default="{ row }">
            <el-button
              v-if="row.sites_count > 0"
              type="primary"
              link
              @click="showSites(row)"
            >
              {{ row.sites_count }} 个
            </el-button>
            <span v-else class="text-muted">0 个</span>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="版本" width="80">
          <template #default="{ row }">
            <el-tag size="small">v{{ row.version }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="80">
          <template #default="{ row }">
            <el-switch
              v-model="row.status"
              :active-value="1"
              :inactive-value="0"
              @change="handleStatusChange(row)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="170">
          <template #default="{ row }">
            {{ formatDate(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button type="danger" size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next, jumper"
        class="pagination"
        @size-change="loadTemplates"
        @current-change="loadTemplates"
      />
    </div>

    <!-- 新增模板弹窗 -->
    <el-dialog
      v-model="addDialogVisible"
      title="新增模板"
      width="500px"
      @close="resetAddForm"
    >
      <el-form
        ref="addFormRef"
        :model="addForm"
        :rules="addRules"
        label-width="100px"
      >
        <el-form-item label="模板标识" prop="name">
          <el-input v-model="addForm.name" placeholder="如：download_site" />
          <div class="form-tip">唯一标识，创建后不可修改</div>
        </el-form-item>
        <el-form-item label="显示名称" prop="display_name">
          <el-input v-model="addForm.display_name" placeholder="如：下载站模板" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="addForm.description"
            type="textarea"
            :rows="3"
            placeholder="模板用途描述"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="addLoading" @click="handleCreate">
          创建并编辑
        </el-button>
      </template>
    </el-dialog>

    <!-- 绑定站点弹窗 -->
    <el-dialog
      v-model="sitesDialogVisible"
      :title="`使用「${currentTemplate?.display_name}」的站点`"
      width="600px"
    >
      <el-table :data="boundSites" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="domain" label="域名" min-width="180">
          <template #default="{ row }">
            <el-link :href="`http://${row.domain}`" target="_blank" type="primary">
              {{ row.domain }}
            </el-link>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="站点名称" min-width="120" />
        <el-table-column prop="status" label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import dayjs from 'dayjs'
import {
  getTemplates,
  createTemplate,
  updateTemplate,
  deleteTemplate,
  getTemplateSites
} from '@/api/templates'
import type { TemplateListItem, Site } from '@/types'

const router = useRouter()

const loading = ref(false)
const addLoading = ref(false)
const addDialogVisible = ref(false)
const sitesDialogVisible = ref(false)
const addFormRef = ref<FormInstance>()

const templates = ref<TemplateListItem[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const searchStatus = ref<number | ''>('')

const currentTemplate = ref<TemplateListItem | null>(null)
const boundSites = ref<Site[]>([])

const addForm = reactive({
  name: '',
  display_name: '',
  description: ''
})

const addRules: FormRules = {
  name: [
    { required: true, message: '请输入模板标识', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_]*$/, message: '只能使用小写字母、数字和下划线，且以字母开头', trigger: 'blur' }
  ],
  display_name: [{ required: true, message: '请输入显示名称', trigger: 'blur' }]
}

const formatDate = (dateStr: string) => {
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss')
}

const loadTemplates = async () => {
  loading.value = true
  try {
    const res = await getTemplates({
      page: currentPage.value,
      page_size: pageSize.value,
      status: searchStatus.value !== '' ? searchStatus.value : undefined
    })
    templates.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

const handleAdd = () => {
  addDialogVisible.value = true
}

const handleEdit = (row: TemplateListItem) => {
  router.push(`/templates/edit/${row.id}`)
}

const handleCreate = async () => {
  await addFormRef.value?.validate()
  addLoading.value = true
  try {
    // 创建模板时使用默认内容
    const res = await createTemplate({
      name: addForm.name,
      display_name: addForm.display_name,
      description: addForm.description,
      content: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <title>{{ title }}</title>
</head>
<body>
    <h1>{{ keyword_with_emoji() }}</h1>
    <p>模板内容...</p>
</body>
</html>`
    })
    if (res.success && res.id) {
      ElMessage.success('创建成功')
      addDialogVisible.value = false
      // 跳转到编辑页面
      router.push(`/templates/edit/${res.id}`)
    } else {
      ElMessage.error(res.message || '创建失败')
    }
  } finally {
    addLoading.value = false
  }
}

const handleDelete = (row: TemplateListItem) => {
  if (row.sites_count > 0) {
    ElMessage.warning(`此模板有 ${row.sites_count} 个站点在使用，请先解除绑定`)
    return
  }

  ElMessageBox.confirm(`确定要删除模板「${row.display_name}」吗？`, '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    const res = await deleteTemplate(row.id)
    if (res.success) {
      ElMessage.success('删除成功')
      loadTemplates()
    } else {
      ElMessage.error(res.message || '删除失败')
    }
  })
}

const handleStatusChange = async (row: TemplateListItem) => {
  try {
    const res = await updateTemplate(row.id, { status: row.status })
    if (res.success) {
      ElMessage.success(row.status === 1 ? '已启用' : '已禁用')
    } else {
      // 恢复状态
      row.status = row.status === 1 ? 0 : 1
      ElMessage.error(res.message || '操作失败')
    }
  } catch {
    row.status = row.status === 1 ? 0 : 1
  }
}

const showSites = async (row: TemplateListItem) => {
  currentTemplate.value = row
  try {
    const res = await getTemplateSites(row.id)
    boundSites.value = res.sites
    sitesDialogVisible.value = true
  } catch (e) {
    ElMessage.error('获取站点列表失败')
  }
}

const resetAddForm = () => {
  addForm.name = ''
  addForm.display_name = ''
  addForm.description = ''
  addFormRef.value?.clearValidate()
}

onMounted(() => {
  loadTemplates()
})
</script>

<style lang="scss" scoped>
// 使用全局 .list-page 样式
// 此处仅保留该页面特有的样式
.template-list {
  .text-muted {
    color: #909399;
  }

  .form-tip {
    font-size: 12px;
    color: #909399;
    margin-top: 4px;
  }
}
</style>
