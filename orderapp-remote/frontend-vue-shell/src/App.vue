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
        <div v-if="showWorkspaceSwitcher" class="workspace-switcher" role="group" aria-label="工作台模式">
          <button
            type="button"
            :class="{ active: workspaceMode === FACTORY_WORKSPACE_MODE }"
            @click="setWorkspaceMode(FACTORY_WORKSPACE_MODE)">
            工厂总览
          </button>
          <button
            type="button"
            :class="{ active: workspaceMode === CUSTOMER_WORKSPACE_MODE }"
            @click="setWorkspaceMode(CUSTOMER_WORKSPACE_MODE)">
            客户账户
          </button>
        </div>
        <label v-if="showWorkspaceCustomerSelector" class="workspace-customer">
          <span>当前客户</span>
          <SearchableSelect
            v-model="workspaceCustomerId"
            :options="workspaceCustomerOptions"
            :option-label="customerOptionLabel"
            :option-meta="customerOptionMeta"
            :option-value="optionNumericValue"
            placeholder="选择客户"
            empty-text="没有匹配客户" />
        </label>
        <div v-if="actorName" class="actor">{{ actorName }}</div>
        <button v-if="currentActor" class="logout" type="button" @click="logout">退出</button>
      </header>
      <div v-if="activeNotification" class="global-notification" :class="notificationToneClass(activeNotification)">
        <button class="notification-main" type="button" @click="openNotification(activeNotification)">
          <strong>{{ activeNotification.title }}</strong>
          <span>{{ activeNotification.body || '点击查看订单' }}</span>
        </button>
        <button class="notification-close" type="button" aria-label="关闭通知" @click="dismissNotification(activeNotification)">x</button>
      </div>
      <div v-if="authLoading" class="status">加载中</div>
      <div v-else-if="authError" class="status">{{ authError }}</div>
      <div v-else-if="!isCurrentAllowed" class="status">无权访问</div>
      <component
        v-else
        :is="currentInternalView"
        class="internal-view"
        :title="title"
        :view-key="currentKey"
        :view-params="currentViewParams"
        :workspace-mode="workspaceMode"
        :customer-context-id="workspaceCustomerContextId"
        :customer-context-label="workspaceCustomerLabel"
        :customer-account-actor="isCustomerActor" />
    </main>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AllocationLogsView from './views/AllocationLogsView.vue'
import AuditView from './views/AuditView.vue'
import BomView from './views/BomView.vue'
import CompanyProfileView from './views/CompanyProfileView.vue'
import CompanyStaffView from './views/CompanyStaffView.vue'
import ContractsView from './views/ContractsView.vue'
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
import NotificationSettingsView from './views/NotificationSettingsView.vue'
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
import UISettingsView from './views/UISettingsView.vue'
import WipMaterialsView from './views/WipMaterialsView.vue'
import WarehouseInventoryView from './views/WarehouseInventoryView.vue'
import WorkOrdersView from './views/WorkOrdersView.vue'
import { clearStoredAuthToken, fetchCurrentActor, hasStoredAuthToken, logoutCurrentSession } from './api/auth.js'
import { apiGet, appURL } from './api/client.js'
import { fetchCustomerProcessingPortalOverview } from './api/customer-fulfillment.js'
import { fetchERPNotifications, markNotificationRead } from './api/message-center.js'
import { fetchUISettings } from './api/ui-settings.js'
import { replaceHistoryURL, viewNavigationURL } from './lib/url-state.js'
import { installTableAutoPagination } from './lib/table-auto-pagination.js'
import SearchableSelect from './components/SearchableSelect.vue'
import {
  defaultExpandedGroups,
  groupForView,
  menuGroups,
  menuMap,
  restoreExpandedGroups,
  toggleExpandedGroup,
} from './lib/menu-ia.js'
import { actorHasFullViewAccess, filterMenuGroups, isViewAllowed } from './lib/menu-permissions.js'
import {
  CUSTOMER_WORKSPACE_MODE,
  FACTORY_WORKSPACE_MODE,
  customerOptionLabel,
  customerOptionMeta,
  defaultWorkspaceEntryKey,
  isCustomerAccountActor,
  menuGroupsForWorkspaceMode,
  normalizeWorkspaceMode,
  workspaceViewParams,
} from './lib/workspace-mode.js'

