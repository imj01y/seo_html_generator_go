/**
 * 系统资源监控 WebSocket API
 */

import type { SystemStats } from '@/types/system-stats'

let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

export function connectSystemStatsWs(onMessage: (data: SystemStats) => void): void {
  // 清理之前的连接
  disconnectSystemStatsWs()

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws/system-stats`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    console.log('[SystemStats] WebSocket connected')
  }

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data) as SystemStats
      onMessage(data)
    } catch (e) {
      console.error('[SystemStats] Failed to parse message:', e)
    }
  }

  ws.onerror = (error) => {
    console.error('[SystemStats] WebSocket error:', error)
  }

  ws.onclose = () => {
    console.log('[SystemStats] WebSocket closed')
    ws = null
  }
}

export function disconnectSystemStatsWs(): void {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (ws) {
    ws.close()
    ws = null
  }
}
