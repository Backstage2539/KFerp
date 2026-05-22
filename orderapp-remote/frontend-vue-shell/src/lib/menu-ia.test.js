import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  defaultExpandedGroups,
  groupForView,
  menuGroups,
  primaryMenuKeys,
  restoreExpandedGroups,
  toggleExpandedGroup,
} from './menu-ia.js'

test('primary menu replaces overlapping inventory pages with warehouse inventory workspaces', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('warehouseInventory'))
  assert.ok(keys.includes('stockOperations'))
  assert.ok(keys.includes('stockOutboundLogs'))
  assert.ok(keys.includes('materials'))
  assert.equal(keys.includes('materialBatches'), false)
  assert.equal(keys.includes('stockBatches'), false)
  assert.equal(keys.includes('stockLedger'), false)
  assert.equal(keys.includes('inventory'), false)
})

test('warehouse inventory and legacy stock views resolve to the inventory group', () => {
  assert.equal(groupForView(menuGroups, 'warehouseInventory')?.id, 'inventory')
  assert.equal(groupForView(menuGroups, 'stockOperations')?.id, 'inventory')
  assert.equal(groupForView(menuGroups, 'stockOutboundLogs')?.id, 'inventory')
})

test('expanded menu groups persist and keep current group open', () => {
  const initial = defaultExpandedGroups(menuGroups, 'warehouseInventory')
  assert.ok(initial.includes('inventory'))

  const closed = toggleExpandedGroup(initial, 'inventory')
  assert.equal(closed.includes('inventory'), false)

  const restored = restoreExpandedGroups(menuGroups, JSON.stringify(['sales']), 'warehouseInventory')
  assert.deepEqual(restored, ['sales', 'inventory'])
})

test('product menu exposes unified product settings and keeps legacy views hidden', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('productSettings'))
  assert.ok(keys.includes('mallSettings'))
  assert.ok(keys.includes('costing'))
  assert.equal(keys.includes('products'), false)
  assert.equal(groupForView(menuGroups, 'productSettings')?.id, 'product')
  assert.equal(groupForView(menuGroups, 'mallSettings')?.id, 'product')
  assert.equal(groupForView(menuGroups, 'costing')?.id, 'product')
})

test('production menu exposes the production flow manual as a primary page', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('productionManual'))
  assert.equal(groupForView(menuGroups, 'productionManual')?.id, 'production')
})

test('operation manuals live inside their functional menu groups', () => {
  const expectations = [
    ['orderSalesManual', 'sales'],
    ['productionManual', 'production'],
    ['inventoryMaterialsManual', 'inventory'],
    ['costingManual', 'product'],
    ['settingsAuditManual', 'settings'],
    ['customerPortalManual', 'settings'],
    ['requirementsManual', 'requirements'],
  ]
  const keys = primaryMenuKeys(menuGroups)
  for (const [key, groupID] of expectations) {
    assert.ok(keys.includes(key), `${key} should be a primary menu item`)
    assert.equal(groupForView(menuGroups, key)?.id, groupID)
  }
  assert.equal(menuGroups.some((group) => /手册|文档/.test(group.name)), false)
})

test('settings menu exposes sales order settings and keeps sales order detail hidden', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('companyProfile'))
  assert.ok(keys.includes('salesOrderSettings'))
  assert.ok(keys.includes('logisticsSettings'))
  assert.ok(keys.includes('customerPortalSettings'))
  assert.ok(keys.includes('customerPortalManual'))
  assert.ok(keys.includes('customerFulfillment'))
  assert.ok(keys.includes('customerFulfillmentManual'))
  assert.equal(keys.includes('salesOrder'), false)
  assert.equal(groupForView(menuGroups, 'companyProfile')?.id, 'settings')
  assert.equal(groupForView(menuGroups, 'salesOrderSettings')?.id, 'settings')
  assert.equal(groupForView(menuGroups, 'logisticsSettings')?.id, 'settings')
  assert.equal(groupForView(menuGroups, 'customerPortalSettings')?.id, 'settings')
  assert.equal(groupForView(menuGroups, 'customerPortalManual')?.id, 'settings')
  assert.equal(groupForView(menuGroups, 'customerFulfillment')?.id, 'settings')
  assert.equal(groupForView(menuGroups, 'customerFulfillmentManual')?.id, 'settings')
})

test('finance menu exposes monthly finance workflows as primary pages', () => {
  const keys = primaryMenuKeys(menuGroups)
  for (const key of ['financeDashboard', 'financeExpenses', 'financeClosing', 'financeReport', 'financeSettings', 'financeManual']) {
    assert.ok(keys.includes(key))
    assert.equal(groupForView(menuGroups, key)?.id, 'finance')
  }
})
