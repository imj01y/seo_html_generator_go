import request from '@/utils/request'
import type { DashboardStats, DailyStats } from '@/types'

// 后端返回格式（响应拦截器已自动提取 data 字段）
interface BackendDashboardStats {
  site_count: number
  keyword_count: number
  image_count: number
  article_count: number
  template_count: number
}

interface BackendSpiderVisits {
  total_visits: number
  today_visits: number
}

interface GroupStatsResponse {
  total: number
  groups?: Array<{ group_id: number; group_name: string; count: number }>
}

export const getDashboardStats = async (): Promise<DashboardStats> => {
  const res: BackendDashboardStats = await request.get('/dashboard/stats')

  // 并行获取详细统计（包含真实的 memory_mb）
  let keywordStats = { total: res.keyword_count, cursor: 0, remaining: res.keyword_count, loaded: res.keyword_count > 0, memory_mb: 0 }
  let imageStats = { total: res.image_count, cursor: 0, remaining: res.image_count, loaded: res.image_count > 0, memory_mb: 0 }

  try {
    const [kStats, iStats] = await Promise.all([
      request.get('/keywords/stats').catch(() => null) as Promise<GroupStatsResponse | null>,
      request.get('/images/urls/stats').catch(() => null) as Promise<GroupStatsResponse | null>
    ])
    if (kStats) {
      keywordStats = {
        total: kStats.total,
        cursor: 0,
        remaining: kStats.total,
        loaded: kStats.total > 0,
        memory_mb: 0
      }
    }
    if (iStats) {
      imageStats = {
        total: iStats.total,
        cursor: 0,
        remaining: iStats.total,
        loaded: iStats.total > 0,
        memory_mb: 0
      }
    }
  } catch {
    // 静默处理，使用默认值
  }

  return {
    sites_count: res.site_count,
    keywords_total: res.keyword_count,
    images_total: res.image_count,
    articles_total: res.article_count,
    keyword_group_stats: keywordStats,
    image_group_stats: imageStats
  }
}

export const getSpiderStats = async () => {
  const res: BackendSpiderVisits = await request.get('/dashboard/spider-visits')
  return {
    total_visits: res.total_visits || 0,
    by_spider: {} as Record<string, number>,
    by_site: {} as Record<string, number>,
    by_status: {} as Record<string, number>
  }
}

export const getDailySpiderStats = async (_days?: number): Promise<DailyStats[]> => {
  // 后端暂不支持趋势数据
  return []
}

export const getHourlySpiderStats = async (_date?: string): Promise<{ hour: number; total: number }[]> => {
  // 后端暂无此接口，返回空数据
  return []
}

// 获取更详细的分组统计
export const getKeywordGroupStats = (): Promise<GroupStatsResponse> =>
  request.get('/keywords/stats')

export const getImageGroupStats = (): Promise<GroupStatsResponse> =>
  request.get('/images/urls/stats')

export const getCacheStats = () =>
  request.get('/cache/stats')
