function amount(value) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

export function finishedReceiptGroupingKey(row = {}) {
  const outputType = String(row.output_type || 'product').trim().toLowerCase()
  const outputID = outputType === 'material'
    ? Number(row.output_material_id || 0) || `work-order:${Number(row.work_order_id || 0)}`
    : Number(row.output_product_id || row.product_id || 0) || `work-order:${Number(row.work_order_id || 0)}`
  return JSON.stringify([
    outputType,
    outputID,
    Number(row.bom_spec_id || 0),
    Number(row.spec_g || 0),
    String(row.output_unit || '').trim().toLowerCase(),
    Number(row.inventory_owner_customer_id ?? row.customer_id ?? 0),
    String(row.package_spec_label || '').trim(),
    String(row.target_warehouse || '').trim(),
  ])
}

export function groupFinishedReceiptTasks(rows = [], warehouses = []) {
  const warehouseByCode = new Map(warehouses.map((warehouse) => [String(warehouse?.code || ''), warehouse]))
  const grouped = new Map()
  for (const row of rows) {
    const key = finishedReceiptGroupingKey(row)
    const warehouseCode = String(row.target_warehouse || '').trim()
    const warehouse = warehouseByCode.get(warehouseCode)
    let group = grouped.get(key)
    if (!group) {
      group = {
        ...row,
        key,
        rows: [],
        reported_qty: 0,
        received_qty: 0,
        available_qty: 0,
        processable_qty: 0,
        blocked_count: 0,
        warehouse_name: row.target_warehouse_name || warehouse?.name || (warehouseCode ? '仓库名称未加载' : '按工单冻结仓库'),
      }
      grouped.set(key, group)
    }
    group.rows.push(row)
    group.reported_qty += amount(row.reported_qty)
    group.received_qty += amount(row.received_qty)
    group.available_qty += amount(row.available_qty)
    if (row.quality_blocked || row.blocked_reason) group.blocked_count += 1
    else group.processable_qty += amount(row.available_qty)
  }
  return [...grouped.values()]
    .map((group) => ({ ...group, rows: [...group.rows].sort((a, b) => String(a.work_order_no || '').localeCompare(String(b.work_order_no || ''), 'zh-CN')) }))
    .sort((a, b) => String(a.output_name || '').localeCompare(String(b.output_name || ''), 'zh-CN')
      || Number(a.spec_g || 0) - Number(b.spec_g || 0)
      || String(a.warehouse_name || '').localeCompare(String(b.warehouse_name || ''), 'zh-CN'))
}

export function finishedReceiptBatchPlan(group = {}, requestedQuantity = group.processable_qty) {
  let remaining = amount(requestedQuantity)
  const plan = []
  for (const row of group.rows || []) {
    if (row.quality_blocked || row.blocked_reason || remaining <= 0) continue
    const quantity = Math.min(amount(row.available_qty), remaining)
    if (quantity <= 0) continue
    plan.push({ work_order_id: Number(row.work_order_id), work_order_no: String(row.work_order_no || ''), quantity })
    remaining -= quantity
  }
  return plan
}

export function completionNoteForWorkOrder(row = {}) {
  return `合并入库，来源工单：${String(row.work_order_no || row.work_order_id || '').trim() || '未知工单'}`
}

export async function executeFinishedReceiptBatch(group, requestedQuantity, submit) {
  const succeeded = []
  const failed = []
  for (const item of finishedReceiptBatchPlan(group, requestedQuantity)) {
    const row = (group.rows || []).find((source) => Number(source.work_order_id) === item.work_order_id)
    if (!row) continue
    try {
      const result = await submit(row, item)
      succeeded.push({ ...item, result })
    } catch (error) {
      failed.push({ ...item, reason: error?.message || '入库失败' })
    }
  }
  return { succeeded, failed }
}

export function receiptCompletionMode(row, quantity) {
  return Number(quantity) < Number(row.remaining_plan_qty ?? row.available_qty) ? 'partial' : 'final'
}

export function receiptSpecLabel(row) {
  return row.package_spec_label || (Number(row.spec_g) > 0 ? `${row.spec_g}g/${row.output_unit || '袋'}` : (row.output_type === 'material' ? row.output_unit : '规格未确认'))
}

// Retain the original request body after an uncertain response; successful rows are never resent.
export function createReceiptAttempt(group, quantity, payload) {
  return { unit: group.output_unit, items: finishedReceiptBatchPlan(group, quantity).map(item => {
    const row = group.rows.find(source => Number(source.work_order_id) === item.work_order_id)
    return { ...item, row: { ...row }, payload: payload(row, item), succeeded: false, reason: '', result: null }
  }) }
}
export async function runReceiptAttempt(attempt, submit) {
  for (const item of attempt.items) {
    if (item.succeeded) continue
    try { item.result = await submit(item.row, item.payload); item.succeeded = true; item.reason = '' }
    catch (error) { item.reason = error?.message || '入库结果未确认，请重试原请求' }
  }
  return attempt
}
