import request from '@/utils/request'
import type { SpiderStatsOverview, SpiderChartDataPoint, SpiderStatsByProject } from '@/types'
import { buildWsUrl, closeWebSocket, type SuccessResponse } from './shared'

// ============================================
// 类型定义
// ============================================

export interface SpiderTreeNode {
  name: string
  path: string
  type: 'file' | 'dir'
  children?: SpiderTreeNode[]
}

export interface SpiderProject {
  id: number
  name: string
  description?: string
  entry_file: string
  entry_function: string
  start_url?: string
  config?: Record<string, unknown>
  concurrency: number
  output_group_id: number
  schedule?: string
  enabled: number
  status: 'idle' | 'running' | 'error'
  last_run_at?: string
  last_run_duration?: number
  last_run_items?: number
  last_error?: string
  total_runs: number
  total_items: number
  created_at: string
  updated_at: string
  // 前端运行时属性
  toggling?: boolean
}

export interface ProjectFile {
  id: number
  filename: string
  path?: string  // 文件路径（后端返回）
  type?: string  // 文件类型：file 或 directory
  content: string
  created_at: string
  updated_at: string
}

export interface ProjectCreate {
  name: string
  description?: string
  entry_file?: string
  entry_function?: string
  start_url?: string
  config?: Record<string, unknown>
  concurrency?: number
  output_group_id?: number
  schedule?: string
  enabled?: number
  files?: { filename: string; content: string }[]
}

export interface ProjectUpdate {
  name?: string
  description?: string
  entry_file?: string
  entry_function?: string
  start_url?: string
  config?: Record<string, unknown>
  concurrency?: number
  output_group_id?: number
  schedule?: string
  enabled?: number
}

export interface ProjectQuery {
  status?: string
  enabled?: number
  search?: string
  page?: number
  page_size?: number
}

export interface CodeTemplate {
  name: string
  display_name: string
  description: string
  code: string
  extra_files?: { filename: string; content: string }[]
}

// ============================================
// 响应类型
// ============================================

interface MutationResponse extends SuccessResponse {
  id?: number
}

interface ToggleResponse extends SuccessResponse {
  enabled: number
}

interface TestStartResponse extends SuccessResponse {
  session_id?: string
}

// ============================================
// 项目管理 API
// ============================================

export async function getProjects(params?: ProjectQuery): Promise<{ items: SpiderProject[]; total: number }> {
  const res: { data: SpiderProject[]; total: number } = await request.get('/spider-projects', { params })
  return { items: res.data || [], total: res.total }
}

export async function getProject(id: number): Promise<SpiderProject> {
  const res: { data: SpiderProject } = await request.get(`/spider-projects/${id}`)
  return res.data
}

export function createProject(data: ProjectCreate): Promise<MutationResponse> {
  return request.post('/spider-projects', data)
}

export function updateProject(id: number, data: ProjectUpdate): Promise<MutationResponse> {
  return request.put(`/spider-projects/${id}`, data)
}

export function deleteProject(id: number): Promise<MutationResponse> {
  return request.delete(`/spider-projects/${id}`)
}

export function toggleProject(id: number): Promise<ToggleResponse> {
  return request.post(`/spider-projects/${id}/toggle`)
}

export function runProject(id: number): Promise<MutationResponse> {
  return request.post(`/spider-projects/${id}/run`)
}

export function stopProject(id: number): Promise<MutationResponse> {
  return request.post(`/spider-projects/${id}/stop`)
}

export function resetProject(id: number): Promise<MutationResponse> {
  return request.post(`/spider-projects/${id}/reset`)
}

export function testProject(id: number, maxItems: number = 0): Promise<TestStartResponse> {
  return request.post(`/spider-projects/${id}/test`, null, { params: { max_items: maxItems } })
}

export function stopTestProject(id: number): Promise<MutationResponse> {
  return request.post(`/spider-projects/${id}/test/stop`)
}

// ============================================
// 项目文件 API
// ============================================

export async function getProjectFiles(projectId: number): Promise<ProjectFile[]> {
  const res: { data: ProjectFile[] } = await request.get(`/spider-projects/${projectId}/files`)
  return res.data || []
}

export async function getProjectFile(projectId: number, filename: string): Promise<ProjectFile> {
  const res: { data: ProjectFile } = await request.get(`/spider-projects/${projectId}/files/${filename}`)
  return res.data
}

export function createProjectFile(
  projectId: number,
  data: { filename: string; content: string }
): Promise<MutationResponse> {
  return request.post(`/spider-projects/${projectId}/files`, data)
}

export function updateProjectFile(projectId: number, filename: string, content: string): Promise<MutationResponse> {
  return request.put(`/spider-projects/${projectId}/files/${filename}`, { content })
}

export function deleteProjectFile(projectId: number, filename: string): Promise<MutationResponse> {
  return request.delete(`/spider-projects/${projectId}/files/${filename}`)
}

// ============================================
// 树形文件操作 API
// ============================================

function cleanPath(path: string): string {
  return path.startsWith('/') ? path.slice(1) : path
}

export async function getProjectFileTree(projectId: number): Promise<SpiderTreeNode> {
  const res = await request.get(`/spider-projects/${projectId}/files`, { params: { tree: 'true' } })
  return res.data
}

