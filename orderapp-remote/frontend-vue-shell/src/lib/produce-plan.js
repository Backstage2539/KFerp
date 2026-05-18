const WEIGHT_UNITS = new Set(['g', 'kg', '克', '千克'])

function normalizeRatioPct(value) {
  const ratio = Number(value || 0)
  if (ratio > 0 && ratio <= 1) return ratio * 100
  return ratio
}

export function normalizedYieldRate(value) {
  const rate = Number(value || 0)
  if (rate <= 0) return 0
  if (rate > 1) return rate / 100
  return rate
}

export function roastExpectedFinishedG(row) {
  const finalInputG = Number(row?.final_input_g || 0)
  const yieldRate = normalizedYieldRate(row?.yield_rate)
  if (finalInputG <= 0 || yieldRate <= 0) return 0
  return Math.round(finalInputG * yieldRate)
}

export function gramsToKgString(value, digits = 2) {
  const grams = Number(value || 0)
  if (grams <= 0) return '0'
  return (grams / 1000).toFixed(digits)
}

export function describeProducePlanRow(row) {
  if (!row || row.production_kind !== 'drip_bag') return []
  const labels = ['挂耳生产']
  const needBags = Number(row.need_bags || row.need_units || 0)
  if (needBags > 0) labels.push(`需求 ${needBags} 袋`)
  const componentShortage = Number(row.finished_product_component_shortage_g || row.upstream_shortage_g || 0)
  if (componentShortage > 0) labels.push(`熟豆组件缺口 ${componentShortage}g`)
  const upstreamDemand = Number(row.upstream_roast_demand_g || 0)
  if (upstreamDemand > 0) labels.push(`上游烘焙需求 ${upstreamDemand}g`)
  return labels
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

function toPositiveInteger(value, fallback = 1) {
  const n = Number(value)
  if (!Number.isFinite(n) || n < 1) return fallback
  return Math.round(n)
}

export function syncRoastPlanRow(row, patch = {}) {
  if (!row || typeof row !== 'object') return row

  if (Object.prototype.hasOwnProperty.call(patch, 'machine')) {
    row.machine = String(patch.machine || '').trim()
  }
  if (Object.prototype.hasOwnProperty.call(patch, 'batch_g')) {
    row.batch_g = patch.batch_g
  }
  if (Object.prototype.hasOwnProperty.call(patch, 'batch_count')) {
    row.batch_count = patch.batch_count
  }

  row.batch_g = toPositiveInteger(row.batch_g, 1)
  row.batch_count = toPositiveInteger(row.batch_count, 1)
  row.final_input_g = row.batch_g * row.batch_count
  return row
}

export function normalizeRoastPlans(roastPlans) {
  return (roastPlans || []).map((row) =>
    syncRoastPlanRow({
      ...row,
      machine: String(row?.machine || '').trim(),
      batch_g: Number(row?.batch_g || 0),
      batch_count: Number(row?.batch_count || 0),
      final_input_g: Number(row?.final_input_g || 0),
    }),
  )
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
