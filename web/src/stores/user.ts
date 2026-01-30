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
    isLoggedIn: (state) => !!state.token
  },

  actions: {
    async loginAction(data: LoginRequest) {
      const res = await login(data)
      this.token = res.access_token
      localStorage.setItem('token', res.access_token)
      await this.fetchUserInfo()
    },

    async fetchUserInfo() {
      if (!this.token) return
      try {
        const userInfo = await getMe()
        this.userInfo = userInfo
      } catch {
        // 获取用户信息失败，清除token
        this.logout()
      }
    },

    async logoutAction() {
      try {
        await logout()
      } catch {
        // 忽略登出接口错误
      } finally {
        this.logout()
      }
    },

    logout() {
      this.token = null
      this.userInfo = null
      localStorage.removeItem('token')
    }
  }
})
