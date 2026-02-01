<template>
  <CodeEditorPanel
    ref="editorPanel"
    :api="contentWorkerApi"
    title="内容处理代码"
    :show-restart-button="true"
    :show-logs-button="true"
    :show-log-panel="true"
    :logs-active="logsActive"
    @restart="handleRestart"
    @toggle-logs="handleToggleLogs"
  />
</template>

<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import CodeEditorPanel from '@/components/CodeEditorPanel/index.vue'
import {
  getFileTree,
  getFile,
  saveFile,
  createItem,
  deleteItem,
  moveItem,
  getDownloadUrl
} from '@/api/contentWorker'
import type { CodeEditorApi } from '@/components/CodeEditorPanel/types'

// 创建内容处理 API 适配器
const contentWorkerApi: CodeEditorApi = {
  getFileTree,
  getFile,
  saveFile,
  createItem,
  deleteItem,
  moveItem,
  getDownloadUrl
}

const editorPanel = ref<InstanceType<typeof CodeEditorPanel> | null>(null)
const logsActive = ref(false)
let logsWs: WebSocket | null = null

// 获取 WebSocket URL
function getWsUrl(path: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  return `${protocol}//${host}${path}`
}

// 处理 WebSocket 消息（重启日志）
function handleWsMessage(event: MessageEvent) {
  const store = editorPanel.value?.store
  if (!store) return

  try {
    const msg = JSON.parse(event.data)

    if (msg.type === 'done') {
      // 重启完成
      store.setLogRunning(false)
    } else {
      // 普通日志
      store.addLog({
        type: msg.type as 'stdout' | 'stderr' | 'info',
        data: msg.data
      })
    }
  } catch {
    // 非 JSON 消息，作为普通日志处理
    store.addLog({ type: 'stdout', data: event.data })
  }
}

// 日志级别到显示类型的映射
const LOG_LEVEL_MAP: Record<string, 'stdout' | 'stderr' | 'info'> = {
  ERROR: 'stderr',
  WARNING: 'info'
}

// 处理数据处理日志消息
function handleProcessorWsMessage(event: MessageEvent) {
  const store = editorPanel.value?.store
  if (!store) return

  try {
    const msg = JSON.parse(event.data)

    if (msg.type === 'log') {
      const level = msg.level?.toUpperCase() || 'INFO'
      const logType = LOG_LEVEL_MAP[level] || 'stdout'

      store.addLog({
        type: logType,
        data: `[${level}] ${msg.message}`
      })
    }
  } catch {
    // 非 JSON 消息，作为普通日志处理
    store.addLog({ type: 'stdout', data: event.data })
  }
}

// 重启内容处理服务
async function handleRestart() {
  try {
    await ElMessageBox.confirm(
      '重启将安装依赖并重启容器，当前正在执行的任务会被中断。确定继续吗？',
      '确认重启',
      { type: 'warning' }
    )
  } catch {
    return // 用户取消
  }

  // 如果正在监听日志，先停止
  if (logsActive.value) {
    stopLogsWs()
  }

  const store = editorPanel.value?.store
  if (!store) return

  // 清空日志并展开面板
  store.clearLogs()
  store.setLogRunning(true)
  store.addLog({ type: 'command', data: '> 正在连接...' })

  // 建立 WebSocket 连接
  const ws = new WebSocket(getWsUrl('/ws/worker-restart'))

  ws.onopen = () => {
    store.addLog({ type: 'info', data: '> 连接成功，开始重启...' })
  }

  ws.onmessage = handleWsMessage

  ws.onerror = () => {
    store.addLog({ type: 'stderr', data: '> WebSocket 连接错误' })
    store.setLogRunning(false)
  }

  ws.onclose = () => {
    if (store.logRunning.value) {
      store.addLog({ type: 'info', data: '> 连接已断开' })
      store.setLogRunning(false)
    }
  }
}

// 切换实时日志
function handleToggleLogs() {
  if (logsActive.value) {
    stopLogsWs()
  } else {
    startLogsWs()
  }
}

// 开始监听日志
function startLogsWs() {
  const store = editorPanel.value?.store
  if (!store) return

  store.clearLogs()
  store.setLogRunning(true)
  store.addLog({ type: 'command', data: '> 正在连接实时日志...' })

  logsWs = new WebSocket(getWsUrl('/ws/processor-logs'))

  logsWs.onopen = () => {
    logsActive.value = true
    store.addLog({ type: 'info', data: '> 已连接，正在监听数据处理日志...' })
  }

  logsWs.onmessage = handleProcessorWsMessage

  logsWs.onerror = () => {
    store.addLog({ type: 'stderr', data: '> WebSocket 连接错误' })
    logsActive.value = false
    store.setLogRunning(false)
  }

  logsWs.onclose = () => {
    logsActive.value = false
    store.addLog({ type: 'info', data: '> 日志监听已停止' })
    store.setLogRunning(false)
    logsWs = null
  }
}

// 停止监听日志
function stopLogsWs() {
  if (logsWs) {
    logsWs.close()
    logsWs = null
  }
  logsActive.value = false
}

// 组件卸载时清理
onUnmounted(() => {
  stopLogsWs()
})
</script>
