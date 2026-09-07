export const prepaymentPresets = [30, 50, 70]
export function prepaymentByRate(receivableAmount, percent) {
  const amount = Number(receivableAmount), rate = Number(percent)
  if (!Number.isFinite(amount) || amount <= 0 || !prepaymentPresets.includes(rate)) return '0.00'
  return (Math.round((amount * rate / 100 + Number.EPSILON) * 100) / 100).toFixed(2)
}
