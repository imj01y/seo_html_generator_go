/**
 * 标题和正文缓存池配置 API
 *
 * 用于管理 Go 服务中的内存缓存池配置
 */
import request from '@/utils/request'

// ============================================
// 类型定义
// ============================================

/** 缓存池配置 */
export interface CachePoolConfig {
  id?: number
  titles_size: number
  contents_size: number
  threshold: number
  refill_interval_ms: number
  keywords_size: number
  images_size: number
  refresh_interval_ms: number
  updated_at?: string
}

// ============================================
// API 接口
// ============================================

/** 获取缓存池配置 */
export function getCachePoolConfig(): Promise<CachePoolConfig> {
  return request.get('/cache-pool/config')
}

/** 更新缓存池配置 */
export function updateCachePoolConfig(config: CachePoolConfig): Promise<{ success: boolean; config: CachePoolConfig }> {
  return request.put('/cache-pool/config', config)
}
