<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import SearchBar from '@/components/SearchBar.vue'
import ResultTabs from '@/components/ResultTabs.vue'
import ResultCard from '@/components/ResultCard.vue'
import EmptyState from '@/components/EmptyState.vue'
import SkeletonCard from '@/components/SkeletonCard.vue'
import { useSearch } from '@/composables/useSearch.js'
import { useHistory } from '@/composables/useHistory.js'
import { useConfigStore } from '@/stores/config.js'
import { health } from '@/api/search.js'

const route = useRoute()
const router = useRouter()
const configStore = useConfigStore()

const {
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
} = useSearch()

const { addHistory } = useHistory()

// ==================== 排序 ====================
const sortMode = ref('relevance') // 'relevance' | 'time'

// ==================== Tab ====================
const activeTab = ref('all')

// ==================== 分页 ====================
const pageSize = 20
const currentPage = ref(1)

// 过滤后的原始列表（保持索引映射）
const filteredList = computed(() => {
  let list = flatResults.value.map((item, origIdx) => ({ item, origIdx }))
  if (activeTab.value !== 'all') {
    list = list.filter(({ item }) => item.cloudType === activeTab.value)
  }
  return list
})

// 按当前排序模式排序
const sortedList = computed(() => {
  let list = filteredList.value
  if (sortMode.value === 'time') {
    list = [...list].sort((a, b) => {
      const ta = Date.parse(a.item.datetime) || 0
      const tb = Date.parse(b.item.datetime) || 0
      return tb - ta
    })
  }
  return list
})

// 分页后的可见项
const totalPages = computed(() => Math.max(1, Math.ceil(sortedList.value.length / pageSize)))

const visibleItems = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  const end = start + pageSize
  return sortedList.value.slice(start, end).map(({ item, origIdx }) => ({
    item,
    origIdx,
    state: checkStates.value[origIdx] || null,
    summary: checkSummaries.value[origIdx] || ''
  }))
})

// 分页按钮范围（当前页前后各2页）
const pageNumbers = computed(() => {
  const total = totalPages.value
  const current = currentPage.value
  const delta = 2
  const pages = []
  for (let i = Math.max(1, current - delta); i <= Math.min(total, current + delta); i++) {
    pages.push(i)
  }
  return pages
})

// 过滤后的总数
const filteredTotal = computed(() => {
  if (activeTab.value === 'all') return total.value
  return (mergedByType.value[activeTab.value] || []).length
})

// 切换排序或 Tab 时重置到第1页
watch([sortMode, activeTab], () => {
  currentPage.value = 1
})

// ==================== 搜索 ====================
const hasSearched = computed(() => Boolean(keyword.value))
const showSkeleton = computed(() => loading.value)

const emptyType = computed(() => {
  if (loading.value) return ''
  if (error.value) return 'error'
  if (visibleItems.value.length > 0) return ''
  return hasSearched.value ? 'empty' : 'initial'
})

const heroSubtitle = computed(() => '聚合上百个 TG 频道 · 几十个搜索源')

function doSearch(kw) {
  if (!kw) return
  activeTab.value = 'all'
  sortMode.value = 'relevance'
  currentPage.value = 1
  addHistory(kw)
  router.replace({ query: { ...route.query, q: kw } })
  run(kw)
}

// 切换数据源后自动重新搜索
function switchSource(s) {
  if (source.value === s) return
  source.value = s
  if (keyword.value) {
    doSearch(keyword.value)
  }
}

function goPage(page) {
  const p = Math.max(1, Math.min(page, totalPages.value))
  if (p !== currentPage.value) currentPage.value = p
}

function retry() {
  if (keyword.value) doSearch(keyword.value)
}

// 返回首页：清空搜索状态与 URL query
function goHome() {
  cancel()
  reset()
  currentPage.value = 1
  activeTab.value = 'all'
  sortMode.value = 'relevance'
  router.replace({ query: { ...route.query, q: undefined } })
}

// ==================== QQ 模态框 ====================
const qqDialogVisible = ref(false)
const qqNumber = '2938603490'

