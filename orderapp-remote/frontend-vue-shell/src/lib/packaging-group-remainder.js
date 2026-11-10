export function groupRemainderSuggestion(item, qty) {
  return Math.max(0, Number(item.packaging_available_g || 0) - Math.round(Number(qty || 0) * Number(item.packaging_standard_g_per_piece || 0)))
}
export function validateGroupRemainders(items, allocations, values, confirmed, reason) {
  for (const allocation of allocations) {
    if (!(Number(allocation.qty) > 0)) continue
    const item = items.find(row => Number(row.job_card_id) === Number(allocation.job_card_id))
    const value = Number(values[allocation.job_card_id])
    if (!item || !confirmed[allocation.job_card_id]) return '请逐张工单称重确认余料'
    if (!Number.isInteger(value) || value < 0 || value > Number(item.packaging_available_g)) return '余料须为不超过可用数量的非负整数克'
    if (Math.abs(value - groupRemainderSuggestion(item, allocation.qty)) > 0.000001 && !String(reason || '').trim()) return '实际余料与预计不同，请填写原因'
  }
  return ''
}
