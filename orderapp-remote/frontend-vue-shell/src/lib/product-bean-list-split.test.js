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
  assert.equal(menuItem('costing')?.label, '产品价格表')
  assert.equal(menuItem('costing')?.title, '产品价格表')
})

test('SKU settings no longer embeds the product bean-list workspace', () => {
  assert.match(productSettingsSource, /<h2>SKU设置<\/h2>/)
  assert.doesNotMatch(productSettingsSource, /import\s+CostingView\s+from/)
  assert.doesNotMatch(productSettingsSource, /<CostingView\b/)
  assert.doesNotMatch(productSettingsSource, /costing-panel/)
  assert.doesNotMatch(productSettingsSource, /豆单和价格试算会按当前归属切换/)
})

test('SKU settings exposes customer context initialization without the public product form', () => {
  for (const expected of [
    'v-if="!selectedCustomerSkuCustomerID"',
    '/api/customer-fulfillment/customers?limit=200',
    'customerSkuCustomerOptions(customerData)',
    'buildCustomerPublicUsagePayload',
    '/api/product-settings/customer-public-usage',
    'savePublicCategoryUsageForCustomer',
    'savePublicSkuUsageForCustomer',
  ]) {
    assert.match(productSettingsSource, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.match(productSettingsSource, /<div v-if="selectedCustomerSkuCustomerID" class="panel custom-product-panel">/)
  assert.match(productSettingsSource, /<span>是否使用公共商品分类<\/span>/)
  assert.match(productSettingsSource, /<span>是否使用公共SKU<\/span>/)
  assert.doesNotMatch(productSettingsSource, /<span>是否使用商品分类<\/span>/)
  assert.doesNotMatch(productSettingsSource, /v-model="customForm\.customer_id"/)
  assert.doesNotMatch(productSettingsSource, /先在顶部选择客户后创建客户专属 SKU/)
})

test('product bean-list page owns customer context for bean-list previews', () => {
  assert.match(costingSource, /<h2>产品价格表<\/h2>/)
  assert.match(costingSource, /const activeBeanListCustomerID = computed/)
  assert.match(costingSource, /const activeCostingScope = computed/)
  assert.match(costingSource, /const activeCustomerPublicUsage = computed/)
  assert.match(costingSource, /const activeBeanListScopeOptions = computed/)
  assert.match(costingSource, /const customerPublicUsages = ref\(\[\]\)/)
  assert.match(costingSource, /versionListScopeCustomerID\(versionListScope\.value\)/)
  assert.match(costingSource, /syncPublicationScopeFromPageContext/)
  assert.match(costingSource, /filterBeanListItemsForScope\(items\.value,\s*activeCostingScope\.value,\s*activeBeanListCustomerID\.value,\s*activeBeanListScopeOptions\.value\)/)
  assert.match(costingSource, /apiGet\('\/api\/product-settings'\)/)
  assert.doesNotMatch(costingSource, /<strong>发布归属<\/strong>/)
})

test('product bean-list live preview loads customer rule scoped prices', () => {
  assert.match(costingSource, /function beanListURLForCustomerRules/)
  assert.match(costingSource, /params\.set\('customer_id', String\(customerID\)\)/)
  assert.match(costingSource, /apiGet\(beanListURLForCustomerRules\(\)\)/)
  assert.match(costingSource, /watch\(activeBeanListCustomerID/)
})

test('product bean-list generation uses product type categories instead of legacy hard-coded list types', () => {
  for (const expected of [
    'productPriceListTypeOptions',
    'selectedProductTypeCategoryID',
    'selectedProductPriceListType',
    'product_type_category_id',
    'product_type_name',
    'beanListPublicationTypeLabel(row)',
  ]) {
    assert.match(costingSource, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.doesNotMatch(costingSource, /<option value="commercial">商用批发豆单<\/option>/)
  assert.doesNotMatch(costingSource, /<option value="drip">挂耳豆单<\/option>/)
  assert.doesNotMatch(costingSource, /<option value="retail">零售豆单<\/option>/)
  assert.doesNotMatch(costingSource, /<option value="green">生豆豆单<\/option>/)
})

test('product bean-list page does not expose pricing trial workspace', () => {
  assert.doesNotMatch(costingSource, /<div class="section-title">价格试算<\/div>/)
  assert.doesNotMatch(costingSource, /pricingCollapsed/)
  assert.doesNotMatch(costingSource, /保存试算/)
  assert.doesNotMatch(costingSource, /发布价格(?!表)/)
  assert.doesNotMatch(costingSource, /试算批次/)
  assert.doesNotMatch(costingSource, /function createRun/)
  assert.doesNotMatch(costingSource, /function publishRun/)
  assert.match(costingSource, /豆单版本列表/)
  assert.match(costingSource, /生成价格表/)
})

test('product bean-list price source drawer keeps temporary tier trial controls', () => {
  for (const expected of [
    '<span>当前试算</span>',
    '<span>临时试算</span>',
    'v-model="explanationOverrides.green_bean_cost_per_kg"',
    'v-model="explanationOverrides.yield_rate"',
    'v-model="explanationOverrides.margin_rate"',
    '@click="loadPriceExplanation"',
    'function cleanExplanationOverrides',
    'cleanExplanationOverrides()',
    '这里的参数只做临时试算',
  ]) {
    assert.match(costingSource, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
})
