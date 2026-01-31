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
import { useWorkerEditorStore } from '@/stores/workerEditor'
import type { TreeNode } from '@/api/worker'
import FileTreeNode from './FileTreeNode.vue'
import ContextMenu from './ContextMenu.vue'
import type { MenuItem } from './ContextMenu.vue'

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
  } else {
    // 单击文件时打开
    store.openFile(node.path, node.name)
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
