import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { customerDossierNavigationDetail } from './customer-portal-settings.js'

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
    "key: 'customerProcessingPortal'",
    'params: { customer_id: customerID }',
    'workspaceCustomersRefreshEvent',
    'refreshWorkspaceCustomers()',
  ]) {
    assert.ok(source.includes(marker), `CustomerPortalSettingsView.vue should include ${marker}`)
  }

  assert.equal(source.includes('openCustomerProcessingPortal'), false)
})
