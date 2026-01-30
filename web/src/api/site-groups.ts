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

// 后端返回格式
interface SiteGroupsResponse {
  items: SiteGroup[]
  total: number
}

interface SiteGroupOptionsResponse {
  keyword_groups: KeywordGroup[]
  image_groups: ImageGroup[]
  article_groups: ArticleGroup[]
  templates: TemplateOption[]
}

// 获取站群列表
export const getSiteGroups = async (): Promise<SiteGroup[]> => {
  const res: SiteGroupsResponse = await request.get('/site-groups')
  return res.items || []
}

// 获取单个站群详情（含统计信息）
export const getSiteGroup = async (id: number): Promise<SiteGroupWithStats> => {
  return await request.get(`/site-groups/${id}`)
}

// 获取站群下的所有资源选项（用于站点配置）
export const getSiteGroupOptions = async (id: number): Promise<SiteGroupOptionsResponse> => {
  return await request.get(`/site-groups/${id}/options`)
}

// 创建站群
export const createSiteGroup = async (data: SiteGroupCreate): Promise<{ success: boolean; id: number }> => {
  const res: { success: boolean; id?: number; message?: string } = await request.post('/site-groups', data)
  if (res.success && res.id) {
    return { success: true, id: res.id }
  }
  throw new Error(res.message || '创建失败')
}

// 更新站群
export const updateSiteGroup = async (id: number, data: SiteGroupUpdate): Promise<{ success: boolean }> => {
  const res: { success: boolean; message?: string } = await request.put(`/site-groups/${id}`, data)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
  return { success: true }
}

// 删除站群（软删除）
export const deleteSiteGroup = async (id: number): Promise<{ success: boolean }> => {
  const res: { success: boolean; message?: string } = await request.delete(`/site-groups/${id}`)
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
  return { success: true }
}
