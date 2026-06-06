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

    <main ref="content" class="content" :style="notificationStackStyle">
      <header class="top" :class="{ compact: !showTitle }">
        <button class="toggle" @click="toggleMenu">{{ toggleLabel }}</button>
        <div v-if="showTitle" class="title">{{ title }}</div>
        <div v-if="showViewContextSelector" class="view-context-switcher workspace-switcher" role="group" aria-label="当前视图">
          <span class="view-context-caption">当前视图</span>
          <button
            type="button"
            :class="{ active: currentViewContext.type === FACTORY_VIEW_CONTEXT }"
            @click="setViewContextType(FACTORY_VIEW_CONTEXT)">
            工厂总览
          </button>
          <button
            type="button"
            :class="{ active: currentViewContext.type === CUSTOMER_VIEW_CONTEXT }"
            @click="setViewContextType(CUSTOMER_VIEW_CONTEXT)">
            客户
          </button>
          <button
            type="button"
            :class="{ active: currentViewContext.type === ORDER_VIEW_CONTEXT }"
            @click="setViewContextType(ORDER_VIEW_CONTEXT)">
            订单
          </button>
        </div>
        <label v-if="showWorkspaceCustomerSelector" class="workspace-customer">
          <span>客户</span>
          <SearchableSelect
            v-model="workspaceCustomerId"
            :options="workspaceCustomerOptions"
            :option-label="customerOptionLabel"
            :option-meta="customerOptionMeta"
            :option-value="optionNumericValue"
            placeholder="选择客户"
            empty-text="没有匹配客户" />
        </label>
        <label v-if="showWorkspaceOrderSelector" class="workspace-customer view-context-order">
          <span>订单</span>
          <SearchableSelect
            v-model="workspaceOrderId"
            :options="workspaceOrderOptions"
            :option-label="orderOptionLabel"
            :option-meta="orderOptionMeta"
            :option-value="optionNumericValue"
            placeholder="选择订单"
            empty-text="没有匹配订单" />
        </label>
        <div v-if="currentViewContextLabel" class="view-context-label">当前视图：{{ currentViewContextLabel }}</div>
        <div v-if="showViewContextSelector" class="view-context-presets" aria-label="保存视图">
          <select v-model.number="selectedViewContextPresetId" @change="applySelectedViewContextPreset">
            <option :value="0">常用视图</option>
            <option v-for="preset in viewContextPresets" :key="preset.id" :value="preset.id">{{ preset.name }}</option>
          </select>
          <button type="button" @click="saveCurrentViewContextPreset">保存当前视图</button>
          <button type="button" :disabled="!selectedViewContextPresetId" @click="disableSelectedViewContextPreset">停用视图</button>
          <button type="button" @click="resetViewContextToDefault">恢复默认视图</button>
        </div>
        <div v-if="actorName" class="actor">{{ actorName }}</div>
        <button v-if="currentActor" class="logout" type="button" @click="logout">退出</button>
      </header>
      <div
        v-if="allNotifications.length"
        ref="notificationStack"
        class="global-notification-stack"
        :class="{ layered: visibleNotifications.length > 1 }">
        <div class="notification-window-toolbar">
          <span class="notification-window-count">{{ notificationWindowSummary }}</span>
          <div class="notification-window-actions">
            <button
              class="notification-arrow"
              type="button"
              aria-label="上一条通知"
              :disabled="!canScrollNotificationUp"
              @click="scrollNotificationWindow(-1)">↑</button>
            <button
              class="notification-arrow"
              type="button"
              aria-label="下一条通知"
              :disabled="!canScrollNotificationDown"
              @click="scrollNotificationWindow(1)">↓</button>
            <button class="notification-clear" type="button" @click="clearAllNotifications">清空</button>
          </div>
        </div>
        <div class="notification-window-list">
          <div
            v-for="(item, idx) in visibleNotifications"
            :key="item.id || idx"
            class="global-notification"
            :class="notificationToneClass(item)"
            :style="{ '--stack-index': idx }">
            <button class="notification-main" type="button" @click="openNotification(item)">
              <strong>{{ item.title }}</strong>
              <span>{{ item.body || '点击查看订单' }}</span>
            </button>
            <button class="notification-close" type="button" aria-label="关闭通知" @click="dismissNotification(item)">x</button>
          </div>
        </div>
      </div>
      <div v-if="authLoading" class="status">加载中</div>
      <div v-else-if="authError" class="status">{{ authError }}</div>
      <div v-else-if="!isCurrentAllowed" class="status">无权访问</div>
      <ProductSettingsView
        v-else-if="isProductSettingsView"
        :key="currentViewIdentity"
        class="internal-view"
        :title="title"
        :view-key="currentKey"
        :section-mode="productSettingsSectionMode"
        :view-params="renderedViewParams"
        :workspace-mode="workspaceMode"
        :view-context="currentViewContext"
        :customer-context-id="workspaceCustomerContextId"
        :customer-context-label="workspaceCustomerLabel"
        :customer-account-actor="isCustomerActor" />
      <component
        v-else
        :key="currentViewIdentity"
        :is="resolveInternalView(currentKey)"
        class="internal-view"
        :title="title"
        :view-key="currentKey"
        :view-params="renderedViewParams"
        :workspace-mode="workspaceMode"
        :view-context="currentViewContext"
        :customer-context-id="workspaceCustomerContextId"
        :customer-context-label="workspaceCustomerLabel"
        :customer-account-actor="isCustomerActor" />
    </main>
  </div>
