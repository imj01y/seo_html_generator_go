<template>
  <div class="code-editor-panel">
    <!-- 页面头部 -->
    <div class="panel-header">
      <h2>{{ title }}</h2>
      <div class="header-actions">
        <slot name="header-actions">
          <el-button
            v-if="showRestartButton"
            type="warning"
            :icon="Refresh"
            :loading="restarting"
            @click="$emit('restart')"
          >
            重启
          </el-button>
          <el-button
            v-if="showRebuildButton"
            type="danger"
            :icon="Setting"
            :loading="rebuilding"
            @click="$emit('rebuild')"
          >
            重新构建
          </el-button>
        </slot>
      </div>
    </div>

    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 侧边栏 -->
      <FileTree
        :width="store.sidebarWidth.value"
        :title="fileTreeTitle"
        :store="store"
        :api="api"
        :runnable="runnable"
        :runnable-extensions="runnableExtensions"
        @update:width="store.sidebarWidth.value = $event"
        @create-file="showCreateDialog('file', $event)"
        @create-dir="showCreateDialog('dir', $event)"
        @rename="showRenameDialog"
        @delete="handleDelete"
        @run="handleRunFromTree"
      />

      <!-- 编辑区 -->
      <div class="editor-area">
        <EditorTabs :store="store" />
        <MonacoEditor
          :store="store"
          :runnable="runnable"
          :runnable-extensions="runnableExtensions"
          @run="handleRun"
        />
        <LogPanel
          v-if="showLogPanel"
          :store="store"
          @stop="handleStop"
        />
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
import { ref, provide, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Setting } from '@element-plus/icons-vue'
import type { TreeNode, CodeEditorApi, CodeEditorPanelProps } from './types'
import { useEditorStore } from './composables/useEditorStore'

import FileTree from './components/FileTree.vue'
import EditorTabs from './components/EditorTabs.vue'
import MonacoEditor from './components/MonacoEditor.vue'
import LogPanel from './components/LogPanel.vue'
import CreateDialog from './components/CreateDialog.vue'
import RenameDialog from './components/RenameDialog.vue'

const props = withDefaults(defineProps<CodeEditorPanelProps>(), {
  title: '代码编辑器',
  runnable: false,
  showLogPanel: false,
  showRestartButton: false,
  showRebuildButton: false,
  runnableExtensions: () => ['.py']
})

const emit = defineEmits<{
  (e: 'restart'): void
  (e: 'rebuild'): void
}>()

// 创建 store 实例
const store = useEditorStore(props.api, props.languageMap)

// 提供给子组件
provide('editorStore', store)
provide('editorApi', props.api)

// 控制状态
const restarting = ref(false)
const rebuilding = ref(false)

// 弹窗状态
const createDialogVisible = ref(false)
const createType = ref<'file' | 'dir'>('file')
const createParentPath = ref('')

const renameDialogVisible = ref(false)
const renamingNode = ref<TreeNode | null>(null)

// 运行控制
let stopRun: (() => void) | null = null

// 计算属性
const fileTreeTitle = props.title?.replace(/管理|编辑器/g, '').trim() || 'FILES'

// 新建
function showCreateDialog(type: 'file' | 'dir', parentPath?: string) {
  createType.value = type
  createParentPath.value = parentPath || ''
  createDialogVisible.value = true
}

async function handleCreate(name: string) {
  try {
    await props.api.createItem(createParentPath.value, name, createType.value)
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
    await props.api.moveItem(oldPath, newPath)
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
    await props.api.deleteItem(node.path)
    ElMessage.success('删除成功')
    store.loadFileTree()

    // 如果删除的文件已打开，关闭对应标签
    const tab = store.tabs.value.find(t => t.path === node.path)
    if (tab) {
      store.closeTab(tab.id)
    }
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '删除失败')
    }
  }
}

// 运行
function handleRunFromTree(node: TreeNode) {
  store.openFile(node.path, node.name)
  // 稍后触发运行
  setTimeout(() => handleRun(), 100)
}

function handleRun() {
  if (!store.activeTab.value || !props.api.runFile) return

  // 停止之前的运行
  if (stopRun) {
    stopRun()
    stopRun = null
  }

  store.clearLogs()
  store.setLogRunning(true)
  store.addLog({ type: 'command', data: `> 运行 ${store.activeTab.value.path}` })

  stopRun = props.api.runFile(store.activeTab.value.path, {
    onStdout: (data) => store.addLog({ type: 'stdout', data }),
    onStderr: (data) => store.addLog({ type: 'stderr', data }),
    onDone: (exitCode, durationMs) => {
      store.addLog({
        type: 'info',
        data: `> 进程退出，code=${exitCode}，耗时 ${durationMs}ms`
      })
      store.setLogRunning(false)
    },
    onError: (error) => {
      store.addLog({ type: 'stderr', data: error })
      store.setLogRunning(false)
    }
  })
}

function handleStop() {
  if (stopRun) {
    stopRun()
    stopRun = null
    store.setLogRunning(false)
    store.addLog({ type: 'info', data: '> 已停止运行' })
  }
}

// 暴露方法给父组件
defineExpose({
  store,
  refresh: () => store.loadFileTree()
})
</script>

<style scoped>
.code-editor-panel {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 100px);
  background: #1e1e1e;
  border-radius: 4px;
  overflow: hidden;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #2d2d2d;
  border-bottom: 1px solid #3c3c3c;
}

.panel-header h2 {
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
