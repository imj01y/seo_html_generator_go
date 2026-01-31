import request from '@/utils/request'
import type { Site, SiteCreate, SiteUpdate, PaginatedResponse, GroupOption } from '@/types'
import { assertSuccess, type SuccessResponse, type CreateResponse, type CountResponse } from './shared'

// ============================================
// 响应类型
// ============================================

interface BackendSite {
  id: number
  site_group_id: number
  domain: string
  name: string
  template: string
  keyword_group_id?: number
  image_group_id?: number
  article_group_id?: number
  status: number
  icp_number?: string
  baidu_token?: string
  analytics?: string
  created_at: string
  updated_at: string
}

interface SiteListResponse {
  items: BackendSite[]
  total: number
}

interface GroupOptionsResponse {
  keyword_groups: GroupOption[]
  image_groups: GroupOption[]
}

// ============================================
// 工具函数
// ============================================

function convertSite(site: BackendSite): Site {
  return {
    id: site.id,
    site_group_id: site.site_group_id,
    domain: site.domain,
    name: site.name,
    template: site.template,
    keyword_group_id: site.keyword_group_id || null,
    image_group_id: site.image_group_id || null,
    article_group_id: site.article_group_id || null,
    status: site.status,
    icp_number: site.icp_number || null,
    baidu_token: site.baidu_token || null,
    analytics: site.analytics || null,
    created_at: site.created_at,
    updated_at: site.updated_at
  }
}

// ============================================
// 站点 API
// ============================================

export async function getSites(params?: { page?: number; page_size?: number }): Promise<Site[]> {
  const res: SiteListResponse = await request.get('/sites', { params })
  return (res.items || []).map(convertSite)
}

export async function getSitesPaginated(params?: { page?: number; page_size?: number }): Promise<PaginatedResponse<Site>> {
  const res: SiteListResponse = await request.get('/sites', { params })
  return {
    items: (res.items || []).map(convertSite),
    total: res.total
  }
}

export async function getSite(id: number): Promise<Site> {
  const site: BackendSite = await request.get(`/sites/${id}`)
  return convertSite(site)
}

export async function createSite(data: SiteCreate): Promise<Site> {
  const backendData = {
    site_group_id: data.site_group_id,
    domain: data.domain,
    name: data.name,
    template: data.template || 'download_site',
    keyword_group_id: data.keyword_group_id,
    image_group_id: data.image_group_id,
    article_group_id: data.article_group_id,
    icp_number: data.icp_number,
    baidu_token: data.baidu_token,
    analytics: data.analytics
  }
  const res: CreateResponse = await request.post('/sites', backendData)
  assertSuccess(res, '创建失败')

  const now = new Date().toISOString()
  return {
    id: res.id!,
    site_group_id: data.site_group_id,
    domain: data.domain,
    name: data.name,
    template: data.template || 'download_site',
    keyword_group_id: data.keyword_group_id || null,
    image_group_id: data.image_group_id || null,
    article_group_id: data.article_group_id || null,
    status: 1,
    icp_number: data.icp_number || null,
    baidu_token: data.baidu_token || null,
    analytics: data.analytics || null,
    created_at: now,
    updated_at: now
  }
}

export async function updateSite(id: number, data: SiteUpdate): Promise<Site> {
  const backendData: Record<string, unknown> = {}
  if (data.site_group_id !== undefined) backendData.site_group_id = data.site_group_id
  if (data.name !== undefined) backendData.name = data.name
  if (data.template !== undefined) backendData.template = data.template
  if (data.status !== undefined) backendData.status = data.status
  if (data.keyword_group_id !== undefined) backendData.keyword_group_id = data.keyword_group_id
  if (data.image_group_id !== undefined) backendData.image_group_id = data.image_group_id
  if (data.article_group_id !== undefined) backendData.article_group_id = data.article_group_id
  if (data.icp_number !== undefined) backendData.icp_number = data.icp_number
  if (data.baidu_token !== undefined) backendData.baidu_token = data.baidu_token
  if (data.analytics !== undefined) backendData.analytics = data.analytics

  const res: SuccessResponse = await request.put(`/sites/${id}`, backendData)
  assertSuccess(res, '更新失败')
  return { id, ...data } as Site
}

export async function deleteSite(id: number): Promise<void> {
  const res: SuccessResponse = await request.delete(`/sites/${id}`)
  assertSuccess(res, '删除失败')
}

// ============================================
// 分组选项 API
// ============================================

export async function getGroupOptions(): Promise<GroupOptionsResponse> {
  const res: GroupOptionsResponse = await request.get('/groups/options')
  return {
    keyword_groups: res.keyword_groups || [],
    image_groups: res.image_groups || []
  }
}

// ============================================
// 批量操作 API
// ============================================

export async function batchDeleteSites(ids: number[]): Promise<{ deleted: number }> {
  const res: CountResponse = await request.delete('/sites/batch/delete', { data: { ids } })
  assertSuccess(res, '批量删除失败')
  return { deleted: res.deleted! }
}

export async function batchUpdateSiteStatus(ids: number[], status: number): Promise<{ updated: number }> {
  const res: CountResponse = await request.put('/sites/batch/status', { ids, status })
  assertSuccess(res, '批量更新状态失败')
  return { updated: res.updated! }
}
