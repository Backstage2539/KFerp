import { normalizeCapacityCostMethod } from './workstation-capacity-costing.js'

export function producePlanKey(productId, specG, bomSpecID = 0) {
  const normalizedBomSpecID = Number(bomSpecID || 0)
  if (normalizedBomSpecID > 0) return `product:${Number(productId || 0)}:bom_spec:${normalizedBomSpecID}`
  return `${productId}-${specG}`
}

function defaultSelectionKey(row) {
	const scoped = String(row?.selection_key || '').trim()
	if (scoped) return scoped
  return producePlanKey(row.parent_product_id || row.product_id, row.spec_g, row.bom_spec_id)
}

export function insufficientSelectionState(rows, selected, keyForRow = defaultSelectionKey) {
  const keys = (rows || []).map(keyForRow)
  const selectedCount = keys.filter((key) => !!selected?.[key]).length
  const total = keys.length
  return {
    checked: total > 0 && selectedCount === total,
    indeterminate: selectedCount > 0 && selectedCount < total,
    selectedCount,
    total,
  }
}

export function buildInsufficientSelection(rows, checked, keyForRow = defaultSelectionKey) {
  if (!checked) return {}
  return Object.fromEntries((rows || []).map((row) => [keyForRow(row), true]))
}

const PRODUCTION_DEMAND_STATUS = {
  unplanned: { label: '待计划', tone: 'unplanned' },
  in_production: { label: '生产中', tone: 'in-production' },
  completed: { label: '生产完成', tone: 'completed' },
}

const PRODUCTION_DEMAND_STATUS_VALUES = new Set(Object.keys(PRODUCTION_DEMAND_STATUS))

export function defaultProductionDemandStatusFilter() {
  return 'unplanned'
}

export function productionDemandStatusOptions() {
  return [
    { value: '', label: '全部' },
    ...Object.entries(PRODUCTION_DEMAND_STATUS).map(([value, item]) => ({ value, label: item.label })),
  ]
}

export function productionDemandStatusFilterValue(status, fallback = '') {
  const value = String(status || '').trim()
  if (PRODUCTION_DEMAND_STATUS_VALUES.has(value)) return value
  return fallback
}

function normalizedProductionDemandStatus(status) {
  return productionDemandStatusFilterValue(status, 'unplanned')
}

export function productionDemandStatusLabel(status) {
  const key = normalizedProductionDemandStatus(status)
  return PRODUCTION_DEMAND_STATUS[key]?.label || '-'
}

export function productionDemandStatusTone(status) {
  const key = normalizedProductionDemandStatus(status)
  return PRODUCTION_DEMAND_STATUS[key]?.tone || 'unplanned'
}

export function productionDemandPanelTitle(status) {
  const value = String(status || '').trim()
  if (!value) return '生产需求'
  return `${productionDemandStatusLabel(value)}需求`
}

export function productionDemandPanelEmptyText(status) {
  const value = String(status || '').trim()
  if (!value) return '暂无生产需求'
  return `暂无${productionDemandStatusLabel(value)}需求`
}

export function productionDemandSelectable(row) {
  if (String(row?.blocking_reason || '').trim()) return false
  if (row?.demand_selectable === false) return false
  return productionDemandGapQuantity(row) > 0 && normalizedProductionDemandStatus(row?.demand_status) === 'unplanned'
}

export function isBomSpecProductionDemand(row = {}) {
  return Number(row?.bom_spec_id || row?.bomSpecID || 0) > 0
}

export function productionDemandGapQuantity(row = {}) {
  if (isBomSpecProductionDemand(row)) return Math.max(0, Number(row?.gap_inventory_qty || 0))
  return Math.max(0, Number(row?.gap_g || 0))
}

function productionDemandQuantity(value, unit) {
  const amount = Math.max(0, Number(value || 0))
  const formatted = Number.isInteger(amount) ? String(amount) : String(Number(amount.toFixed(6)))
  return unit ? `${formatted} ${unit}` : formatted
}

export function productionDemandQuantityLabel(row = {}, kind = 'need') {
  if (isBomSpecProductionDemand(row)) {
    const fields = { need: 'need_inventory_qty', available: 'available_inventory_qty', gap: 'gap_inventory_qty' }
    return productionDemandQuantity(row?.[fields[kind] || fields.need], String(row?.inventory_unit || row?.output_unit || '').trim())
  }
  const fields = { need: 'need_g', available: 'inv_g', gap: 'gap_g' }
  return `${Math.max(0, Number(row?.[fields[kind] || fields.need] || 0))}g`
}

function productionDemandSelectionKeys(rows) {
  return (rows || []).filter(productionDemandSelectable).map(defaultSelectionKey)
}

export function productionDemandSelectionState(rows, selected) {
  const keys = productionDemandSelectionKeys(rows)
  const selectedCount = keys.filter((key) => !!selected?.[key]).length
  const total = keys.length
  return {
    checked: total > 0 && selectedCount === total,
    indeterminate: selectedCount > 0 && selectedCount < total,
    selectedCount,
    total,
  }
}

export function buildProductionDemandSelection(rows, checked) {
  if (!checked) return {}
  return Object.fromEntries(productionDemandSelectionKeys(rows).map((key) => [key, true]))
}

