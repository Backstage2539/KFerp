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
      <div class="shipping-bar">
        <div>
          <h3>快递处理</h3>
          <p>订单生产完成后，在这里勾选并生成快递录单 Excel。</p>
        </div>
        <div class="shipping-actions">
          <label class="sender-picker">
            <span>本次寄件人</span>
            <select v-model.number="selectedSenderID">
              <option :value="0">默认寄件人</option>
              <option v-for="profile in senderProfiles" :key="profile.sender_id" :value="profile.sender_id">
                {{ profile.sender_label || profile.sender_name || `寄件人${profile.sender_id}` }}{{ profile.is_default ? '（默认）' : '' }}
              </option>
            </select>
          </label>
          <button class="secondary" type="button" @click="applyShipReadyPreset" :disabled="loading">只看生产完成</button>
          <button class="secondary" type="button" @click="selectVisibleCompleted" :disabled="!rows.length">勾选本页生产完成</button>
          <button class="primary" type="button" @click="generateShippingExcel" :disabled="shippingLoading || !selectedOrderIDs.length">
            {{ shippingLoading ? '生成中' : `生成快递录单 Excel(${selectedOrderIDs.length})` }}
          </button>
        </div>
      </div>
      <div v-if="shippingMessage" class="notice ok">
        <span>{{ shippingMessage }}</span>
        <a v-if="shippingExcelUrl" :href="shippingExcelUrl" target="_blank" rel="noopener">下载 Excel</a>
      </div>
      <div v-if="currentShipment.shipment_id" class="tracking-box">
        <div class="tracking-head">
          <div>
            <strong>{{ currentShipment.shipment_no }}</strong>
            <span>回填快递单号后，订单会自动标记为已发货。</span>
          </div>
          <button class="secondary" type="button" @click="load" :disabled="loading">刷新列表</button>
        </div>
        <div class="tracking-grid">
          <label v-for="row in selectedRows" :key="row.id">
            <span>{{ row.order_no }} · {{ row.customer }}</span>
            <input v-model.trim="trackingInputs[Number(row.id)]" placeholder="快递单号" />
          </label>
        </div>
        <div class="tracking-actions">
          <button class="primary" type="button" @click="fillShipmentTracking" :disabled="trackingLoading || !trackingReady">
            {{ trackingLoading ? '回填中' : '回填单号并标记已发货' }}
          </button>
        </div>
      </div>
      <div v-if="shippingError" class="notice error">{{ shippingError }}</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th class="select-col">发货</th>
              <th class="row-sender">寄件人</th>
              <th>订单号</th>
              <th>日期</th>
              <th>客户</th>
              <th>金额</th>
              <th>类型</th>
              <th>收款</th>
              <th>发货</th>
              <th>单号</th>
              <th>生产</th>
              <th>录入</th>
              <th>备注</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" :class="{ voided: row.is_void }">
              <td class="select-col">
                <input
                  type="checkbox"
                  :checked="selectedOrderIDs.includes(Number(row.id))"
                  :disabled="!isProductionComplete(row)"
                  :title="isProductionComplete(row) ? '选择发货' : '生产完成后可发货'"
                  @change="toggleOrder(row, $event.target.checked)"
                />
              </td>
              <td class="row-sender">
                <select
                  v-if="selectedOrderIDs.includes(Number(row.id)) && isProductionComplete(row)"
                  v-model.number="orderSenderIDs[Number(row.id)]"
                  aria-label="单独选择寄件人"
                >
                  <option :value="0">跟随默认</option>
                  <option v-for="profile in senderProfiles" :key="profile.sender_id" :value="profile.sender_id">
                    {{ profile.sender_label || profile.sender_name || `寄件人${profile.sender_id}` }}{{ profile.is_default ? '（默认）' : '' }}
                  </option>
                </select>
                <span v-else class="muted">-</span>
              </td>
              <td><a :href="`/order?edit_id=${row.id}`">{{ row.order_no }}</a></td>
              <td>{{ row.order_date }}</td>
              <td>{{ row.customer }}</td>
              <td>{{ row.grand_total }}</td>
              <td>{{ row.order_type }}</td>
              <td>{{ row.pay_status }}</td>
              <td>{{ row.ship_status }}</td>
              <td>{{ row.ship_tracking_no || '-' }}</td>
              <td>{{ row.process_status }}</td>
              <td>{{ row.created_by_employee }}</td>
              <td class="notes">{{ row.notes }}</td>
              <td><a class="text-link" :href="`/orders/${row.id}/audit`">审计</a></td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="14" class="muted">暂无订单</td>
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
import { computed, onMounted, reactive, ref } from 'vue'
import { replaceHistoryURL } from '../lib/url-state'

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
const selectedOrderIDs = ref([])
const shippingLoading = ref(false)
const shippingExcelUrl = ref('')
const shippingMessage = ref('')
const shippingError = ref('')
const senderProfiles = ref([])
const selectedSenderID = ref(0)
const orderSenderIDs = reactive({})
const currentShipment = reactive({ shipment_id: 0, shipment_no: '' })
const trackingInputs = reactive({})
const trackingLoading = ref(false)

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

