<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>订单列表</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="scope-tabs" role="tablist" aria-label="订单范围">
        <button type="button" :class="{ active: filters.scope === 'all' }" @click="setScope('all')">全部订单</button>
        <button type="button" :class="{ active: filters.scope === 'mine' }" @click="setScope('mine')">我的订单</button>
        <button type="button" :class="{ active: filters.scope === 'fulfillment' }" @click="setScope('fulfillment')">履约客户订单</button>
      </div>
      <div class="filters">
        <label>
          <span>搜索</span>
          <input v-model.trim="filters.q" placeholder="订单号/客户/负责人" @keyup.enter="loadPage(1)" />
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
          <h3>顺丰发货</h3>
          <p>订单生产完成、标记“无需生产”或进入“库存待发货”后，在这里勾选并生成顺丰发货 Excel；单号在订单抽屉内回填。</p>
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
          <button class="secondary" type="button" @click="applyShipReadyPreset" :disabled="loading">只看可发货</button>
          <button class="secondary" type="button" @click="selectVisibleShipReady" :disabled="!rows.length">勾选本页可发货</button>
          <button class="primary" type="button" @click="generateShippingExcel" :disabled="shippingLoading || !selectedOrderIDs.length">
            {{ shippingLoading ? '生成中' : `生成顺丰发货 Excel(${selectedOrderIDs.length})` }}
          </button>
          <label class="tracking-upload">
            <span>回传 Excel</span>
            <input type="file" accept=".xlsx,.xls" @change="handleTrackingExcelFile" />
          </label>
          <button class="secondary" type="button" @click="uploadTrackingExcel" :disabled="trackingExcelLoading || !trackingExcelFile">
            {{ trackingExcelLoading ? '上传中' : '上传回填' }}
          </button>
        </div>
      </div>
      <div v-if="shippingMessage" class="notice ok">
        <span>{{ shippingMessage }}</span>
        <a v-if="shippingExcelUrl" :href="shippingExcelUrl" target="_blank" rel="noopener">下载 Excel</a>
      </div>
      <div v-if="shippingError" class="notice error">{{ shippingError }}</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th class="select-col">发货</th>
              <th>订单号</th>
              <th>日期</th>
              <th>客户</th>
              <th>负责人</th>
              <th>金额</th>
              <th>类型</th>
              <th>快递信息</th>
              <th>订单状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" :class="orderRowClass(row)">
              <td class="select-col">
                <input
                  type="checkbox"
                  :checked="selectedOrderIDs.includes(Number(row.id))"
                  :disabled="!isShipReady(row)"
                  :title="isShipReady(row) ? '选择发货' : '生产完成、无需生产或库存待发货后可发货'"
                  @change="toggleOrder(row, $event.target.checked)"
                />
              </td>
              <td><button class="order-link" type="button" @click.prevent="openOrderDetailDrawer(row)">{{ row.order_no }}</button></td>
              <td>{{ row.order_date }}</td>
              <td>{{ row.customer }}</td>
              <td>{{ row.responsible_name || '-' }}</td>
              <td>{{ row.grand_total }}</td>
              <td>{{ row.order_type }}</td>
              <td>
                <div class="shipping-summary">
                  <span>{{ senderDisplay(row) }}</span>
                  <strong :title="row.ship_tracking_no || ''">{{ formatTrackingSummary(row.ship_tracking_no) }}</strong>
                </div>
              </td>
              <td>
                <div class="status-stack">
                  <span>收款：{{ row.pay_status || '-' }}{{ row.payment_method ? ' / ' + row.payment_method : '' }}</span>
                  <span>发货：{{ row.ship_status || '-' }}</span>
                  <span>生产：{{ row.process_status || '-' }}</span>
                  <span>发票：{{ invoiceStatusLabel(row.invoice_status) }}</span>
                </div>
              </td>
              <td class="actions-cell">
                <a class="text-link" href="#" @click.prevent="openSalesOrderDrawer(row)">销售单</a>
                <a v-if="isShipped(row)" class="text-link" href="#" @click.prevent="openDeliveryNoteDrawer(row)">出库单</a>
                <span v-else class="muted inline-muted">出库单</span>
                <a class="text-link" href="#" @click.prevent="openInvoiceDrawer(row)">发票</a>
                <a class="text-link" :href="`/orders/${row.id}/audit`">审计</a>
              </td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="10" class="muted">暂无订单</td>
            </tr>
          </tbody>
        </table>
      </div>
      <PaginationControls
        :page="page"
        :page-size="filters.limit"
        :total="totalOrders"
        :disabled="loading"
        @change="handlePaginationChange"
      />
    </section>

    <div v-if="orderDetailDrawerOpen" class="order-detail-drawer-mask" @click.self="closeOrderDetailDrawer">
      <aside class="order-detail-drawer" aria-label="订单详情">
        <div class="drawer-head">
          <div>
            <h3>{{ activeOrderDetail?.order_no || '订单详情' }}</h3>
            <p>{{ activeOrderDetail?.customer || '-' }} · {{ activeOrderDetail?.order_date || '-' }}</p>
          </div>
          <button class="secondary" type="button" @click="closeOrderDetailDrawer">关闭</button>
        </div>
        <div v-if="shippingError" class="notice error">{{ shippingError }}</div>
        <div v-if="shippingMessage" class="notice ok">{{ shippingMessage }}</div>
        <div v-if="activeOrderDetail" class="drawer-body">
          <section class="drawer-section">
            <h4>快递信息</h4>
            <div class="drawer-grid">
              <label>
                <span>寄件人</span>
                <select
                  v-model.number="orderSenderIDs[Number(activeOrderDetail.id)]"
                  aria-label="选择寄件人"
                >
                  <option :value="0">跟随本次寄件人：{{ globalSenderLabel() }}</option>
                  <option v-for="profile in senderProfiles" :key="profile.sender_id" :value="profile.sender_id">
                    {{ profileLabel(profile) }}{{ profile.is_default ? '（默认）' : '' }}
                  </option>
                </select>
              </label>
              <label class="tracking-fill">
                <span>快递单号（可多个）</span>
                <div class="tracking-fill-row">
                  <textarea v-model.trim="drawerTrackingNo" rows="3" placeholder="多个单号可用换行、逗号或分号分隔" />
                  <button class="primary" type="button" @click="fillOrderTracking" :disabled="drawerTrackingSaving || !trackingInputSummary(drawerTrackingNo)">
                    {{ drawerTrackingSaving ? '回填中' : '回填' }}
                  </button>
                </div>
              </label>
            </div>
          </section>
          <section class="drawer-section">
            <h4>订单状态</h4>
            <div class="drawer-status-grid">
              <span>收款：{{ activeOrderDetail.pay_status || '-' }}{{ activeOrderDetail.payment_method ? ' / ' + activeOrderDetail.payment_method : '' }}</span>
              <span>发货：{{ activeOrderDetail.ship_status || '-' }}</span>
              <span>生产：{{ activeOrderDetail.process_status || '-' }}</span>
              <span>发票：{{ invoiceStatusLabel(activeOrderDetail.invoice_status) }}</span>
            </div>
          </section>
          <section class="drawer-section">
            <h4>订单信息</h4>
            <div class="drawer-status-grid">
              <span>金额：{{ activeOrderDetail.grand_total || '-' }}</span>
              <span>类型：{{ activeOrderDetail.order_type || '-' }}</span>
              <span>负责人：{{ activeOrderDetail.responsible_name || '-' }}</span>
              <span>录入：{{ activeOrderDetail.created_by_employee || '-' }}</span>
              <span>备注：{{ activeOrderDetail.notes || '-' }}</span>
            </div>
          </section>
          <div class="drawer-actions">
            <button class="secondary" type="button" @click="openSalesOrderDrawer(activeOrderDetail)">销售单</button>
            <button class="secondary" type="button" @click="openDeliveryNoteDrawer(activeOrderDetail)" :disabled="!isShipped(activeOrderDetail)">出库单</button>
            <button class="secondary" type="button" @click="openInvoiceDrawer(activeOrderDetail)">发票</button>
          </div>
          <section class="drawer-section order-edit-panel">
            <OrderEntryView
              :key="activeOrderDetail.id"
              :edit-id="activeOrderDetail.id"
              embedded
              @close="closeOrderDetailDrawer"
              @saved="handleOrderEditSaved"
            />
          </section>
        </div>
      </aside>
    </div>

    <div v-if="salesOrderDrawerOpen" class="sales-order-drawer-mask" @click.self="closeSalesOrderDrawer">
      <aside class="sales-order-drawer" aria-label="销售单">
        <SalesOrderView :order-id="activeSalesOrderID" embedded @close="closeSalesOrderDrawer" />
      </aside>
    </div>

    <div v-if="deliveryNoteDrawerOpen" class="delivery-note-drawer-mask" @click.self="closeDeliveryNoteDrawer">
      <aside class="delivery-note-drawer" aria-label="出库单">
        <DeliveryNoteView :order-id="activeDeliveryNoteID" embedded @close="closeDeliveryNoteDrawer" />
      </aside>
    </div>

    <div v-if="invoiceDrawerOpen" class="invoice-drawer-mask" @click.self="closeInvoiceDrawer">
      <aside class="invoice-drawer" aria-label="发票">
        <OrderInvoiceView :order-id="activeInvoiceID" embedded @close="closeInvoiceDrawer" @updated="load" />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import PaginationControls from '../components/PaginationControls.vue'