function copyQQ() {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(qqNumber).then(() => {
      ElMessage.success('QQ号已复制到剪贴板')
    }).catch(() => {
      fallbackCopyQQ()
    })
  } else {
    fallbackCopyQQ()
  }
}

function fallbackCopyQQ() {
  const ta = document.createElement('textarea')
  ta.value = qqNumber
  ta.style.position = 'fixed'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.select()
  document.execCommand('copy')
  document.body.removeChild(ta)
  ElMessage.success('QQ号已复制到剪贴板')
}

// ==================== 当前页可见项分批检测 ====================
let checkTimer = null
function scheduleVisibleCheck() {
  clearTimeout(checkTimer)
  checkTimer = setTimeout(() => {
    const indices = visibleItems.value
      .filter(v => v.state == null)
      .map(v => v.origIdx)
    if (indices.length) batchCheck(indices)
  }, 250)
}

watch(
  [flatResults, activeTab, sortMode, currentPage],
  () => { scheduleVisibleCheck() },
  { immediate: true }
)

// ==================== 初始化 ====================
onMounted(async () => {
  try {
    const info = await health()
    configStore.setHealthInfo(info || {})
  } catch {
    // 静默失败
  }

  const q = route.query.q
  if (q && typeof q === 'string') {
    doSearch(q)
  }
})
onUnmounted(() => {
  clearTimeout(checkTimer)
  cancel()
})
watch(
  () => route.query.q,
  (q) => {
    if (q && q !== keyword.value) {
      doSearch(String(q))
    }
  }
)
</script>

