<template>
  <div class="page">
    <ProductionTopNav active-key="productionCosts" />

    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>生产成本</h2>
          <p>按工单查看成本拆解、追溯链路和异常损耗。</p>
        </div>
        <button class="secondary" @click="load" :disabled="loading">刷新</button>
      </div>
      <div class="filters">
        <label><span>工单 ID</span><input v-model.number="filters.work_order_id" type="number" min="0" /></label>
        <label><span>生产批次</span><input v-model.trim="filters.batch_id" placeholder="BATCH-WO-88" /></label>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
    </section>

    <section class="panel summary-grid">
      <div><span>成本差异</span><strong>{{ money(traceAnalytics.total_variance) }}</strong></div>
      <div><span>异常损耗</span><strong>{{ traceAnalytics.abnormal_loss_count || 0 }}</strong></div>
      <div><span>追溯链路</span><strong>{{ traceLinks.length }}</strong></div>
    </section>

    <section class="panel table-wrap">
      <div class="section-title">成本记录</div>
      <table>
        <thead><tr><th>时间</th><th>生产批次</th><th>商品</th><th>物料成本</th><th>工序成本</th><th>总成本</th><th>成品(g)</th><th>元/kg</th></tr></thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id"><td>{{ row.created_at }}</td><td>{{ row.batch_id }}</td><td>{{ row.product_name }}</td><td>{{ money(row.material_cost) }}</td><td>{{ money(row.operation_cost) }}</td><td>{{ money(row.total_cost) }}</td><td>{{ row.finished_g }}</td><td>{{ money(row.unit_cost_per_kg) }}</td></tr>
          <tr v-if="!rows.length"><td colspan="8" class="muted">暂无成本记录</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel table-wrap">
      <div class="section-title">成本差异</div>
      <table>
        <thead><tr><th>工单</th><th>商品</th><th>生产批次</th><th>计划成本</th><th>实际成本</th><th>计划工序成本</th><th>实际工序成本</th><th>差异</th><th>差异率</th></tr></thead>
        <tbody>
          <tr v-for="row in costVarianceRows" :key="`${row.work_order_id}-${row.batch_id}`"><td>{{ row.work_order_no || row.work_order_id }}</td><td>{{ row.product_name || '-' }}</td><td>{{ row.batch_id || '-' }}</td><td>{{ money(row.planned_cost) }}</td><td>{{ money(row.actual_cost) }}</td><td>{{ money(row.planned_operation_cost) }}</td><td>{{ money(row.actual_operation_cost) }}</td><td>{{ money(row.variance) }}</td><td>{{ percent(row.variance_rate) }}</td></tr>
          <tr v-if="!costVarianceRows.length"><td colspan="9" class="muted">暂无成本差异</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel table-wrap">
      <div class="section-title">异常损耗</div>
      <table>
        <thead><tr><th>工单</th><th>工序</th><th>投入</th><th>产出</th><th>损耗</th><th>损耗率</th><th>原因</th><th>等级</th></tr></thead>
        <tbody>
          <tr v-for="row in abnormalLossRows" :key="row.job_card_id"><td>{{ row.work_order_no || row.work_order_id }}</td><td>{{ row.operation || '-' }}</td><td>{{ row.actual_input_qty || 0 }}</td><td>{{ row.actual_output_qty || 0 }}</td><td>{{ row.actual_loss_qty || 0 }}</td><td>{{ percent(row.actual_loss_rate) }}</td><td>{{ row.loss_reason || row.exception_reason || '-' }}</td><td><span :class="['pill', row.severity]">{{ severityLabel(row.severity) }}</span></td></tr>
          <tr v-if="!abnormalLossRows.length"><td colspan="8" class="muted">暂无异常损耗</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel table-wrap">
      <div class="section-title">追溯链路</div>
      <table>
        <thead><tr><th>工单</th><th>工序卡</th><th>Stock Entry</th><th>类型</th><th>物料/成品</th><th>批次</th><th>数量(g)</th><th>时间</th></tr></thead>
        <tbody>
          <tr v-for="row in traceLinks" :key="`${row.work_order_id}-${row.job_card_id}-${row.stock_entry_id}-${row.batch_code}`"><td>{{ row.work_order_no || row.work_order_id }}</td><td>{{ row.operation || row.job_card_id || '-' }}</td><td>{{ row.entry_no || '-' }}</td><td>{{ row.entry_type || '-' }}</td><td>{{ row.material_name || row.material_id || '-' }}</td><td>{{ row.batch_code || row.batch_id || '-' }}</td><td>{{ row.qty_g || 0 }}</td><td>{{ row.created_at || '-' }}</td></tr>
          <tr v-if="!traceLinks.length"><td colspan="8" class="muted">暂无追溯链路</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet } from '../api/client'
import ProductionTopNav from '../components/ProductionTopNav.vue'

const phase3TraceAnalyticsMarkers = ['trace_links', 'cost_variance', 'abnormal_losses']
const rows = ref([])
const traceAnalytics = ref({ trace_links: [], cost_variance: [], abnormal_losses: [], total_variance: 0, abnormal_loss_count: 0 })
const filters = reactive({ work_order_id: 0, batch_id: '' })
const loading = ref(false)
const error = ref('')
const traceLinks = computed(() => traceAnalytics.value.trace_links || [])
const costVarianceRows = computed(() => traceAnalytics.value.cost_variance || [])
const abnormalLossRows = computed(() => traceAnalytics.value.abnormal_losses || [])
const money = (v) => Number(v || 0).toFixed(2)
const percent = (v) => `${(Number(v || 0) * 100).toFixed(1)}%`
function severityLabel(v) {
  return ({ error: '严重', warning: '提醒', info: '记录' })[String(v || '').trim()] || '记录'
}
function traceEndpoint() {
  const params = new URLSearchParams()
  if (Number(filters.work_order_id || 0) > 0) params.set('work_order_id', String(Number(filters.work_order_id || 0)))
  if (String(filters.batch_id || '').trim()) params.set('batch_id', String(filters.batch_id || '').trim())
  params.set('limit', '50')
  return `/api/production-trace/analytics?${params.toString()}`
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const [costData, traceData] = await Promise.all([
      apiGet('/api/produce/costs'),
      apiGet(traceEndpoint()),
    ])
    rows.value = costData.rows || []
    traceAnalytics.value = traceData || { trace_links: [], cost_variance: [], abnormal_losses: [] }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>

<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #eee;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}h2{margin:0 0 4px;font-size:18px}.panel-head p{margin:0;color:#6b7280;font-size:13px}.filters{display:grid;grid-template-columns:160px minmax(220px,1fr);gap:10px;margin-top:12px;max-width:560px}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}input{width:100%;border:1px solid #d1d5db;border-radius:6px;padding:7px 9px;font:inherit}.summary-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.summary-grid div{border:1px solid #e5e7eb;border-radius:8px;padding:10px}.summary-grid span{display:block;color:#6b7280;font-size:12px;margin-bottom:4px}.summary-grid strong{font-size:18px}.section-title{font-weight:700;margin-bottom:10px}button{font:inherit;min-height:36px;border-radius:6px;padding:8px 12px;cursor:pointer}.secondary{border:1px solid #999;background:#fff;color:#111}.table-wrap{overflow:auto}table{width:100%;min-width:860px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px}th{background:#fbfbfb}.pill{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.pill.warning{border-color:#fde68a;background:#fffbeb;color:#92400e}.pill.error{border-color:#fecaca;background:#fef2f2;color:#991b1b}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}@media (max-width:760px){.panel-head{display:grid}.filters,.summary-grid{grid-template-columns:1fr}}
</style>