import { invoiceStatusLabel, invoiceStatusTone } from '../lib/order-invoice'
import { formatTrackingSummary, trackingInputSummary } from '../lib/order-shipping'
import { orderListScopeForRequest } from '../lib/order-scope'
import { normalizePageSize, paginationFromApi } from '../lib/pagination'
import { replaceHistoryURL } from '../lib/url-state'
import DeliveryNoteView from './DeliveryNoteView.vue'
import OrderEntryView from './OrderEntryView.vue'
import OrderInvoiceView from './OrderInvoiceView.vue'
import SalesOrderView from './SalesOrderView.vue'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
})

const loading = ref(false)
const error = ref('')
const rows = ref([])
const payStatuses = ref([])
const shipStatuses = ref([])
const processStatuses = ref([])
const summary = ref({})
const totalOrders = ref(0)
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
const trackingExcelFile = ref(null)
const trackingExcelLoading = ref(false)
const orderDetailDrawerOpen = ref(false)
const activeOrderDetail = ref(null)
const drawerTrackingNo = ref('')
const drawerTrackingSaving = ref(false)
const salesOrderDrawerOpen = ref(false)
const activeSalesOrderID = ref(0)
const deliveryNoteDrawerOpen = ref(false)
const activeDeliveryNoteID = ref(0)
const invoiceDrawerOpen = ref(false)
const activeInvoiceID = ref(0)

