// 时间格式化工具
export function formatRelativeTime(input) {
  if (!input) return ''
  const date = new Date(input)
  if (isNaN(date.getTime())) return ''

  const now = Date.now()
  const diff = now - date.getTime()
  const sec = Math.floor(diff / 1000)
  const min = Math.floor(sec / 60)
  const hour = Math.floor(min / 60)
  const day = Math.floor(hour / 24)

  if (sec < 60) return '刚刚'
  if (min < 60) return `${min}分钟前`
  if (hour < 24) return `${hour}小时前`
  if (day < 30) return `${day}天前`
  return formatDate(date)
}

export function formatDate(input) {
  const date = input instanceof Date ? input : new Date(input)
  if (isNaN(date.getTime())) return ''
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

// 关键词高亮：把文本中命中关键词的部分包成 <span class="hl-keyword">
// 转义特殊字符，避免 XSS
function escapeHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

export function highlight(text, keyword) {
  if (!text) return ''
  const safe = escapeHtml(text)
  const kw = (keyword || '').trim()
  if (!kw) return safe
  // 转义关键词中的正则元字符
  const escapedKw = kw.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const re = new RegExp(escapedKw, 'gi')
  return safe.replace(re, m => `<span class="hl-keyword">${m}</span>`)
}

// 从 url 推断网盘类型（兜底，正常用后端给的 cloudType）
export function inferCloudType(url) {
  if (!url) return 'others'
  const u = String(url).toLowerCase()
  if (u.includes('quark')) return 'quark'
  if (u.includes('baidu') || u.includes('pan.baidu')) return 'baidu'
  if (u.includes('aliyun') || u.includes('alipan')) return 'aliyun'
  if (u.includes('115')) return '115'
  if (u.includes('weiyun') || u.includes('share.weqq')) return 'weiyun'
  return 'others'
}
