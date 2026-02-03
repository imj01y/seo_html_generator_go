/**
 * 标题和正文缓存池配置 API
 *
 * 用于管理 Go 服务中的内存缓存池配置
 */
import request from '@/utils/request'

// ============================================
// 类型定义
// ============================================

/** 池分组信息 */
export interface PoolGroupInfo {
  id: number
  name: string
  count: number
}

/** 池状态 */
export interface PoolStats {
  name: string
  size: number
  available: number
  used: number
  utilization: number
  status: string
  num_workers: number
  last_refresh: string | null
  memory_bytes?: number // 内存占用（字节）
  // 新增字段（复用型池使用）
  pool_type?: 'consumable' | 'reusable' | 'static'
  groups?: PoolGroupInfo[]
  source?: string
}

/** 缓存池配置 */
export interface CachePoolConfig {
  id?: number
  // 标题池
  title_pool_size: number
  title_workers: number
  title_refill_interval_ms: number
  title_threshold: number
  // 正文池
  content_pool_size: number
  content_workers: number
  content_refill_interval_ms: number
  content_threshold: number
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

/** 刷新数据池 */
export function refreshDataPool(pool: string, groupId?: number): Promise<{ success: boolean }> {
  return request.post('/admin/data/refresh', {
    pool,
    group_id: groupId
  })
}