export function buildProductionDemandSummaryQuery(filters = {}, plan = false, selectedKeys = []) {
  const params = new URLSearchParams()
  const from = String(filters.from || '').trim()
  const to = String(filters.to || '').trim()
  const customerID = Number(filters.customer_id || 0)
  const demandStatus = String(filters.demand_status || '').trim()
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  if (customerID > 0) params.set('customer_id', String(customerID))
  if (PRODUCTION_DEMAND_STATUS_VALUES.has(demandStatus)) params.set('demand_status', demandStatus)
  if (plan && selectedKeys.length) {
    params.set('plan', '1')
    params.set('selected', selectedKeys.join(','))
  }
  const query = params.toString()
  return `/api/produce/unproduced${query ? `?${query}` : ''}`
}

export function buildProductionPlanCreatePayload(filters, selectedKeys) {
  filters = filters || {}
  selectedKeys = selectedKeys || []
  return {
    from: filters.from || '',
    to: filters.to || '',
    customer_id: Number(filters.customer_id || 0),
    selected: selectedKeys,
    source_type: 'erp_order',
  }
}

const PRODUCTION_PLAN_STATUS = {
  draft: { label: '草稿', tone: 'draft' },
  submitted: { label: '已提交工单', tone: 'submitted' },
  in_progress: { label: '生产中', tone: 'in-progress' },
  completed: { label: '已完成', tone: 'completed' },
  cancelled: { label: '已取消', tone: 'cancelled' },
}

const PRODUCTION_PLAN_TIME_FIELDS = new Set(['created_at', 'submitted_at', 'completed_at'])

const PRODUCTION_PLAN_STEPS = [
  { key: 'selectDemand', label: '选需求' },
  { key: 'createDraft', label: '生成草稿' },
  { key: 'splitCapacity', label: '拆分产能' },
  { key: 'submitWorkOrders', label: '提交工单' },
  { key: 'startProduction', label: '开始生产' },
]

export function productionPlanSteps() {
  return PRODUCTION_PLAN_STEPS.map((step) => ({ ...step }))
}

export function currentProductionPlanStep(state = {}) {
  const status = String(state.plan?.status || '').trim()
  if (['submitted', 'in_progress', 'completed'].includes(status)) return 'startProduction'
  if (status === 'draft') {
    return Number(state.splitCount || 0) > 0 ? 'submitWorkOrders' : 'splitCapacity'
  }
  return Number(state.selectedCount || 0) > 0 ? 'createDraft' : 'selectDemand'
}

export function buildProductionPlanNextActions(result = {}) {
  const firstSuccess = Array.isArray(result.success) ? result.success[0] : null
  const workOrder = Array.isArray(firstSuccess?.work_orders) ? firstSuccess.work_orders[0] : null
  const jobCard = Array.isArray(firstSuccess?.job_cards) ? firstSuccess.job_cards[0] : null
  const workOrderID = Number(workOrder?.id || jobCard?.work_order_id || 0)
  const jobCardID = Number(jobCard?.id || 0)
  return [
    { key: 'workOrders', label: '打开工单', view: 'workOrders', params: compactPositiveParams({ work_order_id: workOrderID }) },
    { key: 'jobCards', label: '打开工序卡', view: 'jobCards', params: compactPositiveParams({ job_card_id: jobCardID, work_order_id: workOrderID }) },
    { key: 'assignWorkstation', label: '分配工位', view: 'productionOverview', params: compactPositiveParams({ work_order_id: workOrderID, job_card_id: jobCardID }) },
    { key: 'issueWip', label: '生产领料', view: 'stockOperations', params: compactPositiveParams({ tab: 'stockEntries', action: 'issue', return_source: 'work_order', work_order_id: workOrderID, job_card_id: jobCardID }) },
  ]
}

function compactPositiveParams(params = {}) {
  const out = {}
  for (const [key, value] of Object.entries(params)) {
    if (typeof value === 'string') {
      if (value.trim()) out[key] = value
      continue
    }
    const number = Number(value || 0)
    if (number > 0) out[key] = number
  }
  return out
}

export function productionPlanStatusLabel(status) {
  const key = String(status || '').trim()
  return PRODUCTION_PLAN_STATUS[key]?.label || key || '-'
}

export function productionPlanStatusTone(status) {
  const key = String(status || '').trim()
  return PRODUCTION_PLAN_STATUS[key]?.tone || 'unknown'
}

export function productionPlanSelectable(plan) {
  return Number(plan?.id || 0) > 0 && String(plan?.status || '').trim() === 'draft'
}

function productionPlanSelectionKeys(plans) {
  return (plans || []).filter(productionPlanSelectable).map((plan) => String(Number(plan.id)))
}

export function productionPlanSelectionState(plans, selected) {
  const keys = productionPlanSelectionKeys(plans)
  const selectedCount = keys.filter((key) => !!selected?.[key]).length
  const total = keys.length
  return {
    checked: total > 0 && selectedCount === total,
    indeterminate: selectedCount > 0 && selectedCount < total,
    selectedCount,
    total,
  }
}

export function buildProductionPlanSelection(plans, checked) {
  if (!checked) return {}
  return Object.fromEntries(productionPlanSelectionKeys(plans).map((key) => [key, true]))
}

export function buildProductionPlanBatchSubmitPayload(selected) {
  const ids = Object.keys(selected || {})
    .filter((key) => !!selected[key])
    .map((key) => Number(key))
    .filter((id) => Number.isInteger(id) && id > 0)
  return { ids }
}

export function buildCurrentProductionPlanSubmitPayload(plan) {
  const id = Number(plan?.id || 0)
  if (!Number.isInteger(id) || id <= 0) return { ids: [] }
  return { ids: [id] }
}

export function productionPlanBatchSubmitEndpoint() {
  return '/api/production-plans/submit'
}

export function productionPlanDetailEndpoint(plan) {
  const id = Number(plan?.id || 0)
  if (id <= 0) return ''
  return `/api/production-plans/${id}`
}

