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
    groups: preview.groups.filter(group => selected.includes(group.key)).flatMap(group => {
      if (!group.posting_groups) return [{ key: group.key, version: group.version, quantity: Number(quantities[group.key]) }]
      const totals = new Map()
      for (const row of batchIssueAllocations(group, quantities[group.key])) totals.set(row.posting_key, (totals.get(row.posting_key) || 0) + row.actual)
      return group.posting_groups.filter(source => totals.get(source.key) > 0).map(source => ({ key: source.key, version: source.version, quantity: batchIssueQuantity(source, totals.get(source.key)) }))
    }) }
}
// The operator edits one material total; the API retains warehouse-level atomic groups.
export function mergeBatchIssueGroups(groups) {
  const materials = new Map()
  for (const group of groups) {
    const identity = group.item_id ? JSON.stringify([group.item_type, group.item_id, group.bom_spec_id, group.bom_variant_id, group.spec_g, group.owner_customer_id, group.unit, group.quantity_basis, group.to_warehouse]) : group.key
    if (!materials.has(identity)) materials.set(identity, [])
    materials.get(identity).push(group)
  }
  return [...materials.values()].map(sources => {
    if (sources.length === 1) return sources[0]
    const merged = { ...sources[0], key: `material:${sources.map(source => source.key).join(':')}`, from_warehouse: '', posting_groups: sources, allocations: [], source_warehouses: [...new Set(sources.map(source => source.from_warehouse))], reason: [...new Set(sources.map(source => source.reason).filter(Boolean))].join('；') }
    for (const field of ['required', 'wip', 'remaining', 'available', 'max_quantity']) merged[field] = sources.reduce((sum, source) => sum + Number(source[field] || 0), 0)
    merged.allocations = sources.flatMap(source => source.allocations.map(row => ({ ...row, posting_key: source.key, from_warehouse: source.from_warehouse })))
    merged.allocations.sort((a, b) => Number(b.priority || 0) - Number(a.priority || 0) || String(a.planned_start || '9999').localeCompare(String(b.planned_start || '9999')) || Number(a.job_card_id) - Number(b.job_card_id))
    return merged
  })
}
export function batchIssueFactor(group) {
  if (group.quantity_basis !== 'weight') return 1
  return ({ kg: 1000, 公斤: 1000, 千克: 1000, t: 1000000, 吨: 1000000, 斤: 500 })[String(group.unit).toLowerCase()] || 1
}
export function batchIssueQuantity(group, base) { return Number((Number(base || 0) / batchIssueFactor(group)).toFixed(6)) }
export function batchIssueAllocations(group, quantity) {
  let left = Math.round(Number(quantity || 0) * batchIssueFactor(group))
  const rows = (group.allocations || []).map(row => {
    const actual = Math.min(Math.max(0, left), Number(row.available || 0))
    left -= actual
    return { ...row, actual, extra: 0 }
  })
  for (const row of rows) { row.extra = Math.min(Math.max(0, left), Number(row.extra_available || 0)); row.actual += row.extra; left -= row.extra }
  return rows
}
export function batchIssueValidation(group, quantity) {
  const base = Number(quantity) * batchIssueFactor(group)
  if (!Number.isFinite(base) || base <= 0) return '请输入大于 0 的领用数量'
  if (Math.abs(base - Math.round(base)) > 0.00001) return '数量须精确到克或整数件'
  if (base > group.max_quantity) return '实际数量超过当前可领库存'
  return ''
}
