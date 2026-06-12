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
            <option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
        </label>
        <button class="primary" @click="load">查询</button>
      </div>
      <div class="bom-workbench">
        <div class="workbench-head">
          <div>
            <h3>按 BOM 预览生产需求</h3>
            <p>工单以 BOM 为入口，商品档案只作为产出和库存对象。</p>
          </div>
        </div>
        <div class="filters workbench-filters">
          <label>
            <span>生产 BOM</span>
            <select v-model.number="selectedBomID" @change="loadSelectedBomDetail">
              <option :value="0">选择 BOM</option>
              <option v-for="bom in productionBoms" :key="bom.id" :value="Number(bom.id || 0)">
                {{ bom.code }} {{ bom.name }} / {{ bom.output_product_name || '-' }}
              </option>
            </select>
          </label>
          <label>
            <span>生产数量</span>
            <input v-model.number="planQty" type="number" min="0.001" step="0.001" />
          </label>
          <label>
            <span>多层展开策略</span>
            <select v-model="explodeStrategy">
              <option value="shortage">按库存缺口展开</option>
              <option value="first_level">只看第一层</option>
              <option value="full">全部展开</option>
            </select>
          </label>
        </div>
        <div v-if="selectedBomDetail" class="bom-freeze-summary">
          <div><span>冻结 BOM</span><strong>{{ selectedBomDetail.code }} {{ selectedBomDetail.name }} / {{ selectedBomVersion?.version_no || '-' }}</strong></div>
          <div><span>产出商品</span><strong>{{ selectedBomDetail.output_product_name || '-' }}</strong></div>
          <div><span>产出基准</span><strong>{{ formatQty(selectedBomVersion?.output_qty || 1) }} {{ selectedBomVersion?.output_unit || 'unit' }}</strong></div>
        </div>
        <div v-if="selectedBomDetail" class="table-wrap compact-demand">
          <table>
            <thead>
              <tr>
                <th>层级</th>
                <th>组件</th>
                <th>来源</th>
                <th>需求量</th>
                <th>下层 BOM</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in workOrderDemandRows" :key="row.key">
                <td>{{ row.level }}</td>
                <td>{{ row.name }}</td>
                <td>{{ row.component_type === 'product' || row.component_type === 'finished_product' ? '商品组件' : '物料' }}</td>
                <td>{{ formatQty(row.required_qty) }} {{ consumeUnitLabel(row.consume_unit) }}</td>
                <td>{{ row.child_bom_name || '-' }}</td>
              </tr>
              <tr v-if="!workOrderDemandRows.length"><td colspan="5" class="muted">当前 BOM 暂无组件</td></tr>
            </tbody>
          </table>
        </div>
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
            <th>计划数量</th>
            <th>BOM/工艺路线</th>
            <th>工序进度</th>
            <th>工艺参数</th>
            <th>原料参考</th>
            <th>损耗汇总</th>
            <th>领退料/WIP占用</th>
            <th>状态</th>
            <th>成本汇总</th>
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
            <td>
              <strong>{{ formatG(row.planned_g) }}</strong>
              <small>预计 {{ row.planned_units || 0 }} 袋 + {{ row.planned_loose_g || 0 }}g</small>
            </td>
            <td class="summary">
              <strong>{{ bomProcessSummary(row) }}</strong>
              <small>{{ processSnapshotName(row) }}</small>
              <small v-if="processSnapshotSourceText(row)">{{ processSnapshotSourceText(row) }}</small>
            </td>
            <td class="summary">
              <strong>{{ operationProgressText(row) }}</strong>
              <small>工序摘要 {{ operationSummaryText(row) }}</small>
            </td>
            <td class="summary">
              <strong>{{ productionParamsText(row) }}</strong>
              <small>预期产出率 {{ percent(expectedYield(row)) }}</small>
              <small>预期损耗率 {{ percent(expectedLoss(row)) }}</small>
              <small>商品生产配置快照</small>
            </td>
            <td class="summary">{{ row.material_summary || '-' }}</td>
            <td class="summary">
              <strong>{{ formatQty(operationActualSummary(row).actual_loss_qty) }}</strong>
              <small>实际损耗率 {{ percent(operationActualSummary(row).actual_loss_rate) }}</small>
              <small>实际产出 {{ formatQty(operationActualSummary(row).actual_output_qty) }}</small>
            </td>
            <td>
              <strong>可退料 {{ formatG(returnableWipG(row)) }}</strong>
              <small>已领料 {{ formatG(row.wip_reserved_g) }}</small>
              <small>已消耗 {{ formatG(row.wip_consumed_g) }}</small>
            </td>
            <td><span class="status" :class="statusBadgeClass(row.status)">{{ workOrderStatusLabel(row.status) }}</span></td>
            <td>
              <strong>{{ money(row.actual_cost) }}</strong>
              <small>物料 + 工序实际成本</small>
            </td>
            <td><small>建 {{ row.created_at }}</small><small>完 {{ row.completed_at || '-' }}</small></td>
            <td class="row-actions">
              <button class="primary compact" v-if="canStartWorkOrder(row)" @click="startWorkOrder(row)" :disabled="startingId === row.id">开始生产</button>
              <button class="primary compact" v-if="canCompleteWorkOrder(row)" @click="completeWorkOrder(row)" :disabled="completingId === row.id">完工入库</button>
              <button class="secondary compact" @click="printWorkOrder(row)">打印</button>
            </td>
          </tr>
          <tr v-if="!rows.length"><td colspan="15" class="muted">暂无工单</td></tr>
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
        <div><span>计划投料</span><strong>{{ formatG(printRow.planned_g) }}</strong></div>
        <div><span>预期损耗率</span><strong>{{ percent(expectedLoss(printRow)) }}</strong></div>
        <div><span>预期产出率</span><strong>{{ percent(expectedYield(printRow)) }}</strong></div>
        <div><span>BOM/工艺路线</span><strong>{{ bomProcessSummary(printRow) }}</strong></div>
        <div><span>WIP剩余占用</span><strong>{{ formatG(printRow.remaining_reserved_g) }}</strong></div>
        <div><span>预计产出</span><strong>{{ printRow.planned_units || 0 }} 袋 + {{ printRow.planned_loose_g || 0 }}g</strong></div>
        <div><span>实际损耗</span><strong>{{ formatQty(operationActualSummary(printRow).actual_loss_qty) }}</strong></div>
        <div><span>创建时间</span><strong>{{ printRow.created_at }}</strong></div>
        <div><span>完成时间</span><strong>{{ printRow.completed_at || '-' }}</strong></div>
      </div>

      <h2>工艺路线与参数</h2>
      <table class="print-table">
        <tbody>
          <tr><th>工艺参数</th><td>{{ productionParamsText(printRow) }}</td></tr>
          <tr><th>商品生产配置快照</th><td>{{ productConfigSnapshotText(printRow) }}</td></tr>
          <tr><th>工序摘要</th><td>{{ operationSummaryText(printRow) }}</td></tr>
          <tr><th>预期产出率</th><td>{{ percent(expectedYield(printRow)) }}</td></tr>
          <tr><th>预期损耗率</th><td>{{ percent(expectedLoss(printRow)) }}</td></tr>
          <tr><th>原料参考</th><td>{{ printRow.material_summary || '-' }}</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { expectedLossRate, formatPercent } from '../lib/manufacturing-loss'
