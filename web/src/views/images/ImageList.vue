<template>
  <div class="group-list-page image-list">
    <div class="page-header">
      <h2 class="title">图片管理</h2>
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
        <el-dropdown @command="handleDeleteAll" trigger="click">
          <el-button type="danger">
            删除全部 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="group">删除当前图库数据</el-dropdown-item>
              <el-dropdown-item command="all" divided>删除所有图库数据</el-dropdown-item>
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
            <el-icon><Plus /></el-icon> 新建图库
          </el-button>
        </div>
        <div class="sidebar-search">
          <el-input v-model="groupSearch" placeholder="搜索图库" size="small" clearable>
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
            <el-icon><Picture /></el-icon>
            <span class="group-name">{{ group.name }}</span>
            <el-tag v-if="group.is_default" size="small" type="info">默认</el-tag>
          </div>
          <div v-if="filteredGroups.length === 0" class="empty-tip">
            暂无图库
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
                移动到图库 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
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
              v-model="searchUrl"
              placeholder="搜索图片URL"
              clearable
              @keyup.enter="loadImages"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadImages">搜索</el-button>
          </el-form-item>
          <el-form-item>
            <el-button type="success" @click="openAddDialog">
              <el-icon><Plus /></el-icon> 添加图片URL
            </el-button>
          </el-form-item>
        </el-form>

        <!-- 表格 -->
        <el-table
          ref="tableRef"
          :data="images"
          v-loading="loading"
          stripe
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="55" />
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column label="预览" width="80">
            <template #default="{ row }">
              <el-image
                :src="row.url"
                :preview-src-list="[row.url]"
                fit="cover"
                style="width: 50px; height: 50px; border-radius: 4px"
                :preview-teleported="true"
              >
                <template #error>
                  <div class="image-error">
                    <el-icon><Picture /></el-icon>
                  </div>
                </template>
              </el-image>
            </template>
          </el-table-column>
          <el-table-column prop="url" label="URL" min-width="300" show-overflow-tooltip>
            <template #default="{ row }">
              <el-link :href="row.url" target="_blank" type="primary" class="url-link">
                {{ row.url }}
              </el-link>
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
          @size-change="loadImages"
          @current-change="loadImages"
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
        <el-icon><Edit /></el-icon> 编辑图库
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
        <el-icon><Delete /></el-icon> 删除图库
      </div>
    </div>

    <!-- 新建图库弹窗 -->
    <el-dialog v-model="groupDialogVisible" title="新建图库" width="400px">
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupRules" label-width="80px">
        <el-form-item label="图库名称" prop="name">
          <el-input v-model="groupForm.name" placeholder="如：默认图库、产品图库" />
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

    <!-- 编辑图库弹窗 -->
    <el-dialog v-model="editGroupDialogVisible" title="编辑图库" width="400px">
      <el-form ref="editGroupFormRef" :model="editGroupForm" :rules="groupRules" label-width="80px">
        <el-form-item label="图库名称" prop="name">
          <el-input v-model="editGroupForm.name" placeholder="图库名称" />
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

    <!-- 添加图片URL弹窗 -->
    <el-dialog v-model="addDialogVisible" title="添加图片URL" width="550px">
      <el-form :model="addForm" label-width="80px">
        <el-form-item label="所属图库" required>
          <el-select v-model="addForm.group_id" style="width: 100%">
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="图片URL" required>
          <el-input v-model="addForm.url" placeholder="https://example.com/image.jpg" @keyup.enter="handleAddSingle" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="addLoading" @click="handleAddSingle">添加</el-button>
      </template>
    </el-dialog>

    <!-- 编辑图片URL弹窗 -->
    <el-dialog v-model="editDialogVisible" title="编辑图片URL" width="550px">
      <el-form :model="editForm" label-width="80px">
        <el-form-item label="所属图库" required>
          <el-select v-model="editForm.group_id" style="width: 100%">
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="图片URL" required>
          <el-input v-model="editForm.url" placeholder="https://example.com/image.jpg" />
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
    <el-dialog v-model="batchAddVisible" title="批量添加图片URL" width="700px">
      <el-form label-width="80px">
        <el-form-item label="所属图库" required>
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
        title="每行一个URL，最多支持100000个，自动去重"
        type="info"
        :closable="false"
        style="margin-bottom: 16px"
      />
      <el-input
        v-model="batchUrls"
        type="textarea"
        :rows="15"
        placeholder="请输入图片URL，每行一个&#10;https://example.com/image1.jpg&#10;https://example.com/image2.png"
      />
      <div class="batch-stats" v-if="batchUrls">
        <span>当前输入: {{ batchUrls.split('\n').filter(u => u.trim()).length }} 个URL</span>
      </div>
      <template #footer>
        <el-button @click="batchAddVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchLoading" @click="handleBatchAdd">
          添加
        </el-button>
      </template>
    </el-dialog>

    <!-- 上传文件弹窗 -->
    <el-dialog v-model="uploadDialogVisible" title="上传图片URL文件" width="500px" :close-on-click-modal="false" :close-on-press-escape="false" :before-close="handleUploadDialogClose">
      <el-form label-width="80px">
        <el-form-item label="所属图库" required>
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
            multiple
            accept=".txt"
            :on-change="handleFileChange"
            :on-remove="handleFileRemove"
          >
            <template #trigger>
              <el-button type="primary">选择 TXT 文件</el-button>
            </template>
            <template #tip>
              <div class="el-upload__tip">
                支持同时选择多个 .txt 文件，每行一个图片URL
              </div>
            </template>
          </el-upload>
        </el-form-item>
        <!-- 已选文件统计 -->
        <el-form-item v-if="uploadFiles.length > 0" label="已选文件">
          <el-tag type="info">{{ uploadFiles.length }} 个文件</el-tag>
        </el-form-item>
        <!-- 上传进度 -->
        <el-form-item v-if="uploadProgress.total > 0" label="上传进度">
          <el-progress
            :percentage="Math.round((uploadProgress.current / uploadProgress.total) * 100)"
            :format="() => `${uploadProgress.current}/${uploadProgress.total}`"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="closeUploadDialog" :disabled="uploadLoading">取消</el-button>
        <el-button type="primary" :loading="uploadLoading" @click="handleUpload">
          {{ uploadLoading ? `上传中 (${uploadProgress.current}/${uploadProgress.total})` : '上传' }}
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
  getImageGroups,
  createImageGroup,
  updateImageGroup,
  deleteImageGroup,
  getImageUrls,
  addImageUrl,
  addImageUrlsBatch,
  updateImageUrl,
  deleteImageUrl,
  reloadImageGroup,
  batchDeleteImages,
  batchUpdateImageStatus,
  batchMoveImages,
  deleteAllImages,
  uploadImagesFile
} from '@/api/images'
import type { ImageGroup, ImageUrl } from '@/types'

