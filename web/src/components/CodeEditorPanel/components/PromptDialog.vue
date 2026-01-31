<template>
  <Teleport to="body">
    <Transition name="ce-modal">
      <div v-if="visible" class="ce-modal-overlay" @click.self="handleCancel">
        <div class="ce-modal-container">
          <div class="ce-modal-header">
            <span class="ce-modal-title">{{ title }}</span>
            <button class="ce-modal-close" type="button" @click="handleCancel">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 8.707l3.646 3.647.708-.707L8.707 8l3.647-3.646-.707-.708L8 7.293 4.354 3.646l-.708.708L7.293 8l-3.647 3.646.708.708L8 8.707z"/>
              </svg>
            </button>
          </div>
          <div class="ce-modal-body">
            <!-- 确认模式：显示消息 -->
            <div v-if="mode === 'confirm'" class="ce-confirm-content">
              <svg class="ce-confirm-icon" :class="type" width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
              </svg>
              <span class="ce-confirm-message">{{ message }}</span>
            </div>
            <!-- 输入模式：显示输入框 -->
            <div v-else>
              <input
                ref="inputRef"
                v-model="inputValue"
                class="ce-modal-input"
                :placeholder="placeholder"
                @keyup.enter="handleConfirm"
              />
              <p v-if="errorMsg" class="ce-modal-error">{{ errorMsg }}</p>
            </div>
          </div>
          <div class="ce-modal-footer">
            <button class="ce-modal-btn" type="button" @click="handleCancel">取消</button>
            <button
              :class="['ce-modal-btn', type === 'danger' ? 'ce-modal-btn-danger' : 'ce-modal-btn-primary']"
              type="button"
              @click="handleConfirm"
            >
              确定
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

const props = withDefaults(defineProps<{
  visible: boolean
  title: string
  mode?: 'input' | 'confirm'
  type?: 'warning' | 'danger' | 'info'
  message?: string
  placeholder?: string
  defaultValue?: string
  validator?: (value: string) => string | null // 返回错误消息或 null
}>(), {
  mode: 'input',
  type: 'info',
  message: '',
  placeholder: '',
  defaultValue: ''
})

const emit = defineEmits<{
  (e: 'confirm', value?: string): void
  (e: 'cancel'): void
}>()

const inputRef = ref<HTMLInputElement>()
const inputValue = ref('')
const errorMsg = ref('')

watch(() => props.visible, (val) => {
  if (val) {
    inputValue.value = props.defaultValue
    errorMsg.value = ''
    if (props.mode === 'input') {
      nextTick(() => {
        inputRef.value?.focus()
        // 如果有默认值，选中文件名部分（不含扩展名）
        if (props.defaultValue) {
          const dotIndex = props.defaultValue.lastIndexOf('.')
          if (dotIndex > 0) {
            inputRef.value?.setSelectionRange(0, dotIndex)
          } else {
            inputRef.value?.select()
          }
        }
      })
    }
  }
})

function validate(): boolean {
  if (props.mode !== 'input') return true

  const value = inputValue.value.trim()
  if (!value) {
    errorMsg.value = '请输入内容'
    return false
  }

  // 自定义验证
  if (props.validator) {
    const error = props.validator(value)
    if (error) {
      errorMsg.value = error
      return false
    }
  }

  // 默认文件名验证
  if (/[\\/:*?"<>|]/.test(value)) {
    errorMsg.value = '名称包含非法字符'
    return false
  }

  errorMsg.value = ''
  return true
}

function handleConfirm() {
  if (!validate()) return

  if (props.mode === 'input') {
    emit('confirm', inputValue.value.trim())
  } else {
    emit('confirm')
  }
}

function handleCancel() {
  emit('cancel')
}
</script>

<style>
/* 统一弹窗样式 */
.ce-modal-overlay {
  position: fixed !important;
  top: 0 !important;
  left: 0 !important;
  right: 0 !important;
  bottom: 0 !important;
  background: rgba(0, 0, 0, 0.6) !important;
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  z-index: 10000 !important;
}

.ce-modal-container {
  background: #252526 !important;
  border: 1px solid #3c3c3c !important;
  border-radius: 6px !important;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5) !important;
  min-width: 340px !important;
  max-width: 90vw !important;
}

