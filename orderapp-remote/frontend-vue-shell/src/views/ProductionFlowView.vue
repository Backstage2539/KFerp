<template>
  <div class="production-flow-page">
    <header class="page-head">
      <div>
        <h2>生产流程</h2>
        <p>按生产计划、生产工单、工序执行、生产质检和生产验收推进制造流程。</p>
      </div>
    </header>

    <nav class="flow-tabs" role="tablist" aria-label="生产流程">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        role="tab"
        :aria-selected="activeTab === tab.key"
        :class="{ active: activeTab === tab.key }"
        @click="activeTab = tab.key">
        {{ tab.label }}
      </button>
    </nav>

    <section class="tab-panel" role="tabpanel">
      <KeepAlive>
        <component :is="activeComponent" embedded :view-params="props.viewParams" />
      </KeepAlive>
    </section>
  </div>
</template>

<script setup>
import { computed, markRaw, ref } from 'vue'
import JobCardsView from './JobCardsView.vue'
import ProducePlanView from './ProducePlanView.vue'
import ProductionAcceptanceView from './ProductionAcceptanceView.vue'
import QualityInspectionsView from './QualityInspectionsView.vue'
import WorkOrdersView from './WorkOrdersView.vue'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
})

const tabs = [
  { key: 'plan', label: '生产计划', component: markRaw(ProducePlanView) },
  { key: 'orders', label: '生产工单', component: markRaw(WorkOrdersView) },
  { key: 'cards', label: '工序卡', component: markRaw(JobCardsView) },
  { key: 'quality', label: '生产质检', component: markRaw(QualityInspectionsView) },
  { key: 'acceptance', label: '生产验收', component: markRaw(ProductionAcceptanceView) },
]

const activeTab = ref(tabs[0].key)
const activeComponent = computed(() => tabs.find((tab) => tab.key === activeTab.value)?.component || tabs[0].component)
</script>

<style scoped>
* { box-sizing: border-box; }
.production-flow-page { min-height: 100%; background: #f7f8fa; color: #171717; }
.page-head { padding: 18px 18px 12px; background: #fff; border-bottom: 1px solid #e6e8eb; }
h2 { margin: 0; font-size: 22px; }
p { margin: 5px 0 0; color: #666; font-size: 13px; }
.flow-tabs { display: flex; gap: 6px; overflow-x: auto; padding: 12px 18px 0; background: #fff; border-bottom: 1px solid #e6e8eb; }
.flow-tabs button { flex: 0 0 auto; min-height: 42px; border: 0; border-bottom: 3px solid transparent; background: transparent; padding: 8px 14px; color: #555; font-size: 14px; cursor: pointer; }
.flow-tabs button.active { border-bottom-color: #111827; color: #111827; font-weight: 700; }
.tab-panel { min-width: 0; }
@media (max-width: 760px) {
  .page-head { padding: 14px 12px 10px; }
  .flow-tabs { padding-left: 12px; padding-right: 12px; }
}
</style>