const loading = ref(false)
const reloadLoading = ref(false)
const batchLoading = ref(false)
const addLoading = ref(false)
const editLoading = ref(false)
const editGroupLoading = ref(false)
const groupDialogVisible = ref(false)
const editGroupDialogVisible = ref(false)
const batchAddVisible = ref(false)
const addDialogVisible = ref(false)
const editDialogVisible = ref(false)
const groupFormRef = ref<FormInstance>()
const editGroupFormRef = ref<FormInstance>()
const tableRef = ref<TableInstance>()
const uploadRef = ref<UploadInstance>()

// 上传相关
const uploadDialogVisible = ref(false)
const uploadGroupId = ref(1)
const uploadFiles = ref<File[]>([])
const uploadLoading = ref(false)
const uploadProgress = reactive({
  current: 0,
  total: 0
})

const groups = ref<ImageGroup[]>([])
const activeGroupId = ref<number>(0)
const images = ref<ImageUrl[]>([])
const selectedItems = ref<ImageUrl[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const searchUrl = ref('')
const batchUrls = ref('')
const groupSearch = ref('')
const batchAddGroupId = ref<number>(0)

const addForm = reactive({
  group_id: 0,
  url: ''
})

const editForm = reactive({
  id: 0,
  group_id: 0,
  url: '',
  status: 1
})

// 右键菜单
const contextMenuVisible = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuGroup = ref<ImageGroup | null>(null)

const groupForm = reactive({
  site_group_id: 1,
  name: '',
  description: ''
})

const editGroupForm = reactive({
  id: 0,
  name: '',
  description: ''
})

const groupRules: FormRules = {
  name: [{ required: true, message: '请输入图库名称', trigger: 'blur' }]
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
    groups.value = await getImageGroups()
  } catch {
    groups.value = [{ id: 1, site_group_id: 1, name: '默认分组', description: null, is_default: 1, created_at: '' }]
  }
  if (groups.value.length > 0 && !activeGroupId.value) {
    activeGroupId.value = groups.value[0].id
  }
}

