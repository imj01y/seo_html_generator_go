import request from '@/utils/request'
import type { LoginRequest } from '@/types'

// 后端返回格式
interface BackendLoginResponse {
  success: boolean
  token?: string
  message?: string
}

// 后端返回的用户信息格式
interface BackendUserInfo {
  username: string
  role: string
  last_login: string
}

export const login = async (data: LoginRequest) => {
  const res: BackendLoginResponse = await request.post('/auth/login', data)
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

export const logout = (): Promise<{ success: boolean }> =>
  request.post('/auth/logout')

export const getMe = async () => {
  const res: BackendUserInfo = await request.get('/auth/profile')
  return {
    id: 1,
    username: res.username
  }
}

// 修改密码响应格式
interface ChangePasswordResponse {
  success: boolean
  message?: string
}

export const changePassword = async (data: { old_password: string; new_password: string }): Promise<void> => {
  const res: ChangePasswordResponse = await request.post('/auth/change-password', data)
  if (!res.success) {
    throw new Error(res.message || '修改密码失败')
  }
}
