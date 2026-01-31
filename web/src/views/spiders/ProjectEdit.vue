<template>
  <div class="page-container">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-left">
        <el-button @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <h2>{{ isNew ? '新建项目' : `编辑项目: ${form.name}` }}</h2>
      </div>
      <div class="header-actions">
        <el-button @click="showGuide">
          <el-icon><QuestionFilled /></el-icon>
          指南
        </el-button>
        <el-input-number
          v-model="testMaxItems"
          :min="0"
          :max="10000"
          placeholder="0=不限制"
          style="width: 120px; margin-right: 8px;"
        />
        <el-tooltip content="0 表示不限制测试条数" placement="top">
          <el-button type="success" @click="handleTest" :loading="testing">
            测试运行
          </el-button>
        </el-tooltip>
        <el-button type="primary" @click="handleSave" :loading="saving">
          保存
        </el-button>
      </div>
    </div>

    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 左侧：配置 + 文件列表 -->
      <div class="left-panel">
        <!-- 项目配置 -->
        <div class="config-section">
          <div class="section-header">项目配置</div>
          <el-form :model="form" label-position="top" size="small">
            <el-form-item label="项目名称" required>
              <el-input v-model="form.name" placeholder="请输入项目名称" @input="onFormChange" />
            </el-form-item>
            <el-form-item label="描述">
              <el-input v-model="form.description" type="textarea" :rows="2" placeholder="项目描述（可选）" @input="onFormChange" />
            </el-form-item>
            <el-form-item label="入口文件">
              <el-select v-model="form.entry_file" style="width: 100%" @change="onFormChange">
                <el-option
                  v-for="file in files"
                  :key="file.filename"
                  :label="file.filename"
                  :value="file.filename"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="并发数">
              <el-input-number
                v-model="form.concurrency"
                :min="1"
                :max="10"
                style="width: 100%"
                @change="onFormChange"
              />
            </el-form-item>
            <el-form-item label="输出分组">
              <el-select v-model="form.output_group_id" style="width: 100%" @change="onFormChange">
                <el-option
                  v-for="group in articleGroups"
                  :key="group.id"
                  :label="group.name"
                  :value="group.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="调度规则">
              <ScheduleBuilder v-model="form.schedule" @update:modelValue="onFormChange" />
            </el-form-item>
          </el-form>
        </div>

        <!-- 文件列表 -->
        <div class="file-tree-section">
          <div class="section-header">
            <span>文件</span>
            <el-button type="text" size="small" @click="handleAddFile">
              <el-icon><Plus /></el-icon>
              新建文件
            </el-button>
          </div>
          <div class="file-list">
            <div
              v-for="file in files"
              :key="file.filename"
              :class="['file-item', { active: currentFile?.filename === file.filename }]"
              @click="selectFile(file)"
            >
              <el-icon><Document /></el-icon>
              <span class="file-name">{{ file.filename }}</span>
              <el-icon
                v-if="file.filename !== form.entry_file"
                class="delete-icon"
                @click.stop="handleDeleteFile(file)"
              >
                <Delete />
              </el-icon>
            </div>
          </div>
        </div>
      </div>

      <!-- 右侧：代码编辑器 + 日志 -->
      <div class="right-panel">
        <!-- 代码编辑器 -->
        <div class="editor-section">
          <div class="editor-header">
            <span>{{ currentFile?.filename || '选择文件' }}</span>
            <span v-if="lastSaveTime" class="save-status">
              <el-icon v-if="saving" class="is-loading"><Loading /></el-icon>
              <el-icon v-else-if="hasChanges" color="#E6A23C"><Warning /></el-icon>
              <el-icon v-else color="#67C23A"><CircleCheck /></el-icon>
              <span>{{ saveStatusText }}</span>
            </span>
          </div>
          <div class="editor-container">
            <MonacoEditor
              v-model="currentCode"
              language="python"
              height="100%"
              :options="editorOptions"
              @change="handleCodeChange"
              @save="handleSave"
            />
          </div>
        </div>

        <!-- 测试结果/日志 -->
        <div class="log-section" v-if="showTestResult" ref="logSectionRef">
          <div class="log-header">
            <span>测试结果</span>
            <div class="log-actions">
              <el-button v-if="testing" type="danger" size="small" @click="handleStopTest">
                停止测试
              </el-button>
              <el-button type="text" size="small" @click="showTestResult = false">关闭</el-button>
            </div>
          </div>
          <el-tabs v-model="activeTab">
            <el-tab-pane label="日志" name="logs">
              <LogViewer :logs="testLogs" :show-filters="false" />
            </el-tab-pane>
            <el-tab-pane :label="`数据 (${testItems.length})`" name="items">
              <div class="items-list">
                <div v-for="(item, index) in testItems" :key="index"
                     class="item-card clickable"
                     @click="showItemDetail(item)">
                  <div class="item-title">{{ item.title || '(无标题)' }}</div>
                  <div class="item-content">{{ truncateText(item.content, 200) }}</div>
                </div>
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
    </div>

    <!-- 新建文件对话框 -->
    <el-dialog v-model="newFileDialogVisible" title="新建文件" width="400px">
      <el-form>
        <el-form-item label="文件名">
          <el-input v-model="newFileName" placeholder="例如: utils.py">
            <template #append>.py</template>
          </el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="newFileDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmAddFile">创建</el-button>
      </template>
    </el-dialog>

    <!-- 数据详情弹窗 -->
    <el-dialog v-model="itemDetailVisible" title="数据详情" width="700px" top="5vh">
      <div class="item-detail" v-if="currentItem">
        <div class="detail-row" v-for="(value, key) in currentItem" :key="key">
          <div class="detail-label">{{ key }}</div>
          <div class="detail-value" v-if="key === 'content'" v-html="value"></div>
          <div class="detail-value" v-else-if="key === 'source_url'">
            <a :href="value" target="_blank" rel="noopener noreferrer" class="source-link">{{ value }}</a>
          </div>
          <div class="detail-value" v-else>{{ value }}</div>
        </div>
      </div>
      <template #footer>
        <el-button @click="itemDetailVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 爬虫指南 -->
    <SpiderGuide ref="guideRef" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Plus, Document, Delete, QuestionFilled, Loading, CircleCheck, Warning } from '@element-plus/icons-vue'