const loadImages = async () => {
  if (!activeGroupId.value) return
  loading.value = true
  try {
    const res = await getImageUrls({
      group_id: activeGroupId.value,
      page: currentPage.value,
      page_size: pageSize.value,
      search: searchUrl.value || undefined
    })
    images.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

const selectGroup = (groupId: number) => {
  activeGroupId.value = groupId
  currentPage.value = 1
  clearSelection()
  loadImages()
}

const handleSelectionChange = (selection: ImageUrl[]) => {
  selectedItems.value = selection
}

const clearSelection = () => {
  selectedItems.value = []
  tableRef.value?.clearSelection()
}

// 右键菜单
const showContextMenu = (e: MouseEvent, group: ImageGroup) => {
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
    await updateImageGroup(contextMenuGroup.value.id, { is_default: 1 })
    ElMessage.success('已设为默认图库')
    await loadGroups()
  } catch (e) {
    ElMessage.error((e as Error).message || '设置失败')
  }
  hideContextMenu()
}

const handleDeleteGroup = () => {
  if (!contextMenuGroup.value) return
  if (contextMenuGroup.value.is_default || groups.value.length <= 1) {
    ElMessage.warning('不能删除默认图库或最后一个图库')
    hideContextMenu()
    return
  }

  const group = contextMenuGroup.value
  ElMessageBox.confirm(`确定要删除图库 "${group.name}" 吗？图库内所有图片URL将被删除！`, '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await deleteImageGroup(group.id)
      ElMessage.success('删除成功')
      if (activeGroupId.value === group.id) {
        activeGroupId.value = 0
      }
      await loadGroups()
      loadImages()
    } catch (e) {
      ElMessage.warning((e as Error).message || '删除失败')
    }
  })
  hideContextMenu()
}

