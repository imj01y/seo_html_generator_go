<template>
  <div class="group-list-page keyword-list">
    <div class="page-header">
      <h2 class="title">关键词管理</h2>
      <div class="actions">
        <el-button @click="handleReload" :loading="reloadLoading">
          <el-icon><Refresh /></el-icon>
          刷新缓存
        </el-button>
        <el-button type="primary" @click="openBatchAddDialog">
          <el-icon><Upload /></el-icon>
          批量添加
        </el-button>
        <el-button type="success" @click="openUploadDialog">
          <el-icon><Document /></el-icon>
          上传文件
        </el-button>
      </div>
    </div>

    <div class="page-container">
      <!-- 左侧边栏：分组管理 -->
      <aside class="group-sidebar">
        <div class="sidebar-header">
          <el-button type="primary" size="small" @click="groupDialogVisible = true" style="width: 100%">
            <el-icon><Plus /></el-icon> 新建词库
          </el-button>
        </div>
        <div class="sidebar-search">
          <el-input v-model="groupSearch" placeholder="搜索词库" size="small" clearable>
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
            暂无词库
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
                移动到词库 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
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

        <!-- 搜索和添加 -->
        <el-form :inline="true" class="search-form">
          <el-form-item>
            <el-input
              v-model="searchKeyword"
              placeholder="搜索关键词"
              clearable
              @keyup.enter="loadKeywords"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadKeywords">搜索</el-button>
          </el-form-item>
          <el-form-item>
            <el-button type="success" @click="openAddDialog">
              <el-icon><Plus /></el-icon> 添加关键词
            </el-button>
          </el-form-item>
        </el-form>

        <!-- 表格 -->
        <el-table
          ref="tableRef"
          :data="keywords"
          v-loading="loading"
          stripe
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="55" />
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="keyword" label="关键词" min-width="200" />
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
          @size-change="loadKeywords"
          @current-change="loadKeywords"
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
        <el-icon><Edit /></el-icon> 编辑词库
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
        <el-icon><Delete /></el-icon> 删除词库
      </div>
    </div>

    <!-- 新建词库弹窗 -->
    <el-dialog v-model="groupDialogVisible" title="新建关键词库" width="400px">
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupRules" label-width="80px">
        <el-form-item label="词库名称" prop="name">
          <el-input v-model="groupForm.name" placeholder="如：默认词库、行业词库" />
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

    <!-- 编辑词库弹窗 -->
    <el-dialog v-model="editGroupDialogVisible" title="编辑关键词库" width="400px">
      <el-form ref="editGroupFormRef" :model="editGroupForm" :rules="groupRules" label-width="80px">
        <el-form-item label="词库名称" prop="name">
          <el-input v-model="editGroupForm.name" placeholder="词库名称" />
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

    <!-- 添加关键词弹窗 -->
    <el-dialog v-model="addDialogVisible" title="添加关键词" width="450px">
      <el-form :model="addForm" label-width="80px">
        <el-form-item label="所属词库" required>
          <el-select v-model="addForm.group_id" style="width: 100%">
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="关键词" required>
          <el-input v-model="addForm.keyword" placeholder="输入关键词" @keyup.enter="handleAddSingle" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="addLoading" @click="handleAddSingle">添加</el-button>
      </template>
    </el-dialog>

    <!-- 编辑关键词弹窗 -->
    <el-dialog v-model="editDialogVisible" title="编辑关键词" width="450px">
      <el-form :model="editForm" label-width="80px">
        <el-form-item label="所属词库" required>
          <el-select v-model="editForm.group_id" style="width: 100%">
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="关键词" required>
          <el-input v-model="editForm.keyword" placeholder="输入关键词" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="editForm.status" :active-value="1" :inactive-value="0" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="editLoading" @click="handleUpdate">保存</el-button>
      </template>
    </el-dialog>

    <!-- 批量添加弹窗 -->
    <el-dialog v-model="batchAddVisible" title="批量添加关键词" width="600px">
      <el-form label-width="80px">
        <el-form-item label="所属词库" required>
          <el-select v-model="batchAddGroupId" style="width: 200px">
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <el-alert
        title="每行一个关键词，最多支持100000个"
        type="info"
        :closable="false"
        style="margin-bottom: 16px"
      />
      <el-input
        v-model="batchKeywords"
        type="textarea"
        :rows="15"
        placeholder="请输入关键词，每行一个&#10;关键词1&#10;关键词2&#10;关键词3"
      />
      <div class="batch-stats" v-if="batchKeywords">
        <span>当前输入: {{ batchKeywords.split('\n').filter(k => k.trim()).length }} 个关键词</span>
      </div>
      <template #footer>
        <el-button @click="batchAddVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchLoading" @click="handleBatchAdd">
          添加
        </el-button>
      </template>
    </el-dialog>

    <!-- 上传文件弹窗 -->
    <el-dialog v-model="uploadDialogVisible" title="上传关键词文件" width="500px">
      <el-form label-width="80px">
        <el-form-item label="所属词库" required>
          <el-select v-model="uploadGroupId" style="width: 100%">
            <el-option
              v-for="g in groups"
              :key="g.id"
              :label="g.name + (g.is_default ? ' (默认)' : '')"
              :value="g.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="选择文件" required>
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            accept=".txt"
            :on-change="handleFileChange"
            :on-remove="handleFileRemove"
          >
            <template #trigger>
              <el-button type="primary">选择 TXT 文件</el-button>
            </template>
            <template #tip>
              <div class="el-upload__tip">
                仅支持 .txt 文件，每行一个关键词，最多 500000 个
              </div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="uploadDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="uploadLoading" @click="handleUpload">
          上传
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox, FormInstance, FormRules, TableInstance, UploadInstance } from 'element-plus'
import dayjs from 'dayjs'
import {
  getKeywordGroups,
  createKeywordGroup,
  updateKeywordGroup,
  deleteKeywordGroup,
  getKeywords,
  addKeyword,
  addKeywordsBatch,
  updateKeyword,
  deleteKeyword,
  reloadKeywordGroup,
  batchDeleteKeywords,
  batchUpdateKeywordStatus,
  batchMoveKeywords,
  uploadKeywordsFile
} from '@/api/keywords'
import type { KeywordGroup, Keyword } from '@/types'

