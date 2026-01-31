# Worker 代码编辑器 UI 重构实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 Worker 代码管理页面重构为 VS Code 风格的编辑器界面，包含左侧目录树、多标签编辑器和可折叠日志面板。

**Architecture:** 使用 Pinia Store 管理编辑器状态（标签页、展开目录、修改状态等）。主容器组件协调各子组件，目录树使用递归组件渲染，编辑器复用 Monaco Editor。

**Tech Stack:** Vue 3, TypeScript, Pinia, Element Plus, Monaco Editor

**设计文档:** `docs/plans/2026-01-31-worker-editor-ui-refactor-design.md`

---

## Task 1: 创建 Pinia Store

**Files:**
- Create: `web/src/stores/workerEditor.ts`
- Modify: `web/src/stores/index.ts`

**Step 1: 创建 workerEditor store**

```typescript
// web/src/stores/workerEditor.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getFileTree, getFile, saveFile, type TreeNode } from '@/api/worker'

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
      id: `tab-${Date.now()}`,
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
      py: 'python',
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
```

**Step 2: 导出 store**

在 `web/src/stores/index.ts` 添加导出：

```typescript
export { useUserStore } from './user'
export { useWorkerEditorStore } from './workerEditor'
```

**Step 3: 验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

Expected: 无错误

**Step 4: Commit**

```bash
git add web/src/stores/workerEditor.ts web/src/stores/index.ts
git commit -m "feat(web): add workerEditor Pinia store

Add state management for file tree, tabs, and log panel.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: 创建 ContextMenu 组件

**Files:**
- Create: `web/src/views/worker/components/ContextMenu.vue`

**Step 1: 创建右键菜单组件**

```vue
<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="context-menu"
      :style="{ left: x + 'px', top: y + 'px' }"
      @contextmenu.prevent
    >
      <div
        v-for="item in items"
        :key="item.key"
        :class="['menu-item', { divider: item.divider, danger: item.danger, disabled: item.disabled }]"
        @click="handleClick(item)"
      >
        <span v-if="!item.divider" class="label">{{ item.label }}</span>
        <span v-if="item.shortcut" class="shortcut">{{ item.shortcut }}</span>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

export interface MenuItem {
  key: string
  label?: string
  shortcut?: string
  divider?: boolean
  danger?: boolean
  disabled?: boolean
}

const props = defineProps<{
  items: MenuItem[]
}>()

const emit = defineEmits<{
  (e: 'select', key: string): void
  (e: 'close'): void
}>()

const visible = ref(false)
const x = ref(0)
const y = ref(0)

function show(event: MouseEvent) {
  x.value = event.clientX
  y.value = event.clientY
  visible.value = true

  // 确保菜单不超出视口
  setTimeout(() => {
    const menu = document.querySelector('.context-menu') as HTMLElement
    if (menu) {
      const rect = menu.getBoundingClientRect()
      if (rect.right > window.innerWidth) {
        x.value = window.innerWidth - rect.width - 5
      }
      if (rect.bottom > window.innerHeight) {
        y.value = window.innerHeight - rect.height - 5
      }
    }
  }, 0)
}

function hide() {
  visible.value = false
  emit('close')
}

function handleClick(item: MenuItem) {
  if (item.divider || item.disabled) return
  emit('select', item.key)
  hide()
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.context-menu')) {
    hide()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('contextmenu', hide)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('contextmenu', hide)
})

defineExpose({ show, hide })
</script>

<style scoped>
.context-menu {
  position: fixed;
  z-index: 9999;
  min-width: 160px;
  background: #1e1e1e;
  border: 1px solid #454545;
  border-radius: 4px;
  padding: 4px 0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
}

.menu-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 12px;
  cursor: pointer;
  color: #cccccc;
  font-size: 13px;
}

.menu-item:hover:not(.divider):not(.disabled) {
  background: #094771;
}

.menu-item.divider {
  height: 1px;
  background: #454545;
  margin: 4px 0;
  padding: 0;
  cursor: default;
}

.menu-item.danger .label {
  color: #f48771;
}

.menu-item.disabled {
  color: #6e6e6e;
  cursor: not-allowed;
}

