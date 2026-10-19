export function logisticsWeightDraft(grams) {
  return { logistics_weight_qty: grams == null || grams === '' ? '' : Number(grams) / 1000, logistics_weight_unit: 'kg' }
}

export function logisticsWeightGrams(variant = {}) {
  if (!Object.hasOwn(variant, 'logistics_weight_qty')) {
    if (variant.logistics_weight_g == null || variant.logistics_weight_g === '') return null
    const grams = Number(variant.logistics_weight_g)
    if (!Number.isFinite(grams) || grams < 0.000001 || grams > 1e9) throw new Error('物流重量须在 0.000001g 至 1000000000g 之间')
    return grams
  }
  if (variant.logistics_weight_qty == null || variant.logistics_weight_qty === '') return null
  const quantity = Number(variant.logistics_weight_qty)
  const unit = variant.logistics_weight_unit || 'kg'
  if (!['kg', 'g'].includes(unit) || !Number.isFinite(quantity) || quantity <= 0) throw new Error('物流重量必须填写有效的正数和 KG / G 单位')
  const rawGrams = quantity * (unit === 'kg' ? 1000 : 1)
  if (rawGrams < 0.000001 || rawGrams > 1e9) throw new Error('物流重量须在 0.000001g 至 1000000000g 之间')
  const grams = Number(rawGrams.toFixed(6))
  if (grams <= 0) throw new Error('物流重量至少为 0.000001g')
  return grams
}

export function validateLogisticsWeights(variants = []) {
  for (const variant of variants) {
    if (logisticsWeightGrams(variant) == null) throw new Error(`${variant.name || '商品规格'}：请填写物流重量后再发布`)
  }
}

export function formatWeightG(value, unknown = '未记录') {
  if (value == null || value === '' || !Number.isFinite(Number(value)) || Number(value) < 0) return unknown
  return `${Number(Number(value).toFixed(6))}g`
}

export function orderWeightLines(item = {}) {
  let recorded = false
  try {
    const snapshot = JSON.parse(item.weight_snapshot_json || '{}')
    recorded = Boolean(snapshot && Object.keys(snapshot).length && !snapshot.historical_unknown)
  } catch { /* Historical orders can have no usable weight snapshot. */ }
  return [
    ['用料重量（参考）', item.material_unit_weight_g, item.material_item_weight_g, recorded ? '未计算' : '未记录'],
    ['物流重量', item.logistics_unit_weight_g, item.logistics_item_weight_g, recorded ? '未配置' : '未记录'],
  ].map(([label, unit, total, unknown]) => ({ label, value: unit == null && total == null ? unknown : `单件 ${formatWeightG(unit, unknown)} · 本项 ${formatWeightG(total, unknown)}` }))
}
