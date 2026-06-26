const contextKeys = ['tab', 'work_order_id', 'job_card_id', 'running_item_id', 'material_id', 'shortage_g', 'reference_no', 'focus', 'batch_id']

function positiveInt(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0
}

function cleanValue(key, value) {
  if (['work_order_id', 'job_card_id', 'running_item_id', 'material_id', 'shortage_g'].includes(key)) return positiveInt(value)
  return String(value || '').trim()
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

function firstRelatedLink(hub = {}, key) {
  const links = hub.readiness?.related_links || []
  return links.find((link) => link.key === key || link.view === key) || null
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

export function buildExecutionHubActions(hub = {}) {
  const wo = hubWorkOrder(hub)
  const wipLink = firstRelatedLink(hub, 'wip') || firstRelatedLink(hub, 'stockOperations')
  return [
    { key: 'startProduction', label: '开始生产', view: 'workOrders', params: actionParams(hub) },
    { key: 'openWipIssue', label: '打开/创建 WIP 领料', view: 'stockOperations', params: actionParams(hub, { tab: 'wip', ...(wipLink?.params || {}) }) },
    { key: 'openJobCard', label: '打开工序卡', view: 'jobCards', params: actionParams(hub) },
    { key: 'openQuality', label: '打开质检', view: 'qualityInspections', params: actionParams(hub, { reference_no: wo.work_order_no }) },
    { key: 'finishedReceipt', label: '完工入库', view: 'workOrders', params: actionParams(hub, { focus: 'finished_receipt' }) },
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
