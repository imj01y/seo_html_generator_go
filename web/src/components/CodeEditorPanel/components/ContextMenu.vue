<template>
  <Teleport to="body">
    <div
      v-if="visible"
      ref="menuRef"
      class="ce-context-menu"
      :style="{ left: x + 'px', top: y + 'px' }"
      @contextmenu.prevent
    >
      <template v-for="item in items" :key="item.key">
        <div v-if="item.divider" class="ce-menu-divider"></div>
        <div
          v-else
          :class="['ce-menu-item', { danger: item.danger, disabled: item.disabled }]"
          @click="handleClick(item)"
        >
          <span class="ce-menu-label">{{ item.label }}</span>
          <span v-if="item.shortcut" class="ce-menu-shortcut">{{ item.shortcut }}</span>
        </div>
      </template>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import type { MenuItem } from '../types'

defineProps<{
  items: MenuItem[]
}>()

const emit = defineEmits<{
  (e: 'select', key: string): void
  (e: 'close'): void
}>()

const visible = ref(false)
const x = ref(0)
const y = ref(0)
const menuRef = ref<HTMLElement>()

function show(event: MouseEvent) {
  x.value = event.clientX
  y.value = event.clientY
  visible.value = true

  setTimeout(() => {
    if (menuRef.value) {
      const rect = menuRef.value.getBoundingClientRect()
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
  if (item.disabled) return
  emit('select', item.key)
  hide()
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.ce-context-menu')) {
    hide()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

defineExpose({ show, hide })
</script>

<style>
/* 不使用 scoped，因为 Teleport 到 body 时 scoped 样式可能不生效 */
/* 使用 ce- 前缀避免全局样式冲突 */

.ce-context-menu {
  position: fixed;
  z-index: 9999;
  background: #252526;
  border: 1px solid #3c3c3c;
  border-radius: 6px;
  padding: 4px 0;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.5);
  min-width: 120px;
}

.ce-menu-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 12px;
  cursor: pointer;
  color: #cccccc;
  font-size: 13px;
  line-height: 20px;
}

.ce-menu-item:hover:not(.disabled) {
  background: #094771;
}

.ce-menu-item.danger .ce-menu-label {
  color: #f48771;
}

.ce-menu-item.disabled {
  color: #6e6e6e;
  cursor: not-allowed;
}

.ce-menu-divider {
  height: 1px;
  background: #3c3c3c;
  margin: 4px 0;
}

.ce-menu-label {
  flex: 1;
}

.ce-menu-shortcut {
  color: #6e6e6e;
  font-size: 12px;
  margin-left: 16px;
}
</style>
