import request from '@/utils/request'
import type {
  ImageGroup,
  ImageGroupCreate,
  ImageGroupUpdate,
  ImageUrl,
  ImageUrlBatchAdd,
  PaginatedResponse,
  BatchResult,
  GroupStats
} from '@/types'

// 后端返回格式
interface BackendGroupsResponse {
  groups: ImageGroup[]
  error?: string
}

// 图片分组
export const getImageGroups = async (siteGroupId?: number): Promise<ImageGroup[]> => {
  const params = siteGroupId ? { site_group_id: siteGroupId } : {}
  const res: BackendGroupsResponse = await request.get('/images/groups', { params })
  return res.groups || []
}

export const createImageGroup = async (data: ImageGroupCreate): Promise<ImageGroup> => {
  const res: { success: boolean; id?: number; message?: string } = await request.post('/images/groups', data)
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

export const updateImageGroup = async (id: number, data: ImageGroupUpdate): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.put(`/images/groups/${id}`, data)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
}

export const deleteImageGroup = async (id: number): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.delete(`/images/groups/${id}`)
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
}

// 图片URL分页列表
export const getImageUrls = async (params: {
  group_id?: number
  page?: number
  page_size?: number
  search?: string
}): Promise<PaginatedResponse<ImageUrl>> => {
  const res: { items: ImageUrl[]; total: number; page: number; page_size: number } = await request.get('/images/urls/list', {
    params: {
      group_id: params.group_id || 1,
      page: params.page || 1,
      page_size: params.page_size || 20,
      search: params.search
    }
  })
  return { items: res.items || [], total: res.total || 0 }
}

export const addImageUrl = async (data: { group_id: number; url: string }): Promise<ImageUrl> => {
  const res: { success: boolean; id?: number; message?: string } = await request.post('/images/urls/add', data)
  if (res.success && res.id) {
    return {
      id: res.id,
      group_id: data.group_id,
      url: data.url,
      status: 1,
      created_at: new Date().toISOString()
    }
  }
  throw new Error(res.message || '添加失败')
}

export const addImageUrlsBatch = async (data: ImageUrlBatchAdd): Promise<BatchResult> => {
  const res: { success: boolean; added: number; skipped: number } = await request.post('/images/urls/batch', data)
  return { added: res.added, skipped: res.skipped }
}

export const updateImageUrl = async (id: number, data: { url?: string; group_id?: number; status?: number }): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.put(`/images/urls/${id}`, data)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
}

export const deleteImageUrl = async (id: number): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.delete(`/images/urls/${id}`)
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
}

export const reloadImageGroup = async (_group_id?: number): Promise<{ total: number }> => {
  const res: { success: boolean; total: number } = await request.post('/images/urls/reload')
  return { total: res.total }
}

export const clearImageCache = async (): Promise<{ cleared: number; message: string }> => {
  const res: { success: boolean; cleared: number; message: string } = await request.post('/images/cache/clear')
  if (!res.success) {
    throw new Error(res.message || '清理失败')
  }
  return { cleared: res.cleared, message: res.message }
}

export const getRandomImageUrls = async (count?: number): Promise<string[]> => {
  const res: { urls: string[]; count: number } = await request.get('/images/urls/random', {
    params: { count }
  })
  return res.urls
}

export const getImageStats = (): Promise<GroupStats> =>
  request.get('/images/urls/stats')

// 批量操作
export const batchDeleteImages = async (ids: number[]): Promise<{ deleted: number }> => {
  const res: { success: boolean; deleted: number; message?: string } = await request.delete('/images/batch', {
    data: { ids }
  })
  if (!res.success) {
    throw new Error(res.message || '批量删除失败')
  }
  return { deleted: res.deleted }
}

export const batchUpdateImageStatus = async (ids: number[], status: number): Promise<{ updated: number }> => {
  const res: { success: boolean; updated: number; message?: string } = await request.put('/images/batch/status', {
    ids,
    status
  })
  if (!res.success) {
    throw new Error(res.message || '批量更新状态失败')
  }
  return { updated: res.updated }
}

export const batchMoveImages = async (ids: number[], groupId: number): Promise<{ moved: number }> => {
  const res: { success: boolean; moved: number; message?: string } = await request.put('/images/batch/move', {
    ids,
    group_id: groupId
  })
  if (!res.success) {
    throw new Error(res.message || '批量移动失败')
  }
  return { moved: res.moved }
}

// 删除全部图片
export const deleteAllImages = async (groupId?: number): Promise<{ deleted: number }> => {
  const res: { success: boolean; deleted: number; message?: string } = await request.delete('/images/delete-all', {
    data: { group_id: groupId, confirm: true }
  })
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
  return { deleted: res.deleted }
}
