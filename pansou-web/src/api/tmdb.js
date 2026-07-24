import axios from 'axios'

// TMDB 直连（用户浏览器能访问就用这个，不走服务器）
// TMDB_BASE 可通过 vite 环境变量 VITE_TMDB_BASE 覆盖（用于自建 Cloudflare Workers 反代）
const TMDB_API_KEY = '13bf320bec60b9a3c2ec2bbf92efa85c'
const TMDB_BASE = import.meta.env.VITE_TMDB_BASE || 'https://api.themoviedb.org/3'

const tmdbClient = axios.create({
  baseURL: TMDB_BASE,
  timeout: 6000
})

/**
 * 前端直连 TMDB 获取建议
 * 用户浏览器能访问 TMDB 就走这里，无需服务器
 * @param {string} q 关键词
 * @param {number} [limit=8] 最大返回条数
 * @param {AbortSignal} [signal] 用于取消请求
 * @returns {Promise<string[]>}
 */
export async function suggestTMDB(q, limit = 8, signal) {
  const resp = await tmdbClient.get('/search/multi', {
    params: {
      api_key: TMDB_API_KEY,
      language: 'zh-CN',
      query: q,
      page: 1,
      include_adult: false
    },
    signal
  })

  const results = resp.data?.results || []
  const names = []
  const seen = new Set()
  for (const item of results) {
    if (item.media_type === 'person') continue
    const name = (item.name || item.title || item.original_name || '').trim()
    if (!name || seen.has(name)) continue
    seen.add(name)
    names.push(name)
    if (names.length >= limit) break
  }
  return names
}
