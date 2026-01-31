/**
 * PyCharm Darcula 主题配置
 * 用于 Monaco Editor
 */
import * as monaco from 'monaco-editor'

// ============================================
// 注册自定义语言 'python-pycharm'
// ============================================

export function registerPythonPyCharm() {
  // 注册语言
  monaco.languages.register({
    id: 'python-pycharm',
    extensions: ['.py'],
    aliases: ['Python (PyCharm)']
  })

  // 注册 tokenizer
  monaco.languages.setMonarchTokensProvider('python-pycharm', {
    defaultToken: 'identifier',
    tokenPostfix: '',

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

        // 装饰器
        [/@[a-zA-Z_]\w*/, 'decorator'],

        // self.属性 或 cls.属性
        [/\b(self|cls)(\.)([a-zA-Z_]\w*)/, ['variable.self', 'delimiter', 'identifier']],

        // 单独的 self, cls
        [/\b(self|cls)\b/, 'variable.self'],

        // 函数定义：def func_name
        [/\b(def)(\s+)([a-zA-Z_]\w*)/, ['keyword', 'white', 'function.declaration']],

        // 类定义：class ClassName
        [/\b(class)(\s+)([a-zA-Z_]\w*)/, ['keyword', 'white', 'class.declaration']],

        // 关键字
        [/\b(and|as|assert|async|await|break|class|continue|def|del|elif|else|except|finally|for|from|global|if|import|in|is|lambda|nonlocal|not|or|pass|raise|return|try|while|with|yield)\b/, 'keyword'],

        // 布尔和 None
        [/\b(True|False|None)\b/, 'keyword.constant'],

        // Python 内置类型和异常
        [/\b(int|float|str|bool|list|dict|set|tuple|bytes|bytearray|memoryview|object|type|range|slice|frozenset|complex|super|property|classmethod|staticmethod|enumerate|zip|map|filter|reversed|sorted|min|max|sum|abs|round|len|repr|hash|id|input|print|open|format|iter|next|callable|isinstance|issubclass|hasattr|getattr|setattr|delattr|vars|dir|globals|locals|eval|exec|compile|Exception|BaseException|TypeError|ValueError|KeyError|IndexError|AttributeError|RuntimeError|StopIteration|GeneratorExit|AssertionError|ImportError|ModuleNotFoundError|FileNotFoundError|OSError|IOError|PermissionError|TimeoutError|ConnectionError|SyntaxError|IndentationError|SystemError|SystemExit|KeyboardInterrupt|MemoryError|RecursionError|ArithmeticError|FloatingPointError|OverflowError|ZeroDivisionError|LookupError|EOFError|NotImplementedError|Warning|UserWarning|DeprecationWarning|RuntimeWarning|FutureWarning|ResourceWarning)\b/, 'builtin'],

        // 返回类型注解 -> type
        [/(->)(\s*)([a-zA-Z_]\w*)/, ['operator', 'white', 'type']],

        // 类型注解 : type
        [/(:)(\s*)(int|float|str|bool|list|dict|set|tuple|bytes|object|Optional|List|Dict|Set|Tuple|Any|Union|Callable|Iterable|Iterator|Generator)\b/, ['delimiter', 'white', 'builtin']],

        // 魔术变量名 __xxx__
        [/__[a-zA-Z_]+__/, 'identifier'],

        // 全大写常量
        [/\b[A-Z][A-Z_0-9]+\b/, 'constant'],

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

        // 括号
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
}

// ============================================
// 定义 PyCharm Darcula 主题
// ============================================

export function definePyCharmDarculaTheme() {
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

      // 注释 - 灰色
      { token: 'comment', foreground: '808080', fontStyle: 'italic' },

      // 关键字 - RGB(206, 141, 97) = #ce8d61
      { token: 'keyword', foreground: 'ce8d61' },
      { token: 'keyword.constant', foreground: 'ce8d61' },

      // Python 内置类型和异常 - RGB(136, 136, 198) = #8888c6
      { token: 'builtin', foreground: '8888c6' },

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

      // 函数名 - RGB(85, 167, 242) = #55a7f2
      { token: 'function.declaration', foreground: '55a7f2' },

      // 类声明名 - 普通变量色
      { token: 'class.declaration', foreground: 'bbbdc0' },

      // 类型注解 - 内置类型色
      { token: 'type', foreground: '8888c6' },

      // 常量（全大写）- 紫色
      { token: 'constant', foreground: '94558d' },

      // 装饰器 - 黄色
      { token: 'decorator', foreground: 'bbb529' },

      // 运算符 - 普通变量色
      { token: 'operator', foreground: 'bbbdc0' },

      // 分隔符 - 普通变量色
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

      // 光标
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

      // 侧边栏
      'editorGutter.background': '#1e1f22',
      'editorGutter.foldingControlForeground': '#bbbdc0',

      // 编辑器边框
      'editorWidget.background': '#2b2d30',
      'editorWidget.border': '#454647',

      // 悬停提示
      'editorHoverWidget.background': '#2b2d30',
      'editorHoverWidget.border': '#454647'
    }
  })
}

// ============================================
// 初始化主题
// ============================================

let initialized = false

export function initPyCharmDarcula() {
  if (initialized) return
  initialized = true

  registerPythonPyCharm()
  definePyCharmDarculaTheme()
}
