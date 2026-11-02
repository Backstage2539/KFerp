import { inventoryUnitWeightInGrams } from './production-execution-hub.js'

export function transferWeightFactor(unit) {
  const text = String(unit || '').trim().toLowerCase()
  return inventoryUnitWeightInGrams(text) || (['oz', 'ounce', 'ounces', '盎司'].includes(text) ? 28.349523125 : 0)
}

export function transferSpecs(rows, type, itemID) {
  const specs = new Map()
  for (const row of rows.filter(r => r.item_type === type && Number(r.item_id) === Number(itemID))) {
    if (type === 'material') {
      const unit = row.inventory_unit || ''
      specs.set(`material:${unit}`, { key: `material:${unit}`, label: unit || '默认规格', unit, mode: transferWeightFactor(unit) ? 'weight' : 'count' })
    } else if (Number(row.bom_spec_id) > 0) {
      const key = `bom:${row.bom_spec_id}:${row.bom_variant_id || 0}`
      const weight = transferWeightFactor(row.inventory_unit) > 0
      specs.set(key, { key, label: row.spec_name || '规格', unit: weight ? row.inventory_unit : '件', inventoryUnit: row.inventory_unit || '件', mode: weight ? 'canonical_weight' : 'count', bomSpecID: Number(row.bom_spec_id), variantID: Number(row.bom_variant_id || 0) })
    } else if (Number(row.spec_g) > 0) {
      const specG = Number(row.spec_g)
      if (Number(row.qty_units) > 0) specs.set(`legacy:${specG}:count`, { key: `legacy:${specG}:count`, label: `${specG}g`, unit: '件', mode: 'count', specG })
      if (Number(row.qty_g) > Number(row.qty_units || 0) * specG) specs.set(`legacy:${specG}:loose`, { key: `legacy:${specG}:loose`, label: `克（${specG}g库存散装）`, unit: '克', mode: 'loose', specG })
    }
  }
  return [...specs.values()]
}

export function transferBatches(rows, type, itemID, key) {
  return rows.filter(row => row.item_type === type && Number(row.item_id) === Number(itemID) && transferSpecs([row], type, itemID).some(s => s.key === key))
}

export function transferAmount(row, spec, kind = 'available') {
  const available = kind === 'available'
  if (spec.mode === 'loose') return { amount: available ? Number(row.available_g || 0) : Math.max(0, Number(row.qty_g || 0) - Number(row.qty_units || 0) * spec.specG), unit: '克' }
  if (spec.mode === 'weight') return { amount: Number((available ? row.available_g : row.qty_g) || 0) / transferWeightFactor(spec.unit), unit: spec.unit }
  return { amount: Number((available ? row.available_units : row.qty_units) || 0), unit: spec.unit }
}

export async function loadTransferOptions(warehouse, apiGet) {
  if (!warehouse) return []
  const rows = []
  let page = 1
  while (true) {
    const params = new URLSearchParams({ warehouse, page: String(page), limit: '500' })
    const data = await apiGet(`/api/stock/transfer-options?${params}`)
    rows.push(...(data.rows || []))
    if (!data.has_next) return rows
    page += 1
  }
}
