export function stockEntryEndpoint() {
  return '/api/stock-entries'
}

export function workOrderCompleteEndpoint(row) {
  const id = Number(row?.id || 0)
  return id > 0 ? `/api/work-orders/${id}/complete` : ''
}

export function jobCardActionEndpoint(row, action) {
  const id = Number(row?.id || 0)
  const value = String(action || '').trim()
  if (id <= 0 || !['start', 'pause', 'resume', 'complete'].includes(value)) return ''
  return `/api/job-cards/${id}/${value}`
}

export function jobCardStatusOptions() {
  return [
    { value: '', label: '全部' },
    { value: 'pending', label: '待执行' },
    { value: 'ready', label: '可执行' },
    { value: 'running', label: '执行中' },
    { value: 'paused', label: '已暂停' },
    { value: 'completed', label: '已完成' },
    { value: 'cancelled', label: '已取消' },
  ]
}

export function jobCardStatusLabel(status) {
  return jobCardStatusOptions().find((item) => item.value === String(status || '').trim())?.label || status || '-'
}

export function workOrderStatusLabel(status) {
  return ({
    draft: '草稿',
    released: '未开工',
    running: '生产中',
    partially_completed: '部分完成',
    completed: '已完成',
    cancelled: '已取消',
  })[String(status || '').trim()] || status || '-'
}

export function stockEntryTypeOptions() {
  return [
    { value: 'material_issue_to_wip', label: '领料到WIP' },
    { value: 'wip_return', label: 'WIP退料' },
    { value: 'material_consume', label: '工单消耗' },
    { value: 'finished_receipt', label: '完工入库' },
    { value: 'scrap_loss', label: '报废/损耗' },
  ]
}

export function stockEntryTypeLabel(type) {
  return stockEntryTypeOptions().find((item) => item.value === String(type || '').trim())?.label || type || '-'
}

export function canRunJobCardAction(row, action) {
  if (Number(row?.id || 0) <= 0) return false
  const status = String(row?.status || '').trim()
  return ({
    start: ['pending', 'ready'],
    pause: ['running'],
    resume: ['paused'],
    complete: ['running', 'paused'],
  })[String(action || '').trim()]?.includes(status) || false
}

export function canCompleteWorkOrder(row) {
  const status = String(row?.status || '').trim()
  return Number(row?.id || 0) > 0 && Number(row?.running_item_id || 0) > 0 && ['running', 'partially_completed'].includes(status)
}

function numericValue(value) {
  if (value === '' || value === null || value === undefined) return 0
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

function metricsObject(value) {
  if (!value) return {}
  if (typeof value === 'object' && !Array.isArray(value)) return value
  if (typeof value !== 'string') return {}
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

export function buildJobCardActionPayload(draft) {
  const payload = {
    actual_input_qty: numericValue(draft?.actual_input_qty),
    actual_output_qty: numericValue(draft?.actual_output_qty),
    loss_reason: String(draft?.loss_reason || '').trim(),
    metrics_json: metricsObject(draft?.metrics_json),
  }
  const exceptionReason = String(draft?.exception_reason || '').trim()
  if (exceptionReason) payload.exception_reason = exceptionReason
  return payload
}
