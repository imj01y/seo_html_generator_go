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

    switch (status) {
      case 401:
        // 检查是否是登录页面的请求（登录失败）
        if (error.config?.url?.includes('/auth/login')) {
          // 登录失败，抛出带有后端消息的错误，让调用方处理
          const loginError = new Error(message || '用户名或密码错误')
          return Promise.reject(loginError)
        }
        // 其他 401 是 token 过期
        localStorage.removeItem('token')
        router.push('/login')
        ElMessage.error('登录已过期，请重新登录')
        break
      case 403:
        ElMessage.error(message || '没有访问权限')
        break
      case 404:
        ElMessage.error(message || '请求的资源不存在')
        break
      case 422:
        ElMessage.error(message || '参数错误')
        break
      case 503:
        ElMessage.warning(message || '服务暂时不可用')
        break
      default:
        ElMessage.error(message || '请求失败，请稍后重试')
    }

    return Promise.reject(error)
  }
)

export default request
