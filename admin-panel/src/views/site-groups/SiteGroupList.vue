<template>
  <div class="list-page site-group-list">
    <div class="page-header">
      <h2 class="title">站群管理</h2>
      <el-button type="primary" @click="handleAdd">
        <el-icon><Plus /></el-icon>
        新增站群
      </el-button>
    </div>

    <div class="card">
      <!-- 表格 -->
      <el-table :data="siteGroups" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="站群名称" min-width="150" />
        <el-table-column prop="description" label="描述" min-width="200">
          <template #default="{ row }">
            <span>{{ row.description || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="统计" width="280">
          <template #default="{ row }">
            <div class="stats-row" v-if="row.stats">
              <el-tag size="small">站点: {{ row.stats.sites_count }}</el-tag>
              <el-tag size="small" type="success">词库: {{ row.stats.keyword_groups_count }}</el-tag>
              <el-tag size="small" type="warning">图库: {{ row.stats.image_groups_count }}</el-tag>
              <el-tag size="small" type="info">文章库: {{ row.stats.article_groups_count }}</el-tag>
            </div>
            <span v-else class="text-muted">加载中...</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-switch
              v-model="row.status"
              :active-value="1"
              :inactive-value="0"
              @change="handleStatusChange(row)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button type="danger" size="small" @click="handleDelete(row)" :disabled="row.id === 1">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 新增/编辑弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑站群' : '新增站群'"
      width="500px"
      @close="resetForm"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="80px"
      >
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="站群名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="站群描述（可选）"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleSubmit">
          确定
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import dayjs from 'dayjs'
import {
  getSiteGroups,
  getSiteGroup,
  createSiteGroup,
  updateSiteGroup,
  deleteSiteGroup
} from '@/api/site-groups'
import type { SiteGroup, SiteGroupWithStats } from '@/types'

const loading = ref(false)
const submitLoading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const formRef = ref<FormInstance>()

const siteGroups = ref<SiteGroupWithStats[]>([])

const form = reactive({
  id: 0,
  name: '',
  description: ''
})

const rules: FormRules = {
  name: [{ required: true, message: '请输入站群名称', trigger: 'blur' }]
}

const formatDate = (dateStr: string) => {
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss')
}

const loadSiteGroups = async () => {
  loading.value = true
  try {
    const groups = await getSiteGroups()
    // 获取每个站群的统计信息
    const groupsWithStats: SiteGroupWithStats[] = []
    for (const group of groups) {
      try {
        const detail = await getSiteGroup(group.id)
        groupsWithStats.push(detail)
      } catch {
        groupsWithStats.push({
          ...group,
          stats: {
            sites_count: 0,
            keyword_groups_count: 0,
            image_groups_count: 0,
            article_groups_count: 0,
            templates_count: 0
          }
        })
      }
    }
    siteGroups.value = groupsWithStats
  } finally {
    loading.value = false
  }
}

const handleAdd = () => {
  isEdit.value = false
  dialogVisible.value = true
}

const handleEdit = (row: SiteGroup) => {
  isEdit.value = true
  form.id = row.id
  form.name = row.name
  form.description = row.description || ''
  dialogVisible.value = true
}

const handleDelete = (row: SiteGroupWithStats) => {
  if (row.stats && row.stats.sites_count > 0) {
    ElMessage.warning(`无法删除：站群下有 ${row.stats.sites_count} 个站点`)
    return
  }

  ElMessageBox.confirm(`确定要删除站群 "${row.name}" 吗？`, '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await deleteSiteGroup(row.id)
      ElMessage.success('删除成功')
      loadSiteGroups()
    } catch (e) {
      ElMessage.error((e as Error).message || '删除失败')
    }
  })
}

const handleStatusChange = async (row: SiteGroup) => {
  try {
    await updateSiteGroup(row.id, { status: row.status })
    ElMessage.success(row.status === 1 ? '已启用' : '已禁用')
  } catch {
    row.status = row.status === 1 ? 0 : 1
  }
}

const handleSubmit = async () => {
  await formRef.value?.validate()
  submitLoading.value = true
  try {
    if (isEdit.value) {
      await updateSiteGroup(form.id, {
        name: form.name,
        description: form.description
      })
      ElMessage.success('更新成功')
    } else {
      await createSiteGroup({
        name: form.name,
        description: form.description
      })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    loadSiteGroups()
  } catch (e) {
    ElMessage.error((e as Error).message || '操作失败')
  } finally {
    submitLoading.value = false
  }
}

const resetForm = () => {
  form.id = 0
  form.name = ''
  form.description = ''
  formRef.value?.clearValidate()
}

onMounted(() => {
  loadSiteGroups()
})
</script>

<style lang="scss" scoped>
// 使用全局 .list-page 样式
// 此处仅保留该页面特有的样式
.site-group-list {
  .stats-row {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .text-muted {
    color: #909399;
    font-size: 13px;
  }
}
</style>