const handleCreateGroup = async () => {
  await groupFormRef.value?.validate()
  try {
    await createImageGroup(groupForm)
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
    await updateImageGroup(editGroupForm.id, {
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
  addForm.url = ''
  addDialogVisible.value = true
}

const openBatchAddDialog = () => {
  batchAddGroupId.value = activeGroupId.value
  batchUrls.value = ''
  batchAddVisible.value = true
}

const handleAddSingle = async () => {
  if (!addForm.url.trim()) {
    ElMessage.warning('请输入图片URL')
    return
  }
  const url = addForm.url.trim()
  if (!url.startsWith('http://') && !url.startsWith('https://')) {
    ElMessage.warning('请输入有效的URL（以http://或https://开头）')
    return
  }
  addLoading.value = true
  try {
    await addImageUrl({ group_id: addForm.group_id, url })
    ElMessage.success('添加成功')
    addDialogVisible.value = false
    addForm.url = ''
    // 如果添加到当前分组，刷新列表
    if (addForm.group_id === activeGroupId.value) {
      loadImages()
    }
  } catch (e) {
    ElMessage.error((e as Error).message || '添加失败')
  } finally {
    addLoading.value = false
  }
}

const handleBatchAdd = async () => {
  const urls = batchUrls.value
    .split('\n')
    .map(u => u.trim())
    .filter(u => u && (u.startsWith('http://') || u.startsWith('https://')))

  if (urls.length === 0) {
    ElMessage.warning('请输入有效的图片URL')
    return
  }

  if (urls.length > 100000) {
    ElMessage.warning('单次最多添加100000个URL')
    return
  }

  batchLoading.value = true
  try {
    const res = await addImageUrlsBatch({ group_id: batchAddGroupId.value, urls })
    ElMessage.success(`添加成功: ${res.added} 个, 跳过: ${res.skipped} 个`)
    batchAddVisible.value = false
    batchUrls.value = ''
    // 如果添加到当前分组，刷新列表
    if (batchAddGroupId.value === activeGroupId.value) {
      loadImages()
    }
  } finally {
    batchLoading.value = false
  }
}

// 打开上传对话框
const openUploadDialog = () => {
  uploadGroupId.value = activeGroupId.value || 1
  uploadFiles.value = []
  uploadProgress.current = 0
  uploadProgress.total = 0
  uploadDialogVisible.value = true
}

// 上传弹窗关闭前的处理
const handleUploadDialogClose = (done: () => void) => {
  if (uploadLoading.value) {
    ElMessage.warning('上传中，请等待完成')
    return
  }
  resetUploadDialog()
  done()
}

// 重置上传对话框
const resetUploadDialog = () => {
  uploadFiles.value = []
  uploadProgress.current = 0
  uploadProgress.total = 0
  uploadLoading.value = false
  uploadRef.value?.clearFiles()
}

// 关闭上传对话框
const closeUploadDialog = () => {
  if (uploadLoading.value) {
    ElMessage.warning('上传中，请等待完成')
    return
  }
  resetUploadDialog()
  uploadDialogVisible.value = false
}

// 文件选择变化（支持多文件）
const handleFileChange = (file: any, fileList: any[]) => {
  uploadFiles.value = fileList.map(f => f.raw).filter(Boolean)
}

// 文件移除
const handleFileRemove = (file: any, fileList: any[]) => {
  uploadFiles.value = fileList.map(f => f.raw).filter(Boolean)
}

// 执行上传（支持多文件逐个上传）
const handleUpload = async () => {
  if (uploadFiles.value.length === 0) {
    ElMessage.warning('请选择文件')
    return
  }

  uploadLoading.value = true
  uploadProgress.current = 0
  uploadProgress.total = uploadFiles.value.length

  let totalAdded = 0
  let totalSkipped = 0
  let successCount = 0
  let failedFiles: string[] = []

  try {
    for (const file of uploadFiles.value) {
      uploadProgress.current++
      try {
        const res = await uploadImagesFile(file, uploadGroupId.value)
        totalAdded += res.added
        totalSkipped += res.skipped
        successCount++
      } catch (error: any) {
        failedFiles.push(file.name)
      }
    }

    // 显示汇总结果
    if (failedFiles.length === 0) {
      ElMessage.success(
        `全部上传成功！共 ${successCount} 个文件，添加 ${totalAdded} 个图片URL，跳过 ${totalSkipped} 个重复`
      )
      uploadLoading.value = false
      closeUploadDialog()
    } else {
      ElMessage.warning(
        `${successCount} 个文件成功，${failedFiles.length} 个失败。添加 ${totalAdded} 个图片URL，跳过 ${totalSkipped} 个重复`
      )
      uploadLoading.value = false
    }

    // 刷新列表
    if (uploadGroupId.value === activeGroupId.value) {
      await loadImages()
    }
  } catch (error: any) {
    ElMessage.error(error.message || '上传失败')
    uploadLoading.value = false
  }
}

const handleEdit = (row: ImageUrl) => {
  editForm.id = row.id
  editForm.group_id = row.group_id
  editForm.url = row.url
  editForm.status = row.status
  editDialogVisible.value = true
}

const handleUpdate = async () => {
  if (!editForm.url.trim()) {
    ElMessage.warning('请输入图片URL')
    return
  }
  editLoading.value = true
  try {
    await updateImageUrl(editForm.id, {
      url: editForm.url.trim(),
      group_id: editForm.group_id,
      status: editForm.status
    })
    ElMessage.success('更新成功')
    editDialogVisible.value = false
    loadImages()
  } catch (e) {
    ElMessage.error((e as Error).message || '更新失败')
  } finally {
    editLoading.value = false
  }
}

const handleDelete = (row: ImageUrl) => {
  ElMessageBox.confirm('确定要删除这个图片URL吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await deleteImageUrl(row.id)
      ElMessage.success('删除成功')
      loadImages()
    } catch (e) {
      ElMessage.warning((e as Error).message || '删除失败')
    }
  })
}

// 批量操作
const handleBatchDelete = () => {
  if (selectedItems.value.length === 0) return
  ElMessageBox.confirm(`确定要删除选中的 ${selectedItems.value.length} 个图片URL吗？`, '批量删除', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const ids = selectedItems.value.map(item => item.id)
      const res = await batchDeleteImages(ids)
      ElMessage.success(`成功删除 ${res.deleted} 个图片URL`)
      clearSelection()
      loadImages()
    } catch (e) {
      ElMessage.error((e as Error).message || '批量删除失败')
    }
  })
}

