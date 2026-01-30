<template>
  <div ref="editorContainer" class="monaco-editor-container" :style="{ height: height }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import * as monaco from 'monaco-editor'

const props = defineProps<{
  modelValue: string
  language?: string
  theme?: string
  height?: string
  readOnly?: boolean
  options?: monaco.editor.IStandaloneEditorConstructionOptions
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'change', value: string): void
  (e: 'save'): void
}>()

const editorContainer = ref<HTMLElement>()
let editor: monaco.editor.IStandaloneCodeEditor | null = null

onMounted(() => {
  if (!editorContainer.value) return

  // 配置 Monaco Editor 的 worker
  self.MonacoEnvironment = {
    getWorker: function (_moduleId: string, label: string) {
      const getWorkerModule = (moduleUrl: string, label: string) => {
        return new Worker(
          new URL(moduleUrl, import.meta.url),
          { type: 'module', name: label }
        )
      }

      switch (label) {
        case 'json':
          return getWorkerModule('monaco-editor/esm/vs/language/json/json.worker?worker', label)
        case 'css':
        case 'scss':
        case 'less':
          return getWorkerModule('monaco-editor/esm/vs/language/css/css.worker?worker', label)
        case 'html':
        case 'handlebars':
        case 'razor':
          return getWorkerModule('monaco-editor/esm/vs/language/html/html.worker?worker', label)
        case 'typescript':
        case 'javascript':
          return getWorkerModule('monaco-editor/esm/vs/language/typescript/ts.worker?worker', label)
        default:
          return getWorkerModule('monaco-editor/esm/vs/editor/editor.worker?worker', label)
      }
    }
  }

  editor = monaco.editor.create(editorContainer.value, {
    value: props.modelValue,
    language: props.language || 'html',
    theme: props.theme || 'vs-dark',
    readOnly: props.readOnly || false,
    automaticLayout: true,
    minimap: { enabled: true },
    fontSize: 14,
    lineNumbers: 'on',
    wordWrap: 'on',
    scrollBeyondLastLine: false,
    folding: true,
    formatOnPaste: true,
    formatOnType: true,
    tabSize: 2,
    ...props.options
  })

  // 监听内容变化
  editor.onDidChangeModelContent(() => {
    const value = editor?.getValue() || ''
    emit('update:modelValue', value)
    emit('change', value)
  })

  // 添加 Ctrl+S 快捷键保存
  editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
    emit('save')
  })
})

onBeforeUnmount(() => {
  if (editor) {
    editor.dispose()
    editor = null
  }
})

// 监听外部值变化
watch(() => props.modelValue, (newValue) => {
  if (editor && newValue !== editor.getValue()) {
    editor.setValue(newValue)
  }
})

// 监听语言变化
watch(() => props.language, (newLang) => {
  if (editor && newLang) {
    const model = editor.getModel()
    if (model) {
      monaco.editor.setModelLanguage(model, newLang)
    }
  }
})

// 监听主题变化
watch(() => props.theme, (newTheme) => {
  if (newTheme) {
    monaco.editor.setTheme(newTheme)
  }
})

// 暴露方法
defineExpose({
  getEditor: () => editor,
  getValue: () => editor?.getValue() || '',
  setValue: (value: string) => editor?.setValue(value),
  formatDocument: () => {
    editor?.getAction('editor.action.formatDocument')?.run()
  }
})
</script>

<style scoped>
.monaco-editor-container {
  width: 100%;
  min-height: 400px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
}
</style>
