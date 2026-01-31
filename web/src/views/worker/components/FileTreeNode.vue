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
        @select="$emit('select', $event)"
        @open="$emit('open', $event)"
        @context-menu="$emit('context-menu', $event)"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { CaretRight, Folder, Document } from '@element-plus/icons-vue'
import { useWorkerEditorStore } from '@/stores/workerEditor'
import type { TreeNode } from '@/api/worker'

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
const isModified = computed(() => {
  const tab = store.tabs.find(t => t.path === props.node.path)
  return tab ? tab.content !== tab.originalContent : false
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
