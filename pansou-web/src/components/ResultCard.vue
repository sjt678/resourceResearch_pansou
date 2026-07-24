<script setup>
import { ref, computed } from 'vue'
import { CopyDocument, Check } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getCloudMeta } from '@/utils/cloud.js'
import { highlight, formatRelativeTime, formatDate } from '@/utils/format.js'

const props = defineProps({
  item: { type: Object, required: true },
  keyword: { type: String, default: '' },
  // 错峰动画延迟（毫秒）
  index: { type: Number, default: 0 },
  // 链接检测状态：null / 'checking' / 'valid' / 'invalid' / 'expired' / 'error' / 'unsupported'
  checkState: { type: String, default: null },
  // 链接检测摘要
  checkSummary: { type: String, default: '' }
})

const cloud = computed(() => getCloudMeta(props.item.cloudType))
const note = computed(() => props.item.note || '未命名资源')

const copied = ref(false)

async function copyLink() {
  const text = buildCopyText()
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
    } else {
      // 兜底：临时 textarea
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    copied.value = true
    setTimeout(() => (copied.value = false), 600)
  } catch {
    ElMessage.error('复制失败，请手动选择链接复制')
  }
}

function buildCopyText() {
  const url = props.item.url || ''
  const pwd = props.item.password
  if (pwd) return `${url} 提取码：${pwd}`
  return url
}

function openLink() {
  const url = props.item.url
  if (url) window.open(url, '_blank', 'noopener,noreferrer')
}

const dotTitle = computed(() => {
  const summary = props.checkSummary
  switch (props.checkState) {
    case 'checking': return '检测中...'
    case 'valid': return summary || '链接有效'
    case 'invalid': return summary || '链接已失效'
    case 'expired': return summary || '链接已失效'
    case 'unsupported': return summary || '暂不支持检测'
    case 'error': return summary || '检测异常'
    default: return '待检测'
  }
})
</script>

<template>
  <article
    class="card"
    :class="[`state--${checkState || 'none'}`, `idx-${index}`]"
  >
    <header class="card__head">
      <span class="card__cloud" :style="{ background: cloud.color }">{{ cloud.icon }}</span>
      <h3 class="card__title" v-html="highlight(note, keyword)" />
      <!-- 状态圆点 -->
      <span
        class="card__dot"
        :class="`dot--${checkState || 'idle'}`"
        :title="dotTitle"
      />
    </header>

    <p
      v-if="item.content"
      class="card__content"
      v-html="highlight(item.content, keyword)"
    />

    <!-- 检测状态摘要 -->
    <p
      v-if="checkSummary && checkState !== 'valid'"
      class="card__check-summary"
      :class="`summary--${checkState}`"
    >
      {{ checkSummary }}
    </p>

    <div class="card__meta">
      <span class="card__meta-item">
        <span class="card__cloud-label" :style="{ color: cloud.color }">{{ cloud.label }}网盘</span>
      </span>
      <span v-if="item.datetime" class="card__meta-item card__date" :title="formatDate(item.datetime)">
        📅 {{ formatDate(item.datetime) }}
      </span>
      <span v-if="item.datetime" class="card__meta-item card__relative">
        {{ formatRelativeTime(item.datetime) }}
      </span>
    </div>

    <div v-if="item.password" class="card__pwd">
      提取码：<code>{{ item.password }}</code>
    </div>

    <footer class="card__actions">
      <button class="btn btn--open" @click="openLink">
        <span>打开链接</span>
      </button>

      <button class="btn btn--copy" :class="{ 'is-copied': copied }" @click="copyLink">
        <el-icon><Check v-if="copied" /><CopyDocument v-else /></el-icon>
        <span>{{ copied ? '已复制' : '复制链接' }}</span>
      </button>
    </footer>
  </article>
</template>

<style scoped>
.card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast),
    transform var(--transition-fast);
}

.card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-2px);
}

/* 卡片入场动画：仅前 8 个错峰，避免大批量时长时间不可见 */
@keyframes card-in {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.card {
  animation: card-in 0.25s ease-out backwards;
}

.card.idx-0 { animation-delay: 0s; }
.card.idx-1 { animation-delay: 0.02s; }
.card.idx-2 { animation-delay: 0.04s; }
.card.idx-3 { animation-delay: 0.06s; }
.card.idx-4 { animation-delay: 0.08s; }
.card.idx-5 { animation-delay: 0.10s; }
.card.idx-6 { animation-delay: 0.12s; }
.card.idx-7 { animation-delay: 0.14s; }

/* 检测状态边框 */
.card.state--valid {
  border-color: var(--color-success);
}
.card.state--invalid,
.card.state--expired {
  border-color: var(--color-danger);
}
.card.state--invalid .card__title,
.card.state--expired .card__title {
  text-decoration: line-through;
  color: var(--color-text-secondary);
}

.card__head {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.card__cloud {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
  flex-shrink: 0;
  margin-top: 1px;
}

.card__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.4;
  color: var(--color-text);
  word-break: break-word;
  flex: 1;
  min-width: 0;
}

/* 状态圆点 */
.card__dot {
  flex-shrink: 0;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-top: 6px;
  background: #d1d5db;
  box-shadow: 0 0 0 2px var(--color-card);
  transition: background var(--transition-fast);
}

.card__dot.dot--idle {
  background: #d1d5db;
}

.card__dot.dot--checking {
  background: #9ca3af;
  animation: dot-pulse 1s ease-in-out infinite;
}

.card__dot.dot--valid {
  background: var(--color-success);
}

.card__dot.dot--invalid,
.card__dot.dot--expired {
  background: var(--color-danger);
}

.card__dot.dot--unsupported {
  background: #9ca3af;
}

.card__dot.dot--error {
  background: #f59e0b;
}

@keyframes dot-pulse {
  0%, 100% { opacity: 0.4; transform: scale(0.85); }
  50% { opacity: 1; transform: scale(1.1); }
}

.card__content {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-word;
}

/* 检测状态摘要 */
.card__check-summary {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  background: var(--color-bg);
}

.card__check-summary.summary--invalid,
.card__check-summary.summary--expired {
  color: var(--color-danger);
  background: rgba(239, 68, 68, 0.08);
}

.card__check-summary.summary--unsupported {
  color: var(--color-text-secondary);
  background: var(--color-bg);
}

.card__check-summary.summary--error {
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.08);
}

.card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.card__meta-item {
  display: inline-flex;
  align-items: center;
}

.card__cloud-label {
  font-weight: 600;
}

.card__pwd {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.card__pwd code {
  font-family: var(--font-mono);
  padding: 2px 8px;
  background: var(--color-primary-light);
  color: var(--color-primary-hover);
  border-radius: var(--radius-sm);
  font-size: 12px;
}

.card__actions {
  display: flex;
  gap: 8px;
  margin-top: auto;
  padding-top: 6px;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 7px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-card);
  color: var(--color-text);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.btn:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.btn--copy {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #fff;
}

.btn--copy:hover {
  background: var(--color-primary-hover);
  border-color: var(--color-primary-hover);
  color: #fff;
}

.btn--copy.is-copied {
  background: var(--color-success);
  border-color: var(--color-success);
  color: #fff;
}

.btn--open {
  background: var(--color-card);
  border-color: var(--color-primary);
  color: var(--color-primary);
  font-weight: 600;
}

.btn--open:hover {
  background: var(--color-primary);
  color: #fff;
}

@media (max-width: 768px) {
  .card {
    padding: 14px;
  }
  .card__title {
    font-size: 15px;
  }
}
</style>
