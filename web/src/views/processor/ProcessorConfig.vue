<template>
  <div class="processor-config">
    <el-form
      :model="configForm"
      label-width="140px"
      v-loading="configLoading"
    >
      <el-form-item label="启用数据加工">
        <el-switch v-model="configForm.enabled" />
        <span class="form-tip">关闭后 Worker 启动时不会自动处理</span>
      </el-form-item>
      <el-form-item label="并发 Worker 数">
        <el-input-number
          v-model="configForm.concurrency"
          :min="1"
          :max="10"
          :step="1"
        />
        <span class="form-tip">同时处理文章的协程数量</span>
      </el-form-item>
      <el-form-item label="最大重试次数">
        <el-input-number
          v-model="configForm.retry_max"
          :min="0"
          :max="10"
          :step="1"
        />
        <span class="form-tip">超过后放入死信队列</span>
      </el-form-item>
      <el-form-item label="段落最小长度">
        <el-input-number
          v-model="configForm.min_paragraph_length"
          :min="1"
          :max="500"
          :step="10"
        />
        <span class="form-tip">字符，过短的段落将被过滤</span>
      </el-form-item>
      <el-form-item label="批量写入大小">
        <el-input-number
          v-model="configForm.batch_size"
          :min="1"
          :max="200"
          :step="10"
        />
        <span class="form-tip">每批写入数据库的记录数</span>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="saveConfigLoading" @click="handleSaveConfig">
          保存配置
        </el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getProcessorConfig,
  updateProcessorConfig,
  type ProcessorConfig
} from '@/api/processor'

const configLoading = ref(false)
const saveConfigLoading = ref(false)

const configForm = reactive<ProcessorConfig>({
  enabled: true,
  concurrency: 3,
  retry_max: 3,
  min_paragraph_length: 20,
  batch_size: 50
})

const loadConfig = async () => {
  configLoading.value = true
  try {
    const data = await getProcessorConfig()
    Object.assign(configForm, data)
  } catch (e) {
    console.error('加载配置失败:', e)
  } finally {
    configLoading.value = false
  }
}

const handleSaveConfig = async () => {
  saveConfigLoading.value = true
  try {
    await updateProcessorConfig(configForm)
    ElMessage.success('配置已保存')
  } catch (e) {
    ElMessage.error((e as Error).message || '保存失败')
  } finally {
    saveConfigLoading.value = false
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<style lang="scss" scoped>
.processor-config {
  max-width: 600px;
  padding: 20px;

  .form-tip {
    margin-left: 12px;
    color: #909399;
    font-size: 12px;
  }

  .el-input-number {
    width: 180px;
  }
}
</style>
