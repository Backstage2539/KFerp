import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { test } from 'node:test'

const root = new URL('..', import.meta.url).pathname

function source(rel) {
  return readFileSync(join(root, rel), 'utf8')
}

test('vue shell remounts page components when switching menu views after SKU settings', () => {
  const app = source('App.vue')
  assert.match(app, /<ProductSettingsView\s+v-else-if="isProductSettingsView"/)
  assert.match(app, /currentKey\.value === 'productSettings' \|\| currentKey\.value === 'products'/)
  assert.match(app, /:key="currentViewIdentity"/)
  assert.match(app, /:is="resolveInternalView\(currentKey\)"/)
  assert.match(app, /markRaw\(internalViews\[key\] \|\| OrdersView\)/)
  assert.match(app, /function isProductSettingsKey\(key\)/)
  assert.match(app, /isProductSettingsKey\(currentKey\.value\) && !isProductSettingsKey\(key\)/)
  assert.match(app, /window\.location\.assign\(relativeURLForHistory\(url\)\)/)
  assert.doesNotMatch(app, /const currentInternalView\s*=/)
})

test('customer portal settings removed bean list version and processing warehouse editors', () => {
  const view = source('views/CustomerPortalSettingsView.vue')
  assert.doesNotMatch(view, /豆单展示版本/)
  assert.doesNotMatch(view, /bean-list-picker/)
  assert.doesNotMatch(view, /processing_warehouse_code/)
  assert.match(view, /客户仓库/)
  assert.match(view, /row\.customer\.warehouses/)
})

test('warehouse inventory supports binding warehouses to customers from the inventory page', () => {
  const view = source('views/WarehouseInventoryView.vue')
  assert.match(view, /绑定客户/)
  assert.match(view, /saveWarehouseCustomerBinding/)
  assert.match(view, /\/api\/stock\/warehouses\/\$\{encodeURIComponent\(selectedWarehouse\.value\)\}\/customer/)
})

test('customer drawer exposes inline add actions for customer type and order type selectors', () => {
  const view = source('views/CustomersView.vue')
  assert.match(view, /新增客户类型/)
  assert.match(view, /createCustomerTypeOption/)
  assert.match(view, /新增订单类型/)
  assert.match(view, /createOrderTypeOption/)
})