const completedStatusID = computed(() => {
  const hit = processStatuses.value.find((item) => String(item.name || '').includes('生产完成'))
  return Number(hit?.id || 0)
})

const selectedRows = computed(() => {
  const selected = new Set(selectedOrderIDs.value.map((id) => Number(id)))
  return rows.value.filter((row) => selected.has(Number(row.id)))
})

const trackingReady = computed(() => {
  return selectedRows.value.some((row) => String(trackingInputs[Number(row.id)] || '').trim() !== '')
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
  replaceHistoryURL(url)
}

async function loadPage(nextPage) {
  page.value = Math.max(1, nextPage)
  await load()
}

function isProductionComplete(row) {
  return String(row?.process_status || '').includes('生产完成')
}

function toggleOrder(row, checked) {
  const id = Number(row?.id || 0)
  if (!id || !isProductionComplete(row)) return
  if (checked) {
    if (!selectedOrderIDs.value.includes(id)) selectedOrderIDs.value = [...selectedOrderIDs.value, id]
    if (orderSenderIDs[id] === undefined) orderSenderIDs[id] = 0
  } else {
    selectedOrderIDs.value = selectedOrderIDs.value.filter((item) => item !== id)
    delete orderSenderIDs[id]
  }
  shippingError.value = ''
}

function selectVisibleCompleted() {
  const ids = rows.value.filter(isProductionComplete).map((row) => Number(row.id)).filter(Boolean)
  selectedOrderIDs.value = Array.from(new Set([...selectedOrderIDs.value, ...ids]))
  for (const id of ids) {
    if (orderSenderIDs[id] === undefined) orderSenderIDs[id] = 0
  }
  shippingError.value = ids.length ? '' : '本页没有生产完成的订单'
}

async function applyShipReadyPreset() {
  if (completedStatusID.value) filters.process_status_id = completedStatusID.value
  filters.void = 'normal'
  await loadPage(1)
}

async function generateShippingExcel() {
  shippingLoading.value = true
  shippingError.value = ''
  shippingMessage.value = ''
  shippingExcelUrl.value = ''
  try {
    const orderSenders = selectedOrderIDs.value
      .map((id) => ({ order_id: id, sender_id: Number(orderSenderIDs[id] || 0) }))
      .filter((item) => item.sender_id > 0)
    const res = await fetch('/api/orders/shipping-excel', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ order_ids: selectedOrderIDs.value, sender_id: selectedSenderID.value, order_senders: orderSenders }),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '生成失败')
    shippingExcelUrl.value = data.shipping_excel_url || ''
    currentShipment.shipment_id = Number(data.shipment_id || 0)
    currentShipment.shipment_no = data.shipment_no || ''
    for (const id of selectedOrderIDs.value) {
      if (trackingInputs[id] === undefined) trackingInputs[id] = ''
    }
    shippingMessage.value = `已生成 ${Number(data.count || selectedOrderIDs.value.length)} 个订单的快递录单：${currentShipment.shipment_no || '待回填'}`
  } catch (err) {
    shippingError.value = err.message || '生成失败'
  } finally {
    shippingLoading.value = false
  }
}

