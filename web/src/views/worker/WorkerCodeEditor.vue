<template>
  <div class="worker-code-editor">
    <!-- 页面头部 -->
    <div class="page-header">
      <h2>Worker 代码管理</h2>
      <div class="header-actions">
        <el-button
          type="warning"
          :icon="Refresh"
          :loading="restarting"
          @click="handleRestart"
        >
          重启 Worker
        </el-button>
        <el-button
          type="danger"
          :icon="Setting"
          :loading="rebuilding"
          @click="handleRebuild"
        >
          重新构建
        </el-button>
      </div>
    </div>

    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 侧边栏 -->
      <FileTree
        :width="store.sidebarWidth"
        @update:width="store.sidebarWidth = $event"
        @create-file="showCreateDialog('file', $event)"
        @create-dir="showCreateDialog('dir', $event)"
        @rename="showRenameDialog"
        @delete="handleDelete"
        @run="handleRunFromTree"
      />

      <!-- 编辑区 -->
      <div class="editor-area">
        <EditorTabs />
        <CodeEditor />
        <LogPanel />
      </div>
    </div>

    <!-- 新建弹窗 -->
    <CreateDialog
      v-model="createDialogVisible"
      :type="createType"
      @confirm="handleCreate"
    />

    <!-- 重命名弹窗 -->
    <RenameDialog
      v-model="renameDialogVisible"
      :current-name="renamingNode?.name || ''"
      @confirm="handleRename"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Setting } from '@element-plus/icons-vue'
import { useWorkerEditorStore, type TreeNode } from '@/stores/workerEditor'
import {
  createItem,
  deleteItem,
  moveItem,
  restartWorker,
  rebuildWorker
} from '@/api/worker'

import FileTree from './components/FileTree.vue'
import EditorTabs from './components/EditorTabs.vue'
import CodeEditor from './components/CodeEditor.vue'
import LogPanel from './components/LogPanel.vue'
import CreateDialog from './components/CreateDialog.vue'
import RenameDialog from './components/RenameDialog.vue'

const store = useWorkerEditorStore()

// 控制状态
const restarting = ref(false)
const rebuilding = ref(false)

// 弹窗状态
const createDialogVisible = ref(false)
const createType = ref<'file' | 'dir'>('file')
const createParentPath = ref('')

const renameDialogVisible = ref(false)
const renamingNode = ref<TreeNode | null>(null)

// 新建
function showCreateDialog(type: 'file' | 'dir', parentPath?: string) {
  createType.value = type
  createParentPath.value = parentPath || ''
  createDialogVisible.value = true
}

async function handleCreate(name: string) {
  try {
    await createItem(createParentPath.value, name, createType.value)
    ElMessage.success('创建成功')
    store.loadFileTree()
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  }
}

// 重命名
function showRenameDialog(node: TreeNode) {
  renamingNode.value = node
  renameDialogVisible.value = true
}

async function handleRename(newName: string) {
  if (!renamingNode.value) return

  const oldPath = renamingNode.value.path
  const parentPath = oldPath.split('/').slice(0, -1).join('/')
  const newPath = parentPath ? `${parentPath}/${newName}` : newName

  try {
    await moveItem(oldPath, newPath)
    ElMessage.success('重命名成功')
    store.loadFileTree()
  } catch (e: any) {
    ElMessage.error(e.message || '重命名失败')
  }
}

// 删除
async function handleDelete(node: TreeNode) {
  try {
    await ElMessageBox.confirm(
      `确定删除 ${node.name} 吗？${node.type === 'dir' ? '目录下所有文件都将被删除。' : ''}`,
      '确认删除',
      { type: 'warning' }
    )
    await deleteItem(node.path)
    ElMessage.success('删除成功')
    store.loadFileTree()

    // 如果删除的文件已打开，关闭对应标签
    const tab = store.tabs.find(t => t.path === node.path)
    if (tab) {
      store.closeTab(tab.id)
    }
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '删除失败')
    }
  }
}

// 从目录树运行文件
function handleRunFromTree(node: TreeNode) {
  // 先打开文件
  store.openFile(node.path, node.name)
  // 然后触发运行（由 CodeEditor 处理）
}

// 重启 Worker
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

// 重建 Worker
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
</script>

<style scoped>
.worker-code-editor {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 100px);
  background: #1e1e1e;
  border-radius: 4px;
  overflow: hidden;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #2d2d2d;
  border-bottom: 1px solid #3c3c3c;
}

.page-header h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
  color: #cccccc;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.main-content {
  display: flex;
  flex: 1;
  min-height: 0;
}

.editor-area {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}
</style>