.ce-modal-header {
  display: flex !important;
  justify-content: space-between !important;
  align-items: center !important;
  padding: 12px 16px !important;
  border-bottom: 1px solid #3c3c3c !important;
}

.ce-modal-title {
  color: #cccccc !important;
  font-size: 14px !important;
  font-weight: 500 !important;
}

.ce-modal-close {
  background: none !important;
  border: none !important;
  color: #858585 !important;
  cursor: pointer !important;
  padding: 4px !important;
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  border-radius: 4px !important;
  transition: all 0.2s !important;
}

.ce-modal-close:hover {
  color: #cccccc !important;
  background: rgba(255, 255, 255, 0.1) !important;
}

.ce-modal-body {
  padding: 16px !important;
}

.ce-modal-footer {
  display: flex !important;
  justify-content: flex-end !important;
  gap: 8px !important;
  padding: 12px 16px !important;
  border-top: 1px solid #3c3c3c !important;
}

/* 输入框 */
.ce-modal-input {
  width: 100% !important;
  padding: 8px 12px !important;
  background: #3c3c3c !important;
  border: 1px solid #5a5a5a !important;
  border-radius: 4px !important;
  color: #cccccc !important;
  font-size: 14px !important;
  outline: none !important;
  box-sizing: border-box !important;
  transition: border-color 0.2s !important;
}

.ce-modal-input:focus {
  border-color: #007acc !important;
}

.ce-modal-input::placeholder {
  color: #6e6e6e !important;
}

.ce-modal-error {
  margin: 8px 0 0 !important;
  color: #f48771 !important;
  font-size: 12px !important;
}

/* 确认消息 */
.ce-confirm-content {
  display: flex !important;
  align-items: flex-start !important;
  gap: 12px !important;
}

.ce-confirm-icon {
  flex-shrink: 0 !important;
  margin-top: 2px !important;
}

.ce-confirm-icon.warning {
  color: #e6a23c !important;
}

.ce-confirm-icon.danger {
  color: #f56c6c !important;
}

.ce-confirm-icon.info {
  color: #409eff !important;
}

.ce-confirm-message {
  color: #cccccc !important;
  font-size: 14px !important;
  line-height: 1.5 !important;
}

/* 按钮 */
.ce-modal-btn {
  padding: 6px 14px !important;
  border: 1px solid #5a5a5a !important;
  border-radius: 4px !important;
  background: #3c3c3c !important;
  color: #cccccc !important;
  font-size: 13px !important;
  cursor: pointer !important;
  transition: all 0.2s !important;
}

.ce-modal-btn:hover {
  background: #4a4a4a !important;
}

.ce-modal-btn-primary {
  background: #0e639c !important;
  border-color: #0e639c !important;
  color: #ffffff !important;
}

.ce-modal-btn-primary:hover {
  background: #1177bb !important;
}

.ce-modal-btn-danger {
  background: #c45656 !important;
  border-color: #c45656 !important;
  color: #ffffff !important;
}

.ce-modal-btn-danger:hover {
  background: #d35a5a !important;
}

/* 过渡动画 */
.ce-modal-enter-active,
.ce-modal-leave-active {
  transition: opacity 0.2s ease;
}

.ce-modal-enter-active .ce-modal-container,
.ce-modal-leave-active .ce-modal-container {
  transition: transform 0.2s ease;
}

.ce-modal-enter-from,
.ce-modal-leave-to {
  opacity: 0;
}

.ce-modal-enter-from .ce-modal-container,
.ce-modal-leave-to .ce-modal-container {
  transform: scale(0.95);
}
</style>
