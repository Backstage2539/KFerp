<template>
  <div class="page">
    <ProductionTopNav v-if="!props.embedded" active-key="workOrders" />
    <div v-if="props.viewParams?.return_navigation?.key" class="source-plan-context">
      <button class="secondary" type="button" @click="returnToSource">{{ props.viewParams.return_navigation.label || '返回来源操作' }}</button>
    </div>
    <div v-if="sourcePlanNo" class="source-plan-context">来自生产计划 {{ sourcePlanNo }} · {{ rows.length }} 张工单</div>

    <section class="panel no-print">
      <div class="panel-head">
        <h2>生产工单</h2>
        <button class="secondary" @click="load" :disabled="loading || receiptBusy">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <p v-if="receiptMessage" class="receipt-success" role="status">{{ receiptMessage }}</p>
      <div v-if="warehouseLoadError" class="error" role="alert">仓库名称暂不可用：{{ warehouseLoadError }}。工单仍可查看。</div>
      <p class="muted">人员安排跟随当天工位排班；已排班后进入工位处理，无需逐任务分配。</p>
      <div class="filters">
        <label>
          <span>搜索</span>
          <input :disabled="receiptBusy" v-model.trim="query" placeholder="产品或工单号" />
        </label>
        <label>
          <span>状态</span>
          <select :disabled="receiptBusy" v-model="status">
            <option value="active">未结束</option>
            <option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
        </label>
        <button class="primary" :disabled="receiptBusy" @click="load">查询</button>
      </div>
    </section>

    <section class="panel table-wrap no-print work-order-list-card">
      <table data-work-order-status-list>
        <thead>
          <tr>
            <th>产品 / 工单</th>
            <th>目标数量</th>
            <th>当前状态</th>
            <th>工序进度</th>
            <th>当前待办</th>
            <th>快捷操作</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="row in visibleRows" :key="row.id">
          <tr>
            <td class="product-cell" data-label="产品 / 工单">
              <strong>{{ row.output_name || row.product_name || '-' }}</strong>
              <button class="link-button work-order-link" type="button" @click="openExecutionHub(row, 'summary')">{{ row.work_order_no }}</button>
              <small>{{ formatWorkOrderTypedOutput(row) }}</small>
              <small v-if="Number(row.spec_g || 0) > 0">冻结规格：{{ Number(row.spec_g).toLocaleString('zh-CN') }}g/袋</small>
              <small>{{ row.order_nos ? `来源销售订单：${row.order_nos}` : `生产计划 #${row.production_plan_id || '-'}` }}</small>
            </td>
            <td data-label="目标数量"><strong>{{ formatWorkOrderPlannedOutput(row) }}</strong><small>目标仓库：{{ warehouseLabel(row.target_warehouse) }}</small></td>
            <td data-label="当前状态"><span class="status" :class="statusBadgeClass(row.status)">{{ workOrderStatusLabel(row.status) }}</span><small>{{ row.batch_id || '尚未建立运行批次' }}</small></td>
            <td data-label="工序进度"><strong>{{ operationProgressText(row) }}</strong><small>{{ operationCurrentText(row) }}</small></td>
            <td class="todo-cell" data-label="当前待办"><strong>{{ workOrderTodo(row).title }}</strong><small>{{ workOrderTodo(row).detail }}</small></td>
            <td class="row-actions" data-label="快捷操作">
              <button class="primary compact" type="button" :disabled="loading || receiptBusy" @click="openPrimaryAction(row)">{{ receiptBusy && receivingWorkOrderID === Number(row.id) ? '正在入库…' : primaryActionLabel(row) }}</button>
              <button
                v-for="source in workOrderLinkedReplenishmentSources(row)"
                :key="`replenish-${row.id}-${source.source_work_order_id}-${source.material_id}`"
                class="secondary compact"
                type="button"
                :disabled="loading || linkedReplenishment.loading"
                @click="openLinkedReplenishment(row, source)"
              >关联补产</button>
              <button class="secondary compact" type="button" @click="openExecutionHub(row, 'summary')">查看工单</button>
              <button v-if="canEditWorkOrderSplits(row)" class="secondary compact" type="button" @click="openWorkOrderSplitDrawer(row)">编辑拆分</button>
              <button v-if="canCancelWorkOrder(row)" class="secondary compact danger-text" type="button" @click="cancelWorkOrder(row)">取消工单</button>
            </td>
          </tr>
          </template>
          <tr v-if="!visibleRows.length"><td colspan="6" class="muted">暂无符合条件的生产工单</td></tr>
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

    <div v-if="linkedReplenishment.open" class="drawer-backdrop no-print" @click.self="closeLinkedReplenishment">
      <aside class="work-order-split-drawer linked-replenishment-drawer" aria-label="关联补产核对">
        <div class="drawer-head">
          <div>
            <div class="muted text-left">关联补产 · {{ linkedReplenishment.source?.source_work_order_no || '-' }}</div>
            <h2>{{ linkedReplenishment.source?.material_name || '上游物料' }}</h2>
            <p>沿用上游工单冻结配方和工艺路线，按当前库存、其他工单占用和在途补产重算缺口。</p>
          </div>
          <button class="secondary compact" type="button" @click="closeLinkedReplenishment">关闭</button>
        </div>
        <div v-if="linkedReplenishment.error" class="error" role="alert">
          {{ linkedReplenishment.error }}
          <button v-if="linkedReplenishment.error.includes('已变化') || linkedReplenishment.error.includes('已过期')" class="secondary compact" type="button" @click="refreshLinkedReplenishmentPreview" :disabled="linkedReplenishment.loading || linkedReplenishment.submitting">刷新缺口</button>
        </div>
        <div v-if="linkedReplenishment.loading" class="muted drawer-loading">正在核对可用库存和包装工单缺口…</div>
        <template v-else-if="linkedReplenishment.preview">
          <div class="linked-replenishment-summary">
            <div><span>原上游工单</span><strong>{{ linkedReplenishment.preview.source_work_order_no }}</strong></div>
            <div><span>来源仓</span><strong>{{ warehouseLabel(linkedReplenishment.preview.warehouse) }}</strong></div>
            <div><span>当前可用</span><strong>{{ linkedReplenishmentQuantity(linkedReplenishment.preview.available_g, linkedReplenishment.preview.available_units, linkedReplenishment.preview.unit) }}</strong></div>
            <div><span>建议补产</span><strong>{{ linkedReplenishmentQuantity(linkedReplenishment.preview.replenish_g, linkedReplenishment.preview.replenish_units, linkedReplenishment.preview.unit) }}</strong></div>
          </div>
          <div class="table-wrap">
            <table>
              <thead><tr><th>包装工单</th><th>本次可由现有供给满足</th><th>补产缺口</th></tr></thead>
              <tbody>
                <tr v-for="consumer in linkedReplenishment.preview.consumers || []" :key="consumer.work_order_id">
                  <td>{{ consumer.work_order_no }}</td>
                  <td>{{ linkedReplenishmentQuantity(consumer.required_g - consumer.replenish_g, consumer.required_units - consumer.replenish_units, linkedReplenishment.preview.unit) }}</td>
                  <td>{{ linkedReplenishmentQuantity(consumer.replenish_g, consumer.replenish_units, linkedReplenishment.preview.unit) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="linkedReplenishment.preview.output_qty > 0" class="info-banner">将创建 {{ Number(linkedReplenishment.preview.output_qty).toLocaleString('zh-CN', { maximumFractionDigits: 3 }) }} {{ linkedReplenishment.preview.unit }} 的备料生产草稿；提交计划后，产出按包装工单缺口分配。</div>
          <div v-else class="muted section-hint">当前可用库存及已有供给已覆盖所选工单，无需补产。</div>
        </template>
        <div class="drawer-actions">
          <button class="primary" type="button" @click="createLinkedReplenishment" :disabled="linkedReplenishment.loading || linkedReplenishment.submitting || !linkedReplenishment.preview || !(linkedReplenishment.preview.output_qty > 0)">{{ linkedReplenishment.submitting ? '正在创建…' : '创建关联补产草稿' }}</button>
          <button class="secondary" type="button" @click="closeLinkedReplenishment" :disabled="linkedReplenishment.submitting">返回</button>
        </div>
      </aside>
    </div>

    <ProductionExecutionHubDrawer
      :open="executionHub.open"
      :work-order-id="executionHub.workOrderId"
      :focus="executionHub.focus"
      :view-params="{ ...(props.viewParams || {}), work_order_id: executionHub.workOrderId, focus: executionHub.focus }"
      @close="executionHub.open = false"
      @updated="load" />

  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import ProductionExecutionHubDrawer from '../components/ProductionExecutionHubDrawer.vue'
import ProductionTopNav from '../components/ProductionTopNav.vue'
import { formatPercent } from '../lib/manufacturing-loss'
import { workOrderStatusLabel } from '../lib/manufacturing-execution'
import { clearRecordedOutputReceiptAttempt, readRecordedOutputReceiptAttempt, recordedOutputReceiptPayload, saveRecordedOutputReceiptAttempt } from '../lib/recorded-output-receipt.js'
import { loadProductionPlanWorkOrders } from '../lib/production-plan-work-orders'
import {
  applicableOperationCapacities,
  buildOperationCapacityAutoSplits,
  maxAssignableQtyForCapacitySplit,
  plannedCapacitySplitMetrics,
  productionPlanItemOutputTargetG,
  productionPlanSplitBatchCards,
  qtyFromGForCapacityUnit,
} from '../lib/produce-plan'
import {
  normalizeCapacityCostMethod,
  workstationCapacityOptionLabel,
} from '../lib/workstation-capacity-costing'
import {
  buildWorkOrderOperationSplitPayload,
  canCancelWorkOrder,
  canEditWorkOrderSplits,
  formatWorkOrderTypedOutput,
  formatWorkOrderPlannedOutput,
  workOrderHasLinkedReplenishmentSource,
  workOrderLinkedReplenishmentSources,
  workOrderHasUpstreamBlocker,
  workOrderOperationSplitsEndpoint,
  workOrderCancelEndpoint,
  workOrderStatusOptions,
  workOrderUpstreamBlockerLabel,
  workOrderUpstreamSupplyText,
} from '../lib/work-orders'

const props = defineProps({
  embedded: { type: Boolean, default: false },
  viewParams: { type: Object, default: () => ({}) },
})

const rows = ref([])
const sourcePlanNo = ref('')
const warehouseNames = ref({})
const warehouseLoadError = ref('')
function returnToSource() {
  const source = props.viewParams?.return_navigation
  if (source?.key) window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: source.key, params: source.params || {} } }))
}
const workstationCapacities = ref([])
const status = ref('active')
const query = ref('')
const loading = ref(false)
const error = ref('')
const todayRoster = ref(null)
const rosterError = ref('')
let loadRequestID = 0
const workOrderSplitRow = ref(null)
const workOrderSplitRows = ref([])
const workOrderSplitSaving = ref(false)
const workOrderSplitError = ref('')
const statusOptions = workOrderStatusOptions()
const receiptBusy = ref(false)
const receivingWorkOrderID = ref(0)
const receiptAttempts = new Map()
const receiptMessage = ref('')
const executionHub = ref({ open: false, workOrderId: 0, focus: '' })
const linkedReplenishment = ref({ open: false, source: null, consumerWorkOrderIDs: [], preview: null, loading: false, submitting: false, error: '', requestID: '' })
const visibleRows = computed(() => {
  const q = query.value.toLowerCase()
  return rows.value.filter((row) => {
    if (status.value === 'active' && ['completed', 'cancelled'].includes(String(row.status || ''))) return false
    if (!q) return true
    return [row.work_order_no, row.output_name, row.product_name].some((value) => String(value || '').toLowerCase().includes(q))
  })
})

