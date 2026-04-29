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
  assert.ok(keys.includes('materials'))
  assert.equal(keys.includes('materialBatches'), false)
  assert.equal(keys.includes('stockBatches'), false)
  assert.equal(keys.includes('stockLedger'), false)
  assert.equal(keys.includes('inventory'), false)
})

test('warehouse inventory and legacy stock views resolve to the inventory group', () => {
  assert.equal(groupForView(menuGroups, 'warehouseInventory')?.id, 'inventory')
  assert.equal(groupForView(menuGroups, 'stockOperations')?.id, 'inventory')
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
  assert.equal(keys.includes('products'), false)
  assert.equal(keys.includes('costing'), false)
  assert.equal(groupForView(menuGroups, 'productSettings')?.id, 'product')
})

test('production menu exposes the production flow manual as a primary page', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('productionManual'))
  assert.equal(groupForView(menuGroups, 'productionManual')?.id, 'production')
})

test('settings menu exposes sales order settings and keeps sales order detail hidden', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('salesOrderSettings'))
  assert.equal(keys.includes('salesOrder'), false)
  assert.equal(groupForView(menuGroups, 'salesOrderSettings')?.id, 'settings')
})
