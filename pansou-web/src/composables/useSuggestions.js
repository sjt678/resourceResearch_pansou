import { ref } from 'vue'
import { useHistory } from './useHistory.js'

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
  // 共同字符数
  let common = 0
  const set = new Set(q)
  for (const ch of t) if (set.has(ch)) common++
  return common
}

/**
 * 获取输入建议：从历史 + 词库中匹配与输入最相关的若干条
 * @param {string} query 用户输入
 * @param {number} limit 最大返回条数
 * @returns {string[]}
 */
function suggest(query, limit = 8) {
  const q = (query || '').trim().toLowerCase()
  if (!q) return []
  const { history } = useHistory()

  const candidates = new Set()
  ;[...history.value, ...corpus.value].forEach(s => {
    if (s && s.toLowerCase().includes(q)) candidates.add(s)
  })
  // 兜底：从词库里挑相似度高的
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

export function useSuggestions() {
  return {
    corpus,
    addTitles,
    clearCorpus,
    suggest
  }
}