const filters = reactive({
  scope: 'all',
  highlight_order_id: 0,
  q: '',
  from: '',
  to: '',
  pay_status_id: 0,
  ship_status_id: 0,
  process_status_id: 0,
  ship_ready: false,
  void: 'normal',
  limit: 10,
})

function applyUrlFilters() {
  const params = new URL(window.location.href).searchParams
  filters.scope = orderListScopeForRequest(props.viewParams?.scope || params.get('scope') || 'all')
  filters.highlight_order_id = Number(props.viewParams?.highlight_order_id || params.get('highlight_order_id') || 0)
  filters.q = params.get('q') || ''
  filters.from = params.get('from') || ''
  filters.to = params.get('to') || ''
  filters.void = params.get('void') || 'normal'
  filters.pay_status_id = Number(params.get('pay_status_id') || 0)
  filters.ship_status_id = Number(params.get('ship_status_id') || 0)
  filters.process_status_id = Number(params.get('process_status_id') || 0)
  filters.ship_ready = params.get('ship_ready') === '1'
  filters.limit = normalizePageSize(params.get('limit') || filters.limit)
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
  if (filters.ship_ready) url.searchParams.set('ship_ready', '1')
  if (filters.scope && filters.scope !== 'all') url.searchParams.set('scope', filters.scope)
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
  if (filters.ship_ready) url.searchParams.set('ship_ready', '1')
  else url.searchParams.delete('ship_ready')
  if (filters.scope && filters.scope !== 'all') url.searchParams.set('scope', filters.scope)
  else url.searchParams.delete('scope')
  if (filters.highlight_order_id) url.searchParams.set('highlight_order_id', String(filters.highlight_order_id))
  else url.searchParams.delete('highlight_order_id')
  url.searchParams.set('page', String(nextPage))
  url.searchParams.set('limit', String(filters.limit))
  replaceHistoryURL(url)
}

