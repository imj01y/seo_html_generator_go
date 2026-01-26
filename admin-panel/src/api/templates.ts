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
  site_group_id?: number
}): Promise<PaginatedResponse<TemplateListItem>> => {
  const res: TemplateListResponse = await request.get('/templates', { params })
  return {
    items: res.items || [],
    total: res.total
  }
}

/**
 * 获取模板下拉选项
 * @param siteGroupId 可选的站群ID过滤
 */
export const getTemplateOptions = async (siteGroupId?: number): Promise<TemplateOption[]> => {
  const params = siteGroupId ? { site_group_id: siteGroupId } : {}
  const res: TemplateOptionsResponse = await request.get('/templates/options', { params })
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

/**
 * 清除Go服务的模板缓存
 * 模板内容更新后调用此接口使新模板生效
 */
export const clearGoTemplateCache = async (): Promise<{ success: boolean; html_cleared?: number; message?: string }> => {
  try {
    // Go服务地址，可根据环境配置
    const goServerUrl = import.meta.env.VITE_GO_SERVER_URL || 'http://127.0.0.1:8081'
    const response = await fetch(`${goServerUrl}/api/cache/template/clear`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    return await response.json()
  } catch (error) {
    console.warn('Failed to clear Go template cache:', error)
    return { success: false, message: '无法连接Go服务' }
  }
}
