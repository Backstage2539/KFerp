<template>
  <div class="page">
    <section class="panel no-print">
      <div class="panel-head">
        <h2>生产工单</h2>
        <button class="secondary" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label>
          <span>状态</span>
          <select v-model="status">
            <option value="">全部</option>
            <option value="running">running</option>
            <option value="completed">completed</option>
            <option value="cancelled">cancelled</option>
          </select>
        </label>
        <button class="primary" @click="load">查询</button>
      </div>
    </section>

    <section class="panel table-wrap no-print">
      <table>
        <thead>
          <tr>
            <th>工单</th>
            <th>批次</th>
            <th>商品</th>
            <th>规格</th>
            <th>建议投料</th>
            <th>烘焙建议</th>
            <th>工艺快照</th>
            <th>工序执行</th>
            <th>原料参考</th>
            <th>WIP占用</th>
            <th>状态</th>
            <th>成本</th>
            <th>时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td><strong>{{ row.work_order_no }}</strong><small>{{ row.order_nos || '-' }}</small></td>
            <td>{{ row.batch_id }}</td>
            <td>{{ row.product_name }}</td>
            <td>{{ row.spec_g }}g</td>
            <td>{{ formatG(row.suggested_input_g || row.planned_g) }}</td>
            <td class="advice">
              <strong>{{ row.roast_level || '-' }} · {{ percent(row.yield_rate) }}</strong>
              <small>{{ row.suggested_machine || '未匹配设备' }} · {{ row.suggested_batch_plan || '-' }}</small>
              <small>预计 {{ row.planned_units || 0 }} 袋 + {{ row.planned_loose_g || 0 }}g</small>
            </td>
            <td class="summary">
              <strong>{{ row.process_template_name || '默认工序' }}</strong>
              <small v-if="row.process_template_id">模板 #{{ row.process_template_id }}</small>
            </td>
            <td class="summary">{{ operationSummary(row.operation_summary_json) }}</td>
            <td class="summary">{{ row.material_summary || '-' }}</td>
            <td>
              <strong>{{ formatG(row.remaining_reserved_g) }}</strong>
              <small>已占 {{ formatG(row.wip_reserved_g) }}</small>
              <small>已耗 {{ formatG(row.wip_consumed_g) }}</small>
            </td>
            <td><span class="status">{{ row.status }}</span></td>
            <td>{{ money(row.actual_cost) }}</td>
            <td><small>建 {{ row.created_at }}</small><small>完 {{ row.completed_at || '-' }}</small></td>
            <td><button class="secondary compact" @click="printWorkOrder(row)">打印</button></td>
          </tr>
          <tr v-if="!rows.length"><td colspan="14" class="muted">暂无工单</td></tr>
        </tbody>
      </table>
    </section>

    <section v-if="printRow" class="print-sheet">
      <header class="print-head">
        <div>
          <h1>生产工单</h1>
          <p>{{ printRow.work_order_no }}</p>
        </div>
        <div class="print-status">{{ printRow.status }}</div>
      </header>

      <div class="print-grid">
        <div><span>生产批次</span><strong>{{ printRow.batch_id }}</strong></div>
        <div><span>商品</span><strong>{{ printRow.product_name }}</strong></div>
        <div><span>规格</span><strong>{{ printRow.spec_g }}g</strong></div>
        <div><span>订单</span><strong>{{ printRow.order_nos || '-' }}</strong></div>
        <div><span>建议投料</span><strong>{{ formatG(printRow.suggested_input_g || printRow.planned_g) }}</strong></div>
        <div><span>WIP剩余占用</span><strong>{{ formatG(printRow.remaining_reserved_g) }}</strong></div>
        <div><span>预计产出</span><strong>{{ printRow.planned_units || 0 }} 袋 + {{ printRow.planned_loose_g || 0 }}g</strong></div>
        <div><span>创建时间</span><strong>{{ printRow.created_at }}</strong></div>
        <div><span>完成时间</span><strong>{{ printRow.completed_at || '-' }}</strong></div>
      </div>

      <h2>烘焙建议</h2>
      <table class="print-table">
        <tbody>
          <tr><th>烘焙度</th><td>{{ printRow.roast_level || '-' }}</td></tr>
          <tr><th>计划出成率</th><td>{{ percent(printRow.yield_rate) }}</td></tr>
          <tr><th>建议设备</th><td>{{ printRow.suggested_machine || '未匹配设备' }}</td></tr>
          <tr><th>建议锅次</th><td>{{ printRow.suggested_batch_count || 1 }} 锅，{{ printRow.suggested_batch_plan || '-' }}</td></tr>
          <tr><th>原料参考</th><td>{{ printRow.material_summary || '-' }}</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { apiGet } from '../api/client'

