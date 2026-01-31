<template>
  <el-dialog v-model="visible" title="重命名" width="400px">
    <el-form @submit.prevent="confirm">
      <el-form-item label="新名称">
        <el-input v-model="newName" ref="inputRef" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="confirm" :disabled="!newName.trim()">
        确定
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps<{
  modelValue: boolean
  currentName: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm', newName: string): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const newName = ref('')
const inputRef = ref()

watch(() => props.modelValue, (val) => {
  if (val) {
    newName.value = props.currentName
    nextTick(() => {
      inputRef.value?.focus()
      inputRef.value?.select()
    })
  }
})

function confirm() {
  if (newName.value.trim() && newName.value !== props.currentName) {
    emit('confirm', newName.value.trim())
    visible.value = false
  }
}
</script>
