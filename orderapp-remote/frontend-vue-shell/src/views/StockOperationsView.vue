<template>
  <div class="stock-operations-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>库存作业</h2>
          <p>入库、WIP 领退、成品转仓和盘点调整集中在这里处理。</p>
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
    </section>
    <component :is="activeComponent" embedded />
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import FinishedTransfersView from './FinishedTransfersView.vue'
import MaterialReceiptsView from './MaterialReceiptsView.vue'
import StockAdjustmentsView from './StockAdjustmentsView.vue'
import WipMaterialsView from './WipMaterialsView.vue'

const props = defineProps({
  embedded: { type: Boolean, default: false },
  initialTab: { type: String, default: 'receipts' },
})

const tabs = [
  { key: 'receipts', label: '原料入库', component: MaterialReceiptsView },
  { key: 'wip', label: 'WIP领退/转仓', component: WipMaterialsView },
  { key: 'finishedTransfers', label: '成品转仓', component: FinishedTransfersView },
  { key: 'adjustments', label: '库存调整', component: StockAdjustmentsView },
]

function normalizedTab(key) {
  return tabs.some((tab) => tab.key === key) ? key : 'receipts'
}

const activeTab = ref(normalizedTab(props.initialTab))
const activeComponent = computed(() => tabs.find((tab) => tab.key === activeTab.value)?.component || MaterialReceiptsView)

watch(() => props.initialTab, (key) => {
  activeTab.value = normalizedTab(key)
})
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
@media (max-width:900px){ .stock-operations-page{padding:12px;} .tabs{display:grid;grid-template-columns:1fr;} }
</style>
