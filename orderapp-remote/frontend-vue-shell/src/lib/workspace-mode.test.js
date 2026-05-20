import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { menuGroups, primaryMenuKeys } from './menu-ia.js'
import {
  CUSTOMER_WORKSPACE_MODE,
  FACTORY_WORKSPACE_MODE,
  defaultWorkspaceEntryKey,
  menuGroupsForWorkspaceMode,
  normalizeWorkspaceMode,
  workspaceViewParams,
} from './workspace-mode.js'

test('factory workspace keeps the existing primary ERP menu layout', () => {
  const groups = menuGroupsForWorkspaceMode(menuGroups, FACTORY_WORKSPACE_MODE)

  assert.equal(groups.length, menuGroups.length)
  assert.deepEqual(groups.map((group) => group.id), menuGroups.map((group) => group.id))
  assert.equal(defaultWorkspaceEntryKey(groups), 'customerFulfillment')
})

test('customer workspace rearranges existing views around one customer account', () => {
  const groups = menuGroupsForWorkspaceMode(menuGroups, CUSTOMER_WORKSPACE_MODE)
  const keys = primaryMenuKeys(groups)

  assert.deepEqual(groups.map((group) => group.id), [
    'customerAccount',
    'customerGoods',
    'customerPortal',
    'customerFinance',
  ])
  assert.deepEqual(groups.map((group) => group.name), [
    '客户账户',
    '客户商品与配方',
    '门户与能力',
    '客户财务',
  ])

  for (const key of [
    'customerFulfillment',
    'order',
    'orders',
    'warehouseInventory',
    'producePlan',
    'workspaceModeManual',
    'productSettings',
    'costing',
    'bom',
    'mallSettings',
    'customerPortalSettings',
    'customerCapabilityTemplates',
    'financeExpenses',
    'financeClosing',
    'financeReport',
  ]) {
    assert.ok(keys.includes(key), `${key} should be reachable in customer workspace`)
  }

  for (const key of ['employees', 'departments', 'reqProduct', 'reqDev']) {
    assert.equal(keys.includes(key), false, `${key} should stay out of the customer workspace`)
  }

  assert.equal(defaultWorkspaceEntryKey(groups), 'customerFulfillment')
})

test('customer workspace injects current customer into routed view params', () => {
  assert.equal(normalizeWorkspaceMode('customer'), CUSTOMER_WORKSPACE_MODE)
  assert.equal(normalizeWorkspaceMode('unknown'), FACTORY_WORKSPACE_MODE)

  assert.deepEqual(
    workspaceViewParams({ scope: 'fulfillment' }, { mode: CUSTOMER_WORKSPACE_MODE, customerID: 18 }),
    { scope: 'fulfillment', customer_id: '18' },
  )

  assert.deepEqual(
    workspaceViewParams({ scope: 'all' }, { mode: FACTORY_WORKSPACE_MODE, customerID: 18 }),
    { scope: 'all' },
  )

  assert.deepEqual(
    workspaceViewParams({ customer_id: '3' }, { mode: CUSTOMER_WORKSPACE_MODE, customerID: 0 }),
    { customer_id: '3' },
  )
})

test('vue shell wires workspace mode into navigation and routed pages', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  for (const marker of [
    'workspace-switcher',
    'menuGroupsForWorkspaceMode',
    'kferp.workspace.customerId',
    ':customer-context-id="workspaceCustomerContextId"',
    ':customer-context-label="workspaceCustomerLabel"',
    '/api/customer-fulfillment/customers?limit=200',
  ]) {
    assert.ok(source.includes(marker), `App.vue should include ${marker}`)
  }
})
