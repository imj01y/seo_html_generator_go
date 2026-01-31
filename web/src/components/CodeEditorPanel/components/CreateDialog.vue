<template>
  <el-dialog
    :model-value="modelValue"
    :title="type === 'file' ? '新建文件' : '新建目录'"
    width="400px"
    @update:model-value="$emit('update:modelValue', $event)"
    @closed="handleClosed"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="60px">
      <el-form-item label="名称" prop="name">
        <el-input
          v-model="form.name"
          :placeholder="type === 'file' ? '请输入文件名' : '请输入目录名'"
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
import { ref, reactive, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'

const props = defineProps<{
  modelValue: boolean
  type: 'file' | 'dir'
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm', name: string): void
}>()

const formRef = ref<FormInstance>()
const form = reactive({ name: '' })

const rules: FormRules = {
  name: [
    { required: true, message: '请输入名称', trigger: 'blur' },
    { pattern: /^[^\\/:*?"<>|]+$/, message: '名称包含非法字符', trigger: 'blur' }
  ]
}

watch(() => props.modelValue, (val) => {
  if (val) {
    form.name = ''
  }
})

function handleClosed() {
  formRef.value?.resetFields()
}

async function handleConfirm() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
    emit('confirm', form.name)
    emit('update:modelValue', false)
  } catch {
    // 验证失败
  }
}
</script>