<template>
  <div class="page">
    <!-- 顶栏 -->
    <header class="header">
      <div class="container header__inner">
        <div class="header__logo" @click="goHome" title="返回首页">
          <span class="header__logo-icon">🔍</span>
          <span class="header__logo-text">搜盘大师</span>
        </div>
        <div class="header__right">
          <span v-if="configStore.authEnabled && configStore.isLoggedIn" class="header__user">
            👤 {{ configStore.username }}
          </span>
          <!-- 联系作者 -->
          <button class="qq-btn" title="联系作者" @click="qqDialogVisible = true">
            <svg viewBox="0 0 1024 1024" width="18" height="18" fill="currentColor">
              <path d="M824.8 613.2c-16-12.8-35.2-22.4-56-28.8 12.8-44.8 19.2-92.8 19.2-144 0-214.4-128-387.2-288-387.2S211.2 227.2 211.2 441.6c0 51.2 6.4 99.2 19.2 144-20.8 6.4-40 16-56 28.8-41.6 32-67.2 76.8-67.2 126.4 0 22.4 6.4 44.8 17.6 64 25.6 44.8 76.8 76.8 137.6 89.6 9.6 1.6 19.2 3.2 28.8 3.2 6.4 0 12.8-1.6 17.6-4.8 12.8-8 17.6-24 11.2-37.6l-11.2-22.4c28.8 8 59.2 12.8 92.8 12.8 33.6 0 64-4.8 92.8-12.8l-11.2 22.4c-6.4 14.4-1.6 30.4 11.2 37.6 4.8 3.2 11.2 4.8 17.6 4.8 9.6 0 19.2-1.6 28.8-3.2 60.8-12.8 112-44.8 137.6-89.6 11.2-19.2 17.6-41.6 17.6-64 0-49.6-25.6-94.4-67.2-126.4zM512 128c128 0 233.6 140.8 233.6 313.6 0 56-11.2 108.8-30.4 155.2-48-19.2-105.6-30.4-166.4-33.6-3.2 0-6.4 0-9.6 1.6-12.8 4.8-20.8 16-20.8 28.8 0 14.4 11.2 27.2 25.6 28.8 52.8 3.2 100.8 12.8 142.4 28.8-30.4 73.6-96 124.8-174.4 124.8-78.4 0-144-51.2-174.4-124.8 41.6-16 89.6-25.6 142.4-28.8 14.4-1.6 25.6-14.4 25.6-28.8 0-12.8-8-24-20.8-28.8-3.2-1.6-6.4-1.6-9.6-1.6-60.8 3.2-118.4 14.4-166.4 33.6C289.6 549.6 278.4 497.6 278.4 441.6c0-172.8 105.6-313.6 233.6-313.6z"/>
            </svg>
            <span>联系作者</span>
          </button>
        </div>
      </div>
    </header>

    <!-- Hero 搜索区 -->
    <section class="hero" :class="{ 'hero--compact': hasSearched }">
      <div class="container">
        <h1 class="hero__title">
          <span class="hero__title-gradient">网盘资源搜索</span>
        </h1>
        <p v-if="!hasSearched" class="hero__subtitle">{{ heroSubtitle }}</p>

        <div class="hero__search">
          <!-- 数据源切换 -->
          <div class="source-switch">
            <button
              class="source-btn"
              :class="{ active: source === 'aggregate' }"
              @click="switchSource('aggregate')"
            >
              <span class="source-btn__icon">🚀</span>
              <span class="source-btn__text">聚合搜索</span>
              <span class="source-btn__desc">极速 · 多源聚合</span>
            </button>
            <button
              class="source-btn"
              :class="{ active: source === 'tg' }"
              @click="switchSource('tg')"
            >
              <span class="source-btn__icon">📡</span>
              <span class="source-btn__text">频道搜索</span>
              <span class="source-btn__desc">118个TG频道</span>
            </button>
            <button
              class="source-btn"
              :class="{ active: source === 'plugin' }"
              @click="switchSource('plugin')"
            >
              <span class="source-btn__icon">🔍</span>
              <span class="source-btn__text">插件搜索</span>
              <span class="source-btn__desc">11个精选插件</span>
            </button>
          </div>
          <SearchBar v-model="keyword" :loading="loading" @search="doSearch" />
        </div>
      </div>
    </section>

    <!-- 结果区 -->
    <main class="container main">
      <!-- 搜索中：骨架屏 -->
      <SkeletonCard v-if="showSkeleton" :count="6" />

      <!-- 错误 / 空 / 初始 -->
      <EmptyState
        v-else-if="emptyType"
        :type="emptyType"
        :keyword="keyword"
        @retry="retry"
      />

      <!-- 结果展示 -->
      <div v-else>
        <!-- 结果统计 + 排序 + 用时 -->
        <div class="result-meta">
          <span class="result-meta__count">
            找到 <strong>{{ filteredTotal }}</strong> 条结果
          </span>

          <!-- 排序切换 -->
          <div class="sort-bar">
            <button
              class="sort-btn"
              :class="{ active: sortMode === 'relevance' }"
              @click="sortMode = 'relevance'"
            >
              相关度
            </button>
            <button
              class="sort-btn"
              :class="{ active: sortMode === 'time' }"
              @click="sortMode = 'time'"
            >
              时间
            </button>
          </div>

          <span v-if="duration > 0" class="result-meta__time">用时 {{ (duration / 1000).toFixed(2) }}s</span>
        </div>

        <!-- 后台加载中提示 -->
        <div v-if="loading" class="result-loading">
          <span class="result-loading__dot" /> 更新中...
        </div>
        <div v-else-if="checking" class="result-loading">
          <span class="result-loading__dot" /> 正在检测链接有效性...
        </div>

        <!-- 网盘类型 Tab -->
        <ResultTabs v-model="activeTab" :merged-by-type="mergedByType" />

        <!-- 卡片网格：固定3列 -->
        <div class="result-grid">
          <ResultCard
            v-for="(entry, idx) in visibleItems"
            :key="entry.item.url + idx"
            :item="entry.item"
            :keyword="keyword"
            :index="idx"
            :check-state="entry.state"
            :check-summary="entry.summary"
          />
        </div>

        <!-- 分页 -->
        <div v-if="totalPages > 1" class="pagination">
          <button
            class="page-btn"
            :disabled="currentPage === 1"
            @click="goPage(currentPage - 1)"
          >
            上一页
          </button>

          <button
            v-if="pageNumbers[0] > 1"
            class="page-btn"
            @click="goPage(1)"
          >
            1
          </button>
          <span v-if="pageNumbers[0] > 2" class="page-ellipsis">...</span>

          <button
            v-for="p in pageNumbers"
            :key="p"
            class="page-btn"
            :class="{ active: p === currentPage }"
            @click="goPage(p)"
          >
            {{ p }}
          </button>

          <span v-if="pageNumbers[pageNumbers.length - 1] < totalPages - 1" class="page-ellipsis">...</span>
          <button
            v-if="pageNumbers[pageNumbers.length - 1] < totalPages"
            class="page-btn"
            @click="goPage(totalPages)"
          >
            {{ totalPages }}
          </button>

          <button
            class="page-btn"
            :disabled="currentPage === totalPages"
            @click="goPage(currentPage + 1)"
          >
            下一页
          </button>

          <!-- 跳转 -->
          <span class="page-jump">
            跳至
            <input
              type="number"
              min="1"
              :max="totalPages"
              class="page-jump-input"
              @keyup.enter="(e) => goPage(Number(e.target.value))"
            />
            页
          </span>
        </div>

        <div v-else-if="visibleItems.length > 0" class="load-end">
          共 {{ filteredTotal }} 条结果
        </div>
      </div>
    </main>

    <!-- 页脚 -->
    <footer class="footer">
      <div class="container">
        <span>搜盘大师 · 网盘资源聚合搜索</span>
        <span class="footer__sep">·</span>
        <span>仅供学习交流，请勿用于商业用途</span>
      </div>
    </footer>

    <!-- QQ 模态框 -->
    <el-dialog
      v-model="qqDialogVisible"
      title="联系作者"
      width="360px"
      align-center
    >
      <div class="qq-dialog-body">
        <div class="qq-avatar">🐧</div>
        <p class="qq-label">QQ 号码</p>
        <p class="qq-number">{{ qqNumber }}</p>
        <button class="qq-copy-btn" @click="copyQQ">
          <span>📋 复制QQ号</span>
        </button>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* 顶栏 */
