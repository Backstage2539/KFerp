import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'

import { menuGroups } from './menu-ia.js'

const here = dirname(fileURLToPath(import.meta.url))
const productSettingsSource = readFileSync(resolve(here, '../views/ProductSettingsView.vue'), 'utf8')
const costingSource = readFileSync(resolve(here, '../views/CostingView.vue'), 'utf8')
const beanListPdfSource = readFileSync(resolve(here, './bean-list-pdf.js'), 'utf8')

function menuItem(key) {
  return menuGroups.flatMap((group) => group.items).find((item) => item.key === key)
}

test('product menu is split into product archive, customer names, template and price-list pages', () => {
  assert.equal(menuItem('productMaster')?.label, '商品档案')
  assert.equal(menuItem('productMaster')?.title, '商品档案')
  assert.equal(menuItem('customerProductAliases')?.label, '客户商品名')
  assert.equal(menuItem('productConfigTemplates')?.label, '商品配置和分类模板')
  assert.equal(menuItem('costing')?.label, '商品价格表')
  assert.equal(menuItem('costing')?.title, '商品价格表')
})

test('product archive no longer embeds the product bean-list workspace', () => {
  assert.match(productSettingsSource, /productSectionTitle/)
  assert.match(productSettingsSource, /商品档案/)
  assert.match(productSettingsSource, /客户商品名/)
  assert.match(productSettingsSource, /商品配置模板/)
  assert.doesNotMatch(productSettingsSource, /import\s+CostingView\s+from/)
  assert.doesNotMatch(productSettingsSource, /<CostingView\b/)
  assert.doesNotMatch(productSettingsSource, /costing-panel/)
  assert.doesNotMatch(productSettingsSource, /豆单和价格试算会按当前归属切换/)
})