const money = (v) => Number(v || 0).toFixed(2)
const percent = (v) => formatPercent(v)
const formatG = (v) => `${Number(v || 0).toLocaleString('zh-CN')}g`
const formatQty = (v) => {
  const n = Number(v || 0)
  return n ? `${n.toLocaleString('zh-CN', { maximumFractionDigits: 3 })}` : '-'
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

function operationCurrentText(row) {
  const items = operationSummaryRows(row)
  const current = items.find((item) => !['completed', 'cancelled'].includes(String(item.status || '').trim()))
  if (!current) return items.length ? '全部工序已完成' : '等待生成工序任务'
  const state = ({pending:'未开工',ready:'未开工',running:'进行中',paused:'已暂停'})[String(current.status || '').trim()] || '待执行'
  const output = Number(current.actual_output_qty || 0)
  return `${current.operation || '工序'} · ${state}${output > 0 ? ` · 已完成 ${formatWorkOrderQuantity(output, current.output_unit)}` : ''} · ${current.workstation || current.work_center || '待分配工位'}`
}

function workOrderTodo(row) {
  if (row.status === 'completed') return { title: '查看记录', detail: '工单已完成，查看工序、耗料和入库记录', action: 'records' }
  if (row.status === 'cancelled') return { title: '已取消', detail: '工单已结束', action: 'summary' }
  if (workOrderHasUpstreamBlocker(row)) return { title: '等待上游供给', detail: workOrderUpstreamBlockerLabel(row) }
  const items = operationSummaryRows(row)
  if (!items.length) return { title: '等待生成工序任务', detail: '请查看工单的工序配置', action: 'summary' }
  const incomplete = items.filter((item) => !['completed', 'cancelled'].includes(String(item.status || '').trim()))
  if (!incomplete.length) return { title: '确认数量并入库', detail: '在本行填写实际数量并直接入库', action: 'receipt' }
  const assignments = todayRoster.value?.assignments || []
  const assignmentFor = item => assignments.find(assignment => Number(item.workstation_id || 0) > 0
    ? Number(assignment.workstation_id) === Number(item.workstation_id)
    : String(item.workstation || '').trim() && String(assignment.workstation || '').trim() === String(item.workstation).trim())
  if (incomplete.some(item => ['running', 'paused'].includes(item.status) && (assignmentFor(item)?.handover_task_ids || []).some(id => Number(id) === Number(item.id || item.job_card_id)))) {
    return { title: '待工位交接', detail: '当天负责人已变化，请到工位确认接手', action: 'workstation' }
  }
  if (incomplete.some(item => item.status === 'running')) return { title: `继续${incomplete.find(item => item.status === 'running')?.operation || '当前工序'}`, detail: operationCurrentText(row), action: 'workstation' }
  if (incomplete.some(item => item.status === 'paused')) return { title: `继续${incomplete.find(item => item.status === 'paused')?.operation || '已暂停工序'}`, detail: operationCurrentText(row), action: 'workstation' }
  const missingStations = new Map()
  for (const item of incomplete) {
    const assignment = assignmentFor(item)
    if (!assignment || assignment.unattended || assignment.override_invalid || !Number(assignment.employee_id || 0)) {
      const name = item.workstation || '未指定工位'
      const key = Number(item.workstation_id || 0) || name
      missingStations.set(key, `${name}：${assignment?.reason || (assignment?.override_invalid ? '临时调整已失效' : '当天无人值班')}`)
    }
  }
  const owners = [...new Set(incomplete.map(item => assignmentFor(item)?.employee_name).filter(Boolean))]
  const first = incomplete[0]
  const title = ['pending','ready','released'].includes(String(first.status || '')) ? `去${first.operation || '对应工序'}` : '继续对应工序'
  const supply = workOrderUpstreamSupplyText(row)
  const rosterNotice = loading.value
    ? '正在核对当天工位排班'
    : rosterError.value
      ? `排班暂不可用：${rosterError.value}`
      : !todayRoster.value
        ? '当天排班尚未读取'
        : !Number(todayRoster.value.roster_version || 0)
          ? '本周排班尚未保存，进入工序后仍需先安排负责人'
          : missingStations.size
            ? `待安排工位：${[...missingStations.values()].join('；')}`
            : ''
  return { title, detail: [rosterNotice, owners.length ? `今日负责人：${owners.join('、')}` : '', operationCurrentText(row), supply].filter(Boolean).join(' · '), action: 'workstation' }
}

function primaryActionLabel(row) {
  return { receipt: '确认数量并入库', workstation: workOrderTodo(row).title, records: '查看记录' }[workOrderTodo(row).action] || '查看工单'
}

async function openPrimaryAction(row) {
  const todo = workOrderTodo(row)
  const action = todo.action
  if (action === 'retry') return load()
  if (action === 'receipt') { await receiveRecordedOutput(row); return }
  if (action === 'records') return openExecutionHub(row, 'summary')
  if (action === 'workstation') {
    const card = operationSummaryRows(row).find(item => !['completed','cancelled'].includes(String(item.status || '').trim()))
    if (!card) return openExecutionHub(row, 'summary')
    window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: {
      key: 'workstationView',
      params: { work_order_id: row.id, job_card_id: Number(card.id || card.job_card_id || 0), workstation_id: Number(card.workstation_id || 0), station_id: Number(card.workstation_id || 0), focus: 'workstation_task' },
      returnNavigation: { key: 'workOrders', label: '返回生产工单', params: { ...props.viewParams, work_order_id: row.id } },
    } }))
    return
  }
  const key = { roster: 'productionSchedule' }[action]
  if (key) {
    window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: {
      key, params: { work_order_id: row.id },
      returnNavigation: { key: 'workOrders', label: '返回生产工单', params: { ...props.viewParams, work_order_id: row.id } },
    } }))
    return
  }
  openExecutionHub(row, 'summary')
}

