import request from '@/utils/request'
import type {
  ArticleGroup,
  ArticleGroupCreate,
  ArticleGroupUpdate,
  Article,
  ArticleCreate,
  ArticleUpdate,
  PaginatedResponse
} from '@/types'
import { assertSuccess, type SuccessResponse, type CreateResponse, type CountResponse } from './shared'

// ============================================
// 响应类型
// ============================================

interface GroupsResponse {
  groups: ArticleGroup[]
}

interface ArticleListResponse {
  items: Article[]
  total: number
}

// ============================================
// 分组 API
// ============================================

export async function getArticleGroups(siteGroupId?: number): Promise<ArticleGroup[]> {
  const params = siteGroupId ? { site_group_id: siteGroupId } : {}
  const res: GroupsResponse = await request.get('/articles/groups', { params })
  return res.groups || []
}

export async function createArticleGroup(data: ArticleGroupCreate): Promise<ArticleGroup> {
  const res: CreateResponse = await request.post('/articles/groups', data)
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

export async function updateArticleGroup(id: number, data: ArticleGroupUpdate): Promise<void> {
  const res: SuccessResponse = await request.put(`/articles/groups/${id}`, data)
  assertSuccess(res, '更新失败')
}

export async function deleteArticleGroup(id: number): Promise<void> {
  const res: SuccessResponse = await request.delete(`/articles/groups/${id}`)
  assertSuccess(res, '删除失败')
}

// ============================================
// 文章 API
// ============================================

export async function getArticles(params: {
  group_id?: number
  page?: number
  page_size?: number
  search?: string
}): Promise<PaginatedResponse<Article>> {
  const res: ArticleListResponse = await request.get('/articles/list', {
    params: {
      group_id: params.group_id || 1,
      page: params.page || 1,
      page_size: params.page_size || 20,
      search: params.search
    }
  })
  return { items: res.items || [], total: res.total || 0 }
}

export async function getArticle(id: number): Promise<Article> {
  return request.get(`/articles/${id}`)
}

export async function createArticle(data: ArticleCreate): Promise<Article> {
  const res: CreateResponse = await request.post('/articles/add', {
    group_id: data.group_id,
    title: data.title,
    content: data.content
  })
  assertSuccess(res, '添加失败')
  const now = new Date().toISOString()
  return {
    id: res.id!,
    group_id: data.group_id,
    title: data.title,
    content: data.content,
    status: 1,
    created_at: now,
    updated_at: now
  }
}

export async function updateArticle(id: number, data: ArticleUpdate): Promise<Article> {
  const updateData: Record<string, unknown> = {}
  if (data.group_id !== undefined) updateData.group_id = data.group_id
  if (data.title !== undefined) updateData.title = data.title
  if (data.content !== undefined) updateData.content = data.content
  if (data.status !== undefined) updateData.status = data.status

  const res: SuccessResponse = await request.put(`/articles/${id}`, updateData)
  assertSuccess(res, '更新失败')
  return { id, ...data } as Article
}

export async function deleteArticle(id: number): Promise<void> {
  const res: SuccessResponse = await request.delete(`/articles/${id}`)
  assertSuccess(res, '删除失败')
}

// ============================================
// 批量操作 API
// ============================================

export async function batchDeleteArticles(ids: number[]): Promise<{ deleted: number }> {
  const res: CountResponse = await request.delete('/articles/batch/delete', { data: { ids } })
  assertSuccess(res, '批量删除失败')
  return { deleted: res.deleted! }
}

export async function batchUpdateArticleStatus(ids: number[], status: number): Promise<{ updated: number }> {
  const res: CountResponse = await request.put('/articles/batch/status', { ids, status })
  assertSuccess(res, '批量更新状态失败')
  return { updated: res.updated! }
}

export async function batchMoveArticles(ids: number[], groupId: number): Promise<{ moved: number }> {
  const res: CountResponse = await request.put('/articles/batch/move', { ids, group_id: groupId })
  assertSuccess(res, '批量移动失败')
  return { moved: res.moved! }
}

export async function deleteAllArticles(groupId?: number): Promise<{ deleted: number }> {
  const res: CountResponse = await request.delete('/articles/delete-all', {
    data: { group_id: groupId, confirm: true }
  })
  assertSuccess(res, '删除失败')
  return { deleted: res.deleted! }
}
