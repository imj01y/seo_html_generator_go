import request from '@/utils/request'
import type {
  SiteGroup,
  SiteGroupCreate,
  SiteGroupUpdate,
  SiteGroupWithStats,
  KeywordGroup,
  ImageGroup,
  ArticleGroup,
  TemplateOption
} from '@/types'
import { assertSuccess, type SuccessResponse, type CreateResponse } from './shared'

// ============================================
// 响应类型
// ============================================

interface SiteGroupsResponse {
  groups: SiteGroupWithStats[]
}

interface SiteGroupOptionsResponse {
  keyword_groups: KeywordGroup[]
  image_groups: ImageGroup[]
  article_groups: ArticleGroup[]
  templates: TemplateOption[]
}

// ============================================
// 站群 API
// ============================================

export async function getSiteGroups(): Promise<SiteGroup[]> {
  const res: SiteGroupsResponse = await request.get('/site-groups')
  return res.groups || []
}

export async function getSiteGroup(id: number): Promise<SiteGroupWithStats> {
  return request.get(`/site-groups/${id}`)
}

export async function getSiteGroupOptions(id: number): Promise<SiteGroupOptionsResponse> {
  return request.get(`/site-groups/${id}/options`)
}

export async function createSiteGroup(data: SiteGroupCreate): Promise<{ success: boolean; id: number }> {
  const res: CreateResponse = await request.post('/site-groups', data)
  assertSuccess(res, '创建失败')
  return { success: true, id: res.id! }
}

export async function updateSiteGroup(id: number, data: SiteGroupUpdate): Promise<{ success: boolean }> {
  const res: SuccessResponse = await request.put(`/site-groups/${id}`, data)
  assertSuccess(res, '更新失败')
  return { success: true }
}

export async function deleteSiteGroup(id: number): Promise<{ success: boolean }> {
  const res: SuccessResponse = await request.delete(`/site-groups/${id}`)
  assertSuccess(res, '删除失败')
  return { success: true }
}
