<template>
  <div class="pool-config-page">
    <div class="page-header">
      <h2 class="title">渲染并发配置</h2>
      <p class="description">配置页面渲染时的对象池和数据池大小，系统会根据并发数自动计算最优配置</p>
    </div>

    <el-row :gutter="20">
      <!-- 并发配置 -->
      <el-col :xs="24" :lg="12">
        <div class="card">
          <div class="card-header">
            <span class="title">并发等级</span>
          </div>
          <el-form label-width="100px" v-loading="loading">
            <el-form-item label="预设等级">
              <el-radio-group v-model="configForm.preset" @change="handlePresetChange">
                <el-radio-button
                  v-for="preset in presets"
                  :key="preset.key"
                  :value="preset.key"
                >
                  {{ preset.name }} ({{ preset.concurrency }})
                </el-radio-button>
                <el-radio-button value="custom">自定义</el-radio-button>
              </el-radio-group>
            </el-form-item>

            <el-form-item v-if="configForm.preset === 'custom'" label="自定义并发">
              <el-input-number
                v-model="configForm.concurrency"
                :min="10"
                :max="10000"
                :step="50"
              />
              <span class="form-tip">范围: 10 - 10000</span>
            </el-form-item>

            <!-- 高级选项 -->
            <el-collapse v-model="advancedOpen">
              <el-collapse-item title="高级选项" name="advanced">
                <el-form-item label="缓冲时间">
                  <el-input-number
                    v-model="configForm.buffer_seconds"
                    :min="5"
                    :max="30"
                  />
                  <span class="form-tip">秒 (5-30)</span>
                </el-form-item>
              </el-collapse-item>
            </el-collapse>

            <el-form-item style="margin-top: 20px">
              <el-button type="primary" :loading="saving" @click="handleSave">
                应用配置
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-col>

      <!-- 资源预估 -->
      <el-col :xs="24" :lg="12">
        <div class="card">
          <div class="card-header">
            <span class="title">资源预估</span>
          </div>

          <div class="estimate-section" v-loading="loading">
            <!-- 模板基准 -->
            <div class="estimate-group">
              <div class="group-title">
                模板基准
                <span class="source-template" v-if="templateStats.source_template">
                  (来自: {{ templateStats.source_template }})
                </span>
              </div>
              <div class="estimate-items">
                <div class="estimate-item">
                  <span class="label">单页关键词</span>
                  <span class="value">{{ templateStats.max_keyword }} 个</span>
                </div>
                <div class="estimate-item">
                  <span class="label">单页图片</span>
                  <span class="value">{{ templateStats.max_image }} 个</span>
                </div>
                <div class="estimate-item">
                  <span class="label">单页内链</span>
                  <span class="value">{{ templateStats.max_url }} 个</span>
                </div>
                <div class="estimate-item">
                  <span class="label">单页 CSS 类</span>
                  <span class="value">{{ templateStats.max_cls }} 个</span>
                </div>
              </div>
            </div>

            <!-- 池大小预估 -->
            <div class="estimate-group">
              <div class="group-title">池大小预估</div>
              <div class="estimate-items">
                <div class="estimate-item">
                  <span class="label">关键词池</span>
                  <span class="value">{{ formatNumber(poolSizes.KeywordPoolSize) }} 条</span>
                </div>
                <div class="estimate-item">
                  <span class="label">图片池</span>
                  <span class="value">{{ formatNumber(poolSizes.ImagePoolSize) }} 条</span>
                </div>
                <div class="estimate-item">
                  <span class="label">CSS 类名池</span>
                  <span class="value">{{ formatNumber(poolSizes.ClsPoolSize) }} 条</span>
                </div>
                <div class="estimate-item">
                  <span class="label">URL 池</span>
                  <span class="value">{{ formatNumber(poolSizes.URLPoolSize) }} 条</span>
                </div>
              </div>
            </div>

            <!-- 内存预估 -->
            <div class="estimate-group memory-group">
              <div class="group-title">内存预估</div>
              <div class="memory-total">
                <span class="label">预估总内存</span>
                <span class="value highlight">{{ memoryEstimate.human }}</span>
              </div>
            </div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 池运行状态监控 -->
    <div class="card pool-status-section">
      <div class="card-header">
        <span class="title">池运行状态</span>
        <div class="header-actions">
          <el-button size="small" @click="handleRefreshAll" :loading="poolStatusLoading">
            刷新全部
          </el-button>
        </div>
      </div>

      <el-row :gutter="20" v-loading="poolStatusLoading">
        <!-- 左侧: Go 对象池 -->
        <el-col :xs="24" :lg="12">
          <div class="pool-section">
            <div class="section-header">
              <span class="section-title">Go 对象池</span>
              <div class="section-actions">
                <el-button size="small" @click="handleWarmup" :loading="operationLoading">
                  预热
                </el-button>
                <el-button size="small" @click="handlePause" :loading="operationLoading">
                  暂停
                </el-button>
                <el-button size="small" @click="handleResume" :loading="operationLoading">
                  恢复
                </el-button>
              </div>
            </div>
            <div class="pool-cards">
              <PoolStatusCard
                v-for="pool in objectPoolStats"
                :key="pool.name"
                :pool="pool"
              />
              <el-empty v-if="objectPoolStats.length === 0" description="暂无数据" />
            </div>
          </div>
        </el-col>

        <!-- 右侧: Python 数据池 -->
        <el-col :xs="24" :lg="12">
          <div class="pool-section">
            <div class="section-header">
              <span class="section-title">Python 数据池</span>
              <div class="section-actions">
                <el-button size="small" @click="handleRefreshData" :loading="operationLoading">
                  刷新数据
                </el-button>
              </div>
            </div>
            <div class="pool-cards">
              <PoolStatusCard
                v-for="pool in dataPoolStats"
                :key="pool.name"
                :pool="pool"
              />
              <el-empty v-if="dataPoolStats.length === 0" description="暂无数据" />
            </div>
          </div>
        </el-col>
      </el-row>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import PoolStatusCard from '@/components/PoolStatusCard.vue'
