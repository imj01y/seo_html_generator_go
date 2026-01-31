<template>
  <el-dialog
    :model-value="modelValue"
    title="重命名"
    width="400px"
    @update:model-value="$emit('update:modelValue', $event)"
    @closed="handleClosed"
    @open="handleOpen"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
      <el-form-item label="新名称" prop="newName">
        <el-input
          ref="inputRef"
          v-model="form.newName"
          placeholder="请输入新名称"
          @keyup.enter="handleConfirm"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" @click="handleConfirm">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, nextTick } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'

const props = defineProps<{
  modelValue: boolean
  currentName: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm', newName: string): void
}>()

const formRef = ref<FormInstance>()
const inputRef = ref<HTMLInputElement>()
const form = reactive({ newName: '' })

const rules: FormRules = {
  newName: [
    { required: true, message: '请输入新名称', trigger: 'blur' },
    { pattern: /^[^\\/:*?"<>|]+$/, message: '名称包含非法字符', trigger: 'blur' }
  ]
}

function handleOpen() {
  form.newName = props.currentName
  nextTick(() => {
    inputRef.value?.focus()
    // 选中文件名（不含扩展名）
    const dotIndex = props.currentName.lastIndexOf('.')
    if (dotIndex > 0) {
      inputRef.value?.setSelectionRange(0, dotIndex)
    } else {
      inputRef.value?.select()
    }
  })
}

function handleClosed() {
  formRef.value?.resetFields()
}

async function handleConfirm() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
    emit('confirm', form.newName)
    emit('update:modelValue', false)
  } catch {
    // 验证失败
  }
}
</script>