const loading = ref(false)
const reloadLoading = ref(false)
const batchLoading = ref(false)
const addLoading = ref(false)
const editLoading = ref(false)
const editGroupLoading = ref(false)
const groupDialogVisible = ref(false)
const editGroupDialogVisible = ref(false)
const addDialogVisible = ref(false)
const editDialogVisible = ref(false)
const batchAddVisible = ref(false)
const groupFormRef = ref<FormInstance>()
const editGroupFormRef = ref<FormInstance>()
const tableRef = ref<TableInstance>()
const uploadRef = ref<UploadInstance>()

// 上传相关
const uploadDialogVisible = ref(false)
const uploadGroupId = ref(1)
const uploadFile = ref<File | null>(null)
const uploadLoading = ref(false)

const groups = ref<KeywordGroup[]>([])
const activeGroupId = ref<number>(0)
const keywords = ref<Keyword[]>([])
const selectedItems = ref<Keyword[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const searchKeyword = ref('')
const batchKeywords = ref('')
const batchAddGroupId = ref(0)
const groupSearch = ref('')

const addForm = reactive({
  group_id: 0,
  keyword: ''
})

const editForm = reactive({
  id: 0,
  group_id: 0,
  keyword: '',
  status: 1
})

// 右键菜单
const contextMenuVisible = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuGroup = ref<KeywordGroup | null>(null)

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
  name: [{ required: true, message: '请输入词库名称', trigger: 'blur' }]
}

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
    groups.value = await getKeywordGroups()
  } catch {
    groups.value = [{ id: 1, site_group_id: 1, name: '默认分组', description: null, is_default: 1, created_at: '' }]
  }
  if (groups.value.length > 0 && !activeGroupId.value) {
    activeGroupId.value = groups.value[0].id
  }
}

