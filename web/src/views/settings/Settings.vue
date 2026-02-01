<template>
  <div class="settings-page">
    <div class="page-header">
      <h2 class="title">系统设置</h2>
    </div>

    <el-tabs v-model="activeTab" class="settings-tabs">
      <!-- 系统配置 -->
      <el-tab-pane label="系统配置" name="system">
        <div class="tab-content">
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
      </el-tab-pane>

      <!-- 账号安全 -->
      <el-tab-pane label="账号安全" name="security">
        <div class="tab-content">
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
      </el-tab-pane>

      <!-- API Token -->
      <el-tab-pane label="API Token" name="token">
        <div class="tab-content">
          <div class="token-header">
            <el-button size="small" @click="showApiTokenGuide">
              <el-icon><QuestionFilled /></el-icon>
              指南
            </el-button>
            <el-switch
              v-model="apiTokenForm.enabled"
              active-text="启用"
              inactive-text="禁用"
              @change="handleSaveApiToken"
            />
          </div>
          <el-form label-width="120px" v-loading="apiTokenLoading">
            <el-form-item label="Token">
              <el-input
                v-model="apiTokenForm.token"
                :type="showApiToken ? 'text' : 'password'"
                placeholder="点击生成按钮创建 Token"
                readonly
              >
                <template #append>
                  <el-button @click="showApiToken = !showApiToken">
                    <el-icon><View v-if="!showApiToken" /><Hide v-else /></el-icon>
                  </el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="handleGenerateToken">
                生成新 Token
              </el-button>
              <el-button @click="handleCopyToken" :disabled="!apiTokenForm.token">
                复制
              </el-button>
              <el-button type="success" @click="handleSaveApiToken" :loading="apiTokenSaving">
                保存
              </el-button>
            </el-form-item>
          </el-form>
        </div>
        <ApiTokenGuide ref="apiTokenGuideRef" :token="apiTokenForm.token" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, FormInstance, FormRules } from 'element-plus'
import { View, Hide, QuestionFilled } from '@element-plus/icons-vue'
import { getSettings, updateSettings, getApiTokenSettings, updateApiTokenSettings, generateApiToken } from '@/api/settings'
import ApiTokenGuide from '@/components/ApiTokenGuide.vue'
import { changePassword } from '@/api/auth'

const activeTab = ref('system')

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

// API Token 相关
const apiTokenLoading = ref(false)
const apiTokenSaving = ref(false)
const showApiToken = ref(false)
const apiTokenGuideRef = ref()
const apiTokenForm = reactive({
  token: '',
  enabled: true
})

function showApiTokenGuide() {
  apiTokenGuideRef.value?.show()
}

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

// API Token 相关函数
const loadApiTokenSettings = async () => {
  apiTokenLoading.value = true
  try {
    const res = await getApiTokenSettings()
    if (res.success) {
      apiTokenForm.enabled = res.enabled ?? true
      if (res.token) {
        apiTokenForm.token = res.token
      } else {
        // 没有已保存的 Token，自动生成
        const genRes = await generateApiToken()
        if (genRes.success) {
          apiTokenForm.token = genRes.token
        }
      }
    }
  } catch (e) {
    console.error('Failed to load API token settings:', e)
  } finally {
    apiTokenLoading.value = false
  }
}

const handleGenerateToken = async () => {
  try {
    const res = await generateApiToken()
    if (res.success) {
      apiTokenForm.token = res.token
      ElMessage.success('Token 已生成，请保存')
    }
  } catch (e) {
    ElMessage.error((e as Error).message || '生成失败')
  }
}

const handleSaveApiToken = async () => {
  apiTokenSaving.value = true
  try {
    const res = await updateApiTokenSettings({
      token: apiTokenForm.token,
      enabled: apiTokenForm.enabled
    })
    if (res.success) {
      ElMessage.success('保存成功')
    } else {
      ElMessage.error(res.message || '保存失败')
    }
  } catch (e) {
    ElMessage.error((e as Error).message || '保存失败')
  } finally {
    apiTokenSaving.value = false
  }
}

const handleCopyToken = async () => {
  if (!apiTokenForm.token) return
  try {
    await navigator.clipboard.writeText(apiTokenForm.token)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败')
  }
}

onMounted(() => {
  loadSettings()
  loadApiTokenSettings()
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

  .settings-tabs {
    background-color: #fff;
    border-radius: 8px;
    padding: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);

    :deep(.el-tabs__header) {
      margin-bottom: 20px;
    }
  }

  .tab-content {
    max-width: 500px;

    .el-form {
      .el-input,
      .el-input-number {
        width: 100%;
        max-width: 300px;
      }
    }
  }

  .token-header {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 20px;
    padding-bottom: 16px;
    border-bottom: 1px solid #ebeef5;
  }

  .form-tip {
    margin-left: 12px;
    color: #909399;
    font-size: 12px;
  }
}
</style>
