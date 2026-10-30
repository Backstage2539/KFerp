// Amounts from the preview are integer cents. Never calculate aggregate
// collection or overpayment using floating-point currency values.
export function combinedPaymentSummary(orders = []) {
  const summary = { totalCents: 0, paidCents: 0, dueCents: 0, payableCount: 0, customerID: 0, error: '' }
  if (!orders.length) return { ...summary, error: '请先选择订单' }
  for (const order of orders) {
    if (order.error) return { ...summary, error: `订单读取失败：${order.error}` }
    const customerID = Number(order.customer_id)
    if (!Number.isSafeInteger(customerID) || customerID <= 0) return { ...summary, error: '订单客户信息不完整，请重新读取' }
    if (summary.customerID && customerID !== summary.customerID) return { ...summary, error: '合并付款仅支持同一客户的订单' }
    summary.customerID = customerID
    if (!Number.isSafeInteger(Number(order.order_id)) || Number(order.order_id) <= 0 || !String(order.edit_revision || '').trim()) return { ...summary, error: '订单版本信息不完整，请重新读取' }
    for (const [field, target] of [['total_cents', 'totalCents'], ['paid_cents', 'paidCents'], ['due_cents', 'dueCents']]) {
      const value = order[field]
      if (!Number.isSafeInteger(value) || value < 0 || !Number.isSafeInteger(summary[target] + value)) return { ...summary, error: '订单金额信息无效，请重新读取' }
      summary[target] += value
    }
    if (order.due_cents > 0) summary.payableCount++
  }
  return summary
}

export function parseCombinedMoneyCents(value) {
  const text = String(value ?? '').trim()
  if (!/^\d+(?:\.\d{1,2})?$/.test(text)) throw new Error('实际收款须为非负金额，最多两位小数')
  const [whole, fraction = ''] = text.split('.')
  const cents = Number(whole) * 100 + Number(fraction.padEnd(2, '0'))
  if (!Number.isSafeInteger(cents)) throw new Error('实际收款金额过大')
  return cents
}

export function combinedPaymentRequest(orders, { actual, method, key, voucherID = 0, shipStatusID = 0 }) {
  const summary = combinedPaymentSummary(orders)
  if (summary.error) throw new Error(summary.error)
  if (summary.dueCents <= 0 || !summary.payableCount) throw new Error('所选订单没有待收金额，无需合并付款')
  const actualCents = parseCombinedMoneyCents(actual)
  if (actualCents < summary.dueCents) throw new Error('实际收款不足，不能标记已付款')
  const paymentMethod = String(method || '').trim()
  if (!paymentMethod) throw new Error('请选择实际收款方式')
  if (!String(key || '').trim()) throw new Error('付款操作标识缺失，请刷新后重试')
  return {
    orders: orders.map(order => ({ order_id: Number(order.order_id), edit_revision: String(order.edit_revision) })),
    actual_cents: actualCents,
    expected_due_cents: summary.dueCents,
    payment_method: paymentMethod,
    payment_voucher_asset_id: Number(voucherID || 0),
    ...(Number(shipStatusID) > 0 ? { ship_status_id: Number(shipStatusID) } : {}),
    idempotency_key: String(key),
  }
}
