function parseObject(value, fallback) {
  if (value && typeof value === 'object') return value
  try { return JSON.parse(value || '') } catch { return fallback }
}

export function operationExecutionStatus(row = {}) {
  const status = String(row.status || '').trim().toLowerCase()
  if (status === 'paused') return '已暂停'
  if (status === 'running') return '执行中'
  return row.status_label || ({ pending: '待处理', ready: '可开始', released: '待执行', completed: '已完成', cancelled: '已取消' }[status]) || row.status || '待处理'
}

export function frozenWorkOrderDetails(row = {}) {
  const route = parseObject(row.process_snapshot_json, {})
  const materials = parseObject(row.material_snapshot, [])
  const rawYield = route && Object.hasOwn(route, 'yield_rate') ? Number(route.yield_rate) : NaN
  const yieldRate = rawYield > 1 && rawYield <= 100 ? rawYield / 100 : rawYield
  return {
    bomVersion: Number(row.frozen_bom_version_id) > 0 ? Number(row.frozen_bom_version_id) : null,
    bomVersionLabel: typeof route.bom_version_no === 'string' && route.bom_version_no.trim() ? route.bom_version_no.trim() : (Number(row.frozen_bom_version_id) > 0 ? `#${Number(row.frozen_bom_version_id)}` : '未记录'),
    lossRate: Number.isFinite(yieldRate) && yieldRate > 0 && yieldRate <= 1 ? Math.round((1 - yieldRate) * 100000) / 1000 : null,
    materials: Array.isArray(materials) ? materials : [],
    orders: Array.isArray(row.related_orders) ? row.related_orders : [],
  }
}

export function workstationTodoTarget(reason = {}, operations = [], workOrderID = 0) {
  const link = (reason.related_links || []).find(item => item.view === 'workstationView')
  const requested = Number(link?.params?.job_card_id || 0)
  const row = requested ? operations.find(item => Number(item.job_card_id) === requested) : operations.find(item => !['completed','cancelled'].includes(item.status))
  if (!row) return null
  return {work_order_id:Number(link?.params?.work_order_id || row.work_order_id || workOrderID || 0),job_card_id:Number(row.job_card_id),workstation:row.workstation || '',workstation_id:Number(row.workstation_id || 0),station_id:Number(row.workstation_id || 0),focus:'workstation_task'}
}

export function executionBatchProgress(data = {}) {
  if (!Array.isArray(data.batches) || !data.batches.length) return null
  return {completed:data.batches.filter(row => row.status === 'completed').length,total:data.batches.length}
}

export function workOrderCostDetails(cost = {}, cards = [], startedMaterialCost = null, completion = {}) {
  const actualAvailable = Number(cost.id || 0) > 0 && !cost.actual_cost_unavailable
  const plannedAvailable = cards.length > 0 && cards.every(card => !card.planned_cost_unavailable && card.planned_operation_cost != null)
  return {
    material:startedMaterialCost != null ? Number(startedMaterialCost) : null,
    actualOperation:actualAvailable ? Number(cost.operation_cost) : null,
    accumulated:actualAvailable ? Number(cost.total_cost) : null,
    final:actualAvailable && completion.status === 'completed' && completion.receipt_complete === true ? Number(cost.total_cost) : null,
    plannedOperation:plannedAvailable ? cards.reduce((sum, card) => sum + Number(card.planned_operation_cost), 0) : null,
  }
}

export function snapshotMaterialQuantity(row = {}) {
  const n = Number(row.qty_per_unit)
  if (!(n > 0)) return Number(row.ratio_pct) > 0 ? `${Number(row.ratio_pct)}%` : '用量未记录'
  const type = String(row.consume_unit || '').trim().toLowerCase()
  const perUnit = { g_per_bag:'g / 袋', unit_per_bag:'个 / 袋', unit_per_box:'个 / 盒' }
  if (perUnit[type]) return `${n} ${perUnit[type]}`
  const unit = {g:'g', gram:'g', grams:'g', '克':'g', kg:'kg', kilogram:'kg', kilograms:'kg', '千克':'kg', '公斤':'kg', lb:'lb', lbs:'lb', pound:'lb', pounds:'lb', '磅':'lb', oz:'oz', ounce:'oz', ounces:'oz', '盎司':'oz', unit:'个', units:'个', '个':'个', '件':'件', '袋':'袋', '盒':'盒'}[type]
  if (!unit) return '用量单位未记录'
  const outputQty = Number(row.output_qty)
  const outputUnit = String(row.output_unit || '').trim()
  return outputQty > 0 && outputUnit ? `${n} ${unit} / ${outputQty} ${outputUnit}` : `${n} ${unit}（产出基准未记录）`
}
