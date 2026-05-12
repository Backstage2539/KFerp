<template>
  <div class="layout">
    <div v-if="isMobile && mobileOpen" class="overlay" @click="mobileOpen = false"></div>

    <aside class="sidebar" :class="sidebarClass">
      <div class="brand">ERP</div>
      <nav>
        <template v-for="g in availableMenuGroups" :key="g.id">
          <button
            class="section-toggle"
            :class="{ active: currentGroupId === g.id }"
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
        <div v-if="actorName" class="actor">{{ actorName }}</div>
        <button v-if="currentActor" class="logout" type="button" @click="logout">退出</button>
      </header>
      <div v-if="authLoading" class="status">加载中</div>
      <div v-else-if="authError" class="status">{{ authError }}</div>
      <div v-else-if="!isCurrentAllowed" class="status">无权访问</div>
      <component v-else :is="currentInternalView" class="internal-view" :title="title" :view-key="currentKey" :view-params="currentViewParams" />
    </main>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AllocationLogsView from './views/AllocationLogsView.vue'
import AuditView from './views/AuditView.vue'
import BomView from './views/BomView.vue'
import CompanyProfileView from './views/CompanyProfileView.vue'
import CompanyStaffView from './views/CompanyStaffView.vue'
import CostingView from './views/CostingView.vue'
import CostingSettingsView from './views/CostingSettingsView.vue'
import CustomerCapabilityTemplatesView from './views/CustomerCapabilityTemplatesView.vue'
import CustomersView from './views/CustomersView.vue'
import CustomerFulfillmentView from './views/CustomerFulfillmentView.vue'
import CustomerProcessingPortalView from './views/CustomerProcessingPortalView.vue'
import CustomerPortalSettingsView from './views/CustomerPortalSettingsView.vue'
import DeliveryNoteView from './views/DeliveryNoteView.vue'
import FinanceClosingView from './views/FinanceClosingView.vue'
import FinanceDashboardView from './views/FinanceDashboardView.vue'
import FinanceExpensesView from './views/FinanceExpensesView.vue'
import FinanceReportView from './views/FinanceReportView.vue'
import FinanceSettingsView from './views/FinanceSettingsView.vue'
import FinanceTaxLedgerView from './views/FinanceTaxLedgerView.vue'
import InventoryView from './views/InventoryView.vue'
import JobCardsView from './views/JobCardsView.vue'
import MachinesView from './views/MachinesView.vue'
import MallSettingsView from './views/MallSettingsView.vue'
import MaterialBatchesView from './views/MaterialBatchesView.vue'
import MaterialReceiptsView from './views/MaterialReceiptsView.vue'
import MaterialsView from './views/MaterialsView.vue'
import OrderEntryView from './views/OrderEntryView.vue'
import OrderInvoiceView from './views/OrderInvoiceView.vue'
import OrdersView from './views/OrdersView.vue'
import OperationManualView from './views/OperationManualView.vue'
import OutsourceSettingsView from './views/OutsourceSettingsView.vue'
import ProducePlanView from './views/ProducePlanView.vue'
import ProduceRunningView from './views/ProduceRunningView.vue'
import ProductionAcceptanceView from './views/ProductionAcceptanceView.vue'
import ProductionCostsView from './views/ProductionCostsView.vue'
import ProductionLogsView from './views/ProductionLogsView.vue'
import ProductSettingsView from './views/ProductSettingsView.vue'
import PurchaseView from './views/PurchaseView.vue'
import QualityInspectionsView from './views/QualityInspectionsView.vue'
import RequirementsView from './views/RequirementsView.vue'
import SalesOrderSettingsView from './views/SalesOrderSettingsView.vue'
import SalesOrderView from './views/SalesOrderView.vue'
import SenderSettingsView from './views/SenderSettingsView.vue'
import StockAdjustmentsView from './views/StockAdjustmentsView.vue'
import StockBatchesView from './views/StockBatchesView.vue'
import StockLedgerView from './views/StockLedgerView.vue'
import StockOperationsView from './views/StockOperationsView.vue'
import StockOutboundLogsView from './views/StockOutboundLogsView.vue'
import UserPermissionsView from './views/UserPermissionsView.vue'
import WipMaterialsView from './views/WipMaterialsView.vue'
import WarehouseInventoryView from './views/WarehouseInventoryView.vue'
import WorkOrdersView from './views/WorkOrdersView.vue'
import { clearStoredAuthToken, fetchCurrentActor, hasStoredAuthToken, logoutCurrentSession } from './api/auth.js'
import { appURL } from './api/client.js'
import { replaceHistoryURL, viewNavigationURL } from './lib/url-state.js'
import {
  defaultExpandedGroups,
  groupForView,
  menuGroups,
  menuMap,
  restoreExpandedGroups,
  toggleExpandedGroup,
} from './lib/menu-ia.js'
import { actorHasFullViewAccess, filterMenuGroups, isViewAllowed } from './lib/menu-permissions.js'