async function receiveRecordedOutput(row) {
  if (receiptBusy.value) return
  const id=Number(row.id),attemptKey=String(id)
  receiptBusy.value=true;receivingWorkOrderID.value=id;error.value='';receiptMessage.value=''
  try {
    let attempt=receiptAttempts.get(attemptKey)||readRecordedOutputReceiptAttempt(window.sessionStorage,id)
    if(!attempt){
      const preview=await apiSend('/api/production/legacy-review/preview',{body:{work_order_ids:[id]}})
      const review=(preview.rows||[]).find(item=>Number(item.work_order_id)===id)
      if(!review)throw new Error('本单没有待入库的已完成数量，请刷新工单列表')
      if(review.receipt_blocked_reason)throw new Error(review.receipt_blocked_reason)
      attempt=recordedOutputReceiptPayload(review.version,crypto.randomUUID())
      receiptAttempts.set(attemptKey,attempt)
      saveRecordedOutputReceiptAttempt(window.sessionStorage,id,attempt)
    }
    const result=await apiSend(`/api/produce/work-orders/${id}/receive-completed-output`,{body:attempt})
    const completion=result.completion||{}
    receiptMessage.value=`${row.work_order_no}：${completion.message||'入库成功'} · ${completion.output_qty??''} ${completion.output_unit||''} · 单据 ${(completion.stock_entry_ids||[]).join('、')}${completion.inventory_warning?` · ${completion.inventory_warning}`:''}`
    receiptAttempts.delete(attemptKey)
    clearRecordedOutputReceiptAttempt(window.sessionStorage,id)
    await load()
  } catch(err) {
    if(err.status&&err.status<500){receiptAttempts.delete(attemptKey);clearRecordedOutputReceiptAttempt(window.sessionStorage,id)}
    error.value=(!err.status||err.status>=500)?'尚未收到入库结果；再次点击会以原请求重试，不会重复入库。':(err.message||'入库失败')
  } finally { receiptBusy.value=false;receivingWorkOrderID.value=0 }
}