import { canCompleteWorkOrder, workOrderCompleteEndpoint, workOrderStatusLabel } from '../lib/manufacturing-execution'
import { canStartWorkOrder, workOrderStartEndpoint, workOrderStatusOptions } from '../lib/work-orders'

const rows = ref([])
const productionBoms = ref([])
const productionBomDetails = ref({})
const selectedBomID = ref(0)
const planQty = ref(1)
const explodeStrategy = ref('shortage')
const status = ref('')
const loading = ref(false)
const startingId = ref(0)
const completingId = ref(0)
const error = ref('')
const printRow = ref(null)
const statusOptions = workOrderStatusOptions()

const money = (v) => Number(v || 0).toFixed(2)
const percent = (v) => formatPercent(v)
const formatG = (v) => `${Number(v || 0).toLocaleString('zh-CN')}g`
const formatQty = (v) => {
  const n = Number(v || 0)
  return n ? `${n.toLocaleString('zh-CN', { maximumFractionDigits: 3 })}` : '-'
}
const expectedYield = (row) => Number(row?.expected_yield_rate || row?.yield_rate || 0)
const expectedLoss = (row) => {
  if (row && Object.prototype.hasOwnProperty.call(row, 'expected_loss_rate')) return Number(row.expected_loss_rate || 0)
  return expectedLossRate(expectedYield(row))
}
const selectedBomDetail = computed(() => productionBomDetails.value[String(selectedBomID.value)] || null)
const selectedBomVersion = computed(() => {
  const detail = selectedBomDetail.value
  if (!detail) return null
  return (detail.versions || []).find((version) => Number(version.id || 0) === Number(detail.latest_version_id || 0)) || (detail.versions || [])[0] || null
})
const bomByOutputProductID = computed(() => {
  const map = new Map()
  for (const row of productionBoms.value) {
    const productID = Number(row.output_product_id || 0)
    if (productID > 0 && !map.has(productID)) map.set(productID, row)
  }
  return map
})
const workOrderDemandRows = computed(() => buildDemandRows(selectedBomDetail.value, Number(planQty.value || 0), explodeStrategy.value))

