import request from '@/utils/request'

// ============================================
// 类型定义
// ============================================

export interface SpiderProject {
  id: number
  name: string
  description?: string
  entry_file: string
  entry_function: string
  start_url?: string
  config?: Record<string, any>
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
}

export interface ProjectFile {
  id: number
  filename: string
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
  config?: Record<string, any>
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
  config?: Record<string, any>
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

export interface TestResult {
  success: boolean
  items: Record<string, any>[]
  logs: { time: string; level: string; message: string }[]
  duration: number
  error?: string
}

// ============================================
// 响应类型
// ============================================

interface ProjectListResponse {
  success: boolean
  data: SpiderProject[]
  total: number
  page: number
  page_size: number
}

interface ProjectResponse {
  success: boolean
  data: SpiderProject
}

interface FilesResponse {
  success: boolean
  data: ProjectFile[]
}

interface FileResponse {
  success: boolean
  data: ProjectFile
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

interface TestResponse {
  success: boolean
  data?: TestResult
  error?: string
}

// ============================================
// 项目管理 API
// ============================================

/**
 * 获取项目列表
 */
export const getProjects = async (params?: ProjectQuery) => {
  const res: ProjectListResponse = await request.get('/spider-projects', { params })
  return {
    items: res.data || [],
    total: res.total
  }
}

/**
 * 获取项目详情
 */
export const getProject = async (id: number): Promise<SpiderProject> => {
  const res: ProjectResponse = await request.get(`/spider-projects/${id}`)
  return res.data
}

/**
 * 创建项目
 */
export const createProject = async (data: ProjectCreate): Promise<MutationResponse> => {
  return await request.post('/spider-projects', data)
}

/**
 * 更新项目
 */
export const updateProject = async (id: number, data: ProjectUpdate): Promise<MutationResponse> => {
  return await request.put(`/spider-projects/${id}`, data)
}

/**
 * 删除项目
 */
export const deleteProject = async (id: number): Promise<MutationResponse> => {
  return await request.delete(`/spider-projects/${id}`)
}

/**
 * 切换启用状态
 */
export const toggleProject = async (id: number): Promise<ToggleResponse> => {
  return await request.post(`/spider-projects/${id}/toggle`)
}

/**
 * 运行项目
 */
export const runProject = async (id: number): Promise<MutationResponse> => {
  return await request.post(`/spider-projects/${id}/run`)
}

/**
 * 停止项目
 */
export const stopProject = async (id: number): Promise<MutationResponse> => {
  return await request.post(`/spider-projects/${id}/stop`)
}

/**
 * 重置项目 - 清空所有队列数据和失败请求记录
 */
export const resetProject = async (id: number): Promise<MutationResponse> => {
  return await request.post(`/spider-projects/${id}/reset`)
}

/**
 * 启动测试运行项目（返回 session_id，通过 WebSocket 订阅日志）
 * @param id 项目ID
 * @param maxItems 最大测试条数，0 表示不限制
 */
export const testProject = async (
  id: number,
  maxItems: number = 0
): Promise<{ success: boolean; session_id?: string; message?: string }> => {
  return await request.post(`/spider-projects/${id}/test`, null, {
    params: { max_items: maxItems }
  })
}

/**
 * 停止测试运行
 */
export const stopTestProject = async (id: number): Promise<MutationResponse> => {
  return await request.post(`/spider-projects/${id}/test/stop`)
}

// ============================================
// 项目文件 API
// ============================================

/**
 * 获取项目文件列表
 */
export const getProjectFiles = async (projectId: number): Promise<ProjectFile[]> => {
  const res: FilesResponse = await request.get(`/spider-projects/${projectId}/files`)
  return res.data || []
}

/**
 * 获取单个文件
 */
export const getProjectFile = async (projectId: number, filename: string): Promise<ProjectFile> => {
  const res: FileResponse = await request.get(`/spider-projects/${projectId}/files/${filename}`)
  return res.data
}

/**
 * 创建文件
 */
export const createProjectFile = async (
  projectId: number,
  data: { filename: string; content: string }
): Promise<MutationResponse> => {
  return await request.post(`/spider-projects/${projectId}/files`, data)
}

/**
 * 更新文件
 */
export const updateProjectFile = async (
  projectId: number,
  filename: string,
  content: string
): Promise<MutationResponse> => {
  return await request.put(`/spider-projects/${projectId}/files/${filename}`, { content })
}

/**
 * 删除文件
 */
export const deleteProjectFile = async (
  projectId: number,
  filename: string
): Promise<MutationResponse> => {
  return await request.delete(`/spider-projects/${projectId}/files/${filename}`)
}

// ============================================
// 工具 API
// ============================================

/**
 * 获取代码模板
 */
export const getCodeTemplates = async (): Promise<CodeTemplate[]> => {
  const res: TemplatesResponse = await request.get('/spider-projects/templates')
  return res.data || []
}

// ============================================
// WebSocket API
// ============================================

/** 构建 WebSocket URL */
function buildWsUrl(path: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}${path}`
}

/** 安全关闭 WebSocket */
function closeWebSocket(ws: WebSocket | null): void {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    ws.close()
  }
}

/** WebSocket 消息处理器配置 */
interface LogSubscriptionHandlers {
  onLog: (level: string, message: string) => void
  onEnd: () => void
  onError?: (error: string) => void
  onItem?: (item: Record<string, any>) => void
}

/**
 * 创建 WebSocket 日志订阅（通用实现）
 */
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
      switch (msg.type) {
        case 'log':
          // 处理 ITEM 类型的日志（包含数据项）
          if (msg.level === 'ITEM' && onItem) {
            try {
              const item = JSON.parse(msg.message)
              onItem(item)
            } catch {
              onLog(msg.level, msg.message)
            }
          } else {
            onLog(msg.level, msg.message)
          }
          break
        case 'end':
          finished = true
          onEnd()
          closeWebSocket(ws)
          break
        case 'error':
          finished = true
          onError?.(msg.message)
          closeWebSocket(ws)
          break
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
    // 无论是否正常关闭，如果还没有收到 end 消息，都应该结束
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

/**
 * 订阅项目执行日志
 */
export function subscribeProjectLogs(
  projectId: number,
  onLog: (level: string, message: string) => void,
  onEnd: () => void,
  onError?: (error: string) => void
): () => void {
  const wsUrl = buildWsUrl(`/ws/spider-logs/${projectId}?type=project`)
  return createLogSubscription(wsUrl, { onLog, onEnd, onError })
}

/**
 * 订阅测试执行日志
 */
export function subscribeTestLogs(
  projectId: number,
  onLog: (level: string, message: string) => void,
  onItem: (item: Record<string, any>) => void,
  onEnd: () => void,
  onError?: (error: string) => void
): () => void {
  const wsUrl = buildWsUrl(`/ws/spider-logs/${projectId}?type=test`)
  return createLogSubscription(wsUrl, { onLog, onEnd, onError, onItem })
}


// ============================================
// 爬虫统计 API
// ============================================

import type { SpiderStatsOverview, SpiderChartDataPoint, SpiderStatsByProject } from '@/types'

interface StatsOverviewResponse {
  success: boolean
  data: SpiderStatsOverview
}

interface ChartDataResponse {
  success: boolean
  data: SpiderChartDataPoint[]
}

interface StatsByProjectResponse {
  success: boolean
  data: SpiderStatsByProject[]
}

/**
 * 获取统计概览
 */
export const getStatsOverview = async (params?: {
  project_id?: number
  period?: string
  start?: string
  end?: string
}): Promise<SpiderStatsOverview> => {
  const res: StatsOverviewResponse = await request.get('/spider-stats/overview', { params })
  return res.data
}

/**
 * 获取图表数据
 */
export const getChartStats = async (params?: {
  project_id?: number
  period?: string
  start?: string
  end?: string
  limit?: number
}): Promise<SpiderChartDataPoint[]> => {
  const res: ChartDataResponse = await request.get('/spider-stats/chart', { params })
  return res.data || []
}

/**
 * 获取按项目统计
 */
export const getStatsByProject = async (params?: {
  period?: string
  start?: string
  end?: string
}): Promise<SpiderStatsByProject[]> => {
  const res: StatsByProjectResponse = await request.get('/spider-stats/by-project', { params })
  return res.data || []
}
