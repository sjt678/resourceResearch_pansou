<script setup>
// 骨架屏：搜索 loading 时展示
defineProps({
  count: { type: Number, default: 6 }
})
</script>

<template>
  <div class="skeleton-grid">
    <div v-for="n in count" :key="n" class="skeleton-card">
      <div class="skeleton-card__head">
        <div class="skeleton skeleton-card__icon" />
        <div class="skeleton skeleton-card__title" />
      </div>
      <div class="skeleton skeleton-card__line" />
      <div class="skeleton skeleton-card__line skeleton-card__line--short" />
      <div class="skeleton-card__meta">
        <div class="skeleton skeleton-card__tag" />
        <div class="skeleton skeleton-card__tag" />
      </div>
      <div class="skeleton-card__actions">
        <div class="skeleton skeleton-card__btn" />
        <div class="skeleton skeleton-card__btn" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.skeleton-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.skeleton-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 18px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}

.skeleton-card__head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.skeleton-card__icon {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  flex-shrink: 0;
}

.skeleton-card__title {
  height: 18px;
  width: 70%;
}

.skeleton-card__line {
  height: 12px;
  width: 100%;
}

.skeleton-card__line--short {
  width: 50%;
}

.skeleton-card__meta {
  display: flex;
  gap: 10px;
}

.skeleton-card__tag {
  height: 14px;
  width: 60px;
  border-radius: 7px;
}

.skeleton-card__actions {
  display: flex;
  gap: 8px;
  margin-top: 6px;
}

.skeleton-card__btn {
  height: 30px;
  width: 90px;
  border-radius: var(--radius-sm);
}

@media (max-width: 768px) {
  .skeleton-grid {
    grid-template-columns: 1fr;
  }
}
</style>