const rows = ref([])
const status = ref('')
const loading = ref(false)
const error = ref('')
const printRow = ref(null)

const money = (v) => Number(v || 0).toFixed(2)
const percent = (v) => {
  const n = Number(v || 0)
  if (!n) return '-'
  return `${(n * 100).toFixed(n * 100 % 1 === 0 ? 0 : 1)}%`
}
const formatG = (v) => `${Number(v || 0).toLocaleString('zh-CN')}g`
function operationSummary(raw) {
  if (!raw) return '-'
  try {
    const items = JSON.parse(raw)
    if (!Array.isArray(items) || !items.length) return '-'
    return items.map((item) => {
      const name = item.operation || '-'
      const status = item.status || '-'
      const loss = Number(item.actual_loss_qty || 0)
      const rate = Number(item.actual_loss_rate || 0)
      const suffix = loss > 0 ? `，损耗 ${loss.toFixed(0)}g / ${(rate * 100).toFixed(2)}%` : ''
      return `${name} ${status}${suffix}`
    }).join('\n')
  } catch {
    return String(raw)
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/produce/work-orders', window.location.origin)
    if (status.value) url.searchParams.set('status', status.value)
    const data = await apiGet(url)
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function clearPrintMode() {
  document.body.classList.remove('work-order-printing')
}

function printWorkOrder(row) {
  printRow.value = { ...row }
  document.body.classList.add('work-order-printing')
  window.setTimeout(() => window.print(), 50)
}

onMounted(() => {
  load()
  window.addEventListener('afterprint', clearPrintMode)
})

onBeforeUnmount(() => {
  window.removeEventListener('afterprint', clearPrintMode)
  clearPrintMode()
})
</script>

<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,button{font:inherit;min-height:36px;border-radius:6px}select{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #9ca3af;background:#fff;color:#111}.compact{min-height:30px;padding:5px 10px}.table-wrap{overflow:auto}table{width:100%;min-width:1500px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.advice strong{display:block}.summary{max-width:240px;line-height:1.45;white-space:pre-wrap}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}.print-sheet{display:none}

@media print{
  :global(body.work-order-printing .sidebar),:global(body.work-order-printing .top){display:none!important}
  :global(body.work-order-printing .content){width:100%!important;margin:0!important;padding:0!important}
  .page{display:block;padding:0}.no-print{display:none!important}.print-sheet{display:block;color:#111;padding:18mm;font-family:system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial}.print-head{display:flex;justify-content:space-between;align-items:flex-start;border-bottom:2px solid #111;padding-bottom:12px;margin-bottom:16px}.print-head h1{font-size:24px;margin:0 0 6px}.print-head p{margin:0;color:#444}.print-status{border:1px solid #111;border-radius:4px;padding:6px 12px}.print-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;margin-bottom:18px}.print-grid div{border:1px solid #ddd;border-radius:6px;padding:8px}.print-grid span{display:block;color:#555;font-size:11px;margin-bottom:4px}.print-grid strong{font-size:13px}.print-sheet h2{font-size:16px;margin:18px 0 8px}.print-table{min-width:0;width:100%;border:1px solid #ddd}.print-table th,.print-table td{border:1px solid #ddd;padding:8px}.print-table th{width:120px;background:#f7f7f7}
}
</style>
