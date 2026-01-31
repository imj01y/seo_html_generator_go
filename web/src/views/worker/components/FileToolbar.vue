<template>
  <div class="file-toolbar">
    <!-- 面包屑导航 -->
    <el-breadcrumb separator="/">
      <el-breadcrumb-item @click="$emit('navigate', '')">
        <el-icon><Folder /></el-icon> worker
      </el-breadcrumb-item>
      <el-breadcrumb-item
        v-for="(segment, index) in pathSegments"
        :key="index"
        @click="navigateToSegment(index)"
      >
        {{ segment }}
      </el-breadcrumb-item>
    </el-breadcrumb>

    <!-- 操作按钮 -->
    <div class="actions">
      <el-upload
        :action="uploadUrl"
        :headers="uploadHeaders"
        :show-file-list="false"
        :on-success="onUploadSuccess"
        :on-error="onUploadError"
        multiple
        name="files"
      >
        <el-button :icon="Upload">上传</el-button>
      </el-upload>
      <el-button :icon="DocumentAdd" @click="$emit('create-file')">新建文件</el-button>
      <el-button :icon="FolderAdd" @click="$emit('create-dir')">新建目录</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Upload, DocumentAdd, FolderAdd, Folder } from '@element-plus/icons-vue'

const props = defineProps<{
  currentPath: string
}>()

const emit = defineEmits<{
  (e: 'navigate', path: string): void
  (e: 'upload-success'): void
  (e: 'create-file'): void
  (e: 'create-dir'): void
}>()

const pathSegments = computed(() => {
  if (!props.currentPath) return []
  return props.currentPath.split('/').filter(Boolean)
})

const uploadUrl = computed(() => {
  return `/api/worker/upload/${props.currentPath || ''}`
})

const uploadHeaders = computed(() => {
  return {
    Authorization: `Bearer ${localStorage.getItem('token')}`
  }
})

function navigateToSegment(index: number) {
  const segments = pathSegments.value.slice(0, index + 1)
  emit('navigate', segments.join('/'))
}

function onUploadSuccess() {
  ElMessage.success('上传成功')
  emit('upload-success')
}

function onUploadError(error: any) {
  ElMessage.error('上传失败: ' + (error.message || '未知错误'))
}
</script>

<style scoped>
.file-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 15px;
  background: #f5f7fa;
  border-radius: 4px;
  margin-bottom: 15px;
}

.el-breadcrumb {
  font-size: 14px;
}

.el-breadcrumb-item {
  cursor: pointer;
}

.el-breadcrumb-item:hover {
  color: #409eff;
}

.actions {
  display: flex;
  gap: 10px;
}
</style>
