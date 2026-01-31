import request from '@/utils/request'

// ============================================
// 类型定义
// ============================================

export interface FileInfo {
  name: string
  type: 'file' | 'dir'
  size?: number
  mtime: string
}

export interface FileContent {
  path: string
  content: string
  size: number
  mtime: string
}

export interface DirContent {
  path: string
  files: FileInfo[]
}

export interface TreeNode {
  name: string
  path: string
  type: 'file' | 'dir'
  children?: TreeNode[]
}

// ============================================
// 文件操作 API
// ============================================

/**
 * 获取目录内容
 */
export const getDir = async (path: string = ''): Promise<DirContent> => {
  return request.get(`/content-worker/files/${path}`)
}

/**
 * 获取文件内容
 */
export const getFile = async (path: string): Promise<FileContent> => {
  return request.get(`/content-worker/files/${path}`)
}

/**
 * 获取目录树
 */
export const getFileTree = async (): Promise<TreeNode> => {
  return request.get('/content-worker/files', { params: { tree: 'true' } })
}

/**
 * 保存文件
 */
export const saveFile = async (path: string, content: string): Promise<void> => {
  await request.put(`/content-worker/files/${path}`, { content })
}

/**
 * 创建文件或目录
 */
export const createItem = async (
  parentPath: string,
  name: string,
  type: 'file' | 'dir'
): Promise<void> => {
  await request.post(`/content-worker/files/${parentPath}`, { name, type })
}

/**
 * 删除文件或目录
 */
export const deleteItem = async (path: string): Promise<void> => {
  await request.delete(`/content-worker/files/${path}`)
}

/**
 * 移动/重命名
 */
export const moveItem = async (oldPath: string, newPath: string): Promise<void> => {
  await request.patch(`/content-worker/files/${oldPath}`, { new_path: newPath })
}

/**
 * 获取下载 URL
 */
export const getDownloadUrl = (path: string): string => {
  const token = localStorage.getItem('token')
  return `/api/content-worker/download/${path}?token=${token}`
}