export function productionPlanOperationSplitsEndpoint(plan) {
  const id = Number(plan?.id || 0)
  if (id <= 0) return ''
  return `/api/production-plans/${id}/operation-splits`
}

export function productionPlanOperationSplitsPreviewEndpoint(plan) {
  const id = Number(plan?.id || 0)
  if (id <= 0) return ''
  return `/api/production-plans/${id}/operation-splits/preview`
}

function compactProductionQuantity(value) {
  const number = Number(value || 0)
  if (!Number.isFinite(number)) return '0'
  return String(Number(number.toFixed(9)))
}

function productionInventoryUnitLabel(unit) {
  const value = String(unit || '').trim()
  if (value.toLowerCase() === 'kg') return 'Kg'
  return value
}

export function productionPlanItemQuantitySummary(item = {}) {
  const count = Number(item.sales_spec_count || 0)
  const quantityPerUnit = Number(item.inventory_qty_per_sales_unit || 0)
  const plannedQuantity = Number(item.planned_inventory_qty || 0)
  const unit = productionInventoryUnitLabel(item.inventory_unit)
  if (count > 0 && quantityPerUnit > 0 && plannedQuantity > 0 && unit) {
    return `${compactProductionQuantity(count)}件、${compactProductionQuantity(quantityPerUnit)}${unit}/件、合计${compactProductionQuantity(plannedQuantity)}${unit}`
  }
  const legacySpecG = Number(item.spec_g || 0)
  if (legacySpecG > 0) return `${compactProductionQuantity(legacySpecG)}g`
  return '按商品单位'
}

export function productionPlanLegacyGramLabel(value) {
  const grams = Number(value || 0)
  if (!Number.isFinite(grams)) return '0g'
  return `${compactProductionQuantity(grams)}g`
}

export function productionPlanBomSummary(row = {}) {
  if (String(row.bom_summary_error || '').trim()) return 'BOM 配置待完善'
  const lossRate = Number(row.bom_material_loss_rate || 0)
  if (!Number.isFinite(lossRate) || lossRate <= 0 || lossRate >= 1) return '默认 BOM'
  return `BOM原料损耗 ${(lossRate * 100).toFixed(2)}%`
}

export function productionPlanItemBomSourceLabel(item = {}) {
  const versionID = Number(item.bom_version_id || 0)
  const versionLabel = versionID > 0 ? `BOM版本 #${versionID}` : '默认 BOM'
  if (item.bom_inherited === true) return `${versionLabel} · 继承父商品BOM`
  if (Number(item.bom_source_product_id || 0) > 0) return `${versionLabel} · 具体SKU BOM`
  return versionLabel
}

const OPERATION_SPLIT_PREVIEW_STATUS = {
  matched: { label: '已覆盖', tone: 'matched' },
  short: { label: '不足', tone: 'short' },
  over: { label: '超排', tone: 'over' },
  missing: { label: '未安排', tone: 'missing' },
}

export function operationSplitPreviewStatusLabel(status) {
  return OPERATION_SPLIT_PREVIEW_STATUS[String(status || '').trim()]?.label || '-'
}

export function operationSplitPreviewStatusTone(status) {
  return OPERATION_SPLIT_PREVIEW_STATUS[String(status || '').trim()]?.tone || 'missing'
}

const COUNT_CAPACITY_UNITS = new Set(['件', '个', '袋', '盒', 'unit', 'units', 'pc', 'pcs'])
const MATERIAL_WEIGHT_UNITS = new Set(['g', 'kg', '克', '千克', '公斤'])

function normalizedCapacityUnit(unit) {
  return String(unit || '').trim().toLowerCase()
}

function isWeightCapacityUnit(unit) {
  const normalized = normalizedCapacityUnit(unit)
  return normalized === 'kg' || normalized === '千克' || normalized === '公斤' || normalized === 'g' || normalized === '克'
}

function isCountCapacityUnit(unit) {
  return COUNT_CAPACITY_UNITS.has(normalizedCapacityUnit(unit))
}

function plannedCapacitySplitQtyG(qty, unit, specG = 0, salesSpecCount = 0, targetG = 0) {
  const normalized = String(unit || '').trim().toLowerCase()
  if (normalized === 'kg' || normalized === '千克' || normalized === '公斤') return Math.round(qty * 1000)
  if (normalized === 'g' || normalized === '克') return Math.round(qty)
  if (isCountCapacityUnit(unit) && Number(salesSpecCount || 0) > 0 && Number(targetG || 0) > 0) {
    return Math.round(qty * Number(targetG) / Number(salesSpecCount))
  }
  if (isCountCapacityUnit(unit) && Number(specG || 0) > 0) return Math.round(qty * Number(specG || 0))
  return 0
}

export function productionPlanItemTargetG(item = {}) {
  return Math.max(0, Number(item.planned_g || item.planned_output_g || item.gap_g || 0))
}

export function productionPlanItemOutputTargetG(item = {}) {
  const frozenOutputG = Math.max(0, Number(item.planned_output_g || 0))
  if (frozenOutputG > 0) return frozenOutputG

  const inventoryUnit = String(item.inventory_unit || '').trim().toLowerCase()
  const toGrams = (quantity) => {
    const value = Math.max(0, Number(quantity || 0))
    if (inventoryUnit === 'g' || inventoryUnit === '克') return Math.round(value)
    if (inventoryUnit === 'kg' || inventoryUnit === '千克' || inventoryUnit === '公斤') return Math.round(value * 1000)
    if (inventoryUnit === 'lb' || inventoryUnit === '磅') return Math.round(value * 453.59237)
    return 0
  }

  const plannedInventoryG = toGrams(item.planned_inventory_qty)
  if (plannedInventoryG > 0) return plannedInventoryG

  const count = Math.max(0, Number(item.sales_spec_count || 0))
  const inventoryQtyPerSalesUnit = Math.max(0, Number(item.inventory_qty_per_sales_unit || 0))
  const convertedG = toGrams(count * inventoryQtyPerSalesUnit)
  if (convertedG > 0) return convertedG

  const specG = Math.max(0, Number(item.spec_g || 0))
  if (count > 0 && specG > 0) return Math.round(count * specG)
  return productionPlanItemTargetG(item)
}

