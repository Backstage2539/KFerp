import { paymentSummary } from './prepayment'

export type CustomerFeeSummary = { goods_cents: number; shipping_cents: number; service_cents: number; payable_cents: number; paid_cents: number; unpaid_cents: number }
export function customerOrderAmounts(order: { fee_summary?: CustomerFeeSummary; grand_total?: string; pay_status?: string; prepayment_amount?: string }) {
  const fee = order.fee_summary
  if (fee) {
    const money = (value: number) => (Number(value || 0) / 100).toFixed(2)
    return { payable: money(fee.payable_cents), paid: money(fee.paid_cents), unpaid: money(fee.unpaid_cents), goods: money(fee.goods_cents), shipping: money(fee.shipping_cents), service: money(fee.service_cents) }
  }
  const total = Number(order.grand_total || 0)
  const payment = paymentSummary(order.pay_status || '', Number(order.prepayment_amount || 0), total)
  return { payable: total.toFixed(2), paid: payment.paid, unpaid: payment.unpaid, goods: '0.00', shipping: '0.00', service: '0.00' }
}
export function priceUsageForMode(mode: 'stock' | 'on_demand'): 'product_order' | 'direct_ship' {
  return mode === 'stock' ? 'product_order' : 'direct_ship'
}
export function customerOrderPaymentStatus(order: { fee_summary?: CustomerFeeSummary; pay_status?: string }): string {
 const fee = order.fee_summary
 if (!fee) return order.pay_status || '未收款'
 if (fee.unpaid_cents > 0) return fee.paid_cents > 0 ? '部分付款' : '未付款'
 return fee.payable_cents > 0 ? '已付款' : '无需收款'
}
