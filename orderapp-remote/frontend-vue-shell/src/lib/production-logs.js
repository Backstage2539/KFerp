export function parseProductionMaterialSummary(raw) {
  if (Array.isArray(raw)) return raw
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

export function productionLogMaterialBatchCodes(raw) {
  const seen = new Set()
  const out = []
  for (const item of parseProductionMaterialSummary(raw)) {
    const code = String(item?.batch_code || item?.material_batch_code || '').trim()
    if (!code || seen.has(code)) continue
    seen.add(code)
    out.push(code)
  }
  return out
}

export function productionLogMaterialSummaryText(raw) {
  if (!raw) return ''
  const items = parseProductionMaterialSummary(raw)
  if (!items.length) return String(raw)
  return items.map((item) => {
    const name = item.material_name || item.name || `物料${item.material_id || ''}`
    const unit = item.unit || ''
    const qty = Number(item.deduct_units || 0) > 0 ? item.deduct_units : item.deduct_g
    const batch = String(item.batch_code || item.material_batch_code || '').trim()
    return [batch ? `${name}(${batch})` : name, `${qty}${unit}`].join(': ')
  }).join('\n')
}