import {
  getPoolConfig,
  updatePoolConfig,
  getPresets,
  formatMemorySize,
  getObjectPoolStats,
  getDataPoolStats,
  warmupPool,
  pausePool,
  resumePool,
  refreshDataPool,
  type PoolPreset,
  type PoolSizes,
  type TemplateStats,
  type MemoryEstimate,
  type PoolStats
} from '@/api/pool-config'

const loading = ref(false)
const saving = ref(false)
const advancedOpen = ref<string[]>([])

const presets = ref<PoolPreset[]>(getPresets())

const configForm = reactive({
  preset: 'medium',
  concurrency: 200,
  buffer_seconds: 10
})

const templateStats = reactive<TemplateStats>({
  max_cls: 0,
  max_url: 0,
  max_keyword_emoji: 0,
  max_keyword: 0,
  max_image: 0,
  max_content: 0,
  source_template: ''
})

const poolSizes = reactive<PoolSizes>({
  ClsPoolSize: 0,
  URLPoolSize: 0,
  KeywordEmojiPoolSize: 0,
  NumberPoolSize: 0,
  KeywordPoolSize: 0,
  ImagePoolSize: 0
})

const memoryEstimate = reactive<MemoryEstimate>({
  bytes: 0,
  human: '0 MB'
})

// 池状态监控
const poolStatusLoading = ref(false)
const objectPoolStats = ref<PoolStats[]>([])
const dataPoolStats = ref<PoolStats[]>([])
const operationLoading = ref(false)

const formatNumber = (num: number): string => {
  return num.toLocaleString()
}

const handlePresetChange = (preset: string) => {
  if (preset !== 'custom') {
    const selectedPreset = presets.value.find(p => p.key === preset)
    if (selectedPreset) {
      configForm.concurrency = selectedPreset.concurrency
    }
  }
  // 重新计算预估值
  calculateEstimate()
}

const calculateEstimate = () => {
  const concurrency = configForm.preset === 'custom'
    ? configForm.concurrency
    : (presets.value.find(p => p.key === configForm.preset)?.concurrency || 200)
  const buffer = configForm.buffer_seconds

  // 基于模板统计计算池大小
  poolSizes.KeywordPoolSize = templateStats.max_keyword * concurrency * buffer
  poolSizes.ImagePoolSize = templateStats.max_image * concurrency * buffer
  poolSizes.ClsPoolSize = templateStats.max_cls * concurrency * buffer
  poolSizes.URLPoolSize = templateStats.max_url * concurrency * buffer
  poolSizes.KeywordEmojiPoolSize = templateStats.max_keyword_emoji * concurrency * buffer

  // 计算内存预估（简化版）
  const keywordBytes = poolSizes.KeywordPoolSize * 50
  const imageBytes = poolSizes.ImagePoolSize * 150
  const clsBytes = poolSizes.ClsPoolSize * 20
  const urlBytes = poolSizes.URLPoolSize * 100
  const totalBytes = (keywordBytes + imageBytes + clsBytes + urlBytes) * 1.2 // 20% overhead

  memoryEstimate.bytes = totalBytes
  memoryEstimate.human = formatMemorySize(totalBytes)
}

