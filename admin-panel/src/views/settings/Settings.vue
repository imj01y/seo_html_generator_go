<template>
  <div class="settings-page">
    <div class="page-header">
      <h2 class="title">系统设置</h2>
    </div>

    <el-row :gutter="20">
      <!-- 系统设置 -->
      <el-col :xs="24" :lg="12">
        <div class="card">
          <div class="card-header">
            <span class="title">系统配置</span>
          </div>
          <el-form
            ref="settingsFormRef"
            :model="settingsForm"
            label-width="120px"
            v-loading="settingsLoading"
          >
            <el-form-item label="系统名称">
              <el-input v-model="settingsForm.site_name" placeholder="SEO HTML Generator" />
            </el-form-item>
            <el-form-item label="编码混合比例">
              <el-input-number
                v-model="settingsForm.encoding_ratio"
                :min="0"
                :max="1"
                :step="0.1"
                :precision="1"
              />
              <span class="form-tip">十六进制编码占比（0-1）</span>
            </el-form-item>
            <el-form-item label="日志保留天数">
              <el-input-number
                v-model="settingsForm.log_retention_days"
                :min="1"
                :max="365"
              />
              <span class="form-tip">天</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saveLoading" @click="handleSaveSettings">
                保存设置
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-col>

      <!-- 账号安全 -->
      <el-col :xs="24" :lg="12">
        <div class="card">
          <div class="card-header">
            <span class="title">账号安全</span>
          </div>
          <el-form
            ref="passwordFormRef"
            :model="passwordForm"
            :rules="passwordRules"
            label-width="120px"
          >
            <el-form-item label="当前密码" prop="old_password">
              <el-input
                v-model="passwordForm.old_password"
                type="password"
                placeholder="请输入当前密码"
                show-password
              />
            </el-form-item>
            <el-form-item label="新密码" prop="new_password">
              <el-input
                v-model="passwordForm.new_password"
                type="password"
                placeholder="请输入新密码（至少6位）"
                show-password
              />
            </el-form-item>
            <el-form-item label="确认新密码" prop="confirm_password">
              <el-input
                v-model="passwordForm.confirm_password"
                type="password"
                placeholder="请再次输入新密码"
                show-password
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="passwordLoading" @click="handleChangePassword">
                修改密码
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, FormInstance, FormRules } from 'element-plus'
import { getSettings, updateSettings } from '@/api/settings'
import { changePassword } from '@/api/auth'

const settingsLoading = ref(false)
const saveLoading = ref(false)
const passwordLoading = ref(false)
const passwordFormRef = ref<FormInstance>()

const settingsForm = reactive({
  site_name: 'SEO HTML Generator',
  encoding_ratio: 0.5,
  log_retention_days: 30
})

const passwordForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

const validateConfirmPassword = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
  if (value !== passwordForm.new_password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const passwordRules: FormRules = {
  old_password: [
    { required: true, message: '请输入当前密码', trigger: 'blur' }
  ],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度至少6位', trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

const loadSettings = async () => {
  settingsLoading.value = true
  try {
    const settings = await getSettings()
    settings.forEach(s => {
      if (s.key in settingsForm) {
        const value = s.value
        if (s.key === 'encoding_ratio') {
          settingsForm[s.key] = parseFloat(value) || 0.5
        } else if (s.key === 'log_retention_days') {
          (settingsForm as Record<string, string | number>)[s.key] = parseInt(value) || 0
        } else {
          (settingsForm as Record<string, string | number>)[s.key] = value
        }
      }
    })
  } finally {
    settingsLoading.value = false
  }
}

const handleSaveSettings = async () => {
  saveLoading.value = true
  try {
    await updateSettings({
      site_name: settingsForm.site_name,
      encoding_ratio: String(settingsForm.encoding_ratio),
      log_retention_days: String(settingsForm.log_retention_days)
    })
    ElMessage.success('保存成功')
  } catch (e) {
    ElMessage.warning((e as Error).message || '保存失败')
  } finally {
    saveLoading.value = false
  }
}

const handleChangePassword = async () => {
  await passwordFormRef.value?.validate()

  passwordLoading.value = true
  try {
    await changePassword({
      old_password: passwordForm.old_password,
      new_password: passwordForm.new_password
    })
    ElMessage.success('密码修改成功，请使用新密码重新登录')
    // 清空表单
    passwordForm.old_password = ''
    passwordForm.new_password = ''
    passwordForm.confirm_password = ''
  } catch (e) {
    ElMessage.error((e as Error).message || '密码修改失败')
  } finally {
    passwordLoading.value = false
  }
}

onMounted(() => {
  loadSettings()
})
</script>

<style lang="scss" scoped>
.settings-page {
  .page-header {
    margin-bottom: 20px;

    .title {
      font-size: 20px;
      font-weight: 600;
      color: #303133;
    }
  }

  .card {
    background-color: #fff;
    border-radius: 8px;
    padding: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 20px;

      .title {
        font-size: 16px;
        font-weight: 600;
        color: #303133;
      }
    }
  }

  .form-tip {
    margin-left: 12px;
    color: #909399;
    font-size: 12px;
  }
}
</style>
