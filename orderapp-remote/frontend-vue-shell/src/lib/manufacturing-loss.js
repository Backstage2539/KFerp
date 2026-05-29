export function expectedLossRate(yieldRate) {
  const rate = Number(yieldRate || 0)
  const normalized = rate > 0 && rate <= 1 ? rate : 0.8
  return Math.round((1 - normalized) * 10000) / 10000
}

export function expectedYieldRate(lossRate) {
  const rate = Number(lossRate || 0)
  if (rate < 0 || rate >= 1) return 0
  return Math.round((1 - rate) * 10000) / 10000
}

export function formatPercent(value, digits = 1) {
  const rate = Number(value || 0)
  if (!rate) return '-'
  return `${(rate * 100).toFixed(digits)}%`
}
