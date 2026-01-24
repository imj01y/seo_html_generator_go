import request from '@/utils/request'
import type {
  Template,
  TemplateListItem,
  TemplateCreate,
  TemplateUpdate,
  TemplateOption,
  PaginatedResponse,
  Site
} from '@/types'

// 后端返回格式
interface TemplateListResponse {
  items: TemplateListItem[]
  total: number
  page: number
  page_size: number
}

interface TemplateOptionsResponse {
  options: TemplateOption[]
}

interface TemplateSitesResponse {
  sites: Site[]
  template_name: string
}

/**
 * 获取模板列表（不含content）
 */
export const getTemplates = async (params?: {
  page?: number
  page_size?: number
  status?: number
}): Promise<PaginatedResponse<TemplateListItem>> => {
  const res: TemplateListResponse = await request.get('/templates', { params })
  return {
    items: res.items || [],
    total: res.total
  }
}

/**
 * 获取模板下拉选项
 */
export const getTemplateOptions = async (): Promise<TemplateOption[]> => {
  const res: TemplateOptionsResponse = await request.get('/templates/options')
  return res.options || []
}

/**
 * 获取模板详情（含content）
 */
export const getTemplate = async (id: number): Promise<Template> => {
  return await request.get(`/templates/${id}`)
}

/**
 * 获取使用此模板的站点
 */
export const getTemplateSites = async (id: number): Promise<TemplateSitesResponse> => {
  return await request.get(`/templates/${id}/sites`)
}

/**
 * 创建模板
 */
export const createTemplate = async (data: TemplateCreate): Promise<{ success: boolean; id?: number; message?: string }> => {
  return await request.post('/templates', data)
}

/**
 * 更新模板
 */
export const updateTemplate = async (id: number, data: TemplateUpdate): Promise<{ success: boolean; message?: string }> => {
  return await request.put(`/templates/${id}`, data)
}

/**
 * 删除模板
 */
export const deleteTemplate = async (id: number): Promise<{ success: boolean; message?: string }> => {
  return await request.delete(`/templates/${id}`)
}
