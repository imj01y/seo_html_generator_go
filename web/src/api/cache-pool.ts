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
  // 标题池
  title_pool_size: number
  title_workers: number
  title_refill_interval_ms: number
  title_threshold: number
  // cls类名池
  cls_pool_size: number
  cls_workers: number
  cls_refill_interval_ms: number
  cls_threshold: number
  // url池
  url_pool_size: number
  url_workers: number
  url_refill_interval_ms: number
  url_threshold: number
  // 关键词表情池
  keyword_emoji_pool_size: number
  keyword_emoji_workers: number
  keyword_emoji_refill_interval_ms: number
  keyword_emoji_threshold: number
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
