<template>
  <div class="stock-operations-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>库存作业</h2>
          <p>库存移动统一使用库存单据；实物盘点和成本修正独立处理。</p>
        </div>
      </div>
      <div class="tabs" role="tablist" aria-label="库存作业">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          role="tab"
          class="tab"
          :class="{ active: activeTab === tab.key }"
          :aria-selected="activeTab === tab.key"
          @click="activeTab = tab.key">
          {{ tab.label }}
        </button>
      </div>
      <div v-if="contextBadges.length" class="context-badges" aria-label="生产上下文">
        <span v-for="badge in contextBadges" :key="badge">{{ badge }}</span>
      </div>
    </section>
    <component :is="activeComponent" embedded :view-params="props.viewParams" />
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import StockAdjustmentsView from './StockAdjustmentsView.vue'
import StockEntriesView from './StockEntriesView.vue'

const props = defineProps({
  embedded: { type: Boolean, default: false },
  initialTab: { type: String, default: 'stockEntries' },
  viewParams: { type: Object, default: () => ({}) },
})

const tabs = [
  { key: 'stockEntries', label: '库存单据', component: StockEntriesView },
  { key: 'adjustments', label: '盘点调整', component: StockAdjustmentsView },
]

function normalizedTab(key) {
  if (['receipts', 'wip', 'finishedTransfers'].includes(key)) return 'stockEntries'
  return tabs.some((tab) => tab.key === key) ? key : 'stockEntries'
}

const initialActiveTab = computed(() => normalizedTab(props.viewParams?.tab || props.initialTab))
const activeTab = ref(initialActiveTab.value)
const activeComponent = computed(() => tabs.find((tab) => tab.key === activeTab.value)?.component || StockEntriesView)
const contextBadges = computed(() => {
  const params = props.viewParams || {}
  return [
    params.work_order_id ? `工单 #${params.work_order_id}` : '',
    params.job_card_id ? `工序卡 #${params.job_card_id}` : '',
    params.running_item_id ? `生产中 #${params.running_item_id}` : '',
    params.material_id ? `物料 #${params.material_id}` : '',
    params.shortage_g ? `缺口 ${params.shortage_g}g` : '',
  ].filter(Boolean)
})

watch(() => props.initialTab, (key) => {
  activeTab.value = normalizedTab(props.viewParams?.tab || key)
})

watch(() => props.viewParams, () => {
  activeTab.value = initialActiveTab.value
}, { deep: true })
</script>

<style scoped>
.stock-operations-page { padding:16px; display:grid; gap:16px; }
.stock-operations-page.embedded { padding:0; }
.panel { border:1px solid #e5e7eb; border-radius:8px; background:#fff; padding:12px; }
.panel-head { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; margin-bottom:12px; }
.panel-head h2 { margin:0 0 4px; font-size:18px; }
.panel-head p { margin:0; color:#6b7280; font-size:13px; }
.tabs { display:flex; flex-wrap:wrap; gap:8px; }
.tab { font:inherit; min-height:36px; border:1px solid #d1d5db; background:#fff; border-radius:6px; padding:8px 12px; cursor:pointer; }
.tab.active { border-color:#111; background:#111; color:#fff; }
.context-badges { display:flex; flex-wrap:wrap; gap:6px; margin-top:10px; }
.context-badges span { border:1px solid #bfdbfe; background:#eff6ff; color:#1d4ed8; border-radius:999px; padding:3px 8px; font-size:12px; }
@media (max-width:900px){ .stock-operations-page{padding:12px;} .tabs{display:grid;grid-template-columns:1fr;} }
</style>
