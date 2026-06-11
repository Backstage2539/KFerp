export function workOrderStatusOptions() {
  return [
    { value: '', label: '全部' },
    { value: 'draft', label: '草稿' },
    { value: 'released', label: '已下达' },
    { value: 'running', label: '生产中' },
    { value: 'completed', label: '已完成' },
    { value: 'cancelled', label: '已取消' },
  ]
}

export function canStartWorkOrder(row) {
  return Number(row?.id || 0) > 0 && String(row?.status || '').trim() === 'released'
}

export function workOrderStartEndpoint(row) {
  const id = Number(row?.id || 0)
  if (id <= 0) return ''
  return `/api/work-orders/${id}/start`
}
