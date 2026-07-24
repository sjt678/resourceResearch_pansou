import { defineStore } from 'pinia'

export const useConfigStore = defineStore('config', {
  state: () => ({
    // 后端是否启用认证（来自 /api/health）
    authEnabled: false,
    // 当前登录用户名
    username: localStorage.getItem('pansou_user') || '',
    // 频道数与插件数（首页 hero 副标题用）
    channelsCount: 0,
    pluginsCount: 0,
    plugins: []
  }),
  getters: {
    isLoggedIn: state => !!localStorage.getItem('pansou_token')
  },
  actions: {
    setHealthInfo(info) {
      this.authEnabled = !!info.auth_enabled
      this.channelsCount = info.channels_count || 0
      this.pluginsCount = info.plugin_count || 0
      this.plugins = info.plugins || []
    },
    setLogin(token, username) {
      localStorage.setItem('pansou_token', token)
      localStorage.setItem('pansou_user', username)
      this.username = username
    },
    logout() {
      localStorage.removeItem('pansou_token')
      localStorage.removeItem('pansou_user')
      this.username = ''
    }
  }
})
