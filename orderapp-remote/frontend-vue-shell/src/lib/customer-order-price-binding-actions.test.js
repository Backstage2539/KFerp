import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const source = fs.readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
const start = source.indexOf('class="customer-order-price-bindings"')
const end = source.indexOf('class="version-bulk-actions"', start)
const bindingPanel = source.slice(start, end)

test('customer order price table bindings use one shared save and cancel action set', () => {
  assert.ok(start >= 0 && end > start)
  assert.equal((bindingPanel.match(/>保存<\/button>/g) || []).length, 1)
  assert.equal((bindingPanel.match(/>取消<\/button>/g) || []).length, 1)
  assert.doesNotMatch(bindingPanel, /保存指定|取消指定/)
  assert.match(bindingPanel, /saveCustomerOrderPriceTableBindings/)
  assert.match(bindingPanel, /cancelCustomerOrderPriceTableBindings/)
  assert.match(source, /persistCustomerOrderPriceTableBinding\('direct_ship', 0\)/)
  assert.match(source, /persistCustomerOrderPriceTableBinding\('product_order', 0\)/)
})
