import request from '@/utils/request'
import type { Setting } from '@/types'
import { assertSuccess, type SuccessResponse } from './shared'

// ============================================
// 类型定义
// ============================================

export interface ApiTokenResponse {
  success: boolean
  token?: string
  enabled?: boolean
  message?: string
}

// ============================================
// 响应类型
// ============================================

interface BackendSettings {
  server: { host: string; port: number }
  cache: { enabled: boolean; ttl_hours: number; max_size_gb: number; gzip_enabled: boolean }
  seo: { internal_links_count: number; encoding_mix_ratio: number; emoji_count_min: number; emoji_count_max: number }
  spider_detector: { enabled: boolean; dns_verify_enabled: boolean; return_404_for_non_spider: boolean }
}

interface CacheStats {
  html_cache_entries: number
  html_cache_memory_mb: number
  initialized: boolean
  scanning: boolean
  last_scan_at: string | null
  site_cache: { item_count: number; memory_bytes: number }
  template_cache: { item_count: number; memory_bytes: number }
}

interface RecalculateResponse {
  total_entries: number
  total_size_mb: number
  duration_ms: number
  message: string
}

// ============================================
// 设置 API
// ============================================

export async function getSettings(): Promise<Setting[]> {
  const res: BackendSettings = await request.get('/settings')
  return [
    { key: 'server_host', value: res.server.host, description: '服务器地址', updated_at: '' },
    { key: 'server_port', value: String(res.server.port), description: '服务器端口', updated_at: '' },
    { key: 'cache_enabled', value: String(res.cache.enabled), description: '启用缓存', updated_at: '' },
    { key: 'cache_ttl', value: String(res.cache.ttl_hours * 3600), description: '缓存过期时间(秒)', updated_at: '' },
    { key: 'encoding_ratio', value: String(res.seo.encoding_mix_ratio), description: '编码混合比例', updated_at: '' },
    { key: 'internal_links_count', value: String(res.seo.internal_links_count), description: '内链数量', updated_at: '' }
  ]
}

export async function updateSettings(settings: Record<string, string>): Promise<void> {
  const res: SuccessResponse = await request.put('/settings/cache', settings)
  assertSuccess(res, '更新失败')
}

// ============================================
// 缓存 API
// ============================================

export async function clearCache(): Promise<{ message: string }> {
  const res: { success: boolean; cleared: number } = await request.post('/cache/clear')
  return { message: `已清理 ${res.cleared} 条缓存` }
}

export async function getCacheStats(): Promise<CacheStats> {
  let htmlCacheStats = {
    total_entries: 0,
    total_size_mb: 0,
    initialized: false,
    scanning: false,
    last_scan_at: null as string | null
  }

  try {
    htmlCacheStats = await request.get('/cache/stats')
  } catch {
    // ignore
  }

  return {
    html_cache_entries: htmlCacheStats.total_entries || 0,
    html_cache_memory_mb: htmlCacheStats.total_size_mb || 0,
    initialized: htmlCacheStats.initialized ?? false,
    scanning: htmlCacheStats.scanning ?? false,
    last_scan_at: htmlCacheStats.last_scan_at || null,
    site_cache: htmlCacheStats.site_cache || { item_count: 0, memory_bytes: 0 },
    template_cache: htmlCacheStats.template_cache || { item_count: 0, memory_bytes: 0 }
  }
}

export async function recalculateCacheStats(): Promise<RecalculateResponse> {
  return request.post('/cache/stats/recalculate')
}

export function clearDomainCache(domain: string): Promise<{ success: boolean; cleared: number }> {
  return request.post(`/cache/clear/${domain}`)
}

// ============================================
// API Token API
// ============================================

export function getApiTokenSettings(): Promise<ApiTokenResponse> {
  return request.get('/settings/api-token')
}

export function updateApiTokenSettings(data: { token: string; enabled: boolean }): Promise<SuccessResponse> {
  return request.put('/settings/api-token', data)
}

export function generateApiToken(): Promise<{ success: boolean; token: string }> {
  return request.post('/settings/api-token/generate')
}
