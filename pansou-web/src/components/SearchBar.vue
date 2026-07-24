<script setup>
import { ref, computed, watch } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { useHistory } from '@/composables/useHistory.js'
import { useSuggestions } from '@/composables/useSuggestions.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  loading: { type: Boolean, default: false }
})
const emit = defineEmits(['update:modelValue', 'search'])

const { history, hasHistory, removeHistory, clearHistory } = useHistory()
const { suggest } = useSuggestions()

const input = ref(props.modelValue)
const focused = ref(false)
const suggestions = ref([])
let suggestTimer = null

watch(
  () => props.modelValue,
  v => {
    if (v !== input.value) input.value = v
  }
)

// 输入变化时实时更新联想建议
watch(input, v => {
  clearTimeout(suggestTimer)
  const kw = (v || '').trim()
  if (!kw) {
    suggestions.value = []
    return
  }
  // 防抖，避免输入过快频繁计算
  suggestTimer = setTimeout(() => {
    suggestions.value = suggest(kw, 8)
  }, 120)
})

// 同时显示建议与历史
const showSuggestions = computed(
  () => focused.value && suggestions.value.length > 0
)
const showHistory = computed(
  () => focused.value && hasHistory.value && !showSuggestions.value
)

function onSearch() {
  const kw = input.value.trim()
  if (!kw || props.loading) return
  emit('update:modelValue', kw)
  emit('search', kw)
}

function pickSuggestion(kw) {
  input.value = kw
  focused.value = false
  suggestions.value = []
  emit('update:modelValue', kw)
  emit('search', kw)
}

function pickHistory(kw) {
  input.value = kw
  focused.value = false
  emit('update:modelValue', kw)
  emit('search', kw)
}

function onClearHistory(e) {
  e.stopPropagation()
  clearHistory()
}
</script>

<template>
  <div class="searchbar" @focusout="focused = false">
    <div class="searchbar__box">
      <el-icon class="searchbar__icon"><Search /></el-icon>
      <input
        v-model="input"
        type="text"
        class="searchbar__input"
        placeholder="输入关键词搜索网盘资源，回车开始"
        autocomplete="off"
        spellcheck="false"
        @focus="focused = true"
        @focusout="focused = false"
        @keyup.enter="onSearch"
      />
      <button
        class="searchbar__btn"
        :disabled="!input.trim() || loading"
        @click="onSearch"
      >
        <span v-if="loading" class="searchbar__spinner" />
        <span v-else>搜索</span>
      </button>
    </div>

    <!-- 输入建议（优先显示，输入有匹配时出现） -->
    <transition name="fade">
      <div v-if="showSuggestions" class="searchbar__dropdown searchbar__suggest">
        <div class="searchbar__dropdown-head">
          <span>相关建议</span>
        </div>
        <ul class="searchbar__list searchbar__list--col">
          <li v-for="kw in suggestions" :key="kw" class="searchbar__suggest-item">
            <button class="searchbar__suggest-btn" @mousedown.prevent="pickSuggestion(kw)">
              <el-icon class="searchbar__suggest-icon"><Search /></el-icon>
              <span>{{ kw }}</span>
            </button>
          </li>
        </ul>
      </div>
    </transition>

    <!-- 最近搜索（无建议时显示） -->
    <transition name="fade">
      <div v-if="showHistory" class="searchbar__dropdown">
        <div class="searchbar__dropdown-head">
          <span>最近搜索</span>
          <button class="searchbar__clear" @click="onClearHistory">清空</button>
        </div>
        <ul class="searchbar__list">
          <li v-for="kw in history" :key="kw" class="searchbar__item">
            <button class="searchbar__tag" @mousedown.prevent="pickHistory(kw)">
              {{ kw }}
            </button>
            <button
              class="searchbar__remove"
              @mousedown.prevent.stop="removeHistory(kw)"
              title="删除"
            >
              ×
            </button>
          </li>
        </ul>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.searchbar {
  position: relative;
  width: 100%;
}

.searchbar__box {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 8px 8px 18px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-md);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
}

.searchbar__box:focus-within {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 4px var(--color-primary-light);
}

.searchbar__icon {
  color: var(--color-text-secondary);
  font-size: 20px;
  flex-shrink: 0;
}

.searchbar__input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 16px;
  color: var(--color-text);
  padding: 8px 0;
}

.searchbar__input::placeholder {
  color: var(--color-text-secondary);
}

.searchbar__btn {
  flex-shrink: 0;
  padding: 10px 28px;
  border: none;
  border-radius: var(--radius-lg);
  background: var(--color-primary);
  color: #fff;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: background var(--transition-fast), transform var(--transition-fast);
}

.searchbar__btn:hover:not(:disabled) {
  background: var(--color-primary-hover);
  transform: scale(1.02);
}

.searchbar__btn:disabled {
  background: var(--color-text-secondary);
  cursor: not-allowed;
  opacity: 0.6;
}

.searchbar__spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.4);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.searchbar__dropdown {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  right: 0;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  padding: 12px 16px;
  z-index: 20;
}

.searchbar__dropdown-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  color: var(--color-text-secondary);
  margin-bottom: 10px;
}

.searchbar__clear {
  border: none;
  background: none;
  color: var(--color-text-secondary);
  cursor: pointer;
  font-size: 13px;
  padding: 2px 6px;
  border-radius: var(--radius-sm);
}

.searchbar__clear:hover {
  color: var(--color-danger);
  background: #fef2f2;
}

.searchbar__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.searchbar__list--col {
  flex-direction: column;
  gap: 4px;
}

.searchbar__item {
  display: inline-flex;
  align-items: center;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.searchbar__tag {
  border: none;
  background: none;
  padding: 5px 10px;
  font-size: 13px;
  color: var(--color-text);
  cursor: pointer;
}

.searchbar__tag:hover {
  color: var(--color-primary);
}

.searchbar__remove {
  border: none;
  background: none;
  padding: 0 8px;
  font-size: 16px;
  color: var(--color-text-secondary);
  cursor: pointer;
  line-height: 1;
}

.searchbar__remove:hover {
  color: var(--color-danger);
}

/* 输入建议样式 */
.searchbar__suggest-item {
  list-style: none;
}

.searchbar__suggest-btn {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: none;
  background: none;
  color: var(--color-text);
  font-size: 14px;
  text-align: left;
  cursor: pointer;
  border-radius: var(--radius-sm);
  transition: background var(--transition-fast);
}

.searchbar__suggest-btn:hover {
  background: var(--color-primary-light);
  color: var(--color-primary);
}

.searchbar__suggest-icon {
  font-size: 14px;
  color: var(--color-text-secondary);
  flex-shrink: 0;
}

.searchbar__suggest-btn:hover .searchbar__suggest-icon {
  color: var(--color-primary);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--transition-fast);
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

@media (max-width: 768px) {
  .searchbar__box {
    padding: 6px 6px 6px 14px;
  }
  .searchbar__btn {
    padding: 9px 18px;
    font-size: 14px;
  }
  .searchbar__input {
    font-size: 15px;
  }
}
</style>
