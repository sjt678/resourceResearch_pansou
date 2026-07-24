import { ref } from 'vue'
import { useHistory } from './useHistory.js'
import { suggest as suggestApi } from '@/api/suggest.js'
import { suggestTMDB } from '@/api/tmdb.js'

const CORPUS_KEY = 'pansou_title_corpus'
const MAX_CORPUS = 2000 // 扩大词库容量

// 模块级共享词库
const corpus = ref(loadCorpus())

function loadCorpus() {
  try {
    const raw = localStorage.getItem(CORPUS_KEY)
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
}

function saveCorpus() {
  try {
    localStorage.setItem(CORPUS_KEY, JSON.stringify(corpus.value))
  } catch {
    // 忽略
  }
}

// 将搜索结果标题加入词库
function addTitles(titles) {
  if (!titles || !titles.length) return
  const set = new Set(corpus.value)
  for (const t of titles) {
    const trimmed = (t || '').trim()
    if (trimmed && trimmed.length >= 2 && trimmed.length <= 60) {
      set.add(trimmed)
    }
  }
  let arr = Array.from(set)
  if (arr.length > MAX_CORPUS) arr = arr.slice(arr.length - MAX_CORPUS)
  corpus.value = arr
  saveCorpus()
}

function clearCorpus() {
  corpus.value = []
  saveCorpus()
}

// 相似度：包含关系优先；其次公共子串长度
function similarity(query, target) {
  const q = query.toLowerCase()
  const t = target.toLowerCase()
  if (t.includes(q)) return 100 + q.length / t.length
  if (q.includes(t)) return 60 + t.length / q.length
  let common = 0
  const set = new Set(q)
  for (const ch of t) if (set.has(ch)) common++
  return common
}

/**
 * 从本地词库匹配建议（同步、零延迟）
 */
function localSuggest(query, limit = 8) {
  const q = (query || '').trim().toLowerCase()
  if (!q) return []
  const { history } = useHistory()
  const candidates = new Set()
  ;[...history.value, ...corpus.value].forEach(s => {
    if (s && s.toLowerCase().includes(q)) candidates.add(s)
  })
  if (candidates.size < limit) {
    for (const s of corpus.value) {
      if (similarity(q, s) > 1) candidates.add(s)
    }
  }
  return Array.from(candidates)
    .map(s => ({ s, score: similarity(q, s) }))
    .sort((a, b) => b.score - a.score)
    .slice(0, limit)
    .map(x => x.s)
}

// 请求缓存：同一 query 7 天内不重复请求 TMDB
const requestCache = new Map()
const REQUEST_CACHE_TTL = 7 * 24 * 60 * 60 * 1000

function getCachedRequest(q) {
  const entry = requestCache.get(q)
  if (entry && Date.now() - entry.t < REQUEST_CACHE_TTL) return entry.names
  return null
}

function setCachedRequest(q, names) {
  requestCache.set(q, { names, t: Date.now() })
}

// TMDB 直连失败重试控制
let tmdbDirectFailed = false
let tmdbDirectRetryAt = 0

/**
 * 获取输入建议（极速版）
 *
 * 策略：
 * 1. 同步立即返回本地词库匹配结果（0ms）
 * 2. 异步请求 TMDB，有新结果时通过回调更新
 * 3. 7 天内同一查询只请求一次 TMDB
 *
 * @param {string} query 用户输入
 * @param {number} limit 最大返回条数
 * @param {AbortSignal} [signal] 用于取消请求
 * @param {(names: string[], source: 'local'|'tmdb'|'backend') => void} [onUpdate] 异步更新回调
 * @returns {{ immediate: string[], promise: Promise<string[]> }}
 */
function suggest(query, limit = 8, signal, onUpdate) {
  const q = (query || '').trim()
  if (!q) return { immediate: [], promise: Promise.resolve([]) }

  // 1. 同步：立即返回本地词库匹配
  const immediate = localSuggest(q, limit)
  if (onUpdate && immediate.length) {
    onUpdate(immediate, 'local')
  }

  // 2. 异步：请求 TMDB 补充
  const promise = (async () => {
    // 先查请求缓存
    const cached = getCachedRequest(q)
    if (cached) {
      if (onUpdate && cached.length) {
        addTitles(cached)
        onUpdate(cached, 'tmdb')
      }
      return cached
    }

    // 优先：前端直连 TMDB
    if (!tmdbDirectFailed || Date.now() > tmdbDirectRetryAt) {
      try {
        const names = await suggestTMDB(q, limit, signal)
        if (names && names.length) {
          addTitles(names)
          setCachedRequest(q, names)
          if (onUpdate) onUpdate(names, 'tmdb')
          return names
        }
      } catch (e) {
        if (e?.code !== 'ERR_CANCELED' && e?.name !== 'CanceledError') {
          tmdbDirectFailed = true
          tmdbDirectRetryAt = Date.now() + 30 * 1000
        }
      }
    }

    // 兜底：后端
    try {
      const names = await suggestApi(q, limit, signal)
      if (names && names.length) {
        addTitles(names)
        setCachedRequest(q, names)
        if (onUpdate) onUpdate(names, 'backend')
        return names
      }
    } catch {
      // 忽略
    }

    return immediate
  })()

  return { immediate, promise }
}

export function useSuggestions() {
  return {
    corpus,
    addTitles,
    clearCorpus,
    suggest
  }
}
