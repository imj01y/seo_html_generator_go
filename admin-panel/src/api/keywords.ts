import request from '@/utils/request'
import type {
  KeywordGroup,
  KeywordGroupCreate,
  KeywordGroupUpdate,
  Keyword,
  KeywordBatchAdd,
  PaginatedResponse,
  BatchResult,
  GroupStats
} from '@/types'

// 后端返回格式
interface BackendGroupsResponse {
  groups: KeywordGroup[]
  error?: string
}

// 关键词分组
export const getKeywordGroups = async (siteGroupId?: number): Promise<KeywordGroup[]> => {
  const params = siteGroupId ? { site_group_id: siteGroupId } : {}
  const res: BackendGroupsResponse = await request.get('/keywords/groups', { params })
  return res.groups || []
}

export const createKeywordGroup = async (data: KeywordGroupCreate): Promise<KeywordGroup> => {
  const res: { success: boolean; id?: number; message?: string } = await request.post('/keywords/groups', data)
  if (res.success && res.id) {
    return {
      id: res.id,
      site_group_id: data.site_group_id || 1,
      name: data.name,
      description: data.description || null,
      is_default: data.is_default ? 1 : 0,
      created_at: new Date().toISOString()
    }
  }
  throw new Error(res.message || '创建失败')
}

export const updateKeywordGroup = async (id: number, data: KeywordGroupUpdate): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.put(`/keywords/groups/${id}`, data)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
}

export const deleteKeywordGroup = async (id: number): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.delete(`/keywords/groups/${id}`)
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
}

// 关键词分页列表
export const getKeywords = async (params: {
  group_id?: number
  page?: number
  page_size?: number
  search?: string
}): Promise<PaginatedResponse<Keyword>> => {
  const res: { items: Keyword[]; total: number; page: number; page_size: number } = await request.get('/keywords/list', {
    params: {
      group_id: params.group_id || 1,
      page: params.page || 1,
      page_size: params.page_size || 20,
      search: params.search
    }
  })
  return { items: res.items || [], total: res.total || 0 }
}

export const addKeyword = async (data: { group_id: number; keyword: string }): Promise<Keyword> => {
  const res: { success: boolean; id?: number; message?: string } = await request.post('/keywords/add', data)
  if (res.success && res.id) {
    return {
      id: res.id,
      group_id: data.group_id,
      keyword: data.keyword,
      status: 1,
      created_at: new Date().toISOString()
    }
  }
  throw new Error(res.message || '添加失败')
}

export const addKeywordsBatch = async (data: KeywordBatchAdd): Promise<BatchResult> => {
  const res: { success: boolean; added: number; skipped: number } = await request.post('/keywords/batch', data)
  return { added: res.added, skipped: res.skipped }
}

export const updateKeyword = async (id: number, data: { keyword?: string; group_id?: number; status?: number }): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.put(`/keywords/${id}`, data)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
}

export const deleteKeyword = async (id: number): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.delete(`/keywords/${id}`)
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
}

export const reloadKeywordGroup = async (_group_id?: number): Promise<{ total: number }> => {
  const res: { success: boolean; total: number } = await request.post('/keywords/reload')
  return { total: res.total }
}

export const clearKeywordCache = async (): Promise<{ cleared: number; message: string }> => {
  const res: { success: boolean; cleared: number; message: string } = await request.post('/keywords/cache/clear')
  if (!res.success) {
    throw new Error(res.message || '清理失败')
  }
  return { cleared: res.cleared, message: res.message }
}

export const getRandomKeywords = async (count?: number): Promise<string[]> => {
  const res: { keywords: string[]; count: number } = await request.get('/keywords/random', {
    params: { count }
  })
  return res.keywords
}

export const getKeywordStats = (): Promise<GroupStats> =>
  request.get('/keywords/stats')

// 批量操作
export const batchDeleteKeywords = async (ids: number[]): Promise<{ deleted: number }> => {
  const res: { success: boolean; deleted: number; message?: string } = await request.delete('/keywords/batch', {
    data: { ids }
  })
  if (!res.success) {
    throw new Error(res.message || '批量删除失败')
  }
  return { deleted: res.deleted }
}

export const batchUpdateKeywordStatus = async (ids: number[], status: number): Promise<{ updated: number }> => {
  const res: { success: boolean; updated: number; message?: string } = await request.put('/keywords/batch/status', {
    ids,
    status
  })
  if (!res.success) {
    throw new Error(res.message || '批量更新状态失败')
  }
  return { updated: res.updated }
}

export const batchMoveKeywords = async (ids: number[], groupId: number): Promise<{ moved: number }> => {
  const res: { success: boolean; moved: number; message?: string } = await request.put('/keywords/batch/move', {
    ids,
    group_id: groupId
  })
  if (!res.success) {
    throw new Error(res.message || '批量移动失败')
  }
  return { moved: res.moved }
}

// 上传 TXT 文件批量添加关键词
export const uploadKeywordsFile = async (file: File, groupId: number): Promise<{
  success: boolean
  message: string
  total: number
  added: number
  skipped: number
}> => {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('group_id', String(groupId))

  return await request.post('/keywords/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data'
    }
  })
}