import MonacoEditor from '@/components/MonacoEditor.vue'
import LogViewer from '@/components/LogViewer.vue'
import SpiderGuide from '@/components/SpiderGuide.vue'
import ScheduleBuilder from '@/components/ScheduleBuilder.vue'
import {
  getProject,
  createProject,
  updateProject,
  getProjectFiles,
  createProjectFile,
  updateProjectFile,
  deleteProjectFile,
  testProject,
  stopTestProject,
  subscribeTestLogs,
  type ProjectFile
} from '@/api/spiderProjects'

const route = useRoute()
const router = useRouter()
const guideRef = ref()
const logSectionRef = ref<HTMLElement | null>(null)

// 判断是否为新建
const projectId = computed(() => {
  const id = route.params.id
  return id === 'create' ? null : Number(id)
})
const isNew = computed(() => !projectId.value)

// 表单数据
const form = reactive({
  name: '',
  description: '',
  entry_file: 'spider.py',
  concurrency: 3,
  output_group_id: 1,
  schedule: '',
  enabled: 1
})

// 文件列表
const files = ref<ProjectFile[]>([])
const currentFile = ref<ProjectFile | null>(null)
const currentCode = ref('')
const originalCode = ref('')  // 用于检测是否有修改

// 文章分组（用于输出目标选择）
const articleGroups = ref([{ id: 1, name: '默认文章分组' }])

// 编辑器配置
const editorOptions = {
  minimap: { enabled: false },
  fontSize: 14,
  lineNumbers: 'on',
  scrollBeyondLastLine: false,
  automaticLayout: true
}