.shortcut {
  color: #6e6e6e;
  font-size: 12px;
  margin-left: 20px;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/components/ContextMenu.vue
git commit -m "feat(web): add ContextMenu component

Dark theme context menu with keyboard shortcuts display.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: 创建 FileTreeNode 递归组件

**Files:**
- Create: `web/src/views/worker/components/FileTreeNode.vue`

**Step 1: 创建递归树节点组件**

```vue
<template>
  <div class="tree-node">
    <!-- 节点行 -->
    <div
      :class="['node-row', { active: isActive, selected: isSelected }]"
      :style="{ paddingLeft: depth * 16 + 8 + 'px' }"
      @click="handleClick"
      @dblclick="handleDblClick"
      @contextmenu.prevent="handleContextMenu"
    >
      <!-- 展开图标 -->
      <span v-if="node.type === 'dir'" class="expand-icon" @click.stop="toggleExpand">
        <el-icon :class="{ expanded: isExpanded }">
          <CaretRight />
        </el-icon>
      </span>
      <span v-else class="expand-icon placeholder"></span>

      <!-- 文件/目录图标 -->
      <el-icon class="node-icon" :class="node.type">
        <Folder v-if="node.type === 'dir'" />
        <component v-else :is="fileIcon" />
      </el-icon>

      <!-- 名称 -->
      <span class="node-name" :title="node.path">
        {{ node.name }}
        <span v-if="isModified" class="modified-dot">●</span>
      </span>
    </div>

    <!-- 子节点 -->
    <template v-if="node.type === 'dir' && isExpanded && node.children">
      <FileTreeNode
        v-for="child in sortedChildren"
        :key="child.path"
        :node="child"
        :depth="depth + 1"
        :active-path="activePath"
        @select="$emit('select', $event)"
        @open="$emit('open', $event)"
        @context-menu="$emit('context-menu', $event)"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { CaretRight, Folder, Document, Setting } from '@element-plus/icons-vue'
import { useWorkerEditorStore, type TreeNode } from '@/stores/workerEditor'

const props = defineProps<{
  node: TreeNode
  depth?: number
  activePath?: string | null
}>()

const emit = defineEmits<{
  (e: 'select', node: TreeNode): void
  (e: 'open', node: TreeNode): void
  (e: 'context-menu', payload: { event: MouseEvent; node: TreeNode }): void
}>()

const store = useWorkerEditorStore()
const depth = computed(() => props.depth ?? 0)

const isExpanded = computed(() => store.isDirExpanded(props.node.path))
const isActive = computed(() => props.activePath === props.node.path)
const isSelected = computed(() =>
  store.tabs.some(t => t.path === props.node.path)
)
const isModified = computed(() =>
  store.tabs.find(t => t.path === props.node.path)?.content !==
  store.tabs.find(t => t.path === props.node.path)?.originalContent
)

const fileIcon = computed(() => {
  if (props.node.type === 'dir') return Folder
  const ext = props.node.name.split('.').pop()?.toLowerCase()
  if (ext === 'py') return Document  // 可以替换为 Python 图标
  if (ext === 'json' || ext === 'yaml' || ext === 'yml') return Setting
  return Document
})

const sortedChildren = computed(() => {
  if (!props.node.children) return []
  return [...props.node.children].sort((a, b) => {
    // 目录在前
    if (a.type !== b.type) {
      return a.type === 'dir' ? -1 : 1
    }
    // 同类型按名称排序
    return a.name.localeCompare(b.name)
  })
})

function toggleExpand() {
  if (props.node.type === 'dir') {
    store.toggleDir(props.node.path)
  }
}

function handleClick() {
  emit('select', props.node)
}

function handleDblClick() {
  if (props.node.type === 'file') {
    emit('open', props.node)
  }
}

function handleContextMenu(event: MouseEvent) {
  emit('context-menu', { event, node: props.node })
}
</script>

<style scoped>
.node-row {
  display: flex;
  align-items: center;
  height: 24px;
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
}

.node-row:hover {
  background: rgba(255, 255, 255, 0.05);
}

.node-row.selected {
  background: rgba(255, 255, 255, 0.08);
}

.node-row.active {
  background: #094771;
}

.expand-icon {
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.expand-icon .el-icon {
  font-size: 12px;
  color: #c5c5c5;
  transition: transform 0.15s;
}

.expand-icon .el-icon.expanded {
  transform: rotate(90deg);
}

.expand-icon.placeholder {
  visibility: hidden;
}

.node-icon {
  font-size: 16px;
  margin-right: 6px;
  flex-shrink: 0;
}

.node-icon.dir {
  color: #dcb67a;
}

.node-icon.file {
  color: #c5c5c5;
}

.node-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  color: #cccccc;
  font-size: 13px;
}

.modified-dot {
  color: #e2c08d;
  margin-left: 4px;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/components/FileTreeNode.vue
git commit -m "feat(web): add FileTreeNode recursive component

Tree node with expand/collapse, icons, and context menu support.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: 创建 FileTree 侧边栏组件

**Files:**
- Create: `web/src/views/worker/components/FileTree.vue`

**Step 1: 创建目录树组件**

```vue
<template>
  <div class="file-tree" :style="{ width: width + 'px' }">
    <!-- 标题栏 -->
    <div class="tree-header">
      <span class="title">WORKER</span>
      <div class="actions">
        <el-tooltip content="刷新" placement="bottom">
          <el-icon class="action-btn" @click="handleRefresh"><Refresh /></el-icon>
        </el-tooltip>
        <el-tooltip content="新建文件" placement="bottom">
          <el-icon class="action-btn" @click="$emit('create-file')"><DocumentAdd /></el-icon>
        </el-tooltip>
        <el-tooltip content="新建目录" placement="bottom">
          <el-icon class="action-btn" @click="$emit('create-dir')"><FolderAdd /></el-icon>
        </el-tooltip>
      </div>
    </div>

    <!-- 树内容 -->
    <div class="tree-content" v-loading="store.treeLoading">
      <template v-if="store.fileTree">
        <FileTreeNode
          v-for="child in sortedRootChildren"
          :key="child.path"
          :node="child"
          :depth="0"
          :active-path="activePath"
          @select="handleSelect"
          @open="handleOpen"
          @context-menu="handleContextMenu"
        />
      </template>
      <div v-else-if="!store.treeLoading" class="empty-tip">
        暂无文件
      </div>
    </div>

    <!-- 拖拽调整宽度 -->
    <div
      class="resize-handle"
      @mousedown="startResize"
    ></div>

    <!-- 右键菜单 -->
    <ContextMenu
      ref="contextMenuRef"
      :items="contextMenuItems"
      @select="handleMenuSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Refresh, DocumentAdd, FolderAdd } from '@element-plus/icons-vue'
import { useWorkerEditorStore, type TreeNode } from '@/stores/workerEditor'
import FileTreeNode from './FileTreeNode.vue'
import ContextMenu, { type MenuItem } from './ContextMenu.vue'

const props = defineProps<{
  width: number
}>()

const emit = defineEmits<{
  (e: 'update:width', value: number): void
  (e: 'create-file', parentPath?: string): void
  (e: 'create-dir', parentPath?: string): void
  (e: 'rename', node: TreeNode): void
  (e: 'delete', node: TreeNode): void
  (e: 'run', node: TreeNode): void
}>()

const store = useWorkerEditorStore()
const contextMenuRef = ref<InstanceType<typeof ContextMenu>>()
const contextNode = ref<TreeNode | null>(null)

const activePath = computed(() => store.activeTab?.path || null)

const sortedRootChildren = computed(() => {
  if (!store.fileTree?.children) return []
  return [...store.fileTree.children].sort((a, b) => {
    if (a.type !== b.type) return a.type === 'dir' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
})

const contextMenuItems = computed<MenuItem[]>(() => {
  if (!contextNode.value) return []
  const node = contextNode.value
  const isFile = node.type === 'file'
  const isPython = node.name.endsWith('.py')

  const items: MenuItem[] = [
    { key: 'new-file', label: '新建文件' },
    { key: 'new-dir', label: '新建目录' },
    { key: 'divider-1', divider: true },
    { key: 'rename', label: '重命名', shortcut: 'F2' },
    { key: 'copy-path', label: '复制路径' },
  ]

  if (isFile) {
    items.push({ key: 'divider-2', divider: true })
    items.push({ key: 'open', label: '在编辑器打开' })
    if (isPython) {
      items.push({ key: 'run', label: '运行' })
    }
    items.push({ key: 'download', label: '下载' })
  }

  items.push({ key: 'divider-3', divider: true })
  items.push({ key: 'delete', label: '删除', shortcut: 'Del', danger: true })

  return items
})

function handleRefresh() {
  store.loadFileTree()
}

function handleSelect(node: TreeNode) {
  if (node.type === 'dir') {
    store.toggleDir(node.path)
  }
}

function handleOpen(node: TreeNode) {
  if (node.type === 'file') {
    store.openFile(node.path, node.name)
  }
}

function handleContextMenu(payload: { event: MouseEvent; node: TreeNode }) {
  contextNode.value = payload.node
  contextMenuRef.value?.show(payload.event)
}

function handleMenuSelect(key: string) {
  if (!contextNode.value) return
  const node = contextNode.value

  switch (key) {
    case 'new-file':
      emit('create-file', node.type === 'dir' ? node.path : getParentPath(node.path))
      break
    case 'new-dir':
      emit('create-dir', node.type === 'dir' ? node.path : getParentPath(node.path))
      break
    case 'rename':
      emit('rename', node)
      break
    case 'copy-path':
      navigator.clipboard.writeText(node.path)
      break
    case 'open':
      handleOpen(node)
      break
    case 'run':
      emit('run', node)
      break
    case 'download':
      window.open(`/api/worker/download/${node.path}?token=${localStorage.getItem('token')}`, '_blank')
      break
    case 'delete':
      emit('delete', node)
      break
  }
}

function getParentPath(path: string): string {
  const parts = path.split('/')
  parts.pop()
  return parts.join('/')
}

// 拖拽调整宽度
function startResize(event: MouseEvent) {
  const startX = event.clientX
  const startWidth = props.width

  function onMouseMove(e: MouseEvent) {
    const newWidth = Math.max(150, Math.min(400, startWidth + e.clientX - startX))
    emit('update:width', newWidth)
  }

  function onMouseUp() {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

onMounted(() => {
  store.loadFileTree()
})
</script>

<style scoped>
.file-tree {
  position: relative;
  height: 100%;
  background: #252526;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #3c3c3c;
}

.tree-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid #3c3c3c;
}

.title {
  font-size: 11px;
  font-weight: 600;
  color: #bbbbbb;
  letter-spacing: 0.5px;
}

.actions {
  display: flex;
  gap: 4px;
}

.action-btn {
  font-size: 16px;
  color: #858585;
  cursor: pointer;
  padding: 2px;
  border-radius: 3px;
}

.action-btn:hover {
  color: #cccccc;
  background: rgba(255, 255, 255, 0.1);
}

.tree-content {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}

.empty-tip {
  padding: 20px;
  text-align: center;
  color: #6e6e6e;
  font-size: 13px;
}

.resize-handle {
  position: absolute;
  right: 0;
  top: 0;
  width: 4px;
  height: 100%;
  cursor: ew-resize;
}

.resize-handle:hover {
  background: #007acc;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/components/FileTree.vue
git commit -m "feat(web): add FileTree sidebar component

File tree with header actions, resize handle, and context menu.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: 创建 EditorTabs 标签栏组件

**Files:**
- Create: `web/src/views/worker/components/EditorTabs.vue`

**Step 1: 创建标签栏组件**

```vue
<template>
  <div class="editor-tabs">
    <!-- 标签列表 -->
    <div class="tabs-container" ref="tabsContainer">
      <div
        v-for="tab in store.tabs"
        :key="tab.id"
        :class="['tab', { active: tab.id === store.activeTabId }]"
        :title="tab.path"
        @click="store.setActiveTab(tab.id)"
        @mousedown.middle="handleClose(tab.id)"
        @contextmenu.prevent="showTabMenu($event, tab.id)"
      >
        <el-icon class="tab-icon"><Document /></el-icon>
        <span class="tab-name">{{ tab.name }}</span>
        <span v-if="store.isTabModified(tab.id)" class="modified-dot">●</span>
        <el-icon
          class="close-btn"
          @click.stop="handleClose(tab.id)"
        >
          <Close />
        </el-icon>
      </div>
    </div>

    <!-- 空白状态 -->
    <div v-if="store.tabs.length === 0" class="empty-tabs">
      <span>选择文件开始编辑</span>
    </div>

    <!-- 标签右键菜单 -->
    <ContextMenu
      ref="tabMenuRef"
      :items="tabMenuItems"
      @select="handleTabMenuSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Document, Close } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus'
import { useWorkerEditorStore } from '@/stores/workerEditor'
import ContextMenu, { type MenuItem } from './ContextMenu.vue'

const store = useWorkerEditorStore()
const tabMenuRef = ref<InstanceType<typeof ContextMenu>>()
const contextTabId = ref<string | null>(null)

const tabMenuItems: MenuItem[] = [
  { key: 'close', label: '关闭' },
  { key: 'close-others', label: '关闭其他' },
  { key: 'close-right', label: '关闭右侧' },
  { key: 'divider', divider: true },
  { key: 'copy-path', label: '复制路径' }
]

async function handleClose(tabId: string) {
  if (store.isTabModified(tabId)) {
    const tab = store.tabs.find(t => t.id === tabId)
    try {
      await ElMessageBox.confirm(
        `${tab?.name} 有未保存的更改，是否保存？`,
        '保存文件',
        {
          confirmButtonText: '保存',
          cancelButtonText: '不保存',
          distinguishCancelAndClose: true,
          type: 'warning'
        }
      )
      await store.saveTab(tabId)
    } catch (action) {
      if (action === 'close') return // 点击 X 关闭弹窗，不做任何操作
      // 点击"不保存"继续关闭
    }
  }
  store.closeTab(tabId)
}

function showTabMenu(event: MouseEvent, tabId: string) {
  contextTabId.value = tabId
  tabMenuRef.value?.show(event)
}

function handleTabMenuSelect(key: string) {
  if (!contextTabId.value) return

  switch (key) {
    case 'close':
      handleClose(contextTabId.value)
      break
    case 'close-others':
      store.closeOtherTabs(contextTabId.value)
      break
    case 'close-right':
      store.closeTabsToRight(contextTabId.value)
      break
    case 'copy-path':
      const tab = store.tabs.find(t => t.id === contextTabId.value)
      if (tab) navigator.clipboard.writeText(tab.path)
      break
  }
}
</script>

<style scoped>
.editor-tabs {
  height: 35px;
  background: #252526;
  display: flex;
  align-items: center;
  border-bottom: 1px solid #3c3c3c;
}

.tabs-container {
  display: flex;
  flex: 1;
  overflow-x: auto;
  overflow-y: hidden;
}

.tabs-container::-webkit-scrollbar {
  height: 3px;
}

.tabs-container::-webkit-scrollbar-thumb {
  background: #5a5a5a;
}

.tab {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 10px;
  height: 35px;
  background: #2d2d2d;
  border-right: 1px solid #252526;
  cursor: pointer;
  flex-shrink: 0;
  max-width: 150px;
}

.tab:hover {
  background: #323232;
}

.tab.active {
  background: #1e1e1e;
  border-bottom: 1px solid #1e1e1e;
  margin-bottom: -1px;
}

.tab-icon {
  font-size: 14px;
  color: #75beff;
  flex-shrink: 0;
}

.tab-name {
  font-size: 13px;
  color: #969696;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tab.active .tab-name {
  color: #ffffff;
}

.modified-dot {
  color: #e2c08d;
  font-size: 16px;
  line-height: 1;
}

.close-btn {
  font-size: 14px;
  color: transparent;
  flex-shrink: 0;
  padding: 2px;
  border-radius: 3px;
}

.tab:hover .close-btn {
  color: #969696;
}

.close-btn:hover {
  color: #ffffff !important;
  background: rgba(255, 255, 255, 0.1);
}

.empty-tabs {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #6e6e6e;
  font-size: 13px;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/components/EditorTabs.vue
git commit -m "feat(web): add EditorTabs component

Multi-tab support with close, context menu, and modified indicator.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: 创建 CodeEditor 编辑器组件

**Files:**
- Create: `web/src/views/worker/components/CodeEditor.vue`

**Step 1: 创建编辑器组件**

```vue
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

const emit = defineEmits<{
  (e: 'run', path: string): void
}>()

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
```

**Step 2: Commit**

```bash
git add web/src/views/worker/components/CodeEditor.vue
git commit -m "feat(web): add CodeEditor component with Monaco

Editor with toolbar, save (Ctrl+S), and run functionality.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: 创建 LogPanel 日志面板组件

**Files:**
- Create: `web/src/views/worker/components/LogPanel.vue`

**Step 1: 创建日志面板组件**

```vue
<template>
  <div
    class="log-panel"
    :class="{ expanded: store.logExpanded }"
    :style="{ height: store.logExpanded ? height + 'px' : '28px' }"
  >
    <!-- 拖拽调整高度 -->
    <div
      v-if="store.logExpanded"
      class="resize-handle"
      @mousedown="startResize"
    ></div>

    <!-- 标题栏 -->
    <div class="panel-header" @click="toggleExpand">
      <div class="header-left">
        <el-icon class="expand-icon">
          <CaretRight v-if="!store.logExpanded" />
          <CaretBottom v-else />
        </el-icon>
        <span class="title">运行日志</span>
        <span v-if="store.logRunning" class="running-badge">运行中...</span>
        <span v-else-if="store.logs.length === 0" class="empty-badge">无输出</span>
      </div>
      <div class="header-right" @click.stop>
        <el-button
          v-if="store.logRunning"
          text
          size="small"
          type="danger"
          @click="handleStop"
        >
          停止
        </el-button>
        <el-button text size="small" @click="handleCopy">复制</el-button>
        <el-button text size="small" @click="store.clearLogs">清空</el-button>
      </div>
    </div>

    <!-- 日志内容 -->
    <div v-if="store.logExpanded" class="log-content" ref="logContent">
      <div
        v-for="(log, index) in store.logs"
        :key="index"
        :class="['log-line', log.type]"
      >
        <span class="log-text">{{ log.data }}</span>
        <span class="log-time">{{ formatTime(log.timestamp) }}</span>
      </div>
      <div v-if="store.logs.length === 0" class="empty-log">
        运行 Python 文件查看输出
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { CaretRight, CaretBottom } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useWorkerEditorStore } from '@/stores/workerEditor'

const store = useWorkerEditorStore()
const logContent = ref<HTMLElement>()
const height = ref(200)

function toggleExpand() {
  store.logExpanded = !store.logExpanded
}

function startResize(event: MouseEvent) {
  const startY = event.clientY
  const startHeight = height.value

  function onMouseMove(e: MouseEvent) {
    const newHeight = Math.max(100, Math.min(400, startHeight - (e.clientY - startY)))
    height.value = newHeight
  }

  function onMouseUp() {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function handleCopy() {
  const text = store.logs.map(l => l.data).join('\n')
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制到剪贴板')
}

function handleStop() {
  // 停止逻辑由 CodeEditor 组件处理
  // 这里只是 UI 占位
}

// 自动滚动到底部
watch(() => store.logs.length, () => {
  nextTick(() => {
    if (logContent.value) {
      logContent.value.scrollTop = logContent.value.scrollHeight
    }
  })
})
</script>

<style scoped>
.log-panel {
  background: #1e1e1e;
  border-top: 1px solid #3c3c3c;
  display: flex;
  flex-direction: column;
  transition: height 0.15s;
  flex-shrink: 0;
}

.resize-handle {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  cursor: ns-resize;
  z-index: 10;
}

.resize-handle:hover {
  background: #007acc;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 12px;
  background: #252526;
  cursor: pointer;
  user-select: none;
  height: 28px;
  box-sizing: border-box;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.expand-icon {
  font-size: 12px;
  color: #cccccc;
}

.title {
  font-size: 12px;
  font-weight: 500;
  color: #cccccc;
}

.running-badge {
  font-size: 11px;
  color: #3794ff;
}

.empty-badge {
  font-size: 11px;
  color: #6e6e6e;
}

.header-right {
  display: flex;
  gap: 4px;
}

.log-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 12px;
  line-height: 1.6;
}

.log-line {
  display: flex;
  justify-content: space-between;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-text {
  flex: 1;
}

.log-time {
  flex-shrink: 0;
  margin-left: 16px;
  color: #4e4e4e;
}

.log-line.command {
  color: #808080;
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

.empty-log {
  color: #6e6e6e;
  text-align: center;
  padding: 20px;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/components/LogPanel.vue
git commit -m "feat(web): add LogPanel component

Collapsible log panel with auto-scroll and copy support.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: 创建 WorkerCodeEditor 主容器组件

**Files:**
- Create: `web/src/views/worker/WorkerCodeEditor.vue`

**Step 1: 创建主容器组件**

```vue
<template>
  <div class="worker-code-editor">
    <!-- 页面头部 -->
    <div class="page-header">
      <h2>Worker 代码管理</h2>
      <div class="header-actions">
        <el-button
          type="warning"
          :icon="Refresh"
          :loading="restarting"
          @click="handleRestart"
        >
          重启 Worker
        </el-button>
        <el-button
          type="danger"
          :icon="Setting"
          :loading="rebuilding"
          @click="handleRebuild"
        >
          重新构建
        </el-button>
      </div>
    </div>

    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 侧边栏 -->
      <FileTree
        :width="store.sidebarWidth"
        @update:width="store.sidebarWidth = $event"
        @create-file="showCreateDialog('file', $event)"
        @create-dir="showCreateDialog('dir', $event)"
        @rename="showRenameDialog"
        @delete="handleDelete"
        @run="handleRunFromTree"
      />

      <!-- 编辑区 -->
      <div class="editor-area">
        <EditorTabs />
        <CodeEditor />
        <LogPanel />
      </div>
    </div>

    <!-- 新建弹窗 -->
    <CreateDialog
      v-model="createDialogVisible"
      :type="createType"
      @confirm="handleCreate"
    />

    <!-- 重命名弹窗 -->
    <RenameDialog
      v-model="renameDialogVisible"
      :current-name="renamingNode?.name || ''"
      @confirm="handleRename"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Setting } from '@element-plus/icons-vue'
import { useWorkerEditorStore, type TreeNode } from '@/stores/workerEditor'
import {
  createItem,
  deleteItem,
  moveItem,
  restartWorker,
  rebuildWorker,
  runFile
} from '@/api/worker'

import FileTree from './components/FileTree.vue'
import EditorTabs from './components/EditorTabs.vue'
import CodeEditor from './components/CodeEditor.vue'
import LogPanel from './components/LogPanel.vue'
import CreateDialog from './components/CreateDialog.vue'
import RenameDialog from './components/RenameDialog.vue'

const store = useWorkerEditorStore()

// 控制状态
const restarting = ref(false)
const rebuilding = ref(false)

// 弹窗状态
const createDialogVisible = ref(false)
const createType = ref<'file' | 'dir'>('file')
const createParentPath = ref('')

const renameDialogVisible = ref(false)
const renamingNode = ref<TreeNode | null>(null)

// 新建
function showCreateDialog(type: 'file' | 'dir', parentPath?: string) {
  createType.value = type
  createParentPath.value = parentPath || ''
  createDialogVisible.value = true
}

async function handleCreate(name: string) {
  try {
    await createItem(createParentPath.value, name, createType.value)
    ElMessage.success('创建成功')
    store.loadFileTree()
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  }
}

// 重命名
function showRenameDialog(node: TreeNode) {
  renamingNode.value = node
  renameDialogVisible.value = true
}

async function handleRename(newName: string) {
  if (!renamingNode.value) return

  const oldPath = renamingNode.value.path
  const parentPath = oldPath.split('/').slice(0, -1).join('/')
  const newPath = parentPath ? `${parentPath}/${newName}` : newName

  try {
    await moveItem(oldPath, newPath)
    ElMessage.success('重命名成功')
    store.loadFileTree()
  } catch (e: any) {
    ElMessage.error(e.message || '重命名失败')
  }
}

// 删除
async function handleDelete(node: TreeNode) {
  try {
    await ElMessageBox.confirm(
      `确定删除 ${node.name} 吗？${node.type === 'dir' ? '目录下所有文件都将被删除。' : ''}`,
      '确认删除',
      { type: 'warning' }
    )
    await deleteItem(node.path)
    ElMessage.success('删除成功')
    store.loadFileTree()

    // 如果删除的文件已打开，关闭对应标签
    const tab = store.tabs.find(t => t.path === node.path)
    if (tab) {
      store.closeTab(tab.id)
    }
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '删除失败')
    }
  }
}

// 从目录树运行文件
function handleRunFromTree(node: TreeNode) {
  // 先打开文件
  store.openFile(node.path, node.name)
  // 然后触发运行（由 CodeEditor 处理）
}

// 重启 Worker
async function handleRestart() {
  try {
    await ElMessageBox.confirm('确定重启 Worker 吗？', '确认重启', { type: 'warning' })
    restarting.value = true
    await restartWorker()
    ElMessage.success('重启指令已发送')
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '重启失败')
    }
  } finally {
    restarting.value = false
  }
}

