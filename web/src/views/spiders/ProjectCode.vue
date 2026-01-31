<template>
  <div class="project-code-page">
    <CodeEditorPanel
      ref="editorRef"
      :api="editorApi"
      :title="pageTitle"
      :runnable="false"
      :show-log-panel="true"
      :extra-tabs="extraTabs"
    >
      <template #header-actions>
        <el-button @click="showGuide">
          <el-icon><QuestionFilled /></el-icon>
          指南
        </el-button>
        <el-input-number
          v-model="testMaxItems"
          :min="0"
          :max="10000"
          placeholder="0=不限制"
          style="width: 120px"
        />
        <el-tooltip content="0 表示不限制测试条数" placement="top">
          <el-button type="success" :loading="testing" @click="handleTest">
            {{ testing ? '测试中...' : '测试运行' }}
          </el-button>
        </el-tooltip>
        <el-button v-if="testing" type="danger" @click="handleStopTest">
          停止
        </el-button>
        <el-button @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
      </template>
    </CodeEditorPanel>

    <!-- 爬虫指南 -->
    <SpiderGuide ref="guideRef" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, markRaw } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, QuestionFilled } from '@element-plus/icons-vue'
import CodeEditorPanel from '@/components/CodeEditorPanel/index.vue'
import SpiderGuide from '@/components/SpiderGuide.vue'
import DataPreview from './components/DataPreview.vue'
import {
  getProject,
  createSpiderEditorApi,
  testProject,
  stopTestProject,
  subscribeTestLogs,
  type SpiderProject
} from '@/api/spiderProjects'
import type { ExtraTab } from '@/components/CodeEditorPanel/types'

const route = useRoute()
const router = useRouter()
const editorRef = ref<InstanceType<typeof CodeEditorPanel>>()
const guideRef = ref<InstanceType<typeof SpiderGuide>>()

// 项目信息
const projectId = computed(() => Number(route.params.id))
const project = ref<SpiderProject | null>(null)
const pageTitle = computed(() => project.value ? `${project.value.name} - 代码编辑` : '代码编辑')

// API 适配器
const editorApi = computed(() => createSpiderEditorApi(projectId.value))

// 测试状态
const testing = ref(false)
const testMaxItems = ref(0)
const testItems = ref<Record<string, any>[]>([])
let unsubscribeTest: (() => void) | null = null

// 日志面板额外标签页
const extraTabs = computed<ExtraTab[]>(() => [
  {
    key: 'data',
    label: '数据',
    badge: testItems.value.length || undefined,
    component: markRaw(DataPreview),
    props: { items: testItems.value }
  }
])

// 加载项目信息
async function loadProject() {
  try {
    project.value = await getProject(projectId.value)
  } catch (e: any) {
    ElMessage.error(e.message || '加载项目失败')
    router.push('/spiders/projects')
  }
}

// 测试运行
async function handleTest() {
  // 先保存所有修改
  const store = editorRef.value?.store
  if (store?.hasModifiedFiles.value) {
    for (const tab of store.modifiedTabs.value) {
      await store.saveTab(tab.id)
    }
  }

  // 清理状态
  unsubscribeTest?.()
  testing.value = true
  testItems.value = []

  // 展开日志面板
  if (store) {
    store.logExpanded.value = true
    store.clearLogs()
    store.addLog({ type: 'command', data: '> 开始测试运行...' })
  }

  try {
    const res = await testProject(projectId.value, testMaxItems.value)
    if (!res.success) {
      store?.addLog({ type: 'stderr', data: res.message || '启动测试失败' })
      testing.value = false
      return
    }

    // 订阅日志
    unsubscribeTest = subscribeTestLogs(
      projectId.value,
      (level, message) => {
        store?.addLog({
          type: level === 'ERROR' ? 'stderr' : 'stdout',
          data: `[${level}] ${message}`
        })
      },
      (item) => {
        testItems.value.push(item)
      },
      () => {
        testing.value = false
        store?.addLog({ type: 'info', data: '> 测试运行完成' })
      },
      (error) => {
        store?.addLog({ type: 'stderr', data: error })
        testing.value = false
      }
    )
  } catch (e: any) {
    store?.addLog({ type: 'stderr', data: e.message || '测试失败' })
    testing.value = false
  }
}

// 停止测试
async function handleStopTest() {
  try {
    await stopTestProject(projectId.value)
    editorRef.value?.store?.addLog({ type: 'info', data: '> 正在停止测试...' })
  } catch (e: any) {
    ElMessage.error(e.message || '停止失败')
  }
}

// 显示指南
function showGuide() {
  guideRef.value?.show()
}

// 返回列表
function goBack() {
  router.push('/spiders/projects')
}

onMounted(() => {
  loadProject()
})

onUnmounted(() => {
  unsubscribeTest?.()
})
</script>

<style scoped>
.project-code-page {
  padding: 20px;
  height: calc(100vh - 60px);
  box-sizing: border-box;
}
</style>