const collapsed = ref(false)
const requestedView = new URL(window.location.href).searchParams.get('view')
const requestedViewFromUrl = !!requestedView
const freshLogin = new URL(window.location.href).searchParams.get('fresh_login') === '1'
const currentKey = ref(requestedView && menuMap[requestedView] ? requestedView : 'order')
const currentViewParams = ref(readViewParams())
const isMobile = ref(false)
const mobileOpen = ref(false)
const expandedGroups = ref(defaultExpandedGroups(menuGroups, currentKey.value))
const menuStorageKey = 'kferp.menu.expandedGroups'
const authLoading = ref(true)
const authError = ref('')
const currentActor = ref(null)

const internalViews = {
  order: OrderEntryView,
  orders: OrdersView,
  orderSalesManual: OperationManualView,
  orderInvoice: OrderInvoiceView,
  salesOrder: SalesOrderView,
  deliveryNote: DeliveryNoteView,
  warehouseInventory: WarehouseInventoryView,
  stockOperations: StockOperationsView,
  purchase: PurchaseView,
  materials: MaterialsView,
  materialReceipts: MaterialReceiptsView,
  materialBatches: MaterialBatchesView,
  wipMaterials: WipMaterialsView,
  stockLedger: StockLedgerView,
  stockBatches: StockBatchesView,
  stockAdjustments: StockAdjustmentsView,
  stockOutboundLogs: StockOutboundLogsView,
  inventoryMaterialsManual: OperationManualView,
  bom: BomView,
  productSettings: ProductSettingsView,
  mallSettings: MallSettingsView,
  costing: CostingView,
  costingManual: OperationManualView,
  costingSettings: CostingSettingsView,
  financeDashboard: FinanceDashboardView,
  financeExpenses: FinanceExpensesView,
  financeClosing: FinanceClosingView,
  financeReport: FinanceReportView,
  financeTaxLedger: FinanceTaxLedgerView,
  financeSettings: FinanceSettingsView,
  financeManual: OperationManualView,
  producePlan: ProducePlanView,
  productionAcceptance: ProductionAcceptanceView,
  produceRunning: ProduceRunningView,
  produceLogs: ProductionLogsView,
  workOrders: WorkOrdersView,
  jobCards: JobCardsView,
  qualityInspections: QualityInspectionsView,
  productionCosts: ProductionCostsView,
  productionManual: OperationManualView,
  allocationLogs: AllocationLogsView,
  customers: CustomersView,
  products: ProductSettingsView,
  departments: CompanyStaffView,
  employees: CompanyStaffView,
  inventory: InventoryView,
  machines: MachinesView,
  companyProfile: CompanyProfileView,
  salesOrderSettings: SalesOrderSettingsView,
  senderSettings: SenderSettingsView,
  outsourceSettings: OutsourceSettingsView,
  customerCapabilityTemplates: CustomerCapabilityTemplatesView,
  customerPortalSettings: CustomerPortalSettingsView,
  customerPortalManual: OperationManualView,
  customerFulfillment: CustomerFulfillmentView,
  customerFulfillmentManual: OperationManualView,
  customerProcessingPortal: CustomerProcessingPortalView,
  settingsAuditManual: OperationManualView,
  audit: AuditView,
  userPermissions: UserPermissionsView,
  reqProduct: RequirementsView,
  reqDev: RequirementsView,
  reqUnit: RequirementsView,
  reqApi: RequirementsView,
  reqReview: RequirementsView,
  requirementsManual: OperationManualView,
}

function readViewParams() {
  const params = new URL(window.location.href).searchParams
  const out = {}
  for (const key of ['warehouse', 'item_type', 'batch', 'ship_ready']) {
    const value = params.get(key)
    if (value) out[key] = value
  }
  return out
}

function applyKeyToUrl(key, params = {}) {
  const url = new URL(window.location.href)
  replaceHistoryURL(viewNavigationURL(url, key, params))
}

function open(key, params = {}) {
  if (!menuMap[key]) return
  if (!isViewAllowed(key, allowedViewKeys.value)) return
  currentKey.value = key
  currentViewParams.value = { ...(params || {}) }
  ensureCurrentGroupOpen(key)
  applyKeyToUrl(key, currentViewParams.value)
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
  const group = groupForView(availableMenuGroups.value, key)
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
  if (key && menuMap[key] && isViewAllowed(key, allowedViewKeys.value)) {
    open(key, event?.detail?.params || {})
  }
}

function firstAllowedMenuKey() {
  const primary = availableMenuGroups.value[0]?.items?.[0]?.key
  if (primary) return primary
  if (isViewAllowed('customerProcessingPortal', allowedViewKeys.value)) return 'customerProcessingPortal'
  return ''
}

function clearFreshLoginFlag() {
  if (!freshLogin) return
  const url = new URL(window.location.href)
  url.searchParams.delete('fresh_login')
  replaceHistoryURL(url)
}

