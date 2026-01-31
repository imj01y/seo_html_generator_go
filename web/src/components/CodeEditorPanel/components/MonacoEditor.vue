<template>
  <div class="monaco-editor-wrapper">
    <!-- 工具栏 -->
    <div class="editor-toolbar" v-if="store.activeTab.value">
      <div class="toolbar-left">
        <span class="file-path">{{ store.activeTab.value.path }}</span>
        <span v-if="lastSavedText" class="last-saved">{{ lastSavedText }}</span>
        <span v-if="autoSaving" class="auto-saving">自动保存中...</span>
      </div>
      <div class="actions">
        <el-button
          v-if="canRun"
          type="primary"
          size="small"
          :icon="VideoPlay"
          :loading="store.logRunning.value"
          @click="$emit('run')"
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
    <div class="editor-container">
      <div v-if="!store.activeTab.value" class="empty-editor">
        <el-icon :size="48"><Files /></el-icon>
        <p>选择文件开始编辑</p>
      </div>
      <div v-show="store.activeTab.value" class="monaco-mount" ref="editorContainer"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { VideoPlay, Check, Files } from '@element-plus/icons-vue'
import * as monaco from 'monaco-editor'
import type { EditorStore } from '../composables/useEditorStore'
import { initPyCharmDarcula } from '../themes/pycharm-darcula'

const props = withDefaults(defineProps<{
  store: EditorStore
  runnable?: boolean
  runnableExtensions?: string[]
  autoSave?: boolean
  autoSaveDelay?: number
}>(), {
  autoSave: true,
  autoSaveDelay: 3000
})

const emit = defineEmits<{
  (e: 'run'): void
}>()

const editorContainer = ref<HTMLElement>()
const saving = ref(false)
const autoSaving = ref(false)
const now = ref(new Date())

let editor: monaco.editor.IStandaloneCodeEditor | null = null
let autoSaveTimer: ReturnType<typeof setTimeout> | null = null
let nowTimer: ReturnType<typeof setInterval> | null = null

initPyCharmDarcula()

const runnableExts = computed(() => props.runnableExtensions || ['.py'])

const canRun = computed(() => {
  if (!props.runnable) return false
  const tab = props.store.activeTab.value
  if (!tab) return false
  return runnableExts.value.some(ext => tab.name.endsWith(ext))
})

const isModified = computed(() =>
  props.store.activeTab.value ? props.store.isTabModified(props.store.activeTab.value.id) : false
)

const lastSavedText = computed(() => {
  const tab = props.store.activeTab.value
  if (!tab?.lastSavedAt) return ''

  const diff = now.value.getTime() - tab.lastSavedAt.getTime()
  const seconds = Math.floor(diff / 1000)

  if (seconds < 5) return '刚刚保存'
  if (seconds < 60) return `${seconds}秒前保存`

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}分钟前保存`

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}小时前保存`

  return tab.lastSavedAt.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }) + ' 保存'
})

watch(() => props.store.activeTab.value, (tab) => {
  if (tab && editor) {
    const model = monaco.editor.createModel(tab.content, tab.language)
    editor.setModel(model)
  }
  clearAutoSaveTimer()
}, { immediate: true })

watch(() => props.store.activeTab.value?.content, (content) => {
  if (content !== undefined && editor) {
    const currentValue = editor.getValue()
    if (currentValue !== content) {
      editor.setValue(content)
    }
  }
})

watch(isModified, (modified) => {
  if (modified && props.autoSave) {
    scheduleAutoSave()
  } else {
    clearAutoSaveTimer()
  }
})

onMounted(() => {
  nowTimer = setInterval(() => {
    now.value = new Date()
  }, 10000)

  nextTick(() => {
    if (editorContainer.value) {
      editor = monaco.editor.create(editorContainer.value, {
        value: props.store.activeTab.value?.content || '',
        language: props.store.activeTab.value?.language || 'plaintext',
        theme: 'pycharm-darcula',
        automaticLayout: true,
        minimap: { enabled: true },
        fontSize: 14,
        tabSize: 4,
        lineNumbers: 'on',
        scrollBeyondLastLine: false,
        wordWrap: 'on'
      })

      editor.onDidChangeModelContent(() => {
        if (props.store.activeTab.value && editor) {
          props.store.updateTabContent(props.store.activeTab.value.id, editor.getValue())
        }
      })

      editor.addAction({
        id: 'save-file',
        label: '保存文件',
        keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS],
        run: () => {
          handleSave()
        }
      })
    }
  })
})

onUnmounted(() => {
  editor?.dispose()
  clearAutoSaveTimer()
  if (nowTimer) {
    clearInterval(nowTimer)
  }
})

function clearAutoSaveTimer() {
  if (autoSaveTimer) {
    clearTimeout(autoSaveTimer)
    autoSaveTimer = null
  }
}

function scheduleAutoSave() {
  clearAutoSaveTimer()
  autoSaveTimer = setTimeout(() => {
    handleAutoSave()
  }, props.autoSaveDelay)
}

async function handleAutoSave() {
  const tab = props.store.activeTab.value
  if (!tab || !isModified.value) return

  autoSaving.value = true
  try {
    await props.store.saveTab(tab.id)
  } catch (e) {
    console.error('Auto save failed:', e)
  } finally {
    autoSaving.value = false
  }
}

async function handleSave() {
  const tab = props.store.activeTab.value
  if (!tab) return

  if (!isModified.value) {
    ElMessage.info('文件未修改')
    return
  }

  clearAutoSaveTimer()

  saving.value = true
  try {
    await props.store.saveTab(tab.id)
    ElMessage.success('保存成功')
  } catch (e: unknown) {
    const message = e instanceof Error ? e.message : '保存失败'
    ElMessage.error(message)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.monaco-editor-wrapper {
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
  background: #2b2d30;
  border-bottom: 1px solid #1e1f22;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.file-path {
  font-size: 12px;
  color: #bbbdc0;
}

.last-saved {
  font-size: 11px;
  color: #6e6e6e;
}

.auto-saving {
  font-size: 11px;
  color: #3794ff;
}

.actions {
  display: flex;
  gap: 8px;
}

.editor-container {
  flex: 1;
  min-height: 0;
  position: relative;
}

.monaco-mount {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
}

.empty-editor {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #6e6e6e;
  background: #1e1f22;
}

.empty-editor p {
  margin-top: 16px;
  font-size: 14px;
}
</style>
