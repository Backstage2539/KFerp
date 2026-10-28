import test from 'node:test'
import assert from 'node:assert/strict'

import {
  adjustCustomerFulfillmentCustodyInventory,
  createCustomerFulfillmentSettlement,
  createCustomerFulfillmentExternalUser,
  fetchCustomerFulfillmentCustomers,
  fetchCustomerFulfillmentExternalUsers,
  fetchCustomerFulfillmentERPBindings,
  fetchCustomerFulfillmentOrderDetail,
  fetchCustomerFulfillmentOrders,
  fetchCustomerFulfillmentOptions,
  fetchCustomerFulfillmentOverview,
  fetchCustomerProcessingPortalOverview,
  fetchCustomerProcessingPortalOptions,
  submitCustomerFulfillmentDirectShipOrder,
  submitCustomerFulfillmentProcessingWorkOrder,
  resetCustomerFulfillmentExternalUserPassword,
  setCustomerFulfillmentExternalUserLoginEnabled,
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

test('customer fulfillment API wrappers call the expected endpoints', async () => {
  await withMockFetch(async (requests) => {
    await fetchCustomerFulfillmentOverview(147)
    await fetchCustomerFulfillmentOptions(147)
    await fetchCustomerFulfillmentCustomers('誉观山', 60)
    await createCustomerFulfillmentSettlement(147, { period_from: '2026-03-01', period_to: '2026-03-31' })
    await adjustCustomerFulfillmentCustodyInventory(147, { item_type: 'raw_bean', item_name: '埃塞花魁', quantity_g_delta: 1000 })
    await fetchCustomerFulfillmentExternalUsers(147)
    await createCustomerFulfillmentExternalUser(147, { name: '誉观山账号', phone: '13800138075', password: 'secret123' })
    await resetCustomerFulfillmentExternalUserPassword(147, 23, 'secret456')
    await setCustomerFulfillmentExternalUserLoginEnabled(147, 23, false)
    await fetchCustomerFulfillmentERPBindings(147)
    await upsertCustomerFulfillmentERPBinding(147, { employee_id: 23, status: 'active' })
    await submitCustomerFulfillmentProcessingWorkOrder(147, { product_name: '誉观山冷萃豆', input_quantity_g: 5000, planned_output_units: 50 })
    await submitCustomerFulfillmentDirectShipOrder(147, { receiver_name: '张三', receiver_phone: '13800000000', receiver_address: '杭州', product_name: '誉观山冷萃豆', quantity_units: 1 })
    await fetchCustomerFulfillmentOrders(147, { page: 2, limit: 15 })
    await fetchCustomerFulfillmentOrderDetail(88)
    await fetchCustomerProcessingPortalOverview()
    await fetchCustomerProcessingPortalOptions()
    await submitCustomerProcessingWorkOrder({ product_name: '誉观山冷萃豆', input_quantity_g: 5000, planned_output_units: 50 })
    await submitCustomerDirectShipOrder({ receiver_name: '张三', receiver_phone: '13800000000', receiver_address: '杭州', product_name: '誉观山冷萃豆', quantity_units: 1 })

    assert.deepEqual(requests.map((req) => [new URL(req.url).pathname, req.init.method || 'GET']), [
      ['/api/customer-fulfillment/147/overview', 'GET'],
      ['/api/customer-fulfillment/147/options', 'GET'],
      ['/api/customer-fulfillment/customers', 'GET'],
      ['/api/customer-fulfillment/147/settlements', 'POST'],
      ['/api/customer-fulfillment/147/custody-adjustments', 'POST'],
      ['/api/customer-fulfillment/147/external-users', 'GET'],
      ['/api/customer-fulfillment/147/external-users', 'POST'],
      ['/api/customer-fulfillment/147/external-users/23/password/reset', 'POST'],
      ['/api/customer-fulfillment/147/external-users/23/login-enabled', 'POST'],
      ['/api/customer-fulfillment/147/erp-bindings', 'GET'],
      ['/api/customer-fulfillment/147/erp-bindings', 'POST'],
      ['/api/customer-fulfillment/147/work-orders', 'POST'],
      ['/api/customer-fulfillment/147/direct-ship-orders', 'POST'],
      ['/api/orders', 'GET'],
      ['/api/orders/88/detail', 'GET'],
      ['/api/customer-processing/portal/overview', 'GET'],
      ['/api/customer-processing/portal/options', 'GET'],
      ['/api/customer-processing/portal/work-orders', 'POST'],
      ['/api/customer-processing/portal/direct-ship-orders', 'POST'],
    ])
    assert.equal(new URL(requests[2].url).searchParams.get('q'), '誉观山')
    assert.equal(new URL(requests[2].url).searchParams.get('limit'), '60')
    assert.equal(new URL(requests[13].url).searchParams.get('scope'), 'fulfillment')
    assert.equal(new URL(requests[13].url).searchParams.get('customer_id'), '147')
    assert.equal(new URL(requests[13].url).searchParams.get('page'), '2')
    assert.equal(new URL(requests[13].url).searchParams.get('limit'), '15')
    assert.equal(new URL(requests[14].url).search, '')
    assert.equal(JSON.parse(requests[3].init.body).period_from, '2026-03-01')
    assert.equal(JSON.parse(requests[4].init.body).item_name, '埃塞花魁')
    assert.equal(JSON.parse(requests[6].init.body).phone, '13800138075')
    assert.equal(JSON.parse(requests[7].init.body).password, 'secret456')
    assert.equal(JSON.parse(requests[8].init.body).login_enabled, false)
    assert.equal(JSON.parse(requests[10].init.body).employee_id, 23)
    assert.equal(JSON.parse(requests[11].init.body).product_name, '誉观山冷萃豆')
    assert.equal(JSON.parse(requests[12].init.body).receiver_name, '张三')
    assert.equal(JSON.parse(requests[17].init.body).product_name, '誉观山冷萃豆')
    assert.equal(JSON.parse(requests[18].init.body).receiver_name, '张三')
  })
})
