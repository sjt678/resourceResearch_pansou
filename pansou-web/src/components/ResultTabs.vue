<script setup>
import { computed } from 'vue'
import { getCloudMeta, sortCloudTypes } from '@/utils/cloud.js'

const props = defineProps({
  // { quark: [...], baidu: [...], ... }
  mergedByType: { type: Object, default: () => ({}) },
  modelValue: { type: String, default: 'all' }
})
const emit = defineEmits(['update:modelValue'])

const tabs = computed(() => {
  const types = Object.keys(props.mergedByType || {}).filter(
    t => (props.mergedByType[t] || []).length > 0
  )
  const sorted = sortCloudTypes(types)
  const list = sorted.map(type => ({
    type,
    label: getCloudMeta(type).label,
    color: getCloudMeta(type).color,
    icon: getCloudMeta(type).icon,
    count: props.mergedByType[type].length
  }))
  const total = list.reduce((s, t) => s + t.count, 0)
  return [{ type: 'all', label: '全部', color: '#0ea5e9', icon: '全', count: total }, ...list]
})

function select(type) {
  emit('update:modelValue', type)
}
</script>

<template>
  <div class="tabs">
    <button
      v-for="tab in tabs"
      :key="tab.type"
      class="tabs__item"
      :class="{ 'is-active': modelValue === tab.type }"
      @click="select(tab.type)"
    >
      <span class="tabs__icon" :style="{ background: tab.color }">{{ tab.icon }}</span>
      <span class="tabs__label">{{ tab.label }}</span>
      <span class="tabs__count">{{ tab.count }}</span>
    </button>
  </div>
</template>

<style scoped>
.tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  padding: 6px 0;
}

.tabs__item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-card);
  cursor: pointer;
  font-size: 14px;
  color: var(--color-text-secondary);
  transition: all var(--transition-fast);
}

.tabs__item:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
  transform: translateY(-1px);
}

.tabs__item.is-active {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #fff;
  box-shadow: var(--shadow-md);
}

.tabs__item.is-active .tabs__count {
  background: rgba(255, 255, 255, 0.25);
  color: #fff;
}

.tabs__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 6px;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  flex-shrink: 0;
}

.tabs__label {
  font-weight: 500;
}

.tabs__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  background: var(--color-bg);
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

@media (max-width: 768px) {
  .tabs__item {
    padding: 6px 12px;
    font-size: 13px;
  }
}
</style>