// 状态
const saving = ref(false)
const hasChanges = ref(false)              // 是否有未保存的更改
const lastSaveTime = ref<Date | null>(null) // 上次保存时间
const testing = ref(false)
const showTestResult = ref(false)
const activeTab = ref('logs')
const testLogs = ref<{ time: string; level: string; message: string }[]>([])
const testItems = ref<Record<string, any>[]>([])
const testMaxItems = ref(0)  // 测试条数限制，0 表示不限制
const itemDetailVisible = ref(false)  // 数据详情弹窗
const currentItem = ref<Record<string, any> | null>(null)  // 当前查看的数据项

// 新建文件对话框
const newFileDialogVisible = ref(false)
const newFileName = ref('')

// 测试 WebSocket 取消订阅函数
let unsubscribeTest: (() => void) | null = null

// 标志位：是否正在程序化加载内容（用于区分用户编辑和程序设置）
const isLoadingContent = ref(false)

// 检测是否有未保存的更改（用于切换文件时的检测）
const hasUnsavedChanges = computed(() => {
  return currentCode.value !== originalCode.value
})

// 保存状态文字
const saveStatusText = computed(() => {
  if (saving.value) return '保存中...'
  if (hasChanges.value) return '未保存'
  if (lastSaveTime.value) {
    return `已保存 ${lastSaveTime.value.toLocaleTimeString()}`
  }
  return ''
})

// ============================================
// 草稿自动保存功能
// ============================================

// 草稿存储 key
const draftKey = computed(() =>
  projectId.value ? `spider_project_draft_${projectId.value}` : 'spider_project_draft_new'
)

// 保存草稿到 localStorage
function saveDraft() {
  const draft = {
    form: { ...form },
    files: files.value.map(f => ({ filename: f.filename, content: f.content })),
    currentFileName: currentFile.value?.filename
  }
  localStorage.setItem(draftKey.value, JSON.stringify(draft))
}

// 加载草稿
function loadDraft() {
  const draftStr = localStorage.getItem(draftKey.value)
  if (!draftStr) return null
  try {
    const draft = JSON.parse(draftStr)
    // 验证草稿有效性：必须有文件且文件内容不为空
    if (!draft.files || draft.files.length === 0) return null
    return draft
  } catch {
    return null
  }
}

// 检查草稿是否与默认内容不同（用于新建项目）
function isDraftDifferentFromDefault(draft: any): boolean {
  if (!draft || !draft.files || draft.files.length === 0) return false

  // 如果只有一个文件且是默认入口文件
  if (draft.files.length === 1 && draft.files[0].filename === 'spider.py') {
    const draftContent = draft.files[0].content.trim()
    const defaultContent = getDefaultCode().trim()
    // 内容相同则认为没有修改
    if (draftContent === defaultContent) return false
  }

  // 检查表单是否有修改（名称不为空表示有修改）
  if (draft.form && draft.form.name && draft.form.name.trim()) return true

  // 有多个文件表示有修改
  if (draft.files.length > 1) return true

  // 单文件但内容和默认不同
  return true
}

// 清除草稿
function clearDraft() {
  localStorage.removeItem(draftKey.value)
}

// 防抖自动保存草稿
const autoSaveDraft = useDebounceFn(() => {
  saveDraft()
}, 2000)

// 自动保存到数据库（3秒防抖）- 仅对已存在的项目生效
const autoSaveToDatabase = useDebounceFn(async () => {
  if (isNew.value || !hasChanges.value || saving.value) return
  await saveContentSilently()
}, 3000)

// 静默保存内容（不显示 Message）
async function saveContentSilently() {
  if (!projectId.value) return

  if (currentFile.value) {
    currentFile.value.content = currentCode.value
  }

  saving.value = true
  try {
    await updateProject(projectId.value, {
      name: form.name,
      description: form.description,
      entry_file: form.entry_file,
      concurrency: form.concurrency,
      output_group_id: form.output_group_id,
      schedule: form.schedule
    })

    const serverFiles = await getProjectFiles(projectId.value)
    const serverFileNames = new Set(serverFiles.map(f => f.filename))

    for (const file of files.value) {
      if (serverFileNames.has(file.filename)) {
        await updateProjectFile(projectId.value, file.filename, file.content)
      } else {
        await createProjectFile(projectId.value, { filename: file.filename, content: file.content })
      }
    }

    hasChanges.value = false
    lastSaveTime.value = new Date()
    originalCode.value = currentCode.value
    clearDraft()
  } catch (e) {
    console.error('自动保存失败:', e)
  } finally {
    saving.value = false
  }
}

