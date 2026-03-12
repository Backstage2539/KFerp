<template>
  <div class="layout">
    <aside class="sidebar" :class="{ collapsed }">
      <div class="brand">ERP</div>
      <button class="toggle" @click="collapsed = !collapsed">{{ collapsed ? '弹出菜单' : '收起菜单' }}</button>
      <nav>
        <button class="menu" :class="{ active: currentKey === 'order' }" @click="open('order')">录单</button>
        <button class="menu" :class="{ active: currentKey === 'orders' }" @click="open('orders')">订单列表</button>
        <button class="menu" :class="{ active: currentKey === 'producePlan' }" @click="open('producePlan')">生产计划/开始生产</button>
        <button class="menu" :class="{ active: currentKey === 'produceRunning' }" @click="open('produceRunning')">生产中</button>
        <button class="menu" :class="{ active: currentKey === 'materials' }" @click="open('materials')">物料档案/库存</button>
        <button class="menu" :class="{ active: currentKey === 'bom' }" @click="open('bom')">BOM配方维护</button>
        <button class="menu" :class="{ active: currentKey === 'customers' }" @click="open('customers')">客户档案</button>
        <button class="menu" :class="{ active: currentKey === 'products' }" @click="open('products')">商品档案</button>
        <button class="menu" :class="{ active: currentKey === 'departments' }" @click="open('departments')">部门维护</button>
        <button class="menu" :class="{ active: currentKey === 'employees' }" @click="open('employees')">员工维护</button>
        <button class="menu" :class="{ active: currentKey === 'inventory' }" @click="open('inventory')">成品库存</button>
        <button class="menu" :class="{ active: currentKey === 'quotePrint' }" @click="open('quotePrint')">报价导出</button>
        <button class="menu" :class="{ active: currentKey === 'machines' }" @click="open('machines')">设备产能配置</button>
        <button class="menu" :class="{ active: currentKey === 'senderSettings' }" @click="open('senderSettings')">发货人设置</button>
        <button class="menu" :class="{ active: currentKey === 'audit' }" @click="open('audit')">操作日志</button>
        <button class="menu" :class="{ active: currentKey === 'reqProduct' }" @click="open('reqProduct')">产品需求表</button>
        <button class="menu" :class="{ active: currentKey === 'reqDev' }" @click="open('reqDev')">开发需求表</button>
        <button class="menu" :class="{ active: currentKey === 'reqUnit' }" @click="open('reqUnit')">单元测试表</button>
        <button class="menu" :class="{ active: currentKey === 'reqApi' }" @click="open('reqApi')">API 测试表</button>
      </nav>
    </aside>
    <main class="content">
      <header class="top">
        <button class="toggle" @click="collapsed = !collapsed">{{ collapsed ? '弹出' : '收起' }}</button>
        <div class="title">{{ title }}</div>
      </header>
      <iframe class="frame" :src="currentUrl" />
    </main>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'

const collapsed = ref(false)
const currentKey = ref('order')
const menuMap = {
  order: { title: '录单', url: '/order' },
  orders: { title: '订单列表', url: '/orders' },
  producePlan: { title: '生产计划/开始生产', url: '/produce/unproduced' },
  produceRunning: { title: '生产中', url: '/produce/running' },
  materials: { title: '物料档案/库存', url: '/materials' },
  bom: { title: 'BOM配方维护', url: '/bom-react' },
  customers: { title: '客户档案', url: '/customers' },
  products: { title: '商品档案', url: '/products' },
  departments: { title: '部门维护', url: '/company/departments' },
  employees: { title: '员工维护', url: '/company/employees' },
  inventory: { title: '成品库存', url: '/products/inventory' },
  quotePrint: { title: '报价导出', url: '/products/print' },
  machines: { title: '设备产能配置', url: '/produce/machines' },
  senderSettings: { title: '发货人设置', url: '/settings/sender' },
  audit: { title: '操作日志', url: '/audit' },
  reqProduct: { title: '产品需求表', url: '/req/product' },
  reqDev: { title: '开发需求表', url: '/req/dev' },
  reqUnit: { title: '单元测试表', url: '/req/unit' },
  reqApi: { title: 'API 测试表', url: '/req/api' },
}

function open(key) {
  if (!menuMap[key]) return
  currentKey.value = key
}

const title = computed(() => menuMap[currentKey.value]?.title || '')
const currentUrl = computed(() => menuMap[currentKey.value]?.url || '/order')
</script>

<style scoped>
* { box-sizing: border-box; }
.layout { display: flex; min-height: 100vh; font-family: system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial; }
.sidebar { width: 220px; border-right: 1px solid #eee; padding: 12px; background: #fafafa; transition: width .2s ease; overflow: hidden; }
.sidebar.collapsed { width: 72px; }
.brand { font-weight: 700; margin-bottom: 10px; white-space: nowrap; }
.toggle { border: 1px solid #999; background: #fff; border-radius: 8px; padding: 6px 10px; cursor: pointer; margin-bottom: 12px; }
.menu { width: 100%; text-align: left; border: 1px solid #ddd; background: #fff; border-radius: 8px; padding: 10px; cursor: pointer; margin-bottom: 8px; }
.menu.active { border-color: #111; background: #111; color: #fff; }
.content { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.top { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-bottom: 1px solid #eee; }
.title { font-weight: 600; }
.frame { width: 100%; height: calc(100vh - 56px); border: 0; background: #fff; }
@media (max-width: 900px) {
  .sidebar { position: fixed; z-index: 20; height: 100vh; }
  .content { margin-left: 220px; }
  .sidebar.collapsed + .content { margin-left: 72px; }
}
</style>
