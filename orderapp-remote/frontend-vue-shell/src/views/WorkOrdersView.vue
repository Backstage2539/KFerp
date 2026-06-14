<template>
  <div class="page">
    <ProductionTopNav active-key="workOrders" />

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
              <small>计划 {{ operationPlanText(row) }}</small>
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
              <button class="secondary compact" v-if="canEditWorkOrderSplits(row)" @click="openWorkOrderSplitDrawer(row)">编辑拆分</button>
              <button class="primary compact" v-if="canStartWorkOrder(row)" @click="startWorkOrder(row)" :disabled="startingId === row.id">开始生产</button>
              <button class="primary compact" v-if="canCompleteWorkOrder(row)" @click="completeWorkOrder(row)" :disabled="completingId === row.id">完工入库</button>
              <button class="secondary compact" @click="printWorkOrder(row)">打印</button>
            </td>
          </tr>
          <tr v-if="!rows.length"><td colspan="15" class="muted">暂无工单</td></tr>
        </tbody>
      </table>
    </section>

    <div v-if="workOrderSplitRow" class="drawer-backdrop no-print" @click.self="closeWorkOrderSplitDrawer">
      <aside class="work-order-split-drawer" aria-label="工单工序产能拆分">
        <div class="drawer-head">
          <div>
            <div class="muted text-left">工单产能拆分</div>
            <h2>{{ workOrderSplitRow.work_order_no || '-' }}</h2>
            <p>{{ workOrderSplitRow.product_name || '-' }} / {{ workOrderSplitRow.spec_g || 0 }}g / 计划 {{ formatG(workOrderSplitRow.planned_g) }}</p>
          </div>
          <button class="secondary compact" type="button" @click="closeWorkOrderSplitDrawer">关闭</button>
        </div>
        <div v-if="workOrderSplitError" class="error">{{ workOrderSplitError }}</div>
        <div v-for="operation in workOrderSplitOperations" :key="`${operation.seq || operation.sequence_no || operation.operation}-${operation.operation_id || 0}`" class="split-operation-block">
          <div class="split-operation-head">
            <strong>{{ operation.seq || operation.sequence_no || '-' }}. {{ operation.operation || '工序' }}</strong>
            <span>{{ operation.workstation || '工位待定' }}</span>
            <button class="secondary compact" type="button" @click="autoSplitWorkOrderOperation(operation)" :disabled="!applicableOperationCapacities(operation, activeWorkstationCapacities).length">自动拆分</button>
            <button class="secondary compact" type="button" @click="addWorkOrderOperationSplit(operation)">添加拆分</button>
          </div>
          <div class="split-row" v-for="(split, splitIndex) in workOrderSplitRowsForOperation(operation)" :key="split.local_key || split.id || `${operation.seq}-${splitIndex}`">
            <label>
              <span>工位产能</span>
              <select v-model.number="split.workstation_capacity_id" @change="applyWorkOrderSplitCapacity(split)">
                <option value="0">选择工位产能，例如 布勒 18kg / 智烘 4kg</option>
                <option v-for="capacity in activeWorkstationCapacities" :key="capacity.id" :value="capacity.id">{{ capacityOptionLabel(capacity) }}</option>
              </select>
            </label>
            <label>
              <span>承担产量{{ splitQuantityUnit(split) }}</span>
              <input v-model.number="split.planned_qty" type="number" min="0" :step="splitQuantityStep(split)" />
            </label>
            <button class="secondary compact" type="button" @click="assignRemainingWorkOrderSplitQty(split)" :disabled="!split.workstation_capacity_id">分配剩余产量</button>
            <div class="split-metric">
              <span>自动批次数</span>
              <strong>{{ plannedCapacitySplitMetrics(split).planned_batch_count || 0 }}</strong>
            </div>
            <div class="split-metric">
              <span>计划分钟</span>
              <strong>{{ plannedCapacitySplitMetrics(split).planned_minutes || 0 }}</strong>
            </div>
            <div class="split-metric">
              <span>计划工序成本</span>
              <strong>{{ plannedCapacitySplitMetrics(split).planned_operation_cost || 0 }}</strong>
            </div>
            <button class="secondary compact danger-text" type="button" @click="removeWorkOrderOperationSplit(split)">删除</button>
            <div v-if="splitBatchCards(split).length" class="split-batch-cards" aria-label="自动批次卡片">
              <div
                v-for="batch in splitBatchCards(split)"
                :key="`${split.local_key || split.id || splitIndex}-${batch.label}`"
                class="split-batch-card"
                :class="{ underfilled: batch.underfilled }"
              >
                <strong>{{ batch.label }}</strong>
                <span>{{ batch.workstation_capacity_name || split.workstation_capacity_name || '工位产能' }}</span>
                <small>单批标准 {{ splitQtyText(batch.batch_size_qty, batch.batch_size_unit) }}</small>
                <small>本批计划 {{ splitQtyText(batch.planned_qty, batch.batch_size_unit) }}</small>
                <small>计划分钟 {{ batch.planned_minutes || 0 }}</small>
                <em v-if="batch.underfilled">不足标准批量</em>
              </div>
            </div>
          </div>
          <div v-if="!workOrderSplitRowsForOperation(operation).length" class="muted section-hint">暂无拆分，保存空拆分会恢复为按工序生成一张工序卡。</div>
        </div>
        <div v-if="!workOrderSplitOperations.length" class="muted section-hint">暂无工序快照</div>
        <div class="drawer-actions">
          <button class="primary" type="button" @click="saveWorkOrderOperationSplits" :disabled="workOrderSplitSaving">保存拆分</button>
          <button class="secondary" type="button" @click="closeWorkOrderSplitDrawer">取消</button>
        </div>
      </aside>
    </div>

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
          <tr><th>工序计划</th><td>{{ operationPlanText(printRow) }}</td></tr>
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
import ProductionTopNav from '../components/ProductionTopNav.vue'
import { expectedLossRate, formatPercent } from '../lib/manufacturing-loss'
import { canCompleteWorkOrder, workOrderCompleteEndpoint, workOrderStatusLabel } from '../lib/manufacturing-execution'
import {
  applicableOperationCapacities,
  buildOperationCapacityAutoSplits,
  maxAssignableQtyForCapacitySplit,
  plannedCapacitySplitMetrics,
  productionPlanSplitBatchCards,
  qtyFromGForCapacityUnit,
} from '../lib/produce-plan'
import {
  buildWorkOrderOperationSplitPayload,
  canEditWorkOrderSplits,
  canStartWorkOrder,
  workOrderOperationSplitsEndpoint,
  workOrderStartEndpoint,
  workOrderStatusOptions,
} from '../lib/work-orders'

