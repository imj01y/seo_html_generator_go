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

// 注册自定义 Python 语法高亮（精确匹配 PyCharm Darcula 配色）
monaco.languages.setMonarchTokensProvider('python', {
  defaultToken: '',
  tokenPostfix: '.python',

  brackets: [
    { open: '{', close: '}', token: 'delimiter.curly' },
    { open: '[', close: ']', token: 'delimiter.bracket' },
    { open: '(', close: ')', token: 'delimiter.parenthesis' }
  ],

  tokenizer: {
    root: [
      // 空白
      [/\s+/, 'white'],

      // 文档字符串（三引号）
      [/"""/, 'string.docstring', '@docstring_double'],
      [/'''/, 'string.docstring', '@docstring_single'],

      // 装饰器 - 黄绿色
      [/@[a-zA-Z_]\w*/, 'decorator'],

      // self.属性 或 cls.属性 - 整体处理：self 紫色斜体，属性紫色
      [/\b(self|cls)(\.)([a-zA-Z_]\w*)/, ['variable.self', 'delimiter', 'variable.instance']],

      // 单独的 self, cls - 紫色斜体
      [/\b(self|cls)\b/, 'variable.self'],

      // 函数定义：def func_name
      [/\b(def)(\s+)([a-zA-Z_]\w*)/, ['keyword', 'white', 'function.declaration']],

      // 类定义：class ClassName
      [/\b(class)(\s+)([a-zA-Z_]\w*)/, ['keyword', 'white', 'class.declaration']],

      // 全大写常量 - 紫色
      [/\b[A-Z][A-Z_0-9]+\b/, 'constant'],

      // 私有属性（以下划线开头）- 紫色
      [/\b_[a-zA-Z_]\w*\b/, 'variable.instance'],

      // 关键字 - 橙色（不加粗）
      [/\b(and|as|assert|async|await|break|class|continue|def|del|elif|else|except|finally|for|from|global|if|import|in|is|lambda|nonlocal|not|or|pass|raise|return|try|while|with|yield)\b/, 'keyword'],

      // 布尔和 None - 橙色
      [/\b(True|False|None)\b/, 'keyword.constant'],

      // 返回类型注解 -> type
      [/(->)(\s*)([a-zA-Z_]\w*)/, ['operator', 'white', 'type']],

      // 类型注解 : type (在参数中)
      [/(:)(\s*)(int|float|str|bool|list|dict|set|tuple|bytes|object|Optional|List|Dict|Set|Tuple|Any|Union|Callable|Iterable|Iterator|Generator)\b/, ['delimiter', 'white', 'type']],

      // 魔术方法名 __xxx__
      [/__[a-zA-Z_]+__/, 'function.magic'],

      // 普通标识符
      [/[a-zA-Z_]\w*/, 'identifier'],

      // 浮点数
      [/\d+\.\d*([eE][-+]?\d+)?/, 'number.float'],
      [/\d*\.\d+([eE][-+]?\d+)?/, 'number.float'],

      // 十六进制
      [/0[xX][0-9a-fA-F]+/, 'number.hex'],

      // 八进制
      [/0[oO][0-7]+/, 'number.octal'],

      // 二进制
      [/0[bB][01]+/, 'number.binary'],

      // 整数
      [/\d+/, 'number'],

      // f-string
      [/[fF]"/, 'string.fstring', '@fstring_double'],
      [/[fF]'/, 'string.fstring', '@fstring_single'],

      // 普通字符串
      [/"/, 'string', '@string_double'],
      [/'/, 'string', '@string_single'],

      // 注释
      [/#.*$/, 'comment'],

      // 括号 - 显式指定 token 以确保颜色正确
      [/[{]/, 'delimiter.curly'],
      [/[}]/, 'delimiter.curly'],
      [/[\[]/, 'delimiter.bracket'],
      [/[\]]/, 'delimiter.bracket'],
      [/[(]/, 'delimiter.parenthesis'],
      [/[)]/, 'delimiter.parenthesis'],

      // 运算符
      [/[+\-*/%&|^~<>=!]=?/, 'operator'],
      [/<<|>>|\/\/|\*\*/, 'operator'],

      // 分隔符
      [/[;,.:@]/, 'delimiter']
    ],

    docstring_double: [
      [/[^"]+/, 'string.docstring'],
      [/"""/, 'string.docstring', '@pop'],
      [/"/, 'string.docstring']
    ],

    docstring_single: [
      [/[^']+/, 'string.docstring'],
      [/'''/, 'string.docstring', '@pop'],
      [/'/, 'string.docstring']
    ],

    string_double: [
      [/[^\\"]+/, 'string'],
      [/\\./, 'string.escape'],
      [/"/, 'string', '@pop']
    ],

    string_single: [
      [/[^\\']+/, 'string'],
      [/\\./, 'string.escape'],
      [/'/, 'string', '@pop']
    ],

    fstring_double: [
      [/\{/, 'string.fstring.bracket', '@fstring_expr'],
      [/[^\\"{]+/, 'string.fstring'],
      [/\\./, 'string.escape'],
      [/"/, 'string.fstring', '@pop']
    ],

    fstring_single: [
      [/\{/, 'string.fstring.bracket', '@fstring_expr'],
      [/[^\\'{]+/, 'string.fstring'],
      [/\\./, 'string.escape'],
      [/'/, 'string.fstring', '@pop']
    ],

    fstring_expr: [
      [/\}/, 'string.fstring.bracket', '@pop'],
      [/[^}]+/, 'identifier']
    ]
  }
})

// PyCharm Darcula 主题配色（用户提供的精确 RGB 值）
monaco.editor.defineTheme('pycharm-darcula', {
  base: 'vs-dark',
  inherit: false,
  rules: [
    // 变量名 - RGB(187, 189, 192) = #bbbdc0
    { token: '', foreground: 'bbbdc0' },
    { token: 'identifier', foreground: 'bbbdc0' },
    { token: 'white', foreground: 'bbbdc0' },

    // self, cls - RGB(148, 85, 141) = #94558d
    { token: 'variable.self', foreground: '94558d', fontStyle: 'italic' },

    // 实例属性（self.xxx, _xxx）- 紫色
    { token: 'variable.instance', foreground: '94558d' },

    // 注释 - 灰色
    { token: 'comment', foreground: '808080', fontStyle: 'italic' },

    // 关键字 (async, def, import 等) - RGB(206, 141, 97) = #ce8d61
    { token: 'keyword', foreground: 'ce8d61' },
    { token: 'keyword.constant', foreground: 'ce8d61' },

    // 字符串 - RGB(92, 170, 114) = #5caa72
    { token: 'string', foreground: '5caa72' },
    { token: 'string.docstring', foreground: '5caa72', fontStyle: 'italic' },
    { token: 'string.fstring', foreground: '5caa72' },
    { token: 'string.fstring.bracket', foreground: 'ce8d61' },
    { token: 'string.escape', foreground: 'ce8d61' },

    // 数字 - 蓝色
    { token: 'number', foreground: '6897bb' },
    { token: 'number.float', foreground: '6897bb' },
    { token: 'number.hex', foreground: '6897bb' },
    { token: 'number.octal', foreground: '6897bb' },
    { token: 'number.binary', foreground: '6897bb' },

    // 函数名 - RGB(85, 167, 242) = #55a7f2 (蓝色)
    { token: 'function.declaration', foreground: '55a7f2' },

    // 魔术变量 (__name__ 等) - 普通变量色 #bbbdc0
    { token: 'function.magic', foreground: 'bbbdc0' },

    // 类声明名 - 普通变量色 #bbbdc0
    { token: 'class.declaration', foreground: 'bbbdc0' },

    // 类型注解 - 关键字色 #ce8d61
    { token: 'type', foreground: 'ce8d61' },

    // 常量（全大写）- 紫色
    { token: 'constant', foreground: '94558d' },

    // 系统保留变量名 - RGB(135, 126, 134) = #877e86
    { token: 'predefined', foreground: '877e86' },

    // 装饰器 - 黄色
    { token: 'decorator', foreground: 'bbb529' },

    // 运算符 - 普通变量色 #bbbdc0
    { token: 'operator', foreground: 'bbbdc0' },

    // 分隔符 - 普通变量色 #bbbdc0
    { token: 'delimiter', foreground: 'bbbdc0' },
    { token: 'delimiter.parenthesis', foreground: 'bbbdc0' },
    { token: 'delimiter.bracket', foreground: 'bbbdc0' },
    { token: 'delimiter.curly', foreground: 'bbbdc0' },

    // JSON
    { token: 'string.key.json', foreground: '94558d' },
    { token: 'string.value.json', foreground: '5caa72' },

    // YAML
    { token: 'type.yaml', foreground: 'ce8d61' },
    { token: 'string.yaml', foreground: '5caa72' }
  ],
  colors: {
    // 编辑器背景 - RGB(30, 31, 34) = #1e1f22
    'editor.background': '#1e1f22',
    'editor.foreground': '#bbbdc0',

    // 行号
    'editorLineNumber.foreground': '#606366',
    'editorLineNumber.activeForeground': '#a4a3a3',

    // 光标 - 白色
    'editorCursor.foreground': '#ffffff',

    // 选中
    'editor.selectionBackground': '#214283',
    'editor.inactiveSelectionBackground': '#323232',

    // 当前行
    'editor.lineHighlightBackground': '#26282e',
    'editor.lineHighlightBorder': '#00000000',

    // 匹配括号
    'editorBracketMatch.background': '#3b514d',
    'editorBracketMatch.border': '#ffef28',

    // 缩进线
    'editorIndentGuide.background': '#393939',
    'editorIndentGuide.activeBackground': '#505050',

    // 小地图
    'minimap.background': '#1e1f22',

    // 滚动条
    'scrollbarSlider.background': '#4e4e4e80',
    'scrollbarSlider.hoverBackground': '#5a5a5a',
    'scrollbarSlider.activeBackground': '#6e6e6e',

    // 查找匹配
    'editor.findMatchBackground': '#32593d',
    'editor.findMatchHighlightBackground': '#274a2d80',

    // 侧边栏/边距
    'editorGutter.background': '#1e1f22',

    // 代码折叠
    'editorGutter.foldingControlForeground': '#bbbdc0',

    // 编辑器边框
    'editorWidget.background': '#2b2d30',
    'editorWidget.border': '#454647',

    // 悬停提示
    'editorHoverWidget.background': '#2b2d30',
    'editorHoverWidget.border': '#454647'
  }
})

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
