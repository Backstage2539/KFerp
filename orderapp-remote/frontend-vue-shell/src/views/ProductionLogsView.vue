<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>生产日志</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label>
          <span>开始日期</span>
          <input v-model.trim="filters.from" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model.trim="filters.to" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>产品</span>
          <select v-model.number="filters.product_id">
            <option :value="0">全部产品</option>
            <option v-for="p in products" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </label>
        <label>
          <span>批次号</span>
          <input v-model.trim="filters.batch_id" placeholder="例如 PB-20260425-001" />
        </label>
        <label>
          <span>完成人</span>
          <input v-model.trim="filters.operator" placeholder="员工姓名" />
        </label>
        <button class="primary" type="button" @click="load" :disabled="loading">筛选</button>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">生产日志明细</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>完成时间</th>
              <th>批次</th>
              <th>产品</th>
              <th>规格(g)</th>
              <th>订单号</th>
              <th>计划成品(g)</th>
              <th>投料数(g)</th>
              <th>BOM出品率</th>
              <th>完成件数</th>
              <th>散装余料(g)</th>
              <th>实际产出(g)</th>
              <th>真实出品率</th>
              <th>开始人</th>
              <th>开始时间</th>
              <th>完成人</th>
              <th>库存前</th>
              <th>库存后</th>
              <th>物料摘要</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.finished_at }}</td>
              <td>{{ row.batch_id }}</td>
              <td>{{ row.product_name }}</td>
              <td>{{ row.spec_g }}</td>
              <td class="muted">{{ row.order_nos }}</td>
              <td>{{ row.planned_need_g }}</td>
              <td>{{ row.input_g }}</td>
              <td>{{ percent(row.bom_yield_rate) }}</td>
              <td>{{ row.finished_units }}</td>
              <td>{{ row.finished_loose_g }}</td>
              <td>{{ row.finished_total_g }}</td>
              <td>{{ percent(row.actual_yield_rate) }}</td>
              <td>{{ row.started_by }}</td>
              <td>{{ row.started_at }}</td>
              <td>{{ row.finished_by }}</td>
              <td>{{ row.inventory_units_before }} 件 / {{ row.inventory_loose_g_before }}g</td>
              <td>{{ row.inventory_units_after }} 件 / {{ row.inventory_loose_g_after }}g</td>
              <td class="summary">{{ materialSummaryText(row.material_summary) }}</td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="18" class="muted">暂无生产日志</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { replaceHistoryURL } from '../lib/url-state'

const loading = ref(false)
const error = ref('')
const products = ref([])
const rows = ref([])

const filters = reactive({
  from: '',
  to: '',
  product_id: 0,
  batch_id: '',
  operator: '',
})

function percent(v) {
  return `${(Number(v || 0) * 100).toFixed(2)}%`
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'produceLogs')
  for (const key of ['from', 'to', 'batch_id', 'operator']) {
    if (filters[key]) url.searchParams.set(key, filters[key])
    else url.searchParams.delete(key)
  }
  if (filters.product_id) url.searchParams.set('product_id', String(filters.product_id))
  else url.searchParams.delete('product_id')
  replaceHistoryURL(url)
}

function applyUrlFilters() {
  const params = new URL(window.location.href).searchParams
  filters.from = params.get('from') || ''
  filters.to = params.get('to') || ''
  filters.batch_id = params.get('batch_id') || ''
  filters.operator = params.get('operator') || ''
  filters.product_id = Number(params.get('product_id') || 0)
}

function materialSummaryText(raw) {
  if (!raw) return ''
  try {
    const items = JSON.parse(raw)
    if (!Array.isArray(items)) return String(raw)
    return items.map((item) => {
      const name = item.material_name || item.name || `物料${item.material_id || ''}`
      const unit = item.unit || ''
      const qty = Number(item.deduct_units || 0) > 0 ? item.deduct_units : item.deduct_g
      return `${name}: ${qty}${unit}`
    }).join('\n')
  } catch {
    return String(raw)
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/produce/logs', window.location.origin)
    if (filters.from) url.searchParams.set('from', filters.from)
    if (filters.to) url.searchParams.set('to', filters.to)
    if (filters.product_id) url.searchParams.set('product_id', String(filters.product_id))
    if (filters.batch_id) url.searchParams.set('batch_id', filters.batch_id)
    if (filters.operator) url.searchParams.set('operator', filters.operator)

    const res = await fetch(url)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    products.value = (data.products || []).map((p) => ({ id: Number(p.id || p.ID || 0), name: p.name || p.Name || '' }))
    rows.value = data.rows || []
    updateUrl()
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
.page { padding: 18px; color: #151515; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.filters { display: grid; grid-template-columns: repeat(5, minmax(140px, 1fr)) 96px; gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.section-title { font-weight: 700; margin-bottom: 10px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1500px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.muted { color: #666; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; border-radius: 6px; padding: 9px; margin-bottom: 12px; color: #8a1f1f; }
.summary { white-space: pre-wrap; font-size: 12px; line-height: 1.45; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters { grid-template-columns: 1fr; }
  table { min-width: 1200px; }
}
</style>
