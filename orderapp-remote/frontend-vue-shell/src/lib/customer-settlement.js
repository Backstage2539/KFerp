export const customerSettlementChargeTypes = [
  { value: 'order_goods', label: '订单款项' },
  { value: 'shipping', label: '运费' },
  { value: 'direct_ship_service', label: '代发服务费' },
  { value: 'product', label: '商品费用' },
  { value: 'processing', label: '加工费' },
  { value: 'roasting', label: '烘焙费' },
  { value: 'labor', label: '人工费' },
  { value: 'material', label: '物料费' },
  { value: 'packaging', label: '包装费' },
  { value: 'storage', label: '仓储费' },
  { value: 'adjustment', label: '调整费' },
]

export function settlementPeriodRange(mode, now = new Date()) {
  if (mode === 'custom') return null
  const parts = new Intl.DateTimeFormat('en-US', { timeZone: 'Asia/Shanghai', year: 'numeric', month: 'numeric' }).formatToParts(now)
  const year = Number(parts.find(part => part.type === 'year')?.value)
  const month = Number(parts.find(part => part.type === 'month')?.value)
  const monthOffset = mode === 'previous' ? -1 : 0
  const start = new Date(Date.UTC(year, month - 1 + monthOffset, 1))
  const end = new Date(Date.UTC(year, month + monthOffset, 0))
  return { period_from: start.toISOString().slice(0, 10), period_to: end.toISOString().slice(0, 10) }
}

export function settlementChargeOptions(fees = []) {
  const options = [...customerSettlementChargeTypes]
  const known = new Set(options.map(option => option.value))
  for (const row of fees || []) {
    const value = String(row?.fee_type || '').trim()
    if (value && !known.has(value)) {
      known.add(value)
      options.push({ value, label: String(row?.fee_name || value) })
    }
  }
  return options
}
