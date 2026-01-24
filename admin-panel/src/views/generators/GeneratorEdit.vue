<template>
  <div class="generator-edit" v-loading="loading">
    <!-- 顶部操作栏 -->
    <div class="page-header">
      <div class="header-left">
        <el-button @click="goBack" :icon="ArrowLeft">返回</el-button>
        <h2 class="title">{{ isEdit ? (generator?.display_name || '编辑生成器') : '新增生成器' }}</h2>
        <el-tag v-if="generator" size="small">{{ generator.name }}</el-tag>
        <el-tag v-if="generator" type="info" size="small">v{{ generator.version }}</el-tag>
      </div>
      <div class="header-right">
        <span v-if="lastSaveTime" class="save-status">
          <el-icon v-if="saving" class="is-loading"><Loading /></el-icon>
          <el-icon v-else-if="hasChanges" color="#E6A23C"><Warning /></el-icon>
          <el-icon v-else color="#67C23A"><CircleCheck /></el-icon>
          <span>{{ saveStatusText }}</span>
        </span>
        <el-button type="primary" :loading="saving" @click="handleSave">
          <el-icon><Check /></el-icon>
          保存 (Ctrl+S)
        </el-button>
      </div>
    </div>

    <!-- 基本信息 -->
    <div class="card info-card">
      <el-form ref="formRef" :model="form" :rules="rules" :inline="true" label-width="80px">
        <el-form-item v-if="!isEdit" label="标识" prop="name">
          <el-input
            v-model="form.name"
            style="width: 150px"
            placeholder="唯一标识"
            @change="hasChanges = true"
          />
        </el-form-item>
        <el-form-item label="显示名称" prop="display_name">
          <el-input
            v-model="form.display_name"
            style="width: 200px"
            placeholder="显示名称"
            @change="hasChanges = true"
          />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            style="width: 400px"
            placeholder="生成器描述"
            @change="hasChanges = true"
          />
        </el-form-item>
      </el-form>
    </div>

    <!-- 代码编辑器 -->
    <div class="card editor-card">
      <div class="editor-header">
        <span class="editor-title">Python 代码</span>
        <div class="editor-actions">
          <el-dropdown @command="handleSelectTemplate">
            <el-button size="small">
              <el-icon><Document /></el-icon>
              从模板创建
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="tpl in codeTemplates"
                  :key="tpl.name"
                  :command="tpl"
                >
                  <div class="template-item">
                    <span class="template-name">{{ tpl.display_name }}</span>
                    <span class="template-desc">{{ tpl.description }}</span>
                  </div>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button size="small" @click="toggleTheme">
            <el-icon><Sunny v-if="theme === 'vs-dark'" /><Moon v-else /></el-icon>
            {{ theme === 'vs-dark' ? '浅色' : '深色' }}
          </el-button>
          <el-button size="small" type="success" :loading="testing" @click="handleTest">
            <el-icon><CaretRight /></el-icon>
            测试代码
          </el-button>
        </div>
      </div>
      <MonacoEditor
        ref="editorRef"
        v-model="form.code"
        language="python"
        :theme="theme"
        :height="editorHeight"
        @change="onCodeChange"
        @save="handleSave"
      />
    </div>

    <!-- 测试面板 -->
    <div class="card test-card">
      <el-collapse v-model="testPanelActive">
        <el-collapse-item title="测试面板" name="test">
          <div class="test-panel">
            <div class="test-input">
              <div class="panel-title">测试段落（每行一个）</div>
              <el-input
                v-model="testInput"
                type="textarea"
                :rows="6"
                placeholder="这是第一段内容&#10;这是第二段内容&#10;这是第三段内容"
              />
            </div>
            <div class="test-output">
              <div class="panel-title">
                输出结果
                <span v-if="testDuration" class="duration">({{ testDuration }})</span>
              </div>
              <div class="output-content" :class="{ error: testError }">
                <template v-if="testResult">
                  <pre>{{ testResult }}</pre>
                </template>
                <template v-else-if="testError">
                  <pre class="error-text">{{ testError }}</pre>
                </template>
                <template v-else>
                  <span class="placeholder">点击"测试代码"运行</span>
                </template>
              </div>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </div>

    <!-- 变量提示 -->
    <div class="card tips-card">
      <div class="tips-header">
        <span>可用变量和函数</span>
      </div>
      <div class="tips-content">
        <el-tag
          v-for="item in codeVariables"
          :key="item"
          type="info"
          effect="plain"
        >
          {{ item }}
        </el-tag>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import { useDebounceFn } from '@vueuse/core'
import {
  ArrowLeft,
  ArrowDown,
  Check,
  Loading,
  CircleCheck,
  Warning,
  Sunny,
  Moon,
  Document,
  CaretRight
} from '@element-plus/icons-vue'
import MonacoEditor from '@/components/MonacoEditor.vue'
import {
  getGenerator,
  createGenerator,
  updateGenerator,
  testGeneratorCode,
  getCodeTemplates
} from '@/api/generators'
import type { ContentGenerator, CodeTemplate } from '@/types'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const hasChanges = ref(false)
const lastSaveTime = ref<Date | null>(null)
const theme = ref<'vs-dark' | 'vs'>('vs-dark')
const editorRef = ref<InstanceType<typeof MonacoEditor>>()
const formRef = ref<FormInstance>()