export async function getProjectFileByPath(projectId: number, path: string): Promise<{ content: string }> {
  const res = await request.get(`/spider-projects/${projectId}/files/${cleanPath(path)}`)
  return res.data
}

export async function saveProjectFileByPath(projectId: number, path: string, content: string): Promise<void> {
  await request.put(`/spider-projects/${projectId}/files/${cleanPath(path)}`, { content })
}

export async function createProjectItem(
  projectId: number,
  parentPath: string,
  name: string,
  type: 'file' | 'dir'
): Promise<void> {
  const clean = cleanPath(parentPath)
  const url = clean ? `/spider-projects/${projectId}/files/${clean}` : `/spider-projects/${projectId}/files`
  await request.post(url, { name, type })
}

export async function deleteProjectItem(projectId: number, path: string): Promise<void> {
  await request.delete(`/spider-projects/${projectId}/files/${cleanPath(path)}`)
}

export async function moveProjectItem(projectId: number, oldPath: string, newPath: string): Promise<void> {
  await request.patch(`/spider-projects/${projectId}/files/${cleanPath(oldPath)}`, { new_path: newPath })
}

// ============================================
// 编辑器 API 适配器
// ============================================

export function createSpiderEditorApi(projectId: number) {
  return {
    getFileTree: () => getProjectFileTree(projectId),
    getFile: (path: string) => getProjectFileByPath(projectId, path),
    saveFile: (path: string, content: string) => saveProjectFileByPath(projectId, path, content),
    createItem: (parentPath: string, name: string, type: 'file' | 'dir') =>
      createProjectItem(projectId, parentPath, name, type),
    deleteItem: (path: string) => deleteProjectItem(projectId, path),
    moveItem: (oldPath: string, newPath: string) => moveProjectItem(projectId, oldPath, newPath)
  }
}

// ============================================
// 代码模板 API
// ============================================

export async function getCodeTemplates(): Promise<CodeTemplate[]> {
  const res: { data: CodeTemplate[] } = await request.get('/spider-projects/templates')
  return res.data || []
}

// ============================================
// WebSocket API
// ============================================

interface LogSubscriptionHandlers {
  onLog: (level: string, message: string) => void
  onEnd: () => void
  onError?: (error: string) => void
  onItem?: (item: Record<string, unknown>) => void
}

function createLogSubscription(wsUrl: string, handlers: LogSubscriptionHandlers): () => void {
  const { onLog, onEnd, onError, onItem } = handlers
  let ws: WebSocket | null = null
  let finished = false

  try {
    ws = new WebSocket(wsUrl)
  } catch (e) {
    onError?.(`WebSocket 创建失败: ${e}`)
    return () => {}
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'log') {
        if (msg.level === 'ITEM' && onItem) {
          try {
            onItem(JSON.parse(msg.message))
          } catch {
            onLog(msg.level, msg.message)
          }
        } else {
          onLog(msg.level, msg.message)
        }
      } else if (msg.type === 'end') {
        finished = true
        onEnd()
        closeWebSocket(ws)
      } else if (msg.type === 'error') {
        finished = true
        onError?.(msg.message)
        closeWebSocket(ws)
      }
    } catch {
      // 忽略解析错误
    }
  }

  ws.onerror = () => {
    if (!finished) {
      onError?.('WebSocket 连接失败')
    }
  }

  ws.onclose = () => {
    if (!finished) {
      finished = true
      onEnd()
    }
  }

  return () => {
    finished = true
    closeWebSocket(ws)
  }
}

export function subscribeProjectLogs(
  projectId: number,
  onLog: (level: string, message: string) => void,
  onEnd: () => void,
  onError?: (error: string) => void
): () => void {
  const wsUrl = buildWsUrl(`/ws/spider-logs/${projectId}?type=project`)
  return createLogSubscription(wsUrl, { onLog, onEnd, onError })
}

export function subscribeTestLogs(
  projectId: number,
  onLog: (level: string, message: string) => void,
  onItem: (item: Record<string, unknown>) => void,
  onEnd: () => void,
  onError?: (error: string) => void
): () => void {
  const wsUrl = buildWsUrl(`/ws/spider-logs/${projectId}?type=test`)
  return createLogSubscription(wsUrl, { onLog, onEnd, onError, onItem })
}

// ============================================
// 爬虫统计 API
// ============================================

export async function getStatsOverview(params?: {
  project_id?: number
  period?: string
  start?: string
  end?: string
}): Promise<SpiderStatsOverview> {
  const res: { data: SpiderStatsOverview } = await request.get('/spider-stats/overview', { params })
  return res.data
}

export async function getChartStats(params?: {
  project_id?: number
  period?: string
  start?: string
  end?: string
  limit?: number
}): Promise<SpiderChartDataPoint[]> {
  const res: { data: SpiderChartDataPoint[] } = await request.get('/spider-stats/chart', { params })
  return res.data || []
}

export async function getStatsByProject(params?: {
  period?: string
  start?: string
  end?: string
}): Promise<SpiderStatsByProject[]> {
  const res: { data: SpiderStatsByProject[] } = await request.get('/spider-stats/by-project', { params })
  return res.data || []
}
