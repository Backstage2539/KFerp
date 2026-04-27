<template>
  <div class="layout">
    <div v-if="isMobile && mobileOpen" class="overlay" @click="mobileOpen = false"></div>

    <aside class="sidebar" :class="sidebarClass">
      <div class="brand">ERP</div>
      <nav>
        <template v-for="g in menuGroups" :key="g.id">
          <button
            class="section-toggle"
            type="button"
            :aria-expanded="isGroupOpen(g.id)"
            @click="toggleGroup(g.id)">
            <span class="section-name">{{ g.name }}</span>
            <span class="section-caret">{{ isGroupOpen(g.id) ? 'v' : '>' }}</span>
          </button>
          <div v-show="isGroupOpen(g.id)" class="section-items">
            <button
              v-for="item in g.items"
              :key="item.key"
              class="menu"
              :class="{ active: currentKey === item.key }"
              @click="open(item.key)">
              {{ item.label }}
            </button>
          </div>
        </template>
      </nav>
    </aside>

    <main class="content">
      <header class="top" :class="{ compact: !showTitle }">
        <button class="toggle" @click="toggleMenu">{{ toggleLabel }}</button>
        <div v-if="showTitle" class="title">{{ title }}</div>
      </header>
      <component :is="currentInternalView" class="internal-view" :title="title" :view-key="currentKey" />
    </main>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AllocationLogsView from './views/AllocationLogsView.vue'
import AuditView from './views/AuditView.vue'
import BomView from './views/BomView.vue'
import CompanyStaffView from './views/CompanyStaffView.vue'
import CostingSettingsView from './views/CostingSettingsView.vue'
import CustomersView from './views/CustomersView.vue'
import InventoryView from './views/InventoryView.vue'
import JobCardsView from './views/JobCardsView.vue'
import MachinesView from './views/MachinesView.vue'
import MaterialBatchesView from './views/MaterialBatchesView.vue'
import MaterialReceiptsView from './views/MaterialReceiptsView.vue'
import MaterialsView from './views/MaterialsView.vue'
import OrderEntryView from './views/OrderEntryView.vue'
import OrdersView from './views/OrdersView.vue'
import OutsourceSettingsView from './views/OutsourceSettingsView.vue'
import ProducePlanView from './views/ProducePlanView.vue'
import ProduceRunningView from './views/ProduceRunningView.vue'
import ProductionCostsView from './views/ProductionCostsView.vue'
import ProductionLogsView from './views/ProductionLogsView.vue'
import ProductionManualView from './views/ProductionManualView.vue'
import ProductSettingsView from './views/ProductSettingsView.vue'
import ProductsView from './views/ProductsView.vue'
import RequirementsView from './views/RequirementsView.vue'
import SenderSettingsView from './views/SenderSettingsView.vue'
import StockAdjustmentsView from './views/StockAdjustmentsView.vue'
import StockBatchesView from './views/StockBatchesView.vue'
import StockLedgerView from './views/StockLedgerView.vue'
import StockOperationsView from './views/StockOperationsView.vue'
import WipMaterialsView from './views/WipMaterialsView.vue'
import WarehouseInventoryView from './views/WarehouseInventoryView.vue'
import WorkOrdersView from './views/WorkOrdersView.vue'
import {
  defaultExpandedGroups,
  groupForView,
  menuGroups,
  menuMap,
  restoreExpandedGroups,
  toggleExpandedGroup,
} from './lib/menu-ia.js'

const collapsed = ref(false)
const currentKey = ref('order')
const isMobile = ref(false)
const mobileOpen = ref(false)
const expandedGroups = ref(defaultExpandedGroups(menuGroups, currentKey.value))
const menuStorageKey = 'kferp.menu.expandedGroups'

const internalViews = {
  order: OrderEntryView,
  orders: OrdersView,
  warehouseInventory: WarehouseInventoryView,
  stockOperations: StockOperationsView,
  materials: MaterialsView,
  materialReceipts: MaterialReceiptsView,
  materialBatches: MaterialBatchesView,
  wipMaterials: WipMaterialsView,
  stockLedger: StockLedgerView,
  stockBatches: StockBatchesView,
  stockAdjustments: StockAdjustmentsView,
  bom: BomView,
  productSettings: ProductSettingsView,
  costing: ProductSettingsView,
  costingSettings: CostingSettingsView,
  producePlan: ProducePlanView,
  produceRunning: ProduceRunningView,
  produceLogs: ProductionLogsView,
  workOrders: WorkOrdersView,
  jobCards: JobCardsView,
  productionCosts: ProductionCostsView,
  productionManual: ProductionManualView,
  allocationLogs: AllocationLogsView,
  customers: CustomersView,
  products: ProductSettingsView,
  departments: CompanyStaffView,
  employees: CompanyStaffView,
  inventory: InventoryView,
  quotePrint: ProductsView,
  machines: MachinesView,
  senderSettings: SenderSettingsView,
  outsourceSettings: OutsourceSettingsView,
  audit: AuditView,
  reqProduct: RequirementsView,
  reqDev: RequirementsView,
  reqUnit: RequirementsView,
  reqApi: RequirementsView,
  reqReview: RequirementsView,
}