.header {
  position: sticky;
  top: 0;
  z-index: 50;
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: saturate(180%) blur(12px);
  -webkit-backdrop-filter: saturate(180%) blur(12px);
  border-bottom: 1px solid var(--color-border);
}

.header__inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 60px;
}

.header__logo {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: var(--font-display);
  font-weight: 800;
  font-size: 20px;
  color: var(--color-text);
  cursor: pointer;
  user-select: none;
  transition: opacity var(--transition-fast);
}

.header__logo:hover {
  opacity: 0.75;
}

.header__logo-icon {
  font-size: 22px;
}

.header__right {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 14px;
}

.header__user {
  color: var(--color-text-secondary);
}

.qq-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 6px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-card);
  color: var(--color-text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.qq-btn:hover {
  border-color: #12b7f5;
  color: #12b7f5;
  background: rgba(18, 183, 245, 0.06);
}

/* Hero */
.hero {
  padding: 70px 0 40px;
  text-align: center;
  background: linear-gradient(180deg, var(--color-primary-light) 0%, transparent 100%);
  transition: padding var(--transition-normal);
}

.hero--compact {
  padding: 36px 0 24px;
}

.hero__title {
  margin: 0 0 12px;
  font-family: var(--font-display);
  font-size: 44px;
  font-weight: 800;
  line-height: 1.2;
  letter-spacing: -0.02em;
}

.hero--compact .hero__title {
  font-size: 30px;
  margin-bottom: 8px;
}

.hero__title-gradient {
  background: linear-gradient(135deg, var(--color-primary) 0%, #6366f1 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.hero__subtitle {
  margin: 0 0 32px;
  font-size: 16px;
  color: var(--color-text-secondary);
}

.hero__search {
  max-width: 720px;
  margin: 0 auto;
}

/* 数据源切换 */
.source-switch {
  display: flex;
  gap: 12px;
  justify-content: center;
  margin-bottom: 16px;
}

.source-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: 1.5px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-card);
  cursor: pointer;
  transition: all var(--transition-normal);
  position: relative;
  overflow: hidden;
}

.source-btn:hover:not(.active) {
  border-color: var(--color-primary);
  background: var(--color-primary-light);
}

