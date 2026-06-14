<template>
  <nav class="production-top-nav" aria-label="生产管理视图">
    <button
      v-for="item in navItems"
      :key="item.key"
      type="button"
      :class="{ active: item.key === activeKey }"
      @click="openView(item.key)"
    >
      <span>{{ item.label }}</span>
      <strong v-if="item.badge" class="nav-badge">{{ item.badge }}</strong>
    </button>
  </nav>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { fetchProductionWorkstationOverview } from '../api/production.js'
import { navItemsWithProductionBadges, productionTopNavItems } from '../lib/production-workstation.js'

defineProps({
  activeKey: { type: String, default: '' },
})

const navBadges = ref({})
const navItems = computed(() => navItemsWithProductionBadges(productionTopNavItems, navBadges.value))

function openView(key) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key } }))
}

onMounted(async () => {
  try {
    const overview = await fetchProductionWorkstationOverview({ limit: 200 })
    navBadges.value = overview.nav_badges || {}
  } catch {
    navBadges.value = {}
  }
})
</script>

<style scoped>
.production-top-nav {
  position: sticky;
  top: 0;
  z-index: 18;
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  padding: 8px 0 12px;
  border-bottom: 1px solid #e7e3dd;
  margin-bottom: 14px;
  background: #f8f7f4;
}

.production-top-nav button {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
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

.nav-badge {
  border-radius: 999px;
  background: #f0ede8;
  color: #5d554b;
  padding: 2px 6px;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.2;
}

.production-top-nav button:hover {
  border-color: #9f978c;
}

.production-top-nav button.active {
  border-color: #202020;
  background: #202020;
  color: #fff;
}

.production-top-nav button.active .nav-badge {
  background: rgba(255, 255, 255, .16);
  color: #fff;
}
</style>
