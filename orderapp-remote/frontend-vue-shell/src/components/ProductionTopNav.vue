<template>
  <nav class="production-top-nav" aria-label="生产管理视图">
    <button
      v-for="item in productionTopNavItems"
      :key="item.key"
      type="button"
      :class="{ active: item.key === activeKey }"
      @click="openView(item.key)"
    >
      {{ item.label }}
    </button>
  </nav>
</template>

<script setup>
import { productionTopNavItems } from '../lib/production-workstation.js'

defineProps({
  activeKey: { type: String, default: '' },
})

function openView(key) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key } }))
}
</script>

<style scoped>
.production-top-nav {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  padding: 8px 0 12px;
  border-bottom: 1px solid #e7e3dd;
  margin-bottom: 14px;
}

.production-top-nav button {
  flex: 0 0 auto;
  min-height: 34px;
  border: 1px solid #d5d0c8;
  border-radius: 8px;
  background: #fff;
  color: #333;
  padding: 6px 10px;
  font: inherit;
  font-size: 13px;
  line-height: 1.25;
  cursor: pointer;
}

.production-top-nav button:hover {
  border-color: #9f978c;
}

.production-top-nav button.active {
  border-color: #202020;
  background: #202020;
  color: #fff;
}
</style>
