import request from '@/utils/request'
import type { Site, SiteCreate, SiteUpdate, PaginatedResponse, GroupOption } from '@/types'

// 后端返回的站点格式（与前端类型一致）
interface BackendSite {
  id: number
  site_group_id: number
  domain: string
  name: string
  template: string
  keyword_group_id?: number
  image_group_id?: number
  article_group_id?: number
  status: number  // 1=启用, 0=禁用
  icp_number?: string
  baidu_token?: string
  analytics?: string
  created_at: string
  updated_at: string
}

// 后端返回格式
interface BackendSiteListResponse {
  items: BackendSite[]
  total: number
  page: number
  page_size: number
}

// 分组选项响应
interface GroupOptionsResponse {
  keyword_groups: GroupOption[]
  image_groups: GroupOption[]
}

// 转换后端站点为前端格式
const convertSite = (site: BackendSite): Site => ({
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
})

export const getSites = async (params?: { page?: number; page_size?: number }): Promise<Site[]> => {
  const res: BackendSiteListResponse = await request.get('/sites', { params })
  return (res.items || []).map(convertSite)
}

export const getSitesPaginated = async (params?: { page?: number; page_size?: number }): Promise<PaginatedResponse<Site>> => {
  const res: BackendSiteListResponse = await request.get('/sites', { params })
  return {
    items: (res.items || []).map(convertSite),
    total: res.total
  }
}

export const getSite = async (id: number): Promise<Site> => {
  const site: BackendSite = await request.get(`/sites/${id}`)
  return convertSite(site)
}

export const createSite = async (data: SiteCreate): Promise<Site> => {
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
  const res: { success: boolean; id?: number; message?: string } = await request.post('/sites', backendData)
  if (res.success && res.id) {
    return {
      id: res.id,
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
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    }
  }
  throw new Error(res.message || '创建失败')
}

export const updateSite = async (id: number, data: SiteUpdate): Promise<Site> => {
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

  const res: { success: boolean; message?: string } = await request.put(`/sites/${id}`, backendData)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
  return { id, ...data } as Site
}

export const deleteSite = async (id: number): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.delete(`/sites/${id}`)
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
}

// 获取分组选项（用于站点绑定下拉列表）
export const getGroupOptions = async (): Promise<GroupOptionsResponse> => {
  const res: GroupOptionsResponse = await request.get('/groups/options')
  return {
    keyword_groups: res.keyword_groups || [],
    image_groups: res.image_groups || []
  }
}

// 批量操作
export const batchDeleteSites = async (ids: number[]): Promise<{ deleted: number }> => {
  const res: { success: boolean; deleted: number; message?: string } = await request.delete('/sites/batch/delete', {
    data: { ids }
  })
  if (!res.success) {
    throw new Error(res.message || '批量删除失败')
  }
  return { deleted: res.deleted }
}

export const batchUpdateSiteStatus = async (ids: number[], status: number): Promise<{ updated: number }> => {
  const res: { success: boolean; updated: number; message?: string } = await request.put('/sites/batch/status', {
    ids,
    status
  })
  if (!res.success) {
    throw new Error(res.message || '批量更新状态失败')
  }
  return { updated: res.updated }
}
