<template>
  <div class="group-list-page template-list">
    <div class="page-header">
      <h2 class="title">模板管理</h2>
      <div class="header-actions">
        <el-button type="warning" :loading="reloadingCache" @click="handleReloadCache">
          <el-icon><Refresh /></el-icon>
          刷新缓存
        </el-button>
        <el-button type="primary" @click="handleAdd">
          <el-icon><Plus /></el-icon>
          新增模板
        </el-button>
      </div>
    </div>

    <div class="page-container">
      <!-- 左侧边栏：模板库管理 -->
      <aside class="group-sidebar">
        <div class="sidebar-header">
          <el-button type="primary" size="small" @click="groupDialogVisible = true" style="width: 100%">
            <el-icon><Plus /></el-icon> 新建模板库
          </el-button>
        </div>
        <div class="sidebar-search">
          <el-input v-model="groupSearch" placeholder="搜索模板库" size="small" clearable>
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
        </div>
        <div class="sidebar-list">
          <div
            v-for="group in filteredGroups"
            :key="group.id"
            class="group-item"
            :class="{ active: activeGroupId === group.id }"
            @click="selectGroup(group.id)"
            @contextmenu.prevent="showContextMenu($event, group)"
          >
            <el-icon><Folder /></el-icon>
            <span class="group-name">{{ group.name }}</span>
            <el-tag v-if="group.is_default" size="small" type="info">默认</el-tag>
          </div>
          <div v-if="filteredGroups.length === 0" class="empty-tip">
            暂无模板库
          </div>
        </div>
      </aside>

      <!-- 右侧内容区 -->
      <main class="content-area">
        <!-- 批量操作栏 -->
        <transition name="fade">
          <div v-if="selectedItems.length > 0" class="batch-actions">
            <span class="selected-count">已选中 {{ selectedItems.length }} 项</span>
            <el-button size="small" type="danger" @click="handleBatchDelete">
              <el-icon><Delete /></el-icon> 删除
            </el-button>
            <el-button size="small" type="success" @click="handleBatchEnable">
              <el-icon><Check /></el-icon> 启用
            </el-button>
            <el-button size="small" type="warning" @click="handleBatchDisable">
              <el-icon><Close /></el-icon> 禁用
            </el-button>
            <el-dropdown @command="handleBatchMove" trigger="click">
              <el-button size="small">
                移动到模板库 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item
                    v-for="group in groups"
                    :key="group.id"
                    :command="group.id"
                    :disabled="group.id === activeGroupId"
                  >
                    {{ group.name }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button size="small" link @click="clearSelection">取消选择</el-button>
          </div>
        </transition>

        <!-- 搜索栏 -->
        <el-form :inline="true" class="search-form">
          <el-form-item label="状态">
            <el-select v-model="searchStatus" placeholder="全部" clearable @change="loadTemplates" style="width: 100px">
              <el-option label="启用" :value="1" />
              <el-option label="禁用" :value="0" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadTemplates">搜索</el-button>
          </el-form-item>
        </el-form>

        <!-- 表格 -->
        <el-table
          ref="tableRef"
          :data="templates"
          v-loading="loading"
          stripe
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="55" />
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
      </main>
    </div>

    <!-- 右键菜单 -->
    <div
      v-if="contextMenuVisible"
      class="context-menu"
      :style="{ left: contextMenuX + 'px', top: contextMenuY + 'px' }"
    >
      <div class="menu-item" @click="handleEditGroup">
        <el-icon><Edit /></el-icon> 编辑模板库
      </div>
      <div class="menu-item" @click="handleSetDefault" v-if="!contextMenuGroup?.is_default">
        <el-icon><Star /></el-icon> 设为默认
      </div>
      <div class="menu-divider"></div>
      <div
        class="menu-item danger"
        @click="handleDeleteGroup"
        :class="{ disabled: contextMenuGroup?.is_default || groups.length <= 1 }"
      >
        <el-icon><Delete /></el-icon> 删除模板库
      </div>
    </div>

    <!-- 新建模板库弹窗 -->
    <el-dialog v-model="groupDialogVisible" title="新建模板库" width="400px">
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupRules" label-width="80px">
        <el-form-item label="库名称" prop="name">
          <el-input v-model="groupForm.name" placeholder="如：默认模板库、游戏站模板" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="groupForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleCreateGroup">确定</el-button>
      </template>
    </el-dialog>

    <!-- 编辑模板库弹窗 -->
    <el-dialog v-model="editGroupDialogVisible" title="编辑模板库" width="400px">
      <el-form ref="editGroupFormRef" :model="editGroupForm" :rules="groupRules" label-width="80px">
        <el-form-item label="库名称" prop="name">
          <el-input v-model="editGroupForm.name" placeholder="模板库名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="editGroupForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editGroupDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="editGroupLoading" @click="handleUpdateGroup">确定</el-button>
      </template>
    </el-dialog>

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
        <el-form-item label="所属模板库" prop="site_group_id">
          <el-select v-model="addForm.site_group_id" style="width: 100%">
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
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
        <div class="dialog-footer">
          <el-button @click="showGuide" class="guide-btn">
            <el-icon><QuestionFilled /></el-icon>
            模板标签指南
          </el-button>
          <div class="footer-right">
            <el-button @click="addDialogVisible = false">取消</el-button>
            <el-button type="primary" :loading="addLoading" @click="handleCreate">
              创建并编辑
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>

    <!-- 模板标签指南 -->
    <TemplateGuide ref="guideRef" />

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
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, FormInstance, FormRules, TableInstance } from 'element-plus'
import { Plus, Search, Folder, Delete, Check, Close, Edit, Star, ArrowDown, Refresh, QuestionFilled } from '@element-plus/icons-vue'
import TemplateGuide from '@/components/TemplateGuide.vue'
import dayjs from 'dayjs'
import {
  getTemplates,
  createTemplate,
  updateTemplate,
  deleteTemplate,
  getTemplateSites,
  reloadGoTemplateCache
} from '@/api/templates'
import {
  getSiteGroups,
  createSiteGroup,
  updateSiteGroup,
  deleteSiteGroup
} from '@/api/site-groups'
import type { TemplateListItem, Site, SiteGroup } from '@/types'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const addLoading = ref(false)
const editGroupLoading = ref(false)
const reloadingCache = ref(false)
const addDialogVisible = ref(false)
const sitesDialogVisible = ref(false)
const groupDialogVisible = ref(false)
const editGroupDialogVisible = ref(false)
const addFormRef = ref<FormInstance>()
const groupFormRef = ref<FormInstance>()
const editGroupFormRef = ref<FormInstance>()
const tableRef = ref<TableInstance>()
const guideRef = ref<InstanceType<typeof TemplateGuide>>()

