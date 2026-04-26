<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>订单列表</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label>
          <span>搜索</span>
          <input v-model.trim="filters.q" placeholder="订单号/客户" @keyup.enter="loadPage(1)" />
        </label>
        <label>
          <span>开始日期</span>
          <input v-model.trim="filters.from" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model.trim="filters.to" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>收款</span>
          <select v-model.number="filters.pay_status_id">
            <option :value="0">全部</option>
            <option v-for="item in payStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>发货</span>
          <select v-model.number="filters.ship_status_id">
            <option :value="0">全部</option>
            <option v-for="item in shipStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>生产</span>
          <select v-model.number="filters.process_status_id">
            <option :value="0">全部</option>
            <option v-for="item in processStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>作废</span>
          <select v-model="filters.void">
            <option value="normal">正常</option>
            <option value="void">已作废</option>
            <option value="all">全部</option>
          </select>
        </label>
        <button class="primary" type="button" @click="loadPage(1)" :disabled="loading">查询</button>
      </div>
      <div class="summary">
        <span>订单 {{ summary.orders || 0 }}</span>
        <span>客户 {{ summary.customers || 0 }}</span>
      </div>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>订单号</th>
              <th>日期</th>
              <th>客户</th>
              <th>金额</th>
              <th>类型</th>
              <th>收款</th>
              <th>发货</th>
              <th>生产</th>
              <th>录入</th>
              <th>备注</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" :class="{ voided: row.is_void }">
              <td><a :href="`/order?edit_id=${row.id}`">{{ row.order_no }}</a></td>
              <td>{{ row.order_date }}</td>
              <td>{{ row.customer }}</td>
              <td>{{ row.grand_total }}</td>
              <td>{{ row.order_type }}</td>
              <td>{{ row.pay_status }}</td>
              <td>{{ row.ship_status }}</td>
              <td>{{ row.process_status }}</td>
              <td>{{ row.created_by_employee }}</td>
              <td class="notes">{{ row.notes }}</td>
              <td><a class="text-link" :href="`/orders/${row.id}/audit`">审计</a></td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="11" class="muted">暂无订单</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="pager">
        <button class="secondary" type="button" @click="loadPage(page - 1)" :disabled="!hasPrev || loading">上一页</button>
        <span>第 {{ page }} 页</span>
        <button class="secondary" type="button" @click="loadPage(page + 1)" :disabled="!hasNext || loading">下一页</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'

const loading = ref(false)
const error = ref('')
const rows = ref([])
const payStatuses = ref([])
const shipStatuses = ref([])
const processStatuses = ref([])
const summary = ref({})
const page = ref(1)
const hasPrev = ref(false)
const hasNext = ref(false)

const filters = reactive({
  q: '',
  from: '',
  to: '',
  pay_status_id: 0,
  ship_status_id: 0,
  process_status_id: 0,
  void: 'normal',
  limit: 10,
})

function applyUrlFilters() {
  const params = new URL(window.location.href).searchParams
  filters.q = params.get('q') || ''
  filters.from = params.get('from') || ''
  filters.to = params.get('to') || ''
  filters.void = params.get('void') || 'normal'
  filters.pay_status_id = Number(params.get('pay_status_id') || 0)
  filters.ship_status_id = Number(params.get('ship_status_id') || 0)
  filters.process_status_id = Number(params.get('process_status_id') || 0)
  page.value = Math.max(1, Number(params.get('page') || 1))
}

function buildUrl(nextPage) {
  const url = new URL('/api/orders', window.location.origin)
  for (const key of ['q', 'from', 'to', 'void']) {
    if (filters[key]) url.searchParams.set(key, filters[key])
  }
  for (const key of ['pay_status_id', 'ship_status_id', 'process_status_id']) {
    if (filters[key]) url.searchParams.set(key, String(filters[key]))
  }
  url.searchParams.set('page', String(nextPage))
  url.searchParams.set('limit', String(filters.limit))
  return url
}

function updateBrowserUrl(nextPage) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'orders')
  for (const key of ['q', 'from', 'to', 'void']) {
    if (filters[key]) url.searchParams.set(key, filters[key])
    else url.searchParams.delete(key)
  }
  for (const key of ['pay_status_id', 'ship_status_id', 'process_status_id']) {
    if (filters[key]) url.searchParams.set(key, String(filters[key]))
    else url.searchParams.delete(key)
  }
  url.searchParams.set('page', String(nextPage))
  window.history.replaceState({}, '', url.toString())
}

async function loadPage(nextPage) {
  page.value = Math.max(1, nextPage)
  await load()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch(buildUrl(page.value))
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    rows.value = data.rows || []
    payStatuses.value = data.pay_statuses || []
    shipStatuses.value = data.ship_statuses || []
    processStatuses.value = data.process_statuses || []
    summary.value = data.summary || {}
    hasPrev.value = !!data.has_prev
    hasNext.value = !!data.has_next
    page.value = Number(data.page || page.value)
    updateBrowserUrl(page.value)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  applyUrlFilters()
  load()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.filters { display: grid; grid-template-columns: repeat(4, minmax(130px, 1fr)) 90px; gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.summary { display: flex; gap: 14px; margin-top: 10px; color: #555; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1180px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
a, .text-link { color: #1f4f82; text-decoration: none; }
.notes { max-width: 220px; white-space: pre-wrap; }
.muted { color: #666; text-align: center; }
.voided { color: #8a1f1f; background: #fff7f7; }
.pager { display: flex; gap: 10px; align-items: center; justify-content: flex-end; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; border-radius: 6px; padding: 9px; margin-bottom: 12px; color: #8a1f1f; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters { grid-template-columns: 1fr; }
  table { min-width: 980px; }
}
</style>
