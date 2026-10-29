import test from 'node:test'
import assert from 'node:assert/strict'
import {
  fetchCustomerBillingSummary,
  fetchCustomerUnbilledCharges,
  fetchCustomerIssuedBills,
  previewCustomerBill,
  issueCustomerBill,
  fetchCustomerBill,
  fetchCustomerBillPDFSettings,
  saveCustomerBillPDFSettings,
  updateCustomerBillPayment,
} from './customer-billing.js'

test('customer billing API keeps filters, paging, selected sources and idempotency in the expected routes', async () => {
  const oldWindow = globalThis.window
  const oldFetch = globalThis.fetch
  const requests = []
  globalThis.window = { location: { origin: 'https://erp.qacoohee.com' }, localStorage: { getItem: () => '' } }
  globalThis.fetch = async (url, init = {}) => {
    requests.push({ url: new URL(url), init })
    return new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  try {
    await fetchCustomerBillingSummary(488)
    await fetchCustomerUnbilledCharges(488, { fee_type: 'shipping', date_from: '2026-10-01', date_to: '2026-10-31', page: 2, limit: 25 })
    await fetchCustomerIssuedBills(488, { status: 'unpaid', page: 1, limit: 20 })
    const payload = { sources: [{ charge_type: 'shipping', source_type: 'customer_fee_item', source_id: 7 }], preview_version: 'v1' }
    await previewCustomerBill(488, payload)
    await issueCustomerBill(488, { ...payload, idempotency_key: 'issue-1' })
    await fetchCustomerBill(488, 91)
    await fetchCustomerBillPDFSettings(488, 91)
    await saveCustomerBillPDFSettings(488, 91, { seal_asset_id: 5, placements: [] })
    await updateCustomerBillPayment(488, 91, 'pay', 1200, 'pay-key', 0, '微信支付')
    await updateCustomerBillPayment(488, 91, 'reverse', 0, 'reverse-key', 1)
    assert.deepEqual(requests.map(({ url, init }) => [url.pathname, init.method || 'GET']), [
      ['/api/customer-fulfillment/488/billing/summary', 'GET'],
      ['/api/customer-fulfillment/488/billing/unbilled', 'GET'],
      ['/api/customer-fulfillment/488/billing/bills', 'GET'],
      ['/api/customer-fulfillment/488/settlements/preview', 'POST'],
      ['/api/customer-fulfillment/488/settlements/issue', 'POST'],
      ['/api/customer-fulfillment/488/billing/bills/91', 'GET'],
      ['/api/customer-fulfillment/488/billing/bills/91/pdf-settings', 'GET'],
      ['/api/customer-fulfillment/488/billing/bills/91/pdf-settings', 'PUT'],
      ['/api/customer-fulfillment/488/billing/bills/91/payment', 'POST'],
      ['/api/customer-fulfillment/488/billing/bills/91/payment', 'POST'],
    ])
    assert.equal(requests[1].url.searchParams.get('fee_type'), 'shipping')
    assert.equal(requests[1].url.searchParams.get('page'), '2')
    assert.equal(requests[4].init.body, JSON.stringify({ ...payload, idempotency_key: 'issue-1' }))
    assert.equal(requests[8].init.body, JSON.stringify({ action: 'pay', expected_due_amount_cents: 1200, idempotency_key: 'pay-key', expected_payment_revision: 0, payment_method: '微信支付' }))
    assert.equal(requests[9].init.body, JSON.stringify({ action: 'reverse', expected_due_amount_cents: 0, idempotency_key: 'reverse-key', expected_payment_revision: 1 }))
  } finally {
    globalThis.window = oldWindow
    globalThis.fetch = oldFetch
  }
})
