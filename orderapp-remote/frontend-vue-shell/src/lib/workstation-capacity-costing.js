export function normalizeCapacityCostMethod(value) {
  const raw = typeof value === 'object' && value !== null ? value.cost_method : value
  return String(raw || '').trim().toLowerCase() === 'piece' ? 'piece' : 'time'
}

export function capacityCostMethodLabel(value) {
  return normalizeCapacityCostMethod(value) === 'piece' ? '按件' : '按时间'
}

export function isCountCapacityUnit(value) {
  return ['件', 'unit', 'units', 'pc', 'pcs', 'piece', 'pieces']
    .includes(String(value || '').trim().toLowerCase())
}

export function formatCapacityCostNumber(value) {
  const num = Number(value || 0)
  if (!Number.isFinite(num)) return '0'
  return Number(num.toFixed(4)).toString()
}

export function workstationCapacityUnitCost(capacity = {}) {
  if (normalizeCapacityCostMethod(capacity) === 'piece') {
    return Number(Math.max(0, Number(capacity.piece_rate || 0)).toFixed(4))
  }
  if (capacity.scheduling_mode === 'quantity') {
    const speed = Number(capacity.throughput_per_hour || 0)
    return speed > 0 ? Number((Number(capacity.hourly_rate || 0) / speed).toFixed(4)) : null
  }
  const qty = Number(capacity.batch_size_qty || 0)
  const minutes = Number(capacity.standard_minutes || 0)
  const hourly = Number(capacity.hourly_rate || 0)
  if (qty <= 0 || minutes <= 0 || hourly <= 0) return 0
  return Number(((hourly * minutes / 60) / qty).toFixed(4))
}

export function workstationCapacityCostMeta(capacity = {}) {
  const unit = String(capacity.batch_size_unit || '件').trim() || '件'
  if (normalizeCapacityCostMethod(capacity) === 'piece') {
    return `计件成本 ${formatCapacityCostNumber(capacity.piece_rate)}元/销售规格件`
  }
  if (capacity.scheduling_mode === 'quantity') {
    const speed = Number(capacity.throughput_per_hour || 0)
    return speed > 0 ? `小时费率 ${formatCapacityCostNumber(capacity.hourly_rate)} / 参考速度 ${formatCapacityCostNumber(speed)}${unit}/小时 = ${formatCapacityCostNumber(workstationCapacityUnitCost(capacity))}/${unit}` : '未配置参考速度，无法估算标准成本'
  }
  const cost = workstationCapacityUnitCost(capacity)
  return `小时费率 ${formatCapacityCostNumber(capacity.hourly_rate)} × 标准分钟 ${formatCapacityCostNumber(capacity.standard_minutes)} / 60 / 标准批量 ${formatCapacityCostNumber(capacity.batch_size_qty)}${unit} = ${formatCapacityCostNumber(cost)}/${unit}`
}

export function workstationCapacityOptionLabel(capacity = {}) {
  const unit = String(capacity.batch_size_unit || '').trim()
  const parts = [capacity.name || `#${capacity.id || ''}`]
  if (capacity.scheduling_mode === 'quantity') parts.push(Number(capacity.throughput_per_hour || 0) > 0 ? `${formatCapacityCostNumber(capacity.throughput_per_hour)}${unit}/小时` : '未配置参考速度')
  if (capacity.scheduling_mode !== 'quantity' && Number(capacity.batch_size_qty || 0) > 0) parts.push(`${formatCapacityCostNumber(capacity.batch_size_qty)}${unit}`)
  if (capacity.scheduling_mode !== 'quantity' && Number(capacity.standard_minutes || 0) > 0) parts.push(`${formatCapacityCostNumber(capacity.standard_minutes)}分钟/批`)
  if (normalizeCapacityCostMethod(capacity) === 'piece') {
    parts.push(`${formatCapacityCostNumber(capacity.piece_rate)}元/销售规格件`)
  } else if (Number(capacity.hourly_rate || 0) > 0) {
    parts.push(`${formatCapacityCostNumber(capacity.hourly_rate)}/小时`)
  }
  return parts.filter(Boolean).join(' · ')
}
