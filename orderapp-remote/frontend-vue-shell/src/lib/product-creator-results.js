const typeNames = { material: '物料', product: '商品', bom: 'BOM', spec: '规格' }

// Old runs retain per-node execution aliases; this read model has one card per
// real record, including when a record is both an output and a downstream input.
export function productCreatorResultRows(run) {
  const names = new Map((run.workflow?.nodes || []).map(node => [node.id, node.name || '业务步骤']))
  const createdBOMs = new Set()
  for (const [nodeID, entries] of Object.entries(run.business_results?.objects || {})) {
    for (const entry of Array.isArray(entries) ? entries : []) {
      if (entry.type === 'bom' && (entry.new || run.inputs?.[nodeID]?.action !== 'reuse')) createdBOMs.add(Number(entry.id))
    }
  }
  const rows = new Map()
  for (const [nodeID, entries] of Object.entries(run.business_results?.objects || {})) {
    for (const entry of Array.isArray(entries) ? entries : []) {
      if (!typeNames[entry.type] || !Number(entry.id)) continue
      const key = `${entry.type}:${entry.id}`
      const row = rows.get(key) || { ...entry, source_node_ids: [], bom_ids: [] }
      if (!row.source_node_ids.includes(nodeID)) row.source_node_ids.push(nodeID)
      if (entry.bom_id && !row.bom_ids.includes(entry.bom_id)) row.bom_ids.push(entry.bom_id)
      if (entry.code) row.code = entry.code
      const created = entry.new || (entry.type === 'bom' && createdBOMs.has(Number(entry.id))) || (entry.type === 'spec' && createdBOMs.has(Number(entry.bom_id)))
      row.origin = created || row.origin === 'created' ? 'created' : 'reused'
      rows.set(key, row)
    }
  }
  const result = Array.isArray(run.current_objects) ? run.current_objects : [...rows.values()]
  return result.map(row => ({ ...row, origin: row.origin || rows.get(`${row.type}:${row.id}`)?.origin || 'reused', key: `${row.type}:${row.id}`, typeName: typeNames[row.type], stepName: (row.source_node_ids || []).map(id => names.get(id) || '业务步骤').join('、') }))
}
