<template>
  <el-dialog v-model="visible" title="移动到" width="400px">
    <p style="margin-bottom: 10px">选择目标目录：</p>
    <el-tree
      :data="treeData"
      :props="{ label: 'name', children: 'children' }"
      node-key="path"
      highlight-current
      :expand-on-click-node="false"
      @node-click="selectDir"
      default-expand-all
      v-loading="loading"
    >
      <template #default="{ data }">
        <span class="tree-node">
          <el-icon><Folder /></el-icon>
          <span>{{ data.name }}</span>
        </span>
      </template>
    </el-tree>

    <p v-if="selectedPath" style="margin-top: 10px; color: #409eff">
      当前选择：{{ selectedPath }}
    </p>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="confirm" :disabled="!selectedPath">
        确定移动
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Folder } from '@element-plus/icons-vue'
import { getFileTree, type TreeNode } from '@/api/worker'

const props = defineProps<{
  modelValue: boolean
  filePath: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm', targetDir: string): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const treeData = ref<TreeNode[]>([])
const selectedPath = ref('')
const loading = ref(false)

watch(() => props.modelValue, async (val) => {
  if (val) {
    selectedPath.value = ''
    loading.value = true
    try {
      const tree = await getFileTree()
      treeData.value = [tree]
    } catch (err) {
      treeData.value = []
      ElMessage.error('加载目录结构失败')
    } finally {
      loading.value = false
    }
  }
})

function selectDir(data: TreeNode) {
  selectedPath.value = data.path
}

function confirm() {
  if (selectedPath.value) {
    emit('confirm', selectedPath.value)
    visible.value = false
  }
}
</script>

<style scoped>
.tree-node {
  display: flex;
  align-items: center;
  gap: 5px;
}
</style>
