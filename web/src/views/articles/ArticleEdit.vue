<template>
  <div class="article-edit">
    <div class="page-header">
      <h2 class="title">{{ isEdit ? '编辑文章' : '新增文章' }}</h2>
      <el-button @click="router.back()">返回</el-button>
    </div>

    <div class="card">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px" v-loading="loading">
        <el-form-item label="所属分组" prop="group_id">
          <el-select v-model="form.group_id" placeholder="选择文章分组">
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="标题" prop="title">
          <el-input v-model="form.title" placeholder="请输入文章标题" />
        </el-form-item>
        <el-form-item label="内容" prop="content">
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="20"
            placeholder="请输入文章内容（支持HTML）"
          />
        </el-form-item>
        <el-form-item label="原始URL" v-if="isEdit && form.source_url">
          <a :href="form.source_url" target="_blank" rel="noopener noreferrer" class="source-link">
            {{ form.source_url }}
          </a>
        </el-form-item>
        <el-form-item label="抓取时间" v-if="isEdit && form.created_at">
          <span class="info-text">{{ formatDateTime(form.created_at) }}</span>
        </el-form-item>
        <el-form-item label="状态" v-if="isEdit">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="submitLoading" @click="handleSubmit">
            {{ isEdit ? '保存' : '创建' }}
          </el-button>
          <el-button @click="router.back()">取消</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, FormInstance, FormRules } from 'element-plus'
import { getArticleGroups, getArticle, createArticle, updateArticle } from '@/api/articles'
import type { ArticleGroup } from '@/types'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const groups = ref<ArticleGroup[]>([])

const isEdit = computed(() => !!route.params.id)

const form = reactive({
  group_id: 0,
  title: '',
  content: '',
  status: 1,
  source_url: '',
  created_at: ''
})

const formatDateTime = (dateStr: string) => {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

const rules: FormRules = {
  group_id: [{ required: true, message: '请选择文章分组', trigger: 'change' }],
  title: [{ required: true, message: '请输入文章标题', trigger: 'blur' }],
  content: [{ required: true, message: '请输入文章内容', trigger: 'blur' }]
}

const loadGroups = async () => {
  groups.value = await getArticleGroups()
}

const loadArticle = async () => {
  if (!isEdit.value) return
  loading.value = true
  try {
    const article = await getArticle(Number(route.params.id))
    form.group_id = article.group_id
    form.title = article.title
    form.content = article.content
    form.status = article.status
    form.source_url = article.source_url || ''
    form.created_at = article.created_at || ''
  } catch (e) {
    ElMessage.warning((e as Error).message || '无法加载文章')
    router.back()
  } finally {
    loading.value = false
  }
}

const handleSubmit = async () => {
  await formRef.value?.validate()
  submitLoading.value = true
  try {
    if (isEdit.value) {
      await updateArticle(Number(route.params.id), {
        group_id: form.group_id,
        title: form.title,
        content: form.content,
        status: form.status
      })
      ElMessage.success('保存成功')
    } else {
      await createArticle({
        group_id: form.group_id,
        title: form.title,
        content: form.content
      })
      ElMessage.success('创建成功')
    }
    router.back()
  } catch (e) {
    ElMessage.warning((e as Error).message || '操作失败')
  } finally {
    submitLoading.value = false
  }
}

onMounted(async () => {
  await loadGroups()

  // 设置默认分组
  if (route.query.group_id) {
    form.group_id = Number(route.query.group_id)
  } else if (groups.value.length > 0) {
    form.group_id = groups.value[0].id
  }

  loadArticle()
})
</script>

<style lang="scss" scoped>
.article-edit {
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;

    .title {
      font-size: 20px;
      font-weight: 600;
      color: #303133;
    }
  }

  .source-link {
    color: #409eff;
    text-decoration: none;
    word-break: break-all;

    &:hover {
      text-decoration: underline;
    }
  }

  .info-text {
    color: #606266;
  }
}
</style>
