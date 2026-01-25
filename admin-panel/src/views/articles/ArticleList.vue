<template>
  <div class="group-list-page article-list">
    <div class="page-header">
      <h2 class="title">文章管理</h2>
      <div class="actions">
        <el-button type="primary" @click="handleAdd">
          <el-icon><Plus /></el-icon>
          新增文章
        </el-button>
        <el-dropdown @command="handleDeleteAll" trigger="click">
          <el-button type="danger">
            删除全部 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="group">删除当前文章库数据</el-dropdown-item>
              <el-dropdown-item command="all" divided>删除所有文章库数据</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div class="page-container">
      <!-- 左侧边栏：分组管理 -->
      <aside class="group-sidebar">
        <div class="sidebar-header">
          <el-button type="primary" size="small" @click="groupDialogVisible = true" style="width: 100%">
            <el-icon><Plus /></el-icon> 新建文章库
          </el-button>
        </div>
        <div class="sidebar-search">
          <el-input v-model="groupSearch" placeholder="搜索文章库" size="small" clearable>
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
            <el-icon><Document /></el-icon>
            <span class="group-name">{{ group.name }}</span>
            <el-tag v-if="group.is_default" size="small" type="info">默认</el-tag>
          </div>
          <div v-if="filteredGroups.length === 0" class="empty-tip">
            暂无文章库
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
                移动到文章库 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
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

        <!-- 搜索 -->
        <el-form :inline="true" class="search-form">
          <el-form-item>
            <el-input
              v-model="searchTitle"
              placeholder="搜索文章标题"
              clearable
              @keyup.enter="loadArticles"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadArticles">搜索</el-button>
          </el-form-item>
        </el-form>

        <!-- 表格 -->
        <el-table
          ref="tableRef"
          :data="articles"
          v-loading="loading"
          stripe
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="55" />
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="title" label="标题" min-width="250" show-overflow-tooltip>
            <template #default="{ row }">
              <el-link type="primary" @click="handleEdit(row)">{{ row.title }}</el-link>
            </template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
                {{ row.status === 1 ? '启用' : '禁用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="170">
            <template #default="{ row }">
              {{ formatDate(row.created_at) }}
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
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          class="pagination"
          @size-change="loadArticles"
          @current-change="loadArticles"
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
        <el-icon><Edit /></el-icon> 编辑文章库
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
        <el-icon><Delete /></el-icon> 删除文章库
      </div>
    </div>

    <!-- 新建文章库弹窗 -->
    <el-dialog v-model="groupDialogVisible" title="新建文章库" width="400px">
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupRules" label-width="90px">
        <el-form-item label="文章库名称" prop="name">
          <el-input v-model="groupForm.name" placeholder="如：默认文章库、行业文库" />
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

    <!-- 编辑文章库弹窗 -->
    <el-dialog v-model="editGroupDialogVisible" title="编辑文章库" width="400px">
      <el-form ref="editGroupFormRef" :model="editGroupForm" :rules="groupRules" label-width="90px">
        <el-form-item label="文章库名称" prop="name">
          <el-input v-model="editGroupForm.name" placeholder="文章库名称" />
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, FormInstance, FormRules, TableInstance } from 'element-plus'
import dayjs from 'dayjs'
import {
  getArticleGroups,
  createArticleGroup,
  updateArticleGroup,
  deleteArticleGroup,
  getArticles,
  deleteArticle,
  batchDeleteArticles,
  batchUpdateArticleStatus,
  batchMoveArticles,
  deleteAllArticles
} from '@/api/articles'
import type { ArticleGroup, Article } from '@/types'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const editGroupLoading = ref(false)
const groupDialogVisible = ref(false)
const editGroupDialogVisible = ref(false)
const groupFormRef = ref<FormInstance>()
const editGroupFormRef = ref<FormInstance>()
const tableRef = ref<TableInstance>()

const groups = ref<ArticleGroup[]>([])
const activeGroupId = ref<number>(0)
const articles = ref<Article[]>([])
const selectedItems = ref<Article[]>([])
const total = ref(0)
const currentPage = ref(Number(route.query.page) || 1)
const pageSize = ref(Number(route.query.pageSize) || 20)
const searchTitle = ref('')
const groupSearch = ref('')

// 右键菜单
const contextMenuVisible = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuGroup = ref<ArticleGroup | null>(null)

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
  name: [{ required: true, message: '请输入文章库名称', trigger: 'blur' }]
}

// 同步分页状态到 URL
const updateUrlQuery = () => {
  router.replace({
    query: {
      ...route.query,
      page: currentPage.value.toString(),
      pageSize: pageSize.value.toString(),
      group: activeGroupId.value.toString()
    }
  })
}

// 监听分页变化，同步到 URL
watch([currentPage, pageSize, activeGroupId], () => {
  updateUrlQuery()
})

const filteredGroups = computed(() => {
  if (!groupSearch.value) return groups.value
  return groups.value.filter(g =>
    g.name.toLowerCase().includes(groupSearch.value.toLowerCase())
  )
})

const formatDate = (dateStr: string) => {
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss')
}

const loadGroups = async () => {
  try {
    groups.value = await getArticleGroups()
  } catch {
    groups.value = [{ id: 1, site_group_id: 1, name: '默认分组', description: null, is_default: 1, created_at: '' }]
  }
  if (groups.value.length > 0 && !activeGroupId.value) {
    activeGroupId.value = groups.value[0].id
  }
}

