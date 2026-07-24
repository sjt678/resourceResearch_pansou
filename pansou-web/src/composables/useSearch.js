import { ref, shallowRef } from 'vue'
import { search as searchApi, flattenResults } from '@/api/search.js'
import { checkLinks } from '@/api/check.js'

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

  // 链接检测结果数组，与 flatResults 一一对应
  // 每项：{ state: string, summary: string }
  // state: null(未检测) / 'checking' / 'valid' / 'invalid' / 'expired' / 'error' / 'unsupported'
  const checkResults = ref([])
  const checking = ref(false)

  let controller = null
  // 每次 run 自增序号，用于识别过期请求
  let seq = 0

  async function run(kw, options = {}) {
    const trimmed = (kw || '').trim()
    if (!trimmed) return

    // 取消上一次未完成请求（搜索 + 检测共用一个 controller）
    if (controller) controller.abort()
    controller = new AbortController()
    const mySeq = ++seq

    loading.value = true
    error.value = null
    keyword.value = trimmed
    // 清空旧结果，便于骨架屏展示
    flatResults.value = []
    mergedByType.value = {}
    total.value = 0
    checkResults.value = []
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
      flatResults.value = results
      duration.value = Math.round(performance.now() - start)
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

    // ===== 自动批量检测所有链接 =====
    if (results.length && mySeq === seq) {
      checking.value = true
      // 先全部置为 'checking'，让圆点显示脉冲
      checkResults.value = results.map(() => ({ state: 'checking', summary: '正在检测...' }))

      try {
        const data = await checkLinks(
          results.map(it => ({
            disk_type: it.cloudType,
            url: it.url,
            password: it.password || ''
          })),
          controller.signal
        )
        if (mySeq !== seq) return
        const arr = data?.results || []
        checkResults.value = results.map((_, i) => {
          const r = arr[i]
          if (!r) return { state: 'error', summary: '检测失败' }
          return {
            state: r.state || 'error',
            summary: r.summary || defaultSummary(r.state)
          }
        })
      } catch (e) {
        if (mySeq !== seq) return
        if (e.name !== 'CanceledError' && e.code !== 'ERR_CANCELED') {
          // 检测失败：全部标记为 error
          checkResults.value = results.map(() => ({ state: 'error', summary: '检测失败' }))
        }
      } finally {
        if (mySeq === seq) {
          checking.value = false
        }
      }
    }
  }

  function defaultSummary(state) {
    switch (state) {
      case 'valid': return '链接有效'
      case 'invalid':
      case 'expired': return '链接已失效'
      case 'unsupported': return '当前平台暂不支持检测'
      case 'error': return '检测异常'
      default: return '未知'
    }
  }

  function cancel() {
    if (controller) {
      controller.abort()
      controller = null
    }
  }

  function reset() {
    cancel()
    seq++ // 让进行中的请求变过期
    loading.value = false
    checking.value = false
    error.value = null
    keyword.value = ''
    mergedByType.value = {}
    flatResults.value = []
    checkResults.value = []
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
    checkResults,
    checking,
    run,
    cancel,
    reset
  }
}