// 模板库相关
const groups = ref<SiteGroup[]>([])
const activeGroupId = ref<number>(0)
const groupSearch = ref('')

// 模板数据
const templates = ref<TemplateListItem[]>([])
const selectedItems = ref<TemplateListItem[]>([])
const total = ref(0)
const currentPage = ref(Number(route.query.page) || 1)
const pageSize = ref(Number(route.query.pageSize) || 20)
const searchStatus = ref<number | ''>('')

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

const currentTemplate = ref<TemplateListItem | null>(null)
const boundSites = ref<Site[]>([])

// 右键菜单
const contextMenuVisible = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuGroup = ref<SiteGroup | null>(null)

// 模板库表单
const groupForm = reactive({
  name: '',
  description: ''
})

const editGroupForm = reactive({
  id: 0,
  name: '',
  description: ''
})

const groupRules: FormRules = {
  name: [{ required: true, message: '请输入模板库名称', trigger: 'blur' }]
}

// 新增模板表单
const addForm = reactive({
  site_group_id: 1,
  name: '',
  display_name: '',
  description: ''
})

const addRules: FormRules = {
  site_group_id: [{ required: true, message: '请选择所属模板库', trigger: 'change' }],
  name: [
    { required: true, message: '请输入模板标识', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_]*$/, message: '只能使用小写字母、数字和下划线，且以字母开头', trigger: 'blur' }
  ],
  display_name: [{ required: true, message: '请输入显示名称', trigger: 'blur' }]
}

