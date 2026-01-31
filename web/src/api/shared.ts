/**
 * API 共享类型和工具函数
 */

// ============================================
// 通用响应类型
// ============================================

/** 基础成功响应 */
export interface SuccessResponse {
  success: boolean
  message?: string
}

/** 带 ID 的创建响应 */
export interface CreateResponse extends SuccessResponse {
  id?: number
}

/** 带数量的操作响应 */
export interface CountResponse extends SuccessResponse {
  deleted?: number
  updated?: number
  moved?: number
  cleared?: number
  count?: number
}

// ============================================
// WebSocket 工具函数
// ============================================

/** 构建 WebSocket URL */
export function buildWsUrl(path: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}${path}`
}

/** 安全关闭 WebSocket */
export function closeWebSocket(ws: WebSocket | null): void {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    ws.close()
  }
}

// ============================================
// API 响应处理工具
// ============================================

/** 检查成功响应，失败时抛出异常 */
export function assertSuccess(res: SuccessResponse, defaultMessage: string): void {
  if (!res.success) {
    throw new Error(res.message || defaultMessage)
  }
}