const rows = ref([])
const workstationCapacities = ref([])
const status = ref('')
const loading = ref(false)
const startingId = ref(0)
const completingId = ref(0)
const error = ref('')
const printRow = ref(null)
const workOrderSplitRow = ref(null)
const workOrderSplitRows = ref([])
const workOrderSplitSaving = ref(false)
const workOrderSplitError = ref('')
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
const activeWorkstationCapacities = computed(() => workstationCapacities.value.filter((row) => String(row.status || 'active') === 'active'))
const workOrderSplitOperations = computed(() => {
  const row = workOrderSplitRow.value
  if (!row) return []
  const snapshot = processSnapshot(row)
  if (Array.isArray(snapshot.operations) && snapshot.operations.length) {
    return snapshot.operations.map(normalizeSplitOperation)
  }
  return operationSummaryRows(row).map(normalizeSplitOperation)
})

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
    workstation_capacity_name: item.workstation_capacity_name || item.capacity_name || '',
    planned_minutes: Number(item.planned_minutes || 0),
    planned_operation_cost: Number(item.planned_operation_cost || 0),
    status: item.status || 'frozen',
  }))
}

function normalizeSplitOperation(item = {}) {
  return {
    seq: Number(item.seq || item.sequence_no || 0),
    sequence_no: Number(item.sequence_no || item.seq || 0),
    operation_id: Number(item.operation_id || 0),
    operation: item.operation || item.operation_name || item.name || '',
    workstation: item.workstation || item.workstation_name || '',
  }
}

function operationIdentity(operation = {}) {
  return {
    seq: Number(operation.seq || operation.sequence_no || 0),
    id: Number(operation.operation_id || 0),
    name: String(operation.operation || '').trim(),
  }
}

function splitMatchesOperation(split, operation) {
  const identity = operationIdentity(operation)
  if (identity.seq > 0 && Number(split?.operation_seq || 0) === identity.seq) return true
  if (identity.id > 0 && Number(split?.operation_id || 0) === identity.id) return true
  return !identity.seq && identity.name && String(split?.operation || '').trim() === identity.name
}

