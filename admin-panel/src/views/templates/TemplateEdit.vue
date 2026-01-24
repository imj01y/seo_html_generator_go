<template>
  <div class="template-edit" v-loading="loading">
    <!-- 顶部操作栏 -->
    <div class="page-header">
      <div class="header-left">
        <el-button @click="goBack" :icon="ArrowLeft">返回</el-button>
        <h2 class="title">{{ template?.display_name || '编辑模板' }}</h2>
        <el-tag v-if="template" size="small">{{ template.name }}</el-tag>
        <el-tag v-if="template" type="info" size="small">v{{ template.version }}</el-tag>
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

    <!-- 基本信息编辑 -->
    <div class="card info-card">
      <el-form :inline="true" label-width="80px">
        <el-form-item label="显示名称">
          <el-input
            v-model="form.display_name"
            style="width: 200px"
            @change="hasInfoChanges = true"
          />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            style="width: 400px"
            @change="hasInfoChanges = true"
          />
        </el-form-item>
        <el-form-item>
          <el-button v-if="hasInfoChanges" type="primary" size="small" @click="saveInfo">
            保存信息
          </el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 代码编辑器 -->
    <div class="card editor-card">
      <div class="editor-header">
        <span class="editor-title">模板代码 (HTML/Jinja2)</span>
        <div class="editor-actions">
          <el-button size="small" @click="formatCode">
            <el-icon><MagicStick /></el-icon>
            格式化
          </el-button>
          <el-button size="small" @click="toggleTheme">
            <el-icon><Sunny v-if="theme === 'vs-dark'" /><Moon v-else /></el-icon>
            {{ theme === 'vs-dark' ? '浅色' : '深色' }}
          </el-button>
        </div>
      </div>
      <MonacoEditor
        ref="editorRef"
        v-model="form.content"
        :language="'html'"
        :theme="theme"
        :height="editorHeight"
        @change="onContentChange"
        @save="handleSave"
      />
    </div>

    <!-- 变量提示 -->
    <div class="card tips-card">
      <div class="tips-header">
        <span>可用变量和函数</span>
      </div>
      <div class="tips-content">
        <el-tag
          v-for="item in templateVariables"
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
import { ElMessage, ElMessageBox } from 'element-plus'
import { useDebounceFn } from '@vueuse/core'
import { ArrowLeft, Check, Loading, CircleCheck, Warning, MagicStick, Sunny, Moon } from '@element-plus/icons-vue'
import MonacoEditor from '@/components/MonacoEditor.vue'
import { getTemplate, updateTemplate } from '@/api/templates'
import type { Template } from '@/types'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const saving = ref(false)
const hasChanges = ref(false)
const hasInfoChanges = ref(false)
const lastSaveTime = ref<Date | null>(null)
const theme = ref<'vs-dark' | 'vs'>('vs-dark')
const editorRef = ref<InstanceType<typeof MonacoEditor>>()

const template = ref<Template | null>(null)
const templateId = computed(() => Number(route.params.id))

const form = reactive({
  display_name: '',
  description: '',
  content: ''
})

// 编辑器高度（减去其他元素高度）
const editorHeight = 'calc(100vh - 350px)'

// 模板变量提示列表
const templateVariables = [
  '{{ title }}',
  '{{ keyword_with_emoji() }}',
  '{{ random_url() }}',
  '{{ random_image() }}',
  '{{ random_hotspot() }}',
  '{{ random_number(min, max) }}',
  '{{ now() }}',
  '{{ cls(name) }}',
  '{{ content_with_pinyin() }}',
  '{{ analytics_code }}',
  '{{ site_id }}'
]

// 保存状态文字
const saveStatusText = computed(() => {
  if (saving.value) return '保存中...'
  if (hasChanges.value) return '未保存'
  if (lastSaveTime.value) {
    return `已保存 ${lastSaveTime.value.toLocaleTimeString()}`
  }
  return ''
})

// 加载模板
const loadTemplate = async () => {
  if (!templateId.value) return

  loading.value = true
  try {
    template.value = await getTemplate(templateId.value)
    form.display_name = template.value.display_name
    form.description = template.value.description || ''
    form.content = template.value.content

    // 检查本地草稿
    const draft = localStorage.getItem(`template_draft_${templateId.value}`)
    if (draft && draft !== template.value.content) {
      ElMessageBox.confirm(
        '检测到本地有未保存的草稿，是否恢复？',
        '恢复草稿',
        {
          confirmButtonText: '恢复',
          cancelButtonText: '放弃',
          type: 'info'
        }
      ).then(() => {
        form.content = draft
        hasChanges.value = true
      }).catch(() => {
        localStorage.removeItem(`template_draft_${templateId.value}`)
      })
    }
  } finally {
    loading.value = false
  }
}

// 内容变化时的处理
const onContentChange = (value: string) => {
  hasChanges.value = true
  // 保存草稿到本地
  localStorage.setItem(`template_draft_${templateId.value}`, value)
  // 触发自动保存
  autoSave()
}

// 自动保存（防抖3秒）
const autoSave = useDebounceFn(async () => {
  if (!hasChanges.value || !templateId.value) return
  await saveContent()
}, 3000)

// 保存内容
const saveContent = async () => {
  if (!templateId.value) return

  saving.value = true
  try {
    const res = await updateTemplate(templateId.value, {
      content: form.content
    })
    if (res.success) {
      hasChanges.value = false
      lastSaveTime.value = new Date()
      // 清除本地草稿
      localStorage.removeItem(`template_draft_${templateId.value}`)
      // 更新版本号
      if (template.value) {
        template.value.version += 1
      }
    } else {
      ElMessage.error(res.message || '保存失败')
    }
  } catch (e) {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

// 手动保存
const handleSave = async () => {
  await saveContent()
  if (!hasChanges.value) {
    ElMessage.success('保存成功')
  }
}

// 保存基本信息
const saveInfo = async () => {
  if (!templateId.value) return

  try {
    const res = await updateTemplate(templateId.value, {
      display_name: form.display_name,
      description: form.description
    })
    if (res.success) {
      hasInfoChanges.value = false
      ElMessage.success('信息已更新')
      if (template.value) {
        template.value.display_name = form.display_name
        template.value.description = form.description
      }
    } else {
      ElMessage.error(res.message || '更新失败')
    }
  } catch {
    ElMessage.error('更新失败')
  }
}

// 格式化代码
const formatCode = () => {
  editorRef.value?.formatDocument()
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
      await saveContent()
      router.push('/templates')
    }).catch((action) => {
      if (action === 'cancel') {
        localStorage.removeItem(`template_draft_${templateId.value}`)
        router.push('/templates')
      }
    })
  } else {
    router.push('/templates')
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
  loadTemplate()
  window.addEventListener('beforeunload', handleBeforeUnload)
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
})
</script>

<style lang="scss" scoped>
.template-edit {
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
