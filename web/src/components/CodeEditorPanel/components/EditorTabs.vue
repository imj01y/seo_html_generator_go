<template>
  <div class="editor-tabs" v-if="store.tabs.value.length > 0">
    <div class="tabs-scroll">
      <div
        v-for="tab in store.tabs.value"
        :key="tab.id"
        :class="['tab', { active: tab.id === store.activeTabId.value }]"
        @click="store.setActiveTab(tab.id)"
        @contextmenu.prevent="showTabMenu($event, tab)"
        @mousedown.middle.prevent="store.closeTab(tab.id)"
      >
        <span class="tab-name">{{ tab.name }}</span>
        <span v-if="store.isTabModified(tab.id)" class="modified-indicator">●</span>
        <el-icon class="close-btn" @click.stop="store.closeTab(tab.id)">
          <Close />
        </el-icon>
      </div>
    </div>

    <!-- 标签页右键菜单 -->
    <ContextMenu
      ref="tabMenuRef"
      :items="tabMenuItems"
      @select="handleTabMenuSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Close } from '@element-plus/icons-vue'
import type { Tab, MenuItem } from '../types'
import type { EditorStore } from '../composables/useEditorStore'
import ContextMenu from './ContextMenu.vue'

const props = defineProps<{
  store: EditorStore
}>()

const tabMenuRef = ref<InstanceType<typeof ContextMenu>>()
const contextTab = ref<Tab | null>(null)

const tabMenuItems: MenuItem[] = [
  { key: 'close', label: '关闭' },
  { key: 'close-others', label: '关闭其他' },
  { key: 'close-right', label: '关闭右侧' },
  { key: 'divider', divider: true },
  { key: 'copy-path', label: '复制路径' }
]

function showTabMenu(event: MouseEvent, tab: Tab) {
  contextTab.value = tab
  tabMenuRef.value?.show(event)
}

function handleTabMenuSelect(key: string) {
  if (!contextTab.value) return
  const tab = contextTab.value

  switch (key) {
    case 'close':
      props.store.closeTab(tab.id)
      break
    case 'close-others':
      props.store.closeOtherTabs(tab.id)
      break
    case 'close-right':
      props.store.closeTabsToRight(tab.id)
      break
    case 'copy-path':
      navigator.clipboard.writeText(tab.path)
      break
  }
}
</script>

<style scoped>
.editor-tabs {
  display: flex;
  background: #252526;
  border-bottom: 1px solid #3c3c3c;
  height: 35px;
}

.tabs-scroll {
  display: flex;
  overflow-x: auto;
  scrollbar-width: none;
}

.tabs-scroll::-webkit-scrollbar {
  display: none;
}

.tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 12px;
  height: 35px;
  border-right: 1px solid #3c3c3c;
  cursor: pointer;
  background: #2d2d2d;
  color: #969696;
  font-size: 13px;
  white-space: nowrap;
}

.tab:hover {
  background: #323232;
}

.tab.active {
  background: #1e1e1e;
  color: #ffffff;
}

.tab-name {
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.modified-indicator {
  color: #e2c08d;
  font-size: 10px;
}

.close-btn {
  font-size: 14px;
  color: #969696;
  border-radius: 3px;
  padding: 2px;
}

.close-btn:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.1);
}
</style>
