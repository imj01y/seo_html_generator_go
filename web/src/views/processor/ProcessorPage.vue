<template>
  <div class="processor-page">
    <el-tabs v-model="activeTab" class="processor-tabs">
      <el-tab-pane label="监控面板" name="monitor">
        <ProcessorManage />
      </el-tab-pane>
      <el-tab-pane label="代码编辑" name="code">
        <ContentWorkerEditor />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ProcessorManage from './ProcessorManage.vue'
import ContentWorkerEditor from '@/views/content-worker/ContentWorkerEditor.vue'

const activeTab = ref('monitor')
</script>

<style lang="scss" scoped>
.processor-page {
  height: 100%;

  .processor-tabs {
    height: 100%;

    :deep(.el-tabs__content) {
      height: calc(100% - 40px);
      overflow: auto;
    }

    :deep(.el-tab-pane) {
      height: 100%;
    }
  }
}
</style>
