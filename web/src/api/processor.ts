import request from '@/utils/request'
import { assertSuccess, type SuccessResponse } from './shared'

// ============================================
// 类型定义
// ============================================

export interface ProcessorConfig {
  enabled: boolean
  concurrency: number
  retry_max: number
  min_paragraph_length: number
  batch_size: number
}

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

// ============================================
// 响应类型
// ============================================

interface DataResponse<T> extends SuccessResponse {
  data: T
}

interface CountResponse extends SuccessResponse {
  count: number
}

// ============================================
// 数据加工 API
// ============================================

export async function getProcessorConfig(): Promise<ProcessorConfig> {
  const res: DataResponse<ProcessorConfig> = await request.get('/processor/config')
  assertSuccess(res, '获取配置失败')
  return res.data
}

export async function updateProcessorConfig(config: ProcessorConfig): Promise<ProcessorConfig> {
  const res: DataResponse<ProcessorConfig> = await request.put('/processor/config', config)
  assertSuccess(res, '更新配置失败')
  return res.data
}

export async function startProcessor(): Promise<void> {
  const res: SuccessResponse = await request.post('/processor/start')
  assertSuccess(res, '启动失败')
}

export async function stopProcessor(): Promise<void> {
  const res: SuccessResponse = await request.post('/processor/stop')
  assertSuccess(res, '停止失败')
}

export async function retryAllFailed(): Promise<{ count: number }> {
  const res: CountResponse = await request.post('/processor/retry-all')
  assertSuccess(res, '重试失败')
  return { count: res.count }
}

export async function clearDeadQueue(): Promise<{ count: number }> {
  const res: CountResponse = await request.delete('/processor/dead-queue')
  assertSuccess(res, '清空失败')
  return { count: res.count }
}
