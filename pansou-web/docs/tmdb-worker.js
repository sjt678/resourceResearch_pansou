/**
 * TMDB API 反向代理 - Cloudflare Worker
 *
 * 部署步骤：
 * 1. 注册/登录 https://dash.cloudflare.com（免费）
 * 2. 左侧菜单 → Workers & Pages → Create → Create Worker
 * 3. 起个名字（比如 tmdb-proxy）→ Deploy
 * 4. 编辑代码 → 把本文件内容粘贴进去 → Deploy
 * 5. 复制你的 Worker 地址，形如 https://tmdb-proxy.xxx.workers.dev
 * 6. 在 pansou-web/.env.production 或 .env.development 里填：
 *    VITE_TMDB_BASE=https://tmdb-proxy.xxx.workers.dev/3
 * 7. 在 pansou/docker-compose.yml 里填：
 *    - TMDB_BASE_URL=https://tmdb-proxy.xxx.workers.dev/3
 *
 * 免费额度：每天 10 万次请求，搜索联想完全够用
 * 国内可访问：Cloudflare 有国内节点
 */

const UPSTREAM = 'https://api.themoviedb.org/3'

export default {
  async fetch(request) {
    const url = new URL(request.url)

    // 健康检查
    if (url.pathname === '/' || url.pathname === '/health') {
      return new Response('TMDB proxy OK', { status: 200 })
    }

    // 拼接上游 URL：把 /3 前缀去掉，避免重复
    let upstreamPath = url.pathname
    if (upstreamPath.startsWith('/3')) {
      upstreamPath = upstreamPath.slice(2)
    } else if (upstreamPath.startsWith('/3/')) {
      upstreamPath = upstreamPath.slice(3)
    }
    const upstreamUrl = UPSTREAM + upstreamPath + (url.search || '')

    // 转发请求
    const init = {
      method: request.method,
      headers: {
        'Accept': 'application/json',
        'User-Agent': 'PanSou-TMDB-Proxy/1.0'
      }
    }

    try {
      const resp = await fetch(upstreamUrl, init)
      const body = await resp.text()

      // 返回响应，加 CORS 头（允许前端跨域访问）
      return new Response(body, {
        status: resp.status,
        headers: {
          'Content-Type': 'application/json; charset=utf-8',
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
          'Access-Control-Allow-Headers': '*',
          'Cache-Control': 'public, max-age=86400'
        }
      })
    } catch (e) {
      return new Response(JSON.stringify({ error: 'upstream failed', message: e.message }), {
        status: 502,
        headers: { 'Content-Type': 'application/json' }
      })
    }
  }
}
