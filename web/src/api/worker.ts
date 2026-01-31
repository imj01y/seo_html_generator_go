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
  return request.get(`/worker/files/${path}`)
}

/**
 * 获取文件内容
 */
export const getFile = async (path: string): Promise<FileContent> => {
  return request.get(`/worker/files/${path}`)
}

/**
 * 获取目录树
 */
export const getFileTree = async (): Promise<TreeNode> => {
  return request.get('/worker/files', { params: { tree: 'true' } })
}

/**
 * 保存文件
 */
export const saveFile = async (path: string, content: string): Promise<void> => {
  await request.put(`/worker/files/${path}`, { content })
}

/**
 * 创建文件或目录
 */
export const createItem = async (
  parentPath: string,
  name: string,
  type: 'file' | 'dir'
): Promise<void> => {
  await request.post(`/worker/files/${parentPath}`, { name, type })
}

/**
 * 删除文件或目录
 */
export const deleteItem = async (path: string): Promise<void> => {
  await request.delete(`/worker/files/${path}`)
}

/**
 * 移动/重命名
 */
export const moveItem = async (oldPath: string, newPath: string): Promise<void> => {
  await request.patch(`/worker/files/${oldPath}`, { new_path: newPath })
}

/**
 * 获取下载 URL
 */
export const getDownloadUrl = (path: string): string => {
  const token = localStorage.getItem('token')
  return `/api/worker/download/${path}?token=${token}`
}

// ============================================
// 控制 API
// ============================================

/**
 * 重启 Worker
 */
export const restartWorker = async (): Promise<{ message: string }> => {
  return request.post('/worker/restart')
}

/**
 * 重新构建 Worker
 */
export const rebuildWorker = async (): Promise<{ message: string; output: string }> => {
  return request.post('/worker/rebuild')
}

// ============================================
// WebSocket API
// ============================================

interface RunLogHandlers {
  onStdout: (data: string) => void
  onStderr: (data: string) => void
  onDone: (exitCode: number, durationMs: number) => void
  onError?: (error: string) => void
}

/**
 * 运行 Python 文件
 */
export function runFile(filePath: string, handlers: RunLogHandlers): () => void {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws/worker/run`

  let ws: WebSocket | null = null

  try {
    ws = new WebSocket(wsUrl)
  } catch (e) {
    handlers.onError?.(`WebSocket 创建失败: ${e}`)
    return () => {}
  }

  ws.onopen = () => {
    ws?.send(JSON.stringify({ action: 'run', file: filePath }))
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      switch (msg.type) {
        case 'stdout':
          handlers.onStdout(msg.data)
          break
        case 'stderr':
          handlers.onStderr(msg.data)
          break
        case 'done':
          handlers.onDone(msg.exit_code, msg.duration_ms || 0)
          ws?.close()
          break
      }
    } catch {
      // 忽略解析错误
    }
  }

  ws.onerror = () => {
    handlers.onError?.('WebSocket 连接失败')
  }

  return () => {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      ws.close()
    }
  }
}