function processSnapshot(row) {
  if (!row?.process_snapshot_json) return {}
  try {
    const parsed = JSON.parse(row.process_snapshot_json)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function processSnapshotName(row) {
  const snapshot = processSnapshot(row)
  return snapshot.route_name || snapshot.name || row?.process_template_name || '默认工序'
}

function processSnapshotSourceText(row) {
  const snapshot = processSnapshot(row)
  if (snapshot.source === 'process_route' && Number(snapshot.route_id || row?.process_template_id || 0) > 0) {
    return `工艺路线 #${Number(snapshot.route_id || row.process_template_id || 0)}`
  }
  if (Number(row?.process_template_id || 0) > 0) {
    return `工艺模板 #${Number(row.process_template_id || 0)}`
  }
  return ''
}

function bomProcessSummary(row) {
  const parts = []
  const bomVersionID = Number(row?.bom_version_id || 0)
  if (bomVersionID > 0) parts.push(`BOM版本 #${bomVersionID}`)
  else parts.push('默认BOM')
  const snapshotText = processSnapshotSourceText(row)
  if (snapshotText) parts.push(snapshotText)
  else if (Number(row?.process_template_id || 0) > 0) parts.push(`工艺模板 #${Number(row.process_template_id || 0)}`)
  else parts.push('默认工艺路线')
  return parts.join(' / ')
}

function productionParamsText(row) {
  const value = String(row?.roast_level || '').trim()
  return value || '按商品生产配置'
}

function productConfigSnapshotText(row) {
  const parts = []
  const bomVersionID = Number(row?.bom_version_id || 0)
  if (bomVersionID > 0) parts.push(`BOM版本 #${bomVersionID}`)
  if (Number(row?.process_template_id || 0) > 0) parts.push(`工艺模板 #${Number(row.process_template_id || 0)}`)
  if (productionParamsText(row) !== '按商品生产配置') parts.push(`工艺参数：${productionParamsText(row)}`)
  return parts.join('；') || '按商品生产配置'
}

function operationSummaryRows(row) {
  if (row?.operation_summary_json) {
    try {
      const parsed = JSON.parse(row.operation_summary_json)
      if (Array.isArray(parsed)) return parsed
      if (parsed && typeof parsed === 'object') return [parsed]
    } catch {
      return []
    }
  }
  const snapshot = processSnapshot(row)
  if (!Array.isArray(snapshot.operations)) return []
  return snapshot.operations.map((item) => ({
    ...item,
    operation: item.operation || item.operation_name || item.name || '',
    workstation: item.workstation || item.workstation_name || '',
    status: item.status || 'frozen',
  }))
}

function operationActualSummary(row) {
  const items = operationSummaryRows(row)
  const summary = { actual_input_qty: 0, actual_output_qty: 0, actual_loss_qty: 0, actual_loss_rate: 0 }
  for (const item of items) {
    summary.actual_input_qty += Number(item.actual_input_qty || 0)
    summary.actual_output_qty += Number(item.actual_output_qty || 0)
    summary.actual_loss_qty += Number(item.actual_loss_qty || 0)
  }
  if (summary.actual_input_qty > 0) summary.actual_loss_rate = summary.actual_loss_qty / summary.actual_input_qty
  return summary
}

function operationProgressText(row) {
  const items = operationSummaryRows(row)
  if (!items.length) return '0/0'
  const completed = items.filter((item) => String(item.status || '').trim() === 'completed').length
  return `${completed}/${items.length}`
}

function operationSummaryText(row) {
  const items = operationSummaryRows(row)
  if (!items.length) return '-'
  return items.map((item) => {
    const parts = [
      item.operation || '-',
      item.workstation || item.workstation_name || '',
      item.status || '-',
    ].filter(Boolean)
    return parts.join(' ')
  }).join(' / ')
}

function returnableWipG(row) {
  const remaining = Number(row?.remaining_reserved_g || 0)
  if (remaining > 0) return remaining
  return Math.max(0, Number(row?.wip_reserved_g || 0) - Number(row?.wip_consumed_g || 0))
}

function statusBadgeClass(statusValue) {
  return {
    draft: 'neutral',
    released: 'info',
    running: 'warning',
    partially_completed: 'warning',
    completed: 'success',
    cancelled: 'danger',
  }[String(statusValue || '').trim()] || 'neutral'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/produce/work-orders', window.location.origin)
    if (status.value) url.searchParams.set('status', status.value)
    const [data, bomData] = await Promise.all([
      apiGet(url),
      apiGet('/api/production-boms?status=all'),
    ])
    rows.value = data.rows || []
    productionBoms.value = bomData.rows || bomData || []
    if (!selectedBomID.value && productionBoms.value.length) selectedBomID.value = Number(productionBoms.value[0].id || 0)
    if (selectedBomID.value) await loadSelectedBomDetail()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function startWorkOrder(row) {
  const endpoint = workOrderStartEndpoint(row)
  if (!endpoint) return
  startingId.value = Number(row.id || 0)
  error.value = ''
  try {
    await apiSend(endpoint, { body: {} })
    status.value = 'running'
    await load()
  } catch (err) {
    error.value = err.message || '开始生产失败'
  } finally {
    startingId.value = 0
  }
}

async function completeWorkOrder(row) {
  const endpoint = workOrderCompleteEndpoint(row)
  if (!endpoint) return
  completingId.value = Number(row.id || 0)
  error.value = ''
  try {
    await apiSend(endpoint, {
      body: {
        finished_units: Number(row.planned_units || 0),
        finished_loose_g: Number(row.planned_loose_g || 0),
        consumed_input_g: Number(row.wip_consumed_g || row.planned_g || 0),
        warehouse: 'finished_goods',
        note: '生产工单页完工入库',
      },
    })
    status.value = 'completed'
    await load()
  } catch (err) {
    error.value = err.message || '完工入库失败'
  } finally {
    completingId.value = 0
  }
}

async function loadSelectedBomDetail() {
  const id = Number(selectedBomID.value || 0)
  if (!id || productionBomDetails.value[String(id)]) return
  const detail = await apiGet(`/api/production-boms/${id}`)
  productionBomDetails.value = { ...productionBomDetails.value, [String(id)]: detail }
}

function buildDemandRows(detail, qty, strategy, level = 1, seen = new Set()) {
  if (!detail || qty <= 0) return []
  const version = (detail.versions || []).find((row) => Number(row.id || 0) === Number(detail.latest_version_id || 0)) || (detail.versions || [])[0] || {}
  const outputQty = Number(version.output_qty || 1) || 1
  const multiplier = qty / outputQty
  const rows = []
  for (const [index, item] of (detail.items || []).entries()) {
    const isProduct = item.component_type === 'product' || item.component_type === 'finished_product'
    const requiredQty = item.consume_unit === 'ratio_pct' ? Number(item.ratio_pct || 0) * multiplier : Number(item.qty_per_unit || 0) * multiplier
    const childBom = isProduct ? bomByOutputProductID.value.get(Number(item.component_product_id || 0)) : null
    rows.push({
      key: `${level}:${index}:${item.id || item.component_product_id || item.material_id}`,
      level,
      component_type: item.component_type,
      name: isProduct ? (item.component_product_name || `商品 #${item.component_product_id}`) : (item.material_name || `物料 #${item.material_id}`),
      consume_unit: item.consume_unit,
      required_qty: requiredQty,
      child_bom_name: childBom ? `${childBom.code || ''} ${childBom.name || ''}`.trim() : '',
    })
    if (strategy === 'full' && childBom && !seen.has(Number(childBom.id || 0)) && productionBomDetails.value[String(childBom.id || 0)]) {
      const nextSeen = new Set(seen)
      nextSeen.add(Number(childBom.id || 0))
      rows.push(...buildDemandRows(productionBomDetails.value[String(childBom.id || 0)], requiredQty, strategy, level + 1, nextSeen))
    }
  }
  return rows
}

function consumeUnitLabel(unit) {
  return ({
    ratio_pct: '%',
    g_per_bag: '克/袋',
    unit_per_bag: '个/袋',
    unit_per_box: '个/盒',
    fixed_qty: '固定数量',
    unit: '个',
    kg: 'kg',
    g: 'g',
  })[String(unit || '')] || unit || '-'
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
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}h3{margin:0;font-size:16px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,input,button{font:inherit;min-height:36px;border-radius:6px}select,input{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #9ca3af;background:#fff;color:#111}.compact{min-height:30px;padding:5px 10px}.row-actions{display:flex;gap:6px;flex-wrap:wrap}.table-wrap{overflow:auto}table{width:100%;min-width:1260px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.advice strong{display:block}.summary{max-width:220px;line-height:1.45}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.status.info{border-color:#93c5fd;background:#eff6ff;color:#1d4ed8}.status.warning{border-color:#fed7aa;background:#fff7ed;color:#c2410c}.status.success{border-color:#bbf7d0;background:#f0fdf4;color:#15803d}.status.danger{border-color:#fecaca;background:#fef2f2;color:#b91c1c}.status.neutral{border-color:#d1d5db;background:#f9fafb;color:#374151}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}.print-sheet{display:none}.bom-workbench{margin-top:14px;padding-top:12px;border-top:1px solid #e5e7eb;display:grid;gap:10px}.workbench-head p{margin:4px 0 0;color:#666;font-size:12px}.workbench-filters{grid-template-columns:minmax(260px,1.2fr) minmax(120px,.4fr) minmax(180px,.7fr)}.bom-freeze-summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px}.bom-freeze-summary div{border:1px solid #e5e7eb;border-radius:6px;padding:8px;background:#fbfbfb}.bom-freeze-summary span{display:block;color:#666;font-size:12px;margin-bottom:3px}.compact-demand table{min-width:760px}

@media print{
  :global(body.work-order-printing .sidebar),:global(body.work-order-printing .top){display:none!important}
  :global(body.work-order-printing .content){width:100%!important;margin:0!important;padding:0!important}
  .page{display:block;padding:0}.no-print{display:none!important}.print-sheet{display:block;color:#111;padding:18mm;font-family:system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial}.print-head{display:flex;justify-content:space-between;align-items:flex-start;border-bottom:2px solid #111;padding-bottom:12px;margin-bottom:16px}.print-head h1{font-size:24px;margin:0 0 6px}.print-head p{margin:0;color:#444}.print-status{border:1px solid #111;border-radius:4px;padding:6px 12px}.print-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;margin-bottom:18px}.print-grid div{border:1px solid #ddd;border-radius:6px;padding:8px}.print-grid span{display:block;color:#555;font-size:11px;margin-bottom:4px}.print-grid strong{font-size:13px}.print-sheet h2{font-size:16px;margin:18px 0 8px}.print-table{min-width:0;width:100%;border:1px solid #ddd}.print-table th,.print-table td{border:1px solid #ddd;padding:8px}.print-table th{width:120px;background:#f7f7f7}
}
</style>
