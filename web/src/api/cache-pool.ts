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
  updated_at?: string
}

/** 单个分组的池状态 */
export interface GroupPoolStats {
  current: number
  max_size: number
  threshold: number
}

/** 缓存池状态统计 */
export interface CachePoolStats {
  titles: Record<number, GroupPoolStats>
  contents: Record<number, GroupPoolStats>
  config: {
    titles_size: number
    contents_size: number
    threshold: number
    refill_interval_ms: number
  }
}

/** 更新配置响应 */
export interface UpdateCachePoolConfigResponse {
  success: boolean
  config: CachePoolConfig
}

// ============================================
// API 接口
// ============================================

/** 获取缓存池配置 */
export function getCachePoolConfig(): Promise<CachePoolConfig> {
  return request.get('/cache-pool/config')
}

/** 更新缓存池配置 */
export function updateCachePoolConfig(config: CachePoolConfig): Promise<UpdateCachePoolConfigResponse> {
  return request.put('/cache-pool/config', config)
}

/** 获取缓存池状态统计 */
export function getCachePoolStats(): Promise<CachePoolStats> {
  return request.get('/cache-pool/stats')
}

/** 重载缓存池配置 */
export function reloadCachePool(): Promise<{ success: boolean }> {
  return request.post('/cache-pool/reload')
}

// ============================================
// 工具函数
// ============================================

/** 格式化池状态摘要 */
export function formatPoolSummary(pools: Record<number, GroupPoolStats>): string {
  const entries = Object.entries(pools)
  if (entries.length === 0) return '无数据'
  return entries.map(([gid, p]) => `分组${gid}: ${p.current}/${p.max_size}`).join(', ')
}

/** 计算池使用率 */
export function calculateUtilization(current: number, maxSize: number): number {
  if (maxSize <= 0) return 0
  return Math.round((current / maxSize) * 100)
}
