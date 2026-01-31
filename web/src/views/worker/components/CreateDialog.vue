<template>
  <el-dialog
    v-model="visible"
    :title="type === 'file' ? '新建文件' : '新建目录'"
    width="400px"
  >
    <el-form @submit.prevent="confirm">
      <el-form-item :label="type === 'file' ? '文件名' : '目录名'">
        <el-input v-model="name" :placeholder="placeholder" ref="inputRef" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="confirm" :disabled="!name.trim()">
        创建
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps<{
  modelValue: boolean
  type: 'file' | 'dir'
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm', name: string): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const name = ref('')
const inputRef = ref()

const placeholder = computed(() => {
  return props.type === 'file' ? '例如: processor.py' : '例如: utils'
})

watch(() => props.modelValue, (val) => {
  if (val) {
    name.value = ''
    nextTick(() => inputRef.value?.focus())
  }
})

function confirm() {
  if (name.value.trim()) {
    emit('confirm', name.value.trim())
    visible.value = false
  }
}
</script>
