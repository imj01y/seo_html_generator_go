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