const collapsed = ref(false)
const viewAliases = { userPermissions: 'employees' }
function normalizeViewKey(key) {
  return viewAliases[key] || key
}
const workspaceModeStorageKey = 'kferp.workspace.mode'
const workspaceCustomerStorageKey = 'kferp.workspace.customerId'
const requestedURLParams = new URL(window.location.href).searchParams
const requestedViewParam = requestedURLParams.get('view')
const requestedView = normalizeViewKey(requestedViewParam)
const requestedViewFromUrl = !!requestedViewParam
const freshLogin = requestedURLParams.get('fresh_login') === '1'
const workspaceMode = ref(normalizeWorkspaceMode(requestedURLParams.get('workspace') || readStorage(workspaceModeStorageKey)))
const workspaceCustomerId = ref(Number(requestedURLParams.get('customer_id') || readStorage(workspaceCustomerStorageKey) || 0))
const workspaceCustomerOptions = ref([])
const currentKey = ref(requestedView && menuMap[requestedView] ? requestedView : 'order')
const currentViewParams = ref(workspaceViewParams(readViewParams(), workspaceContext()))
const isMobile = ref(false)
const mobileOpen = ref(false)
const expandedGroups = ref(defaultExpandedGroups(menuGroupsForWorkspaceMode(menuGroups, workspaceMode.value), currentKey.value))
const menuStorageKey = 'kferp.menu.expandedGroups'
const authLoading = ref(true)
const authError = ref('')
const currentActor = ref(null)
const customerAccountContext = ref(null)
const uiSettings = ref({ hide_customer_account_fulfillment: true })
const notifications = ref([])
let notificationTimer = 0
let stopTableAutoPagination = null

const internalViews = {
  order: OrderEntryView,
  orders: OrdersView,
  contracts: ContractsView,
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
  greenBeanSalesManual: OperationManualView,
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
  notificationSettings: NotificationSettingsView,
  notificationManual: OperationManualView,
  customerCapabilityTemplates: CustomerCapabilityTemplatesView,
  customerPortalSettings: CustomerPortalSettingsView,
  customerPortalManual: OperationManualView,
  customerFulfillment: CustomerFulfillmentView,
  customerFulfillmentManual: OperationManualView,
  workspaceModeManual: OperationManualView,
  customerProcessingPortal: CustomerProcessingPortalView,
  uiSettings: UISettingsView,
  settingsAuditManual: OperationManualView,
  audit: AuditView,
  reqProduct: RequirementsView,
  reqDev: RequirementsView,
  reqUnit: RequirementsView,
  reqApi: RequirementsView,
  reqReview: RequirementsView,
  requirementsManual: OperationManualView,
}

const customerAccountActorMenuGroups = [
  {
    id: 'customerWorkbench',
    name: '工作台',
    items: [{ key: 'customerProcessingPortal', label: '工作台', title: '客户工作台' }],
  },
  {
    id: 'customerFinance',
    name: '费用相关',
    items: [
      { key: 'financeExpenses', label: '费用明细', title: '客户费用明细' },
      { key: 'financeReport', label: '经营报告', title: '客户经营报告' },
      { key: 'financeClosing', label: '结账相关', title: '客户结账相关' },
    ],
  },
]

function readViewParams() {
  const params = new URL(window.location.href).searchParams
  const out = {}
  for (const key of ['warehouse', 'item_type', 'batch', 'ship_ready', 'scope', 'highlight_order_id', 'customer_id']) {
    const value = params.get(key)
    if (value) out[key] = value
  }
  return out
}

function readStorage(key) {
  try {
    return window.localStorage.getItem(key) || ''
  } catch {
    return ''
  }
}

function writeStorage(key, value) {
  try {
    window.localStorage.setItem(key, String(value || ''))
  } catch {
    // Workspace preferences are a convenience; private mode should not block navigation.
  }
}

