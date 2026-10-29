import test from 'node:test'
import assert from 'node:assert/strict'
import { makeBatchStatusDraft, refreshBatchStatusDraft, batchStatusRequest } from './order-batch-status.js'

test('shipping-only request does not send or alter payment data', () => {
  const draft = makeBatchStatusDraft({ order_id: 3, edit_revision: 'r1', payment_goods_amount: '123.45', payment_voucher_asset_id: 8 })
  assert.deepEqual(batchStatusRequest(draft, 0, 2), { order_id: 3, edit_revision: 'r1', ship_status_id: 2 })
})

test('refresh keeps only manually changed fields and uses fresh version and saved values', () => {
  const draft = makeBatchStatusDraft({ order_id: 1, edit_revision: 'old', payment_method: '微信支付', payment_goods_amount: '1', prepayment_amount: '0', payment_voucher_asset_id: 4 })
  draft.payment_goods_amount = '5'
  const refreshed = refreshBatchStatusDraft(draft, { order_id: 1, edit_revision: 'new', payment_method: '银行转账', payment_goods_amount: '2', prepayment_amount: '3', payment_voucher_asset_id: 4 })
  assert.equal(refreshed.edit_revision, 'new')
  assert.equal(refreshed.payment_method, '银行转账')
  assert.equal(refreshed.prepayment_amount, '3')
  assert.equal(refreshed.payment_goods_amount, '5')
  assert.equal(refreshed.saved.payment_goods_amount, '2')
})

test('payment values and vouchers remain specific to each order', () => {
  const a = makeBatchStatusDraft({ order_id: 1, edit_revision: 'a', prepayment_amount: '2.50', payment_goods_amount: '0', payment_voucher_asset_id: 5 })
  const b = makeBatchStatusDraft({ order_id: 2, edit_revision: 'b', prepayment_amount: '3', payment_goods_amount: '8.2', payment_voucher_asset_id: 6 })
  assert.deepEqual(batchStatusRequest(a, 7, 0), { order_id: 1, edit_revision: 'a', pay_status_id: 7, payment_method: '', prepayment_amount: 2.5, payment_goods_amount: 0, payment_voucher_asset_id: 5 })
  assert.equal(batchStatusRequest(b, 7, 9).payment_voucher_asset_id, 6)
  assert.equal(batchStatusRequest(b, 7, 9).payment_goods_amount, 8.2)
  assert.throws(() => batchStatusRequest({ ...a, prepayment_amount: '1.001' }, 7, 0), /最多两位小数/)
})