test('SKU settings exposes customer context initialization with product archive creation in a drawer', () => {
  for (const expected of [
    '/api/customer-fulfillment/customers?limit=200',
    'customerSkuCustomerOptions(customerData)',
    'buildSkuCreatePayload',
    '/api/product-settings/products/${row.id}/copy',
    'buildCustomerPublicUsagePayload',
    '/api/product-settings/customer-public-usage',
    'savePublicCategoryUsageForCustomer',
    '创建新商品档案',
  ]) {
    assert.match(productSettingsSource, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.match(productSettingsSource, /class="settings-drawer product-editor-drawer"/)
  assert.match(productSettingsSource, /class="sku-create-form product-create-form product-drawer-form" @submit\.prevent="createSku"/)
  assert.doesNotMatch(productSettingsSource, /class="settings-drawer sku-copy-drawer"/)
  assert.match(productSettingsSource, /增加分类/)
  assert.doesNotMatch(productSettingsSource, /<span>是否使用公共SKU<\/span>/)
  assert.doesNotMatch(productSettingsSource, /<span>是否使用商品分类<\/span>/)
  assert.doesNotMatch(productSettingsSource, /v-model="customForm\.customer_id"/)
  assert.doesNotMatch(productSettingsSource, /先在顶部选择客户后创建客户专属 SKU/)
  assert.doesNotMatch(productSettingsSource, /<form v-if="!selectedCustomerSkuCustomerID" class="product-create-form product-drawer-form" @submit\.prevent="createProduct">/)
  assert.doesNotMatch(productSettingsSource, /<form v-else class="custom-product-form product-drawer-form" @submit\.prevent="createCustomProduct">/)
})

test('product and customer classification actions are split into separate action cards', () => {
  assert.match(productSettingsSource, /classification-action-card add-classification-card[\s\S]*<span>增加分类<\/span>/)
  assert.match(productSettingsSource, /classification-action-card move-classification-card[\s\S]*移动到分类[\s\S]*移动到子类/)
  const toolbarStart = productSettingsSource.indexOf('product-classification-tabs')
  const toolbarEnd = productSettingsSource.indexOf('<div class="table-wrap sku-table-wrap">')
  assert.notEqual(toolbarStart, -1)
  assert.notEqual(toolbarEnd, -1)
  const toolbar = productSettingsSource.slice(toolbarStart, toolbarEnd)
  assert.match(toolbar, /add-classification-card/)
  assert.match(toolbar, /move-classification-card/)
})

test('customer alias list has shared filters batch disable and industry field controls', () => {
  assert.match(productSettingsSource, /alias-filters/)
  assert.match(productSettingsSource, /aliasFilters\.query/)
  assert.match(productSettingsSource, /aliasFilters\.active/)
  assert.match(productSettingsSource, /batchDisableCustomerProductAliases/)
  assert.match(productSettingsSource, /客户行业字段/)
  assert.match(productSettingsSource, /openAliasIndustryFieldDrawer/)
})

test('classification template editor keeps template actions at bottom and category templates side by side', () => {
  assert.match(productSettingsSource, /classification-template-actions-bottom[\s\S]*保存分类模板[\s\S]*删除模板/)
  assert.match(productSettingsSource, /classification-category-template-row[\s\S]*分类项阶梯价模板[\s\S]*分类项单位模板/)
  assert.match(productSettingsSource, /classification-template-create-fields/)
})

test('industry field template page uses simplified key-only fields and space separated select options', () => {
  const source = readFileSync(resolve(here, '../views/IndustryFieldTemplatesView.vue'), 'utf8')
  assert.match(source, /industry-template-layout/)
  assert.match(source, /模板列表/)
  assert.match(source, /industryTemplateFilters\.query/)
  assert.match(source, /industryTemplateFilters\.status/)
  assert.match(source, /filteredIndustryTemplateRows/)
  assert.match(source, /placeholder="搜索模板名"/)
  assert.match(source, /<option value="active">启用<\/option>/)
  assert.match(source, /<option value="inactive">停用<\/option>/)
  assert.match(source, /class="panel industry-template-list-panel"/)
  assert.match(source, /class="panel editor industry-template-editor-panel"/)
  assert.match(source, /字段键/)
  assert.match(source, /空格分隔/)
  assert.match(source, /输入默认文本/)
  assert.match(source, /options_text/)
  assert.match(source, /split\(\/\\s\+\/\)/)
  assert.doesNotMatch(source, /选项 JSON/)
  assert.doesNotMatch(source, /<span>行业键<\/span>/)
  assert.doesNotMatch(source, /<th>行业键<\/th>/)
  assert.doesNotMatch(source, /<span>显示名<\/span>/)
  assert.doesNotMatch(source, /<span>单位<\/span>/)
  assert.doesNotMatch(source, /<span>必填<\/span>/)
})

test('product and customer alias tables expose industry field columns', () => {
  assert.match(productSettingsSource, /<th>行业字段<\/th>/)
  assert.match(productSettingsSource, /industryFieldSummary/)
})

test('product bean-list page owns customer context for bean-list previews', () => {
  assert.match(costingSource, /<h2>商品价格表<\/h2>/)
  assert.match(costingSource, /价格表归属/)
  assert.match(costingSource, /const activeBeanListCustomerID = computed/)
  assert.match(costingSource, /const activeCostingScope = computed/)
  assert.match(costingSource, /const customerProductAliases = ref\(\[\]\)/)
  assert.match(costingSource, /activePriceListCustomerAliases/)
  assert.match(costingSource, /versionListScopeCustomerID\(versionListScope\.value\)/)
  assert.match(costingSource, /syncPublicationScopeFromPageContext/)
  assert.match(costingSource, /applyCustomerProductAliasesToBeanListItems\(scoped,\s*activePriceListCustomerAliases\.value,\s*activeBeanListCustomerID\.value\)/)
  assert.match(costingSource, /apiGet\('\/api\/customer-product-aliases\?active=all'\)/)
  assert.doesNotMatch(costingSource, /<strong>发布归属<\/strong>/)
  assert.doesNotMatch(costingSource, /豆单范围/)
})

test('product bean-list preview and PDF selection reuse alias-filtered visible items', () => {
  const beanListItemsStart = costingSource.indexOf('function beanListItemsForType')
  assert.notEqual(beanListItemsStart, -1)
  const beanListItemsSource = costingSource.slice(beanListItemsStart, beanListItemsStart + 500)

  assert.match(costingSource, /function priceListScopedItems/)
  assert.match(beanListItemsSource, /priceListScopedItems\(\)/)
  assert.doesNotMatch(beanListItemsSource, /scopedBeanListItems/)
  assert.match(costingSource, /const pdfAvailableItems = computed\(\(\) => beanListItemsForType/)
  assert.match(costingSource, /const categoryProductGroups = computed\(\(\) => productGroupsForType/)
  assert.match(costingSource, /const code = classificationCategoryID > 0 \? `classification-category-\$\{classificationCategoryID\}` : \(String\(meta\.code \|\| ''\)\.split\('\.'\)\[0\] \|\| category \|\| '未分类'\)/)
  assert.match(costingSource, /return String\(meta\.code \|\| ''\)\.split\('\.'\)\[0\] \|\| category \|\| '未分类'/)
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
	'classification_template_name',
	'beanListPublicationTypeLabel(row)',
  ]) {
	assert.match(costingSource, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  const helperStart = costingSource.indexOf('function classificationTemplateNameOfItem')
  const helperEnd = costingSource.indexOf('function classificationCategoryNameOfItem')
  assert.notEqual(helperStart, -1)
  assert.notEqual(helperEnd, -1)
  assert.doesNotMatch(costingSource.slice(helperStart, helperEnd), /product_type_name|category_primary_name/)
  assert.doesNotMatch(costingSource, /<option value="commercial">商用批发豆单<\/option>/)
  assert.doesNotMatch(costingSource, /<option value="drip">挂耳豆单<\/option>/)
  assert.doesNotMatch(costingSource, /<option value="retail">零售豆单<\/option>/)
  assert.doesNotMatch(costingSource, /<option value="green">生豆豆单<\/option>/)
})

test('product bean-list publication payload freezes customer alias and product snapshots', () => {
  const source = `${costingSource}\n${beanListPdfSource}`
  for (const expected of [
    'customer_product_alias_id',
    'display_name_snapshot',
    'customer_item_code_snapshot',
    'brand_name_snapshot',
    'display_category_snapshot',
    'product_code_snapshot',
    'product_name_snapshot',
    'bom_version_id_snapshot',
    'bom_usage_mode_snapshot',
    'price_unit_snapshot',
    'tiers_snapshot',
    'special_attrs_snapshot',
    'price_source_json',
  ]) {
    assert.match(source, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
})

test('product bean-list page does not expose pricing trial workspace', () => {
  assert.doesNotMatch(costingSource, /<div class="section-title">价格试算<\/div>/)
  assert.doesNotMatch(costingSource, /pricingCollapsed/)
  assert.doesNotMatch(costingSource, /保存试算/)
  assert.doesNotMatch(costingSource, /发布价格(?!表)/)
  assert.doesNotMatch(costingSource, /试算批次/)
  assert.doesNotMatch(costingSource, /function createRun/)
  assert.doesNotMatch(costingSource, /function publishRun/)
  assert.match(costingSource, /已发布价格表/)
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