function workOrderSplitRowsForOperation(operation) {
  return workOrderSplitRows.value.filter((split) => splitMatchesOperation(split, operation))
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
      item.workstation_capacity_name ? `工位产能 ${item.workstation_capacity_name}` : '',
      item.status || '-',
    ].filter(Boolean)
    return parts.join(' ')
  }).join(' / ')
}

function operationPlanText(row) {
  const items = operationSummaryRows(row)
  if (!items.length) return '-'
  const plannedMinutes = items.reduce((sum, item) => sum + Number(item.planned_minutes || 0), 0)
  const plannedOperationCost = items.reduce((sum, item) => sum + Number(item.planned_operation_cost || 0), 0)
  const capacityNames = items.map((item) => String(item.workstation_capacity_name || '').trim()).filter(Boolean)
  const capacityText = capacityNames.length ? `工位产能 ${capacityNames.join(' / ')}` : '工位产能 -'
  return `${capacityText} · 计划分钟 ${plannedMinutes || 0} · 计划工序成本 ${money(plannedOperationCost)}`
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
    const [data, capacityData] = await Promise.all([
      apiGet(url),
      apiGet('/api/manufacturing-workstation-capacities'),
    ])
    rows.value = data.rows || []
    workstationCapacities.value = capacityData.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function splitQuantityUnit(split) {
  const unit = String(split?.batch_size_unit || '').trim()
  return unit ? `（${unit}）` : ''
}

function splitQuantityStep(split) {
  const unit = String(split?.batch_size_unit || '').trim().toLowerCase()
  if (unit === 'g' || unit === '克') return '1'
  return '0.001'
}

function splitQtyText(qty, unit) {
  const n = Math.max(0, Number(qty || 0))
  const value = n ? n.toLocaleString('zh-CN', { maximumFractionDigits: 3 }) : '0'
  return `${value}${String(unit || '').trim()}`
}

function splitBatchCards(split) {
  return productionPlanSplitBatchCards(split)
}

function qtyFromGForSplitUnit(qtyG, unit) {
  return qtyFromGForCapacityUnit(qtyG, unit, workOrderSplitRow.value?.spec_g || 0)
}

function capacityOptionLabel(capacity) {
  const parts = [capacity?.name || `#${capacity?.id || ''}`]
  if (Number(capacity?.batch_size_qty || 0) > 0) parts.push(`${capacity.batch_size_qty}${capacity.batch_size_unit || ''}`)
  if (Number(capacity?.standard_minutes || 0) > 0) parts.push(`${capacity.standard_minutes}分钟/批`)
  if (Number(capacity?.hourly_rate || 0) > 0) parts.push(`${capacity.hourly_rate}/小时`)
  return parts.filter(Boolean).join(' · ')
}

function splitSameOperation(left, right) {
  const leftSeq = Number(left?.operation_seq || 0)
  const rightSeq = Number(right?.operation_seq || 0)
  if (leftSeq > 0 || rightSeq > 0) return leftSeq === rightSeq
  const leftID = Number(left?.operation_id || 0)
  const rightID = Number(right?.operation_id || 0)
  if (leftID > 0 || rightID > 0) return leftID === rightID
  return String(left?.operation || '').trim() === String(right?.operation || '').trim()
}

function defaultPlannedQtyForWorkOrderSplit(split) {
  return maxAssignableQtyForCapacitySplit(split, workOrderSplitRows.value, {
    planned_g: Math.max(0, Number(workOrderSplitRow.value?.planned_g || 0)),
    spec_g: Number(workOrderSplitRow.value?.spec_g || 0),
  })
}

function normalizeWorkOrderSplit(row = {}) {
  return {
    local_key: row.local_key || `wo-split-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    id: Number(row.id || 0),
    operation_seq: Number(row.operation_seq || row.sequence_no || 0),
    operation_id: Number(row.operation_id || 0),
    operation: row.operation || '',
    workstation_id: Number(row.workstation_id || 0),
    workstation: row.workstation || '',
    workstation_capacity_id: Number(row.workstation_capacity_id || 0),
    workstation_capacity_name: row.workstation_capacity_name || '',
    batch_size_qty: Number(row.batch_size_qty || 0),
    batch_size_unit: row.batch_size_unit || '',
    standard_minutes: Number(row.standard_minutes || 0),
    hourly_rate: Number(row.hourly_rate || 0),
    planned_batch_count: Number(row.planned_batch_count || 0),
    spec_g: Number(row.spec_g || workOrderSplitRow.value?.spec_g || 0),
    planned_qty: Number(row.planned_qty || qtyFromGForSplitUnit(Number(row.planned_input_qty || row.planned_qty_g || 0), row.batch_size_unit)),
    planned_qty_g: Number(row.planned_qty_g || row.planned_input_qty || 0),
    planned_minutes: Number(row.planned_minutes || 0),
    planned_operation_cost: Number(row.planned_operation_cost || 0),
    note: row.note || '',
  }
}

function addWorkOrderOperationSplit(operation) {
  const identity = operationIdentity(operation)
  workOrderSplitRows.value.push(normalizeWorkOrderSplit({
    operation_seq: identity.seq,
    operation_id: identity.id,
    operation: identity.name,
    spec_g: Number(workOrderSplitRow.value?.spec_g || 0),
    planned_qty: 0,
  }))
}

function removeWorkOrderOperationSplit(split) {
  workOrderSplitRows.value = workOrderSplitRows.value.filter((row) => row !== split)
}

function applyWorkOrderSplitCapacity(split) {
  const capacity = workstationCapacities.value.find((row) => Number(row.id || 0) === Number(split.workstation_capacity_id || 0))
  if (!capacity) return
  split.workstation_id = Number(capacity.workstation_id || 0)
  split.workstation = capacity.workstation || ''
  split.workstation_capacity_name = capacity.name || ''
  split.batch_size_qty = Number(capacity.batch_size_qty || 0)
  split.batch_size_unit = capacity.batch_size_unit || ''
  split.standard_minutes = Number(capacity.standard_minutes || 0)
  split.hourly_rate = Number(capacity.hourly_rate || 0)
  split.spec_g = Number(split.spec_g || workOrderSplitRow.value?.spec_g || 0)
  if (Number(split.planned_qty || 0) <= 0) {
    split.planned_qty = defaultPlannedQtyForWorkOrderSplit(split)
  }
}

function autoSplitWorkOrderOperation(operation) {
  if (!workOrderSplitRow.value) return
  const autoRows = buildOperationCapacityAutoSplits({
    id: Number(workOrderSplitRow.value.production_plan_item_id || 0),
    planned_g: Number(workOrderSplitRow.value.planned_g || 0),
    spec_g: Number(workOrderSplitRow.value.spec_g || 0),
  }, operation, activeWorkstationCapacities.value).map(normalizeWorkOrderSplit)
  workOrderSplitRows.value = [
    ...workOrderSplitRows.value.filter((split) => !splitMatchesOperation(split, operation)),
    ...autoRows,
  ]
}

function assignRemainingWorkOrderSplitQty(split) {
  split.planned_qty = defaultPlannedQtyForWorkOrderSplit(split)
}

function withAutoWorkOrderSplits(rows) {
  let nextRows = [...(rows || [])]
  for (const operation of workOrderSplitOperations.value) {
    if (nextRows.some((split) => splitMatchesOperation(split, operation))) continue
    const autoRows = buildOperationCapacityAutoSplits({
      id: Number(workOrderSplitRow.value?.production_plan_item_id || 0),
      planned_g: Number(workOrderSplitRow.value?.planned_g || 0),
      spec_g: Number(workOrderSplitRow.value?.spec_g || 0),
    }, operation, activeWorkstationCapacities.value).map(normalizeWorkOrderSplit)
    if (autoRows.length) nextRows = [...nextRows, ...autoRows]
  }
  return nextRows
}

function openWorkOrderSplitDrawer(row) {
  workOrderSplitRow.value = { ...row }
  workOrderSplitRows.value = withAutoWorkOrderSplits(operationSummaryRows(row)
    .filter((item) => Number(item.workstation_capacity_id || 0) > 0)
    .map(normalizeWorkOrderSplit))
  workOrderSplitError.value = ''
}

function closeWorkOrderSplitDrawer() {
  workOrderSplitRow.value = null
  workOrderSplitRows.value = []
  workOrderSplitError.value = ''
}

async function saveWorkOrderOperationSplits() {
  const endpoint = workOrderOperationSplitsEndpoint(workOrderSplitRow.value)
  if (!endpoint) return
  workOrderSplitSaving.value = true
  workOrderSplitError.value = ''
  try {
    const payload = buildWorkOrderOperationSplitPayload(workOrderSplitRows.value)
    await apiSend(endpoint, { body: payload })
    await load()
    closeWorkOrderSplitDrawer()
  } catch (err) {
    workOrderSplitError.value = err.message || '保存工单拆分失败'
  } finally {
    workOrderSplitSaving.value = false
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
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}h3{margin:0;font-size:16px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,input,button{font:inherit;min-height:36px;border-radius:6px}select,input{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #9ca3af;background:#fff;color:#111}.compact{min-height:30px;padding:5px 10px}.danger-text{color:#b91c1c}.row-actions{display:flex;gap:6px;flex-wrap:wrap}.table-wrap{overflow:auto}table{width:100%;min-width:1260px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.advice strong{display:block}.summary{max-width:220px;line-height:1.45}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.status.info{border-color:#93c5fd;background:#eff6ff;color:#1d4ed8}.status.warning{border-color:#fed7aa;background:#fff7ed;color:#c2410c}.status.success{border-color:#bbf7d0;background:#f0fdf4;color:#15803d}.status.danger{border-color:#fecaca;background:#fef2f2;color:#b91c1c}.status.neutral{border-color:#d1d5db;background:#f9fafb;color:#374151}.muted{color:#666;text-align:center}.text-left{text-align:left}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}.print-sheet{display:none}.drawer-backdrop{position:fixed;inset:0;background:rgba(17,24,39,.28);z-index:40;display:flex;justify-content:flex-end}.work-order-split-drawer{width:min(900px,92vw);height:100%;overflow:auto;background:#fff;padding:18px;box-shadow:-12px 0 28px rgba(15,23,42,.18);display:grid;align-content:start;gap:14px}.drawer-head{display:flex;justify-content:space-between;gap:12px;align-items:flex-start;border-bottom:1px solid #e5e7eb;padding-bottom:12px}.drawer-head p{margin:6px 0 0;color:#666}.split-operation-block{border-top:1px solid #e5e7eb;padding-top:14px;display:grid;gap:10px}.split-operation-head{display:flex;align-items:center;gap:10px;flex-wrap:wrap}.split-operation-head span{color:#666}.split-row{display:grid;grid-template-columns:minmax(220px,1.2fr) minmax(130px,.6fr) repeat(3,minmax(92px,.4fr)) auto;gap:10px;align-items:end;border:1px solid #eef2f7;border-radius:8px;padding:10px}.split-metric{border:1px solid #e5e7eb;border-radius:6px;padding:7px 9px;background:#fbfbfb}.split-metric span{display:block;font-size:12px;color:#666}.split-metric strong{font-size:14px}.split-batch-cards{grid-column:1/-1;display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:8px}.split-batch-card{border:1px solid #dbeafe;border-radius:6px;background:#eff6ff;padding:8px;display:grid;gap:3px}.split-batch-card small,.split-batch-card span{color:#374151}.split-batch-card.underfilled{border-color:#fed7aa;background:#fff7ed}.split-batch-card em{color:#c2410c;font-style:normal;font-size:12px}.section-hint{padding:8px 0}.drawer-actions{display:flex;gap:10px;justify-content:flex-end;border-top:1px solid #e5e7eb;padding-top:12px}

@media print{
  :global(body.work-order-printing .sidebar),:global(body.work-order-printing .top){display:none!important}
  :global(body.work-order-printing .content){width:100%!important;margin:0!important;padding:0!important}
  .page{display:block;padding:0}.no-print{display:none!important}.print-sheet{display:block;color:#111;padding:18mm;font-family:system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial}.print-head{display:flex;justify-content:space-between;align-items:flex-start;border-bottom:2px solid #111;padding-bottom:12px;margin-bottom:16px}.print-head h1{font-size:24px;margin:0 0 6px}.print-head p{margin:0;color:#444}.print-status{border:1px solid #111;border-radius:4px;padding:6px 12px}.print-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;margin-bottom:18px}.print-grid div{border:1px solid #ddd;border-radius:6px;padding:8px}.print-grid span{display:block;color:#555;font-size:11px;margin-bottom:4px}.print-grid strong{font-size:13px}.print-sheet h2{font-size:16px;margin:18px 0 8px}.print-table{min-width:0;width:100%;border:1px solid #ddd}.print-table th,.print-table td{border:1px solid #ddd;padding:8px}.print-table th{width:120px;background:#f7f7f7}
}
</style>
