import request from '@/utils/request'
import type {
  ContentGenerator,
  GeneratorCreate,
  GeneratorUpdate,
  GeneratorQuery,
  CodeTemplate,
  TestCodeRequest,
  TestCodeResult
} from '@/types'

// 响应类型
interface GeneratorListResponse {
  success: boolean
  data: ContentGenerator[]
  total: number
  page: number
  page_size: number
}

interface GeneratorResponse {
  success: boolean
  data: ContentGenerator
}

interface MutationResponse {
  success: boolean
  id?: number
  message?: string
}

interface ToggleResponse {
  success: boolean
  enabled: number
  message?: string
}

interface TemplatesResponse {
  success: boolean
  data: CodeTemplate[]
}

/**
 * 获取生成器列表
 */
export const getGenerators = async (params?: GeneratorQuery) => {
  const res: GeneratorListResponse = await request.get('/generators/', { params })
  return {
    items: res.data || [],
    total: res.total
  }
}

/**
 * 获取生成器详情
 */
export const getGenerator = async (id: number): Promise<ContentGenerator> => {
  const res: GeneratorResponse = await request.get(`/generators/${id}`)
  return res.data
}

/**
 * 创建生成器
 */
export const createGenerator = async (data: GeneratorCreate): Promise<MutationResponse> => {
  return await request.post('/generators/', data)
}

/**
 * 更新生成器
 */
export const updateGenerator = async (
  id: number,
  data: GeneratorUpdate
): Promise<MutationResponse> => {
  return await request.put(`/generators/${id}`, data)
}

/**
 * 删除生成器
 */
export const deleteGenerator = async (id: number): Promise<MutationResponse> => {
  return await request.delete(`/generators/${id}`)
}

/**
 * 切换启用状态
 */
export const toggleGenerator = async (id: number): Promise<ToggleResponse> => {
  return await request.post(`/generators/${id}/toggle`)
}

/**
 * 设为默认生成器
 */
export const setDefaultGenerator = async (id: number): Promise<MutationResponse> => {
  return await request.post(`/generators/${id}/set-default`)
}

/**
 * 热重载生成器
 */
export const reloadGenerator = async (id: number): Promise<MutationResponse> => {
  return await request.post(`/generators/${id}/reload`)
}

/**
 * 测试代码
 */
export const testGeneratorCode = async (data: TestCodeRequest): Promise<TestCodeResult> => {
  return await request.post('/generators/test', data)
}

/**
 * 获取代码模板列表
 */
export const getCodeTemplates = async (): Promise<CodeTemplate[]> => {
  const res: TemplatesResponse = await request.get('/generators/templates/list')
  return res.data || []
}
