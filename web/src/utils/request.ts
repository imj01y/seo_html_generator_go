import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'

// 后端统一响应格式
interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
  timestamp: number
}

// 根据 HTTP 状态码返回默认错误消息
function getDefaultErrorMessage(status?: number): string {
  switch (status) {
    case 400: return '请求参数错误'
    case 403: return '没有访问权限'
    case 404: return '请求的资源不存在'
    case 422: return '参数校验失败'
    case 500: return '服务器内部错误'
    case 502: return '网关错误'
    case 503: return '服务暂时不可用'
    case 504: return '网关超时'
    default: return '请求失败，请稍后重试'
  }
}

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 30000,
})

// 请求拦截器
request.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// 响应拦截器
request.interceptors.response.use(
  (response) => {
    const res = response.data as ApiResponse

    // 检查是否是统一响应格式（包含 code 字段）
    if (res && typeof res === 'object' && 'code' in res) {
      // code === 0 表示成功
      if (res.code === 0) {
        // 返回 data 字段内容，如果没有 data 则返回整个响应
        return res.data !== undefined ? res.data : res
      }

      // 处理业务错误（code !== 0）
      const errorMsg = res.message || '请求失败'

      // 特殊处理 401 未授权
      if (res.code === 401 || res.code === 10401) {
        // 检查是否是登录请求
        if (!response.config?.url?.includes('/auth/login')) {
          localStorage.removeItem('token')
          router.push('/login')
          ElMessage.error('登录已过期，请重新登录')
          // 标记已处理，组件层跳过重复提示
          const authError = new Error('登录已过期，请重新登录')
          ;(authError as any)._handled = true
          return Promise.reject(authError)
        }
        return Promise.reject(new Error(errorMsg))
      }

      // 其他业务错误不弹窗，让调用方处理
      return Promise.reject(new Error(errorMsg))
    }

    // 非统一格式的响应（兼容旧接口），直接返回
    return response.data
  },
  (error: AxiosError<ApiResponse | { message?: string; detail?: string }>) => {
    const status = error.response?.status
    const data = error.response?.data
    // 优先使用后端返回的 message，其次 detail
    const message = (data && 'message' in data ? data.message : undefined) ||
                    (data && 'detail' in data ? data.detail : undefined)

    // 401 特殊处理：需要跳转登录页
    if (status === 401) {
      // 检查是否是登录页面的请求（登录失败）
      if (error.config?.url?.includes('/auth/login')) {
        // 登录失败，抛出带有后端消息的错误，让调用方处理
        return Promise.reject(new Error(message || '用户名或密码错误'))
      }
      // 其他 401 是 token 过期，弹窗提示并跳转
      localStorage.removeItem('token')
      router.push('/login')
      ElMessage.error('登录已过期，请重新登录')
      // 返回一个特殊错误，组件层可以识别并跳过重复提示
      const authError = new Error('登录已过期，请重新登录')
      ;(authError as any)._handled = true
      return Promise.reject(authError)
    }

    // 其他错误：不弹窗，让组件层处理
    // 将后端消息附加到错误对象，便于组件获取
    const errorMessage = message || getDefaultErrorMessage(status)
    return Promise.reject(new Error(errorMessage))
  }
)

/**
 * 通用错误处理函数
 * 用于组件 catch 块中，自动跳过已处理的错误（如 401 登录过期）
 *
 * @example
 * try {
 *   await api.save(data)
 * } catch (e) {
 *   handleError(e, '保存失败')
 * }
 */
export function handleError(error: unknown, fallbackMessage = '操作失败'): void {
  // 跳过已处理的错误（如 401 登录过期已在拦截器中弹窗）
  if (error && typeof error === 'object' && '_handled' in error) {
    return
  }
  const message = error instanceof Error ? error.message : fallbackMessage
  ElMessage.error(message || fallbackMessage)
}

export default request
