export const prepaymentPresets = [30, 50, 70]
export function prepaymentByRate(goodsAmount, percent) {
  const goods = Number(goodsAmount), rate = Number(percent)
  if (!Number.isFinite(goods) || goods <= 0 || !prepaymentPresets.includes(rate)) return '0.00'
  return (Math.round((goods * rate / 100 + Number.EPSILON) * 100) / 100).toFixed(2)
}