function linkedReplenishmentQuantity(grams, units, unit) {
  if (Number(grams || 0) > 0) {
    const value = Number(grams)
    return value >= 1000 ? `${(value / 1000).toLocaleString('zh-CN', { maximumFractionDigits: 3 })} kg` : `${value.toLocaleString('zh-CN')} g`
  }
  return `${Number(units || 0).toLocaleString('zh-CN')} ${unit || '件'}`.trim()
}

function linkedReplenishmentConsumerRows(sourceID) {
  return visibleRows.value.filter((candidate) => workOrderHasLinkedReplenishmentSource(candidate, sourceID))
}

async function openLinkedReplenishment(row, source) {
  const consumers = linkedReplenishmentConsumerRows(source.source_work_order_id)
  const consumerWorkOrderIDs = [...new Set(consumers.map((candidate) => Number(candidate.id || 0)).filter((id) => id > 0))]
  linkedReplenishment.value = {
    open: true,
    source,
    consumerWorkOrderIDs,
    preview: null,
    loading: true,
    submitting: false,
    error: '',
    requestID: '',
  }
  if (!consumerWorkOrderIDs.includes(Number(row.id))) consumerWorkOrderIDs.push(Number(row.id))
  try {
    linkedReplenishment.value.preview = await apiSend('/api/production/work-orders/replenishment/preview', {
      body: { source_work_order_id: source.source_work_order_id, consumer_work_order_ids: consumerWorkOrderIDs },
    })
  } catch (err) {
    linkedReplenishment.value.error = err.message || '关联补产预览失败，请刷新工单后重试'
  } finally {
    linkedReplenishment.value.loading = false
  }
}