// 恢复草稿
async function restoreDraft(draft: any) {
  // 恢复表单数据
  Object.assign(form, draft.form)

  // 恢复文件列表
  files.value = draft.files.map((f: any) => ({
    id: 0,
    filename: f.filename,
    content: f.content,
    created_at: '',
    updated_at: ''
  }))

  // 恢复当前选中的文件
  const targetFile = files.value.find(f => f.filename === draft.currentFileName) || files.value[0]
  if (targetFile) {
    selectFile(targetFile)
  }
}

// 加载项目数据
async function loadProject() {
  // 检查是否有草稿
  const draft = loadDraft()

  if (!projectId.value) {
    // 新建项目：只有草稿内容与默认不同时才提示恢复
    if (draft && isDraftDifferentFromDefault(draft)) {
      try {
        await ElMessageBox.confirm('发现未保存的草稿，是否恢复？', '恢复草稿', {
          confirmButtonText: '恢复',
          cancelButtonText: '放弃',
          type: 'info'
        })
        await restoreDraft(draft)
        ElMessage.success('草稿已恢复')
        return
      } catch {
        // 用户选择放弃草稿
        clearDraft()
      }
    }

    // 创建默认文件
    isLoadingContent.value = true
    files.value = [{
      id: 0,
      filename: 'spider.py',
      content: getDefaultCode(),
      created_at: '',
      updated_at: ''
    }]
    currentFile.value = files.value[0]
    currentCode.value = currentFile.value.content
    originalCode.value = currentCode.value
    hasChanges.value = false
    lastSaveTime.value = null
    nextTick(() => {
      isLoadingContent.value = false
    })
    return
  }

  try {
    // 加载项目信息
    const project = await getProject(projectId.value)
    form.name = project.name
    form.description = project.description || ''
    form.entry_file = project.entry_file
    form.concurrency = project.concurrency
    form.output_group_id = project.output_group_id
    form.schedule = project.schedule || ''
    form.enabled = project.enabled

    // 加载文件列表
    files.value = await getProjectFiles(projectId.value)

    // 检查是否有草稿且与服务器数据不同
    if (draft) {
      const serverFilesHash = JSON.stringify(files.value.map(f => ({ filename: f.filename, content: f.content })))
      const draftFilesHash = JSON.stringify(draft.files)

      if (serverFilesHash !== draftFilesHash) {
        try {
          await ElMessageBox.confirm('发现未保存的草稿，是否恢复？', '恢复草稿', {
            confirmButtonText: '恢复',
            cancelButtonText: '放弃',
            type: 'info'
          })
          await restoreDraft(draft)
          ElMessage.success('草稿已恢复')
          return
        } catch {
          // 用户选择放弃草稿
          clearDraft()
        }
      } else {
        // 草稿与服务器数据相同，清除草稿
        clearDraft()
      }
    }

    if (files.value.length > 0) {
      // 默认选中入口文件
      const entryFile = files.value.find(f => f.filename === form.entry_file)
      selectFile(entryFile || files.value[0])
    }

    // 初始化保存状态
    hasChanges.value = false
    lastSaveTime.value = null
  } catch (e: any) {
    ElMessage.error(e.message || '加载失败')
    router.push('/spiders/projects')
  }
}

// 选择文件
function selectFile(file: ProjectFile) {
  // 保存当前文件的修改
  if (currentFile.value && hasUnsavedChanges.value) {
    currentFile.value.content = currentCode.value
  }

  // 标记为程序化加载，避免触发自动保存
  isLoadingContent.value = true

  currentFile.value = file
  currentCode.value = file.content
  originalCode.value = file.content

  // 下一个 tick 后重置标志位
  nextTick(() => {
    isLoadingContent.value = false
  })
}

