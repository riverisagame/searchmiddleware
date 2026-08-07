import { defineStore } from 'pinia'
import { api, getToken, setToken, clearToken } from '../api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: getToken() || '',
    user: JSON.parse(localStorage.getItem('sm_user') || 'null'),
  }),
  getters: {
    isAdmin: (s) => s.user && s.user.role === 'admin',
    isLoggedIn: (s) => !!s.token,
  },
  actions: {
    async login(username, password) {
      const data = await api.login(username, password)
      this.token = data.token
      this.user = data.user
      setToken(data.token)
      localStorage.setItem('sm_user', JSON.stringify(data.user))
    },
    logout() {
      this.token = ''
      this.user = null
      clearToken()
      localStorage.removeItem('sm_user')
    },
  },
})
