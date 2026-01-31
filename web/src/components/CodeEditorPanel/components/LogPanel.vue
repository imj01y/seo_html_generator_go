<template>
  <div
    class="log-panel"
    :class="{ expanded: store.logExpanded.value }"
    :style="{ height: store.logExpanded.value ? height + 'px' : '28px' }"
  >
    <!-- 拖拽调整高度 -->
    <div
      v-if="store.logExpanded.value"
      class="resize-handle"
      @mousedown="startResize"
    ></div>

    <!-- 标题栏 -->
    <div class="panel-header" @click="toggleExpand">
      <div class="header-left">
        <el-icon class="expand-icon">
          <CaretRight v-if="!store.logExpanded.value" />
          <CaretBottom v-else />
        </el-icon>
        <span class="title">运行日志</span>
        <span v-if="store.logRunning.value" class="running-badge">运行中...</span>
        <span v-else-if="store.logs.value.length === 0" class="empty-badge">无输出</span>
      </div>
      <div class="header-right" @click.stop>
        <el-button
          v-if="store.logRunning.value"
          text
          size="small"
          type="danger"
          @click="$emit('stop')"
        >
          停止
        </el-button>
        <el-button text size="small" @click="handleCopy">复制</el-button>
        <el-button text size="small" @click="store.clearLogs">清空</el-button>
      </div>
    </div>

    <!-- 日志内容 -->
    <div v-if="store.logExpanded.value" class="log-content" ref="logContent">
      <div
        v-for="(log, index) in store.logs.value"
        :key="index"
        :class="['log-line', log.type]"
      >
        <span class="log-text">{{ log.data }}</span>
        <span class="log-time">{{ formatTime(log.timestamp) }}</span>
      </div>
      <div v-if="store.logs.value.length === 0" class="empty-log">
        运行文件查看输出
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { CaretRight, CaretBottom } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { EditorStore } from '../composables/useEditorStore'

const props = defineProps<{
  store: EditorStore
}>()

defineEmits<{
  (e: 'stop'): void
}>()

const logContent = ref<HTMLElement>()
const height = ref(200)

function toggleExpand() {
  props.store.logExpanded.value = !props.store.logExpanded.value
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
  const text = props.store.logs.value.map(l => l.data).join('\n')
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制到剪贴板')
}

// 自动滚动到底部
watch(() => props.store.logs.value.length, () => {
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
