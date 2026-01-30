import request from '@/utils/request'

// ============================================
// 类型定义
// ============================================

export interface LogEntry {
  id: number
  level: string
  module?: string
  spider_project_id?: number
  message: string
  extra?: Record<string, any>
  created_at: string
}

export interface LogQuery {
  level?: string
  module?: string
  spider_project_id?: number
  search?: string
  page?: number
  page_size?: number
}

export interface LogStats {
  level_stats: Record<string, number>
  today_count: number
  recent_errors: {
    id: number
    module?: string
    message: string
    created_at: string
  }[]
  websocket_clients: number
}

// ============================================
// 响应类型
// ============================================

interface LogListResponse {
  success: boolean
  logs: LogEntry[]
  total: number
  page: number
  page_size: number
}

interface LogStatsResponse {
  success: boolean
  data: LogStats
}

interface ClearResponse {
  success: boolean
  deleted: number
  message: string
}

// ============================================
// HTTP API
// ============================================

/**
 * 获取历史日志
 */
export const getLogHistory = async (params?: LogQuery) => {
  const res: LogListResponse = await request.get('/logs/history', { params })
  return {
    logs: res.logs || [],
    total: res.total
  }
}

/**
 * 获取日志统计
 */
export const getLogStats = async (): Promise<LogStats> => {
  const res: LogStatsResponse = await request.get('/logs/stats')
  return res.data
}

/**
 * 清理旧日志
 */
export const clearOldLogs = async (days: number = 30): Promise<ClearResponse> => {
  return await request.delete('/logs/clear', { params: { days } })
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

export interface RealtimeLog {
  type: 'log' | 'heartbeat'
  level?: string
  module?: string
  message?: string
  time?: string
  spider_project_id?: number
}

/**
 * 订阅实时日志
 */
export function subscribeRealtimeLogs(
  onLog: (log: RealtimeLog) => void,
  onError?: (error: string) => void
): () => void {
  const wsUrl = buildWsUrl('/api/logs/ws')
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
      const msg: RealtimeLog = JSON.parse(event.data)
      if (msg.type === 'log') {
        onLog(msg)
      }
      // 忽略 heartbeat
    } catch {
      // 忽略解析错误
    }
  }

  ws.onerror = () => {
    if (!finished) {
      onError?.('WebSocket 连接失败')
    }
  }

  ws.onclose = (event) => {
    if (!finished && !event.wasClean) {
      onError?.('连接断开')
    }
  }

  return () => {
    finished = true
    closeWebSocket(ws)
  }
}
