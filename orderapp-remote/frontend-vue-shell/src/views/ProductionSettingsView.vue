<template>
  <div class="production-settings-page">
    <header class="page-head">
      <div>
        <h2>生产配置</h2>
        <p>集中维护工艺路线、工序、工位设备和生产 BOM 等制造主数据。</p>
      </div>
    </header>

    <nav class="settings-tabs" role="tablist" aria-label="生产配置">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        role="tab"
        :aria-selected="activeTab === tab.key"
        :class="{ active: activeTab === tab.key }"
        @click="selectTab(tab.key)">
        {{ tab.label }}
      </button>
    </nav>

    <section class="tab-panel" role="tabpanel">
      <BomView
        v-if="activeTab === 'bom'"
        :view-params="viewParams"
        :workspace-mode="workspaceMode"
        :customer-context-id="customerContextId"
        :customer-context-label="customerContextLabel" />
      <component v-else :is="activeComponent" />
    </section>
  </div>
</template>

<script setup>
import { computed, markRaw, ref, watch } from 'vue'
import BomView from './BomView.vue'
import ManufacturingOperationsView from './ManufacturingOperationsView.vue'
import ManufacturingWorkstationsView from './ManufacturingWorkstationsView.vue'
import ProcessTemplatesView from './ProcessTemplatesView.vue'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const tabs = [
  { key: 'routes', label: '工艺路线', component: markRaw(ProcessTemplatesView) },
  { key: 'operations', label: '工序', component: markRaw(ManufacturingOperationsView) },
  { key: 'workstations', label: '工位设备', component: markRaw(ManufacturingWorkstationsView) },
  { key: 'bom', label: '生产 BOM', component: markRaw(BomView) },
]

function normalizeTab(value) {
  const key = String(value || '').trim()
  return tabs.some((tab) => tab.key === key) ? key : tabs[0].key
}

const activeTab = ref(normalizeTab(props.viewParams?.tab))
const activeComponent = computed(() => tabs.find((tab) => tab.key === activeTab.value)?.component || tabs[0].component)

function selectTab(key) {
  const next = normalizeTab(key)
  if (next === activeTab.value) return
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'productionConfig',
      params: next === tabs[0].key ? {} : { tab: next },
    },
  }))
}

watch(() => props.viewParams?.tab, (value) => {
  activeTab.value = normalizeTab(value)
})
</script>

<style scoped>
* { box-sizing: border-box; }
.production-settings-page { min-height: 100%; background: #f7f8fa; color: #171717; }
.page-head { padding: 18px 18px 12px; background: #fff; border-bottom: 1px solid #e6e8eb; }
h2 { margin: 0; font-size: 22px; }
p { margin: 5px 0 0; color: #666; font-size: 13px; }
.settings-tabs { display: flex; gap: 6px; overflow-x: auto; padding: 12px 18px 0; background: #fff; border-bottom: 1px solid #e6e8eb; }
.settings-tabs button { flex: 0 0 auto; min-height: 42px; border: 0; border-bottom: 3px solid transparent; background: transparent; padding: 8px 14px; color: #555; font-size: 14px; cursor: pointer; }
.settings-tabs button.active { border-bottom-color: #111827; color: #111827; font-weight: 700; }
.tab-panel { min-width: 0; }
@media (max-width: 760px) {
  .page-head { padding: 14px 12px 10px; }
  .settings-tabs { padding-left: 12px; padding-right: 12px; }
}
</style>
