import { ref, computed } from 'vue'

const STORAGE_KEY = 'pansou_search_history'
const MAX_HISTORY = 20

// 模块级共享状态：多个组件复用同一份历史
const history = ref(loadHistory())

function loadHistory() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
}

function saveHistory() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(history.value))
  } catch {
    // localStorage 满或禁用，忽略
  }
}

export function useHistory() {
  const hasHistory = computed(() => history.value.length > 0)

  function addHistory(keyword) {
    const trimmed = (keyword || '').trim()
    if (!trimmed) return
    // 去重：移除已存在的同名记录
    history.value = history.value.filter(k => k !== trimmed)
    // 加入到头部
    history.value.unshift(trimmed)
    // 截断到上限
    if (history.value.length > MAX_HISTORY) {
      history.value = history.value.slice(0, MAX_HISTORY)
    }
    saveHistory()
  }

  function removeHistory(keyword) {
    history.value = history.value.filter(k => k !== keyword)
    saveHistory()
  }

  function clearHistory() {
    history.value = []
    saveHistory()
  }

  return {
    history,
    hasHistory,
    addHistory,
    removeHistory,
    clearHistory
  }
}
