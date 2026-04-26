<template>
  <div class="layout">
    <div v-if="isMobile && mobileOpen" class="overlay" @click="mobileOpen = false"></div>

    <aside class="sidebar" :class="sidebarClass">
      <div class="brand">ERP</div>
      <nav>
        <template v-for="g in menuGroups" :key="g.name">
          <div class="section">{{ g.name }}</div>
          <button
            v-for="item in g.items"
            :key="item.key"
            class="menu"
            :class="{ active: currentKey === item.key }"
            @click="open(item.key)">
            {{ item.label }}
          </button>
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
import AuditView from './views/AuditView.vue'
import BomView from './views/BomView.vue'
import CompanyStaffView from './views/CompanyStaffView.vue'
import CustomersView from './views/CustomersView.vue'
import InventoryView from './views/InventoryView.vue'
import MachinesView from './views/MachinesView.vue'
import MaterialsView from './views/MaterialsView.vue'
import OrderEntryView from './views/OrderEntryView.vue'
import OrdersView from './views/OrdersView.vue'
import ProducePlanView from './views/ProducePlanView.vue'
import ProduceRunningView from './views/ProduceRunningView.vue'
import ProductionLogsView from './views/ProductionLogsView.vue'
import ProductsView from './views/ProductsView.vue'
import RequirementsView from './views/RequirementsView.vue'
import SenderSettingsView from './views/SenderSettingsView.vue'

const collapsed = ref(false)
const currentKey = ref('order')
const isMobile = ref(false)
const mobileOpen = ref(false)

const menuMap = {
  order: { title: '录单' },
  orders: { title: '订单列表' },
  producePlan: { title: '生产计划/开始生产' },
  produceRunning: { title: '生产中' },
  produceLogs: { title: '生产日志' },
  materials: { title: '物料档案/库存' },
  bom: { title: 'BOM配方维护' },
  customers: { title: '客户档案' },
  products: { title: '商品档案' },
  departments: { title: '部门维护' },
  employees: { title: '员工维护' },
  inventory: { title: '成品库存' },
  quotePrint: { title: '报价导出' },
  machines: { title: '设备产能配置' },
  senderSettings: { title: '发货人设置' },
  audit: { title: '操作日志' },
  reqProduct: { title: '产品需求表' },
  reqDev: { title: '开发需求表' },
  reqUnit: { title: '单元测试表' },
  reqApi: { title: 'API 测试表' },
  reqReview: { title: '需求审核表' },
}

const internalViews = {
  order: OrderEntryView,
  orders: OrdersView,
  materials: MaterialsView,
  bom: BomView,
  producePlan: ProducePlanView,
  produceRunning: ProduceRunningView,
  produceLogs: ProductionLogsView,
  customers: CustomersView,
  products: ProductsView,
  departments: CompanyStaffView,
  employees: CompanyStaffView,
  inventory: InventoryView,
  quotePrint: ProductsView,
  machines: MachinesView,
  senderSettings: SenderSettingsView,
  audit: AuditView,
  reqProduct: RequirementsView,
  reqDev: RequirementsView,
  reqUnit: RequirementsView,
  reqApi: RequirementsView,
  reqReview: RequirementsView,
}

const menuGroups = [
  { name: '订单', items: [{ key: 'order', label: '录单' }, { key: 'orders', label: '订单列表' }] },
  { name: '生产流程', items: [{ key: 'producePlan', label: '生产计划/开始生产' }, { key: 'produceRunning', label: '生产中' }, { key: 'produceLogs', label: '生产日志' }] },
  { name: '物料管理', items: [{ key: 'materials', label: '物料档案/库存' }, { key: 'bom', label: 'BOM配方维护' }] },
  { name: '档案', items: [{ key: 'customers', label: '客户档案' }, { key: 'products', label: '商品档案' }, { key: 'departments', label: '部门维护' }, { key: 'employees', label: '员工维护' }, { key: 'inventory', label: '成品库存' }, { key: 'quotePrint', label: '报价导出' }] },
  { name: '设置', items: [{ key: 'machines', label: '设备产能配置' }, { key: 'senderSettings', label: '发货人设置' }] },
  { name: '日志', items: [{ key: 'audit', label: '操作日志' }] },
  { name: '需求管理', items: [{ key: 'reqProduct', label: '产品需求表' }, { key: 'reqDev', label: '开发需求表' }, { key: 'reqUnit', label: '单元测试表' }, { key: 'reqApi', label: 'API 测试表' }, { key: 'reqReview', label: '需求审核表' }] },
]

function applyKeyToUrl(key) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', key)
  window.history.replaceState({}, '', url.toString())
}

function open(key) {
  if (!menuMap[key]) return
  currentKey.value = key
  applyKeyToUrl(key)
  if (isMobile.value) mobileOpen.value = false
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
.sidebar.collapsed .section,
.sidebar.collapsed .menu { display: none; }
.brand { font-weight: 700; margin-bottom: 10px; white-space: nowrap; }
.section { margin: 10px 0 6px; font-size: 12px; color: #666; font-weight: 600; }
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
