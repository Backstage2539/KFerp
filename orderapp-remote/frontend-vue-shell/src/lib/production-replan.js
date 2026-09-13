function compactNumber(value) {
  return String(Number(Number(value || 0).toFixed(3)))
}

export function summarizeReplanQuantities(rows = []) {
  const groups = new Map()
  for (const row of rows) {
    let value = Number(row?.quantity || 0)
    let unit = String(row?.unit || '').trim()
    if (!(value > 0 && unit)) {
      const grams = Number(row?.quantity_g || 0)
      if (grams <= 0) continue
      value = grams / 1000
      unit = 'kg'
    }
    groups.set(unit, Number(groups.get(unit) || 0) + value)
  }
  if (!groups.size) return '0'
  return Array.from(groups, ([unit, value]) => `${compactNumber(value)} ${unit}`).join(' · ')
}