const loadConfig = async () => {
  loading.value = true
  try {
    const res = await getPoolConfig()

    // 更新配置表单
    configForm.preset = res.config.preset
    configForm.concurrency = res.config.concurrency
    configForm.buffer_seconds = res.config.buffer_seconds

    // 更新模板统计
    Object.assign(templateStats, res.template_stats)

    // 更新池大小和内存预估
    Object.assign(poolSizes, res.calculated)
    Object.assign(memoryEstimate, res.memory)
  } catch (e) {
    ElMessage.error((e as Error).message || '加载配置失败')
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  saving.value = true
  try {
    const concurrency = configForm.preset === 'custom'
      ? configForm.concurrency
      : (presets.value.find(p => p.key === configForm.preset)?.concurrency || 200)

    await updatePoolConfig({
      preset: configForm.preset,
      concurrency: concurrency,
      buffer_seconds: configForm.buffer_seconds
    })

    ElMessage.success('配置已更新并生效')

    // 重新加载以获取最新计算结果
    await loadConfig()
  } catch (e) {
    ElMessage.error((e as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

// ========== 池状态监控方法 ==========

const loadPoolStatus = async () => {
  poolStatusLoading.value = true
  try {
    const [objectRes, dataRes] = await Promise.all([
      getObjectPoolStats(),
      getDataPoolStats()
    ])

    // 转换对象池统计为数组格式
    const objectPools: PoolStats[] = []
    if (objectRes.cls) {
      objectPools.push({ ...objectRes.cls, name: 'CSS 类名池' })
    }
    if (objectRes.url) {
      objectPools.push({ ...objectRes.url, name: 'URL 池' })
    }
    if (objectRes.keyword_emoji) {
      objectPools.push({ ...objectRes.keyword_emoji, name: '关键词表情池' })
    }
    objectPoolStats.value = objectPools

    // 数据池统计
    dataPoolStats.value = dataRes.pools || []
  } catch (e) {
    ElMessage.error((e as Error).message || '加载池状态失败')
  } finally {
    poolStatusLoading.value = false
  }
}

const handleWarmup = async () => {
  operationLoading.value = true
  try {
    await warmupPool(0.5)
    ElMessage.success('预热已启动')
    await loadPoolStatus()
  } catch (e) {
    ElMessage.error((e as Error).message || '预热失败')
  } finally {
    operationLoading.value = false
  }
}

const handlePause = async () => {
  operationLoading.value = true
  try {
    await pausePool()
    ElMessage.success('已暂停补充')
    await loadPoolStatus()
  } catch (e) {
    ElMessage.error((e as Error).message || '暂停失败')
  } finally {
    operationLoading.value = false
  }
}

const handleResume = async () => {
  operationLoading.value = true
  try {
    await resumePool()
    ElMessage.success('已恢复补充')
    await loadPoolStatus()
  } catch (e) {
    ElMessage.error((e as Error).message || '恢复失败')
  } finally {
    operationLoading.value = false
  }
}

const handleRefreshData = async () => {
  operationLoading.value = true
  try {
    await refreshDataPool('all')
    ElMessage.success('数据刷新已启动')
    await loadPoolStatus()
  } catch (e) {
    ElMessage.error((e as Error).message || '刷新失败')
  } finally {
    operationLoading.value = false
  }
}

const handleRefreshAll = async () => {
  await loadPoolStatus()
  ElMessage.success('状态已刷新')
}

onMounted(() => {
  loadConfig()
  loadPoolStatus()
})
</script>

<style lang="scss" scoped>
.pool-config-page {
  .page-header {
    margin-bottom: 20px;

    .title {
      font-size: 20px;
      font-weight: 600;
      color: #303133;
      margin-bottom: 8px;
    }

    .description {
      color: #909399;
      font-size: 14px;
      margin: 0;
    }
  }

  .card {
    background-color: #fff;
    border-radius: 8px;
    padding: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);
    margin-bottom: 20px;

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 20px;
      padding-bottom: 12px;
      border-bottom: 1px solid #ebeef5;

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

  .estimate-section {
    .estimate-group {
      margin-bottom: 24px;

      &:last-child {
        margin-bottom: 0;
      }

      .group-title {
        font-size: 14px;
        font-weight: 500;
        color: #606266;
        margin-bottom: 12px;

        .source-template {
          font-weight: normal;
          color: #909399;
          font-size: 12px;
        }
      }

      .estimate-items {
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: 12px;

        .estimate-item {
          display: flex;
          justify-content: space-between;
          padding: 8px 12px;
          background: #f5f7fa;
          border-radius: 4px;

          .label {
            color: #606266;
            font-size: 13px;
          }

          .value {
            color: #303133;
            font-weight: 500;
            font-size: 13px;
          }
        }
      }
    }

    .memory-group {
      .memory-total {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 16px;
        background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        border-radius: 8px;
        color: #fff;

        .label {
          font-size: 14px;
        }

        .value {
          font-size: 24px;
          font-weight: 600;

          &.highlight {
            text-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
          }
        }
      }
    }
  }

  :deep(.el-collapse) {
    border: none;

    .el-collapse-item__header {
      border: none;
      background: transparent;
      font-size: 14px;
      color: #606266;
    }

    .el-collapse-item__wrap {
      border: none;
      background: transparent;
    }

    .el-collapse-item__content {
      padding-bottom: 0;
    }
  }

  :deep(.el-radio-group) {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .pool-status-section {
    margin-top: 20px;

    .header-actions {
      display: flex;
      gap: 8px;
    }

    .pool-section {
      .section-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 16px;
        padding-bottom: 12px;
        border-bottom: 1px solid #ebeef5;

        .section-title {
          font-size: 15px;
          font-weight: 600;
          color: #303133;
        }

        .section-actions {
          display: flex;
          gap: 8px;
        }
      }

      .pool-cards {
        min-height: 200px;
      }
    }
  }
}
</style>
