export type OrderWeightSnapshot = {
  material_unit_weight_g?: number | string | null
  material_item_weight_g?: number | string | null
  logistics_unit_weight_g?: number | string | null
  logistics_item_weight_g?: number | string | null
  weight_snapshot_json?: string
}

function formatWeight(value: number | string | null | undefined, unknown: string): string {
  if (value == null || value === '' || !Number.isFinite(Number(value)) || Number(value) < 0) return unknown
  return `${Number(Number(value).toFixed(6))}g`
}

export function orderWeightLines(item: OrderWeightSnapshot = {}): Array<{ label: string; value: string }> {
  let recorded = false
  try {
    const snapshot = JSON.parse(item.weight_snapshot_json || '{}')
    recorded = Boolean(snapshot && Object.keys(snapshot).length && !snapshot.historical_unknown)
  } catch { /* Historical orders can have no usable weight snapshot. */ }
  const rows: Array<[string, number | string | null | undefined, number | string | null | undefined, string]> = [
    ['用料重量（参考）', item.material_unit_weight_g, item.material_item_weight_g, recorded ? '未计算' : '未记录'],
    ['物流重量', item.logistics_unit_weight_g, item.logistics_item_weight_g, recorded ? '未配置' : '未记录'],
  ]
  return rows.map(([label, unit, total, unknown]) => ({ label, value: unit == null && total == null ? unknown : `单件 ${formatWeight(unit, unknown)} · 本项 ${formatWeight(total, unknown)}` }))
}
