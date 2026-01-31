<template>
  <div class="file-editor">
    <!-- 工具栏 -->
    <div class="editor-toolbar">
      <span class="file-path">
        <el-icon><Document /></el-icon>
        {{ filePath }}
      </span>
      <div class="actions">
        <el-button type="primary" :icon="VideoPlay" @click="runFile" :loading="running">
          运行
        </el-button>
        <el-button type="success" :icon="Check" @click="handleSave" :loading="saving">
          保存
        </el-button>
        <el-button @click="$emit('close')">关闭</el-button>
      </div>
    </div>

    <!-- Monaco 编辑器 -->
    <div class="editor-container" ref="editorContainer"></div>

    <!-- 运行日志 -->
    <div class="log-panel">
      <div class="log-header">
        <span>运行日志</span>
        <el-button text @click="clearLog" size="small">清空</el-button>
      </div>
      <div class="log-content" ref="logContainer">
        <div
          v-for="(log, index) in logs"
          :key="index"
          :class="['log-line', log.type]"
        >
          {{ log.data }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { Document, VideoPlay, Check } from '@element-plus/icons-vue'
import * as monaco from 'monaco-editor'
import { runFile as runFileApi } from '@/api/worker'

const props = defineProps<{
  filePath: string
  content: string
}>()

const emit = defineEmits<{
  (e: 'save', content: string): void
  (e: 'close'): void
}>()

const editorContainer = ref<HTMLElement>()
const logContainer = ref<HTMLElement>()
let editor: monaco.editor.IStandaloneCodeEditor | null = null

const logs = ref<{ type: string; data: string }[]>([])
const running = ref(false)
const saving = ref(false)
let stopRun: (() => void) | null = null

onMounted(() => {
  if (editorContainer.value) {
    editor = monaco.editor.create(editorContainer.value, {
      value: props.content,
      language: 'python',
      theme: 'vs-dark',
      automaticLayout: true,
      minimap: { enabled: true },
      fontSize: 14,
      tabSize: 4,
      lineNumbers: 'on',
      scrollBeyondLastLine: false,
    })

    // Ctrl+S 保存
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
      handleSave()
    })
  }
})

onUnmounted(() => {
  editor?.dispose()
  stopRun?.()
})

function handleSave() {
  if (!editor) return
  saving.value = true
  emit('save', editor.getValue())
  setTimeout(() => {
    saving.value = false
  }, 500)
}

function runFile() {
  if (!editor) return

  running.value = true
  logs.value = []
  logs.value.push({ type: 'info', data: `> python ${props.filePath}` })

  stopRun = runFileApi(props.filePath, {
    onStdout: (data) => {
      logs.value.push({ type: 'stdout', data })
      scrollToBottom()
    },
    onStderr: (data) => {
      logs.value.push({ type: 'stderr', data })
      scrollToBottom()
    },
    onDone: (exitCode, durationMs) => {
      logs.value.push({
        type: 'info',
        data: `> 进程退出，code=${exitCode}，耗时 ${durationMs}ms`
      })
      running.value = false
      scrollToBottom()
    },
    onError: (error) => {
      logs.value.push({ type: 'stderr', data: error })
      running.value = false
      scrollToBottom()
    }
  })
}

function clearLog() {
  logs.value = []
}

function scrollToBottom() {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}
</script>

<style scoped>
.file-editor {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 150px);
}

.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 15px;
  background: #f5f7fa;
  border-radius: 4px;
  margin-bottom: 10px;
}

.file-path {
  display: flex;
  align-items: center;
  gap: 5px;
  font-weight: 500;
}

.actions {
  display: flex;
  gap: 10px;
}

.editor-container {
  flex: 1;
  min-height: 400px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
}

.log-panel {
  margin-top: 10px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  max-height: 200px;
  display: flex;
  flex-direction: column;
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f5f7fa;
  border-bottom: 1px solid #dcdfe6;
  font-weight: 500;
}

.log-content {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
  background: #1e1e1e;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 13px;
}

.log-line {
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.6;
}

.log-line.stdout {
  color: #d4d4d4;
}

.log-line.stderr {
  color: #f48771;
}

.log-line.info {
  color: #808080;
}
</style>
