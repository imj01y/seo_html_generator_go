import { defineStore } from 'pinia'
import { login, logout, getMe } from '@/api/auth'
import type { UserInfo, LoginRequest } from '@/types'

interface UserState {
  token: string | null
  userInfo: UserInfo | null
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    token: localStorage.getItem('token'),
    userInfo: null
  }),

  getters: {
    isLoggedIn: (state): boolean => !!state.token
  },

  actions: {
    async loginAction(data: LoginRequest): Promise<void> {
      const res = await login(data)
      this.token = res.access_token
      localStorage.setItem('token', res.access_token)
      await this.fetchUserInfo()
    },

    async fetchUserInfo(): Promise<void> {
      if (!this.token) return
      try {
        this.userInfo = await getMe()
      } catch {
        this.clearAuth()
      }
    },

    async logoutAction(): Promise<void> {
      try {
        await logout()
      } catch {
        // 忽略登出接口错误
      } finally {
        this.clearAuth()
      }
    },

    clearAuth(): void {
      this.token = null
      this.userInfo = null
      localStorage.removeItem('token')
    }
  }
})
