import request from '@/utils/request'
import type { LoginRequest } from '@/types'
import { assertSuccess, type SuccessResponse } from './shared'

// ============================================
// 响应类型
// ============================================

interface LoginResponse extends SuccessResponse {
  token?: string
}

interface UserInfoResponse {
  id?: number
  username: string
  role: string
  last_login: string | null
}

// ============================================
// 认证 API
// ============================================

export async function login(data: LoginRequest): Promise<{
  access_token: string
  token_type: string
  expires_in: number
}> {
  const res: LoginResponse = await request.post('/auth/login', data)
  assertSuccess(res, '登录失败')
  return {
    access_token: res.token || '',
    token_type: 'bearer',
    expires_in: 86400
  }
}

export async function logout(): Promise<{ success: boolean }> {
  return request.post('/auth/logout')
}

export async function getMe(): Promise<{ id: number; username: string }> {
  const res: UserInfoResponse = await request.get('/auth/profile')
  return {
    id: res.id || 1,
    username: res.username
  }
}

export async function changePassword(data: { old_password: string; new_password: string }): Promise<void> {
  const res: SuccessResponse = await request.post('/auth/change-password', data)
  assertSuccess(res, '修改密码失败')
}