async function setScope(scope) {
  filters.scope = orderListScopeForRequest(scope)
  filters.highlight_order_id = 0
  await loadPage(1)
}

function openSalesOrderDrawer(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  activeSalesOrderID.value = id
  salesOrderDrawerOpen.value = true
}

function closeSalesOrderDrawer() {
  salesOrderDrawerOpen.value = false
  activeSalesOrderID.value = 0
}

function openDeliveryNoteDrawer(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  activeDeliveryNoteID.value = id
  deliveryNoteDrawerOpen.value = true
}

function closeDeliveryNoteDrawer() {
  deliveryNoteDrawerOpen.value = false
  activeDeliveryNoteID.value = 0
}

function openInvoiceDrawer(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  activeInvoiceID.value = id
  invoiceDrawerOpen.value = true
}

function closeInvoiceDrawer() {
  invoiceDrawerOpen.value = false
  activeInvoiceID.value = 0
}

function openOrderDetailDrawer(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  activeOrderDetail.value = { ...row }
  drawerTrackingNo.value = row.ship_tracking_no || ''
  orderDetailDrawerOpen.value = true
  if (orderSenderIDs[id] === undefined) orderSenderIDs[id] = 0
}

function closeOrderDetailDrawer() {
  orderDetailDrawerOpen.value = false
  activeOrderDetail.value = null
  drawerTrackingNo.value = ''
}

async function loadPage(nextPage) {
  page.value = Math.max(1, nextPage)
  await load()
}

async function handlePaginationChange({ page: nextPage, pageSize }) {
  filters.limit = normalizePageSize(pageSize)
  await loadPage(nextPage)
}

function isShipReady(row) {
  const status = String(row?.process_status || '').trim()
  return status.includes('生产完成') || status === '无需生产' || status === '库存待发货'
}

function isShipped(row) {
  return String(row?.ship_status || '').includes('已发货')
}

function isUnpaid(row) {
  const status = String(row?.pay_status || '').trim()
  return status === '' || status.includes('未')
}

function isUnproduced(row) {
  const status = String(row?.process_status || '').trim()
  if (!status) return true
  return !(status.includes('生产完成') || status === '无需生产' || status === '库存待发货')
}

function isHighlightedOrder(row) {
  return Number(row?.id || 0) > 0 && Number(row.id) === Number(filters.highlight_order_id || 0)
}

function orderRowClass(row) {
  return {
    voided: row?.is_void,
    'highlight-new': isHighlightedOrder(row),
    'state-unpaid': !isHighlightedOrder(row) && isUnpaid(row),
    'state-unshipped': !isHighlightedOrder(row) && !isUnpaid(row) && !isShipped(row),
    'state-unproduced': !isHighlightedOrder(row) && !isUnpaid(row) && isShipped(row) && isUnproduced(row),
  }
}

function profileLabel(profile = {}) {
  return profile.sender_label || profile.sender_name || `寄件人${profile.sender_id}`
}

function senderProfileLabel(id) {
  const hit = senderProfiles.value.find((profile) => Number(profile.sender_id) === Number(id))
  return hit ? profileLabel(hit) : ''
}

function globalSenderLabel() {
  const id = Number(selectedSenderID.value || 0)
  if (id > 0) return senderProfileLabel(id) || `寄件人${id}`
  const def = senderProfiles.value.find((profile) => profile.is_default)
  return def ? profileLabel(def) : '默认寄件人'
}

