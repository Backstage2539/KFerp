<template>
  <section class="panel data-panel">
    <div class="panel-head">
      <h3>{{ title }}</h3>
      <slot name="actions" />
    </div>
    <div v-if="!rows.length" class="muted empty">{{ empty }}</div>
    <slot v-else />
  </section>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  rows: { type: Array, default: () => [] },
  empty: { type: String, default: '暂无数据' },
})
</script>

<style scoped>
.panel {
  background: #fff;
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 14px;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.panel-head h3 {
  margin: 0;
}

.muted {
  color: #64748b;
}

.empty {
  padding: 12px 0;
}
</style>