// 计算属性
const filteredGroups = computed(() => {
  if (!groupSearch.value) return groups.value
  return groups.value.filter(g =>
    g.name.toLowerCase().includes(groupSearch.value.toLowerCase())
  )
})

const formatDate = (dateStr: string) => {
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss')
}

// 加载模板库
const loadGroups = async () => {
  try {
    groups.value = await getSiteGroups()
  } catch {
    groups.value = [{ id: 1, name: '默认模板库', description: null, status: 1, is_default: 1, created_at: '', updated_at: '' }]
  }
  if (groups.value.length > 0 && !activeGroupId.value) {
    activeGroupId.value = groups.value[0].id
  }
}

// 加载模板列表
const loadTemplates = async () => {
  if (!activeGroupId.value) return
  loading.value = true
  try {
    const res = await getTemplates({
      page: currentPage.value,
      page_size: pageSize.value,
      status: searchStatus.value !== '' ? searchStatus.value : undefined,
      site_group_id: activeGroupId.value
    })
    templates.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

// 选择模板库
const selectGroup = (groupId: number) => {
  activeGroupId.value = groupId
  currentPage.value = 1
  clearSelection()
  loadTemplates()
}

// 表格选择
const handleSelectionChange = (selection: TemplateListItem[]) => {
  selectedItems.value = selection
}

const clearSelection = () => {
  selectedItems.value = []
  tableRef.value?.clearSelection()
}

// 右键菜单
const showContextMenu = (e: MouseEvent, group: SiteGroup) => {
  contextMenuX.value = e.clientX
  contextMenuY.value = e.clientY
  contextMenuGroup.value = group
  contextMenuVisible.value = true
}

const hideContextMenu = () => {
  contextMenuVisible.value = false
  contextMenuGroup.value = null
}

const handleEditGroup = () => {
  if (!contextMenuGroup.value) return
  editGroupForm.id = contextMenuGroup.value.id
  editGroupForm.name = contextMenuGroup.value.name
  editGroupForm.description = contextMenuGroup.value.description || ''
  editGroupDialogVisible.value = true
  hideContextMenu()
}

const handleSetDefault = async () => {
  if (!contextMenuGroup.value) return
  try {
    await updateSiteGroup(contextMenuGroup.value.id, { is_default: 1 })
    ElMessage.success('已设为默认模板库')
    await loadGroups()
  } catch (e) {
    ElMessage.error((e as Error).message || '设置失败')
  }
  hideContextMenu()
}

const handleDeleteGroup = () => {
  if (!contextMenuGroup.value) return
  if (contextMenuGroup.value.is_default || groups.value.length <= 1) {
    ElMessage.warning('不能删除默认模板库或最后一个模板库')
    hideContextMenu()
    return
  }

  const group = contextMenuGroup.value
  ElMessageBox.confirm(`确定要删除模板库 "${group.name}" 吗？库内所有模板将被删除！`, '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await deleteSiteGroup(group.id)
      ElMessage.success('删除成功')
      if (activeGroupId.value === group.id) {
        activeGroupId.value = 0
      }
      await loadGroups()
      loadTemplates()
    } catch (e) {
      ElMessage.warning((e as Error).message || '删除失败')
    }
  })
  hideContextMenu()
}

const handleCreateGroup = async () => {
  await groupFormRef.value?.validate()
  try {
    await createSiteGroup(groupForm)
    ElMessage.success('创建成功')
    groupDialogVisible.value = false
    groupForm.name = ''
    groupForm.description = ''
    await loadGroups()
  } catch (e) {
    ElMessage.warning((e as Error).message || '创建失败')
  }
}

