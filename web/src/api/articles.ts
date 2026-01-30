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

// 后端返回格式
interface BackendGroupsResponse {
  groups: ArticleGroup[]
  error?: string
}

// 文章分组
export const getArticleGroups = async (siteGroupId?: number): Promise<ArticleGroup[]> => {
  const params = siteGroupId ? { site_group_id: siteGroupId } : {}
  const res: BackendGroupsResponse = await request.get('/articles/groups', { params })
  return res.groups || []
}

export const createArticleGroup = async (data: ArticleGroupCreate): Promise<ArticleGroup> => {
  const res: { success: boolean; id?: number; message?: string } = await request.post('/articles/groups', data)
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

export const updateArticleGroup = async (id: number, data: ArticleGroupUpdate): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.put(`/articles/groups/${id}`, data)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
}

export const deleteArticleGroup = async (id: number): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.delete(`/articles/groups/${id}`)
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
}

// 文章分页列表
export const getArticles = async (params: {
  group_id?: number
  page?: number
  page_size?: number
  search?: string
}): Promise<PaginatedResponse<Article>> => {
  const res: { items: Article[]; total: number; page: number; page_size: number } = await request.get('/articles/list', {
    params: {
      group_id: params.group_id || 1,
      page: params.page || 1,
      page_size: params.page_size || 20,
      search: params.search
    }
  })
  return { items: res.items || [], total: res.total || 0 }
}

export const getArticle = async (id: number): Promise<Article> => {
  return await request.get(`/articles/${id}`)
}

export const createArticle = async (data: ArticleCreate): Promise<Article> => {
  const res: { success: boolean; id?: number; message?: string } = await request.post('/articles/add', {
    group_id: data.group_id,
    title: data.title,
    content: data.content
  })
  if (res.success && res.id) {
    return {
      id: res.id,
      group_id: data.group_id,
      title: data.title,
      content: data.content,
      status: 1,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    }
  }
  throw new Error(res.message || '添加失败')
}

export const updateArticle = async (id: number, data: ArticleUpdate): Promise<Article> => {
  const updateData: Record<string, unknown> = {}
  if (data.group_id !== undefined) updateData.group_id = data.group_id
  if (data.title !== undefined) updateData.title = data.title
  if (data.content !== undefined) updateData.content = data.content
  if (data.status !== undefined) updateData.status = data.status

  const res: { success: boolean; message?: string } = await request.put(`/articles/${id}`, updateData)
  if (!res.success) {
    throw new Error(res.message || '更新失败')
  }
  return { id, ...data } as Article
}

export const deleteArticle = async (id: number): Promise<void> => {
  const res: { success: boolean; message?: string } = await request.delete(`/articles/${id}`)
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
}

// 批量操作
export const batchDeleteArticles = async (ids: number[]): Promise<{ deleted: number }> => {
  const res: { success: boolean; deleted: number; message?: string } = await request.delete('/articles/batch/delete', {
    data: { ids }
  })
  if (!res.success) {
    throw new Error(res.message || '批量删除失败')
  }
  return { deleted: res.deleted }
}

export const batchUpdateArticleStatus = async (ids: number[], status: number): Promise<{ updated: number }> => {
  const res: { success: boolean; updated: number; message?: string } = await request.put('/articles/batch/status', {
    ids,
    status
  })
  if (!res.success) {
    throw new Error(res.message || '批量更新状态失败')
  }
  return { updated: res.updated }
}

export const batchMoveArticles = async (ids: number[], groupId: number): Promise<{ moved: number }> => {
  const res: { success: boolean; moved: number; message?: string } = await request.put('/articles/batch/move', {
    ids,
    group_id: groupId
  })
  if (!res.success) {
    throw new Error(res.message || '批量移动失败')
  }
  return { moved: res.moved }
}

// 删除全部文章
export const deleteAllArticles = async (groupId?: number): Promise<{ deleted: number }> => {
  const res: { success: boolean; deleted: number; message?: string } = await request.delete('/articles/delete-all', {
    data: { group_id: groupId, confirm: true }
  })
  if (!res.success) {
    throw new Error(res.message || '删除失败')
  }
  return { deleted: res.deleted }
}