function closeLinkedReplenishment() {
  if (linkedReplenishment.value.submitting) return
  linkedReplenishment.value.open = false
}

async function refreshLinkedReplenishmentPreview() {
  const state = linkedReplenishment.value
  if (!state.open || state.loading || state.submitting || !state.source) return
  state.loading = true
  state.error = ''
  state.preview = null
  state.requestID = ''
  try {
    state.preview = await apiSend('/api/production/work-orders/replenishment/preview', {
      body: { source_work_order_id: state.source.source_work_order_id, consumer_work_order_ids: state.consumerWorkOrderIDs },
    })
  } catch (err) {
    state.error = err.message || '刷新缺口失败，请稍后重试'
  } finally {
    state.loading = false
  }
}

async function createLinkedReplenishment() {
  const state = linkedReplenishment.value
  if (!state.preview || !(Number(state.preview.output_qty) > 0) || state.submitting) return
  state.submitting = true
  state.error = ''
  if (!state.requestID) state.requestID = globalThis.crypto?.randomUUID?.() || `linked-replenishment-${Date.now()}-${Math.random().toString(16).slice(2)}`
  try {
    const plan = await apiSend('/api/production/work-orders/replenishment', {
      body: {
        source_work_order_id: state.source.source_work_order_id,
        consumer_work_order_ids: state.consumerWorkOrderIDs,
        expected_version: state.preview.version,
        request_id: state.requestID,
      },
    })
    state.open = false
    await load()
    window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: {
      key: 'producePlan',
      params: { production_plan_id: Number(plan.id || 0) },
      returnNavigation: { key: 'workOrders', label: '返回生产工单', params: { ...props.viewParams } },
    } }))
  } catch (err) {
    state.error = err.message || '关联补产草稿创建失败，请刷新预览后重试'
  } finally {
    state.submitting = false
  }
}