async function fillShipmentTracking() {
  trackingLoading.value = true
  shippingError.value = ''
  shippingMessage.value = ''
  try {
    const items = selectedRows.value
      .map((row) => ({
        order_id: Number(row.id),
        tracking_no: String(trackingInputs[Number(row.id)] || '').trim(),
      }))
      .filter((item) => item.tracking_no)
    const res = await fetch('/api/orders/shipping-tracking', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ shipment_id: currentShipment.shipment_id, items }),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '回填失败')
    shippingMessage.value = `已回填 ${Number(data.updated || 0)} 个订单并标记已发货`
    selectedOrderIDs.value = []
    for (const key of Object.keys(orderSenderIDs)) delete orderSenderIDs[key]
    for (const key of Object.keys(trackingInputs)) delete trackingInputs[key]
    currentShipment.shipment_id = 0
    currentShipment.shipment_no = ''
    await load()
  } catch (err) {
    shippingError.value = err.message || '回填失败'
  } finally {
    trackingLoading.value = false
  }
}

async function loadSenderProfiles() {
  try {
    const res = await fetch('/api/settings/sender')
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载寄件人失败')
    senderProfiles.value = data.profiles || []
    const def = senderProfiles.value.find((profile) => profile.is_default)
    selectedSenderID.value = Number(def?.sender_id || 0)
  } catch (err) {
    shippingError.value = err.message || '加载寄件人失败'
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch(buildUrl(page.value))
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    rows.value = data.rows || []
    selectedOrderIDs.value = selectedOrderIDs.value.filter((id) => rows.value.some((row) => Number(row.id) === id && isProductionComplete(row)))
    for (const key of Object.keys(orderSenderIDs)) {
      const id = Number(key)
      if (!selectedOrderIDs.value.includes(id)) delete orderSenderIDs[key]
    }
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
  loadSenderProfiles()
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
.shipping-bar { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.shipping-bar h3 { margin: 0 0 4px; font-size: 17px; }
.shipping-bar p { margin: 0; color: #666; font-size: 13px; }
.shipping-actions { align-self: end; display: flex; flex-wrap: wrap; align-items: flex-end; gap: 8px; justify-content: flex-end; }
.sender-picker { min-width: 180px; }
.sender-picker select { height: 38px; }
.notice { display: flex; align-items: center; justify-content: space-between; gap: 10px; border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.ok { background: #eef8f1; border: 1px solid #b9dfc4; color: #1f6b38; }
.tracking-box { border: 1px solid #d9e3ef; border-radius: 8px; padding: 10px; margin-bottom: 12px; background: #f8fbff; }
.tracking-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 10px; }
.tracking-head strong { display: block; margin-bottom: 3px; }
.tracking-head span { color: #576575; font-size: 13px; }
.tracking-grid { display: grid; grid-template-columns: repeat(3, minmax(180px, 1fr)); gap: 10px; }
.tracking-actions { display: flex; justify-content: flex-end; margin-top: 10px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1180px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.select-col { width: 54px; text-align: center; }
.select-col input { width: 18px; height: 18px; padding: 0; }
.row-sender { width: 170px; }
.row-sender select { width: 154px; height: 34px; padding: 5px 7px; }
a, .text-link { color: #1f4f82; text-decoration: none; }
.notes { max-width: 220px; white-space: pre-wrap; }
.muted { color: #666; text-align: center; }
.voided { color: #8a1f1f; background: #fff7f7; }
.pager { display: flex; gap: 10px; align-items: center; justify-content: flex-end; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; border-radius: 6px; padding: 9px; margin-bottom: 12px; color: #8a1f1f; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters { grid-template-columns: 1fr; }
  .shipping-bar { align-items: stretch; flex-direction: column; }
  .shipping-actions { align-self: stretch; justify-content: flex-start; }
  .tracking-head { align-items: stretch; flex-direction: column; }
  .tracking-grid { grid-template-columns: 1fr; }
  table { min-width: 980px; }
}
</style>
