import { ref, shallowRef } from 'vue'
import { search as searchApi, flattenResults } from '@/api/search.js'
import { checkLinks } from '@/api/check.js'

function defaultSummary(state) {
  switch (state) {
    case 'valid': return '链接有效'
    case 'invalid':
    case 'expired': return '链接已失效'
    case 'locked': return '需要提取码'
    case 'unsupported': return '当前平台暂不支持检测'
    case 'uncertain': return '暂不可知'
    case 'error': return '检测异常'
    default: return '未知'
  }
}

// 后端状态值 → 前端状态值映射
// 后端返回: ok / bad / locked / unsupported / uncertain
// 前端期望: valid / invalid / locked / unsupported / uncertain / error
function mapBackendState(backendState) {
  switch (backendState) {
    case 'ok': return 'valid'
    case 'bad': return 'invalid'
    case 'locked': return 'locked'
    case 'unsupported': return 'unsupported'
    case 'uncertain': return 'uncertain'  // 独立状态：暂不可知（后端无法判定）
    default: return 'error'
  }
}

/**
 * 搜索逻辑封装：loading / error / 结果 / 取消请求 / 自动批量检测链接 / 手动再检测
 */
export function useSearch() {
  const loading = ref(false)
  const error = ref(null)
  const keyword = ref('')
  // 数据源：'aggregate'(聚合-mipan) | 'tg'(TG频道) | 'plugin'(其余插件)
  const source = ref('aggregate')
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
  // 链接检测提示文本，与 flatResults 一一对应
  const checkSummaries = ref([])
  const checking = ref(false)

  let controller = null
  // 每次 run/batchCheck 自增序号，用于识别过期请求
  let seq = 0

  function setCheckItem(i, item) {
    if (i < 0) return
    const s = checkStates.value
    const m = checkSummaries.value
    while (s.length < i + 1) s.push(null)
    while (m.length < i + 1) m.push('')
    s[i] = item.state
    m[i] = item.summary
    // 触发响应式更新
    checkStates.value = [...s]
    checkSummaries.value = [...m]
  }

  async function doCheck(indices, signal) {
    if (!indices?.length) return
    const targets = indices.filter(i => i >= 0 && i < flatResults.value.length)
    if (!targets.length) return

    const mySeq = seq
    // 先把这几个标为 checking
    targets.forEach(i => setCheckItem(i, { state: 'checking', summary: '正在检测...' }))
    checking.value = true

    try {
      const data = await checkLinks(
        targets.map(i => {
          const it = flatResults.value[i]
          return {
            disk_type: it.cloudType,
            url: it.url,
            password: it.password || ''
          }
        }),
        signal
      )
      if (mySeq !== seq) return
      const arr = data?.results || []
      targets.forEach((i, k) => {
        const r = arr[k]
        if (!r) {
          setCheckItem(i, { state: 'error', summary: '检测失败' })
          return
        }
        const mappedState = mapBackendState(r.state)
        setCheckItem(i, {
          state: mappedState,
          summary: r.summary || defaultSummary(mappedState)
        })
      })
    } catch (e) {
      if (mySeq !== seq) return
      if (e.name !== 'CanceledError' && e.code !== 'ERR_CANCELED') {
        targets.forEach(i =>
          setCheckItem(i, { state: 'error', summary: '检测失败' })
        )
      }
    } finally {
      if (mySeq === seq) checking.value = false
    }
  }

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
    checkStates.value = []
    checkSummaries.value = []
    checking.value = false

    const start = performance.now()

    let results = []
    try {
      // 根据数据源构建搜索参数
      const searchParams = { kw: trimmed, refresh: true, ...options }
      if (source.value === 'aggregate') {
        // 聚合搜索：只用mipan插件
        searchParams.src = 'plugin'
        searchParams.plugins = ['mipan']
      } else if (source.value === 'tg') {
        // 频道搜索：只搜TG频道
        searchParams.src = 'tg'
      } else {
        // 插件搜索：精选可用插件（不含mipan）
        // 2026-07-28 对 61 个插件实测（关键词"仙逆"），仅以下 11 个有返回，其余 50 个全部 0 条（站点失效/反爬/需代理）
        searchParams.src = 'plugin'
        searchParams.plugins = ['duoduo','qupanshe','muou','wanou','ouge','ash','quark4k','hunhepan','xiaozhang','yuhuage','u3c3']
      }
      const data = await searchApi(searchParams, controller.signal)
      // 过期请求：已经有更新的 run 启动了，丢弃本次结果
      if (mySeq !== seq) return

      mergedByType.value = data?.merged_by_type || {}
      total.value = data?.total || 0
      results = flattenResults(mergedByType.value)
      // 给结果打上数据源标签，方便用户识别结果来源
      const sourceTag = source.value === 'aggregate' ? '聚合' 
                      : source.value === 'tg' ? 'TG频道' 
                      : '插件'
      results = results.map(r => ({ ...r, _source: sourceTag }))
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
    // 懒检测：搜索完成后不自动检测，由 Home.vue 在翻页时检测当前页可见项
  }

  /**
   * 检测指定下标的链接（Home.vue 翻页时调用，只检测当前页）
   */
  async function batchCheck(indices) {
    if (!indices?.length) return
    // 取消旧检测任务，避免竞态
    if (controller) controller.abort()
    controller = new AbortController()
    const mySeq = ++seq
    await doCheck(indices, controller.signal)
    void mySeq
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
    checkStates.value = []
    checkSummaries.value = []
    total.value = 0
    duration.value = 0
  }

  return {
    loading,
    error,
    keyword,
    source,
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
