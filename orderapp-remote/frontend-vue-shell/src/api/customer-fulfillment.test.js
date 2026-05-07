import test from 'node:test'
import assert from 'node:assert/strict'

import {
  applyCustomerFulfillmentImport,
  buildCustomerFulfillmentImportForm,
  createCustomerFulfillmentSettlement,
  fetchCustomerFulfillmentCustomers,
  fetchCustomerFulfillmentImportPreview,
  fetchCustomerFulfillmentImportRows,
  fetchCustomerFulfillmentImports,
  fetchCustomerFulfillmentOverview,
  parseCustomerFulfillmentImport,
} from './customer-fulfillment.js'

function withMockFetch(fn) {
  const previousWindow = globalThis.window
  const previousFetch = globalThis.fetch
  const requests = []
  globalThis.window = {
    location: { origin: 'https://erp.qacoohee.com' },
    localStorage: { getItem: () => '' },
  }
  globalThis.fetch = async (url, init = {}) => {
    requests.push({ url, init })
    return new Response(JSON.stringify({ ok: true }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  return Promise.resolve()
    .then(() => fn(requests))
    .finally(() => {
      globalThis.window = previousWindow
      globalThis.fetch = previousFetch
    })
}

test('buildCustomerFulfillmentImportForm appends import_type and file', () => {
  const file = new Blob(['xlsx-bytes'])
  file.name = 'direct.xlsx'
  const form = buildCustomerFulfillmentImportForm('direct_ship_workbook', file)
  assert.equal(form.get('import_type'), 'direct_ship_workbook')
  assert.equal(form.get('file').name, 'direct.xlsx')
})

test('customer fulfillment API wrappers call the expected endpoints', async () => {
  await withMockFetch(async (requests) => {
    const file = new Blob(['xlsx-bytes'])
    file.name = 'direct.xlsx'
    await parseCustomerFulfillmentImport(147, 'direct_ship_workbook', file)
    await applyCustomerFulfillmentImport(55)
    await fetchCustomerFulfillmentOverview(147)
    await fetchCustomerFulfillmentImports(147)
    await fetchCustomerFulfillmentCustomers('誉观山', 60)
    await fetchCustomerFulfillmentImportRows(55, { status: 'invalid', limit: 80 })
    await fetchCustomerFulfillmentImportPreview(55)
    await createCustomerFulfillmentSettlement(147, { period_from: '2026-03-01', period_to: '2026-03-31' })

    assert.deepEqual(requests.map((req) => [new URL(req.url).pathname, req.init.method || 'GET']), [
      ['/api/customer-fulfillment/147/imports/parse', 'POST'],
      ['/api/customer-fulfillment/imports/55/apply', 'POST'],
      ['/api/customer-fulfillment/147/overview', 'GET'],
      ['/api/customer-fulfillment/147/imports', 'GET'],
      ['/api/customer-fulfillment/customers', 'GET'],
      ['/api/customer-fulfillment/imports/55/rows', 'GET'],
      ['/api/customer-fulfillment/imports/55/preview', 'GET'],
      ['/api/customer-fulfillment/147/settlements', 'POST'],
    ])
    assert.equal(new URL(requests[4].url).searchParams.get('q'), '誉观山')
    assert.equal(new URL(requests[4].url).searchParams.get('limit'), '60')
    assert.equal(new URL(requests[5].url).searchParams.get('status'), 'invalid')
    assert.equal(new URL(requests[5].url).searchParams.get('limit'), '80')
    assert.ok(requests[0].init.body instanceof FormData)
    assert.equal(requests[0].init.headers?.['Content-Type'], undefined)
    assert.equal(JSON.parse(requests[7].init.body).period_from, '2026-03-01')
  })
})
