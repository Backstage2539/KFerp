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