</template>

<script setup>
import { computed, markRaw, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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
import IndustryFieldTemplatesView from './views/IndustryFieldTemplatesView.vue'
import JobCardsView from './views/JobCardsView.vue'
import LogisticsSettingsView from './views/LogisticsSettingsView.vue'
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
import ProcessTemplatesView from './views/ProcessTemplatesView.vue'
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
import { apiGet, apiSend, appURL } from './api/client.js'
import { fetchCustomerProcessingPortalOverview } from './api/customer-fulfillment.js'
import { fetchERPNotifications, markNotificationRead } from './api/message-center.js'
import { fetchUISettings } from './api/ui-settings.js'
import {
  clampNotificationWindowStart,
  dedupeNotifications,
  filterDismissedNotifications,
  notificationBackendIDs,
  notificationWindow,
} from './lib/global-notifications.js'
import { relativeURLForHistory, replaceHistoryURL, viewNavigationURL } from './lib/url-state.js'
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
  WORKSPACE_CUSTOMERS_REFRESH_EVENT,
  customerOptionLabel,
  customerOptionMeta,
  defaultWorkspaceEntryKey,
  isCustomerAccountActor,
  menuGroupsForWorkspaceMode,
  normalizeWorkspaceMode,
} from './lib/workspace-mode.js'
import {
  CUSTOMER_VIEW_CONTEXT,
  EXTERNAL_CUSTOMER_VIEW_CONTEXT,
  FACTORY_VIEW_CONTEXT,
  ORDER_VIEW_CONTEXT,
  currentViewLabel,
  customerIDForViewContext,
  customerViewContextFromOption,
  externalCustomerViewContext,
  legacyWorkspaceModeForViewContext,
  menuGroupsForViewContext,
  normalizeViewContext,
  orderIDForViewContext,
  orderViewContextFromOption,
  viewContextFromURL,
  viewContextToURLParams,
  viewContextViewParams,
} from './lib/view-context.js'

const collapsed = ref(false)
const content = ref(null)
const notificationStack = ref(null)
const viewAliases = { userPermissions: 'employees' }
function normalizeViewKey(key) {
  return viewAliases[key] || key
}
const viewContextStorageKey = 'kferp.view-context'
const workspaceModeStorageKey = 'kferp.workspace.mode'
const workspaceCustomerStorageKey = 'kferp.workspace.customerId'
const workspaceOrderStorageKey = 'kferp.workspace.orderId'
const requestedURLParams = new URL(window.location.href).searchParams
const requestedViewParam = requestedURLParams.get('view')
const requestedView = normalizeViewKey(requestedViewParam)
const requestedViewFromUrl = !!requestedViewParam
const freshLogin = requestedURLParams.get('fresh_login') === '1'
const initialViewContext = initialViewContextFromRequest()
const currentViewContext = ref(initialViewContext)
const workspaceMode = ref(legacyWorkspaceModeForViewContext(initialViewContext))
const workspaceCustomerId = ref(customerIDForViewContext(initialViewContext) || Number(readStorage(workspaceCustomerStorageKey) || 0))
const workspaceOrderId = ref(orderIDForViewContext(initialViewContext) || Number(readStorage(workspaceOrderStorageKey) || 0))
const workspaceCustomerOptions = ref([])
const workspaceOrderOptions = ref([])
const viewContextPresets = ref([])
const selectedViewContextPresetId = ref(0)
const currentKey = ref(requestedView && menuMap[requestedView] ? requestedView : 'order')
const currentViewParams = ref(viewContextViewParams(readViewParams(), currentViewContext.value))
const transientReturnNavigation = ref(null)
const externalCustomerContextType = EXTERNAL_CUSTOMER_VIEW_CONTEXT // external_customer
const isMobile = ref(false)
const mobileOpen = ref(false)
let touchStartX = 0
let touchStartY = 0
const mobileSwipeMinDistance = 60
const mobileSwipeMaxVerticalDrift = 80
const expandedGroups = ref(defaultExpandedGroups(menuGroupsForViewContext(menuGroups, currentViewContext.value), currentKey.value))
const menuStorageKey = 'kferp.menu.expandedGroups'
const authLoading = ref(true)
const authError = ref('')
const currentActor = ref(null)
const customerAccountContext = ref(null)
const uiSettings = ref({ hide_customer_account_fulfillment: true })
const dismissedNotificationStorageKey = 'kferp.dismissed-notifications.v1'
const notificationFetchLimit = 100
const notificationWindowSize = 3
const notifications = ref([])
const localNotifications = ref([])
const dismissedNotificationIDs = ref(readDismissedNotificationIDs())
const notificationWindowStart = ref(0)
const notificationStackSpace = ref(0)
const workspaceCustomersRefreshEventName = WORKSPACE_CUSTOMERS_REFRESH_EVENT
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
  processTemplates: ProcessTemplatesView,
  industryFieldTemplates: IndustryFieldTemplatesView,
  productMaster: ProductSettingsView,
  customerProductAliases: ProductSettingsView,
  productCategoryManagement: ProductSettingsView,
  productPriceManagement: ProductSettingsView,
  productConfigTemplates: ProductSettingsView,
  pricingGradientTemplates: ProductSettingsView,
  productUnitTemplates: ProductSettingsView,
  productSettings: ProductSettingsView,
  products: ProductSettingsView,
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
  logisticsSettings: LogisticsSettingsView,
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

