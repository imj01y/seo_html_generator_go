import request from '@/utils/request'

// 数据加工配置
export interface ProcessorConfig {
  enabled: boolean
  concurrency: number
  retry_max: number
  min_paragraph_length: number
  batch_size: number
}

// 数据加工状态
export interface ProcessorStatus {
  running: boolean
  workers: number
  queue_pending: number
  queue_retry: number
  queue_dead: number
  processed_total: number
  processed_today: number
  speed: number
  last_error: string | null
}

// 数据加工统计
export interface ProcessorStats {
  total_processed: number
  total_failed: number
  total_retried: number
  success_rate: number
  avg_processing_ms: number
  titles_generated: number
  contents_generated: number
}

// 获取配置
export const getProcessorConfig = async (): Promise<ProcessorConfig> => {
  const res: { success: boolean; data: ProcessorConfig; message?: string } = await request.get('/processor/config')
  if (!res.success) {
    throw new Error(res.message || '获取配置失败')
  }
  return res.data
}

// 更新配置
export const updateProcessorConfig = async (config: ProcessorConfig): Promise<ProcessorConfig> => {
  const res: { success: boolean; data: ProcessorConfig; message?: string } = await request.put('/processor/config', config)
  if (!res.success) {
    throw new Error(res.message || '更新配置失败')
  }
  return res.data
}

// 获取运行状态
export const getProcessorStatus = async (): Promise<ProcessorStatus> => {
  const res: { success: boolean; data: ProcessorStatus; message?: string } = await request.get('/processor/status')
  if (!res.success) {
    throw new Error(res.message || '获取状态失败')
  }
  return res.data
}

// 启动数据加工
export const startProcessor = async (): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.post('/processor/start')
  if (!res.success) {
    throw new Error(res.message || '启动失败')
  }
}

// 停止数据加工
export const stopProcessor = async (): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.post('/processor/stop')
  if (!res.success) {
    throw new Error(res.message || '停止失败')
  }
}

// 重试所有失败任务
export const retryAllFailed = async (): Promise<{ count: number }> => {
  const res: { success: boolean; count: number; message?: string } = await request.post('/processor/retry-all')
  if (!res.success) {
    throw new Error(res.message || '重试失败')
  }
  return { count: res.count }
}

// 清空死信队列
export const clearDeadQueue = async (): Promise<{ count: number }> => {
  const res: { success: boolean; count: number; message?: string } = await request.delete('/processor/dead-queue')
  if (!res.success) {
    throw new Error(res.message || '清空失败')
  }
  return { count: res.count }
}

// 获取统计数据
export const getProcessorStats = async (): Promise<ProcessorStats> => {
  const res: { success: boolean; data: ProcessorStats; message?: string } = await request.get('/processor/stats')
  if (!res.success) {
    throw new Error(res.message || '获取统计失败')
  }
  return res.data
}
