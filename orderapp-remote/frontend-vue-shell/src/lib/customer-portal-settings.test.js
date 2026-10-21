import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import * as settings from './customer-portal-settings.js'
const { customerDossierNavigationDetail } = settings

test('customer capability summary uses effective customer capability instead of raw template switch', () => {
  const template = { key: 'dropship', capabilities: [{ code: 'product_order', enabled: false }, { code: 'direct_ship', enabled: true }] }
  const row = { customer: { capability_template_key: 'dropship' }, form: { capability_template_key: 'dropship' }, capabilities: [{ code: 'product_order', enabled: true, config: { required_by_fulfillment: true } }, { code: 'direct_ship', enabled: true }] }
  const result = settings.effectiveCustomerCapabilities(row, template)
  assert.equal(result.find(item => item.code === 'product_order').enabled, true)
  assert.equal(result.find(item => item.code === 'product_order').config.required_by_fulfillment, true)
  assert.equal(template.capabilities[0].enabled, false)
  row.form.capability_template_key = 'ordinary'
  assert.equal(settings.effectiveCustomerCapabilities(row, { key: 'ordinary', capabilities: [{ code: 'product_order', enabled: false }] })[0].enabled, true)
  row.capabilities[0].config = {}
  assert.equal(settings.effectiveCustomerCapabilities(row, { key: 'ordinary', capabilities: [{ code: 'product_order', enabled: false }] })[0].enabled, false)
})

test('customer portal settings opens the customer dossier drawer for the current row', () => {
  assert.deepEqual(
    customerDossierNavigationDetail({ customer: { id: 74 } }),
    { key: 'customers', params: { edit_id: 74 } },
  )
})

test('customer portal settings opens the scoped fulfillment workspace and refreshes customer options after account changes', () => {
  const source = readFileSync(new URL('../views/CustomerPortalSettingsView.vue', import.meta.url), 'utf8')

  for (const marker of [
    '打开客户履约工作台',
    'openCustomerProfile(row)',
    "key: 'customerFulfillment'",
    'customerContext: { customerID, customerName: row.customer.name }',
    "returnNavigation: { key: 'customerPortalSettings'",
    'params: { customer_id: customerID }',
    'workspaceCustomersRefreshEvent',
    'refreshWorkspaceCustomers()',
  ]) {
    assert.ok(source.includes(marker), `CustomerPortalSettingsView.vue should include ${marker}`)
  }

  assert.equal(source.includes('openCustomerProcessingPortal'), false)
  assert.equal(source.includes('一件代发指定价格表'), false)
  assert.equal(source.includes('direct_ship_price_table_keys'), false)
})