const loadArticles = async () => {
  if (!activeGroupId.value) return
  loading.value = true
  try {
    const res = await getArticles({
      group_id: activeGroupId.value,
      page: currentPage.value,
      page_size: pageSize.value,
      search: searchTitle.value || undefined
    })
    articles.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

const selectGroup = (groupId: number) => {
  activeGroupId.value = groupId
  currentPage.value = 1
  clearSelection()
  loadArticles()
}

const handleSelectionChange = (selection: Article[]) => {
  selectedItems.value = selection
}

const clearSelection = () => {
  selectedItems.value = []
  tableRef.value?.clearSelection()
}

// 右键菜单
const showContextMenu = (e: MouseEvent, group: ArticleGroup) => {
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
    await updateArticleGroup(contextMenuGroup.value.id, { is_default: 1 })
    ElMessage.success('已设为默认文章库')
    await loadGroups()
  } catch (e) {
    ElMessage.error((e as Error).message || '设置失败')
  }
  hideContextMenu()
}

const handleDeleteGroup = () => {
  if (!contextMenuGroup.value) return
  if (contextMenuGroup.value.is_default || groups.value.length <= 1) {
    ElMessage.warning('不能删除默认文章库或最后一个文章库')
    hideContextMenu()
    return
  }

  const group = contextMenuGroup.value
  ElMessageBox.confirm(`确定要删除文章库 "${group.name}" 吗？文章库内所有文章将被删除！`, '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await deleteArticleGroup(group.id)
      ElMessage.success('删除成功')
      if (activeGroupId.value === group.id) {
        activeGroupId.value = 0
      }
      await loadGroups()
      loadArticles()
    } catch (e) {
      ElMessage.warning((e as Error).message || '删除失败')
    }
  })
  hideContextMenu()
}

const handleCreateGroup = async () => {
  await groupFormRef.value?.validate()
  try {
    await createArticleGroup(groupForm)
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
    await updateArticleGroup(editGroupForm.id, {
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

const handleAdd = () => {
  router.push({ name: 'ArticleEdit', query: { group_id: activeGroupId.value } })
}

const handleEdit = (row: Article) => {
  router.push({ name: 'ArticleEdit', params: { id: row.id } })
}

const handleDelete = (row: Article) => {
  ElMessageBox.confirm(`确定要删除文章 "${row.title}" 吗？`, '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await deleteArticle(row.id)
      ElMessage.success('删除成功')
      loadArticles()
    } catch (e) {
      ElMessage.warning((e as Error).message || '删除失败')
    }
  })
}

// 批量操作
const handleBatchDelete = () => {
  if (selectedItems.value.length === 0) return
  ElMessageBox.confirm(`确定要删除选中的 ${selectedItems.value.length} 篇文章吗？`, '批量删除', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const ids = selectedItems.value.map(item => item.id)
      const res = await batchDeleteArticles(ids)
      ElMessage.success(`成功删除 ${res.deleted} 篇文章`)
      clearSelection()
      loadArticles()
    } catch (e) {
      ElMessage.error((e as Error).message || '批量删除失败')
    }
  })
}

const handleBatchEnable = async () => {
  if (selectedItems.value.length === 0) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const res = await batchUpdateArticleStatus(ids, 1)
    ElMessage.success(`成功启用 ${res.updated} 篇文章`)
    clearSelection()
    loadArticles()
  } catch (e) {
    ElMessage.error((e as Error).message || '批量启用失败')
  }
}

const handleBatchDisable = async () => {
  if (selectedItems.value.length === 0) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const res = await batchUpdateArticleStatus(ids, 0)
    ElMessage.success(`成功禁用 ${res.updated} 篇文章`)
    clearSelection()
    loadArticles()
  } catch (e) {
    ElMessage.error((e as Error).message || '批量禁用失败')
  }
}

const handleBatchMove = async (targetGroupId: number) => {
  if (selectedItems.value.length === 0) return
  if (targetGroupId === activeGroupId.value) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const res = await batchMoveArticles(ids, targetGroupId)
    const targetGroup = groups.value.find(g => g.id === targetGroupId)
    ElMessage.success(`成功移动 ${res.moved} 篇文章到 "${targetGroup?.name}"`)
    clearSelection()
    loadArticles()
  } catch (e) {
    ElMessage.error((e as Error).message || '批量移动失败')
  }
}

// 删除全部
const handleDeleteAll = async (command: 'group' | 'all') => {
  const currentGroup = groups.value.find(g => g.id === activeGroupId.value)

  try {
    if (command === 'group') {
      // 删除当前分组 - 单次确认
      await ElMessageBox.confirm(
        `确定要删除 "${currentGroup?.name}" 中的所有文章吗？此操作不可恢复！`,
        '警告',
        { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' }
      )
      const res = await deleteAllArticles(activeGroupId.value)
      ElMessage.success(`成功删除 ${res.deleted} 篇文章`)
    } else {
      // 删除全部 - 输入确认文字
      await ElMessageBox.prompt(
        '此操作将删除所有文章库中的全部文章！请输入 "确认删除" 以继续',
        '危险操作',
        {
          confirmButtonText: '确定删除',
          cancelButtonText: '取消',
          type: 'error',
          inputPattern: /^确认删除$/,
          inputErrorMessage: '请输入正确的确认文字'
        }
      )
      const res = await deleteAllArticles()
      ElMessage.success(`成功删除 ${res.deleted} 篇文章`)
    }
    loadArticles()
  } catch (e) {
    // 用户取消或输入错误，不做处理
    if ((e as Error).message && !(e as any).toString().includes('cancel')) {
      ElMessage.error((e as Error).message || '删除失败')
    }
  }
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

  // 从 URL 恢复分组
  if (route.query.group) {
    const groupId = Number(route.query.group)
    if (groups.value.some(g => g.id === groupId)) {
      activeGroupId.value = groupId
    }
  }

  loadArticles()
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style lang="scss" scoped>
// 使用全局样式 .group-list-page
// 此处仅保留该页面特有的样式
</style>
