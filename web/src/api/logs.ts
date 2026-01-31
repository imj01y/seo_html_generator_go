import request from '@/utils/request'
import { buildWsUrl, closeWebSocket } from './shared'

// ============================================
// 类型定义
// ============================================

export interface LogEntry {
  id: number
  level: string
  module?: string
  spider_project_id?: number
  message: string
  extra?: Record<string, unknown>
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

export interface RealtimeLog {
  type: 'log' | 'heartbeat'
  level?: string
  module?: string
  message?: string
  time?: string
  spider_project_id?: number
}

// ============================================
// 响应类型
// ============================================

interface LogListResponse {
  logs: LogEntry[]
  total: number
}

interface LogStatsResponse {
  data: LogStats
}

interface ClearResponse {
  deleted: number
  message: string
}

// ============================================
// HTTP API
// ============================================

export async function getLogHistory(params?: LogQuery): Promise<{ logs: LogEntry[]; total: number }> {
  const res: LogListResponse = await request.get('/logs/history', { params })
  return {
    logs: res.logs || [],
    total: res.total
  }
}

export async function getLogStats(): Promise<LogStats> {
  const res: LogStatsResponse = await request.get('/logs/stats')
  return res.data
}

export async function clearOldLogs(days: number = 30): Promise<ClearResponse> {
  return request.delete('/logs/clear', { params: { days } })
}

// ============================================
// WebSocket API
// ============================================

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