// 重建 Worker
async function handleRebuild() {
  try {
    await ElMessageBox.confirm(
      '重新构建将重新安装所有依赖，可能需要几分钟时间。确定继续吗？',
      '确认重建',
      { type: 'warning' }
    )
    rebuilding.value = true
    await rebuildWorker()
    ElMessage.success('Worker 重新构建完成')
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '重建失败')
    }
  } finally {
    rebuilding.value = false
  }
}
</script>

<style scoped>
.worker-code-editor {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 100px);
  background: #1e1e1e;
  border-radius: 4px;
  overflow: hidden;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #2d2d2d;
  border-bottom: 1px solid #3c3c3c;
}

.page-header h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
  color: #cccccc;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.main-content {
  display: flex;
  flex: 1;
  min-height: 0;
}

.editor-area {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/WorkerCodeEditor.vue
git commit -m "feat(web): add WorkerCodeEditor main container

VS Code style layout with sidebar, tabs, editor and log panel.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: 更新路由配置

**Files:**
- Modify: `web/src/router/index.ts`

**Step 1: 更新路由组件引用**

将第 76-80 行：
```typescript
      {
        path: 'worker',
        name: 'WorkerCode',
        component: () => import('@/views/worker/WorkerCodeManager.vue'),
        meta: { title: 'Worker代码', icon: 'EditPen' }
      },
```

修改为：
```typescript
      {
        path: 'worker',
        name: 'WorkerCode',
        component: () => import('@/views/worker/WorkerCodeEditor.vue'),
        meta: { title: 'Worker代码', icon: 'EditPen' }
      },
```

**Step 2: 验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

Expected: 无错误

**Step 3: Commit**

```bash
git add web/src/router/index.ts
git commit -m "feat(web): update router to use WorkerCodeEditor

Switch from WorkerCodeManager to new VS Code style editor.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 10: 删除旧组件文件

**Files:**
- Delete: `web/src/views/worker/WorkerCodeManager.vue`
- Delete: `web/src/views/worker/components/FileTable.vue`
- Delete: `web/src/views/worker/components/FileToolbar.vue`
- Delete: `web/src/views/worker/components/FileEditor.vue`
- Delete: `web/src/views/worker/components/MoveDialog.vue`

**Step 1: 删除文件**

```bash
cd web/src/views/worker
rm WorkerCodeManager.vue
rm components/FileTable.vue
rm components/FileToolbar.vue
rm components/FileEditor.vue
rm components/MoveDialog.vue
```

**Step 2: Commit**

```bash
git add -A
git commit -m "chore(web): remove old worker code manager components

Remove deprecated components replaced by VS Code style editor.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 11: 集成测试与修复

**Step 1: 构建前端**

```bash
cd web && npm run build
```

Expected: 构建成功

**Step 2: 类型检查**

```bash
cd web && npx vue-tsc --noEmit
```

Expected: 无错误

**Step 3: 手动测试**

启动开发服务器并测试以下功能：

1. 目录树
   - [ ] 加载文件树
   - [ ] 展开/折叠目录
   - [ ] 单击文件打开
   - [ ] 右键菜单显示
   - [ ] 拖拽调整宽度

2. 标签栏
   - [ ] 打开多个文件
   - [ ] 切换标签
   - [ ] 关闭标签
   - [ ] 修改标记显示

3. 编辑器
   - [ ] 编辑文件
   - [ ] Ctrl+S 保存
   - [ ] 运行 Python 文件

4. 日志面板
   - [ ] 展开/折叠
   - [ ] 显示运行输出
   - [ ] 清空/复制

5. 其他操作
   - [ ] 新建文件/目录
   - [ ] 重命名
   - [ ] 删除
   - [ ] 重启/重建 Worker

**Step 4: Commit 修复（如有）**

```bash
git add -A
git commit -m "fix(web): integration fixes for worker code editor

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## 完成清单

- [ ] Task 1: 创建 Pinia Store
- [ ] Task 2: 创建 ContextMenu 组件
- [ ] Task 3: 创建 FileTreeNode 递归组件
- [ ] Task 4: 创建 FileTree 侧边栏组件
- [ ] Task 5: 创建 EditorTabs 标签栏组件
- [ ] Task 6: 创建 CodeEditor 编辑器组件
- [ ] Task 7: 创建 LogPanel 日志面板组件
- [ ] Task 8: 创建 WorkerCodeEditor 主容器组件
- [ ] Task 9: 更新路由配置
- [ ] Task 10: 删除旧组件文件
- [ ] Task 11: 集成测试与修复
