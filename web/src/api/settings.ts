import request from '@/utils/request'
import type { Setting, CacheStats } from '@/types'

// 后端返回的设置格式
interface BackendSettings {
  server: {
    host: string
    port: number
  }
  cache: {
    enabled: boolean
    ttl_hours: number
    max_size_gb: number
    gzip_enabled: boolean
  }
  seo: {
    internal_links_count: number
    encoding_mix_ratio: number
    emoji_count_min: number
    emoji_count_max: number
  }
  spider_detector: {
    enabled: boolean
    dns_verify_enabled: boolean
    return_404_for_non_spider: boolean
  }
}

export const getSettings = async (): Promise<Setting[]> => {
  const res: BackendSettings = await request.get('/settings')
  // 转换为扁平的设置列表
  return [
    { key: 'server_host', value: res.server.host, description: '服务器地址', updated_at: '' },
    { key: 'server_port', value: String(res.server.port), description: '服务器端口', updated_at: '' },
    { key: 'cache_enabled', value: String(res.cache.enabled), description: '启用缓存', updated_at: '' },
    { key: 'cache_ttl', value: String(res.cache.ttl_hours * 3600), description: '缓存过期时间(秒)', updated_at: '' },
    { key: 'encoding_ratio', value: String(res.seo.encoding_mix_ratio), description: '编码混合比例', updated_at: '' },
    { key: 'internal_links_count', value: String(res.seo.internal_links_count), description: '内链数量', updated_at: '' },
  ]
}

export const getSetting = async (key: string): Promise<Setting> => {
  const settings = await getSettings()
  const setting = settings.find(s => s.key === key)
  if (!setting) {
    throw new Error(`Setting ${key} not found`)
  }
  return setting
}

export const updateSettings = async (settings: Record<string, string>): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.put('/settings/cache', settings)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
}

// 缓存配置相关
export interface CacheSettingItem {
  value: number | string | boolean
  type: string
  description: string
}

export interface CacheSettingsResponse {
  success: boolean
  settings: Record<string, CacheSettingItem>
  message?: string
}

export const getCacheSettings = async (): Promise<Record<string, CacheSettingItem>> => {
  const res: CacheSettingsResponse = await request.get('/settings/cache')
  if (!res.success) {
    throw new Error(res.message || '获取失败')
  }
  return res.settings
}

export const updateCacheSettings = async (settings: Record<string, number | string | boolean>): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.put('/settings/cache', settings)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
}

export const applyCacheSettings = async (): Promise<{ message: string; applied: string[] }> => {
  const res: { success: boolean; message: string; applied: string[] } = await request.post('/settings/cache/apply')
  if (!res.success) {
    throw new Error(res.message || '应用失败')
  }
  return { message: res.message, applied: res.applied }
}

export const clearCache = async (): Promise<{ message: string }> => {
  const res: { success: boolean; cleared: number } = await request.post('/cache/clear')
  return { message: `已清理 ${res.cleared} 条缓存` }
}

interface GroupStatsResponse {
  total: number
  cursor: number
  remaining: number
  loaded: boolean
  memory_mb: number
  redis_pool_count?: number
}

const defaultGroupStats: GroupStatsResponse = {
  total: 0,
  cursor: 0,
  remaining: 0,
  loaded: false,
  memory_mb: 0
}

export const getCacheStats = async (): Promise<CacheStats> => {
  // 获取页面缓存统计
  let htmlCacheStats = { total_entries: 0, total_size_mb: 0 }
  try {
    htmlCacheStats = await request.get('/cache/stats')
  } catch {
    // ignore
  }

  let keywordStats = { ...defaultGroupStats }
  let imageStats = { ...defaultGroupStats }

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

// 清理指定域名缓存
export const clearDomainCache = (domain: string): Promise<{ success: boolean; cleared: number }> =>
  request.post(`/cache/clear/${domain}`)

// 获取缓存条目
export const getCacheEntries = (params?: { domain?: string; offset?: number; limit?: number }) =>
  request.get('/cache/entries', { params })

// 检查数据库连接
export const checkDatabase = (): Promise<{
  connected: boolean
  pool_size?: number
  free_connections?: number
  error?: string
}> => request.get('/settings/database')

// 获取GeneratorWorker状态
export const getGeneratorWorkerStatus = (): Promise<{
  success: boolean
  running: boolean
  worker_initialized: boolean
}> => request.get('/generator/worker/status')

// 获取待处理队列统计
export const getGeneratorQueueStats = (groupId: number = 1): Promise<{
  success: boolean
  group_id: number
  queue_size: number
}> => request.get('/generator/queue/stats', { params: { group_id: groupId } })

// 清空待处理队列
export const clearGeneratorQueue = (groupId: number = 1): Promise<{
  success: boolean
  cleared: number
  message: string
}> => request.post('/generator/queue/clear', null, { params: { group_id: groupId } })

// 段落池告警状态类型
export interface ContentPoolAlert {
  level: 'normal' | 'warning' | 'critical' | 'exhausted' | 'unknown'
  message: string
  pool_size: number
  used_size: number
  total: number
  updated_at: string
}

// 获取段落池告警状态
export const getContentPoolAlert = (): Promise<{
  success: boolean
  alert: ContentPoolAlert
  message?: string
}> => request.get('/alerts/content-pool')

// 重置段落池
export const resetContentPool = (): Promise<{
  success: boolean
  message: string
  count: number
}> => request.post('/alerts/content-pool/reset')

// CachePool 缓存池统计
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

export const getCachePoolsStats = (): Promise<CachePoolsStatsResponse> =>
  request.get('/cache/pools/stats')

// 文件缓存配置字段
export interface FileCacheSettings {
  file_cache_enabled: boolean
  file_cache_dir: string
  file_cache_max_size_gb: number
  file_cache_nginx_mode: boolean
}

// API Token 设置
export interface ApiTokenResponse {
  success: boolean
  token?: string
  enabled?: boolean
  message?: string
}

export const getApiTokenSettings = (): Promise<ApiTokenResponse> =>
  request.get('/settings/api-token')

export const updateApiTokenSettings = (data: { token: string; enabled: boolean }): Promise<{ success: boolean; message?: string }> =>
  request.put('/settings/api-token', data)

export const generateApiToken = (): Promise<{ success: boolean; token: string }> =>
  request.post('/settings/api-token/generate')
