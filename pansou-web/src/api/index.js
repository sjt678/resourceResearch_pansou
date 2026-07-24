import axios from 'axios'
import { ElMessage } from 'element-plus'

// VITE_API_BASE 为空时走 vite proxy（开发）或同源（生产）
const baseURL = import.meta.env.VITE_API_BASE || ''

const service = axios.create({
  baseURL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器：自动注入 JWT token（如果存在）
service.interceptors.request.use(
  config => {
    const token = localStorage.getItem('pansou_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  error => Promise.reject(error)
)

// 响应拦截器：统一拆包 + 错误提示
service.interceptors.response.use(
  response => {
    const data = response.data

    // /api/check/links 直接返回 { results: [] }，没有 code/message 包装，原样返回
    // /api/health 返回扁平对象，也原样返回
    // /api/search 返回标准 { code, message, data }
    if (data && typeof data === 'object' && 'code' in data) {
      if (data.code === 0) {
        return data.data
      }
      ElMessage.error(data.message || '请求失败')
      return Promise.reject(new Error(data.message || '请求失败'))
    }

    // 非标准包装直接返回
    return data
  },
  error => {
    // 请求被取消（切 tab / 主动取消）静默处理
    if (error.code === 'ERR_CANCELED' || axios.isCancel(error)) {
      return Promise.reject(error)
    }

    // 401 未授权：清除 token，提示重新登录
    if (error.response && error.response.status === 401) {
      localStorage.removeItem('pansou_token')
      localStorage.removeItem('pansou_user')
      ElMessage.error('登录已过期，请重新登录')
      return Promise.reject(error)
    }

    const msg =
      error.response?.data?.error ||
      error.response?.data?.message ||
      error.message ||
      '网络错误'
    ElMessage.error(msg)
    return Promise.reject(error)
  }
)

export default service
