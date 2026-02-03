import request from '@/utils/request'
import type { DashboardStats } from '@/types'

// ============================================
// 响应类型
// ============================================

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

// ============================================
// Dashboard API
// ============================================

export async function getDashboardStats(): Promise<DashboardStats> {
  const res: BackendDashboardStats = await request.get('/dashboard/stats')

  const defaultStats = {
    total: 0,
    cursor: 0,
    remaining: 0,
    loaded: false
  }

  let keywordStats = { ...defaultStats, total: res.keyword_count, remaining: res.keyword_count, loaded: res.keyword_count > 0 }
  let imageStats = { ...defaultStats, total: res.image_count, remaining: res.image_count, loaded: res.image_count > 0 }

  try {
    const [kStats, iStats] = await Promise.all([
      request.get('/keywords/stats').catch(() => null) as Promise<GroupStatsResponse | null>,
      request.get('/images/urls/stats').catch(() => null) as Promise<GroupStatsResponse | null>
    ])

    if (kStats) {
      keywordStats = { ...defaultStats, total: kStats.total, remaining: kStats.total, loaded: kStats.total > 0 }
    }
    if (iStats) {
      imageStats = { ...defaultStats, total: iStats.total, remaining: iStats.total, loaded: iStats.total > 0 }
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

export async function getSpiderStats(): Promise<{
  total_visits: number
  by_spider: Record<string, number>
  by_site: Record<string, number>
  by_status: Record<string, number>
}> {
  const res: BackendSpiderVisits = await request.get('/dashboard/spider-visits')
  return {
    total_visits: res.total_visits || 0,
    by_spider: {},
    by_site: {},
    by_status: {}
  }
}