function senderDisplay(row) {
  const id = Number(row?.id || 0)
  const overrideID = Number(orderSenderIDs[id] || 0)
  if (overrideID > 0) return senderProfileLabel(overrideID) || `寄件人${overrideID}`
  if (selectedOrderIDs.value.includes(id)) return globalSenderLabel()
  if (row?.sender_label || row?.sender_name) return row.sender_label || row.sender_name
  return row?.ship_tracking_no ? '未记录寄件人' : '未生成'
}

function toggleOrder(row, checked) {
  const id = Number(row?.id || 0)
  if (!id || !isShipReady(row)) return
  if (checked) {
    if (!selectedOrderIDs.value.includes(id)) selectedOrderIDs.value = [...selectedOrderIDs.value, id]
    if (orderSenderIDs[id] === undefined) orderSenderIDs[id] = 0
  } else {
    selectedOrderIDs.value = selectedOrderIDs.value.filter((item) => item !== id)
    delete orderSenderIDs[id]
  }
  shippingError.value = ''
}

function selectVisibleShipReady() {
  const ids = rows.value.filter(isShipReady).map((row) => Number(row.id)).filter(Boolean)
  selectedOrderIDs.value = Array.from(new Set([...selectedOrderIDs.value, ...ids]))
  for (const id of ids) {
    if (orderSenderIDs[id] === undefined) orderSenderIDs[id] = 0
  }
  shippingError.value = ids.length ? '' : '本页没有可发货的订单'
}

async function applyShipReadyPreset() {
  filters.process_status_id = 0
  filters.ship_ready = true
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
    const data = await apiSend('/api/orders/shipping-excel', {
      body: { order_ids: selectedOrderIDs.value, sender_id: selectedSenderID.value, order_senders: orderSenders },
    })
    shippingExcelUrl.value = data.shipping_excel_url || ''
    const shipmentNo = data.shipment_no || ''
    shippingMessage.value = `已生成 ${Number(data.count || selectedOrderIDs.value.length)} 个订单的顺丰发货录单${shipmentNo ? `：${shipmentNo}` : ''}，单号请在订单抽屉回填，或上传回传 Excel`
    await load()
  } catch (err) {
    shippingError.value = err.message || '生成失败'
  } finally {
    shippingLoading.value = false
  }
}

async function fillOrderTracking() {
  const orderID = Number(activeOrderDetail.value?.id || 0)
  const trackingNo = trackingInputSummary(drawerTrackingNo.value)
  if (!orderID || !trackingNo) return
  drawerTrackingSaving.value = true
  shippingError.value = ''
  shippingMessage.value = ''
  try {
    const data = await apiSend(`/api/orders/${activeOrderDetail.value.id}/shipping-tracking`, {
      body: { tracking_no: trackingNo },
    })
    shippingMessage.value = `已回填 ${formatTrackingSummary(trackingNo)}，并标记 ${Number(data.updated || 0)} 个订单已发货`
    await load()
    const refreshed = rows.value.find((row) => Number(row.id) === orderID)
    activeOrderDetail.value = refreshed ? { ...refreshed } : { ...activeOrderDetail.value, ship_tracking_no: trackingNo, ship_status: '已发货' }
    drawerTrackingNo.value = activeOrderDetail.value.ship_tracking_no || trackingNo
  } catch (err) {
    shippingError.value = err.message || '回填失败'
  } finally {
    drawerTrackingSaving.value = false
  }
}

async function handleOrderEditSaved() {
  const orderID = Number(activeOrderDetail.value?.id || 0)
  await load()
  const refreshed = rows.value.find((row) => Number(row.id) === orderID)
  if (refreshed) activeOrderDetail.value = { ...refreshed }
}

function handleTrackingExcelFile(event) {
  trackingExcelFile.value = event.target.files?.[0] || null
  shippingError.value = ''
}

async function uploadTrackingExcel() {
  if (!trackingExcelFile.value) {
    shippingError.value = '请选择回传 Excel'
    return
  }
  trackingExcelLoading.value = true
  shippingError.value = ''
  shippingMessage.value = ''
  try {
    const body = new FormData()
    body.append('file', trackingExcelFile.value)
    const data = await apiSend('/api/orders/shipping-tracking-excel', { body })
    shippingMessage.value = `已从 Excel 回填 ${Number(data.updated || 0)}/${Number(data.total || 0)} 个订单的快递单号`
    trackingExcelFile.value = null
    await load()
  } catch (err) {
    shippingError.value = err.message || 'Excel 回填失败'
  } finally {
    trackingExcelLoading.value = false
  }
}

