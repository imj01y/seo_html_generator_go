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
import { assertSuccess, type SuccessResponse, type CreateResponse, type CountResponse } from './shared'

// ============================================
// 响应类型
// ============================================

interface GroupsResponse {
  groups: KeywordGroup[]
}

interface KeywordListResponse {
  items: Keyword[]
  total: number
}

// ============================================
// 分组 API
// ============================================

export async function getKeywordGroups(siteGroupId?: number): Promise<KeywordGroup[]> {
  const params = siteGroupId ? { site_group_id: siteGroupId } : {}
  const res: GroupsResponse = await request.get('/keywords/groups', { params })
  return res.groups || []
}

export async function createKeywordGroup(data: KeywordGroupCreate): Promise<KeywordGroup> {
  const res: CreateResponse = await request.post('/keywords/groups', data)
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

export async function updateKeywordGroup(id: number, data: KeywordGroupUpdate): Promise<void> {
  const res: SuccessResponse = await request.put(`/keywords/groups/${id}`, data)
  assertSuccess(res, '更新失败')
}

export async function deleteKeywordGroup(id: number): Promise<void> {
  const res: SuccessResponse = await request.delete(`/keywords/groups/${id}`)
  assertSuccess(res, '删除失败')
}

// ============================================
// 关键词 API
// ============================================

export async function getKeywords(params: {
  group_id?: number
  page?: number
  page_size?: number
  search?: string
}): Promise<PaginatedResponse<Keyword>> {
  const res: KeywordListResponse = await request.get('/keywords/list', {
    params: {
      group_id: params.group_id || 1,
      page: params.page || 1,
      page_size: params.page_size || 20,
      search: params.search
    }
  })
  return { items: res.items || [], total: res.total || 0 }
}

export async function addKeyword(data: { group_id: number; keyword: string }): Promise<Keyword> {
  const res: CreateResponse = await request.post('/keywords/add', data)
  assertSuccess(res, '添加失败')
  return {
    id: res.id!,
    group_id: data.group_id,
    keyword: data.keyword,
    status: 1,
    created_at: new Date().toISOString()
  }
}

export async function addKeywordsBatch(data: KeywordBatchAdd): Promise<BatchResult> {
  const res: { success: boolean; added: number; skipped: number } = await request.post('/keywords/batch', data)
  return { added: res.added, skipped: res.skipped }
}

export async function updateKeyword(
  id: number,
  data: { keyword?: string; group_id?: number; status?: number }
): Promise<void> {
  const res: SuccessResponse = await request.put(`/keywords/${id}`, data)
  assertSuccess(res, '更新失败')
}

export async function deleteKeyword(id: number): Promise<void> {
  const res: SuccessResponse = await request.delete(`/keywords/${id}`)
  assertSuccess(res, '删除失败')
}

// ============================================
// 缓存操作 API
// ============================================

export async function reloadKeywordGroup(_group_id?: number): Promise<{ total: number }> {
  const res: { success: boolean; total: number } = await request.post('/keywords/reload')
  return { total: res.total }
}

export async function clearKeywordCache(): Promise<{ cleared: number; message: string }> {
  const res: { success: boolean; cleared: number; message: string } = await request.post('/keywords/cache/clear')
  assertSuccess(res, '清理失败')
  return { cleared: res.cleared, message: res.message }
}

export async function getRandomKeywords(count?: number): Promise<string[]> {
  const res: { keywords: string[] } = await request.get('/keywords/random', { params: { count } })
  return res.keywords
}

export function getKeywordStats(): Promise<GroupStats> {
  return request.get('/keywords/stats')
}

// ============================================
// 批量操作 API
// ============================================

export async function batchDeleteKeywords(ids: number[]): Promise<{ deleted: number }> {
  const res: CountResponse = await request.delete('/keywords/batch', { data: { ids } })
  assertSuccess(res, '批量删除失败')
  return { deleted: res.deleted! }
}

export async function batchUpdateKeywordStatus(ids: number[], status: number): Promise<{ updated: number }> {
  const res: CountResponse = await request.put('/keywords/batch/status', { ids, status })
  assertSuccess(res, '批量更新状态失败')
  return { updated: res.updated! }
}

export async function batchMoveKeywords(ids: number[], groupId: number): Promise<{ moved: number }> {
  const res: CountResponse = await request.put('/keywords/batch/move', { ids, group_id: groupId })
  assertSuccess(res, '批量移动失败')
  return { moved: res.moved! }
}

export async function deleteAllKeywords(groupId?: number): Promise<{ deleted: number }> {
  const res: CountResponse = await request.delete('/keywords/delete-all', {
    data: { group_id: groupId, confirm: true }
  })
  assertSuccess(res, '删除失败')
  return { deleted: res.deleted! }
}

// ============================================
// 文件上传 API
// ============================================

export async function uploadKeywordsFile(
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

  return request.post('/keywords/upload', formData, {
    timeout: 300000,
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}
