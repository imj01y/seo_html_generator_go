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

export function getDir(path: string = ''): Promise<DirContent> {
  return request.get(`/content-worker/files/${path}`)
}

export function getFile(path: string): Promise<FileContent> {
  return request.get(`/content-worker/files/${path}`)
}

export function getFileTree(): Promise<TreeNode> {
  return request.get('/content-worker/files', { params: { tree: 'true' } })
}

export async function saveFile(path: string, content: string): Promise<void> {
  await request.put(`/content-worker/files/${path}`, { content })
}

export async function createItem(parentPath: string, name: string, type: 'file' | 'dir'): Promise<void> {
  await request.post(`/content-worker/files/${parentPath}`, { name, type })
}

export async function deleteItem(path: string): Promise<void> {
  await request.delete(`/content-worker/files/${path}`)
}

export async function moveItem(oldPath: string, newPath: string): Promise<void> {
  await request.patch(`/content-worker/files/${oldPath}`, { new_path: newPath })
}

export function getDownloadUrl(path: string): string {
  const token = localStorage.getItem('token')
  return `/api/content-worker/download/${path}?token=${token}`
}