// 代码变更
function handleCodeChange() {
  // 如果是程序化加载内容，跳过自动保存
  if (isLoadingContent.value) {
    return
  }

  if (currentFile.value) {
    currentFile.value.content = currentCode.value
  }
  hasChanges.value = true
  saveDraft()  // 立即保存草稿
  autoSaveToDatabase()  // 触发3秒防抖自动保存到数据库
}

// 表单变化处理
function onFormChange() {
  hasChanges.value = true
  saveDraft()
  autoSaveToDatabase()
}

// 添加文件
function handleAddFile() {
  newFileName.value = ''
  newFileDialogVisible.value = true
}

async function confirmAddFile() {
  let filename = newFileName.value.trim()
  if (!filename) {
    ElMessage.warning('请输入文件名')
    return
  }

  // 确保以 .py 结尾
  if (!filename.endsWith('.py')) {
    filename += '.py'
  }

  // 检查是否已存在
  if (files.value.some(f => f.filename === filename)) {
    ElMessage.warning('文件已存在')
    return
  }

  const newFile: ProjectFile = {
    id: 0,
    filename,
    content: `# ${filename}\n\n`,
    created_at: '',
    updated_at: ''
  }

  files.value.push(newFile)
  selectFile(newFile)
  newFileDialogVisible.value = false
  ElMessage.success('文件已创建')
}

// 删除文件
async function handleDeleteFile(file: ProjectFile) {
  if (file.filename === form.entry_file) {
    ElMessage.warning('不能删除入口文件')
    return
  }

  try {
    await ElMessageBox.confirm(`确定要删除 ${file.filename} 吗？`, '确认删除')

    // 从列表中移除
    const index = files.value.findIndex(f => f.filename === file.filename)
    if (index > -1) {
      files.value.splice(index, 1)
    }

    // 如果删除的是当前文件，切换到入口文件
    if (currentFile.value?.filename === file.filename) {
      const entryFile = files.value.find(f => f.filename === form.entry_file)
      if (entryFile) selectFile(entryFile)
    }

    ElMessage.success('文件已删除')
  } catch {
    // 用户取消
  }
}