const loadKeywords = async () => {
  if (!activeGroupId.value) return
  loading.value = true
  try {
    const res = await getKeywords({
      group_id: activeGroupId.value,
      page: currentPage.value,
      page_size: pageSize.value,
      search: searchKeyword.value || undefined
    })
    keywords.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

const selectGroup = (groupId: number) => {
  activeGroupId.value = groupId
  currentPage.value = 1
  clearSelection()
  loadKeywords()
}

const handleSelectionChange = (selection: Keyword[]) => {
  selectedItems.value = selection
}

const clearSelection = () => {
  selectedItems.value = []
  tableRef.value?.clearSelection()
}

// 右键菜单
const showContextMenu = (e: MouseEvent, group: KeywordGroup) => {
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
    await updateKeywordGroup(contextMenuGroup.value.id, { is_default: 1 })
    ElMessage.success('已设为默认词库')
    await loadGroups()
  } catch (e) {
    ElMessage.error((e as Error).message || '设置失败')
  }
  hideContextMenu()
}

const handleDeleteGroup = () => {
  if (!contextMenuGroup.value) return
  if (contextMenuGroup.value.is_default || groups.value.length <= 1) {
    ElMessage.warning('不能删除默认词库或最后一个词库')
    hideContextMenu()
    return
  }

  const group = contextMenuGroup.value
  ElMessageBox.confirm(`确定要删除关键词库 "${group.name}" 吗？词库内所有关键词将被删除！`, '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await deleteKeywordGroup(group.id)
      ElMessage.success('删除成功')
      if (activeGroupId.value === group.id) {
        activeGroupId.value = 0
      }
      await loadGroups()
      loadKeywords()
    } catch (e) {
      ElMessage.warning((e as Error).message || '删除失败')
    }
  })
  hideContextMenu()
}

const handleCreateGroup = async () => {
  await groupFormRef.value?.validate()
  try {
    await createKeywordGroup(groupForm)
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
    await updateKeywordGroup(editGroupForm.id, {
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

const openAddDialog = () => {
  addForm.group_id = activeGroupId.value
  addForm.keyword = ''
  addDialogVisible.value = true
}

const handleAddSingle = async () => {
  if (!addForm.keyword.trim()) {
    ElMessage.warning('请输入关键词')
    return
  }
  addLoading.value = true
  try {
    await addKeyword({ group_id: addForm.group_id, keyword: addForm.keyword.trim() })
    ElMessage.success('添加成功')
    addDialogVisible.value = false
    // 如果添加到当前分组，刷新列表
    if (addForm.group_id === activeGroupId.value) {
      loadKeywords()
    }
  } finally {
    addLoading.value = false
  }
}

const openBatchAddDialog = () => {
  batchAddGroupId.value = activeGroupId.value
  batchKeywords.value = ''
  batchAddVisible.value = true
}

const handleBatchAdd = async () => {
  const kws = batchKeywords.value
    .split('\n')
    .map(k => k.trim())
    .filter(k => k)

  if (kws.length === 0) {
    ElMessage.warning('请输入关键词')
    return
  }

  if (kws.length > 100000) {
    ElMessage.warning('单次最多添加100000个关键词')
    return
  }

  batchLoading.value = true
  try {
    const res = await addKeywordsBatch({ group_id: batchAddGroupId.value, keywords: kws })
    ElMessage.success(`添加成功: ${res.added} 个, 跳过: ${res.skipped} 个`)
    batchAddVisible.value = false
    batchKeywords.value = ''
    // 如果添加到当前分组，刷新列表
    if (batchAddGroupId.value === activeGroupId.value) {
      loadKeywords()
    }
  } finally {
    batchLoading.value = false
  }
}

const handleEdit = (row: Keyword) => {
  editForm.id = row.id
  editForm.group_id = row.group_id
  editForm.keyword = row.keyword
  editForm.status = row.status
  editDialogVisible.value = true
}

const handleUpdate = async () => {
  if (!editForm.keyword.trim()) {
    ElMessage.warning('请输入关键词')
    return
  }
  editLoading.value = true
  try {
    await updateKeyword(editForm.id, {
      keyword: editForm.keyword.trim(),
      group_id: editForm.group_id,
      status: editForm.status
    })
    ElMessage.success('更新成功')
    editDialogVisible.value = false
    loadKeywords()
  } catch (e) {
    ElMessage.error((e as Error).message || '更新失败')
  } finally {
    editLoading.value = false
  }
}

const handleDelete = (row: Keyword) => {
  ElMessageBox.confirm(`确定要删除关键词 "${row.keyword}" 吗？`, '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await deleteKeyword(row.id)
      ElMessage.success('删除成功')
      loadKeywords()
    } catch (e) {
      ElMessage.warning((e as Error).message || '删除失败')
    }
  })
}

// 批量操作
const handleBatchDelete = () => {
  if (selectedItems.value.length === 0) return
  ElMessageBox.confirm(`确定要删除选中的 ${selectedItems.value.length} 个关键词吗？`, '批量删除', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const ids = selectedItems.value.map(item => item.id)
      const res = await batchDeleteKeywords(ids)
      ElMessage.success(`成功删除 ${res.deleted} 个关键词`)
      clearSelection()
      loadKeywords()
    } catch (e) {
      ElMessage.error((e as Error).message || '批量删除失败')
    }
  })
}

