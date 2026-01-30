import request from '@/utils/request'
import type { SpiderLog, SpiderLogQuery, SpiderStats, DailyStats, PaginatedResponse } from '@/types'

// 后端返回格式
interface BackendSpiderLogsResponse {
  items: SpiderLog[]
  total: number
  page: number
  page_size: number
}

export const getSpiderLogs = async (params: SpiderLogQuery): Promise<PaginatedResponse<SpiderLog>> => {
  const res: BackendSpiderLogsResponse = await request.get('/spiders/logs', {
    params: {
      spider_type: params.spider_type,
      domain: params.site_id,
      start_date: params.start_date,
      end_date: params.end_date,
      page: params.page,
      page_size: params.page_size
    }
  })
  return { items: res.items || [], total: res.total || 0 }
}

export const getSpiderStats = async (): Promise<SpiderStats> => {
  // 从dashboard获取蜘蛛统计
  try {
    const res: { total: number; by_type: Record<string, number> } =
      await request.get('/dashboard/spider-visits')
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

export const getDailyStats = async (days: number = 7): Promise<DailyStats[]> => {
  const res = await request.get('/spiders/daily-stats', { params: { days } })
  // 转换后端数据格式：by_type -> by_spider，并提取 count 值
  const data = res.days || []
  const result = data.map((item: { date: string; total: number; by_type?: Record<string, { count: number }> }) => ({
    date: item.date,
    total: item.total,
    by_spider: item.by_type
      ? Object.fromEntries(Object.entries(item.by_type).map(([k, v]) => [k, v.count]))
      : {}
  }))
  // 按日期升序排列（图表从左到右显示时间）
  return result.sort((a, b) => a.date.localeCompare(b.date))
}

export const getHourlyStats = async (date?: string): Promise<{ hour: number; total: number; by_spider: Record<string, number> }[]> => {
  const res = await request.get('/spiders/hourly-stats', { params: { date } })
  const data = res.hours || []
  // 转换数据格式：by_type -> by_spider
  return data.map((item: { hour: number; total: number; by_type?: Record<string, number> }) => ({
    hour: item.hour,
    total: item.total,
    by_spider: item.by_type || {}
  }))
}

export const clearOldLogs = async (days: number = 30): Promise<{ deleted: number }> => {
  return await request.delete('/spiders/logs/clear', { params: { before_days: days } })
}

// 蜘蛛检测测试
export const testSpiderDetection = (user_agent: string): Promise<{
  is_spider: boolean
  spider_type: string
  spider_name: string
}> => request.post('/spiders/test', null, { params: { user_agent } })

// 获取蜘蛛检测配置
export const getSpiderConfig = (): Promise<{
  enabled: boolean
  dns_verify_enabled: boolean
  dns_verify_types: string[]
  dns_timeout: number
}> => request.get('/spiders/config')
