export function workOrderStatusOptions() {
  return [
    { value: '', label: '全部' },
    { value: 'draft', label: '草稿' },
    { value: 'released', label: '未开工' },
    { value: 'running', label: '生产中' },
    { value: 'partially_completed', label: '部分完成' },
    { value: 'completed', label: '已完成' },
    { value: 'cancelled', label: '已取消' },
  ]
}

export function canStartWorkOrder(row) {
  return Number(row?.id || 0) > 0 && String(row?.status || '').trim() === 'released'
}

export function canEditWorkOrderSplits(row) {
  return Number(row?.id || 0) > 0 && String(row?.status || '').trim() === 'released' && Number(row?.running_item_id || 0) <= 0
}

export function workOrderStartEndpoint(row) {
  const id = Number(row?.id || 0)
  if (id <= 0) return ''
  return `/api/work-orders/${id}/start`
}

export function workOrderOperationSplitsEndpoint(row) {
  const id = Number(row?.id || 0)
  if (id <= 0) return ''
  return `/api/work-orders/${id}/operation-splits`
}

export function buildWorkOrderOperationSplitPayload(rows = []) {
  const items = (rows || [])
    .map((row) => ({
      operation_seq: Math.max(0, Math.round(Number(row.operation_seq || row.sequence_no || 0))),
      operation_id: Number(row.operation_id || 0),
      operation: String(row.operation || '').trim(),
      workstation_capacity_id: Number(row.workstation_capacity_id || 0),
      planned_qty: Number(row.planned_qty || 0),
      note: String(row.note || '').trim(),
    }))
    .filter((row) => row.workstation_capacity_id > 0 && row.planned_qty > 0)
  return { items }
}