const handleUpdateGroup = async () => {
  await editGroupFormRef.value?.validate()
  editGroupLoading.value = true
  try {
    await updateSiteGroup(editGroupForm.id, {
      name: editGroupForm.name,
      description: editGroupForm.description
    })
    ElMessage.success('更新成功')
    editGroupDialogVisible.value = false
    await loadGroups()
  } catch (e) {
    ElMessage.error((e as Error).message || '更新失败')
  } finally {
    editGroupLoading.value = false
  }
}

// 刷新模板缓存
const handleReloadCache = async () => {
  reloadingCache.value = true
  try {
    const res = await reloadGoTemplateCache()
    if (res.success) {
      ElMessage.success(`模板缓存已刷新，共加载 ${res.stats?.item_count || 0} 个模板`)
    } else {
      ElMessage.error(res.message || '刷新缓存失败')
    }
  } catch (e) {
    ElMessage.error('刷新缓存失败')
  } finally {
    reloadingCache.value = false
  }
}

// 模板操作
const handleAdd = () => {
  addForm.site_group_id = activeGroupId.value
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
      site_group_id: addForm.site_group_id,
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

// 批量操作
const handleBatchDelete = () => {
  if (selectedItems.value.length === 0) return

  // 检查是否有模板被站点使用
  const usedTemplates = selectedItems.value.filter(t => t.sites_count > 0)
  if (usedTemplates.length > 0) {
    ElMessage.warning(`有 ${usedTemplates.length} 个模板被站点使用，无法删除`)
    return
  }

  ElMessageBox.confirm(`确定要删除选中的 ${selectedItems.value.length} 个模板吗？`, '批量删除', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    let deleted = 0
    for (const item of selectedItems.value) {
      try {
        const res = await deleteTemplate(item.id)
        if (res.success) deleted++
      } catch {
        // 忽略单个删除失败
      }
    }
    ElMessage.success(`成功删除 ${deleted} 个模板`)
    clearSelection()
    loadTemplates()
  })
}

const handleBatchEnable = async () => {
  if (selectedItems.value.length === 0) return
  let updated = 0
  for (const item of selectedItems.value) {
    try {
      const res = await updateTemplate(item.id, { status: 1 })
      if (res.success) updated++
    } catch {
      // 忽略单个更新失败
    }
  }
  ElMessage.success(`成功启用 ${updated} 个模板`)
  clearSelection()
  loadTemplates()
}

const handleBatchDisable = async () => {
  if (selectedItems.value.length === 0) return
  let updated = 0
  for (const item of selectedItems.value) {
    try {
      const res = await updateTemplate(item.id, { status: 0 })
      if (res.success) updated++
    } catch {
      // 忽略单个更新失败
    }
  }
  ElMessage.success(`成功禁用 ${updated} 个模板`)
  clearSelection()
  loadTemplates()
}

const handleBatchMove = async (targetGroupId: number) => {
  if (selectedItems.value.length === 0) return
  if (targetGroupId === activeGroupId.value) return

  let moved = 0
  for (const item of selectedItems.value) {
    try {
      const res = await updateTemplate(item.id, { site_group_id: targetGroupId })
      if (res.success) moved++
    } catch {
      // 忽略单个更新失败
    }
  }
  const targetGroup = groups.value.find(g => g.id === targetGroupId)
  ElMessage.success(`成功移动 ${moved} 个模板到 "${targetGroup?.name}"`)
  clearSelection()
  loadTemplates()
}

const resetAddForm = () => {
  addForm.name = ''
  addForm.display_name = ''
  addForm.description = ''
  addFormRef.value?.clearValidate()
}

// 显示指南
const showGuide = () => {
  guideRef.value?.show()
}

// 点击其他地方隐藏右键菜单
const handleClickOutside = (e: MouseEvent) => {
  const target = e.target as HTMLElement
  if (!target.closest('.context-menu')) {
    hideContextMenu()
  }
}

onMounted(async () => {
  await loadGroups()
  loadTemplates()
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style lang="scss" scoped>
// 使用全局样式 .group-list-page
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

  .dialog-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
  }

  .footer-right {
    display: flex;
    gap: 12px;
  }
}
</style>