async function loadActor() {
  authLoading.value = true
  authError.value = ''
  if (!hasStoredAuthToken()) {
    redirectToLogin()
    return
  }
  try {
    currentActor.value = await fetchCurrentActor()
    expandedGroups.value = freshLogin
      ? defaultExpandedGroups(availableMenuGroups.value, currentKey.value)
      : restoreExpandedGroups(
        availableMenuGroups.value,
        readStoredExpandedGroups(),
        currentKey.value,
      )
    clearFreshLoginFlag()
    if (!requestedViewFromUrl && !isViewAllowed(currentKey.value, allowedViewKeys.value)) {
      const first = firstAllowedMenuKey()
      if (first) open(first)
    }
  } catch (err) {
    if (err.status === 401) {
      clearStoredAuthToken()
      redirectToLogin()
      return
    }
    authError.value = err.message || '权限加载失败'
  } finally {
    authLoading.value = false
  }
}

function redirectToLogin() {
  window.location.replace(appURL('/login'))
}

async function logout() {
  try {
    await logoutCurrentSession()
  } catch {
    // Local logout should still complete if the session is already invalid.
  }
  clearStoredAuthToken()
  redirectToLogin()
}

onMounted(async () => {
  handleResize()
  expandedGroups.value = restoreExpandedGroups(
    availableMenuGroups.value,
    readStoredExpandedGroups(),
    currentKey.value,
  )
  await loadActor()
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
const allowedViewKeys = computed(() => {
  if (!currentActor.value) return []
  if (actorHasFullViewAccess(currentActor.value)) return null
  return Array.isArray(currentActor.value.allowed_views) ? currentActor.value.allowed_views : []
})
const availableMenuGroups = computed(() => filterMenuGroups(menuGroups, allowedViewKeys.value))
const currentGroupId = computed(() => groupForView(availableMenuGroups.value, currentKey.value)?.id || '')
const toggleLabel = computed(() => {
  if (isMobile.value) return '弹出菜单'
  return collapsed.value ? '弹出菜单' : '收起菜单'
})
const title = computed(() => menuMap[currentKey.value]?.title || '')
const actorName = computed(() => currentActor.value?.name || '')
const isCurrentAllowed = computed(() => menuMap[currentKey.value] && isViewAllowed(currentKey.value, allowedViewKeys.value))
const currentInternalView = computed(() => internalViews[currentKey.value] || OrdersView)
</script>

<style scoped>
* { box-sizing: border-box; }
.layout { display: flex; min-height: 100vh; font-family: system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial; position: relative; }
.sidebar { width: 260px; border-right: 1px solid #eee; padding: 18px 14px; background: #fafafa; transition: width .2s ease, transform .2s ease, padding .2s ease; overflow: auto; }
.sidebar.collapsed { width: 0; border-right: 0; padding: 0; overflow: hidden; }
.sidebar.collapsed .brand,
.sidebar.collapsed nav,
.sidebar.collapsed .section-toggle,
.sidebar.collapsed .section-items,
.sidebar.collapsed .menu { display: none; }
.brand { font-size: 28px; line-height: 1.15; font-weight: 800; margin: 4px 0 22px; white-space: nowrap; letter-spacing: 0; }
.section-toggle {
  width: 100%;
  min-height: 48px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border: 1px solid #d8d8d8;
  background: #fff;
  border-radius: 8px;
  padding: 12px 10px;
  color: #666;
  cursor: pointer;
  margin-bottom: 10px;
}
.section-toggle:hover { border-color: #bdbdbd; }
.section-toggle.active { border-color: #111; box-shadow: 0 0 0 1px #111 inset; background: #fff; color: #111; }
.section-name { font-size: 16px; line-height: 1.25; font-weight: 800; }
.section-caret { width: 20px; text-align: center; font-size: 18px; line-height: 1; color: #666; }
.section-items { margin: -4px 0 8px; padding-left: 10px; }
.toggle { border: 1px solid #999; background: #fff; border-radius: 8px; padding: 6px 10px; cursor: pointer; }
.menu { width: 100%; min-height: 44px; text-align: left; border: 1px solid #ddd; background: #fff; border-radius: 8px; padding: 11px 12px; cursor: pointer; margin-bottom: 8px; font-size: 15px; line-height: 1.25; }
.menu.active { border-color: #111; background: #f5f5f5; color: #111; box-shadow: 0 0 0 1px #111 inset; }
.content { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.top { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-bottom: 1px solid #eee; }
.top.compact { gap: 0; }
.title { font-weight: 600; }
.actor { margin-left: auto; color: #666; font-size: 13px; }
.logout { margin-left: auto; border: 1px solid #d8d8d8; background: #fff; border-radius: 8px; padding: 6px 10px; cursor: pointer; color: #333; }
.actor + .logout { margin-left: 0; }
.logout:hover { border-color: #999; }
.status { padding: 28px; color: #666; }
.internal-view { min-height: calc(100vh - 56px); background: #fff; }
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,.25); z-index: 25; }

@media (max-width: 900px) {
  .sidebar.mobile {
    position: fixed;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 30;
    width: 280px;
    transform: translateX(-110%);
    border-right: 1px solid #eee;
    padding: 18px 14px;
  }
  .sidebar.mobile.open { transform: translateX(0); }
  .content { margin-left: 0 !important; }
  .top { padding: 10px; }
}
</style>