function isProductionMaterialWeightUnit(unit) {
  return MATERIAL_WEIGHT_UNITS.has(String(unit || '').trim().toLowerCase())
}

export function productionMaterialQuantity(item = {}, field) {
  const value = Math.max(0, Number(item?.[field] || 0))
  if (value > 0 || isProductionMaterialWeightUnit(item?.unit)) return value
  if (field !== 'purchase_suggestion_g' && field !== 'shortage_g') return value
  const qty = Math.max(0, Number(item?.qty || 0))
  const available = Math.max(0, Number(item?.available_g || item?.wip_g || 0))
  const raw = Math.max(0, Number(item?.raw_g || 0))
  return Math.max(0, qty - available - raw)
}

export function qtyFromGForCapacityUnit(qtyG, unit, specG = 0) {
  const normalized = normalizedCapacityUnit(unit)
  const value = Number(qtyG || 0)
  if (value <= 0) return 0
  if (normalized === 'kg' || normalized === '千克' || normalized === '公斤') return roundedSplitQty(value / 1000)
  if (normalized === 'g' || normalized === '克') return roundedSplitQty(value)
  if (isCountCapacityUnit(unit) && Number(specG || 0) > 0) return roundedSplitQty(value / Number(specG || 0))
  return 0
}

export function plannedCapacitySplitMetrics(split = {}) {
  const batchSizeQty = Math.max(0, Number(split.batch_size_qty || 0))
  const standardMinutes = Math.max(0, Math.round(Number(split.standard_minutes || 0)))
  const hourlyRate = Math.max(0, Number(split.hourly_rate || 0))
  const legacyBatchCount = Math.max(0, Math.round(Number(split.planned_batch_count || 0)))
  const specG = Number(split.spec_g || split.item_spec_g || 0)
  const salesSpecCount = Number(split.sales_spec_count || split.item_sales_spec_count || 0)
  const itemTargetG = Number(split.item_target_g || split.planned_item_g || 0)
  let plannedQty = Math.max(0, Number(split.planned_qty || 0))
  if (plannedQty <= 0 && legacyBatchCount > 0 && batchSizeQty > 0) {
    plannedQty = legacyBatchCount * batchSizeQty
  }
  plannedQty = Number(plannedQty.toFixed(3))
  const plannedBatchCount = plannedQty > 0 && batchSizeQty > 0
    ? Math.ceil(plannedQty / batchSizeQty)
    : legacyBatchCount
  let plannedQtyG = plannedCapacitySplitQtyG(plannedQty, split.batch_size_unit, specG, salesSpecCount, itemTargetG)
  if (plannedQtyG <= 0 && Number(split.planned_qty_g || 0) > 0) {
    plannedQtyG = Math.round(Number(split.planned_qty_g || 0))
  }
  const plannedMinutes = plannedBatchCount * standardMinutes
  const plannedOperationCost = normalizeCapacityCostMethod(split) === 'piece'
    ? Number((plannedQty * Math.max(0, Number(split.piece_rate || 0))).toFixed(2))
    : Number(((plannedMinutes / 60) * hourlyRate).toFixed(2))
  return {
    planned_batch_count: plannedBatchCount,
    planned_qty: plannedQty,
    planned_qty_g: plannedQtyG,
    planned_minutes: plannedMinutes,
    planned_operation_cost: plannedOperationCost,
  }
}

function roundedSplitQty(qty) {
  return Number(Math.max(0, Number(qty || 0)).toFixed(3))
}

export function capacityDefaultPlannedQty(capacity = {}) {
  return roundedSplitQty(capacity.batch_size_qty)
}

export function productionPlanSplitBatchCards(split = {}) {
  const metrics = plannedCapacitySplitMetrics(split)
  const plannedBatchCount = Math.max(0, Math.round(Number(metrics.planned_batch_count || 0)))
  if (plannedBatchCount <= 0) return []

  const batchSizeQty = roundedSplitQty(split.batch_size_qty)
  const standardMinutes = Math.max(0, Math.round(Number(split.standard_minutes || 0)))
  const unit = String(split.batch_size_unit || '').trim()
  const capacityName = String(split.workstation_capacity_name || '').trim()
  const plannedQty = roundedSplitQty(metrics.planned_qty)
  let remaining = plannedQty

  return Array.from({ length: plannedBatchCount }, (_, index) => {
    const isLast = index === plannedBatchCount - 1
    let plannedQtyForBatch = batchSizeQty > 0
      ? (isLast ? remaining : Math.min(batchSizeQty, remaining))
      : (plannedBatchCount > 0 ? plannedQty / plannedBatchCount : 0)
    if (isLast && plannedQtyForBatch <= 0 && batchSizeQty > 0) plannedQtyForBatch = batchSizeQty
    plannedQtyForBatch = roundedSplitQty(plannedQtyForBatch)
    remaining = roundedSplitQty(remaining - plannedQtyForBatch)

    return {
      label: `第${index + 1}批`,
      workstation_capacity_name: capacityName,
      batch_size_qty: batchSizeQty,
      batch_size_unit: unit,
      planned_qty: plannedQtyForBatch,
      planned_qty_g: plannedCapacitySplitQtyG(
        plannedQtyForBatch,
        unit,
        split.spec_g || split.item_spec_g || 0,
        split.sales_spec_count || split.item_sales_spec_count || 0,
        split.item_target_g || split.planned_item_g || 0,
      ),
      planned_minutes: standardMinutes,
      underfilled: batchSizeQty > 0 && plannedQtyForBatch > 0 && plannedQtyForBatch < batchSizeQty,
    }
  })
}

