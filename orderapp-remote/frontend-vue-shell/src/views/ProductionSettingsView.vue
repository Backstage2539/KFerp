<template>
  <div class="production-settings-page">
    <header class="page-head">
      <div>
        <h2>生产配置</h2>
        <p>集中维护工艺路线、工序和工位/设备等制造主数据。</p>
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
        @click="activeTab = tab.key">
        {{ tab.label }}
      </button>
    </nav>

    <section class="tab-panel" role="tabpanel">
      <component :is="activeComponent" />
    </section>
  </div>
</template>

<script setup>
import { computed, markRaw, ref } from 'vue'
import ManufacturingOperationsView from './ManufacturingOperationsView.vue'
import ManufacturingWorkstationsView from './ManufacturingWorkstationsView.vue'
import ProcessTemplatesView from './ProcessTemplatesView.vue'

const tabs = [
  { key: 'routes', label: '工艺路线', component: markRaw(ProcessTemplatesView) },
  { key: 'operations', label: '工序', component: markRaw(ManufacturingOperationsView) },
  { key: 'workstations', label: '工位/设备', component: markRaw(ManufacturingWorkstationsView) },
]

const activeTab = ref(tabs[0].key)
const activeComponent = computed(() => tabs.find((tab) => tab.key === activeTab.value)?.component || tabs[0].component)
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
