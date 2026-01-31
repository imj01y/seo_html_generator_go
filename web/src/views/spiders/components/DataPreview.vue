<template>
  <div class="data-preview">
    <div v-if="items.length === 0" class="empty-state">
      暂无数据
    </div>
    <div v-else class="items-list">
      <div
        v-for="(item, index) in items"
        :key="index"
        class="item-card"
        @click="showDetail(item)"
      >
        <div class="item-title">{{ item.title || '(无标题)' }}</div>
        <div class="item-content">{{ truncateText(item.content, 150) }}</div>
      </div>
    </div>

    <!-- 详情弹窗 -->
    <el-dialog v-model="detailVisible" title="数据详情" width="75%" top="5vh">
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
        <el-button @click="detailVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  items: Record<string, any>[]
}>()

const detailVisible = ref(false)
const currentItem = ref<Record<string, any> | null>(null)

function showDetail(item: Record<string, any>) {
  currentItem.value = item
  detailVisible.value = true
}

function truncateText(text: string, maxLength: number) {
  if (!text) return ''
  const plainText = text.replace(/<[^>]+>/g, '')
  if (plainText.length <= maxLength) return plainText
  return plainText.substring(0, maxLength) + '...'
}
</script>

<style scoped>
.data-preview {
  height: 100%;
  overflow-y: auto;
}

.empty-state {
  color: #6e6e6e;
  text-align: center;
  padding: 20px;
}

.items-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.item-card {
  padding: 12px;
  background: #2d2d2d;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.2s;
}

.item-card:hover {
  background: #3c3c3c;
}

.item-title {
  font-weight: 500;
  color: #cccccc;
  margin-bottom: 6px;
}

.item-content {
  font-size: 12px;
  color: #808080;
  line-height: 1.5;
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
</style>
