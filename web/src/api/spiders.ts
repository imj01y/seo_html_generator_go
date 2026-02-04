import request from '@/utils/request'
import type { SpiderLog, SpiderLogQuery, SpiderStats, DailyStats, PaginatedResponse } from '@/types'

// ============================================
// 响应类型
// ============================================

interface SpiderLogsResponse {
  items: SpiderLog[]
  total: number
}

interface SpiderVisitsResponse {
  total: number
  by_type: Record<string, number>
}

interface DailyStatsItem {
  date: string
  total: number
  by_type?: Record<string, { count: number }>
}

interface HourlyStatsItem {
  hour: number
  total: number
  by_type?: Record<string, number>
}

// ============================================
// 蜘蛛日志 API
// ============================================

export async function getSpiderLogs(params: SpiderLogQuery): Promise<PaginatedResponse<SpiderLog>> {
  const res: SpiderLogsResponse = await request.get('/spiders/logs', {
    params: {
      spider_type: params.spider_type,
      domain: params.domain,
      start_date: params.start_date,
      end_date: params.end_date,
      page: params.page,
      page_size: params.page_size
    }
  })
  return { items: res.items || [], total: res.total || 0 }
}

export async function getSpiderStats(): Promise<SpiderStats> {
  try {
    const res: SpiderVisitsResponse = await request.get('/dashboard/spider-visits')
    return {
      total_visits: res.total,
      by_spider: res.by_type,
      by_site: {},
      by_status: {}
    }
  } catch {
    return {
      total_visits: 0,
      by_spider: {},
      by_site: {},
      by_status: {}
    }
  }
}

export async function getDailyStats(days: number = 7): Promise<DailyStats[]> {
  const res: { days: DailyStatsItem[] } = await request.get('/spiders/daily-stats', { params: { days } })
  const data = res.days || []

  return data
    .map((item) => ({
      date: item.date,
      total: item.total,
      by_spider: item.by_type
        ? Object.fromEntries(Object.entries(item.by_type).map(([k, v]) => [k, v.count]))
        : {}
    }))
    .sort((a, b) => a.date.localeCompare(b.date))
}

export async function getHourlyStats(
  date?: string
): Promise<{ hour: number; total: number; by_spider: Record<string, number> }[]> {
  const res: { hours: HourlyStatsItem[] } = await request.get('/spiders/hourly-stats', { params: { date } })
  const data = res.hours || []

  return data.map((item) => ({
    hour: item.hour,
    total: item.total,
    by_spider: item.by_type || {}
  }))
}

export async function clearOldLogs(days: number = 30): Promise<{ deleted: number }> {
  return request.delete('/spiders/logs/clear', { params: { before_days: days } })
}

// ============================================
// 蜘蛛检测 API
// ============================================

export function testSpiderDetection(user_agent: string): Promise<{
  is_spider: boolean
  spider_type: string
  spider_name: string
}> {
  return request.post('/spiders/test', null, { params: { user_agent } })
}

export function getSpiderConfig(): Promise<{
  enabled: boolean
  dns_verify_enabled: boolean
  dns_verify_types: string[]
  dns_timeout: number
}> {
  return request.get('/spiders/config')
}

// ============================================
// 蜘蛛日志趋势 API
// ============================================

export interface SpiderTrendPoint {
  time: string
  total: number
  status_2xx: number
  status_3xx: number
  status_4xx: number
  status_5xx: number
  avg_resp_time: number
}

export interface SpiderTrendResponse {
  period: string
  items: SpiderTrendPoint[]
}

export async function getSpiderTrend(params?: {
  period?: 'minute' | 'hour' | 'day' | 'month'
  spider_type?: string
  limit?: number
}): Promise<SpiderTrendResponse> {
  const res: SpiderTrendResponse = await request.get('/spiders/trend', { params })
  return res || { period: params?.period || 'hour', items: [] }
}
