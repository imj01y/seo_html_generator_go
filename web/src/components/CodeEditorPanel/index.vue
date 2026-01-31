<template>
  <div :class="['code-editor-panel', { maximized: isMaximized }]">
    <!-- 页面头部 -->
    <div class="panel-header">
      <h2>{{ title }}</h2>
      <div class="header-actions">
        <slot name="header-actions">
          <el-tooltip
            v-if="showRestartButton"
            content="安装依赖并重启服务（会中断当前任务）"
            placement="bottom"
          >
            <el-button
              type="warning"
              :icon="Refresh"
              :loading="restarting"
              @click="$emit('restart')"
            >
              重启
            </el-button>
          </el-tooltip>
          <el-tooltip
            v-if="showLogsButton"
            :content="logsActive ? '停止监听日志' : '查看实时运行日志'"
            placement="bottom"
          >
            <el-button
              :type="logsActive ? 'danger' : 'info'"
              :icon="Document"
              @click="$emit('toggle-logs')"
            >
              {{ logsActive ? '停止日志' : '实时日志' }}
            </el-button>
          </el-tooltip>
        </slot>
        <el-tooltip :content="isMaximized ? '还原 (Esc)' : '最大化'" placement="bottom">
          <el-button
            :icon="isMaximized ? Close : FullScreen"
            @click="toggleMaximize"
          />
        </el-tooltip>
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
          :extra-tabs="extraTabs"
          @stop="handleStop"
        />
      </div>
    </div>

    <!-- 统一弹窗 -->
    <PromptDialog
      :visible="dialogVisible"
      :title="dialogConfig.title"
      :mode="dialogConfig.mode"
      :type="dialogConfig.type"
      :message="dialogConfig.message"
      :placeholder="dialogConfig.placeholder"
      :default-value="dialogConfig.defaultValue"
      @confirm="handleDialogConfirm"
      @cancel="dialogVisible = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, provide, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, FullScreen, Close, Document } from '@element-plus/icons-vue'
import type { TreeNode, CodeEditorPanelProps, ExtraTab } from './types'
import { useEditorStore } from './composables/useEditorStore'

import FileTree from './components/FileTree.vue'
import EditorTabs from './components/EditorTabs.vue'
import MonacoEditor from './components/MonacoEditor.vue'
import LogPanel from './components/LogPanel.vue'
import PromptDialog from './components/PromptDialog.vue'

const props = withDefaults(defineProps<CodeEditorPanelProps & {
  extraTabs?: ExtraTab[]
  logsActive?: boolean
}>(), {
  title: '代码编辑器',
  runnable: false,
  showLogPanel: false,
  showRestartButton: false,
  showLogsButton: false,
  logsActive: false,
  runnableExtensions: () => ['.py']
})

defineEmits<{
  (e: 'restart'): void
  (e: 'toggle-logs'): void
}>()

const store = useEditorStore(props.api, props.languageMap)

provide('editorStore', store)
provide('editorApi', props.api)

const restarting = ref(false)
const isMaximized = ref(false)

function toggleMaximize() {
  isMaximized.value = !isMaximized.value
}

function handleKeyDown(event: KeyboardEvent) {
  // Esc 退出最大化
  if (event.key === 'Escape' && isMaximized.value) {
    isMaximized.value = false
    return
  }

  // Ctrl+S 保存
  if ((event.ctrlKey || event.metaKey) && event.key === 's') {
    event.preventDefault()
    event.stopPropagation()
    handleSave()
  }
}

async function handleSave() {
  const tab = store.activeTab.value
  if (!tab) return

  // 检查是否有修改
  if (!store.isTabModified(tab.id)) {
    return
  }

  try {
    await store.saveTab(tab.id)
    ElMessage.success('保存成功')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeyDown)
})

const dialogVisible = ref(false)
const dialogConfig = ref({
  title: '',
  mode: 'input' as 'input' | 'confirm',
  type: 'info' as 'warning' | 'danger' | 'info',
  message: '',
  placeholder: '',
  defaultValue: ''
})
const dialogAction = ref<string>('') // 'create-file', 'create-dir', 'rename', 'delete'
const dialogContext = ref<{ parentPath?: string; node?: TreeNode }>({})

let stopRun: (() => void) | null = null

const fileTreeTitle = props.title?.replace(/管理|编辑器/g, '').trim() || 'FILES'

function showCreateDialog(type: 'file' | 'dir', parentPath?: string) {
  dialogAction.value = type === 'file' ? 'create-file' : 'create-dir'
  dialogContext.value = { parentPath: parentPath || '' }
  dialogConfig.value = {
    title: type === 'file' ? '新建文件' : '新建文件夹',
    mode: 'input',
    type: 'info',
    message: '',
    placeholder: type === 'file' ? '请输入文件名' : '请输入文件夹名',
    defaultValue: ''
  }
  dialogVisible.value = true
}

function showRenameDialog(node: TreeNode) {
  dialogAction.value = 'rename'
  dialogContext.value = { node }
  dialogConfig.value = {
    title: '重命名',
    mode: 'input',
    type: 'info',
    message: '',
    placeholder: '请输入新名称',
    defaultValue: node.name
  }
  dialogVisible.value = true
}

function handleDelete(node: TreeNode) {
  dialogAction.value = 'delete'
  dialogContext.value = { node }
  dialogConfig.value = {
    title: '删除确认',
    mode: 'confirm',
    type: 'danger',
    message: `确定删除 ${node.name} 吗？${node.type === 'dir' ? '目录下所有文件都将被删除。' : ''}`,
    placeholder: '',
    defaultValue: ''
  }
  dialogVisible.value = true
}

async function handleDialogConfirm(value?: string) {
  dialogVisible.value = false

  try {
    switch (dialogAction.value) {
      case 'create-file':
      case 'create-dir': {
        const type = dialogAction.value === 'create-file' ? 'file' : 'dir'
        await props.api.createItem(dialogContext.value.parentPath || '', value!, type)
        ElMessage.success('创建成功')
        store.loadFileTree()
        break
      }

      case 'rename': {
        const node = dialogContext.value.node!
        const oldPath = node.path
        const parentPath = oldPath.split('/').slice(0, -1).join('/')
        const newPath = parentPath ? `${parentPath}/${value}` : value!
        await props.api.moveItem(oldPath, newPath)
        ElMessage.success('重命名成功')
        store.loadFileTree()
        break
      }

      case 'delete': {
        const node = dialogContext.value.node!
        await props.api.deleteItem(node.path)
        ElMessage.success('删除成功')
        store.loadFileTree()

        // 如果删除的文件已打开，关闭对应标签
        const tab = store.tabs.value.find(t => t.path === node.path)
        if (tab) {
          store.closeTab(tab.id)
        }
        break
      }
    }
  } catch (e: any) {
    const actionName = {
      'create-file': '创建',
      'create-dir': '创建',
      'rename': '重命名',
      'delete': '删除'
    }[dialogAction.value] || '操作'
    ElMessage.error(e.message || `${actionName}失败`)
  }
}

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
  transition: all 0.3s ease;
}

.code-editor-panel.maximized {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  height: 100vh;
  z-index: 9999;
  border-radius: 0;
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