function operationIDOf(operation = {}) {
  return Number(operation.operation_id || operation.id || 0)
}

function operationSeqOf(operation = {}) {
  return Math.max(0, Math.round(Number(operation.seq || operation.operation_seq || 0)))
}

function operationNameOf(operation = {}) {
  return String(operation.operation || operation.name || '').trim()
}

export function capacityAppliesToOperation(capacity = {}, operation = {}) {
  const opID = operationIDOf(operation)
  const ids = Array.isArray(capacity.applicable_operation_ids)
    ? capacity.applicable_operation_ids.map((id) => Number(id || 0)).filter((id) => id > 0)
    : []
  if (ids.length > 0 && opID > 0) return ids.includes(opID)
  if (ids.length === 0 && Array.isArray(capacity.applicable_operations) && capacity.applicable_operations.length > 0) {
    const opName = operationNameOf(operation)
    return capacity.applicable_operations.some((item) => Number(item?.id || 0) === opID || (opName && String(item?.name || '').trim() === opName))
  }
  return false
}

export function applicableOperationCapacities(operation = {}, capacities = []) {
  return (capacities || [])
    .filter((capacity) => String(capacity?.status || 'active').trim() === 'active')
    .filter((capacity) => Number(capacity?.batch_size_qty || 0) > 0)
    .filter((capacity) => capacityAppliesToOperation(capacity, operation))
}

export function operationCapacityAutoSplitError(item = {}, operation = {}, capacities = []) {
  if (productionPlanItemTargetG(item) <= 0 && Number(item.sales_spec_count || 0) <= 0) return '当前计划行缺少计划产量，无法自动拆分'
  if (!applicableOperationCapacities(operation, capacities).length) {
    return '当前工序没有可用的工位产能，或工位产能未绑定该工序'
  }
  return ''
}

function capacityBaseQty(capacity = {}, item = {}) {
  const batchSizeQty = Number(capacity.batch_size_qty || 0)
  const unit = capacity.batch_size_unit
  const targetG = productionPlanItemTargetG(item)
  if (batchSizeQty <= 0) return { kind: '', batchBaseQty: 0, targetBaseQty: 0 }
  if (isWeightCapacityUnit(unit)) {
    const batchBaseQty = plannedCapacitySplitQtyG(batchSizeQty, unit, item.spec_g || item.item_spec_g || 0)
    return { kind: 'weight', batchBaseQty, targetBaseQty: targetG }
  }
  if (isCountCapacityUnit(unit) && Number(item.sales_spec_count || 0) > 0) {
    return { kind: 'count', batchBaseQty: batchSizeQty, targetBaseQty: Number(item.sales_spec_count) }
  }
  if (isCountCapacityUnit(unit) && Number(item.spec_g || 0) > 0) {
    return { kind: 'count', batchBaseQty: batchSizeQty, targetBaseQty: qtyFromGForCapacityUnit(targetG, unit, item.spec_g || 0) }
  }
  return { kind: '', batchBaseQty: 0, targetBaseQty: 0 }
}

function qtyFromBaseForCapacity(baseQty, unit, specG = 0) {
  const normalized = normalizedCapacityUnit(unit)
  if (normalized === 'kg' || normalized === '千克' || normalized === '公斤') return roundedSplitQty(baseQty / 1000)
  if (normalized === 'g' || normalized === '克') return roundedSplitQty(baseQty)
  if (isCountCapacityUnit(unit)) return roundedSplitQty(baseQty)
  if (Number(specG || 0) > 0) return qtyFromGForCapacityUnit(baseQty, unit, specG)
  return roundedSplitQty(baseQty)
}

function splitRowsSameOperation(row, split) {
  if (Number(row?.production_plan_item_id || 0) !== Number(split?.production_plan_item_id || 0)) return false
  const leftSeq = Math.max(0, Math.round(Number(row?.operation_seq || 0)))
  const rightSeq = Math.max(0, Math.round(Number(split?.operation_seq || 0)))
  if (leftSeq > 0 || rightSeq > 0) return leftSeq === rightSeq
  const leftID = Number(row?.operation_id || 0)
  const rightID = Number(split?.operation_id || 0)
  if (leftID > 0 || rightID > 0) return leftID === rightID
  return String(row?.operation || '').trim() === String(split?.operation || '').trim()
}

