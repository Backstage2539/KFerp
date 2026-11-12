export function isTransferWeight(source) {
  const product = ['product', 'finished_product'].includes(String(source?.component_type || ''))
  if (product && (Number(source?.demand_units || 0) > 0 || Number(source?.required_units || 0) > 0)) return false
  return Number(source?.demand_g || source?.required_g || 0) > 0 || ['kg', 'g', '公斤', '克', '千克'].includes(String(source?.unit || '').toLowerCase())
}
const weightUnit = isTransferWeight
export function transferDisplayUnit(source) {
  const product = ['product', 'finished_product'].includes(String(source?.component_type || ''))
  if (product && !isTransferWeight(source)) return '件'
  return source?.unit || (isTransferWeight(source) ? 'g' : '件')
}
export function transferRowIssue(source, row) {
  if (row?.unavailable_reason) return row.unavailable_reason
  if (!isTransferWeight(source) && ['product', 'finished_product'].includes(String(source?.component_type || '')) && Number(row?.available_units || 0) <= 0 && Number(row?.available_g || 0) > 0) return '该批次只有散装库存，不能按件调拨'
  return ''
}
const gramsPerUnit = (source) => ['kg', '公斤', '千克'].includes(String(source?.unit || '').toLowerCase()) ? 1000 : 1
const neat = (value) => Number(Number(value).toFixed(6))

// Remove only floating-point multiplication noise, never a fraction of a gram.
export function transferGrams(source, quantity) {
  const grams = Number(quantity) * gramsPerUnit(source)
  const rounded = Math.round(grams)
  const tolerance = Number.EPSILON * Math.max(1, Math.abs(grams)) * 4
  return Math.abs(grams - rounded) <= tolerance ? rounded : grams
}

export function remainingTransfer(source, pending = 0) {
  const weight = weightUnit(source)
  const factor = weight ? gramsPerUnit(source) : 1
  const demand = Number(weight ? (source?.demand_g ?? source?.required_g ?? 0) : (source?.demand_units ?? source?.required_units ?? 0))
  const required = Number(weight ? (source?.required_g ?? Math.max(0, demand - Number(source?.upstream_g || 0))) : (source?.required_units ?? Math.max(0, demand - Number(source?.upstream_units || 0))))
  const covered = Number(weight ? source?.wip_covered_g || 0 : source?.wip_covered_units || 0)
  const missing = Math.max(0, demand - covered)
  return { missing: neat(missing / factor), pending: neat(pending / factor), remaining: neat(Math.max(0, required - covered - pending) / factor), unit: transferDisplayUnit(source) }
}

export function transferSelections(source, warehouse, rows) {
  const weight = weightUnit(source)
  const chosen = rows.filter((row) => Number(row.quantity) !== 0 || !Number.isFinite(Number(row.quantity)))
  return {
    source_id: Number(source?.id || 0), warehouse,
    owner_customer_id: Number(chosen[0]?.owner_customer_id || 0),
    batches: chosen.map((row) => ({ batch_code: row.batch_code, qty_g: weight ? transferGrams(source, row.quantity) : 0, qty_units: weight ? 0 : Number(row.quantity) })),
    _rows: chosen,
  }
}

export function transferableQuantity(source, row) {
  return Number(weightUnit(source) ? (row?.transferable_g ?? row?.available_g ?? 0) : (row?.transferable_units ?? row?.available_units ?? 0))
}

export function validateTransferSelections(source, selection, pending = 0) {
  const rows = selection._rows || []
  if (!selection.warehouse) return '请选择来源仓库'
  if (!rows.length) return '请填写本次调拨数量'
  if (rows.some((row) => !Number.isFinite(Number(row.quantity)) || Number(row.quantity) <= 0)) return '调拨数量必须大于 0'
  if (rows.some((row) => Number(row.owner_customer_id || 0) !== selection.owner_customer_id)) return '同一调拨单请选择相同货主'
  const weight = weightUnit(source)
  for (let i = 0; i < rows.length; i++) {
    const qty = weight ? selection.batches[i].qty_g : selection.batches[i].qty_units
    const available = transferableQuantity(source, rows[i])
    if (!Number.isSafeInteger(qty)) return weight ? '数量须精确到克' : '计件数量必须为整数'
    if (transferRowIssue(source, rows[i])) return transferRowIssue(source, rows[i])
    if (!rows[i].batch_code || qty > available) return '调拨数量超过批次可用库存或批次不可用'
  }
  return ''
}

// The shortage is only a starting suggestion. A user can transfer more, up to
// the available quantity of the selected physical batches. Never preselect
// across owners because one document preserves a single owner.
export function suggestedTransferRows(source, rows, pending = 0) {
  const weight = weightUnit(source)
  const factor = weight ? gramsPerUnit(source) : 1
  let remaining = Math.round(remainingTransfer(source, pending).remaining * factor)
  let owner = null
  return rows.map((row) => {
    let quantity = 0
    const available = transferableQuantity(source, row)
    const rowOwner = Number(row.owner_customer_id || 0)
    if (remaining > 0 && Number.isSafeInteger(available) && available > 0 && row.batch_code && !transferRowIssue(source, row) && (owner === null || owner === rowOwner)) {
      owner = rowOwner
      const allocated = Math.min(remaining, available)
      quantity = neat(allocated / factor)
      remaining -= allocated
    }
    return { ...row, quantity }
  })
}

export function transferUnitAmount(source, g, units) {
  return neat(weightUnit(source) ? Number(g || 0) / gramsPerUnit(source) : Number(units || 0))
}

export function replaceTransferSelection(items, index, selection) {
  const { _rows, ...item } = selection
  return items.map((original, i) => i === index ? item : original)
}

export function pendingTransferAmount(documents, source) {
  return (documents || []).filter((doc) => doc.status === 'draft' && (!doc.source_id || Number(doc.source_id) === Number(source?.id)))
    .reduce((sum, doc) => sum + Number(weightUnit(source) ? doc.qty_g || 0 : doc.qty_units || 0), 0)
}

export function reservedAmount(source, row) {
  const field = weightUnit(source) ? 'reserved_g' : 'reserved_units'
  return row?.[field] == null ? null : transferUnitAmount(source, row.reserved_g, row.reserved_units)
}

export function transferQualityLabel(value) {
  return ({ pass: '合格', hold: '冻结', failed: '不合格', unchecked: '待检', pending: '待检' })[String(value || '').toLowerCase()] || value || '未提供'
}

export function transferDocumentStatus(value) {
  return ({ draft: '待仓库确认', submitted: '已确认', cancelled: '已取消' })[value] || value || '未知状态'
}
