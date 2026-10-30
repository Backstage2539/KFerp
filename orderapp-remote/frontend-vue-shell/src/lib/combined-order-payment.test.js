import test from 'node:test'
import assert from 'node:assert/strict'
import { combinedPaymentSummary, parseCombinedMoneyCents, combinedPaymentRequest } from './combined-order-payment.js'

const orders = [
  { order_id: 1, customer_id: 7, edit_revision: 'v1', total_cents: 10000, paid_cents: 3000, due_cents: 7000 },
  { order_id: 2, customer_id: 7, edit_revision: 'v2', total_cents: 5000, paid_cents: 5000, due_cents: 0 },
]

test('合并付款汇总包含已收并排除已结清订单', () => {
  const summary = combinedPaymentSummary(orders)
  assert.deepEqual([summary.totalCents, summary.paidCents, summary.dueCents, summary.payableCount], [15000, 8000, 7000, 1])
  assert.equal(summary.error, '')
})

test('无效预览、混合客户或版本缺失不能提交', () => {
  assert.match(combinedPaymentSummary([orders[0], { ...orders[1], customer_id: 8 }]).error, /同一客户/)
  assert.match(combinedPaymentSummary([orders[0], { ...orders[1], due_cents: undefined }]).error, /金额/)
  assert.match(combinedPaymentSummary([{ ...orders[0], edit_revision: '' }]).error, /版本/)
  assert.match(combinedPaymentSummary([{ ...orders[0], error: '无权查看' }]).error, /无权查看/)
})

test('实际收款采用整数分精确解析并拒绝无效输入', () => {
  assert.equal(parseCombinedMoneyCents('00070.09'), 7009)
  assert.equal(parseCombinedMoneyCents('70'), 7000)
  for (const amount of ['', '-1', '1.001', '1e3', 'abc', '99999999999999999']) assert.throws(() => parseCombinedMoneyCents(amount))
})

test('只允许足额收款并提交全部订单版本，超额保留为待分配', () => {
  assert.throws(() => combinedPaymentRequest(orders, { actual: '69.99', method: '微信支付', key: 'key-1' }), /不足/)
  assert.throws(() => combinedPaymentRequest(orders, { actual: '70.00', method: '', key: 'key-1' }), /方式/)
  const request = combinedPaymentRequest(orders, { actual: '75.25', method: '微信支付', key: 'key-1', voucherID: 3, shipStatusID: 9 })
  assert.deepEqual(request, { orders: [{ order_id: 1, edit_revision: 'v1' }, { order_id: 2, edit_revision: 'v2' }], actual_cents: 7525, expected_due_cents: 7000, payment_method: '微信支付', payment_voucher_asset_id: 3, ship_status_id: 9, idempotency_key: 'key-1' })
  const unchangedShipping = combinedPaymentRequest(orders, { actual: '70', method: '微信支付', key: 'key-2' })
  assert.equal(Object.hasOwn(unchangedShipping, 'ship_status_id'), false)
})