const handleBatchEnable = async () => {
  if (selectedItems.value.length === 0) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const res = await batchUpdateKeywordStatus(ids, 1)
    ElMessage.success(`成功启用 ${res.updated} 个关键词`)
    clearSelection()
    loadKeywords()
  } catch (e) {
    ElMessage.error((e as Error).message || '批量启用失败')
  }
}

const handleBatchDisable = async () => {
  if (selectedItems.value.length === 0) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const res = await batchUpdateKeywordStatus(ids, 0)
    ElMessage.success(`成功禁用 ${res.updated} 个关键词`)
    clearSelection()
    loadKeywords()
  } catch (e) {
    ElMessage.error((e as Error).message || '批量禁用失败')
  }
}

const handleBatchMove = async (targetGroupId: number) => {
  if (selectedItems.value.length === 0) return
  if (targetGroupId === activeGroupId.value) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const res = await batchMoveKeywords(ids, targetGroupId)
    const targetGroup = groups.value.find(g => g.id === targetGroupId)
    ElMessage.success(`成功移动 ${res.moved} 个关键词到 "${targetGroup?.name}"`)
    clearSelection()
    loadKeywords()
  } catch (e) {
    ElMessage.error((e as Error).message || '批量移动失败')
  }
}

const handleReload = async () => {
  reloadLoading.value = true
  try {
    const res = await reloadKeywordGroup(activeGroupId.value || undefined)
    ElMessage.success(`刷新成功，共 ${res.total} 个关键词`)
  } finally {
    reloadLoading.value = false
  }
}

// 打开上传对话框
const openUploadDialog = () => {
  uploadGroupId.value = activeGroupId.value || 1
  uploadFile.value = null
  uploadDialogVisible.value = true
}

// 文件选择变化
const handleFileChange = (file: any) => {
  uploadFile.value = file.raw
}

// 文件移除
const handleFileRemove = () => {
  uploadFile.value = null
}

// 执行上传
const handleUpload = async () => {
  if (!uploadFile.value) {
    ElMessage.warning('请选择文件')
    return
  }

  uploadLoading.value = true
  try {
    const res = await uploadKeywordsFile(uploadFile.value, uploadGroupId.value)
    ElMessage.success(res.message)
    uploadDialogVisible.value = false
    uploadFile.value = null
    uploadRef.value?.clearFiles()

    // 刷新列表
    if (uploadGroupId.value === activeGroupId.value) {
      await loadKeywords()
    }
  } catch (error: any) {
    ElMessage.error(error.message || '上传失败')
  } finally {
    uploadLoading.value = false
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
  loadKeywords()
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
