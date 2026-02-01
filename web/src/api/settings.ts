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

export async function getSetting(key: string): Promise<Setting> {
  const settings = await getSettings()
  const setting = settings.find((s) => s.key === key)
  if (!setting) {
    throw new Error(`Setting ${key} not found`)
  }
  return setting
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
  let htmlCacheStats = { total_entries: 0, total_size_mb: 0 }

  try {
    htmlCacheStats = await request.get('/cache/stats')
  } catch {
    // ignore
  }

  return {
    html_cache_entries: htmlCacheStats.total_entries || 0,
    html_cache_memory_mb: htmlCacheStats.total_size_mb || 0
  }
}

export function clearDomainCache(domain: string): Promise<{ success: boolean; cleared: number }> {
  return request.post(`/cache/clear/${domain}`)
}

// ============================================
// 系统状态 API
// ============================================

export function checkDatabase(): Promise<{
  connected: boolean
  pool_size?: number
  free_connections?: number
  error?: string
}> {
  return request.get('/settings/database')
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
