import request from '@/utils/request'
import type { DashboardStats, DailyStats } from '@/types'

// 后端返回格式
interface BackendDashboardStats {
  sites_count: number
  keywords_count: number
  images_count: number
  articles_count: number
  cache_entries: number
  cache_size_mb: number
  today_generations: number
  today_spider_visits: number
}

interface BackendSpiderVisits {
  total: number
  by_type: Record<string, number>
  trend: Array<{ date: string; count: number }>
}

export const getDashboardStats = async (): Promise<DashboardStats> => {
  const res: BackendDashboardStats = await request.get('/dashboard/stats')

  // 并行获取详细统计（包含真实的 memory_mb）
  let keywordStats = { total: res.keywords_count, cursor: 0, remaining: res.keywords_count, loaded: res.keywords_count > 0, memory_mb: 0 }
  let imageStats = { total: res.images_count, cursor: 0, remaining: res.images_count, loaded: res.images_count > 0, memory_mb: 0 }

  try {
    const [kStats, iStats] = await Promise.all([
      request.get('/keywords/stats').catch(() => null),
      request.get('/images/urls/stats').catch(() => null)
    ])
    if (kStats) keywordStats = kStats
    if (iStats) imageStats = iStats
  } catch {
    // 静默处理，使用默认值
  }

  return {
    sites_count: res.sites_count,
    keywords_total: res.keywords_count,
    images_total: res.images_count,
    articles_total: res.articles_count,
    keyword_group_stats: keywordStats,
    image_group_stats: imageStats
  }
}

export const getSpiderStats = async () => {
  const res: BackendSpiderVisits = await request.get('/dashboard/spider-visits')
  return {
    total_visits: res.total,
    by_spider: res.by_type,
    by_site: {} as Record<string, number>,
    by_status: {} as Record<string, number>
  }
}

export const getDailySpiderStats = async (_days?: number): Promise<DailyStats[]> => {
  // 从 spider-visits 接口获取趋势数据
  const res: BackendSpiderVisits = await request.get('/dashboard/spider-visits')
  if (!res.trend || res.trend.length === 0) {
    return []
  }
  // 转换为 DailyStats 格式
  return res.trend.map(item => ({
    date: item.date,
    total: item.count,
    by_spider: {}
  }))
}

export const getHourlySpiderStats = async (_date?: string): Promise<{ hour: number; total: number }[]> => {
  // 后端暂无此接口，返回空数据
  return []
}

// 获取更详细的分组统计
export const getKeywordGroupStats = () =>
  request.get('/keywords/stats')

export const getImageGroupStats = () =>
  request.get('/images/urls/stats')

export const getCacheStats = () =>
  request.get('/cache/stats')
