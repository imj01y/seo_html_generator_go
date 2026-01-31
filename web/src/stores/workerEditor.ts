import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getFileTree, getFile, saveFile, type TreeNode } from '@/api/worker'

// 重新导出 TreeNode 类型供其他组件使用
export type { TreeNode }

export interface Tab {
  id: string
  path: string
  name: string
  content: string
  originalContent: string
  language: string
}

export interface LogEntry {
  type: 'command' | 'stdout' | 'stderr' | 'info'
  data: string
  timestamp: Date
}

export const useWorkerEditorStore = defineStore('workerEditor', () => {
  // 目录树状态
  const fileTree = ref<TreeNode | null>(null)
  const expandedDirs = ref<Set<string>>(new Set(['/']))
  const treeLoading = ref(false)

  // 标签页状态
  const tabs = ref<Tab[]>([])
  const activeTabId = ref<string | null>(null)

  // 日志状态
  const logs = ref<LogEntry[]>([])
  const logExpanded = ref(false)
  const logRunning = ref(false)

  // 布局状态
  const sidebarWidth = ref(220)
  const logPanelHeight = ref(200)

  // 计算属性
  const activeTab = computed(() =>
    tabs.value.find(t => t.id === activeTabId.value) || null
  )

  const modifiedTabs = computed(() =>
    tabs.value.filter(t => t.content !== t.originalContent)
  )

  const hasModifiedFiles = computed(() => modifiedTabs.value.length > 0)

  // 目录树操作
  async function loadFileTree() {
    treeLoading.value = true
    try {
      fileTree.value = await getFileTree()
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

  // ID 计数器，确保唯一性
  let tabIdCounter = 0

  // 标签页操作
  async function openFile(path: string, name: string) {
    // 检查是否已打开
    const existing = tabs.value.find(t => t.path === path)
    if (existing) {
      activeTabId.value = existing.id
      return
    }

    // 加载文件内容
    const res = await getFile(path)
    const language = getLanguageByExtension(name)

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

    // 如果关闭的是当前标签，切换到相邻标签
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

    await saveFile(tab.path, tab.content)
    tab.originalContent = tab.content
  }

  function isTabModified(tabId: string): boolean {
    const tab = tabs.value.find(t => t.id === tabId)
    return tab ? tab.content !== tab.originalContent : false
  }

  // 日志操作
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

  // 辅助函数
  function getLanguageByExtension(filename: string): string {
    const ext = filename.split('.').pop()?.toLowerCase()
    const langMap: Record<string, string> = {
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
    return langMap[ext || ''] || 'plaintext'
  }

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
})
