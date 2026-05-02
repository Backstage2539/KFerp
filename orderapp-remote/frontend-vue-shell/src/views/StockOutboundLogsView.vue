<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>出库日志</h2>
          <p>已生成的出库单记录集中在库存管理里查看，可直接打开或下载出库单 PDF。</p>
        </div>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="notice error">{{ error }}</div>
      <div class="filters">
        <label>
          <span>搜索</span>
          <input v-model.trim="filters.q" placeholder="订单号/客户/快递单号/发货方式" @keyup.enter="loadPage(1)" />
        </label>
        <label>
          <span>开始日期</span>
          <input v-model.trim="filters.from" placeholder="YYYY-MM-DD" @keyup.enter="loadPage(1)" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model.trim="filters.to" placeholder="YYYY-MM-DD" @keyup.enter="loadPage(1)" />
        </label>
        <button class="primary" type="button" @click="loadPage(1)" :disabled="loading">查询</button>
      </div>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>生成时间</th>
              <th>订单</th>
              <th>客户</th>
              <th>出库信息</th>
              <th>快递</th>
              <th>订单状态</th>
              <th>版本</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.document_id">
              <td>{{ row.created_at || '-' }}</td>
              <td><button class="order-link" type="button" @click="openDeliveryNote(row)">{{ row.order_no || `#${row.order_id}` }}</button></td>
              <td>{{ row.customer_name || '-' }}</td>
              <td>
                <div class="stack">
                  <span>日期：{{ row.posting_date || '-' }}</span>
                  <span>仓库：{{ row.warehouse_name || row.source_warehouse || '-' }}</span>
                </div>
              </td>
              <td>
                <div class="stack">
                  <span>{{ row.delivery_method || '顺丰发货' }}</span>
                  <strong>{{ row.tracking_no || '未记录单号' }}</strong>
                </div>
              </td>
              <td>
                <div class="status-grid">
                  <span>收款：{{ row.pay_status || '-' }}</span>
                  <span>发货：{{ row.ship_status || '-' }}</span>
                  <span>生产：{{ row.process_status || '-' }}</span>
                  <span>发票：{{ invoiceStatusLabel(row.invoice_status) }}</span>
                </div>
              </td>
              <td>
                <span :class="['version-tag', { latest: row.is_latest }]">V{{ row.version_no }}</span>
              </td>
              <td class="actions-cell">
                <button class="text-button" type="button" @click="openDeliveryNote(row)">查看出库单</button>
                <a class="text-link" :href="downloadURL(row)" target="_blank" rel="noopener">下载出库单</a>
              </td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="8" class="muted">暂无出库日志</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="pager">
        <button class="secondary" type="button" @click="loadPage(page - 1)" :disabled="page <= 1 || loading">上一页</button>
        <span>第 {{ page }} 页</span>
        <button class="secondary" type="button" @click="loadPage(page + 1)" :disabled="!hasNext || loading">下一页</button>
      </div>
    </section>

    <div v-if="deliveryNoteDrawerOpen" class="delivery-note-drawer-mask" @click.self="closeDeliveryNote">
      <aside class="delivery-note-drawer" aria-label="出库单">
        <DeliveryNoteView :order-id="activeOrderID" embedded @close="closeDeliveryNote" />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet } from '../api/client'
import DeliveryNoteView from './DeliveryNoteView.vue'

const rows = ref([])
const loading = ref(false)
const error = ref('')
const page = ref(1)
const hasNext = ref(false)
const limit = 50
const deliveryNoteDrawerOpen = ref(false)
const activeOrderID = ref(0)
const filters = reactive({ q: '', from: '', to: '' })

async function loadPage(nextPage) {
  page.value = Math.max(1, Number(nextPage || 1))
  await load()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/stock/outbound-logs', window.location.origin)
    url.searchParams.set('limit', String(limit))
    url.searchParams.set('offset', String((page.value - 1) * limit))
    for (const [key, val] of Object.entries(filters)) {
      if (val) url.searchParams.set(key, val)
    }
    const data = await apiGet(url)
    rows.value = data.rows || []
    hasNext.value = Boolean(data.has_next)
  } catch (err) {
    error.value = err.message || '加载出库日志失败'
  } finally {
    loading.value = false
  }
}

function openDeliveryNote(row) {
  activeOrderID.value = Number(row?.order_id || 0)
  if (!activeOrderID.value) return
  deliveryNoteDrawerOpen.value = true
}

function closeDeliveryNote() {
  deliveryNoteDrawerOpen.value = false
  activeOrderID.value = 0
  load()
}

function downloadURL(row) {
  return row?.download_url || `/orders/${Number(row?.order_id || 0)}/delivery-note-latest.pdf`
}

function invoiceStatusLabel(value) {
  if (value === 'uploaded') return '已上传'
  if (value === 'requested') return '已申请'
  return value || '未申请'
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 16px; display: grid; gap: 16px; color: #171717; }
.panel { border: 1px solid #e5e5e5; border-radius: 8px; padding: 14px; background: #fff; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 19px; }
p { margin: 6px 0 0; color: #555; font-size: 13px; }
.filters { display: grid; grid-template-columns: minmax(260px, 1fr) 150px 150px 90px; gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, button { font: inherit; min-height: 36px; border-radius: 6px; }
input { width: 100%; border: 1px solid #ddd; padding: 7px 9px; }
button { padding: 8px 12px; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.primary { border: 1px solid #1f6f4a; background: #1f6f4a; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.notice { padding: 10px 12px; border-radius: 7px; margin-bottom: 12px; }
.notice.error { background: #fff1f1; border: 1px solid #f0caca; color: #9d1c1c; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1180px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #f0f0f0; padding: 9px 8px; text-align: left; font-size: 13px; vertical-align: top; }
th { background: #fbfbfb; color: #555; font-weight: 600; }
.order-link, .text-button { border: 0; background: transparent; color: #1f6f4a; padding: 0; min-height: 0; font-weight: 600; text-align: left; }
.text-link { color: #1f6f4a; text-decoration: none; font-weight: 600; }
.stack { display: grid; gap: 4px; }
.stack strong { color: #1f2937; }
.status-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 4px 10px; min-width: 220px; }
.version-tag { display: inline-flex; align-items: center; min-height: 24px; border: 1px solid #d8d8d8; border-radius: 999px; padding: 2px 8px; background: #f7f7f7; color: #444; }
.version-tag.latest { border-color: #cfe8d4; background: #eef8f1; color: #1f6f4a; }
.actions-cell { display: flex; gap: 10px; align-items: center; white-space: nowrap; }
.muted { color: #777; text-align: center; }
.pager { display: flex; align-items: center; justify-content: flex-end; gap: 10px; margin-top: 12px; }
.delivery-note-drawer-mask { position: fixed; inset: 0; z-index: 40; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .24); }
.delivery-note-drawer { width: min(980px, calc(100vw - 28px)); height: 100%; overflow: auto; background: #f8f7f4; border-left: 1px solid #e6e0d8; box-shadow: -10px 0 24px rgba(0, 0, 0, .14); }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { flex-direction: column; align-items: stretch; }
  .filters { grid-template-columns: 1fr; }
  .delivery-note-drawer { width: 100vw; }
}
</style>
