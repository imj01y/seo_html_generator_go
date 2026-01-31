/**
 * CodeEditorPanel 可复用状态管理
 * 支持多实例，每个实例独立管理状态
 */
import { ref, computed, type Ref, type ComputedRef } from 'vue'
import type { TreeNode, Tab, LogEntry, CodeEditorApi } from '../types'
import { getLanguageByFilename } from '../types'

export interface EditorStore {
  // 状态
  fileTree: Ref<TreeNode | null>
  expandedDirs: Ref<Set<string>>
  treeLoading: Ref<boolean>
  tabs: Ref<Tab[]>
  activeTabId: Ref<string | null>
  activeTab: ComputedRef<Tab | null>
  modifiedTabs: ComputedRef<Tab[]>
  hasModifiedFiles: ComputedRef<boolean>
  logs: Ref<LogEntry[]>
  logExpanded: Ref<boolean>
  logRunning: Ref<boolean>
  sidebarWidth: Ref<number>
  logPanelHeight: Ref<number>

  // 目录树操作
  loadFileTree: () => Promise<void>
  toggleDir: (path: string) => void
  isDirExpanded: (path: string) => boolean

  // 标签页操作
  openFile: (path: string, name: string) => Promise<void>
  closeTab: (tabId: string) => void
  closeOtherTabs: (tabId: string) => void
  closeTabsToRight: (tabId: string) => void
  setActiveTab: (tabId: string) => void
  updateTabContent: (tabId: string, content: string) => void
  saveTab: (tabId: string) => Promise<void>
  isTabModified: (tabId: string) => boolean

  // 日志操作
  addLog: (entry: Omit<LogEntry, 'timestamp'>) => void
  clearLogs: () => void
  setLogRunning: (running: boolean) => void
}

/**
 * 创建编辑器状态管理实例
 */
export function useEditorStore(
  api: CodeEditorApi,
  languageMap?: Record<string, string>
): EditorStore {
  // ============================================
  // 状态定义
  // ============================================

  const fileTree = ref<TreeNode | null>(null)
  const expandedDirs = ref<Set<string>>(new Set(['/']))
  const treeLoading = ref(false)

  const tabs = ref<Tab[]>([])
  const activeTabId = ref<string | null>(null)

  const logs = ref<LogEntry[]>([])
  const logExpanded = ref(false)
  const logRunning = ref(false)

  const sidebarWidth = ref(220)
  const logPanelHeight = ref(200)

  // ID 计数器
  let tabIdCounter = 0

  // ============================================
  // 计算属性
  // ============================================

  const activeTab = computed(() =>
    tabs.value.find(t => t.id === activeTabId.value) || null
  )

  const modifiedTabs = computed(() =>
    tabs.value.filter(t => t.content !== t.originalContent)
  )

  const hasModifiedFiles = computed(() => modifiedTabs.value.length > 0)

  // ============================================
  // 目录树操作
  // ============================================

  async function loadFileTree() {
    treeLoading.value = true
    try {
      fileTree.value = await api.getFileTree()
    } catch (e) {
      console.error('Failed to load file tree:', e)
      throw e
    } finally {
      treeLoading.value = false
    }
  }

  function toggleDir(path: string) {
    if (expandedDirs.value.has(path)) {
      expandedDirs.value.delete(path)
    } else {
      expandedDirs.value.add(path)
    }
  }

  function isDirExpanded(path: string): boolean {
    return expandedDirs.value.has(path)
  }

  // ============================================
  // 标签页操作
  // ============================================

  async function openFile(path: string, name: string) {
    // 检查是否已打开
    const existing = tabs.value.find(t => t.path === path)
    if (existing) {
      activeTabId.value = existing.id
      return
    }

    // 加载文件内容
    const res = await api.getFile(path)
    const language = getLanguageByFilename(name, languageMap)

    const newTab: Tab = {
      id: `tab-${Date.now()}-${++tabIdCounter}`,
      path,
      name,
      content: res.content,
      originalContent: res.content,
      language
    }

    tabs.value.push(newTab)
    activeTabId.value = newTab.id
  }

  function closeTab(tabId: string) {
    const index = tabs.value.findIndex(t => t.id === tabId)
    if (index === -1) return

    tabs.value.splice(index, 1)

    if (activeTabId.value === tabId) {
      if (tabs.value.length > 0) {
        const newIndex = Math.min(index, tabs.value.length - 1)
        activeTabId.value = tabs.value[newIndex].id
      } else {
        activeTabId.value = null
      }
    }
  }

  function closeOtherTabs(tabId: string) {
    tabs.value = tabs.value.filter(t => t.id === tabId)
    activeTabId.value = tabId
  }

  function closeTabsToRight(tabId: string) {
    const index = tabs.value.findIndex(t => t.id === tabId)
    if (index === -1) return
    tabs.value = tabs.value.slice(0, index + 1)
    if (!tabs.value.find(t => t.id === activeTabId.value)) {
      activeTabId.value = tabId
    }
  }

  function setActiveTab(tabId: string) {
    activeTabId.value = tabId
  }

  function updateTabContent(tabId: string, content: string) {
    const tab = tabs.value.find(t => t.id === tabId)
    if (tab) {
      tab.content = content
    }
  }

  async function saveTab(tabId: string) {
    const tab = tabs.value.find(t => t.id === tabId)
    if (!tab) return

    await api.saveFile(tab.path, tab.content)
    tab.originalContent = tab.content
  }

  function isTabModified(tabId: string): boolean {
    const tab = tabs.value.find(t => t.id === tabId)
    return tab ? tab.content !== tab.originalContent : false
  }

  // ============================================
  // 日志操作
  // ============================================

  function addLog(entry: Omit<LogEntry, 'timestamp'>) {
    logs.value.push({ ...entry, timestamp: new Date() })
  }

  function clearLogs() {
    logs.value = []
  }

  function setLogRunning(running: boolean) {
    logRunning.value = running
    if (running) {
      logExpanded.value = true
    }
  }

  // ============================================
  // 返回状态和方法
  // ============================================

  return {
    // 状态
    fileTree,
    expandedDirs,
    treeLoading,
    tabs,
    activeTabId,
    activeTab,
    modifiedTabs,
    hasModifiedFiles,
    logs,
    logExpanded,
    logRunning,
    sidebarWidth,
    logPanelHeight,

    // 目录树操作
    loadFileTree,
    toggleDir,
    isDirExpanded,

    // 标签页操作
    openFile,
    closeTab,
    closeOtherTabs,
    closeTabsToRight,
    setActiveTab,
    updateTabContent,
    saveTab,
    isTabModified,

    // 日志操作
    addLog,
    clearLogs,
    setLogRunning
  }
}
