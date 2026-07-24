import { ref, shallowRef } from 'vue'
import { search as searchApi, flattenResults } from '@/api/search.js'
import { checkLinks } from '@/api/check.js'
import { useSuggestions } from './useSuggestions.js'

/**
 * 将后端检测状态映射为前端统一状态
 */
function mapCheckState(backendState) {
  switch (backendState) {
    case 'ok': return 'valid'
    case 'bad': return 'invalid'
    case 'locked': return 'expired'
    case 'unsupported': return 'unsupported'
    case 'uncertain': return 'error'
    default: return 'error'
  }
}

/**
 * 计算字符串 a 与 b 的相似度分数（越大约相似）
 * 包含关系优先，其次最长公共子串长度
 */
function textSimilarity(a, b) {
  if (!a || !b) return 0
  const la = a.toLowerCase()
  const lb = b.toLowerCase()
  if (lb.includes(la)) return 100 + la.length / Math.max(1, lb.length)
  if (la.includes(lb)) return 60 + lb.length / Math.max(1, la.length)
  let common = 0
  const set = new Set(la)
  for (const ch of lb) if (set.has(ch)) common++
  return common
}

/**
 * 对搜索结果按与关键词的相似度重排序，让最匹配的排在前面
 * 模糊搜索：即使关键词打错，也会让标题最接近的结果排到前面
 */
function sortByRelevance(results, kw) {
  if (!kw) return results
  const k = kw.trim()
  if (!k) return results
  return results
    .map(item => {
      const title = (item.note || item.title || '').toString()
      const score = Math.max(
        textSimilarity(k, title),
        textSimilarity(k, (item.title || '').toString())
      )
      return { item, score }
    })
    .sort((a, b) => b.score - a.score)
    .map(x => x.item)
}

/**
 * 搜索逻辑封装：loading / error / 结果 / 取消请求 / 自动批量检测链接
 */
export function useSearch() {
  const loading = ref(false)
  const error = ref(null)
  const keyword = ref('')
  // 搜索耗时（毫秒），用于结果区展示
  const duration = ref(0)
  // 后端返回的原始分组数据
  const mergedByType = shallowRef({})
  // 拍平后的统一数组
  const flatResults = ref([])
  const total = ref(0)

  // 链接检测状态数组，与 flatResults 一一对应
  // 每项：null(未检测) / 'checking' / 'valid' / 'invalid' / 'expired' / 'error' / 'unsupported'
  const checkStates = ref([])
  // 链接检测摘要数组，与 flatResults 一一对应
  const checkSummaries = ref([])
  const checking = ref(false)

  let controller = null
  let checkController = null
  // 每次 run 自增序号，用于识别过期请求
  let seq = 0
  let checkSeq = 0

  async function run(kw, options = {}) {
    const trimmed = (kw || '').trim()
    if (!trimmed) return

    // 取消上一次未完成请求（搜索 + 检测）
    cancel()
    controller = new AbortController()
    checkController = new AbortController()
    const mySeq = ++seq

    loading.value = true
    error.value = null
    keyword.value = trimmed
    // 清空旧结果，便于骨架屏展示
    flatResults.value = []
    mergedByType.value = {}
    total.value = 0
    checkStates.value = []
    checkSummaries.value = []
    checking.value = false

    const start = performance.now()

    let results = []
    try {
      const data = await searchApi(
        { kw: trimmed, ...options },
        controller.signal
      )
      // 过期请求：已经有更新的 run 启动了，丢弃本次结果
      if (mySeq !== seq) return

      mergedByType.value = data?.merged_by_type || {}
      total.value = data?.total || 0
      results = flattenResults(mergedByType.value)
      // 模糊搜索：按与关键词的相似度重排序
      results = sortByRelevance(results, trimmed)
      flatResults.value = results
      duration.value = Math.round(performance.now() - start)

      // 把结果标题加入联想词库
      const { addTitles } = useSuggestions()
      addTitles(results.map(r => r.note || r.title || '').filter(Boolean))
    } catch (e) {
      if (mySeq !== seq) return
      // 主动取消不打错误
      if (e.name !== 'CanceledError' && e.code !== 'ERR_CANCELED') {
        error.value = e
      }
      loading.value = false
      return
    }

    // 搜索完成，loading 结束
    if (mySeq === seq) {
      loading.value = false
    }
  }

  /**
   * 分批检测指定索引的链接
   * @param {number[]} indices flatResults 中的索引
   * @param {AbortSignal} [signal]
   */
  async function batchCheck(indices, signal) {
    if (!indices.length) return

    const mySeq = ++checkSeq
    const usedSignal = signal || checkController?.signal

    // 去重并过滤掉已检测的
    const uniqueIdx = [...new Set(indices)].filter(i => {
      return flatResults.value[i] && checkStates.value[i] == null
    })
    if (!uniqueIdx.length) return

    // 标记为检测中
    uniqueIdx.forEach(i => { checkStates.value[i] = 'checking' })
    checking.value = true

    const batchSize = 30
    try {
      for (let i = 0; i < uniqueIdx.length; i += batchSize) {
        if (usedSignal?.aborted || mySeq !== checkSeq) return

        const batchIdx = uniqueIdx.slice(i, i + batchSize)
        const items = batchIdx.map(idx => ({
          disk_type: flatResults.value[idx].cloudType,
          url: flatResults.value[idx].url,
          password: flatResults.value[idx].password || ''
        }))

        const data = await checkLinks(items, usedSignal)
        if (usedSignal?.aborted || mySeq !== checkSeq) return

        const arr = data?.results || []
        batchIdx.forEach((idx, j) => {
          const result = arr[j] || {}
          checkStates.value[idx] = mapCheckState(result.state)
          checkSummaries.value[idx] = result.summary || ''
        })
      }
    } catch (e) {
      if (e.name === 'CanceledError' || e.code === 'ERR_CANCELED') return
      uniqueIdx.forEach(idx => {
        if (checkStates.value[idx] === 'checking') {
          checkStates.value[idx] = 'error'
          checkSummaries.value[idx] = '检测失败'
        }
      })
    } finally {
      if (mySeq === checkSeq) checking.value = false
    }
  }

  function cancel() {
    if (controller) {
      controller.abort()
      controller = null
    }
    if (checkController) {
      checkController.abort()
      checkController = null
    }
  }

  function reset() {
    cancel()
    seq++ // 让进行中的搜索请求变过期
    checkSeq++ // 让进行中的检测请求变过期
    loading.value = false
    checking.value = false
    error.value = null
    keyword.value = ''
    mergedByType.value = {}
    flatResults.value = []
    checkStates.value = []
    checkSummaries.value = []
    total.value = 0
    duration.value = 0
  }

  return {
    loading,
    error,
    keyword,
    duration,
    mergedByType,
    flatResults,
    total,
    checkStates,
    checkSummaries,
    checking,
    run,
    batchCheck,
    cancel,
    reset
  }
}
