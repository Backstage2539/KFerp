const contextKeys = ['tab', 'action', 'return_source', 'work_order_id', 'work_order_no', 'job_card_id', 'running_item_id', 'material_id', 'shortage_g', 'reference_no', 'focus', 'batch_id']
const inventoryWeightUnitGrams = Object.freeze({
  g: 1,
  '克': 1,
  kg: 1000,
  '千克': 1000,
  '公斤': 1000,
  lb: 453.59237,
  lbs: 453.59237,
  '磅': 453.59237,
})

function positiveInt(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0
}

function cleanValue(key, value) {
  if (['work_order_id', 'job_card_id', 'running_item_id', 'material_id', 'shortage_g'].includes(key)) return positiveInt(value)
  return String(value || '').trim()
}

export function inventoryUnitWeightInGrams(unit = '') {
  const text = String(unit || '').trim()
  return inventoryWeightUnitGrams[text] || inventoryWeightUnitGrams[text.toLowerCase()] || 0
}

export function stockQuantityUsesCount(item = {}) {
  const basis = String(item.quantity_basis || '').trim().toLowerCase()
  if (['count', 'units', 'unit', 'quantity_units'].includes(basis)) return true
  if (['weight', 'mass', 'g', 'quantity_g'].includes(basis)) return false
  const unit = String(item.inventory_unit || '').trim().toLowerCase()
  return ['个', '件', '袋', '盒', '条', '包', 'pcs', 'pc', 'unit', 'units'].includes(unit)
}

export function stockCanonicalQuantity(item = {}) {
  const quantity = Number(item.quantity || 0)
  if (stockQuantityUsesCount(item)) {
    if (quantity > 0 && !Number.isInteger(quantity)) throw new Error('计数物料数量必须为整数')
    return { qty_g: 0, qty_units: quantity }
  }
  const factor = Number(item.canonical_qty_per_unit || 0) || inventoryUnitWeightInGrams(item.inventory_unit)
  return { qty_g: factor > 0 ? Math.round(quantity * factor) : 0, qty_units: 0 }
}

export function productionContextParams(input = {}) {
  const out = {}
  for (const key of contextKeys) {
    const value = cleanValue(key, input[key])
    if (value) out[key] = value
  }
  return out
}

function hubWorkOrder(hub = {}) {
  const row = hub.work_order || hub.header || {}
  return {
    id: positiveInt(row.id || row.work_order_id),
    work_order_no: row.work_order_no || '',
    running_item_id: positiveInt(row.running_item_id),
    batch_id: row.batch_id || '',
  }
}

function firstJobCard(hub = {}) {
  const rows = hub.job_cards || hub.operation_progress || []
  return rows.find((row) => positiveInt(row.id || row.job_card_id)) || {}
}

function actionParams(hub = {}, extra = {}) {
  const wo = hubWorkOrder(hub)
  const card = firstJobCard(hub)
  return productionContextParams({
    work_order_id: wo.id,
    job_card_id: card.id || card.job_card_id,
    running_item_id: wo.running_item_id,
    batch_id: wo.batch_id,
    ...extra,
  })
}

function stockActionParams(hub = {}, extra = {}) {
  const wo = hubWorkOrder(hub)
  return productionContextParams({
    work_order_id: wo.id,
    work_order_no: wo.work_order_no,
    batch_id: wo.batch_id,
    ...extra,
  })
}

export function buildExecutionHubActions(hub = {}) {
  const wo = hubWorkOrder(hub)
  return [
    { key: 'startProduction', label: '开始生产', view: 'workOrders', params: actionParams(hub) },
    { key: 'productionIssue', label: '生产领料', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'issue', return_source: 'work_order' }) },
    { key: 'productionSupplement', label: '补料', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'supplement', return_source: 'work_order' }) },
    { key: 'productionReturn', label: '退回未用原料', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'return', return_source: 'work_order' }) },
    { key: 'productionConsume', label: '记录生产消耗', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'consume', return_source: 'work_order' }) },
    { key: 'finishedReceipt', label: '完工入库', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'finish', return_source: 'work_order' }) },
    { key: 'openJobCard', label: '打开工序卡', view: 'jobCards', params: actionParams(hub) },
    { key: 'openQuality', label: '打开质检', view: 'qualityInspections', params: actionParams(hub, { reference_no: wo.work_order_no }) },
    { key: 'openCost', label: '成本', view: 'productionCosts', params: actionParams(hub) },
    { key: 'openLogs', label: '日志', view: 'produceLogs', params: actionParams(hub) },
  ]
}

export function executionHubTimelineFilters() {
  return [
    { key: 'all', label: '全部' },
    { key: 'operation', label: '工序' },
    { key: 'inventory', label: '库存' },
    { key: 'quality', label: '质检' },
    { key: 'cost', label: '成本' },
    { key: 'log', label: '日志' },
  ]
}

export function filterExecutionHubTimeline(rows = [], filter = 'all') {
  if (!filter || filter === 'all') return rows
  return rows.filter((row) => row.type === filter)
}

export function readinessBadgeTone(readiness = {}) {
  if (readiness.severity === 'blocked') return 'danger'
  if (readiness.severity === 'warning') return 'warning'
  if (readiness.can_start || readiness.can_complete || readiness.severity === 'ready') return 'success'
  return 'neutral'
}

export function buildExecutionHubFocus(params = {}) {
  if (params.focus === 'blocked') return { section: 'readiness' }
  if (params.focus === 'finished_receipt') return { section: 'finished_receipt' }
  if (positiveInt(params.job_card_id)) return { section: 'job_card', job_card_id: positiveInt(params.job_card_id) }
  if (params.focus) return { section: String(params.focus) }
  return { section: 'summary' }
}
