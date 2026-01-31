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
import { assertSuccess, type SuccessResponse, type CreateResponse, type CountResponse } from './shared'

// ============================================
// 响应类型
// ============================================

interface GroupsResponse {
  groups: ImageGroup[]
}

interface ImageUrlListResponse {
  items: ImageUrl[]
  total: number
}

// ============================================
// 分组 API
// ============================================

export async function getImageGroups(siteGroupId?: number): Promise<ImageGroup[]> {
  const params = siteGroupId ? { site_group_id: siteGroupId } : {}
  const res: GroupsResponse = await request.get('/images/groups', { params })
  return res.groups || []
}

export async function createImageGroup(data: ImageGroupCreate): Promise<ImageGroup> {
  const res: CreateResponse = await request.post('/images/groups', data)
  assertSuccess(res, '创建失败')
  return {
    id: res.id!,
    site_group_id: data.site_group_id || 1,
    name: data.name,
    description: data.description || null,
    is_default: data.is_default ? 1 : 0,
    created_at: new Date().toISOString()
  }
}

export async function updateImageGroup(id: number, data: ImageGroupUpdate): Promise<void> {
  const res: SuccessResponse = await request.put(`/images/groups/${id}`, data)
  assertSuccess(res, '更新失败')
}

export async function deleteImageGroup(id: number): Promise<void> {
  const res: SuccessResponse = await request.delete(`/images/groups/${id}`)
  assertSuccess(res, '删除失败')
}

// ============================================
// 图片 URL API
// ============================================

export async function getImageUrls(params: {
  group_id?: number
  page?: number
  page_size?: number
  search?: string
}): Promise<PaginatedResponse<ImageUrl>> {
  const res: ImageUrlListResponse = await request.get('/images/urls/list', {
    params: {
      group_id: params.group_id || 1,
      page: params.page || 1,
      page_size: params.page_size || 20,
      search: params.search
    }
  })
  return { items: res.items || [], total: res.total || 0 }
}

export async function addImageUrl(data: { group_id: number; url: string }): Promise<ImageUrl> {
  const res: CreateResponse = await request.post('/images/urls/add', data)
  assertSuccess(res, '添加失败')
  return {
    id: res.id!,
    group_id: data.group_id,
    url: data.url,
    status: 1,
    created_at: new Date().toISOString()
  }
}

export async function addImageUrlsBatch(data: ImageUrlBatchAdd): Promise<BatchResult> {
  const res: { success: boolean; added: number; skipped: number } = await request.post('/images/urls/batch', data)
  return { added: res.added, skipped: res.skipped }
}

export async function updateImageUrl(
  id: number,
  data: { url?: string; group_id?: number; status?: number }
): Promise<void> {
  const res: SuccessResponse = await request.put(`/images/urls/${id}`, data)
  assertSuccess(res, '更新失败')
}

export async function deleteImageUrl(id: number): Promise<void> {
  const res: SuccessResponse = await request.delete(`/images/urls/${id}`)
  assertSuccess(res, '删除失败')
}

// ============================================
// 缓存操作 API
// ============================================

export async function reloadImageGroup(_group_id?: number): Promise<{ total: number }> {
  const res: { success: boolean; total: number } = await request.post('/images/urls/reload')
  return { total: res.total }
}

export async function clearImageCache(): Promise<{ cleared: number; message: string }> {
  const res: { success: boolean; cleared: number; message: string } = await request.post('/images/cache/clear')
  assertSuccess(res, '清理失败')
  return { cleared: res.cleared, message: res.message }
}

export async function getRandomImageUrls(count?: number): Promise<string[]> {
  const res: { urls: string[] } = await request.get('/images/urls/random', { params: { count } })
  return res.urls
}

export function getImageStats(): Promise<GroupStats> {
  return request.get('/images/urls/stats')
}

// ============================================
// 批量操作 API
// ============================================

export async function batchDeleteImages(ids: number[]): Promise<{ deleted: number }> {
  const res: CountResponse = await request.delete('/images/batch', { data: { ids } })
  assertSuccess(res, '批量删除失败')
  return { deleted: res.deleted! }
}

export async function batchUpdateImageStatus(ids: number[], status: number): Promise<{ updated: number }> {
  const res: CountResponse = await request.put('/images/batch/status', { ids, status })
  assertSuccess(res, '批量更新状态失败')
  return { updated: res.updated! }
}

export async function batchMoveImages(ids: number[], groupId: number): Promise<{ moved: number }> {
  const res: CountResponse = await request.put('/images/batch/move', { ids, group_id: groupId })
  assertSuccess(res, '批量移动失败')
  return { moved: res.moved! }
}

export async function deleteAllImages(groupId?: number): Promise<{ deleted: number }> {
  const res: CountResponse = await request.delete('/images/delete-all', {
    data: { group_id: groupId, confirm: true }
  })
  assertSuccess(res, '删除失败')
  return { deleted: res.deleted! }
}

// ============================================
// 文件上传 API
// ============================================

export async function uploadImagesFile(
  file: File,
  groupId: number
): Promise<{
  success: boolean
  message: string
  total: number
  added: number
  skipped: number
}> {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('group_id', String(groupId))

  return request.post('/images/upload', formData, {
    timeout: 300000,
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}
