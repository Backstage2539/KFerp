export const stockBatchKey = (item, batch) => `${item.key}|${batch.warehouse}|${batch.batch_id}`

export function initialStockSelection(items) {
  return Object.fromEntries((items || []).flatMap(item => (item.batches || []).map(batch => [stockBatchKey(item, batch), Number(batch.selected_qty || 0)])))
}

export function stockSelectionPayload(items, values) {
  return (items || []).flatMap(item => (item.batches || []).map(batch => ({ key: item.key, batch_id: Number(batch.batch_id), warehouse: batch.warehouse, qty: Number(values[stockBatchKey(item, batch)] || 0) })).filter(batch => batch.qty > 0))
}

export function stockSelectionError(items, values) {
  for (const item of items || []) {
    let total = 0
    for (const batch of item.batches || []) {
      const raw = values[stockBatchKey(item, batch)]
      const quantity = raw === '' || raw == null ? 0 : Number(raw)
      if (!Number.isFinite(quantity) || quantity < 0) return `${item.name}：请填写有效的使用数量。`
      if (quantity > Number(batch.available_qty || 0) + 1e-9) return `${item.name}：使用数量不能超过批次可用量。`
      total += quantity
    }
    if (total > Number(item.required_qty || 0) + 1e-9) return `${item.name}：使用数量不能超过总需求。`
  }
  return ''
}