export function maxAssignableQtyForCapacitySplit(split = {}, rows = [], target = {}) {
  const specG = Number(target.spec_g || split.spec_g || split.item_spec_g || 0)
  const plannedG = productionPlanItemTargetG(target)
  const salesSpecCount = Number(target.sales_spec_count || split.sales_spec_count || split.item_sales_spec_count || 0)
  if (isCountCapacityUnit(split.batch_size_unit) && salesSpecCount > 0) {
    const usedQty = (rows || [])
      .filter((row) => row !== split && splitRowsSameOperation(row, split))
      .reduce((sum, row) => sum + Number(plannedCapacitySplitMetrics(row).planned_qty || 0), 0)
    const remainingQty = Math.max(0, salesSpecCount - usedQty)
    const batchQty = Number(split.batch_size_qty || 0)
    if (remainingQty <= 0) return 0
    if (batchQty > 0 && remainingQty >= batchQty) return roundedSplitQty(Math.floor(remainingQty / batchQty) * batchQty)
    return roundedSplitQty(remainingQty)
  }
  const usedG = (rows || [])
    .filter((row) => row !== split && splitRowsSameOperation(row, split))
    .reduce((sum, row) => sum + (plannedCapacitySplitMetrics({ ...row, spec_g: row.spec_g || specG }).planned_qty_g || 0), 0)
  const remainingG = Math.max(0, plannedG - usedG)
  const batchG = plannedCapacitySplitQtyG(Number(split.batch_size_qty || 0), split.batch_size_unit, specG)
  if (remainingG <= 0) return 0
  if (batchG > 0 && remainingG >= batchG) {
    return qtyFromGForCapacityUnit(Math.floor(remainingG / batchG) * batchG, split.batch_size_unit, specG)
  }
  return qtyFromGForCapacityUnit(remainingG, split.batch_size_unit, specG)
}

export function buildOperationCapacityAutoSplits(item = {}, operation = {}, capacities = []) {
  const inputTargetG = productionPlanItemTargetG(item)
  const outputTargetG = productionPlanItemOutputTargetG(item)
  if (inputTargetG <= 0 && Number(item.sales_spec_count || 0) <= 0) return []
  const candidates = applicableOperationCapacities(operation, capacities)
    .map((capacity) => {
      const base = capacityBaseQty(capacity, item)
      return { capacity, ...base }
    })
    .filter((row) => row.kind && row.batchBaseQty > 0 && row.targetBaseQty > 0)
  if (!candidates.length) return []

  const groups = new Map()
  for (const row of candidates) {
    if (!groups.has(row.kind)) groups.set(row.kind, [])
    groups.get(row.kind).push(row)
  }
  let selected = []
  for (const rows of groups.values()) {
    if (rows.length > selected.length) selected = rows
  }
  selected.sort((a, b) => b.batchBaseQty - a.batchBaseQty || String(a.capacity.name || '').localeCompare(String(b.capacity.name || '')))

  let remaining = selected[0]?.targetBaseQty || 0
  const assigned = new Map()
  while (remaining > 0.000001) {
    const full = selected.find((row) => row.batchBaseQty <= remaining + 0.000001)
    const row = full || [...selected].sort((a, b) => a.batchBaseQty - b.batchBaseQty)[0]
    if (!row) break
    const amount = full ? row.batchBaseQty : remaining
    assigned.set(row.capacity.id, {
      row,
      amount: (assigned.get(row.capacity.id)?.amount || 0) + amount,
    })
    remaining = Number((remaining - amount).toFixed(6))
  }

  return Array.from(assigned.values()).map(({ row, amount }) => {
    const capacity = row.capacity
    return {
      production_plan_item_id: Number(item.id || item.production_plan_item_id || 0),
      operation_seq: operationSeqOf(operation),
      operation_id: operationIDOf(operation),
      operation: operationNameOf(operation),
      workstation_id: Number(capacity.workstation_id || 0),
      workstation: String(capacity.workstation || ''),
      workstation_capacity_id: Number(capacity.id || 0),
      workstation_capacity_name: String(capacity.name || ''),
      batch_size_qty: Number(capacity.batch_size_qty || 0),
      batch_size_unit: String(capacity.batch_size_unit || ''),
      standard_minutes: Math.max(0, Math.round(Number(capacity.standard_minutes || 0))),
      hourly_rate: Math.max(0, Number(capacity.hourly_rate || 0)),
      cost_method: normalizeCapacityCostMethod(capacity),
      piece_rate: Math.max(0, Number(capacity.piece_rate || 0)),
      spec_g: Number(item.spec_g || 0),
      sales_spec_count: Number(item.sales_spec_count || 0),
      item_target_g: outputTargetG,
      planned_qty: qtyFromBaseForCapacity(amount, capacity.batch_size_unit, item.spec_g || 0),
      note: '',
    }
  })
}

export function buildProductionPlanOperationSplitPayload(rows = []) {
  const items = (rows || [])
    .map((row) => {
      return {
        production_plan_item_id: Number(row.production_plan_item_id || 0),
        operation_seq: Math.max(0, Math.round(Number(row.operation_seq || 0))),
        operation: String(row.operation || '').trim(),
        workstation_capacity_id: Number(row.workstation_capacity_id || 0),
        planned_qty: plannedCapacitySplitMetrics(row).planned_qty,
      }
    })
    .filter((row) => row.production_plan_item_id > 0 && row.workstation_capacity_id > 0 && row.planned_qty > 0)
  return { items }
}

export function buildProductionPlanListQuery(filters = {}) {
  const params = new URLSearchParams()
  const status = String(filters.status || '').trim()
  const timeField = PRODUCTION_PLAN_TIME_FIELDS.has(String(filters.time_field || '').trim())
    ? String(filters.time_field || '').trim()
    : 'created_at'
  const from = String(filters.from || '').trim()
  const to = String(filters.to || '').trim()
  let limit = Number(filters.limit || 50)
  if (!Number.isFinite(limit) || limit <= 0) limit = 50
  if (limit > 500) limit = 500

  if (status) params.set('status', status)
  params.set('time_field', timeField)
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  params.set('limit', String(Math.round(limit)))
  return `/api/production-plans?${params.toString()}`
}

export function productionPlanSubmitEndpoint(plan) {
  const id = Number(plan?.id || 0)
  if (id <= 0) return ''
  return `/api/production-plans/${id}/submit`
}

