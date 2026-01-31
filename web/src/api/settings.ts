import request from '@/utils/request'
import type { Setting, CacheStats } from '@/types'
import { assertSuccess, type SuccessResponse } from './shared'

// ============================================
// 类型定义
// ============================================

export interface CacheSettingItem {
  value: number | string | boolean
  type: string
  description: string
}

export interface ContentPoolAlert {
  level: 'normal' | 'warning' | 'critical' | 'exhausted' | 'unknown'
  message: string
  pool_size: number
  used_size: number
  total: number
  updated_at: string
}

export interface CachePoolStats {
  cache_size: number
  cursor: number
  remaining: number
  low_watermark: number
  target_size: number
  total_consumed: number
  total_refilled: number
  total_merged: number
  running: boolean
}

export interface CachePoolsStatsResponse {
  keyword_pool: CachePoolStats | null
  image_pool: CachePoolStats | null
}

export interface FileCacheSettings {
  file_cache_enabled: boolean
  file_cache_dir: string
  file_cache_max_size_gb: number
  file_cache_nginx_mode: boolean
}

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

interface GroupStatsResponse {
  total: number
  cursor: number
  remaining: number
  loaded: boolean
  memory_mb: number
  redis_pool_count?: number
}

interface CacheSettingsResponse extends SuccessResponse {
  settings: Record<string, CacheSettingItem>
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
// 缓存配置 API
// ============================================

export async function getCacheSettings(): Promise<Record<string, CacheSettingItem>> {
  const res: CacheSettingsResponse = await request.get('/settings/cache')
  assertSuccess(res, '获取失败')
  return res.settings
}

export async function updateCacheSettings(settings: Record<string, number | string | boolean>): Promise<void> {
  const res: SuccessResponse = await request.put('/settings/cache', settings)
  assertSuccess(res, '更新失败')
}

export async function applyCacheSettings(): Promise<{ message: string; applied: string[] }> {
  const res: SuccessResponse & { applied: string[] } = await request.post('/settings/cache/apply')
  assertSuccess(res, '应用失败')
  return { message: res.message || '', applied: res.applied }
}

export async function clearCache(): Promise<{ message: string }> {
  const res: { success: boolean; cleared: number } = await request.post('/cache/clear')
  return { message: `已清理 ${res.cleared} 条缓存` }
}

// ============================================
// 缓存统计 API
// ============================================

const defaultGroupStats: GroupStatsResponse = {
  total: 0,
  cursor: 0,
  remaining: 0,
  loaded: false,
  memory_mb: 0
}

export async function getCacheStats(): Promise<CacheStats> {
  let htmlCacheStats = { total_entries: 0, total_size_mb: 0 }
  let keywordStats = { ...defaultGroupStats }
  let imageStats = { ...defaultGroupStats }

  try {
    htmlCacheStats = await request.get('/cache/stats')
  } catch {
    // ignore
  }

  try {
    keywordStats = await request.get('/keywords/stats')
  } catch {
    // ignore
  }

  try {
    imageStats = await request.get('/images/urls/stats')
  } catch {
    // ignore
  }

  return {
    keyword_cache_size: keywordStats.total,
    image_cache_size: imageStats.redis_pool_count ?? imageStats.total,
    keyword_group_stats: keywordStats,
    image_group_stats: imageStats,
    html_cache_entries: htmlCacheStats.total_entries || 0,
    html_cache_memory_mb: htmlCacheStats.total_size_mb || 0
  }
}

export function clearDomainCache(domain: string): Promise<{ success: boolean; cleared: number }> {
  return request.post(`/cache/clear/${domain}`)
}

export function getCacheEntries(params?: { domain?: string; offset?: number; limit?: number }): Promise<unknown> {
  return request.get('/cache/entries', { params })
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

export function getGeneratorWorkerStatus(): Promise<{
  success: boolean
  running: boolean
  worker_initialized: boolean
}> {
  return request.get('/generator/worker/status')
}

export function getGeneratorQueueStats(groupId: number = 1): Promise<{
  success: boolean
  group_id: number
  queue_size: number
}> {
  return request.get('/generator/queue/stats', { params: { group_id: groupId } })
}

export function clearGeneratorQueue(groupId: number = 1): Promise<{
  success: boolean
  cleared: number
  message: string
}> {
  return request.post('/generator/queue/clear', null, { params: { group_id: groupId } })
}

// ============================================
// 段落池 API
// ============================================

export function getContentPoolAlert(): Promise<{
  success: boolean
  alert: ContentPoolAlert
  message?: string
}> {
  return request.get('/alerts/content-pool')
}

export function resetContentPool(): Promise<{
  success: boolean
  message: string
  count: number
}> {
  return request.post('/alerts/content-pool/reset')
}

// ============================================
// 缓存池 API
// ============================================

export function getCachePoolsStats(): Promise<CachePoolsStatsResponse> {
  return request.get('/cache/pools/stats')
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
