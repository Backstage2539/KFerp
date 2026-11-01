export async function loadTransferInventory(warehouse, apiGet) {
  if (!warehouse) return []
  const rows = []
  let page = 1
  while (true) {
    const params = new URLSearchParams({ warehouse, item_type: 'finished_product', page: String(page), limit: '500' })
    const data = await apiGet(`/api/stock/warehouse-inventory?${params}`)
    rows.push(...(data.rows || []))
    if (!data.has_next) return rows
    page += 1
  }
}

export function transferProductOptions(rows = []) {
  const products = new Map()
  for (const row of rows) {
    const id = Number(row.item_id || 0)
    if (id > 0 && !products.has(id)) products.set(id, { id, name: row.item_name || `#${id}` })
  }
  return [...products.values()]
}

export function transferProductSpecs(specs = [], rows = [], productID = 0) {
  const ids = new Set(rows.filter(row => Number(row.item_id) === Number(productID)).map(row => Number(row.bom_spec_id || 0)))
  return specs.filter(spec => ids.has(Number(spec.bom_spec_id)))
}