export function productionPlanCancelEndpoint(plan) {
  const id = Number(plan?.id || 0)
  if (id <= 0) return ''
  return `/api/production-plans/${id}/cancel`
}

export function productionPlanItemTargetWarehouseEndpoint(plan, item) {
  const planID = Number(plan?.id || 0)
  const itemID = Number(item?.id || item?.production_plan_item_id || 0)
  if (planID <= 0 || itemID <= 0) return ''
  return `/api/production-plans/${planID}/items/${itemID}/target-warehouse`
}

export function buildProductionPlanItemTargetWarehousePayload(targetWarehouse) {
  return { target_warehouse: String(targetWarehouse || '').trim() }
}

export function productionPlanItemComponentSourcesEndpoint(plan, item) {
  const planID = Number(plan?.id || 0)
  const itemID = Number(item?.id || item?.production_plan_item_id || 0)
  if (planID <= 0 || itemID <= 0) return ''
  return `/api/production-plans/${planID}/items/${itemID}/component-sources`
}

export function buildProductionPlanComponentSourcesPayload(sources = []) {
  return {
    sources: (sources || []).map((source) => ({
      component_type: String(source?.component_type || '').trim(),
      component_id: Number(source?.component_id || 0),
      component_bom_spec_id: Number(source?.component_bom_spec_id || 0),
      component_spec_g: Number(source?.component_spec_g || 0),
      source_warehouse: String(source?.source_warehouse || '').trim(),
      source_owner_customer_id: Number(source?.source_owner_customer_id || 0),
    })),
  }
}

export function productionPlanCanCancelSubmitted(plan) {
  if (Number(plan?.id || 0) <= 0 || String(plan?.status || '').trim() !== 'submitted') return false
  const workOrders = Array.isArray(plan?.related_work_orders) ? plan.related_work_orders : []
  if (!workOrders.length) return false
  return workOrders.every((row) => {
    const status = String(row?.status || '').trim()
    return ['released', 'cancelled'].includes(status) && Number(row?.running_item_id || 0) <= 0
  })
}

export function productionPlanCancelTargetsCurrentPlan(cancelledPlan, currentPlan) {
  const cancelledID = Number(cancelledPlan?.id || 0)
  const currentID = Number(currentPlan?.id || 0)
  return cancelledID > 0 && cancelledID === currentID
}

function manufacturingOutputType(item = {}) {
  return String(item.type || item.output_type || item.item_type || '').trim().toLowerCase() === 'material' ? 'material' : 'product'
}

function manufacturingOutputID(item = {}, type = manufacturingOutputType(item)) {
  const typedID = type === 'material'
    ? item.output_material_id || item.material_id
    : item.output_product_id || item.product_id
  return Number(typedID || item.output_id || item.id || item.item_id || 0)
}

function manufacturingOutputName(item = {}) {
  return String(item.name || item.output_name || item.item_name || '').trim()
}

function manufacturingNodeKey(item = {}) {
  const explicitKey = String(item.key || item.node_key || '').trim()
  if (explicitKey) return explicitKey
  const type = manufacturingOutputType(item)
  return `${type}:${manufacturingOutputID(item, type)}`
}

function manufacturingItemLabel(item = {}) {
  const type = manufacturingOutputType(item)
  const typeLabel = type === 'material' ? '物料' : '商品'
  const name = manufacturingOutputName(item) || `#${manufacturingOutputID(item, type)}`
  return `${typeLabel} · ${name}`
}

function manufacturingActionLabel(action = '') {
  return ({ inventory: '库存覆盖', manufacture: '安排生产', purchase: '采购/补料' })[String(action || '').trim().toLowerCase()] || '待处理'
}