// 保存项目
async function handleSave() {
  if (!form.name.trim()) {
    ElMessage.warning('请输入项目名称')
    return
  }

  // 确保当前文件的内容已更新
  if (currentFile.value) {
    currentFile.value.content = currentCode.value
  }

  saving.value = true
  try {
    if (isNew.value) {
      // 创建项目
      const res = await createProject({
        ...form,
        files: files.value.map(f => ({
          filename: f.filename,
          content: f.content
        }))
      })
      // 保存成功后清除草稿
      clearDraft()
      ElMessage.success('创建成功')
      router.push(`/spiders/projects/${res.id}`)
    } else {
      // 更新项目信息
      await updateProject(projectId.value!, {
        name: form.name,
        description: form.description,
        entry_file: form.entry_file,
        concurrency: form.concurrency,
        output_group_id: form.output_group_id,
        schedule: form.schedule
      })

      // 获取服务器上已有的文件列表
      const serverFiles = await getProjectFiles(projectId.value!)
      const serverFileNames = new Set(serverFiles.map(f => f.filename))

      // 更新所有文件
      for (const file of files.value) {
        if (serverFileNames.has(file.filename)) {
          // 服务器上已有此文件，更新
          await updateProjectFile(projectId.value!, file.filename, file.content)
        } else {
          // 服务器上没有此文件，创建
          await createProjectFile(projectId.value!, {
            filename: file.filename,
            content: file.content
          })
        }
      }

      // 更新原始代码
      originalCode.value = currentCode.value

      // 更新保存状态
      hasChanges.value = false
      lastSaveTime.value = new Date()

      // 保存成功后清除草稿
      clearDraft()

      ElMessage.success('保存成功')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// 添加测试日志
function addTestLog(level: string, message: string) {
  testLogs.value.push({
    time: new Date().toLocaleTimeString(),
    level,
    message
  })
}

// 测试结束处理
function finishTest() {
  testing.value = false
  unsubscribeTest = null
}

// 停止测试
async function handleStopTest() {
  if (!projectId.value) return

  try {
    await stopTestProject(projectId.value)
    addTestLog('INFO', '正在停止测试...')
  } catch (e: any) {
    addTestLog('ERROR', e.message || '停止测试失败')
  }
}

// 测试运行
async function handleTest() {
  if (isNew.value) {
    ElMessage.warning('请先保存项目')
    return
  }

  // 先保存当前修改
  await handleSave()

  // 取消之前的订阅
  if (unsubscribeTest) {
    unsubscribeTest()
    unsubscribeTest = null
  }

  // 重置测试状态
  testing.value = true
  showTestResult.value = true
  testLogs.value = []
  testItems.value = []
  activeTab.value = 'logs'

  // 等待 DOM 更新后滚动到日志区域
  nextTick(() => {
    logSectionRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  })

  try {
    const res = await testProject(projectId.value!, testMaxItems.value)
    if (!res.success) {
      addTestLog('ERROR', res.message || '启动测试失败')
      finishTest()
      return
    }

    // 订阅测试日志 WebSocket
    unsubscribeTest = subscribeTestLogs(
      projectId.value!,
      addTestLog,
      (item) => testItems.value.push(item),
      finishTest,
      (error) => {
        addTestLog('ERROR', error)
        finishTest()
      }
    )
  } catch (e: any) {
    addTestLog('ERROR', e.message || '测试失败')
    finishTest()
  }
}

// 显示指南
function showGuide() {
  guideRef.value?.show()
}

// 返回列表
function goBack() {
  if (hasChanges.value) {
    ElMessageBox.confirm('有未保存的更改，确定要离开吗？', '提示', {
      type: 'warning'
    }).then(() => {
      router.push('/spiders/projects')
    }).catch(() => {})
  } else {
    router.push('/spiders/projects')
  }
}

// 辅助函数
function getDefaultCode() {
  return `from core.crawler import Spider, Request

class MySpider(Spider):
    """
    爬虫示例 - Feapder 风格

    数据格式（必填字段）：
    - title: 文章标题
    - content: 文章内容

    可选字段：source_url, author, publish_date, summary, cover_image, tags

    点击右上角 [指南] 查看完整文档
    """
    name = "example"

    def start_requests(self):
        """生成初始请求"""
        for page in range(1, 10):
            yield Request(f"https://example.com/list?page={page}", meta={'page': page})

    def parse(self, request, response):
        """解析列表页，返回详情页请求"""
        for item in response.css('.article'):
            url = item.css('a::attr(href)').get()
            yield Request(response.urljoin(url), callback=self.parse_detail)

    def parse_detail(self, request, response):
        """解析详情页，返回数据"""
        yield {
            'title': response.css('h1::text').get(),         # 必填
            'content': response.css('.content').get(),       # 必填
            'source_url': response.url,                      # 可选
            'author': response.css('.author::text').get(),   # 可选
        }


# 本地测试（可选）
if __name__ == '__main__':
    spider = MySpider()
    for req in spider.start_requests():
        print(req.url)
`
}

// 显示数据详情
function showItemDetail(item: Record<string, any>) {
  currentItem.value = item
  itemDetailVisible.value = true
}

function truncateText(text: string, maxLength: number) {
  if (!text) return ''
  // 移除 HTML 标签
  const plainText = text.replace(/<[^>]+>/g, '')
  if (plainText.length <= maxLength) return plainText
  return plainText.substring(0, maxLength) + '...'
}

// 页面离开保护
const handleBeforeUnload = (e: BeforeUnloadEvent) => {
  if (hasChanges.value) {
    e.preventDefault()
    e.returnValue = ''
  }
}

// 生命周期
onMounted(() => {
  loadProject()
  window.addEventListener('beforeunload', handleBeforeUnload)
})

onUnmounted(() => {
  // 清理 WebSocket 订阅
  if (unsubscribeTest) {
    unsubscribeTest()
    unsubscribeTest = null
  }
  window.removeEventListener('beforeunload', handleBeforeUnload)
})
</script>

<style scoped>
.page-container {
  padding: 20px;
  min-height: calc(100vh - 60px);  /* 改为 min-height，允许页面向下滚动 */
  display: flex;
  flex-direction: column;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-left h2 {
  margin: 0;
  font-size: 18px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.main-content {
  flex: 1;
  display: flex;
  gap: 16px;
  min-height: 0;
}

.left-panel {
  width: 280px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.right-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.file-tree-section,
.config-section {
  background: #fff;
  border-radius: 4px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.1);
}

.section-header {
  padding: 12px 16px;
  border-bottom: 1px solid #e4e7ed;
  font-weight: 500;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.file-list {
  padding: 8px;
}

.file-item {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  border-radius: 4px;
  cursor: pointer;
  gap: 8px;
}

.file-item:hover {
  background: #f5f7fa;
}

.file-item.active {
  background: #ecf5ff;
  color: #409EFF;
}

.file-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.delete-icon {
  color: #909399;
  opacity: 0;
  transition: opacity 0.2s;
}

.file-item:hover .delete-icon {
  opacity: 1;
}

.delete-icon:hover {
  color: #f56c6c;
}

.config-section {
  padding: 16px;
}

.config-section .section-header {
  padding: 0 0 12px 0;
  margin-bottom: 12px;
}

.editor-section {
  flex: 0 0 auto;   /* 不增长不收缩，高度固定 */
  height: calc(100vh - 160px);  /* 铺满初始可视区域：100vh - 导航栏60px - padding40px - 头部56px - 预留4px */
  background: #fff;
  border-radius: 4px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.1);
  display: flex;
  flex-direction: column;
}

.editor-header {
  padding: 12px 16px;
  border-bottom: 1px solid #e4e7ed;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 8px;
}

.editor-container {
  flex: 1;
  min-height: 0;
  height: 100%;
}

.log-section {
  flex: 0 0 auto;   /* 不增长，不收缩，高度由内容决定 */
  min-height: 200px;
  max-height: 1200px;  /* 最大高度限制，超出后内部滚动 */
  background: #fff;
  border-radius: 4px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.1);
  display: flex;
  flex-direction: column;
  overflow: hidden;  /* 配合内部滚动 */
}

.log-header {
  padding: 12px 16px;
  border-bottom: 1px solid #e4e7ed;
  font-weight: 500;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.log-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.log-section :deep(.el-tabs) {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;  /* 允许收缩 */
}

.log-section :deep(.el-tabs__header) {
  padding: 0 16px;
  margin: 0;
  flex-shrink: 0;
}

.log-section :deep(.el-tabs__content) {
  flex: 1;
  overflow-y: auto;  /* 内容区域可滚动 */
  min-height: 0;
}

.log-section :deep(.el-tab-pane) {
  height: 100%;
}

.items-list {
  padding: 12px;
}

.item-card {
  padding: 12px;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  margin-bottom: 8px;
}

.item-title {
  font-weight: 500;
  margin-bottom: 8px;
}

.item-content {
  font-size: 13px;
  color: #606266;
  line-height: 1.5;
}

.item-card.clickable {
  cursor: pointer;
  transition: background-color 0.2s;
}

.item-card.clickable:hover {
  background-color: #f5f7fa;
}

.item-detail {
  max-height: 70vh;
  overflow-y: auto;
}

.detail-row {
  margin-bottom: 16px;
  border-bottom: 1px solid #ebeef5;
  padding-bottom: 12px;
}

.detail-row:last-child {
  border-bottom: none;
}

.detail-label {
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}

.detail-value {
  color: #606266;
  line-height: 1.6;
  word-break: break-word;
  white-space: pre-wrap;
}

.source-link {
  color: #409EFF;
  text-decoration: none;
  word-break: break-all;
}

.source-link:hover {
  text-decoration: underline;
}

.save-status {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #909399;
  margin-left: auto;
}

.save-status .is-loading {
  animation: rotating 2s linear infinite;
}

@keyframes rotating {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
