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

export function canCancelWorkOrder(row) {
  return Number(row?.id || 0) > 0 && String(row?.status || '').trim() === 'released' && Number(row?.running_item_id || 0) <= 0
}

export function workOrderPlannedOutput(row = {}) {
  const explicitUnits = Math.max(0, Math.floor(Number(row.planned_units || 0)))
  const explicitLooseG = Math.max(0, Math.round(Number(row.planned_loose_g || 0)))
  if (explicitUnits > 0 || explicitLooseG > 0) {
    return { units: explicitUnits, loose_g: explicitLooseG }
  }
  const plannedG = Math.max(0, Math.round(Number(row.planned_output_g || row.finished_g || row.planned_g || 0)))
  const specG = Math.max(0, Math.round(Number(row.spec_g || 0)))
  if (plannedG <= 0 || specG <= 0) return { units: 0, loose_g: 0 }
  return {
    units: Math.floor(plannedG / specG),
    loose_g: plannedG % specG,
  }
}

export function formatWorkOrderPlannedOutput(row = {}) {
  const output = workOrderPlannedOutput(row)
  return `${output.units} 袋 + ${output.loose_g}g`
}

export function workOrderOutputIdentity(row = {}) {
  const type = String(row.output_type || '').trim().toLowerCase() === 'material' || (!row.output_type && Number(row.output_material_id || 0) > 0)
    ? 'material'
    : 'product'
  return {
    type,
    id: Number(row.output_id || (type === 'material' ? row.output_material_id : row.output_product_id || row.product_id) || 0),
    name: String(row.output_name || (type === 'material' ? row.output_material_name : row.output_product_name || row.product_name) || '').trim(),
    qty: Number(row.output_qty ?? row.planned_output_qty ?? row.planned_inventory_qty ?? 0),
    unit: String(row.output_unit || row.inventory_unit || '').trim(),
  }
}

export function formatWorkOrderTypedOutput(row = {}) {
  const output = workOrderOutputIdentity(row)
  const typeLabel = output.type === 'material' ? '物料' : '商品'
  const name = output.name || (output.id > 0 ? `#${output.id}` : '-')
  const quantity = output.qty > 0 ? ` · ${output.qty.toLocaleString('zh-CN', { maximumFractionDigits: 3 })} ${output.unit || ''}`.trimEnd() : ''
  return `${typeLabel} · ${name}${quantity}`
}

export function workOrderUpstreamBlockers(row = {}) {
  const rows = row.upstream_blockers || row.upstream_dependencies || row.blocked_by_work_orders || []
  if (Array.isArray(rows) && rows.length) return rows
  const ids = row.upstream_work_order_ids || []
  return Array.isArray(ids) ? ids.map((id) => ({ work_order_id: Number(id || 0) })) : []
}

export function workOrderHasUpstreamBlocker(row = {}) {
  return Boolean(row.upstream_blocked || row.has_unfinished_dependencies || workOrderUpstreamBlockers(row).length)
}

export function workOrderUpstreamBlockerLabel(row = {}) {
  const blocker = workOrderUpstreamBlockers(row)[0]
  if (!blocker) return workOrderHasUpstreamBlocker(row) ? String(row.dependency_blocking_reason || row.upstream_blocking_reason || '等待上游工单') : '无阻塞'
  if (row.dependency_blocking_reason && !blocker.work_order_no && !blocker.output_name) return String(row.dependency_blocking_reason)
  const no = String(blocker.work_order_no || blocker.no || '').trim() || `工单 #${Number(blocker.work_order_id || blocker.id || 0)}`
  const name = String(blocker.output_name || blocker.product_name || blocker.material_name || '').trim()
  return `等待 ${no}${name ? ` · ${name}` : ''}`
}

export function workOrderStartEndpoint(row) {
  const id = Number(row?.id || 0)
  if (id <= 0) return ''
  return `/api/produce/work-orders/${id}/start`
}

export function workOrderCancelEndpoint(row) {
  const id = Number(row?.id || row?.work_order_id || 0)
  if (id <= 0) return ''
  return `/api/produce/work-orders/${id}/cancel`
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
