export function issueSelectionState(tasks, selected) {
  const ids = tasks.filter(task => task.stock_action === 'issue').map(task => Number(task.job_card_id))
  const set = new Set(selected)
  const count = ids.filter(id => set.has(id)).length
  return { checked: ids.length > 0 && count === ids.length, indeterminate: count > 0 && count < ids.length, count, total: ids.length }
}
export function toggleIssueSelection(tasks, selected) {
  return issueSelectionState(tasks, selected).checked ? [] : tasks.filter(task => task.stock_action === 'issue').map(task => Number(task.job_card_id))
}
export function batchIssuePayload(preview, selected, quantities, requestId) {
  return { job_card_ids: preview.job_card_ids, preview_version: preview.version, request_id: requestId,
    groups: preview.groups.filter(group => selected.includes(group.key)).map(group => ({ key: group.key, version: group.version, quantity: Number(quantities[group.key]) })) }
}
export function batchIssueFactor(group) {
  if (group.quantity_basis !== 'weight') return 1
  return ({ kg: 1000, 公斤: 1000, 千克: 1000, t: 1000000, 吨: 1000000, 斤: 500 })[String(group.unit).toLowerCase()] || 1
}
export function batchIssueQuantity(group, base) { return Number((Number(base || 0) / batchIssueFactor(group)).toFixed(6)) }
export function batchIssueAllocations(group, quantity) {
  let left = Math.round(Number(quantity || 0) * batchIssueFactor(group))
  const rows = (group.allocations || []).map(row => {
    const actual = Math.min(Math.max(0, left), row.available)
    left -= actual
    return { ...row, actual, extra: 0 }
  })
  for (const row of rows) { row.extra = Math.min(Math.max(0, left), row.extra_available); row.actual += row.extra; left -= row.extra }
  return rows
}
export function batchIssueValidation(group, quantity) {
  const base = Number(quantity) * batchIssueFactor(group)
  if (!Number.isFinite(base) || base <= 0) return '请输入大于 0 的领用数量'
  if (Math.abs(base - Math.round(base)) > 0.00001) return '数量须精确到克或整数件'
  if (base > group.max_quantity) return '实际数量超过当前可领库存'
  return ''
}
