<template>
  <div class="tree-node">
    <!-- 节点行 -->
    <div
      :class="['node-row', { active: isActive, selected: isSelected, 'drag-over': isDragOver }]"
      :style="{ paddingLeft: depth * 16 + 8 + 'px' }"
      draggable="true"
      @click="handleClick"
      @dblclick="handleDblClick"
      @contextmenu.prevent.stop="handleContextMenu"
      @dragstart="handleDragStart"
      @dragover.prevent="handleDragOver"
      @dragleave="handleDragLeave"
      @drop.prevent="handleDrop"
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
        <Document v-else />
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
        :store="store"
        @select="$emit('select', $event)"
        @open="$emit('open', $event)"
        @context-menu="$emit('context-menu', $event)"
        @move="$emit('move', $event)"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { CaretRight, Folder, Document } from '@element-plus/icons-vue'
import type { TreeNode } from '../types'
import type { EditorStore } from '../composables/useEditorStore'

const props = defineProps<{
  node: TreeNode
  depth?: number
  activePath?: string | null
  store: EditorStore
}>()

const emit = defineEmits<{
  (e: 'select', node: TreeNode): void
  (e: 'open', node: TreeNode): void
  (e: 'context-menu', payload: { event: MouseEvent; node: TreeNode }): void
  (e: 'move', payload: { sourcePath: string; targetPath: string }): void
}>()

const isDragOver = ref(false)

const depth = computed(() => props.depth ?? 0)

const isExpanded = computed(() => props.store.isDirExpanded(props.node.path))
const isActive = computed(() => props.activePath === props.node.path)
const isSelected = computed(() =>
  props.store.tabs.value.some(t => t.path === props.node.path)
)
const isModified = computed(() => {
  const tab = props.store.tabs.value.find(t => t.path === props.node.path)
  return tab ? tab.content !== tab.originalContent : false
})

const sortedChildren = computed(() => {
  if (!props.node.children) return []
  return [...props.node.children].sort((a, b) => {
    if (a.type !== b.type) {
      return a.type === 'dir' ? -1 : 1
    }
    return a.name.localeCompare(b.name)
  })
})

function toggleExpand() {
  if (props.node.type === 'dir') {
    props.store.toggleDir(props.node.path)
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

// 拖放处理
function handleDragStart(event: DragEvent) {
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', props.node.path)
  }
}

function handleDragOver(event: DragEvent) {
  // 只有目录可以作为放置目标
  if (props.node.type === 'dir') {
    isDragOver.value = true
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move'
    }
  }
}

function handleDragLeave() {
  isDragOver.value = false
}

function handleDrop(event: DragEvent) {
  isDragOver.value = false
  if (props.node.type !== 'dir') return

  const sourcePath = event.dataTransfer?.getData('text/plain')
  if (!sourcePath) return

  // 不能移动到自身或自身的子目录
  if (sourcePath === props.node.path || props.node.path.startsWith(sourcePath + '/')) {
    return
  }

  emit('move', { sourcePath, targetPath: props.node.path })
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

.node-row.drag-over {
  background: #264f78;
  outline: 1px dashed #007acc;
  outline-offset: -1px;
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
