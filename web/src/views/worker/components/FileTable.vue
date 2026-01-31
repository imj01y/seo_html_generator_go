<template>
  <div class="file-table-container">
    <el-table
      :data="files"
      v-loading="loading"
      @row-dblclick="handleDblClick"
      style="width: 100%"
    >
      <!-- 文件名 -->
      <el-table-column label="名称" min-width="200">
        <template #default="{ row }">
          <div class="file-name-cell">
            <el-icon v-if="row.type === 'dir'" class="folder-icon"><Folder /></el-icon>
            <el-icon v-else class="file-icon"><Document /></el-icon>
            <span class="file-name">{{ row.name }}</span>
          </div>
        </template>
      </el-table-column>

      <!-- 大小 -->
      <el-table-column label="大小" width="100">
        <template #default="{ row }">
          {{ row.type === 'dir' ? '-' : formatSize(row.size) }}
        </template>
      </el-table-column>

      <!-- 修改时间 -->
      <el-table-column label="修改时间" width="160">
        <template #default="{ row }">
          {{ formatTime(row.mtime) }}
        </template>
      </el-table-column>

      <!-- 操作 -->
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-dropdown @command="handleCommand($event, row)">
            <el-button text type="primary">
              更多 <el-icon><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-if="row.type === 'file'" command="edit">
                  编辑
                </el-dropdown-item>
                <el-dropdown-item command="rename">重命名</el-dropdown-item>
                <el-dropdown-item command="move">移动</el-dropdown-item>
                <el-dropdown-item v-if="row.type === 'file'" command="download">
                  下载
                </el-dropdown-item>
                <el-dropdown-item command="delete" divided>
                  <span style="color: #f56c6c">删除</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>

    <!-- 拖拽上传区域 -->
    <el-upload
      class="upload-dragger"
      drag
      :action="uploadUrl"
      :headers="uploadHeaders"
      :show-file-list="false"
      :on-success="onUploadSuccess"
      multiple
      name="files"
    >
      <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
      <div class="el-upload__text">拖拽文件到此处上传</div>
    </el-upload>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Folder, Document, ArrowDown, UploadFilled } from '@element-plus/icons-vue'
import type { FileInfo } from '@/api/worker'

const props = defineProps<{
  files: FileInfo[]
  loading: boolean
  currentPath: string
}>()

const emit = defineEmits<{
  (e: 'open', item: FileInfo): void
  (e: 'edit', item: FileInfo): void
  (e: 'rename', item: FileInfo): void
  (e: 'move', item: FileInfo): void
  (e: 'download', item: FileInfo): void
  (e: 'delete', item: FileInfo): void
  (e: 'upload-success'): void
}>()

const uploadUrl = computed(() => {
  return `/api/worker/upload/${props.currentPath || ''}`
})

const uploadHeaders = computed(() => {
  return {
    Authorization: `Bearer ${localStorage.getItem('token')}`
  }
})

function handleDblClick(row: FileInfo) {
  emit('open', row)
}

function handleCommand(command: string, row: FileInfo) {
  switch (command) {
    case 'edit':
      emit('edit', row)
      break
    case 'rename':
      emit('rename', row)
      break
    case 'move':
      emit('move', row)
      break
    case 'download':
      emit('download', row)
      break
    case 'delete':
      emit('delete', row)
      break
  }
}

function formatSize(bytes?: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024
    i++
  }
  return `${bytes.toFixed(1)} ${units[i]}`
}

function formatTime(time: string): string {
  if (!time) return '-'
  const date = new Date(time)
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function onUploadSuccess() {
  ElMessage.success('上传成功')
  emit('upload-success')
}
</script>

<style scoped>
.file-table-container {
  margin-bottom: 20px;
}

.file-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.folder-icon {
  color: #e6a23c;
  font-size: 18px;
}

.file-icon {
  color: #909399;
  font-size: 18px;
}

.upload-dragger {
  margin-top: 15px;
}

.upload-dragger :deep(.el-upload-dragger) {
  padding: 20px;
  border-style: dashed;
}

.el-icon--upload {
  font-size: 40px;
  color: #c0c4cc;
}
</style>
