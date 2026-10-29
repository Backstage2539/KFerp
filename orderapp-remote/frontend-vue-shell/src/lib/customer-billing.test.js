import test from 'node:test'
import assert from 'node:assert/strict'
import {
  customerBillingTotals,
  customerBillingRowClass,
  isCustomerOrderFullyPaid,
  pageSelectionState,
  togglePageSelection,
  visibleChargeSelection,
  billPaymentAction,
} from './customer-billing.js'

test('bill payment actions respect outstanding amount and recorded reversal eligibility', () => {
  assert.equal(billPaymentAction({ status: 'confirmed', current_due_amount_cents: 500 }), 'pay')
  assert.equal(billPaymentAction({ status: 'paid', current_due_amount_cents: 0, payment_reversible: true }), 'reverse')
  assert.equal(billPaymentAction({ status: 'paid', current_due_amount_cents: 500, payment_reversible: true }), 'reverse')
  assert.equal(billPaymentAction({ status: 'paid', current_due_amount_cents: 0 }), '')
  assert.equal(billPaymentAction({ status: 'settled', current_due_amount_cents: 500 }), '')
  assert.equal(billPaymentAction({ status: 'confirmed', current_due_amount_cents: 0 }), '')
})

test('billing totals count only unpaid amounts across unbilled fees and issued bills', () => {
  assert.deepEqual(customerBillingTotals({
    unbilled: [{ due_cents: 1200 }, { due_cents: 300 }],
    bills: [{ due_amount_cents: 800 }, { due_amount_cents: 0 }],
  }), { total_unpaid_cents: 2300, unbilled_cents: 1500, issued_cents: 800 })
})

test('page selection is tri-state and only toggles rows on the current page', () => {
  const rows = [{ id: 1 }, { id: 2 }, { id: 3 }]
  assert.deepEqual(pageSelectionState(rows, new Set(['1'])), { checked: false, indeterminate: true })
  assert.deepEqual(togglePageSelection(rows, new Set(['1']), true), new Set(['1', '2', '3']))
  assert.deepEqual(togglePageSelection(rows, new Set(['1', '2', '3', '9']), false), new Set(['9']))
})

test('selection follows only visible eligible charge rows after filtering', () => {
  assert.deepEqual(visibleChargeSelection(
    [{ id: 1, confirmation_status: 'confirmed' }, { id: 2, confirmation_status: 'pending_billing' }],
    new Set(['1', '2']),
  ), new Set(['1']))
})

test('unpaid shipped orders remain red; only fully paid shipped orders are green', () => {
  assert.equal(isCustomerOrderFullyPaid('未付款'), false)
  assert.equal(isCustomerOrderFullyPaid('部分付款'), false)
  assert.equal(isCustomerOrderFullyPaid('已付款'), true)
  assert.equal(customerBillingRowClass({ pay_status: '未付款' }, true), 'order-row-unpaid')
  assert.equal(customerBillingRowClass({ pay_status: '已支付' }, true), 'order-row-complete')
  assert.equal(customerBillingRowClass({ pay_status: '已收款' }, true), 'order-row-complete')
  assert.equal(customerBillingRowClass({ pay_status: '已付款' }, false), '')
  assert.equal(customerBillingRowClass({ pay_status: '已付款' }, true), 'order-row-complete')
})