function manufacturingPlanFallbackRows(plan = {}) {
  const items = Array.isArray(plan.items) ? plan.items : []
  const gaps = Array.isArray(plan.supply_gaps) ? plan.supply_gaps : []
  if (!items.length && !gaps.length) return []

  const itemByPlanID = new Map()
  const itemRows = items.map((item, index) => {
    const type = String(item.output_type || '').trim().toLowerCase() === 'material' || (!item.output_type && Number(item.output_material_id || 0) > 0)
      ? 'material'
      : 'product'
    const output = {
      type,
      id: Number(item.output_id || (type === 'material' ? item.output_material_id : item.output_product_id || item.product_id) || 0),
      name: String(item.output_name || (type === 'material' ? item.output_material_name : item.product_name) || '').trim(),
      unit: String(item.output_unit || item.inventory_unit || '').trim(),
    }
    const required = Number(item.required_qty ?? item.total_required_qty ?? item.output_qty ?? item.planned_inventory_qty ?? 0)
    const covered = Number(item.stock_covered_qty ?? item.inventory_covered_qty ?? 0)
    const shortage = Number(item.shortage_qty ?? item.net_shortage_qty ?? item.output_qty ?? Math.max(0, required - covered))
    const explicitDepth = Number(item.depth ?? item.dependency_depth)
    const depth = Number.isFinite(explicitDepth) && explicitDepth >= 0 ? explicitDepth : (type === 'material' ? 1 : 0)
    const consumerPlanItemID = Number(item.consumer_plan_item_id || item.downstream_plan_item_id || item.parent_plan_item_id || 0)
    const row = {
      ...item,
      key: manufacturingNodeKey(output),
      plan_item_id: Number(item.id || item.production_plan_item_id || 0),
      consumer_plan_item_id: consumerPlanItemID,
      depth,
      type,
      type_label: type === 'material' ? '物料' : '商品',
      name: output.name || (output.id > 0 ? `#${output.id}` : `计划行 #${Number(item.id || index + 1)}`),
      unit: output.unit,
      required_qty: required,
      stock_covered_qty: covered,
      shortage_qty: shortage,
      action_label: manufacturingActionLabel(item.action || 'manufacture'),
      blocking: Boolean(item.blocking),
      dependency_label: type === 'material' ? '供给计划内商品' : '最终产出',
      _index: index,
      _output: output,
    }
    if (row.plan_item_id > 0) itemByPlanID.set(row.plan_item_id, row)
    return row
  })

  for (const row of itemRows) {
    const consumer = itemByPlanID.get(row.consumer_plan_item_id)
    if (!consumer) continue
    row.depth = Math.max(row.depth, consumer.depth + 1)
    row.dependency_label = `供给 ${manufacturingItemLabel(consumer._output)}`
  }

  const gapRows = gaps.map((gap, index) => {
    const type = String(gap.item_type || gap.component_type || '').trim().toLowerCase() === 'product' ? 'product' : 'material'
    const id = Number(gap.item_id || gap.material_id || gap.product_id || 0)
    const consumer = itemByPlanID.get(Number(gap.production_plan_item_id || gap.consumer_plan_item_id || 0))
    const requiredG = Number(gap.required_g || 0)
    const requiredUnits = Number(gap.required_units || 0)
    const required = requiredG > 0 ? requiredG : requiredUnits
    const status = String(gap.status || '').trim().toLowerCase()
    return {
      ...gap,
      key: `gap:${type}:${id}:${Number(gap.id || index + 1)}`,
      depth: consumer ? consumer.depth + 1 : 1,
      type,
      type_label: type === 'material' ? '物料' : '商品',
      name: String(gap.item_name || gap.material_name || gap.product_name || '').trim() || (id > 0 ? `#${id}` : '待补足对象'),
      unit: requiredG > 0 ? 'g' : (requiredUnits > 0 ? '件' : String(gap.unit || '').trim()),
      required_qty: required,
      stock_covered_qty: 0,
      shortage_qty: required,
      action_label: manufacturingActionLabel('purchase'),
      blocking: !['resolved', 'completed', 'closed'].includes(status),
      dependency_label: consumer ? `阻断 ${manufacturingItemLabel(consumer._output)}` : '待补足上游供应',
      _index: items.length + index,
    }
  })

  return [...itemRows, ...gapRows]
    .sort((a, b) => a.depth - b.depth || a._index - b._index)
    .map(({ _index, _output, ...row }) => row)
}

export function manufacturingPlanRows(payload = {}) {
  const plan = payload.manufacturing_plan || payload.multilevel_plan || payload
  const nodes = Array.isArray(plan.nodes) ? plan.nodes : []
  const edges = Array.isArray(plan.edges) ? plan.edges : []
  if (!nodes.length) {
    return manufacturingPlanFallbackRows({
      ...payload,
      ...plan,
      items: plan.items || payload.items || [],
      supply_gaps: plan.supply_gaps || payload.supply_gaps || [],
    })
  }
  const byKey = new Map(nodes.map((node) => {
    const item = node.item || node.output || node
    return [manufacturingNodeKey(item), { node, item }]
  }))
  const suppliersByConsumer = new Map()
  const consumersBySupplier = new Map()
  for (const edge of edges) {
    const consumer = String(edge.consumer_key || edge.consumerKey || '').trim()
    const supplier = String(edge.supplier_key || edge.supplierKey || '').trim()
    if (!consumer || !supplier) continue
    if (!suppliersByConsumer.has(consumer)) suppliersByConsumer.set(consumer, [])
    suppliersByConsumer.get(consumer).push(supplier)
    if (!consumersBySupplier.has(supplier)) consumersBySupplier.set(supplier, [])
    consumersBySupplier.get(supplier).push(consumer)
  }
  const roots = [...byKey.keys()].filter((key) => !(consumersBySupplier.get(key) || []).length)
  const ordered = []
  const seen = new Set()
  const visit = (key, depth) => {
    if (!byKey.has(key) || seen.has(key)) return
    seen.add(key)
    ordered.push({ key, depth })
    for (const supplier of suppliersByConsumer.get(key) || []) visit(supplier, depth + 1)
  }
  for (const key of roots) visit(key, 0)
  for (const key of byKey.keys()) visit(key, 0)
  return ordered.map(({ key, depth }) => {
    const { node, item } = byKey.get(key)
    const consumers = [...new Set(consumersBySupplier.get(key) || [])]
      .map((consumerKey) => byKey.get(consumerKey)?.item)
      .filter(Boolean)
    const backendDepth = Number(node.depth)
    const displayDepth = Number.isFinite(backendDepth) && backendDepth >= 0 ? backendDepth : depth
    const type = key.startsWith('material:') ? 'material' : 'product'
    return {
      ...node,
      key,
      depth: displayDepth,
      type,
      type_label: type === 'material' ? '物料' : '商品',
      name: manufacturingOutputName(item) || `#${manufacturingOutputID(item, type)}`,
      unit: String(item.unit || node.output_unit || '').trim(),
      required_qty: Number(node.required_qty || 0),
      stock_covered_qty: Number(node.stock_covered_qty || 0),
      shortage_qty: Number(node.shortage_qty || 0),
      action_label: manufacturingActionLabel(node.action),
      blocking: Boolean(node.blocking),
      dependency_label: consumers.length
        ? `供给 ${consumers.map((consumer) => manufacturingItemLabel(consumer)).join('；')}`
        : '最终产出',
    }
  })
}