function formatWorkOrderQuantity(qty, unit) { return `${Number(qty || 0).toLocaleString('zh-CN', { maximumFractionDigits: 3 })} ${unit || ''}`.trim() }
function warehouseLabel(code) {
  const key = String(code || '').trim()
  if (!key) return '未设置'
  return warehouseNames.value[key] || (key.startsWith('virtual_') ? '仓库名称待加载' : key)
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
  const requestID = ++loadRequestID
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/produce/work-orders', window.location.origin)
    if (status.value && status.value !== 'active') url.searchParams.set('status', status.value)
    const [data, capacityData, rosterResult, warehouseResult] = await Promise.all([
      Number(props.viewParams?.production_plan_id || 0) > 0
        ? loadProductionPlanWorkOrders(apiGet, props.viewParams.production_plan_id, status.value === 'active' ? '' : status.value)
        : apiGet(url),
      apiGet('/api/manufacturing-workstation-capacities'),
      apiGet('/api/production-roster/today?scope=all')
        .then(data => ({ data, error: '' }))
        .catch(err => ({ data: null, error: err.message || '当天排班读取失败，请刷新重试' })),
      apiGet('/api/stock/warehouses')
        .then(data => ({ rows: data.rows || data || [], error: '' }))
        .catch(err => ({ rows: [], error: err.message || '仓库加载失败' })),
    ])
    if (requestID !== loadRequestID) return
    rows.value = data.rows || []
    warehouseNames.value = Object.fromEntries(warehouseResult.rows.filter(w=>w.active!==false).map(w=>[String(w.code),String(w.name || w.code)]))
    warehouseLoadError.value = warehouseResult.error
    sourcePlanNo.value = data.plan_no || ''
    workstationCapacities.value = capacityData.rows || []
    todayRoster.value = rosterResult.data
    rosterError.value = rosterResult.error
  } catch (err) {
    if (requestID !== loadRequestID) return
    error.value = err.message || '加载失败'
    todayRoster.value = null
    rosterError.value = '当天排班读取失败，请刷新重试'
  } finally {
    if (requestID === loadRequestID) loading.value = false
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
  return workstationCapacityOptionLabel(capacity)
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
    planned_output_g: Math.max(0, Number(workOrderSplitRow.value?.planned_output_g || 0)),
    spec_g: Number(workOrderSplitRow.value?.spec_g || 0),
    sales_spec_count: Number(workOrderSplitRow.value?.sales_spec_count || 0),
    inventory_qty_per_sales_unit: Number(workOrderSplitRow.value?.inventory_qty_per_sales_unit || 0),
    inventory_unit: String(workOrderSplitRow.value?.inventory_unit || ''),
    planned_inventory_qty: Number(workOrderSplitRow.value?.planned_inventory_qty || 0),
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
    cost_method: normalizeCapacityCostMethod(row),
    piece_rate: Number(row.piece_rate || 0),
    planned_batch_count: Number(row.planned_batch_count || 0),
    spec_g: Number(row.spec_g || workOrderSplitRow.value?.spec_g || 0),
    sales_spec_count: Number(row.sales_spec_count || workOrderSplitRow.value?.sales_spec_count || 0),
    item_target_g: Number(row.item_target_g || productionPlanItemOutputTargetG(workOrderSplitRow.value || {})),
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
  split.cost_method = normalizeCapacityCostMethod(capacity)
  split.piece_rate = Number(capacity.piece_rate || 0)
  split.spec_g = Number(split.spec_g || workOrderSplitRow.value?.spec_g || 0)
  split.sales_spec_count = Number(split.sales_spec_count || workOrderSplitRow.value?.sales_spec_count || 0)
  split.item_target_g = Number(split.item_target_g || productionPlanItemOutputTargetG(workOrderSplitRow.value || {}))
  if (Number(split.planned_qty || 0) <= 0) {
    split.planned_qty = defaultPlannedQtyForWorkOrderSplit(split)
  }
}

