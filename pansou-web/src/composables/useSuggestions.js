import { ref } from 'vue'
import { useHistory } from './useHistory.js'
import { suggest as suggestApi } from '@/api/suggest.js'

const CORPUS_KEY = 'pansou_title_corpus'
const MAX_CORPUS = 500

// 模块级共享词库：所有组件复用同一份
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

// 简单相似度：包含关系优先；其次公共子串长度
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
 * 从本地词库匹配建议（兜底）
 * @param {string} query
 * @param {number} limit
 * @returns {string[]}
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

/**
 * 获取输入建议：
 * 1. 优先调后端 /api/suggest（TMDB 数据，更权威）
 * 2. 后端未启用或失败时，回退本地词库
 * @param {string} query 用户输入
 * @param {number} limit 最大返回条数
 * @param {AbortSignal} [signal] 用于取消请求
 * @returns {Promise<string[]>}
 */
async function suggest(query, limit = 8, signal) {
  const q = (query || '').trim()
  if (!q) return []

  // 1. 先尝试后端 TMDB 接口
  try {
    const names = await suggestApi(q, limit, signal)
    if (names && names.length) {
      // 顺便把后端返回的剧名写入本地词库，越用越丰富
      addTitles(names)
      return names
    }
  } catch {
    // 后端失败，走本地兜底
  }

  // 2. 本地兜底
  return localSuggest(q, limit)
}

export function useSuggestions() {
  return {
    corpus,
    addTitles,
    clearCorpus,
    suggest
  }
}
