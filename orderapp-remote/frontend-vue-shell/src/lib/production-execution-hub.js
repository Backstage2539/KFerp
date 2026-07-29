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
  const qtyUnits = Number(item.qty_units || 0)
  const qtyG = Number(item.qty_g || 0)
  if (qtyUnits > 0 && qtyG <= 0) return true
  if (qtyG > 0 && qtyUnits <= 0) return false
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

export function stockDocumentPositiveItems(items = [], canonicalize = stockCanonicalQuantity) {
  const requestItems = items
    .map((item) => ({ item, quantity: canonicalize(item) }))
    .filter(({ quantity }) => Number(quantity.qty_g || 0) > 0 || Number(quantity.qty_units || 0) > 0)
  if (!requestItems.length) throw new Error('至少填写一个大于 0 的领用数量')
  return requestItems
}

export function productionStockDocumentPreviewAction(row = {}) {
  const purpose = String(row.purpose || row.entry_type || '').trim()
  if (row.is_return || purpose === 'material_return_from_manufacture') return 'return'
  if (purpose === 'material_transfer_for_manufacture') return 'issue'
  if (purpose === 'material_consumption_for_manufacture') return 'consume'
  if (purpose === 'manufacture') return 'finish'
  return ''
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
  const readiness = hub.readiness || {}
  const startBlocked = readiness.can_start === false
  const startReason = startBlocked
    ? (readiness.blocking_reasons || []).map((row) => row?.label).find(Boolean) || '当前状态不可开始生产'
    : ''
  return [
    {
      key: 'startProduction',
      label: '开始生产',
      action_type: 'command',
      endpoint: wo.id ? `/api/produce/work-orders/${wo.id}/start` : '',
      params: actionParams(hub),
      disabled: startBlocked || !wo.id,
      reason: startReason,
    },
    { key: 'productionIssue', label: '生产领料', action_type: 'navigate', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'issue', return_source: 'work_order' }) },
    { key: 'productionSupplement', label: '补料', action_type: 'navigate', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'supplement', return_source: 'work_order' }) },
    { key: 'productionReturn', label: '退回未用原料', action_type: 'navigate', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'return', return_source: 'work_order' }) },
    { key: 'productionConsume', label: '记录生产消耗', action_type: 'navigate', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'consume', return_source: 'work_order' }) },
    { key: 'finishedReceipt', label: '完工入库', action_type: 'navigate', view: 'stockOperations', params: stockActionParams(hub, { tab: 'stockEntries', action: 'finish', return_source: 'work_order' }) },
    { key: 'openJobCard', label: '打开工序卡', action_type: 'navigate', view: 'jobCards', params: actionParams(hub) },
    { key: 'openQuality', label: '打开质检', action_type: 'navigate', view: 'qualityInspections', params: actionParams(hub, { reference_no: wo.work_order_no }) },
    { key: 'openCost', label: '成本', action_type: 'navigate', view: 'productionCosts', params: actionParams(hub) },
    { key: 'openLogs', label: '日志', action_type: 'navigate', view: 'produceLogs', params: actionParams(hub) },
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

export function executionHubCommandErrorMessage(error, action = {}) {
  const raw = String(error?.message || '').trim()
  const normalized = raw.toLowerCase()
  const label = String(action?.label || '操作').trim() || '操作'
  if (
    normalized.includes('permission denied')
    || normalized.includes('forbidden')
    || normalized.includes('unauthorized')
  ) {
    return '当前账号没有执行此操作的权限，请联系管理员'
  }
  if (normalized.includes('wip') && (normalized.includes('insufficient') || normalized.includes('shortage'))) {
    return 'WIP库存不足，补足物料后再开始生产'
  }
  if (normalized.includes('not found')) return '生产工单不存在或已失效，请刷新后重试'
  if (normalized.includes('work order must be released')) return '工单必须先下达后才能开始生产'
  if (
    normalized.includes('work order must be running')
    || normalized.includes('work order is not running')
  ) {
    return '工单尚未开始生产，请先从执行枢纽开始生产'
  }
  if (
    normalized.includes('actual input')
    || normalized.includes('actual output')
    || normalized.includes('input/output')
    || normalized.includes('input and output')
  ) {
    return '实际投入或实际产出数量不正确，请检查后重试'
  }
  if (normalized.includes('quantity') || normalized.includes('qty')) {
    return '数量不正确，请检查后重试'
  }
  if (
    normalized.includes('status')
    || normalized.includes('already started')
    || normalized.includes('cannot start')
  ) {
    return '当前工单状态不允许开始生产，请刷新后按最新状态操作'
  }
  return `${label}失败，请稍后重试；如持续失败请联系管理员`
}

export function buildExecutionHubFocus(params = {}) {
  if (params.focus === 'blocked') return { section: 'readiness' }
  if (params.focus === 'finished_receipt') return { section: 'finished_receipt' }
  if (positiveInt(params.job_card_id)) return { section: 'job_card', job_card_id: positiveInt(params.job_card_id) }
  if (params.focus) return { section: String(params.focus) }
  return { section: 'summary' }
}
