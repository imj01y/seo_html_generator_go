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
import type { SuccessResponse } from './shared'

// ============================================
// 响应类型
// ============================================

interface TemplateListResponse {
  items: TemplateListItem[]
  total: number
}

interface TemplateOptionsResponse {
  options: TemplateOption[]
}

interface TemplateSitesResponse {
  sites: Site[]
  template_name: string
}

interface CreateTemplateResponse extends SuccessResponse {
  id?: number
}

interface ClearCacheResponse extends SuccessResponse {
  html_cleared?: number
}

interface ReloadCacheResponse extends SuccessResponse {
  stats?: { item_count: number }
}

// ============================================
// 模板 API
// ============================================

export async function getTemplates(params?: {
  page?: number
  page_size?: number
  status?: number
  site_group_id?: number
}): Promise<PaginatedResponse<TemplateListItem>> {
  const res: TemplateListResponse = await request.get('/templates', { params })
  return {
    items: res.items || [],
    total: res.total
  }
}

export async function getTemplateOptions(siteGroupId?: number): Promise<TemplateOption[]> {
  const params = siteGroupId ? { site_group_id: siteGroupId } : {}
  const res: TemplateOptionsResponse = await request.get('/templates/options', { params })
  return res.options || []
}

export async function getTemplate(id: number): Promise<Template> {
  return request.get(`/templates/${id}`)
}

export async function getTemplateSites(id: number): Promise<TemplateSitesResponse> {
  return request.get(`/templates/${id}/sites`)
}

export async function createTemplate(data: TemplateCreate): Promise<CreateTemplateResponse> {
  return request.post('/templates', data)
}

export async function updateTemplate(id: number, data: TemplateUpdate): Promise<SuccessResponse> {
  return request.put(`/templates/${id}`, data)
}

export async function deleteTemplate(id: number): Promise<SuccessResponse> {
  return request.delete(`/templates/${id}`)
}

// ============================================
// Go 服务缓存 API
// ============================================

export async function clearGoTemplateCache(): Promise<ClearCacheResponse> {
  try {
    const response = await fetch('/go/api/cache/template/clear', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    return response.json()
  } catch (error) {
    console.warn('Failed to clear Go template cache:', error)
    return { success: false, message: '无法连接Go服务' }
  }
}

export async function reloadGoTemplateCache(): Promise<ReloadCacheResponse> {
  try {
    const response = await fetch('/go/api/cache/template/reload', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    return response.json()
  } catch (error) {
    console.warn('Failed to reload Go template cache:', error)
    return { success: false, message: '无法连接Go服务' }
  }
}
