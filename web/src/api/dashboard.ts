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

// ============================================
// Dashboard API
// ============================================

export async function getDashboardStats(): Promise<DashboardStats> {
  const res: BackendDashboardStats = await request.get('/dashboard/stats')

  return {
    sites_count: res.site_count,
    keywords_total: res.keyword_count,
    images_total: res.image_count,
    articles_total: res.article_count
  }
}