const generator = ref<ContentGenerator | null>(null)
const codeTemplates = ref<CodeTemplate[]>([])
const generatorId = computed(() => route.params.id ? Number(route.params.id) : null)
const isEdit = computed(() => !!generatorId.value)

// 测试面板
const testPanelActive = ref<string[]>(['test'])
const testInput = ref('')
const testResult = ref('')
const testError = ref('')
const testDuration = ref('')

const form = reactive({
  name: '',
  display_name: '',
  description: '',
  code: `async def generate(ctx):
    """
    生成正文内容

    可用变量:
        ctx.paragraphs - 段落列表
        ctx.titles - 标题列表
    可用函数:
        annotate_pinyin(text) - 添加拼音标注
    返回:
        str - 生成的HTML内容
    """
    content = ""
    for para in ctx.paragraphs:
        content += f"<p>{para}</p>\\n"
    return content
`
})

const rules: FormRules = {
  name: [
    { required: true, message: '请输入标识', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_]*$/, message: '只能包含小写字母、数字和下划线，且以字母开头', trigger: 'blur' }
  ],
  display_name: [{ required: true, message: '请输入显示名称', trigger: 'blur' }]
}

// 代码变量提示
const codeVariables = [
  'ctx.paragraphs',
  'ctx.titles',
  'annotate_pinyin(text)',
  'random.choice()',
  'random.randint()',
  're.sub()',
  'html模块'
]

// 编辑器高度
const editorHeight = 'calc(100vh - 520px)'

// 保存状态文字
const saveStatusText = computed(() => {
  if (saving.value) return '保存中...'
  if (hasChanges.value) return '未保存'
  if (lastSaveTime.value) {
    return `已保存 ${lastSaveTime.value.toLocaleTimeString()}`
  }
  return ''
})

// 加载生成器
const loadGenerator = async () => {
  if (!generatorId.value) return

  loading.value = true
  try {
    generator.value = await getGenerator(generatorId.value)
    form.name = generator.value.name
    form.display_name = generator.value.display_name
    form.description = generator.value.description || ''
    form.code = generator.value.code

    // 检查本地草稿
    const draft = localStorage.getItem(`generator_draft_${generatorId.value}`)
    if (draft && draft !== generator.value.code) {
      ElMessageBox.confirm(
        '检测到本地有未保存的草稿，是否恢复？',
        '恢复草稿',
        {
          confirmButtonText: '恢复',
          cancelButtonText: '放弃',
          type: 'info'
        }
      ).then(() => {
        form.code = draft
        hasChanges.value = true
      }).catch(() => {
        localStorage.removeItem(`generator_draft_${generatorId.value}`)
      })
    }
  } finally {
    loading.value = false
  }
}

// 加载代码模板
const loadTemplates = async () => {
  try {
    codeTemplates.value = await getCodeTemplates()
  } catch {
    codeTemplates.value = []
  }
}

// 代码变化时的处理
const onCodeChange = (value: string) => {
  hasChanges.value = true
  // 保存草稿到本地
  if (generatorId.value) {
    localStorage.setItem(`generator_draft_${generatorId.value}`, value)
  }
  // 触发自动保存
  if (isEdit.value) {
    autoSave()
  }
}

// 自动保存（防抖3秒）
const autoSave = useDebounceFn(async () => {
  if (!hasChanges.value || !generatorId.value) return
  await saveGenerator()
}, 3000)

// 保存生成器
const saveGenerator = async () => {
  await formRef.value?.validate()

  saving.value = true
  try {
    if (isEdit.value && generatorId.value) {
      const res = await updateGenerator(generatorId.value, {
        display_name: form.display_name,
        description: form.description || undefined,
        code: form.code
      })
      if (res.success) {
        hasChanges.value = false
        lastSaveTime.value = new Date()
        localStorage.removeItem(`generator_draft_${generatorId.value}`)
        if (generator.value) {
          generator.value.version += 1
        }
      } else {
        ElMessage.error(res.message || '保存失败')
      }
    } else {
      const res = await createGenerator({
        name: form.name,
        display_name: form.display_name,
        description: form.description || undefined,
        code: form.code
      })
      if (res.success) {
        hasChanges.value = false
        ElMessage.success('创建成功')
        router.replace(`/generators/edit/${res.id}`)
      } else {
        ElMessage.error(res.message || '创建失败')
      }
    }
  } finally {
    saving.value = false
  }
}

// 手动保存
const handleSave = async () => {
  await saveGenerator()
  if (!hasChanges.value && isEdit.value) {
    ElMessage.success('保存成功')
  }
}

