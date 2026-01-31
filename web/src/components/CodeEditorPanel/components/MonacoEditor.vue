<template>
  <div class="monaco-editor-wrapper">
    <!-- 工具栏 -->
    <div class="editor-toolbar" v-if="store.activeTab.value">
      <span class="file-path">{{ store.activeTab.value.path }}</span>
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
    <div class="editor-container" ref="editorContainer">
      <div v-if="!store.activeTab.value" class="empty-editor">
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
import type { EditorStore } from '../composables/useEditorStore'
import { initPyCharmDarcula } from '../themes/pycharm-darcula'

const props = defineProps<{
  store: EditorStore
  runnable?: boolean
  runnableExtensions?: string[]
}>()

const emit = defineEmits<{
  (e: 'run'): void
}>()

const editorContainer = ref<HTMLElement>()
const saving = ref(false)

let editor: monaco.editor.IStandaloneCodeEditor | null = null

// 初始化 PyCharm Darcula 主题
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

// 监听活动标签变化
watch(() => props.store.activeTab.value, (tab) => {
  if (tab && editor) {
    const model = monaco.editor.createModel(tab.content, tab.language)
    editor.setModel(model)
  }
}, { immediate: true })

// 监听标签内容变化（外部加载）
watch(() => props.store.activeTab.value?.content, (content) => {
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

      // 内容变化时更新 store
      editor.onDidChangeModelContent(() => {
        if (props.store.activeTab.value && editor) {
          props.store.updateTabContent(props.store.activeTab.value.id, editor.getValue())
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
  editor?.dispose()
})

async function handleSave() {
  if (!props.store.activeTab.value || !isModified.value) return

  saving.value = true
  try {
    await props.store.saveTab(props.store.activeTab.value.id)
    ElMessage.success('保存成功')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
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

.file-path {
  font-size: 12px;
  color: #bbbdc0;
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
  background: #1e1f22;
}

.empty-editor p {
  margin-top: 16px;
  font-size: 14px;
}
</style>
