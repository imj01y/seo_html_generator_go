import request from '@/utils/request'

// 类型定义
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
  NumberPoolSize: number
  KeywordPoolSize: number
  ImagePoolSize: number
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

// 预设列表（前端固定，与后端 PoolPresets 保持一致）
export const poolPresets: PoolPreset[] = [
  { key: 'low', name: '低', description: '适用于小站点、低配服务器', concurrency: 50 },
  { key: 'medium', name: '中', description: '适用于中等规模站群', concurrency: 200 },
  { key: 'high', name: '高', description: '适用于大规模站群', concurrency: 500 },
  { key: 'extreme', name: '极高', description: '适用于高性能服务器', concurrency: 1000 }
]

/**
 * 获取预设列表
 */
export const getPresets = (): PoolPreset[] => {
  return poolPresets
}

/**
 * 获取当前池配置
 */
export const getPoolConfig = (): Promise<PoolConfigResponse> => {
  return request.get('/pool-config')
}

/**
 * 更新池配置
 */
export const updatePoolConfig = (config: UpdateConfigRequest): Promise<UpdateConfigResponse> => {
  return request.put('/pool-config', config)
}

/**
 * 格式化内存大小
 */
export const formatMemorySize = (bytes: number): string => {
  if (bytes < 1024) {
    return `${bytes} B`
  } else if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(2)} KB`
  } else if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  } else {
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
  }
}
