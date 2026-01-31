<template>
  <CodeEditorPanel
    :api="workerApi"
    title="Worker 代码管理"
    :runnable="true"
    :show-log-panel="true"
    :show-restart-button="true"
    :show-rebuild-button="true"
    :runnable-extensions="['.py']"
    @restart="handleRestart"
    @rebuild="handleRebuild"
  />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import CodeEditorPanel from '@/components/CodeEditorPanel/index.vue'
import {
  getFileTree,
  getFile,
  saveFile,
  createItem,
  deleteItem,
  moveItem,
  runFile,
  getDownloadUrl,
  restartWorker,
  rebuildWorker
} from '@/api/worker'
import type { CodeEditorApi } from '@/components/CodeEditorPanel/types'

// 创建 Worker API 适配器
const workerApi: CodeEditorApi = {
  getFileTree,
  getFile,
  saveFile,
  createItem,
  deleteItem,
  moveItem,
  runFile,
  getDownloadUrl
}

const restarting = ref(false)
const rebuilding = ref(false)

// 重启 Worker
async function handleRestart() {
  try {
    await ElMessageBox.confirm('确定重启 Worker 吗？', '确认重启', { type: 'warning' })
    restarting.value = true
    await restartWorker()
    ElMessage.success('重启指令已发送')
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '重启失败')
    }
  } finally {
    restarting.value = false
  }
}

// 重建 Worker
async function handleRebuild() {
  try {
    await ElMessageBox.confirm(
      '重新构建将重新安装所有依赖，可能需要几分钟时间。确定继续吗？',
      '确认重建',
      { type: 'warning' }
    )
    rebuilding.value = true
    await rebuildWorker()
    ElMessage.success('Worker 重新构建完成')
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '重建失败')
    }
  } finally {
    rebuilding.value = false
  }
}
</script>
