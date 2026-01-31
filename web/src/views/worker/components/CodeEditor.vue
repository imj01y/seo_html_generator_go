<template>
  <div class="code-editor">
    <!-- 工具栏 -->
    <div class="editor-toolbar" v-if="store.activeTab">
      <span class="file-path">{{ store.activeTab.path }}</span>
      <div class="actions">
        <el-button
          v-if="isPythonFile"
          type="primary"
          size="small"
          :icon="VideoPlay"
          :loading="store.logRunning"
          @click="handleRun"
        >
          运行
        </el-button>
        <el-button
          type="success"
          size="small"
          :icon="Check"
          :loading="saving"
          :disabled="!isModified"
          @click="handleSave"
        >
          保存
        </el-button>
      </div>
    </div>

    <!-- Monaco 编辑器 -->
    <div class="editor-container" ref="editorContainer">
      <div v-if="!store.activeTab" class="empty-editor">
        <el-icon :size="48"><Files /></el-icon>
        <p>选择文件开始编辑</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { VideoPlay, Check, Files } from '@element-plus/icons-vue'
import * as monaco from 'monaco-editor'
import { useWorkerEditorStore } from '@/stores/workerEditor'
import { runFile } from '@/api/worker'

const store = useWorkerEditorStore()
const editorContainer = ref<HTMLElement>()
const saving = ref(false)

let editor: monaco.editor.IStandaloneCodeEditor | null = null
let stopRun: (() => void) | null = null

const isPythonFile = computed(() =>
  store.activeTab?.name.endsWith('.py') ?? false
)

const isModified = computed(() =>
  store.activeTab ? store.isTabModified(store.activeTab.id) : false
)

// 监听活动标签变化
watch(() => store.activeTab, (tab) => {
  if (tab && editor) {
    const model = monaco.editor.createModel(tab.content, tab.language)
    editor.setModel(model)
  }
}, { immediate: true })

// 监听标签内容变化（外部加载）
watch(() => store.activeTab?.content, (content) => {
  if (content !== undefined && editor) {
    const currentValue = editor.getValue()
    if (currentValue !== content) {
      editor.setValue(content)
    }
  }
})

onMounted(() => {
  nextTick(() => {
    if (editorContainer.value) {
      editor = monaco.editor.create(editorContainer.value, {
        value: store.activeTab?.content || '',
        language: store.activeTab?.language || 'plaintext',
        theme: 'vs-dark',
        automaticLayout: true,
        minimap: { enabled: true },
        fontSize: 14,
        tabSize: 4,
        lineNumbers: 'on',
        scrollBeyondLastLine: false,
        wordWrap: 'on'
      })

      // 内容变化时更新 store
      editor.onDidChangeModelContent(() => {
        if (store.activeTab && editor) {
          store.updateTabContent(store.activeTab.id, editor.getValue())
        }
      })

      // Ctrl+S 保存
      editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
        handleSave()
      })
    }
  })
})

onUnmounted(() => {
  stopRun?.()
  editor?.dispose()
})

async function handleSave() {
  if (!store.activeTab || !isModified.value) return

  saving.value = true
  try {
    await store.saveTab(store.activeTab.id)
    ElMessage.success('保存成功')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

function handleRun() {
  if (!store.activeTab) return

  // 停止之前的运行
  if (stopRun) {
    stopRun()
    stopRun = null
  }

  store.clearLogs()
  store.setLogRunning(true)
  store.addLog({ type: 'command', data: `> python ${store.activeTab.path}` })

  stopRun = runFile(store.activeTab.path, {
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
</script>

<style scoped>
.code-editor {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 12px;
  background: #2d2d2d;
  border-bottom: 1px solid #3c3c3c;
}

.file-path {
  font-size: 12px;
  color: #969696;
}

.actions {
  display: flex;
  gap: 8px;
}

.editor-container {
  flex: 1;
  min-height: 0;
}

.empty-editor {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #6e6e6e;
  background: #1e1e1e;
}

.empty-editor p {
  margin-top: 16px;
  font-size: 14px;
}
</style>
