import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'

import { menuGroups } from './menu-ia.js'

const here = dirname(fileURLToPath(import.meta.url))
const productSettingsSource = readFileSync(resolve(here, '../views/ProductSettingsView.vue'), 'utf8')
const costingSource = readFileSync(resolve(here, '../views/CostingView.vue'), 'utf8')

function menuItem(key) {
  return menuGroups.flatMap((group) => group.items).find((item) => item.key === key)
}

test('product menu is split into SKU settings and product bean-list pages', () => {
  assert.equal(menuItem('productSettings')?.label, 'SKU设置')
  assert.equal(menuItem('productSettings')?.title, 'SKU设置')
  assert.equal(menuItem('costing')?.label, '产品豆单')
  assert.equal(menuItem('costing')?.title, '产品豆单')
})

test('SKU settings no longer embeds the product bean-list workspace', () => {
  assert.match(productSettingsSource, /<h2>SKU设置<\/h2>/)
  assert.doesNotMatch(productSettingsSource, /import\s+CostingView\s+from/)
  assert.doesNotMatch(productSettingsSource, /<CostingView\b/)
  assert.doesNotMatch(productSettingsSource, /costing-panel/)
  assert.doesNotMatch(productSettingsSource, /豆单和价格试算会按当前归属切换/)
})

test('product bean-list page owns customer context for bean-list previews', () => {
  assert.match(costingSource, /<h2>产品豆单<\/h2>/)
  assert.match(costingSource, /const activeBeanListCustomerID = computed/)
  assert.match(costingSource, /if \(normalizedCustomerContextID\.value > 0\) return 'customer'/)
  assert.match(costingSource, /filterBeanListItemsForScope\(items\.value,\s*activeCostingScope\.value,\s*activeBeanListCustomerID\.value\)/)
})

test('product bean-list page does not expose pricing trial workspace', () => {
  assert.doesNotMatch(costingSource, /试算/)
  assert.doesNotMatch(costingSource, /价格试算/)
  assert.doesNotMatch(costingSource, /pricingCollapsed/)
  assert.doesNotMatch(costingSource, /保存试算/)
  assert.doesNotMatch(costingSource, /发布价格/)
  assert.doesNotMatch(costingSource, /试算批次/)
  assert.doesNotMatch(costingSource, /function createRun/)
  assert.doesNotMatch(costingSource, /function publishRun/)
  assert.match(costingSource, /豆单版本列表/)
  assert.match(costingSource, /生成豆单/)
})