async function loadSenderProfiles() {
  try {
    const data = await apiGet('/api/settings/sender')
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
    const data = await apiGet(buildUrl(page.value))
    const currentDetailID = Number(activeOrderDetail.value?.id || 0)
    const previousDrawerTrackingNo = activeOrderDetail.value?.ship_tracking_no || ''
    rows.value = data.rows || []
    if (currentDetailID) {
      const refreshed = rows.value.find((row) => Number(row.id) === currentDetailID)
      if (refreshed) {
        activeOrderDetail.value = { ...refreshed }
        if (drawerTrackingNo.value === previousDrawerTrackingNo) drawerTrackingNo.value = refreshed.ship_tracking_no || ''
      }
    }
    selectedOrderIDs.value = selectedOrderIDs.value.filter((id) => rows.value.some((row) => Number(row.id) === id && isShipReady(row)))
    for (const key of Object.keys(orderSenderIDs)) {
      const id = Number(key)
      if (!rows.value.some((row) => Number(row.id) === id)) delete orderSenderIDs[key]
    }
    payStatuses.value = data.pay_statuses || []
    shipStatuses.value = data.ship_statuses || []
    processStatuses.value = data.process_statuses || []
    summary.value = data.summary || {}
    const pagination = paginationFromApi(data)
    totalOrders.value = pagination.total || Number(summary.value.orders || rows.value.length || 0)
    hasPrev.value = pagination.hasPrev
    hasNext.value = pagination.hasNext
    page.value = pagination.page
    filters.limit = pagination.pageSize
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

watch(() => props.viewParams, async () => {
  const nextScope = orderListScopeForRequest(props.viewParams?.scope || 'all')
  const nextHighlightID = Number(props.viewParams?.highlight_order_id || 0)
  if (filters.scope === nextScope && Number(filters.highlight_order_id || 0) === nextHighlightID) return
  filters.scope = nextScope
  filters.highlight_order_id = nextHighlightID
  await loadPage(1)
}, { deep: true })
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.scope-tabs { display: inline-flex; border: 1px solid #d8d1c8; border-radius: 8px; overflow: hidden; margin-bottom: 12px; }
.scope-tabs button { height: 36px; border: 0; border-right: 1px solid #d8d1c8; border-radius: 0; background: #fff; color: #333; }
.scope-tabs button:last-child { border-right: 0; }
.scope-tabs button.active { background: #1f1f1f; color: #fff; }
.filters { display: grid; grid-template-columns: repeat(4, minmax(130px, 1fr)) 90px; gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input, select { height: 38px; }
textarea { min-height: 38px; resize: vertical; }
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
.tracking-upload { min-width: 180px; }
.tracking-upload input { padding: 6px; }
.notice { display: flex; align-items: center; justify-content: space-between; gap: 10px; border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.ok { background: #eef8f1; border: 1px solid #b9dfc4; color: #1f6b38; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 960px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
tr.highlight-new { background: #ecfdf3; box-shadow: inset 4px 0 0 #22c55e; }
tr.state-unproduced { background: #fff9e6; box-shadow: inset 4px 0 0 #eab308; }
tr.state-unshipped { background: #eef6ff; box-shadow: inset 4px 0 0 #3b82f6; }
tr.state-unpaid { background: #fff1f2; box-shadow: inset 4px 0 0 #ef4444; }
tr.voided { opacity: .55; }
.select-col { width: 54px; text-align: center; }
.select-col input { width: 18px; height: 18px; padding: 0; }
a, .text-link { color: #1f4f82; text-decoration: none; }
.link-button { display: inline-flex; align-items: center; justify-content: center; text-decoration: none; }
.order-link { height: auto; border: 0; border-radius: 0; padding: 0; background: transparent; color: #1f4f82; font: inherit; text-align: left; cursor: pointer; }
.shipping-summary { display: grid; gap: 4px; min-width: 130px; }
.shipping-summary span { color: #666; font-size: 12px; }
.shipping-summary strong { font-weight: 600; color: #171717; }
.status-stack { display: grid; grid-template-columns: repeat(2, minmax(90px, 1fr)); gap: 4px 8px; min-width: 230px; color: #333; font-size: 13px; }
.actions-cell { min-width: 210px; }
.actions-cell a, .actions-cell .inline-muted { display: inline-block; margin-right: 8px; }
.inline-muted { font-size: 14px; }
.invoice-status { display: inline-block; border: 1px solid #ddd5ca; border-radius: 6px; padding: 3px 7px; font-size: 12px; white-space: nowrap; }
.invoice-status.ok { background: #eef8f1; border-color: #cfe8d4; color: #1f6f4a; }
.invoice-status.warn { background: #fff8e8; border-color: #ead9a8; color: #765a11; }
.invoice-status.muted { color: #777; background: #f8f7f4; }
.invoice-file-link { margin-left: 6px; font-size: 12px; }
.muted { color: #666; text-align: center; }
.voided { color: #8a1f1f; background: #fff7f7; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; border-radius: 6px; padding: 9px; margin-bottom: 12px; color: #8a1f1f; }
.order-detail-drawer-mask { position: fixed; inset: 0; z-index: 35; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .24); }
.order-detail-drawer { width: min(1040px, calc(100vw - 28px)); height: 100%; overflow: auto; background: #f8f7f4; border-left: 1px solid #e6e0d8; box-shadow: -10px 0 24px rgba(0, 0, 0, .14); padding: 16px; }
.drawer-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
.drawer-head h3 { margin: 0 0 4px; font-size: 18px; }
.drawer-head p { margin: 0; color: #666; font-size: 13px; }
.drawer-body { display: grid; gap: 12px; }
.drawer-section { background: #fff; border: 1px solid #e6e0d8; border-radius: 8px; padding: 12px; }
.order-edit-panel { border: 0; background: transparent; padding: 0; }
.section-head { display: flex; justify-content: space-between; gap: 12px; align-items: center; margin-bottom: 10px; }
.drawer-section h4 { margin: 0 0 10px; font-size: 15px; }
.section-head h4 { margin-bottom: 0; }
.drawer-grid { display: grid; gap: 10px; }
.tracking-fill-row { display: grid; grid-template-columns: 1fr auto; gap: 8px; align-items: start; }
.drawer-status-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; color: #333; font-size: 13px; }
.drawer-actions { display: flex; flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
.sales-order-drawer-mask { position: fixed; inset: 0; z-index: 35; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .24); }
.sales-order-drawer { width: min(1160px, calc(100vw - 28px)); height: 100%; overflow: auto; background: #f8f7f4; border-left: 1px solid #e6e0d8; box-shadow: -10px 0 24px rgba(0, 0, 0, .14); }
.delivery-note-drawer-mask { position: fixed; inset: 0; z-index: 35; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .24); }
.delivery-note-drawer { width: min(1160px, calc(100vw - 28px)); height: 100%; overflow: auto; background: #f8f7f4; border-left: 1px solid #e6e0d8; box-shadow: -10px 0 24px rgba(0, 0, 0, .14); }
.invoice-drawer-mask { position: fixed; inset: 0; z-index: 35; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .24); }
.invoice-drawer { width: min(760px, calc(100vw - 28px)); height: 100%; overflow: auto; background: #f8f7f4; border-left: 1px solid #e6e0d8; box-shadow: -10px 0 24px rgba(0, 0, 0, .14); }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters { grid-template-columns: 1fr; }
  .shipping-bar { align-items: stretch; flex-direction: column; }
  .shipping-actions { align-self: stretch; justify-content: flex-start; }
  table { min-width: 980px; }
  .status-stack, .drawer-status-grid { grid-template-columns: 1fr; }
  .order-detail-drawer, .sales-order-drawer, .delivery-note-drawer, .invoice-drawer { width: 100vw; }
}
</style>
