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

export function productionLogWeight(value) {
  if (value === null || value === undefined || value === '' || !Number.isFinite(Number(value))) return '未记录'
  const grams = Number(value)
  return Math.abs(grams) >= 1000 ? `${Number((grams / 1000).toFixed(3))} kg` : `${grams} g`
}

export function productionLogYield(row) {
  if (!(Number(row?.input_g) > 0) || row?.actual_yield_rate == null || !Number.isFinite(Number(row.actual_yield_rate))) return '待核对'
  return `${(Number(row.actual_yield_rate) * 100).toFixed(2)}%`
}

export function productionLogOutput(row) {
  const pieces = Number(row?.finished_units || 0)
  const loose = Number(row?.finished_loose_g || 0)
  if (productionLogNeedsReview(row)) return `${pieces}（历史单位待核对）`
  if (!pieces && !(Number(row?.spec_g) > 0)) return '散装产出'
  return `${pieces} 件${loose ? ` · 余料 ${productionLogWeight(loose)}` : ''}`
}

export function productionLogNeedsReview(row) {
  return Number(row?.finished_units) > 0 && !(Number(row?.spec_g) > 0) && !(Number(row?.finished_total_g) > 0)
}

export function productionLogMaterialQuantity(item) {
  if (Number(item?.deduct_units) > 0) return `${Number(item.deduct_units)} ${item.unit && !['g','kg'].includes(item.unit) ? item.unit : '件'}`
  return productionLogWeight(item?.deduct_g)
}

export function productionLogDefaultFilters() {
  return { from: '', to: '', q: '', product_id: 0, batch_id: '', running_item_id: 0, work_order_id: 0, operator: '', page: 1, limit: 20 }
}

export function productionLogFilters(params) {
  const result = productionLogDefaultFilters()
  for (const key of Object.keys(result)) {
    const value = typeof params?.get === 'function' ? params.get(key) : params?.[key]
    if (value != null) result[key] = typeof result[key] === 'number' ? Math.max(0, Number(value) || 0) : String(value).trim()
  }
  result.page = Math.max(1, result.page)
  result.limit = [10, 20, 50, 100].includes(result.limit) ? result.limit : 20
  return result
}

export function productionLogDateError(filters) {
  for (const value of [filters.from, filters.to]) {
    if (value && (!/^\d{4}-\d{2}-\d{2}$/.test(value) || !Number.isFinite(Date.parse(value)) || new Date(value).toISOString().slice(0, 10) !== value)) return '请选择有效的开始和结束日期'
  }
  return filters.from && filters.to && filters.from > filters.to ? '开始日期不能晚于结束日期' : ''
}
