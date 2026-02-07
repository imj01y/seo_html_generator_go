import request from '@/utils/request'

// ============================================
// 类型定义
// ============================================

export interface PoolPreset {
  key: string
  name: string
  description: string
  concurrency: number
}

export interface PoolConfig {
  preset: string
  concurrency: number
  buffer_seconds: number
}

export interface TemplateStats {
  max_cls: number
  max_url: number
  max_keyword_emoji: number
  max_keyword: number
  max_image: number
  max_content: number
  source_template: string
}

export interface PoolSizes {
  ClsPoolSize: number
  URLPoolSize: number
  KeywordEmojiPoolSize: number
}

export interface MemoryEstimate {
  bytes: number
  human: string
}

export interface PoolConfigResponse {
  config: PoolConfig
  template_stats: TemplateStats
  calculated: PoolSizes
  memory: MemoryEstimate
}

export interface UpdateConfigRequest {
  preset: string
  concurrency: number
  buffer_seconds: number
}

export interface UpdateConfigResponse {
  message: string
  calculated: PoolSizes
}

export type PoolStatus = 'running' | 'paused' | 'stopped'

export interface PoolGroupInfo {
  id: number
  name: string
  count: number
}

export interface PoolStats {
  name: string
  size: number
  available: number
  used: number
  utilization: number
  status: PoolStatus
  num_workers: number
  last_refresh: string | null
  memory_bytes?: number // 内存占用（字节）
  total_generated?: number
  total_consumed?: number
  paused?: boolean
  refill_count?: number
  // 池类型（复用型池使用）
  pool_type?: 'consumable' | 'reusable' | 'static'
  groups?: PoolGroupInfo[]
  source?: string
}

// ============================================
// 预设配置
// ============================================

export const poolPresets: PoolPreset[] = [
  { key: 'low', name: '低', description: '适用于小站点、低配服务器', concurrency: 50 },
  { key: 'medium', name: '中', description: '适用于中等规模站群', concurrency: 200 },
  { key: 'high', name: '高', description: '适用于大规模站群', concurrency: 500 },
  { key: 'extreme', name: '极高', description: '适用于高性能服务器', concurrency: 1000 }
]

// ============================================
// 池配置 API
// ============================================

export function getPresets(): PoolPreset[] {
  return poolPresets
}

export function getPoolConfig(): Promise<PoolConfigResponse> {
  return request.get('/pool-config')
}

export function updatePoolConfig(config: UpdateConfigRequest): Promise<UpdateConfigResponse> {
  return request.put('/pool-config', config)
}

// ============================================
// 池状态监控 API
// ============================================

export function refreshDataPool(pool?: string): Promise<void> {
  return request.post('/admin/data/refresh', { pool: pool || 'all' })
}

// ============================================
// 工具函数
// ============================================

export function formatMemorySize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(2)} KB`
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}
