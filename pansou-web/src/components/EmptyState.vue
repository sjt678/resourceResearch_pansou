<script setup>
defineProps({
  // 类型：initial（初始） | empty（无结果） | error（出错）
  type: { type: String, default: 'initial' },
  keyword: { type: String, default: '' }
})
const emit = defineEmits(['retry'])
</script>

<template>
  <div class="empty">
    <div class="empty__icon">
      <template v-if="type === 'error'">⚠️</template>
      <template v-else-if="type === 'empty'">🔍</template>
      <template v-else>📦</template>
    </div>

    <h3 v-if="type === 'initial'" class="empty__title">开始搜索你的网盘资源</h3>
    <h3 v-else-if="type === 'empty'" class="empty__title">
      没有找到「{{ keyword }}」相关资源
    </h3>
    <h3 v-else class="empty__title">搜索出错了</h3>

    <p v-if="type === 'initial'" class="empty__desc">
      输入电影、剧集、软件、学习资料等关键词，回车即可聚合搜索 100+ TG 频道与众多搜索源
    </p>
    <p v-else-if="type === 'empty'" class="empty__desc">
      换个关键词试试？比如更具体的作品名、英文名或年份
    </p>
    <p v-else class="empty__desc">
      后端服务可能暂时不可用，请稍后重试
    </p>

    <button v-if="type === 'error'" class="empty__retry" @click="emit('retry')">
      重新尝试
    </button>
  </div>
</template>

<style scoped>
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  text-align: center;
  color: var(--color-text-secondary);
}

.empty__icon {
  font-size: 56px;
  margin-bottom: 16px;
  opacity: 0.8;
}

.empty__title {
  margin: 0 0 10px;
  font-size: 20px;
  font-weight: 700;
  color: var(--color-text);
}

.empty__desc {
  margin: 0 0 20px;
  font-size: 14px;
  max-width: 420px;
  line-height: 1.7;
}

.empty__retry {
  padding: 10px 28px;
  border: none;
  border-radius: var(--radius-sm);
  background: var(--color-primary);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: background var(--transition-fast);
}

.empty__retry:hover {
  background: var(--color-primary-hover);
}
</style>