const handleBatchEnable = async () => {
  if (selectedItems.value.length === 0) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const res = await batchUpdateImageStatus(ids, 1)
    ElMessage.success(`成功启用 ${res.updated} 个图片URL`)
    clearSelection()
    loadImages()
  } catch (e) {
    ElMessage.error((e as Error).message || '批量启用失败')
  }
}

const handleBatchDisable = async () => {
  if (selectedItems.value.length === 0) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const res = await batchUpdateImageStatus(ids, 0)
    ElMessage.success(`成功禁用 ${res.updated} 个图片URL`)
    clearSelection()
    loadImages()
  } catch (e) {
    ElMessage.error((e as Error).message || '批量禁用失败')
  }
}

const handleBatchMove = async (targetGroupId: number) => {
  if (selectedItems.value.length === 0) return
  if (targetGroupId === activeGroupId.value) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const res = await batchMoveImages(ids, targetGroupId)
    const targetGroup = groups.value.find(g => g.id === targetGroupId)
    ElMessage.success(`成功移动 ${res.moved} 个图片URL到 "${targetGroup?.name}"`)
    clearSelection()
    loadImages()
  } catch (e) {
    ElMessage.error((e as Error).message || '批量移动失败')
  }
}

const handleReload = async () => {
  reloadLoading.value = true
  try {
    const res = await reloadImageGroup(activeGroupId.value || undefined)
    ElMessage.success(`刷新成功，共 ${res.total} 个图片URL`)
  } finally {
    reloadLoading.value = false
  }
}

// 删除全部
const handleDeleteAll = async (command: 'group' | 'all') => {
  const currentGroup = groups.value.find(g => g.id === activeGroupId.value)

  try {
    if (command === 'group') {
      // 删除当前分组 - 单次确认
      await ElMessageBox.confirm(
        `确定要删除 "${currentGroup?.name}" 中的所有图片URL吗？此操作不可恢复！`,
        '警告',
        { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' }
      )
      const res = await deleteAllImages(activeGroupId.value)
      ElMessage.success(`成功删除 ${res.deleted} 个图片URL`)
    } else {
      // 删除全部 - 输入确认文字
      await ElMessageBox.prompt(
        '此操作将删除所有图库中的全部图片URL！请输入 "确认删除" 以继续',
        '危险操作',
        {
          confirmButtonText: '确定删除',
          cancelButtonText: '取消',
          type: 'error',
          inputPattern: /^确认删除$/,
          inputErrorMessage: '请输入正确的确认文字'
        }
      )
      const res = await deleteAllImages()
      ElMessage.success(`成功删除 ${res.deleted} 个图片URL`)
    }
    loadImages()
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
  loadImages()
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style lang="scss" scoped>
// 使用全局样式 .group-list-page
// 此处仅保留该页面特有的样式

.image-list {
  .url-link {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: inline-block;
  }

  .image-error {
    width: 50px;
    height: 50px;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: #f5f7fa;
    border-radius: 4px;
    color: #c0c4cc;
  }
}
</style>