// 选择模板
const handleSelectTemplate = (template: CodeTemplate) => {
  ElMessageBox.confirm(
    `确定要使用模板"${template.display_name}"吗？当前代码将被替换。`,
    '使用模板',
    { type: 'warning' }
  ).then(() => {
    form.code = template.code
    hasChanges.value = true
  }).catch(() => {})
}

// 测试代码
const handleTest = async () => {
  const paragraphs = testInput.value.split('\n').filter(p => p.trim())
  if (paragraphs.length === 0) {
    ElMessage.warning('请输入测试段落')
    return
  }

  testing.value = true
  testResult.value = ''
  testError.value = ''
  testDuration.value = ''

  const startTime = Date.now()

  try {
    const res = await testGeneratorCode({
      code: form.code,
      paragraphs
    })

    testDuration.value = `${((Date.now() - startTime) / 1000).toFixed(2)}s`

    if (res.success) {
      testResult.value = res.content || ''
    } else {
      testError.value = res.message || '执行失败'
    }
  } catch (e: any) {
    testDuration.value = `${((Date.now() - startTime) / 1000).toFixed(2)}s`
    testError.value = e.message || '请求失败'
  } finally {
    testing.value = false
  }
}

// 切换主题
const toggleTheme = () => {
  theme.value = theme.value === 'vs-dark' ? 'vs' : 'vs-dark'
}

// 返回列表
const goBack = () => {
  if (hasChanges.value) {
    ElMessageBox.confirm(
      '有未保存的更改，确定要离开吗？',
      '提示',
      {
        confirmButtonText: '保存并离开',
        cancelButtonText: '直接离开',
        distinguishCancelAndClose: true,
        type: 'warning'
      }
    ).then(async () => {
      await saveGenerator()
      router.push('/generators')
    }).catch((action) => {
      if (action === 'cancel') {
        if (generatorId.value) {
          localStorage.removeItem(`generator_draft_${generatorId.value}`)
        }
        router.push('/generators')
      }
    })
  } else {
    router.push('/generators')
  }
}

// 页面离开前提示
const handleBeforeUnload = (e: BeforeUnloadEvent) => {
  if (hasChanges.value) {
    e.preventDefault()
    e.returnValue = ''
  }
}

onMounted(() => {
  loadGenerator()
  loadTemplates()
  window.addEventListener('beforeunload', handleBeforeUnload)
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
})
</script>

<style lang="scss" scoped>
.generator-edit {
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
    padding: 16px 20px;
    background: #fff;
    border-radius: 4px;

    .header-left {
      display: flex;
      align-items: center;
      gap: 12px;

      .title {
        font-size: 18px;
        font-weight: 600;
        color: #303133;
        margin: 0;
      }
    }

    .header-right {
      display: flex;
      align-items: center;
      gap: 16px;

      .save-status {
        display: flex;
        align-items: center;
        gap: 4px;
        font-size: 13px;
        color: #909399;
      }
    }
  }

  .card {
    background: #fff;
    border-radius: 4px;
    padding: 16px 20px;
    margin-bottom: 16px;
  }

  .info-card {
    :deep(.el-form-item) {
      margin-bottom: 0;
    }
  }

  .editor-card {
    padding: 0;

    .editor-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 16px;
      border-bottom: 1px solid #ebeef5;

      .editor-title {
        font-weight: 500;
        color: #303133;
      }

      .editor-actions {
        display: flex;
        gap: 8px;
      }
    }
  }

  .template-item {
    .template-name {
      font-weight: 500;
    }

    .template-desc {
      font-size: 12px;
      color: #909399;
      display: block;
      margin-top: 2px;
    }
  }

  .test-card {
    padding: 0;

    :deep(.el-collapse-item__header) {
      padding: 0 16px;
      font-weight: 500;
    }

    :deep(.el-collapse-item__content) {
      padding: 0 16px 16px;
    }
  }

  .test-panel {
    display: flex;
    gap: 16px;

    .test-input,
    .test-output {
      flex: 1;
    }

    .panel-title {
      font-weight: 500;
      color: #303133;
      margin-bottom: 8px;

      .duration {
        font-weight: normal;
        color: #909399;
        font-size: 12px;
      }
    }

    .output-content {
      min-height: 150px;
      padding: 12px;
      background: #f5f7fa;
      border-radius: 4px;
      border: 1px solid #ebeef5;
      overflow: auto;

      &.error {
        background: #fef0f0;
        border-color: #fde2e2;
      }

      pre {
        margin: 0;
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 13px;
        white-space: pre-wrap;
        word-break: break-all;
      }

      .error-text {
        color: #f56c6c;
      }

      .placeholder {
        color: #c0c4cc;
      }
    }
  }

  .tips-card {
    .tips-header {
      font-weight: 500;
      color: #303133;
      margin-bottom: 12px;
    }

    .tips-content {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;

      .el-tag {
        font-family: monospace;
      }
    }
  }
}
</style>