function autoSplitWorkOrderOperation(operation) {
  if (!workOrderSplitRow.value) return
  const autoRows = buildOperationCapacityAutoSplits({
    id: Number(workOrderSplitRow.value.production_plan_item_id || 0),
    planned_g: Number(workOrderSplitRow.value.planned_g || 0),
    planned_output_g: Number(workOrderSplitRow.value.planned_output_g || 0),
    spec_g: Number(workOrderSplitRow.value.spec_g || 0),
    sales_spec_count: Number(workOrderSplitRow.value.sales_spec_count || 0),
    inventory_qty_per_sales_unit: Number(workOrderSplitRow.value.inventory_qty_per_sales_unit || 0),
    inventory_unit: String(workOrderSplitRow.value.inventory_unit || ''),
    planned_inventory_qty: Number(workOrderSplitRow.value.planned_inventory_qty || 0),
  }, operation, activeWorkstationCapacities.value).map(normalizeWorkOrderSplit)
  workOrderSplitRows.value = [
    ...workOrderSplitRows.value.filter((split) => !splitMatchesOperation(split, operation)),
    ...autoRows,
  ]
}

function withAutoWorkOrderSplits(rows) {
  let nextRows = [...(rows || [])]
  for (const operation of workOrderSplitOperations.value) {
    if (nextRows.some((split) => splitMatchesOperation(split, operation))) continue
    const autoRows = buildOperationCapacityAutoSplits({
      id: Number(workOrderSplitRow.value?.production_plan_item_id || 0),
      planned_g: Number(workOrderSplitRow.value?.planned_g || 0),
      planned_output_g: Number(workOrderSplitRow.value?.planned_output_g || 0),
      spec_g: Number(workOrderSplitRow.value?.spec_g || 0),
      sales_spec_count: Number(workOrderSplitRow.value?.sales_spec_count || 0),
      inventory_qty_per_sales_unit: Number(workOrderSplitRow.value?.inventory_qty_per_sales_unit || 0),
      inventory_unit: String(workOrderSplitRow.value?.inventory_unit || ''),
      planned_inventory_qty: Number(workOrderSplitRow.value?.planned_inventory_qty || 0),
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

function openExecutionHub(row, focus = 'summary') {
  const id = Number(row?.id || row?.work_order_id || props.viewParams?.work_order_id || 0)
  if (!id) return
  executionHub.value = { open: true, workOrderId: id, focus }
}

async function cancelWorkOrder(row) {
  const endpoint = workOrderCancelEndpoint(row)
  if (!canCancelWorkOrder(row) || !endpoint) return
  const workOrderNo = String(row?.work_order_no || `#${Number(row?.id || 0)}`)
  if (!window.confirm(`确认取消未开工工单 ${workOrderNo}？取消后该工单将不能再开始生产。`)) return
  error.value = ''
  try {
    await apiSend(workOrderCancelEndpoint(row), { body: {} })
    await load()
  } catch (err) {
    error.value = err.message || '取消工单失败'
  }
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


onMounted(() => {
  load()
  if (Number(props.viewParams?.work_order_id || 0) > 0) {
    openExecutionHub({ id: Number(props.viewParams.work_order_id) }, props.viewParams?.focus || 'summary')
  }
})

</script>

<style scoped>
.receipt-success{padding:12px;color:#21824b;background:#edf7f0;border-radius:8px}
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}h3{margin:0;font-size:16px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,input,button{font:inherit;min-height:36px;border-radius:6px}select,input{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #9ca3af;background:#fff;color:#111}.compact{min-height:30px;padding:5px 10px}.danger-text{color:#b91c1c}.link-button{border:0;background:transparent;color:#1d4ed8;padding:0;min-height:0;text-decoration:underline}.work-order-link{font-weight:700}.row-actions{display:flex;gap:6px;flex-wrap:wrap}.table-wrap{overflow:auto}table{width:100%;min-width:1260px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.advice strong{display:block}.summary{max-width:220px;line-height:1.45}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.status.info{border-color:#93c5fd;background:#eff6ff;color:#1d4ed8}.status.warning{border-color:#fed7aa;background:#fff7ed;color:#c2410c}.status.success{border-color:#bbf7d0;background:#f0fdf4;color:#15803d}.status.danger{border-color:#fecaca;background:#fef2f2;color:#b91c1c}.status.neutral{border-color:#d1d5db;background:#f9fafb;color:#374151}.muted{color:#666;text-align:center}.text-left{text-align:left}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}.drawer-backdrop{position:fixed;inset:0;background:rgba(17,24,39,.28);z-index:40;display:flex;justify-content:flex-end}.work-order-split-drawer{width:min(900px,92vw);height:100%;overflow:auto;background:#fff;padding:18px;box-shadow:-12px 0 28px rgba(15,23,42,.18);display:grid;align-content:start;gap:14px}.drawer-head{display:flex;justify-content:space-between;gap:12px;align-items:flex-start;border-bottom:1px solid #e5e7eb;padding-bottom:12px}.drawer-head p{margin:6px 0 0;color:#666}.split-operation-block{border-top:1px solid #e5e7eb;padding-top:14px;display:grid;gap:10px}.split-operation-head{display:flex;align-items:center;gap:10px;flex-wrap:wrap}.split-operation-head span{color:#666}.split-row{display:grid;grid-template-columns:minmax(220px,1.2fr) minmax(130px,.6fr) repeat(3,minmax(92px,.4fr)) auto;gap:10px;align-items:end;border:1px solid #eef2f7;border-radius:8px;padding:10px}.split-metric{border:1px solid #e5e7eb;border-radius:6px;padding:7px 9px;background:#fbfbfb}.split-metric span{display:block;font-size:12px;color:#666}.split-metric strong{font-size:14px}.split-batch-cards{grid-column:1/-1;display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:8px}.split-batch-card{border:1px solid #dbeafe;border-radius:6px;background:#eff6ff;padding:8px;display:grid;gap:3px}.split-batch-card small,.split-batch-card span{color:#374151}.split-batch-card.underfilled{border-color:#fed7aa;background:#fff7ed}.split-batch-card em{color:#c2410c;font-style:normal;font-size:12px}.section-hint{padding:8px 0}.drawer-actions{display:flex;gap:10px;justify-content:flex-end;border-top:1px solid #e5e7eb;padding-top:12px}

.page{background:#f7f8fafa}.panel{border-radius:10px;padding:14px}.filters{grid-template-columns:minmax(220px,1fr) 160px 90px}.work-order-list-card table{min-width:900px}.work-order-list-card th,.work-order-list-card td{padding:13px 10px;border-color:#edf0f2}.work-order-list-card th{background:#f6f8f9;color:#56616b}.product-cell{display:grid;gap:4px}.todo-cell strong{color:#a85a08}.primary{border-color:#2f8f5b;background:#2f8f5b}.link-button{color:#24704a;text-decoration:none}.row-actions{align-items:flex-start}

@media (max-width:900px){.filters{grid-template-columns:1fr}.work-order-list-card{overflow:visible}.work-order-list-card table,.work-order-list-card tbody,.work-order-list-card td{display:block;min-width:0}.work-order-list-card thead{display:none}.work-order-list-card tr{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px 18px;border:1px solid #e5e7eb;border-radius:10px;padding:12px;margin-bottom:10px}.work-order-list-card td{border:0;padding:5px 0}.work-order-list-card td::before{content:attr(data-label);display:block;margin-bottom:4px;color:#7a8580;font-size:11px;font-weight:700}.work-order-list-card .product-cell,.work-order-list-card .row-actions{grid-column:1/-1}.work-order-list-card .product-cell::before{display:none}.work-order-list-card .row-actions{display:flex;flex-wrap:wrap;margin-top:2px}.work-order-list-card .row-actions::before{flex-basis:100%;margin-bottom:0}}
@media (max-width:600px){.work-order-list-card tr{grid-template-columns:1fr}.work-order-list-card .product-cell,.work-order-list-card .row-actions{grid-column:auto}}

.page{background:#f7f8fa}.linked-replenishment-drawer{width:min(760px,92vw)}.linked-replenishment-summary{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}.linked-replenishment-summary>div{display:grid;gap:4px;border:1px solid #e5e7eb;border-radius:8px;padding:10px}.linked-replenishment-summary span{font-size:12px;color:#6b7280}.linked-replenishment-drawer .info-banner{border-radius:8px;background:#eff6ff;color:#1e40af;padding:12px}.linked-replenishment-drawer .table-wrap table{min-width:520px}
</style>
