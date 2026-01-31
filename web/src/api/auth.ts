import request from '@/utils/request'
import type { LoginRequest } from '@/types'

// 登录响应数据（现在直接从 data 字段获取）
interface LoginData {
  success: boolean
  token?: string
  message?: string
}

// 后端返回的用户信息格式
interface BackendUserInfo {
  id?: number
  username: string
  role: string
  last_login: string | null
}

export const login = async (data: LoginRequest) => {
  const res: LoginData = await request.post('/auth/login', data)
  // 检查登录是否成功
  if (!res.success) {
    throw new Error(res.message || '登录失败')
  }
  // 转换为前端期望的格式
  return {
    access_token: res.token || '',
    token_type: 'bearer',
    expires_in: 86400
  }
}

export const logout = async (): Promise<{ success: boolean }> => {
  return await request.post('/auth/logout')
}

export const getMe = async () => {
  const res: BackendUserInfo = await request.get('/auth/profile')
  return {
    id: res.id || 1,
    username: res.username
  }
}

// 修改密码响应格式
interface ChangePasswordData {
  success: boolean
  message?: string
}

export const changePassword = async (data: { old_password: string; new_password: string }): Promise<void> => {
  const res: ChangePasswordData = await request.post('/auth/change-password', data)
  if (!res.success) {
    throw new Error(res.message || '修改密码失败')
  }
}
