<template>
  <div class="log-viewer">
    <!-- 过滤器 -->
    <div class="log-filters" v-if="showFilters">
      <el-select v-model="levelFilter" placeholder="日志级别" clearable style="width: 120px">
        <el-option label="全部" value="" />
        <el-option label="DEBUG" value="DEBUG" />
        <el-option label="INFO" value="INFO" />
        <el-option label="WARNING" value="WARNING" />
        <el-option label="ERROR" value="ERROR" />
      </el-select>
      <el-input
        v-model="searchText"
        placeholder="搜索日志内容..."
        clearable
        style="width: 200px; margin-left: 8px"
      />
      <el-switch
        v-model="autoScroll"
        active-text="自动滚动"
        style="margin-left: 16px"
      />
      <el-button
        type="text"
        style="margin-left: auto"
        @click="clearLogs"
      >
        清空
      </el-button>
    </div>

    <!-- 日志内容区域 -->
    <div class="log-content" ref="logContainer">
      <div
        v-for="(log, index) in filteredLogs"
        :key="index"
        :class="['log-line', `log-${log.level.toLowerCase()}`]"
      >
        <span class="log-time">{{ log.time }}</span>
        <span :class="['log-level', `level-${log.level.toLowerCase()}`]">{{ log.level }}</span>
        <span class="log-message">{{ log.message }}</span>
      </div>
      <div v-if="filteredLogs.length === 0" class="log-empty">
        暂无日志
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

interface LogItem {
  time: string
  level: string
  message: string
}

const props = withDefaults(defineProps<{
  logs?: LogItem[]
  maxLines?: number
  showFilters?: boolean
}>(), {
  logs: () => [],
  maxLines: 1000,
  showFilters: true
})

const emit = defineEmits<{
  (e: 'clear'): void
}>()

const levelFilter = ref('')
const searchText = ref('')
const autoScroll = ref(true)
const logContainer = ref<HTMLElement | null>(null)

// 计算过滤后的日志
const filteredLogs = computed(() => {
  let result = props.logs

  if (levelFilter.value) {
    result = result.filter(log => log.level === levelFilter.value)
  }

  if (searchText.value) {
    const search = searchText.value.toLowerCase()
    result = result.filter(log => log.message.toLowerCase().includes(search))
  }

  // 限制最大行数
  if (result.length > props.maxLines) {
    result = result.slice(-props.maxLines)
  }

  return result
})

// 监听日志变化，自动滚动
watch(
  () => props.logs.length,
  async () => {
    if (autoScroll.value) {
      await nextTick()
      scrollToBottom()
    }
  }
)

function scrollToBottom() {
  if (logContainer.value) {
    logContainer.value.scrollTop = logContainer.value.scrollHeight
  }
}

function clearLogs() {
  emit('clear')
}

// 暴露方法
defineExpose({
  scrollToBottom
})
</script>

<style scoped>
.log-viewer {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 200px;
}

.log-filters {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  background: #fafafa;
  border-bottom: 1px solid #e4e7ed;
}

.log-content {
  flex: 1;
  overflow-y: auto;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  line-height: 1.6;
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 8px;
}

.log-line {
  padding: 2px 8px;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.05);
}

.log-time {
  color: #6a9955;
  margin-right: 8px;
}

.log-level {
  display: inline-block;
  width: 70px;
  font-weight: bold;
  margin-right: 8px;
}

.level-debug { color: #808080; }
.level-info { color: #4fc1ff; }
.level-warning { color: #dcdcaa; }
.level-error { color: #f14c4c; }
.level-print { color: #ce9178; }
.level-request { color: #c586c0; }
.level-item { color: #4ec9b0; }

.log-message {
  color: #d4d4d4;
}

.log-debug .log-message { color: #808080; }
.log-error .log-message { color: #f14c4c; }

.log-empty {
  color: #808080;
  text-align: center;
  padding: 40px;
}
</style>
