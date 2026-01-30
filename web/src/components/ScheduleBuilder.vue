<template>
  <div class="schedule-builder">
    <!-- 频率类型选择 -->
    <el-select v-model="scheduleType" placeholder="选择调度频率" style="width: 100%; margin-bottom: 12px;">
      <el-option
        v-for="option in typeOptions"
        :key="option.value"
        :label="option.label"
        :value="option.value"
      />
    </el-select>

    <!-- 每隔N分钟 -->
    <div v-if="scheduleType === 'interval_minutes'" class="schedule-options">
      <span class="option-label">间隔</span>
      <el-select v-model="intervalMinutes" style="width: 120px;">
        <el-option v-for="m in minuteOptions" :key="m" :label="`${m} 分钟`" :value="m" />
      </el-select>
    </div>

    <!-- 每隔N小时 -->
    <div v-if="scheduleType === 'interval_hours'" class="schedule-options">
      <span class="option-label">间隔</span>
      <el-select v-model="intervalHours" style="width: 120px;">
        <el-option v-for="h in 24" :key="h" :label="`${h} 小时`" :value="h" />
      </el-select>
    </div>

    <!-- 每天 -->
    <div v-if="scheduleType === 'daily'" class="schedule-options">
      <span class="option-label">执行时间</span>
      <el-time-picker
        v-model="dailyTime"
        format="HH:mm"
        value-format="HH:mm"
        placeholder="选择时间"
        style="width: 120px;"
      />
    </div>

    <!-- 每周 -->
    <div v-if="scheduleType === 'weekly'" class="schedule-options">
      <div class="weekday-selector">
        <el-checkbox-group v-model="weekDays">
          <el-checkbox v-for="day in weekDayOptions" :key="day.value" :value="day.value">
            {{ day.label }}
          </el-checkbox>
        </el-checkbox-group>
      </div>
      <div class="time-selector">
        <span class="option-label">执行时间</span>
        <el-time-picker
          v-model="weeklyTime"
          format="HH:mm"
          value-format="HH:mm"
          placeholder="选择时间"
          style="width: 120px;"
        />
      </div>
    </div>

    <!-- 每月 -->
    <div v-if="scheduleType === 'monthly'" class="schedule-options">
      <div class="date-selector">
        <span class="option-label">执行日期</span>
        <el-select v-model="monthDates" multiple placeholder="选择日期" style="width: 200px;">
          <el-option v-for="d in 31" :key="d" :label="`${d}号`" :value="d" />
        </el-select>
      </div>
      <div class="time-selector">
        <span class="option-label">执行时间</span>
        <el-time-picker
          v-model="monthlyTime"
          format="HH:mm"
          value-format="HH:mm"
          placeholder="选择时间"
          style="width: 120px;"
        />
      </div>
    </div>

    <!-- 预览文本 -->
    <div v-if="previewText" class="schedule-preview">
      <el-icon><Clock /></el-icon>
      <span>{{ previewText }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { Clock } from '@element-plus/icons-vue'

// 定义调度配置类型
interface ScheduleConfig {
  type: 'none' | 'interval_minutes' | 'interval_hours' | 'daily' | 'weekly' | 'monthly'
  interval?: number
  time?: string
  days?: number[]
  dates?: number[]
}

const props = defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

// 频率类型选项
const typeOptions = [
  { value: 'none', label: '不启用调度' },
  { value: 'interval_minutes', label: '每隔N分钟' },
  { value: 'interval_hours', label: '每隔N小时' },
  { value: 'daily', label: '每天' },
  { value: 'weekly', label: '每周' },
  { value: 'monthly', label: '每月' },
]

// 分钟选项
const minuteOptions = [5, 10, 15, 30, 60]

// 星期选项
const weekDayOptions = [
  { value: 1, label: '周一' },
  { value: 2, label: '周二' },
  { value: 3, label: '周三' },
  { value: 4, label: '周四' },
  { value: 5, label: '周五' },
  { value: 6, label: '周六' },
  { value: 0, label: '周日' },
]

// 内部状态
const scheduleType = ref<ScheduleConfig['type']>('none')
const intervalMinutes = ref(30)
const intervalHours = ref(1)
const dailyTime = ref('08:00')
const weekDays = ref<number[]>([1, 3, 5])
const weeklyTime = ref('09:00')
const monthDates = ref<number[]>([1, 15])
const monthlyTime = ref('10:00')

// 解析 modelValue
function parseModelValue(value: string): void {
  if (!value) {
    scheduleType.value = 'none'
    return
  }

  try {
    const config: ScheduleConfig = JSON.parse(value)
    scheduleType.value = config.type || 'none'

    switch (config.type) {
      case 'interval_minutes':
        intervalMinutes.value = config.interval || 30
        break
      case 'interval_hours':
        intervalHours.value = config.interval || 1
        break
      case 'daily':
        dailyTime.value = config.time || '08:00'
        break
      case 'weekly':
        weekDays.value = config.days || [1, 3, 5]
        weeklyTime.value = config.time || '09:00'
        break
      case 'monthly':
        monthDates.value = config.dates || [1, 15]
        monthlyTime.value = config.time || '10:00'
        break
    }
  } catch {
    // 如果解析失败，可能是旧的 Cron 格式，设为不启用
    scheduleType.value = 'none'
  }
}

// 生成输出 JSON
const outputJson = computed((): string => {
  let config: ScheduleConfig

  switch (scheduleType.value) {
    case 'interval_minutes':
      config = { type: 'interval_minutes', interval: intervalMinutes.value }
      break
    case 'interval_hours':
      config = { type: 'interval_hours', interval: intervalHours.value }
      break
    case 'daily':
      config = { type: 'daily', time: dailyTime.value }
      break
    case 'weekly':
      config = { type: 'weekly', days: [...weekDays.value].sort((a, b) => a - b), time: weeklyTime.value }
      break
    case 'monthly':
      config = { type: 'monthly', dates: [...monthDates.value].sort((a, b) => a - b), time: monthlyTime.value }
      break
    default:
      config = { type: 'none' }
  }

  return JSON.stringify(config)
})

// 预览文本
const previewText = computed((): string => {
  switch (scheduleType.value) {
    case 'interval_minutes':
      return `每隔 ${intervalMinutes.value} 分钟执行一次`
    case 'interval_hours':
      return `每隔 ${intervalHours.value} 小时执行一次`
    case 'daily':
      return `每天 ${dailyTime.value} 执行`
    case 'weekly': {
      if (weekDays.value.length === 0) return '请选择星期'
      const dayNames = weekDays.value
        .sort((a, b) => (a === 0 ? 7 : a) - (b === 0 ? 7 : b))
        .map(d => weekDayOptions.find(o => o.value === d)?.label || '')
        .join('、')
      return `每${dayNames} ${weeklyTime.value} 执行`
    }
    case 'monthly': {
      if (monthDates.value.length === 0) return '请选择日期'
      const dateNames = [...monthDates.value].sort((a, b) => a - b).map(d => `${d}号`).join('、')
      return `每月${dateNames} ${monthlyTime.value} 执行`
    }
    default:
      return ''
  }
})

// 监听内部状态变化，更新 modelValue
watch(
  [scheduleType, intervalMinutes, intervalHours, dailyTime, weekDays, weeklyTime, monthDates, monthlyTime],
  () => {
    const newValue = scheduleType.value === 'none' ? '' : outputJson.value
    if (newValue !== props.modelValue) {
      emit('update:modelValue', newValue)
    }
  },
  { deep: true }
)

// 监听 modelValue 变化，更新内部状态
watch(
  () => props.modelValue,
  (newValue) => {
    parseModelValue(newValue)
  }
)

// 初始化时解析 modelValue
onMounted(() => {
  parseModelValue(props.modelValue)
})
</script>

<style scoped>
.schedule-builder {
  width: 100%;
}

.schedule-options {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 12px;
}

.option-label {
  font-size: 13px;
  color: #606266;
  margin-right: 8px;
}

.weekday-selector {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.weekday-selector :deep(.el-checkbox) {
  margin-right: 8px;
}

.time-selector,
.date-selector {
  display: flex;
  align-items: center;
  margin-top: 8px;
}

.schedule-preview {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  background: #f0f9eb;
  border-radius: 4px;
  color: #67c23a;
  font-size: 13px;
}
</style>