function resolveInternalView(key) {
  return markRaw(internalViews[key] || OrdersView)
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
  for (const key of ['warehouse', 'item_type', 'batch', 'ship_ready', 'scope', 'highlight_order_id', 'customer_id', 'order_id', 'order_no']) {
    const value = params.get(key)
    if (value) out[key] = value
  }
  return out
}

function readStoredViewContext() {
  try {
    const raw = window.localStorage.getItem(viewContextStorageKey)
    if (raw) return normalizeViewContext(JSON.parse(raw))
  } catch {
    // Ignore invalid or unavailable localStorage.
  }
  const legacyMode = normalizeWorkspaceMode(readStorage(workspaceModeStorageKey))
  const legacyCustomerID = Number(readStorage(workspaceCustomerStorageKey) || 0)
  if (legacyMode === CUSTOMER_WORKSPACE_MODE || legacyCustomerID > 0) {
    return normalizeViewContext({ type: CUSTOMER_VIEW_CONTEXT, customerID: legacyCustomerID })
  }
  return { type: FACTORY_VIEW_CONTEXT }
}

function initialViewContextFromRequest() {
  return normalizeViewContext(viewContextFromURL(window.location.href, readStoredViewContext()))
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

function dismissedNotificationIDList(value) {
  try {
    const raw = typeof value === 'string' ? JSON.parse(value || '[]') : value
    return Array.isArray(raw)
      ? raw.map((id) => Number(id || 0)).filter((id) => id > 0)
      : []
  } catch {
    return []
  }
}

function readDismissedNotificationIDs() {
  return dismissedNotificationIDList(readStorage(dismissedNotificationStorageKey))
}

function saveDismissedNotificationIDs(ids = []) {
  const unique = []
  const seen = new Set()
  for (const id of dismissedNotificationIDList(ids)) {
    if (seen.has(id)) continue
    seen.add(id)
    unique.push(id)
    if (unique.length >= 200) break
  }
  writeStorage(dismissedNotificationStorageKey, JSON.stringify(unique))
}

function rememberDismissedNotification(item) {
  const id = Number(item?.id || 0)
  if (id <= 0) return
  const next = [id, ...dismissedNotificationIDs.value.filter((existing) => Number(existing) !== id)]
  dismissedNotificationIDs.value = next.slice(0, 200)
  saveDismissedNotificationIDs(dismissedNotificationIDs.value)
}

function rememberDismissedNotificationIDs(ids = []) {
  const next = [
    ...notificationBackendIDs((ids || []).map((id) => ({ id }))),
    ...dismissedNotificationIDs.value,
  ]
  dismissedNotificationIDs.value = next.slice(0, 200)
  saveDismissedNotificationIDs(dismissedNotificationIDs.value)
}

function workspaceContext() {
  return { mode: workspaceMode.value, customerID: customerIDForViewContext(currentViewContext.value) || workspaceCustomerId.value }
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function applyViewContextToUrl(url) {
  for (const key of ['view_context', 'context', 'workspace', 'customer_id', 'customer_name', 'order_id', 'order_no']) {
    url.searchParams.delete(key)
  }
  const params = viewContextToURLParams(currentViewContext.value)
  for (const [key, value] of Object.entries(params)) {
    if (value) url.searchParams.set(key, value)
  }
  // Compatibility: old links using workspace=customer continue to round-trip.
  return url
}

function applyKeyToUrl(key, params = {}) {
  const url = new URL(window.location.href)
  replaceHistoryURL(applyViewContextToUrl(viewNavigationURL(url, key, viewContextViewParams(params, currentViewContext.value))))
}

function isProductSettingsKey(key) {
  return ['productMaster', 'customerProductAliases', 'productCategoryManagement', 'productPriceManagement', 'productConfigTemplates', 'pricingGradientTemplates', 'productUnitTemplates', 'productSettings', 'products'].includes(key)
}

function hardNavigateToView(key, params = {}) {
  const url = applyViewContextToUrl(viewNavigationURL(new URL(window.location.href), key, viewContextViewParams(params, currentViewContext.value)))
  window.location.assign(relativeURLForHistory(url))
}

function scrollCurrentViewToTop() {
  nextTick(() => {
    if (content.value && typeof content.value.scrollTo === 'function') {
      content.value.scrollTo({ top: 0, left: 0, behavior: 'auto' })
    } else if (content.value) {
      content.value.scrollTop = 0
      content.value.scrollLeft = 0
    }
    if (window.scrollY || window.scrollX) {
      window.scrollTo({ top: 0, left: 0, behavior: 'auto' })
    }
  })
}

function open(key, params = {}, options = {}) {
  if (!menuMap[key]) return
  if (!isViewAllowed(key, allowedViewKeys.value)) return
  transientReturnNavigation.value = options.returnNavigation
    ? { ...options.returnNavigation, targetKey: key }
    : null
  currentKey.value = key
  currentViewParams.value = viewContextViewParams(params, currentViewContext.value)
  ensureCurrentGroupOpen(key)
  applyKeyToUrl(key, currentViewParams.value)
  scrollCurrentViewToTop()
  if (isMobile.value) mobileOpen.value = false
}

function setCurrentViewContext(context, { reopen = true } = {}) {
  if (isCustomerActor.value) return
  const next = normalizeViewContext(context)
  currentViewContext.value = next
  workspaceMode.value = legacyWorkspaceModeForViewContext(next)
  const customerID = customerIDForViewContext(next)
  workspaceCustomerId.value = customerID
  workspaceOrderId.value = orderIDForViewContext(next)
  writeStorage(viewContextStorageKey, JSON.stringify(next))
  writeStorage(workspaceModeStorageKey, workspaceMode.value)
  if (customerID > 0) writeStorage(workspaceCustomerStorageKey, customerID)
  if (workspaceOrderId.value > 0) writeStorage(workspaceOrderStorageKey, workspaceOrderId.value)
  expandedGroups.value = defaultExpandedGroups(availableMenuGroups.value, currentKey.value)
  if (reopen && !groupForView(availableMenuGroups.value, currentKey.value)) {
    const first = firstAllowedMenuKey()
    if (first) open(first)
    return
  }
  currentViewParams.value = viewContextViewParams(currentViewParams.value, next)
  applyKeyToUrl(currentKey.value, currentViewParams.value)
  scrollCurrentViewToTop()
}

function setViewContextType(type) {
  if (type === FACTORY_VIEW_CONTEXT) {
    setCurrentViewContext({ type: FACTORY_VIEW_CONTEXT })
    return
  }
  if (type === ORDER_VIEW_CONTEXT) {
    const option = workspaceOrderOptions.value.find((item) => Number(item.id || item.order_id || 0) === Number(workspaceOrderId.value || 0))
    setCurrentViewContext(option ? orderViewContextFromOption(option) : { type: ORDER_VIEW_CONTEXT })
    return
  }
  const option = workspaceCustomerOptions.value.find((item) => Number(item.id || item.customer_id || 0) === Number(workspaceCustomerId.value || 0))
  setCurrentViewContext(option ? customerViewContextFromOption(option) : { type: CUSTOMER_VIEW_CONTEXT, customerID: workspaceCustomerId.value })
}

function setWorkspaceMode(mode) {
  if (isCustomerActor.value) {
    workspaceMode.value = CUSTOMER_WORKSPACE_MODE
    open('customerProcessingPortal')
    return
  }
  const nextMode = normalizeWorkspaceMode(mode)
  setViewContextType(nextMode === CUSTOMER_WORKSPACE_MODE ? CUSTOMER_VIEW_CONTEXT : FACTORY_VIEW_CONTEXT)
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
  syncNotificationStackSpace()
}

function syncNotificationStackSpace() {
  nextTick(() => {
    if (!isMobile.value || !visibleNotifications.value.length || !notificationStack.value) {
      notificationStackSpace.value = 0
      return
    }
    const bottom = Number(notificationStack.value.getBoundingClientRect().bottom || 0)
    notificationStackSpace.value = bottom > 0 ? Math.ceil(bottom + 10) : 0
  })
}

function handleTouchStart(event) {
  if (!isMobile.value || !event.touches?.length) return
  const touch = event.touches[0]
  touchStartX = Number(touch.clientX || 0)
  touchStartY = Number(touch.clientY || 0)
}

function handleTouchEnd(event) {
  if (!isMobile.value || !touchStartX || !event.changedTouches?.length) return
  const touch = event.changedTouches[0]
  const deltaX = Number(touch.clientX || 0) - touchStartX
  const deltaY = Math.abs(Number(touch.clientY || 0) - touchStartY)
  touchStartX = 0
  touchStartY = 0
  if (deltaY > mobileSwipeMaxVerticalDrift || Math.abs(deltaX) < mobileSwipeMinDistance) return
  if (deltaX > 0 && !mobileOpen.value) {
    mobileOpen.value = true
    return
  }
  if (deltaX < 0 && mobileOpen.value) {
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
    open(key, event?.detail?.params || {}, { returnNavigation: event?.detail?.returnNavigation || event?.detail?.return_navigation || null })
  }
}

function handleWorkspaceCustomerChange(event) {
  const nextCustomerID = Number(event?.detail?.customerID || 0)
  if (nextCustomerID > 0 && currentViewContext.value.type === CUSTOMER_VIEW_CONTEXT) {
    const option = workspaceCustomerOptions.value.find((item) => Number(item.id || item.customer_id || 0) === nextCustomerID)
    setCurrentViewContext(option ? customerViewContextFromOption(option) : { type: CUSTOMER_VIEW_CONTEXT, customerID: nextCustomerID })
  }
}

function handleWorkspaceCustomersRefresh() {
  loadWorkspaceCustomers()
}

async function loadWorkspaceCustomers() {
  try {
    const data = await apiGet('/api/view-context/options?type=customer&limit=200')
    workspaceCustomerOptions.value = (data.options || []).map(customerOptionFromViewContextOption)
  } catch {
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
}

async function loadWorkspaceOrders() {
  try {
    const data = await apiGet('/api/view-context/options?type=order&limit=80')
    workspaceOrderOptions.value = data.options || []
  } catch {
    workspaceOrderOptions.value = []
  }
}

async function loadViewContextPresets() {
  try {
    const data = await apiGet('/api/view-context/presets')
    viewContextPresets.value = data.presets || []
  } catch {
    viewContextPresets.value = []
  }
}

function presetPayloadForCurrentViewContext(name) {
  return {
    name,
    context_type: currentViewContext.value.type,
    context_json: contextJSONForPreset(currentViewContext.value),
    menu_keys_json: availableMenuGroups.value.flatMap((group) => (group.items || []).map((item) => item.key)),
    sort_order: viewContextPresets.value.length + 1,
  }
}

function contextJSONForPreset(context) {
  const ctx = normalizeViewContext(context)
  const out = { type: ctx.type }
  if (ctx.customerID) out.customer_id = ctx.customerID
  if (ctx.customerName) out.customer_name = ctx.customerName
  if (ctx.orderID) out.order_id = ctx.orderID
  if (ctx.orderNo) out.order_no = ctx.orderNo
  return out
}

async function saveCurrentViewContextPreset() {
  const defaultName = currentViewContextLabel.value || '当前视图'
  const name = window.prompt('保存当前视图', defaultName)
  if (!name || !name.trim()) return
  try {
    const data = await apiSend('/api/view-context/presets', {
      body: presetPayloadForCurrentViewContext(name.trim()),
    })
    await loadViewContextPresets()
    selectedViewContextPresetId.value = Number(data?.preset?.id || 0)
  } catch (err) {
    window.alert(err.message || '保存视图失败')
  }
}

function applySelectedViewContextPreset() {
  const preset = viewContextPresets.value.find((row) => Number(row.id || 0) === Number(selectedViewContextPresetId.value || 0))
  if (!preset) return
  setCurrentViewContext({
    ...(preset.context_json || {}),
    type: preset.context_type,
  })
}

async function disableSelectedViewContextPreset() {
  const id = Number(selectedViewContextPresetId.value || 0)
  if (!id) return
  try {
    await apiSend(`/api/view-context/presets/${id}/disable`)
    selectedViewContextPresetId.value = 0
    await loadViewContextPresets()
  } catch (err) {
    window.alert(err.message || '停用视图失败')
  }
}

function resetViewContextToDefault() {
  selectedViewContextPresetId.value = 0
  setCurrentViewContext({ type: FACTORY_VIEW_CONTEXT })
}

function customerOptionFromViewContextOption(option) {
  return {
    id: Number(option?.customer_id || option?.id || 0),
    name: option?.customer_name || option?.label || '',
    company_name: option?.company_name || '',
    contact: option?.contact || '',
    phone: option?.phone || '',
  }
}

function orderOptionLabel(option) {
  return option?.label || option?.order_no || `订单 #${option?.order_id || option?.id || ''}`
}

function orderOptionMeta(option) {
  const parts = []
  if (option?.customer_name) parts.push(option.customer_name)
  if (option?.order_date) parts.push(option.order_date)
  if (option?.status) parts.push(option.status)
  return parts.join(' / ')
}

async function loadCustomerAccountContext() {
  const data = await fetchCustomerProcessingPortalOverview()
  customerAccountContext.value = data || {}
  const customerID = Number(data?.customer_id || 0)
  if (customerID <= 0) return
  currentViewContext.value = externalCustomerViewContext(data)
  workspaceCustomerId.value = customerID
  workspaceMode.value = CUSTOMER_WORKSPACE_MODE
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
    const data = await fetchERPNotifications(notificationFetchLimit)
    notifications.value = dedupeNotifications(filterDismissedNotifications(data.notifications || [], dismissedNotificationIDs.value))
  } catch {
    // Notification polling must not block the main ERP workspace.
  }
}

async function clearAllNotifications() {
  const ids = notificationBackendIDs(allNotifications.value)
  rememberDismissedNotificationIDs(ids)
  localNotifications.value = []
  notifications.value = []
  notificationWindowStart.value = 0
  if (!ids.length) return
  try {
    await Promise.allSettled(ids.map((id) => markNotificationRead(id)))
  } catch {
    // Local dismissal already clears the stack; server read sync can retry on future item-level closes.
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
  if (item?.local_notice) {
    localNotifications.value = localNotifications.value.filter((row) => String(row.id) !== String(item?.id))
    return
  }
  rememberDismissedNotification(item)
  notifications.value = filterDismissedNotifications(notifications.value, dismissedNotificationIDs.value)
  if (item?.id) {
    try {
      await markNotificationRead(item.id)
    } catch {
      // Local dismissal already hides the notice; server read sync can recover on the next close/open cycle.
    }
  }
}

async function openNotification(item) {
  if (item?.local_notice) {
    await dismissNotification(item)
    return
  }
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

function scrollNotificationWindow(delta) {
  notificationWindowStart.value = clampNotificationWindowStart(
    notificationWindowStart.value + Number(delta || 0),
    allNotifications.value.length,
    notificationWindowSize,
  )
}

function handleLocalNotification(event) {
  const detail = event?.detail || {}
  const title = String(detail.title || detail.message || '操作提示').trim()
  const body = String(detail.body || detail.message || '').trim()
  const rawTone = detail.tone || detail.type || 'info'
  const tone = rawTone === 'error' ? 'danger' : (rawTone === 'success' ? 'info' : rawTone)
  const id = `local-${Date.now()}-${Math.random().toString(36).slice(2)}`
  localNotifications.value = [
    {
      id,
      local_notice: true,
      title,
      body,
      tone,
    },
    ...localNotifications.value,
  ].slice(0, 5)
  window.setTimeout(() => {
    localNotifications.value = localNotifications.value.filter((row) => row.id !== id)
  }, 4200)
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
      await loadCustomerAccountContext()
      if (!['customerProcessingPortal', 'financeExpenses', 'financeClosing', 'financeReport'].includes(currentKey.value)) {
        currentKey.value = 'customerProcessingPortal'
        currentViewParams.value = {}
      } else {
        currentViewParams.value = viewContextViewParams(currentViewParams.value, currentViewContext.value)
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
  await Promise.all([loadActor(), loadWorkspaceCustomers(), loadWorkspaceOrders(), loadViewContextPresets()])
  window.addEventListener('resize', handleResize)
  window.addEventListener('touchstart', handleTouchStart, { passive: true })
  window.addEventListener('touchend', handleTouchEnd, { passive: true })
  window.addEventListener('kferp:navigate-view', handleNavigateView)
  window.addEventListener('kferp:notify', handleLocalNotification)
  window.addEventListener('kferp:workspace-customer-change', handleWorkspaceCustomerChange)
  window.addEventListener(workspaceCustomersRefreshEventName, handleWorkspaceCustomersRefresh)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('touchstart', handleTouchStart)
  window.removeEventListener('touchend', handleTouchEnd)
  window.removeEventListener('kferp:navigate-view', handleNavigateView)
  window.removeEventListener('kferp:notify', handleLocalNotification)
  window.removeEventListener('kferp:workspace-customer-change', handleWorkspaceCustomerChange)
  window.removeEventListener(workspaceCustomersRefreshEventName, handleWorkspaceCustomersRefresh)
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
const showViewContextSelector = computed(() => Boolean(currentActor.value) && !isCustomerActor.value)
const showWorkspaceCustomerSelector = computed(() => currentViewContext.value.type === CUSTOMER_VIEW_CONTEXT && !isCustomerActor.value)
const showWorkspaceOrderSelector = computed(() => currentViewContext.value.type === ORDER_VIEW_CONTEXT && !isCustomerActor.value)
const workspaceMenuGroups = computed(() => (isCustomerActor.value ? customerAccountActorMenuGroups : menuGroupsForViewContext(menuGroups, currentViewContext.value)))
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
const isProductSettingsView = computed(() => isProductSettingsKey(currentKey.value))
const productSettingsSectionMode = computed(() => {
  if (currentKey.value === 'customerProductAliases') return 'aliases'
  if (currentKey.value === 'productCategoryManagement') return 'productCategoryManagement'
  if (currentKey.value === 'productPriceManagement') return 'productPriceManagement'
  if (currentKey.value === 'productConfigTemplates') return 'templates'
  if (currentKey.value === 'pricingGradientTemplates') return 'pricingGradientTemplates'
  if (currentKey.value === 'productUnitTemplates') return 'productUnitTemplates'
  return 'master'
})
const currentViewIdentity = computed(() => `${currentKey.value}:${currentViewContext.value.type}:${workspaceCustomerContextId.value || 0}:${orderIDForViewContext(currentViewContext.value) || 0}`)
const renderedViewParams = computed(() => {
  const params = { ...(currentViewParams.value || {}) }
  if (transientReturnNavigation.value && transientReturnNavigation.value.targetKey === currentKey.value) {
    params.return_navigation = transientReturnNavigation.value
  }
  return params
})
const allNotifications = computed(() => dedupeNotifications([...localNotifications.value, ...filterDismissedNotifications(notifications.value, dismissedNotificationIDs.value)]))
const visibleNotifications = computed(() => notificationWindow(allNotifications.value, notificationWindowStart.value, notificationWindowSize))
const canScrollNotificationUp = computed(() => notificationWindowStart.value > 0)
const canScrollNotificationDown = computed(() => notificationWindowStart.value + notificationWindowSize < allNotifications.value.length)
const notificationWindowSummary = computed(() => {
  const total = allNotifications.value.length
  if (!total) return '0 / 0'
  const start = clampNotificationWindowStart(notificationWindowStart.value, total, notificationWindowSize)
  const end = Math.min(total, start + notificationWindowSize)
  return `${start + 1}-${end} / ${total}`
})
const notificationStackStyle = computed(() => ({
  '--kferp-notice-stack-space': isMobile.value && visibleNotifications.value.length
    ? `${notificationStackSpace.value}px`
    : '0px',
}))
const workspaceCustomerOption = computed(() => (
  workspaceCustomerOptions.value.find((item) => Number(item.id) === Number(workspaceCustomerId.value)) || null
))
const workspaceCustomerLabel = computed(() => {
  if (legacyWorkspaceModeForViewContext(currentViewContext.value) !== CUSTOMER_WORKSPACE_MODE) return ''
  if (isCustomerActor.value && customerAccountContext.value?.customer_name) {
    return customerAccountContext.value.customer_name
  }
  if (currentViewContext.value.customerName) return currentViewContext.value.customerName
  return workspaceCustomerOption.value ? customerOptionLabel(workspaceCustomerOption.value) : (workspaceCustomerId.value ? `客户 #${workspaceCustomerId.value}` : '')
})
const workspaceCustomerContextId = computed(() => (
  legacyWorkspaceModeForViewContext(currentViewContext.value) === CUSTOMER_WORKSPACE_MODE
    ? Number(customerAccountContext.value?.customer_id || customerIDForViewContext(currentViewContext.value) || workspaceCustomerId.value || 0)
    : 0
))
const currentViewContextLabel = computed(() => currentViewLabel(currentViewContext.value))

watch(workspaceCustomerId, (next) => {
  writeStorage(workspaceCustomerStorageKey, Number(next || 0))
  if (currentViewContext.value.type !== CUSTOMER_VIEW_CONTEXT) return
  const option = workspaceCustomerOptions.value.find((item) => Number(item.id || 0) === Number(next || 0))
  currentViewContext.value = option ? customerViewContextFromOption(option) : normalizeViewContext({ type: CUSTOMER_VIEW_CONTEXT, customerID: next })
  writeStorage(viewContextStorageKey, JSON.stringify(currentViewContext.value))
  currentViewParams.value = viewContextViewParams(currentViewParams.value, currentViewContext.value)
  applyKeyToUrl(currentKey.value, currentViewParams.value)
})

watch(workspaceOrderId, (next) => {
  writeStorage(workspaceOrderStorageKey, Number(next || 0))
  if (currentViewContext.value.type !== ORDER_VIEW_CONTEXT) return
  const option = workspaceOrderOptions.value.find((item) => Number(item.id || item.order_id || 0) === Number(next || 0))
  currentViewContext.value = option ? orderViewContextFromOption(option) : normalizeViewContext({ type: ORDER_VIEW_CONTEXT, orderID: next })
  workspaceMode.value = legacyWorkspaceModeForViewContext(currentViewContext.value)
  workspaceCustomerId.value = customerIDForViewContext(currentViewContext.value)
  writeStorage(viewContextStorageKey, JSON.stringify(currentViewContext.value))
  currentViewParams.value = viewContextViewParams(currentViewParams.value, currentViewContext.value)
  applyKeyToUrl(currentKey.value, currentViewParams.value)
})

watch(allNotifications, (rows) => {
  notificationWindowStart.value = clampNotificationWindowStart(notificationWindowStart.value, rows.length, notificationWindowSize)
}, { flush: 'post' })

watch([visibleNotifications, isMobile], syncNotificationStackSpace, { flush: 'post' })
</script>

<style scoped>
* { box-sizing: border-box; }
.layout { display: flex; height: 100vh; overflow: hidden; font-family: system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial; position: relative; }
.sidebar { width: 260px; flex: 0 0 260px; height: 100vh; border-right: 1px solid #eee; padding: 18px 14px; background: #fafafa; transition: width .2s ease, flex-basis .2s ease, transform .2s ease, padding .2s ease; overflow-y: auto; overflow-x: hidden; overscroll-behavior: contain; -webkit-overflow-scrolling: touch; }
.sidebar.collapsed { width: 0; flex-basis: 0; border-right: 0; padding: 0; overflow: hidden; }
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
.content { flex: 1; display: flex; flex-direction: column; min-width: 0; height: 100vh; overflow-y: auto; overflow-x: hidden; overscroll-behavior: contain; -webkit-overflow-scrolling: touch; }
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
.view-context-caption {
  padding: 0 9px;
  color: #555;
  font-size: 13px;
  line-height: 32px;
  border-right: 1px solid #d8d8d8;
  white-space: nowrap;
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
.view-context-label {
  max-width: min(420px, 100%);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 6px 10px;
  color: #444;
  background: #f9f9f9;
  font-size: 13px;
  line-height: 1.35;
}
.view-context-presets {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.view-context-presets select,
.view-context-presets button {
  min-height: 32px;
  border: 1px solid #d8d8d8;
  border-radius: 8px;
  background: #fff;
  padding: 6px 9px;
  color: #333;
  font-size: 13px;
  line-height: 1.2;
}
.view-context-presets button:disabled { color: #aaa; cursor: not-allowed; }
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
.workspace-customer :deep(.select-control input) { min-height: 32px; padding: 6px 70px 6px 8px; }
.workspace-customer :deep(.select-toggle) { min-height: 32px; }
.actor { margin-left: auto; color: #666; font-size: 13px; }
.logout { margin-left: auto; border: 1px solid #d8d8d8; background: #fff; border-radius: 8px; padding: 6px 10px; cursor: pointer; color: #333; }
.actor + .logout { margin-left: 0; }
.logout:hover { border-color: #999; }
.status { padding: 28px; color: #666; }
.internal-view { min-height: calc(100vh - 56px); background: #fff; }
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,.25); z-index: 25; }
.global-notification-stack {
  display: grid;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid #eef0f5;
  background: #fff;
}
.notification-window-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.notification-window-count {
  color: #5f6368;
  font-size: 12px;
}
.notification-window-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.notification-window-actions button {
  min-height: 28px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  background: #fff;
  color: #1f2937;
  cursor: pointer;
}
.notification-window-actions button:disabled {
  opacity: .45;
  cursor: not-allowed;
}
.notification-arrow {
  width: 30px;
  padding: 0;
}
.notification-clear {
  padding: 3px 9px;
}
.notification-window-list {
  display: grid;
  gap: 8px;
}
.global-notification {
  display: flex;
  align-items: stretch;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid #d7eadf;
  border-radius: 8px;
  background: #edf9f1;
  color: #11442b;
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.08);
}
.global-notification.tone-warning { background: #fff7df; border-color: #ead99a; color: #684800; }
.global-notification.tone-danger { background: #fff0f0; border-color: #efb9b9; color: #8a1f1f; }
.global-notification.tone-info { background: #eef6ff; border-color: #cfe0f5; color: #143b68; }
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
  .global-notification-stack {
    position: relative;
    z-index: 65;
    gap: 6px;
    padding: 8px max(12px, env(safe-area-inset-right)) 10px max(12px, env(safe-area-inset-left));
    border-bottom: 0;
    background: transparent;
  }
  .notification-window-toolbar {
    padding: 0 2px;
  }
  .notification-window-actions button {
    min-height: 32px;
  }
  .notification-window-list {
    gap: 0;
  }
  .global-notification-stack .global-notification {
    min-height: 64px;
    border-radius: 14px;
    box-shadow: 0 14px 32px rgba(15, 23, 42, 0.14);
    transform: translateY(calc(var(--stack-index) * 0px)) scale(calc(1 - var(--stack-index) * .025));
    transform-origin: top center;
  }
  .global-notification-stack .global-notification + .global-notification {
    margin-top: -10px;
  }
  .global-notification-stack.layered .global-notification:not(:first-child) {
    opacity: .92;
  }
  .notification-main {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
    justify-content: center;
  }
  .notification-main span {
    line-height: 1.35;
  }
}
</style>