function applyKeyToUrl(key) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', key)
  window.history.replaceState({}, '', url.toString())
}

function open(key) {
  if (!menuMap[key]) return
  currentKey.value = key
  ensureCurrentGroupOpen(key)
  applyKeyToUrl(key)
  if (isMobile.value) mobileOpen.value = false
}

function persistExpandedGroups() {
  try {
    window.localStorage.setItem(menuStorageKey, JSON.stringify(expandedGroups.value))
  } catch {
    // localStorage may be unavailable in private or embedded contexts.
  }
}

function readStoredExpandedGroups() {
  try {
    return window.localStorage.getItem(menuStorageKey)
  } catch {
    return null
  }
}

function ensureCurrentGroupOpen(key) {
  const group = groupForView(menuGroups, key)
  if (!group || expandedGroups.value.includes(group.id)) return
  expandedGroups.value = [...expandedGroups.value, group.id]
  persistExpandedGroups()
}

function isGroupOpen(id) {
  return expandedGroups.value.includes(id)
}

function toggleGroup(id) {
  expandedGroups.value = toggleExpandedGroup(expandedGroups.value, id)
  persistExpandedGroups()
}

function handleResize() {
  isMobile.value = window.innerWidth <= 900
  if (!isMobile.value) {
    mobileOpen.value = false
  }
}

function toggleMenu() {
  if (isMobile.value) {
    mobileOpen.value = !mobileOpen.value
    return
  }
  collapsed.value = !collapsed.value
}

function handleNavigateView(event) {
  const key = event?.detail?.key
  if (key && menuMap[key]) {
    open(key)
  }
}

onMounted(() => {
  handleResize()
  const view = new URL(window.location.href).searchParams.get('view')
  if (view && menuMap[view]) {
    currentKey.value = view
  }
  expandedGroups.value = restoreExpandedGroups(
    menuGroups,
    readStoredExpandedGroups(),
    currentKey.value,
  )
  window.addEventListener('resize', handleResize)
  window.addEventListener('kferp:navigate-view', handleNavigateView)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('kferp:navigate-view', handleNavigateView)
})

const sidebarClass = computed(() => ({
  collapsed: !isMobile.value && collapsed.value,
  mobile: isMobile.value,
  open: isMobile.value && mobileOpen.value,
}))

const showTitle = computed(() => !isMobile.value && !collapsed.value)
const toggleLabel = computed(() => {
  if (isMobile.value) return '弹出菜单'
  return collapsed.value ? '弹出菜单' : '收起菜单'
})
const title = computed(() => menuMap[currentKey.value]?.title || '')
const currentInternalView = computed(() => internalViews[currentKey.value] || OrdersView)
</script>

<style scoped>
* { box-sizing: border-box; }
.layout { display: flex; min-height: 100vh; font-family: system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial; position: relative; }
.sidebar { width: 220px; border-right: 1px solid #eee; padding: 12px; background: #fafafa; transition: width .2s ease, transform .2s ease, padding .2s ease; overflow: auto; }
.sidebar.collapsed { width: 0; border-right: 0; padding: 0; overflow: hidden; }
.sidebar.collapsed .brand,
.sidebar.collapsed nav,
.sidebar.collapsed .section-toggle,
.sidebar.collapsed .section-items,
.sidebar.collapsed .menu { display: none; }
.brand { font-weight: 700; margin-bottom: 10px; white-space: nowrap; }
.section-toggle { width: 100%; display: flex; align-items: center; justify-content: space-between; gap: 8px; border: 0; background: transparent; padding: 10px 4px 6px; color: #666; cursor: pointer; }
.section-name { font-size: 12px; font-weight: 700; }
.section-caret { width: 16px; text-align: center; font-size: 12px; color: #777; }
.section-items { margin-bottom: 2px; }
.toggle { border: 1px solid #999; background: #fff; border-radius: 8px; padding: 6px 10px; cursor: pointer; }
.menu { width: 100%; text-align: left; border: 1px solid #ddd; background: #fff; border-radius: 8px; padding: 10px; cursor: pointer; margin-bottom: 8px; }
.menu.active { border-color: #111; background: #111; color: #fff; }
.content { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.top { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-bottom: 1px solid #eee; }
.top.compact { gap: 0; }
.title { font-weight: 600; }
.internal-view { min-height: calc(100vh - 56px); background: #fff; }
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,.25); z-index: 25; }

@media (max-width: 900px) {
  .sidebar.mobile {
    position: fixed;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 30;
    width: 220px;
    transform: translateX(-110%);
    border-right: 1px solid #eee;
    padding: 12px;
  }
  .sidebar.mobile.open { transform: translateX(0); }
  .content { margin-left: 0 !important; }
  .top { padding: 10px; }
}
</style>
