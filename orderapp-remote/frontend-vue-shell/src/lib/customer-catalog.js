export function customerCatalogCustomerID(ownership, contextID = 0) {
  return Number(contextID || (String(ownership).startsWith('customer:') ? String(ownership).slice(9) : 0)) || 0
}
export function customerCatalogCopyPayload(customerID, mode, ids = []) {
  if (!(Number(customerID) > 0) || !['all', 'selected'].includes(mode)) throw new Error('请选择客户和复制范围')
  const productIDs = mode === 'all' ? [] : [...new Set(ids.map(Number).filter(id => id > 0))]
  if (mode === 'selected' && !productIDs.length) throw new Error('请选择商品')
  return { customer_id: Number(customerID), mode, product_ids: productIDs }
}
export function customerCatalogProjection(data = {}) {
  const nodes = data.nodes || []
  const groups = nodes.filter(n => !Number(n.source_item_id) && Number(n.source_group_id) > 0).map(root => {
    const rows = nodes.filter(n => Number(n.source_group_id) === Number(root.source_group_id) && Number(n.source_item_id) > 0)
    const items = rows.map(n => ({ id: Number(n.source_item_id), group_id: Number(n.source_group_id), parent_id: Number(n.parent_source_item_id || 0), name: n.name, code: n.code || '', active: true, sort_order: n.sort_order, customer_catalog_node_id: n.id, children: [] }))
    const byID = new Map(items.map(i => [i.id, i]))
    for (const item of items) if (byID.has(item.parent_id)) byID.get(item.parent_id).children.push(item)
    return { id: Number(root.source_group_id), name: root.name, code: root.code || '', active: true, sort_order: root.sort_order, customer_catalog_node_id: root.id, items: items.filter(i => !byID.has(i.parent_id)) }
  })
  return { groups, assignments: data.assignments || [], groupIDs: groups.map(g => g.id) }
}
export function customerCatalogProductRows(rows, references, customerID) {
  if (!(Number(customerID) > 0)) return rows
  const refs = new Map(references.filter(r => Number(r.customer_id) === Number(customerID) && r.active !== false).map(r => [Number(r.product_id), r]))
  return rows.map(row => {
    const ref = refs.get(Number(row.id)); const name = String(ref?.customer_display_name || '').trim()
    return name ? { ...row, canonical_name: row.canonical_name || row.name, name, customer_reference: ref } : row
  })
}
