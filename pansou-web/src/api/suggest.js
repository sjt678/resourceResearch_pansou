import service from './index.js'

/**
 * 获取搜索建议（TMDB 后端接口）
 * @param {string} q 关键词
 * @param {number} [limit=8] 最大返回条数
 * @param {AbortSignal} [signal] 用于取消请求
 * @returns {Promise<string[]>}
 */
export function suggest(q, limit = 8, signal) {
  return service
    .get('/api/suggest', {
      params: { q, limit },
      signal
    })
    .then(data => {
      // 后端返回 { suggestions: [] } 或 { data: { suggestions: [] } }
      if (Array.isArray(data?.suggestions)) return data.suggestions
      if (Array.isArray(data?.data?.suggestions)) return data.data.suggestions
      return []
    })
    .catch(() => [])
}