function workspaceContext() {
  return { mode: workspaceMode.value, customerID: workspaceCustomerId.value }
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function applyWorkspaceToUrl(url) {
  if (workspaceMode.value === CUSTOMER_WORKSPACE_MODE) {
    url.searchParams.set('workspace', CUSTOMER_WORKSPACE_MODE)
    if (workspaceCustomerId.value) {
      url.searchParams.set('customer_id', String(Number(workspaceCustomerId.value || 0)))
    } else {
      url.searchParams.delete('customer_id')
    }
    return url
  }
  url.searchParams.delete('workspace')
  url.searchParams.delete('customer_id')
  return url
}

function applyKeyToUrl(key, params = {}) {
  const url = new URL(window.location.href)
  replaceHistoryURL(applyWorkspaceToUrl(viewNavigationURL(url, key, workspaceViewParams(params, workspaceContext()))))
}

function open(key, params = {}) {
  if (!menuMap[key]) return
  if (!isViewAllowed(key, allowedViewKeys.value)) return
  currentKey.value = key
  currentViewParams.value = workspaceViewParams(params, workspaceContext())
  ensureCurrentGroupOpen(key)
  applyKeyToUrl(key, currentViewParams.value)
  if (isMobile.value) mobileOpen.value = false
}

function setWorkspaceMode(mode) {
  if (isCustomerActor.value) {
    workspaceMode.value = CUSTOMER_WORKSPACE_MODE
    open('customerProcessingPortal')
    return
  }
  const nextMode = normalizeWorkspaceMode(mode)
  if (workspaceMode.value !== nextMode) {
    workspaceMode.value = nextMode
    writeStorage(workspaceModeStorageKey, workspaceMode.value)
  }
  expandedGroups.value = defaultExpandedGroups(availableMenuGroups.value, currentKey.value)
  if (!groupForView(availableMenuGroups.value, currentKey.value)) {
    const first = firstAllowedMenuKey()
    if (first) open(first)
    return
  }
  currentViewParams.value = workspaceViewParams(currentViewParams.value, workspaceContext())
  applyKeyToUrl(currentKey.value, currentViewParams.value)
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

function handleWorkspaceCustomerChange(event) {
  const nextCustomerID = Number(event?.detail?.customerID || 0)
  if (nextCustomerID > 0 && workspaceMode.value === CUSTOMER_WORKSPACE_MODE) {
    workspaceCustomerId.value = nextCustomerID
  }
}

async function loadWorkspaceCustomers() {
  try {
    const data = await apiGet('/api/customer-fulfillment/customers?limit=200')
    workspaceCustomerOptions.value = data.customers || data.items || []
  } catch {
    try {
      const data = await apiGet('/api/customers?limit=200')
      workspaceCustomerOptions.value = data.customers || data.items || []
    } catch {
      workspaceCustomerOptions.value = []
    }
  }
}

async function loadCustomerAccountContext() {
  const data = await fetchCustomerProcessingPortalOverview()
  customerAccountContext.value = data || {}
  const customerID = Number(data?.customer_id || 0)
  if (customerID <= 0) return
  workspaceCustomerId.value = customerID
  if (workspaceCustomerOptions.value.some((item) => Number(item.id) === customerID)) return
  workspaceCustomerOptions.value = [
    ...workspaceCustomerOptions.value,
    {
      id: customerID,
      name: data?.customer_name || `客户 #${customerID}`,
      company_name: data?.customer_name || '',
    },
  ]
}

async function loadNotifications() {
  if (!currentActor.value || !isViewAllowed('orders', allowedViewKeys.value)) return
  try {
    const data = await fetchERPNotifications(5)
    notifications.value = data.notifications || []
  } catch {
    // Notification polling must not block the main ERP workspace.
  }
}

async function loadUISettings() {
  try {
    const data = await fetchUISettings()
    uiSettings.value = {
      hide_customer_account_fulfillment: data?.settings?.hide_customer_account_fulfillment !== false,
    }
  } catch {
    uiSettings.value = { hide_customer_account_fulfillment: true }
  }
}

function startNotificationPolling() {
  stopNotificationPolling()
  loadNotifications()
  notificationTimer = window.setInterval(loadNotifications, 15000)
}

function stopNotificationPolling() {
  if (!notificationTimer) return
  window.clearInterval(notificationTimer)
  notificationTimer = 0
}

async function dismissNotification(item) {
  notifications.value = notifications.value.filter((row) => Number(row.id) !== Number(item?.id))
  if (item?.id) {
    try {
      await markNotificationRead(item.id)
    } catch {
      // The next poll will reconcile read state.
    }
  }
}

async function openNotification(item) {
  const payload = item?.payload || {}
  const orderID = Number(payload.highlight_order_id || payload.order_id || item?.source_id || 0)
  await dismissNotification(item)
  open('orders', {
    scope: payload.orders_scope || 'fulfillment',
    highlight_order_id: orderID || undefined,
  })
}

function notificationToneClass(item) {
  return `tone-${item?.tone || 'info'}`
}

function firstAllowedMenuKey() {
  if (isCustomerActor.value) return 'customerProcessingPortal'
  const primary = defaultWorkspaceEntryKey(availableMenuGroups.value)
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
    await loadUISettings()
    if (isCustomerAccountActor(currentActor.value)) {
      workspaceMode.value = CUSTOMER_WORKSPACE_MODE
      await loadCustomerAccountContext()
      if (!['customerProcessingPortal', 'financeExpenses', 'financeClosing', 'financeReport'].includes(currentKey.value)) {
        currentKey.value = 'customerProcessingPortal'
        currentViewParams.value = {}
      } else {
        currentViewParams.value = workspaceViewParams(currentViewParams.value, workspaceContext())
      }
      applyKeyToUrl(currentKey.value, currentViewParams.value)
    }
    expandedGroups.value = freshLogin
      ? defaultExpandedGroups(availableMenuGroups.value, currentKey.value)
      : restoreExpandedGroups(
        availableMenuGroups.value,
        readStoredExpandedGroups(),
        currentKey.value,
      )
    clearFreshLoginFlag()
    if (requestedViewParam && requestedView !== requestedViewParam) {
      applyKeyToUrl(currentKey.value, currentViewParams.value)
    }
    if (!requestedViewFromUrl && !isViewAllowed(currentKey.value, allowedViewKeys.value)) {
      const first = firstAllowedMenuKey()
      if (first) open(first)
    }
    startNotificationPolling()
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
  stopNotificationPolling()
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
  stopTableAutoPagination = installTableAutoPagination(document.querySelector('.content') || document.body)
  expandedGroups.value = restoreExpandedGroups(
    availableMenuGroups.value,
    readStoredExpandedGroups(),
    currentKey.value,
  )
  await Promise.all([loadActor(), loadWorkspaceCustomers()])
  window.addEventListener('resize', handleResize)
  window.addEventListener('kferp:navigate-view', handleNavigateView)
  window.addEventListener('kferp:workspace-customer-change', handleWorkspaceCustomerChange)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('kferp:navigate-view', handleNavigateView)
  window.removeEventListener('kferp:workspace-customer-change', handleWorkspaceCustomerChange)
  if (stopTableAutoPagination) stopTableAutoPagination()
  stopNotificationPolling()
})

const sidebarClass = computed(() => ({
  collapsed: !isMobile.value && collapsed.value,
  mobile: isMobile.value,
  open: isMobile.value && mobileOpen.value,
}))

const showTitle = computed(() => !isMobile.value && !collapsed.value)
const allowedViewKeys = computed(() => {
  if (!currentActor.value) return []
  if (isCustomerAccountActor(currentActor.value)) return ['customerProcessingPortal', 'financeExpenses', 'financeClosing', 'financeReport']
  if (actorHasFullViewAccess(currentActor.value)) return null
  return Array.isArray(currentActor.value.allowed_views) ? currentActor.value.allowed_views : []
})
const isCustomerActor = computed(() => isCustomerAccountActor(currentActor.value))
const showWorkspaceSwitcher = computed(() => Boolean(currentActor.value) && !isCustomerActor.value)
const showWorkspaceCustomerSelector = computed(() => workspaceMode.value === CUSTOMER_WORKSPACE_MODE && !isCustomerActor.value)
const workspaceMenuGroups = computed(() => (isCustomerActor.value ? customerAccountActorMenuGroups : menuGroupsForWorkspaceMode(menuGroups, workspaceMode.value)))
const availableMenuGroups = computed(() => filterMenuGroups(workspaceMenuGroups.value, allowedViewKeys.value, {
  actor: currentActor.value,
  workspaceMode: workspaceMode.value,
  hideCustomerAccountFulfillment: uiSettings.value.hide_customer_account_fulfillment,
}))
const currentGroupId = computed(() => groupForView(availableMenuGroups.value, currentKey.value)?.id || '')
const toggleLabel = computed(() => {
  if (isMobile.value) return '弹出菜单'
  return collapsed.value ? '弹出菜单' : '收起菜单'
})
const title = computed(() => menuMap[currentKey.value]?.title || '')
const actorName = computed(() => currentActor.value?.name || '')
const isCurrentAllowed = computed(() => menuMap[currentKey.value] && isViewAllowed(currentKey.value, allowedViewKeys.value))
const currentInternalView = computed(() => internalViews[currentKey.value] || OrdersView)
const activeNotification = computed(() => notifications.value[0] || null)
const workspaceCustomerOption = computed(() => (
  workspaceCustomerOptions.value.find((item) => Number(item.id) === Number(workspaceCustomerId.value)) || null
))
const workspaceCustomerLabel = computed(() => {
  if (workspaceMode.value !== CUSTOMER_WORKSPACE_MODE) return ''
  if (isCustomerActor.value && customerAccountContext.value?.customer_name) {
    return customerAccountContext.value.customer_name
  }
  return workspaceCustomerOption.value ? customerOptionLabel(workspaceCustomerOption.value) : (workspaceCustomerId.value ? `客户 #${workspaceCustomerId.value}` : '')
})
const workspaceCustomerContextId = computed(() => (
  workspaceMode.value === CUSTOMER_WORKSPACE_MODE ? Number(customerAccountContext.value?.customer_id || workspaceCustomerId.value || 0) : 0
))

watch(workspaceCustomerId, (next) => {
  writeStorage(workspaceCustomerStorageKey, Number(next || 0))
  if (workspaceMode.value !== CUSTOMER_WORKSPACE_MODE) return
  currentViewParams.value = workspaceViewParams(currentViewParams.value, workspaceContext())
  applyKeyToUrl(currentKey.value, currentViewParams.value)
})
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
.top { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-bottom: 1px solid #eee; flex-wrap: wrap; }
.top.compact { gap: 8px; }
.title { font-weight: 600; }
.workspace-switcher {
  display: inline-flex;
  align-items: center;
  border: 1px solid #d8d8d8;
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}
.workspace-switcher button {
  min-height: 32px;
  border: 0;
  border-right: 1px solid #d8d8d8;
  background: #fff;
  padding: 6px 10px;
  font-size: 13px;
  line-height: 1.2;
  cursor: pointer;
  color: #333;
}
.workspace-switcher button:last-child { border-right: 0; }
.workspace-switcher button.active { background: #111; color: #fff; }
.workspace-customer {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: min(320px, 100%);
  color: #555;
  font-size: 13px;
}
.workspace-customer span { white-space: nowrap; }
.workspace-customer :deep(.searchable-select) { flex: 1; min-width: 180px; }
.workspace-customer :deep(.select-control input) { min-height: 32px; padding: 6px 8px; }
.workspace-customer :deep(.select-toggle) { min-height: 32px; }
.actor { margin-left: auto; color: #666; font-size: 13px; }
.logout { margin-left: auto; border: 1px solid #d8d8d8; background: #fff; border-radius: 8px; padding: 6px 10px; cursor: pointer; color: #333; }
.actor + .logout { margin-left: 0; }
.logout:hover { border-color: #999; }
.status { padding: 28px; color: #666; }
.internal-view { min-height: calc(100vh - 56px); background: #fff; }
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,.25); z-index: 25; }
.global-notification {
  display: flex;
  align-items: stretch;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid #d7eadf;
  background: #edf9f1;
  color: #11442b;
}
.global-notification.tone-warning { background: #fff7df; border-bottom-color: #ead99a; color: #684800; }
.global-notification.tone-danger { background: #fff0f0; border-bottom-color: #efb9b9; color: #8a1f1f; }
.global-notification.tone-info { background: #eef6ff; border-bottom-color: #cfe0f5; color: #143b68; }
.notification-main {
  flex: 1;
  min-height: 38px;
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
  padding: 0;
}
.notification-main strong { font-size: 14px; }
.notification-main span { font-size: 13px; }
.notification-close {
  width: 32px;
  border: 1px solid currentColor;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  cursor: pointer;
}

:global(.list-pagination-controls) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 12px;
  color: #333;
}
:global(.list-pagination-controls .pagination-summary) { font-size: 13px; color: #5f6368; }
:global(.list-pagination-controls .pagination-actions) { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
:global(.list-pagination-controls label) { display: inline-flex; align-items: center; gap: 6px; color: #666; font-size: 13px; }
:global(.list-pagination-controls input),
:global(.list-pagination-controls select) {
  height: 34px;
  border: 1px solid #cfc8bf;
  border-radius: 6px;
  padding: 6px 8px;
  font: inherit;
  background: #fff;
}
:global(.list-pagination-controls input) { width: 72px; }
:global(.list-pagination-controls button) {
  min-height: 34px;
  border-radius: 6px;
  border: 1px solid #999;
  padding: 6px 10px;
  font: inherit;
  background: #fff;
  color: #1f1f1f;
  cursor: pointer;
}
:global(.list-pagination-controls button:disabled) { cursor: not-allowed; opacity: .55; }

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
