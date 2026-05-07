const WEIGHT_UNITS = new Set(['g', 'kg', '克', '千克'])

function normalizeRatioPct(value) {
  const ratio = Number(value || 0)
  if (ratio > 0 && ratio <= 1) return ratio * 100
  return ratio
}

export function producePlanKey(productId, specG) {
  return `${productId}-${specG}`
}

function defaultSelectionKey(row) {
  return producePlanKey(row.product_id, row.spec_g)
}

export function insufficientSelectionState(rows, selected, keyForRow = defaultSelectionKey) {
  const keys = (rows || []).map(keyForRow)
  const selectedCount = keys.filter((key) => !!selected?.[key]).length
  const total = keys.length
  return {
    checked: total > 0 && selectedCount === total,
    indeterminate: selectedCount > 0 && selectedCount < total,
    selectedCount,
    total,
  }
}

export function buildInsufficientSelection(rows, checked, keyForRow = defaultSelectionKey) {
  if (!checked) return {}
  return Object.fromEntries((rows || []).map((row) => [keyForRow(row), true]))
}

export function buildFinalInputMap(roastPlans) {
  const out = {}
  for (const row of roastPlans || []) {
    out[row.key] = Number(row.final_input_g || 0)
  }
  return out
}

export function rebuildPlanRows(planRows, roastPlans) {
  const finalInputByKey = buildFinalInputMap(roastPlans)
  return (planRows || []).map((row) => ({
    ...row,
    input_g: finalInputByKey[producePlanKey(row.product_id, row.spec_g)] || Number(row.input_g || 0),
  }))
}

export function buildMaterialSummary(planRows, roastPlans, materialRatios, initialMaterials) {
  const out = new Map()
  const dynamicNames = new Set()
  const availabilityByName = new Map()

  for (const item of initialMaterials || []) {
    availabilityByName.set(`${item.name}::${String(item.unit || '').trim().toLowerCase()}`, item)
  }

  for (const ratio of materialRatios || []) {
    if (WEIGHT_UNITS.has(String(ratio.material_unit || '').trim().toLowerCase())) {
      dynamicNames.add(ratio.material_name)
    }
  }

  for (const item of initialMaterials || []) {
    if (dynamicNames.has(item.name)) continue
    out.set(`${item.name}::${item.unit}`, { ...item, qty: Number(item.qty || 0) })
  }

  const roastByKey = new Map((roastPlans || []).map((row) => [row.key, row]))
  for (const ratio of materialRatios || []) {
    const unit = String(ratio.material_unit || '').trim() || 'g'
    const normalized = unit.toLowerCase()
    if (!WEIGHT_UNITS.has(normalized)) continue

    const roast = roastByKey.get(ratio.key)
    const finalInputG = Number(roast?.final_input_g || 0)
    if (finalInputG <= 0) continue

    let qty = 0
    if (normalized === 'kg' || normalized === '千克') {
      qty = Math.ceil((finalInputG * normalizeRatioPct(ratio.ratio_pct)) / 100 / 1000)
    } else {
      qty = Math.ceil((finalInputG * normalizeRatioPct(ratio.ratio_pct)) / 100)
    }
    if (qty <= 0) continue

    const mapKey = `${ratio.material_name}::${unit}`
    const initial = availabilityByName.get(`${ratio.material_name}::${normalized}`) || {}
    const existing = out.get(mapKey) || {
      ...initial,
      name: ratio.material_name,
      unit,
      qty: 0,
    }
    existing.qty += qty
    out.set(mapKey, existing)
  }

  return [...out.values()].sort((a, b) => {
    if (a.unit !== b.unit) return String(a.unit).localeCompare(String(b.unit), 'zh-CN')
    return String(a.name).localeCompare(String(b.name), 'zh-CN')
  })
}

export function buildStartPayload(filters, selectedKeys, roastPlans, planRows) {
  const finalInputByKey = buildFinalInputMap(roastPlans)
  const fallbackByKey = Object.fromEntries((planRows || []).map((row) => [producePlanKey(row.product_id, row.spec_g), Number(row.input_g || 0)]))
  const input_by_key = {}
  for (const key of selectedKeys) {
    input_by_key[key] = finalInputByKey[key] || fallbackByKey[key] || 0
  }
  return {
    from: filters.from || '',
    to: filters.to || '',
    customer_id: Number(filters.customer_id || 0),
    selected: selectedKeys,
    input_by_key,
  }
}
