<template>
  <div class="worker-code-manager">
    <!-- 页面标题和操作 -->
    <div class="page-header">
      <h2>Worker 代码管理</h2>
      <div class="header-actions">
        <el-button type="warning" :icon="Refresh" @click="handleRestart" :loading="restarting">
          重启 Worker
        </el-button>
        <el-button type="danger" :icon="Setting" @click="handleRebuild" :loading="rebuilding">
          重新构建
        </el-button>
      </div>
    </div>

    <!-- 编辑器模式 -->
    <FileEditor
      v-if="editingFile"
      :file-path="editingFile.path"
      :content="editingFile.content"
      @save="handleSave"
      @close="closeEditor"
    />

    <!-- 文件列表模式 -->
    <template v-else>
      <!-- 工具栏 -->
      <FileToolbar
        :current-path="currentPath"
        @navigate="navigateTo"
        @upload-success="loadDir"
        @create-file="showCreateDialog('file')"
        @create-dir="showCreateDialog('dir')"
      />

      <!-- 文件列表 -->
      <FileTable
        :files="files"
        :loading="loading"
        :current-path="currentPath"
        @open="handleOpen"
        @edit="handleEdit"
        @rename="showRenameDialog"
        @move="showMoveDialog"
        @download="handleDownload"
        @delete="handleDelete"
        @upload-success="loadDir"
      />
    </template>

    <!-- 新建弹窗 -->
    <CreateDialog
      v-model="createDialogVisible"
      :type="createType"
      @confirm="handleCreate"
    />

    <!-- 重命名弹窗 -->
    <RenameDialog
      v-model="renameDialogVisible"
      :current-name="renamingItem?.name || ''"
      @confirm="handleRename"
    />

    <!-- 移动弹窗 -->
    <MoveDialog
      v-model="moveDialogVisible"
      :file-path="movingItem?.name || ''"
      @confirm="handleMove"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Setting } from '@element-plus/icons-vue'
import FileToolbar from './components/FileToolbar.vue'
import FileTable from './components/FileTable.vue'
import FileEditor from './components/FileEditor.vue'
import CreateDialog from './components/CreateDialog.vue'
import RenameDialog from './components/RenameDialog.vue'
import MoveDialog from './components/MoveDialog.vue'
import {
  getDir,
  getFile,
  saveFile,
  createItem,
  deleteItem,
  moveItem,
  getDownloadUrl,
  restartWorker,
  rebuildWorker,
  type FileInfo
} from '@/api/worker'

// 状态
const currentPath = ref('')
const files = ref<FileInfo[]>([])
const loading = ref(false)
const restarting = ref(false)
const rebuilding = ref(false)

// 编辑状态
const editingFile = ref<{ path: string; content: string } | null>(null)

// 弹窗状态
const createDialogVisible = ref(false)
const createType = ref<'file' | 'dir'>('file')
const renameDialogVisible = ref(false)
const renamingItem = ref<FileInfo | null>(null)
const moveDialogVisible = ref(false)
const movingItem = ref<FileInfo | null>(null)

// 加载目录
async function loadDir() {
  loading.value = true
  try {
    const res = await getDir(currentPath.value)
    files.value = res.files
  } catch (e: any) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

// 导航
function navigateTo(path: string) {
  currentPath.value = path
  loadDir()
}

// 打开（双击）
function handleOpen(item: FileInfo) {
  if (item.type === 'dir') {
    currentPath.value = currentPath.value ? `${currentPath.value}/${item.name}` : item.name
    loadDir()
  } else {
    handleEdit(item)
  }
}

// 编辑文件
async function handleEdit(item: FileInfo) {
  try {
    const path = currentPath.value ? `${currentPath.value}/${item.name}` : item.name
    const res = await getFile(path)
    editingFile.value = { path, content: res.content }
  } catch (e: any) {
    ElMessage.error(e.message || '读取文件失败')
  }
}

// 保存文件
async function handleSave(content: string) {
  if (!editingFile.value) return
  try {
    await saveFile(editingFile.value.path, content)
    ElMessage.success('保存成功')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  }
}

// 关闭编辑器
function closeEditor() {
  editingFile.value = null
}

// 新建
function showCreateDialog(type: 'file' | 'dir') {
  createType.value = type
  createDialogVisible.value = true
}

async function handleCreate(name: string) {
  try {
    await createItem(currentPath.value, name, createType.value)
    ElMessage.success('创建成功')
    loadDir()
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  }
}

// 重命名
function showRenameDialog(item: FileInfo) {
  renamingItem.value = item
  renameDialogVisible.value = true
}

async function handleRename(newName: string) {
  if (!renamingItem.value) return
  const oldPath = currentPath.value
    ? `${currentPath.value}/${renamingItem.value.name}`
    : renamingItem.value.name
  const newPath = currentPath.value ? `${currentPath.value}/${newName}` : newName
  try {
    await moveItem(oldPath, newPath)
    ElMessage.success('重命名成功')
    loadDir()
  } catch (e: any) {
    ElMessage.error(e.message || '重命名失败')
  }
}

// 移动
function showMoveDialog(item: FileInfo) {
  movingItem.value = item
  moveDialogVisible.value = true
}

async function handleMove(targetDir: string) {
  if (!movingItem.value) return
  const oldPath = currentPath.value
    ? `${currentPath.value}/${movingItem.value.name}`
    : movingItem.value.name
  const newPath = `${targetDir}/${movingItem.value.name}`
  try {
    await moveItem(oldPath, newPath)
    ElMessage.success('移动成功')
    loadDir()
  } catch (e: any) {
    ElMessage.error(e.message || '移动失败')
  }
}

// 下载
function handleDownload(item: FileInfo) {
  const path = currentPath.value ? `${currentPath.value}/${item.name}` : item.name
  window.open(getDownloadUrl(path), '_blank')
}

// 删除
async function handleDelete(item: FileInfo) {
  try {
    await ElMessageBox.confirm(`确定删除 ${item.name} 吗？`, '确认删除', {
      type: 'warning'
    })
    const path = currentPath.value ? `${currentPath.value}/${item.name}` : item.name
    await deleteItem(path)
    ElMessage.success('删除成功')
    loadDir()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '删除失败')
    }
  }
}

// 重启
async function handleRestart() {
  try {
    await ElMessageBox.confirm('确定重启 Worker 吗？', '确认重启', { type: 'warning' })
    restarting.value = true
    await restartWorker()
    ElMessage.success('重启指令已发送')
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '重启失败')
    }
  } finally {
    restarting.value = false
  }
}

// 重建
async function handleRebuild() {
  try {
    await ElMessageBox.confirm(
      '重新构建将重新安装所有依赖，可能需要几分钟时间。确定继续吗？',
      '确认重建',
      { type: 'warning' }
    )
    rebuilding.value = true
    await rebuildWorker()
    ElMessage.success('Worker 重新构建完成')
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '重建失败')
    }
  } finally {
    rebuilding.value = false
  }
}

onMounted(() => {
  loadDir()
})
</script>

<style scoped>
.worker-code-manager {
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
}

.header-actions {
  display: flex;
  gap: 10px;
}
</style>
