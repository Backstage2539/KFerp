export function paymentSummary(status: string, prepayment: number, total: number) {
  const paid = /已付款|已收款|已支付/.test(status) ? total : prepayment
  return { paid: paid.toFixed(2), unpaid: Math.max(0, total - paid).toFixed(2) }
}
export function copyOrderURL(id: number): string {
  return Number.isSafeInteger(id) && id > 0 ? `/pages/employee-order-entry/employee-order-entry?copy_id=${id}` : ''
}

export const prepaymentPresets = [30, 50, 70]
export function prepaymentByRate(goodsAmount: number, percent: number): number {
  if (!Number.isFinite(goodsAmount) || goodsAmount <= 0 || !prepaymentPresets.includes(percent)) return 0
  return Math.round((goodsAmount * percent / 100 + Number.EPSILON) * 100) / 100
}
