import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { menuGroups, primaryMenuKeys } from './menu-ia.js'
import {
  CUSTOMER_WORKSPACE_MODE,
  FACTORY_WORKSPACE_MODE,
  WORKSPACE_CUSTOMERS_REFRESH_EVENT,
  defaultWorkspaceEntryKey,
  isCustomerAccountActor,
  menuGroupsForWorkspaceMode,
  normalizeWorkspaceMode,
  workspaceCustomersRefreshEvent,
  workspaceViewParams,
} from './workspace-mode.js'

test('factory workspace keeps the existing primary ERP menu layout', () => {
  const groups = menuGroupsForWorkspaceMode(menuGroups, FACTORY_WORKSPACE_MODE)
  const keys = primaryMenuKeys(groups)

  assert.equal(groups.length, menuGroups.length)
  assert.deepEqual(groups.map((group) => group.id), menuGroups.map((group) => group.id))
  assert.equal(keys.includes('customerFulfillment'), false)
  assert.equal(keys.includes('customerPortalSettings'), true)
  assert.equal(keys.includes('customerCapabilityTemplates'), true)
  assert.equal(defaultWorkspaceEntryKey(groups), 'customerPortalSettings')
})

test('customer workspace keeps only customer-facing operations and finance', () => {
  const groups = menuGroupsForWorkspaceMode(menuGroups, CUSTOMER_WORKSPACE_MODE)
  const keys = primaryMenuKeys(groups)

  assert.deepEqual(groups.map((group) => group.id), [
    'customerAccount',
    'customerGoods',
    'customerFinance',
  ])
  assert.deepEqual(groups.map((group) => group.name), [
    '客户账户',
    '客户商品与配方',
    '客户财务',
  ])

  for (const key of [
    'customerFulfillment',
    'order',
    'orders',
    'warehouseInventory',
    'productSettings',
    'costing',
    'bom',
    'mallSettings',
    'financeExpenses',
  ]) {
    assert.ok(keys.includes(key), `${key} should be reachable in customer workspace`)
  }

  for (const key of [
    'customerPortalSettings',
    'customerCapabilityTemplates',
    'financeClosing',
    'financeReport',
    'employees',
    'departments',
    'reqProduct',
    'reqDev',
    'workspaceModeManual',
    'productionManual',
    'productionAcceptance',
    'producePlan',
    'produceRunning',
    'workOrders',
    'jobCards',
    'qualityInspections',
    'produceLogs',
    'productionCosts',
  ]) {
    assert.equal(keys.includes(key), false, `${key} should stay out of the customer workspace`)
  }

  assert.equal(defaultWorkspaceEntryKey(groups), 'customerFulfillment')
})

test('customer login actors are detected from customer-only roles or portal view access', () => {
  assert.equal(isCustomerAccountActor({ basic_auth_admin: true }), false)
  assert.equal(isCustomerAccountActor({ roles: [{ code: 'admin' }] }), false)
  assert.equal(isCustomerAccountActor({ roles: [{ code: 'customer_processing_customer' }] }), true)
  assert.equal(isCustomerAccountActor({ roles: [{ code: 'customer_direct_ship_customer' }] }), true)
  assert.equal(isCustomerAccountActor({ allowed_views: ['customerProcessingPortal'] }), true)
  assert.equal(isCustomerAccountActor({ allowed_views: ['orders', 'customerProcessingPortal'] }), false)
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
    'showWorkspaceSwitcher',
    'isCustomerAccountActor',
    "return ['customerProcessingPortal', 'financeExpenses', 'financeClosing', 'financeReport']",
    'customerProcessingPortal',
    "name: '工作台'",
    "label: '工作台'",
    "name: '费用相关'",
    "key: 'financeExpenses'",
    "key: 'financeClosing'",
    "key: 'financeReport'",
    'fetchCustomerProcessingPortalOverview',
    'menuGroupsForWorkspaceMode',
    'kferp.workspace.customerId',
    ':customer-context-id="workspaceCustomerContextId"',
    ':customer-context-label="workspaceCustomerLabel"',
    '/api/customer-fulfillment/customers?limit=200',
    'WORKSPACE_CUSTOMERS_REFRESH_EVENT',
    'workspaceCustomersRefreshEventName',
  ]) {
    assert.ok(source.includes(marker), `App.vue should include ${marker}`)
  }
})

test('workspace customer refresh event has a stable browser event name', () => {
  const event = workspaceCustomersRefreshEvent()

  assert.equal(WORKSPACE_CUSTOMERS_REFRESH_EVENT, 'kferp:workspace-customers-refresh')
  assert.equal(event.type, WORKSPACE_CUSTOMERS_REFRESH_EVENT)
})
