import test from 'node:test'
import assert from 'node:assert/strict'

import {
  applyCustomerFulfillmentImport,
  adjustCustomerFulfillmentCustodyInventory,
  buildCustomerFulfillmentImportForm,
  createCustomerFulfillmentSettlement,
  fetchCustomerFulfillmentCustomers,
  fetchCustomerFulfillmentERPBindings,
  fetchCustomerFulfillmentOptions,
  fetchCustomerFulfillmentImportPreview,
  fetchCustomerFulfillmentImportRows,
  fetchCustomerFulfillmentImports,
  fetchCustomerFulfillmentOverview,
  fetchCustomerProcessingPortalOverview,
  fetchCustomerProcessingPortalOptions,
  submitCustomerFulfillmentDirectShipOrder,
  submitCustomerFulfillmentProcessingWorkOrder,
  parseCustomerFulfillmentImport,
  submitCustomerDirectShipOrder,
  submitCustomerProcessingWorkOrder,
  upsertCustomerFulfillmentERPBinding,
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
    await fetchCustomerFulfillmentOptions(147)
    await fetchCustomerFulfillmentImports(147)
    await fetchCustomerFulfillmentCustomers('誉观山', 60)
    await fetchCustomerFulfillmentImportRows(55, { status: 'invalid', limit: 80 })
    await fetchCustomerFulfillmentImportPreview(55)
    await createCustomerFulfillmentSettlement(147, { period_from: '2026-03-01', period_to: '2026-03-31' })
    await adjustCustomerFulfillmentCustodyInventory(147, { item_type: 'raw_bean', item_name: '埃塞花魁', quantity_g_delta: 1000 })
    await fetchCustomerFulfillmentERPBindings(147)
    await upsertCustomerFulfillmentERPBinding(147, { employee_id: 23, status: 'active' })
    await submitCustomerFulfillmentProcessingWorkOrder(147, { product_name: '誉观山冷萃豆', input_quantity_g: 5000, planned_output_units: 50 })
    await submitCustomerFulfillmentDirectShipOrder(147, { receiver_name: '张三', receiver_phone: '13800000000', receiver_address: '杭州', product_name: '誉观山冷萃豆', quantity_units: 1 })
    await fetchCustomerProcessingPortalOverview()
    await fetchCustomerProcessingPortalOptions()
    await submitCustomerProcessingWorkOrder({ product_name: '誉观山冷萃豆', input_quantity_g: 5000, planned_output_units: 50 })
    await submitCustomerDirectShipOrder({ receiver_name: '张三', receiver_phone: '13800000000', receiver_address: '杭州', product_name: '誉观山冷萃豆', quantity_units: 1 })

    assert.deepEqual(requests.map((req) => [new URL(req.url).pathname, req.init.method || 'GET']), [
      ['/api/customer-fulfillment/147/imports/parse', 'POST'],
      ['/api/customer-fulfillment/imports/55/apply', 'POST'],
      ['/api/customer-fulfillment/147/overview', 'GET'],
      ['/api/customer-fulfillment/147/options', 'GET'],
      ['/api/customer-fulfillment/147/imports', 'GET'],
      ['/api/customer-fulfillment/customers', 'GET'],
      ['/api/customer-fulfillment/imports/55/rows', 'GET'],
      ['/api/customer-fulfillment/imports/55/preview', 'GET'],
      ['/api/customer-fulfillment/147/settlements', 'POST'],
      ['/api/customer-fulfillment/147/custody-adjustments', 'POST'],
      ['/api/customer-fulfillment/147/erp-bindings', 'GET'],
      ['/api/customer-fulfillment/147/erp-bindings', 'POST'],
      ['/api/customer-fulfillment/147/work-orders', 'POST'],
      ['/api/customer-fulfillment/147/direct-ship-orders', 'POST'],
      ['/api/customer-processing/portal/overview', 'GET'],
      ['/api/customer-processing/portal/options', 'GET'],
      ['/api/customer-processing/portal/work-orders', 'POST'],
      ['/api/customer-processing/portal/direct-ship-orders', 'POST'],
    ])
    assert.equal(new URL(requests[5].url).searchParams.get('q'), '誉观山')
    assert.equal(new URL(requests[5].url).searchParams.get('limit'), '60')
    assert.equal(new URL(requests[6].url).searchParams.get('status'), 'invalid')
    assert.equal(new URL(requests[6].url).searchParams.get('limit'), '80')
    assert.ok(requests[0].init.body instanceof FormData)
    assert.equal(requests[0].init.headers?.['Content-Type'], undefined)
    assert.equal(JSON.parse(requests[8].init.body).period_from, '2026-03-01')
    assert.equal(JSON.parse(requests[9].init.body).item_name, '埃塞花魁')
    assert.equal(JSON.parse(requests[11].init.body).employee_id, 23)
    assert.equal(JSON.parse(requests[12].init.body).product_name, '誉观山冷萃豆')
    assert.equal(JSON.parse(requests[13].init.body).receiver_name, '张三')
    assert.equal(JSON.parse(requests[16].init.body).product_name, '誉观山冷萃豆')
    assert.equal(JSON.parse(requests[17].init.body).receiver_name, '张三')
  })
})
