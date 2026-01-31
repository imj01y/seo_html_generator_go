<template>
  <div class="file-tree" :style="{ width: width + 'px' }">
    <!-- 标题栏 -->
    <div class="tree-header">
      <span class="title">{{ title }}</span>
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
    <div
      :class="['tree-content', { 'drag-over-root': isDragOverRoot }]"
      v-loading="store.treeLoading.value"
      @contextmenu.prevent="handleTreeContextMenu"
      @dragover.prevent="handleRootDragOver"
      @dragleave="handleRootDragLeave"
      @drop.prevent="handleRootDrop"
    >
      <template v-if="store.fileTree.value">
        <FileTreeNode
          v-for="child in sortedRootChildren"
          :key="child.path"
          :node="child"
          :depth="0"
          :active-path="activePath"
          :store="store"
          @select="handleSelect"
          @open="handleOpen"
          @context-menu="handleContextMenu"
          @move="handleMove"
        />
      </template>
      <div v-else-if="!store.treeLoading.value" class="empty-tip">
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
import { ElMessage } from 'element-plus'
import { Refresh, DocumentAdd, FolderAdd } from '@element-plus/icons-vue'
import type { TreeNode, MenuItem, CodeEditorApi } from '../types'
import type { EditorStore } from '../composables/useEditorStore'
import FileTreeNode from './FileTreeNode.vue'
import ContextMenu from './ContextMenu.vue'

const props = defineProps<{
  width: number
  title?: string
  store: EditorStore
  api: CodeEditorApi
  runnable?: boolean
  runnableExtensions?: string[]
}>()

const emit = defineEmits<{
  (e: 'update:width', value: number): void
  (e: 'create-file', parentPath?: string): void
  (e: 'create-dir', parentPath?: string): void
  (e: 'rename', node: TreeNode): void
  (e: 'delete', node: TreeNode): void
  (e: 'run', node: TreeNode): void
}>()

const contextMenuRef = ref<InstanceType<typeof ContextMenu>>()
const contextNode = ref<TreeNode | null>(null)
const isDragOverRoot = ref(false)

const activePath = computed(() => props.store.activeTab.value?.path || null)

const sortedRootChildren = computed(() => {
  if (!props.store.fileTree.value?.children) return []
  return [...props.store.fileTree.value.children].sort((a, b) => {
    if (a.type !== b.type) return a.type === 'dir' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
})

const runnableExts = computed(() => props.runnableExtensions || ['.py'])

const contextMenuItems = computed<MenuItem[]>(() => {
  const node = contextNode.value

  // 空白区域右键：只显示新建选项
  if (!node) {
    return [
      { key: 'new-file', label: '新建文件' },
      { key: 'new-dir', label: '新建目录' },
    ]
  }

  // 节点上右键：显示完整菜单
  const isFile = node.type === 'file'
  const isRunnable = isFile && runnableExts.value.some(ext => node.name.endsWith(ext))

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
    if (props.runnable && isRunnable) {
      items.push({ key: 'run', label: '运行' })
    }
    if (props.api.getDownloadUrl) {
      items.push({ key: 'download', label: '下载' })
    }
  }

  items.push({ key: 'divider-3', divider: true })
  items.push({ key: 'delete', label: '删除', shortcut: 'Del', danger: true })

  return items
})

function handleRefresh() {
  props.store.loadFileTree()
}

function handleSelect(node: TreeNode) {
  if (node.type === 'dir') {
    props.store.toggleDir(node.path)
  } else {
    props.store.openFile(node.path, node.name)
  }
}

function handleOpen(node: TreeNode) {
  if (node.type === 'file') {
    props.store.openFile(node.path, node.name)
  }
}

function handleContextMenu(payload: { event: MouseEvent; node: TreeNode }) {
  contextNode.value = payload.node
  contextMenuRef.value?.show(payload.event)
}

function handleTreeContextMenu(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (target.closest('.node-row')) return

  contextNode.value = null
  contextMenuRef.value?.show(event)
}

function handleMenuSelect(key: string) {
  const node = contextNode.value

  if (!node) {
    if (key === 'new-file') emit('create-file', '')
    else if (key === 'new-dir') emit('create-dir', '')
    return
  }

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
      if (props.api.getDownloadUrl) {
        window.open(props.api.getDownloadUrl(node.path), '_blank')
      }
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

async function handleMove(payload: { sourcePath: string; targetPath: string }) {
  const { sourcePath, targetPath } = payload
  const fileName = sourcePath.split('/').pop()
  const newPath = targetPath ? targetPath + '/' + fileName : fileName

  try {
    await props.api.moveItem(sourcePath, newPath!)
    await props.store.loadFileTree()
    ElMessage.success('移动成功')
  } catch (e: any) {
    ElMessage.error(e.message || '移动失败')
  }
}

function handleRootDragOver(event: DragEvent) {
  const target = event.target as HTMLElement
  if (target.closest('.node-row')) {
    isDragOverRoot.value = false
    return
  }
  isDragOverRoot.value = true
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'move'
  }
}

function handleRootDragLeave(event: DragEvent) {
  const relatedTarget = event.relatedTarget as HTMLElement
  if (!relatedTarget?.closest('.tree-content')) {
    isDragOverRoot.value = false
  }
}

function handleRootDrop(event: DragEvent) {
  isDragOverRoot.value = false

  const target = event.target as HTMLElement
  if (target.closest('.node-row')) return

  const sourcePath = event.dataTransfer?.getData('text/plain')
  if (!sourcePath || !sourcePath.includes('/')) return

  handleMove({ sourcePath, targetPath: '' })
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
  props.store.loadFileTree()
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
  transition: background-color 0.2s;
}

.tree-content.drag-over-root {
  background-color: rgba(0, 122, 204, 0.1);
  outline: 1px dashed #007acc;
  outline-offset: -2px;
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