.source-btn.active {
  border-color: var(--color-primary);
  background: linear-gradient(135deg, var(--color-primary) 0%, #6366f1 100%);
  color: #fff;
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
}

.source-btn__icon {
  font-size: 18px;
  line-height: 1;
}

.source-btn__text {
  font-size: 15px;
  font-weight: 600;
}

.source-btn__desc {
  font-size: 12px;
  opacity: 0.7;
  font-weight: 400;
}

.source-btn.active .source-btn__desc {
  opacity: 0.85;
}

/* 主区域 */
.main {
  flex: 1;
  padding-top: 28px;
  padding-bottom: 60px;
}

.result-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 18px;
  font-size: 14px;
  color: var(--color-text-secondary);
}

.result-meta__count strong {
  color: var(--color-text);
  font-size: 16px;
  margin: 0 2px;
}

.result-meta__time {
  font-family: var(--font-mono);
  font-size: 13px;
}

/* 排序切换 */
.sort-bar {
  display: flex;
  width: 100%;
  max-width: 200px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.sort-btn {
  flex: 1;
  text-align: center;
  padding: 5px 0;
  border: none;
  background: var(--color-card);
  color: var(--color-text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
}
.sort-bar {
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.sort-btn {
  padding: 5px 14px;
  border: none;
  background: var(--color-card);
  color: var(--color-text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.sort-btn.active {
  background: var(--color-primary);
  color: #fff;
}

.sort-btn:hover:not(.active) {
  background: var(--color-primary-light);
  color: var(--color-primary);
}

.result-loading {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 14px;
  font-size: 13px;
  color: var(--color-primary);
}

.result-loading__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-primary);
  animation: pulse 1s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 0.4; transform: scale(0.9); }
  50% { opacity: 1; transform: scale(1.1); }
}

/* 卡片网格：固定3列 */
.result-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-top: 16px;
}

/* 分页 */
.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 36px;
}

.page-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 34px;
  height: 34px;
  padding: 0 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-card);
  color: var(--color-text);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.page-btn:hover:not(:disabled):not(.active) {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.page-btn.active {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #fff;
  font-weight: 600;
}

.page-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.page-ellipsis {
  color: var(--color-text-secondary);
  font-size: 13px;
  padding: 0 4px;
}

.page-jump {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: 8px;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.page-jump-input {
  width: 44px;
  height: 28px;
  padding: 0 4px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  text-align: center;
  font-size: 13px;
  color: var(--color-text);
  background: var(--color-card);
}

.page-jump-input:focus {
  outline: none;
  border-color: var(--color-primary);
}

.load-end {
  text-align: center;
  margin-top: 36px;
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* 页脚 */
.footer {
  border-top: 1px solid var(--color-border);
  padding: 20px 0;
  font-size: 13px;
  color: var(--color-text-secondary);
  text-align: center;
}

.footer .container {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex-wrap: wrap;
}

.footer__sep {
  opacity: 0.5;
}

/* QQ 模态框 */
.qq-dialog-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 12px 0;
}

.qq-avatar {
  font-size: 48px;
  line-height: 1;
}

.qq-label {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.qq-number {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  font-family: var(--font-mono);
  color: var(--color-text);
  letter-spacing: 2px;
}

.qq-copy-btn {
  margin-top: 8px;
  padding: 8px 20px;
  border: 1px solid var(--color-primary);
  border-radius: var(--radius-lg);
  background: var(--color-primary);
  color: #fff;
  font-size: 14px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.qq-copy-btn:hover {
  background: var(--color-primary-hover);
  border-color: var(--color-primary-hover);
}

/* 响应式 */
@media (max-width: 768px) {
  .hero {
    padding: 40px 0 28px;
  }
  .hero__title {
    font-size: 32px;
  }
  .hero--compact .hero__title {
    font-size: 24px;
  }
  .hero__subtitle {
    font-size: 14px;
    margin-bottom: 22px;
  }
  .source-switch {
    flex-direction: column;
    gap: 8px;
  }
  .source-btn {
    justify-content: center;
    padding: 8px 16px;
  }
  .source-btn__desc {
    display: none;
  }
  .result-grid {
    grid-template-columns: 1fr;
  }
}

@media (min-width: 769px) and (max-width: 1100px) {
  .result-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
