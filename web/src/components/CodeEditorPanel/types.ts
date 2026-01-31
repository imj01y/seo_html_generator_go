/**
 * CodeEditorPanel 通用组件类型定义
 */

// ============================================
// 文件树类型
// ============================================

export interface TreeNode {
  name: string
  path: string
  type: 'file' | 'dir'
  children?: TreeNode[]
}

// ============================================
// 标签页类型
// ============================================

export interface Tab {
  id: string
  path: string
  name: string
  content: string
  originalContent: string
  language: string
  lastSavedAt?: Date
}

// ============================================
// 日志类型
// ============================================

export interface LogEntry {
  type: 'command' | 'stdout' | 'stderr' | 'info'
  data: string
  timestamp: Date
}

// ============================================
// 运行处理器类型
// ============================================

export interface RunHandlers {
  onStdout: (data: string) => void
  onStderr: (data: string) => void
  onDone: (exitCode: number, durationMs: number) => void
  onError?: (error: string) => void
}

// ============================================
// API 适配器接口
// ============================================

export interface CodeEditorApi {
  /** 获取文件树 */
  getFileTree: () => Promise<TreeNode>

  /** 获取文件内容 */
  getFile: (path: string) => Promise<{ content: string }>

  /** 保存文件 */
  saveFile: (path: string, content: string) => Promise<void>

  /** 创建文件或目录 */
  createItem: (parentPath: string, name: string, type: 'file' | 'dir') => Promise<void>

  /** 删除文件或目录 */
  deleteItem: (path: string) => Promise<void>

  /** 移动/重命名 */
  moveItem: (oldPath: string, newPath: string) => Promise<void>

  /** 运行文件（可选） */
  runFile?: (path: string, handlers: RunHandlers) => () => void

  /** 获取下载 URL（可选） */
  getDownloadUrl?: (path: string) => string
}

// ============================================
// 组件 Props 类型
// ============================================

export interface CodeEditorPanelProps {
  /** API 适配器 */
  api: CodeEditorApi

  /** 面板标题 */
  title?: string

  /** 是否显示运行按钮（需要 api.runFile） */
  runnable?: boolean

  /** 是否显示日志面板 */
  showLogPanel?: boolean

  /** 是否显示重启按钮 */
  showRestartButton?: boolean

  /** 是否显示重建按钮 */
  showRebuildButton?: boolean

  /** 文件扩展名到语言的映射 */
  languageMap?: Record<string, string>

  /** 可运行的文件扩展名 */
  runnableExtensions?: string[]
}

// ============================================
// 菜单项类型
// ============================================

export interface MenuItem {
  key: string
  label?: string
  shortcut?: string
  divider?: boolean
  danger?: boolean
}

// ============================================
// 辅助函数
// ============================================

/** 默认语言映射 */
export const defaultLanguageMap: Record<string, string> = {
  py: 'python-pycharm',
  js: 'javascript',
  ts: 'typescript',
  json: 'json',
  yaml: 'yaml',
  yml: 'yaml',
  md: 'markdown',
  html: 'html',
  css: 'css',
  sql: 'sql',
  sh: 'shell',
  txt: 'plaintext'
}

/** 根据文件名获取语言 */
export function getLanguageByFilename(
  filename: string,
  languageMap: Record<string, string> = defaultLanguageMap
): string {
  const ext = filename.split('.').pop()?.toLowerCase() || ''
  return languageMap[ext] || 'plaintext'
}
