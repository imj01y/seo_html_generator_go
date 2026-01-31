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
