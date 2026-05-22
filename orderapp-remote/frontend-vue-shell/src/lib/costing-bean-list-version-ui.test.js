import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const viewSource = readFileSync(resolve(here, '../views/CostingView.vue'), 'utf8')

test('product bean-list view exposes publication versions without pricing trial workspace', () => {
  const versionListIndex = viewSource.indexOf('豆单版本列表')

  assert.ok(versionListIndex > -1, 'missing visible bean-list version list section')
  assert.equal(viewSource.indexOf('价格试算'), -1, '产品豆单 should not expose the pricing trial workspace')
  assert.equal(viewSource.indexOf('pricingCollapsed'), -1, 'pricing trial collapse state should be removed from 产品豆单')

  for (const expected of [
    'v-model="pdfOptions.listType"',
    'currentScopePublicationRows',
    'function beanListPublicationStatusLabel',
    'function beanListPublicationStatusClass',
    'function beanListPublicationTime',
    'function publicationScopeLabel',
    'function beanListPublicationOwnerLabel',
    'function beanListPublicationSourceLabel',
    'function startBeanListFromPublication',
    'withdrawBeanList(row)',
  ]) {
    assert.ok(viewSource.includes(expected), `missing version list behavior: ${expected}`)
  }

  for (const forbidden of [
    'v-model="publicationScope"',
    'applyCopiedBeanListPublicationConfig(row)',
    'selectedCopyPublicationID',
    '复制已有豆单配置',
  ]) {
    assert.equal(viewSource.includes(forbidden), false, `old bean-list copy/scope behavior should be removed: ${forbidden}`)
  }
})

test('product bean-list version list downloads the selected publication snapshot', () => {
  const versionListStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const versionListEnd = viewSource.indexOf('<section class="panel">', versionListStart)
  assert.ok(versionListStart > -1 && versionListEnd > versionListStart, 'missing bean-list version panel')
  const versionListSource = viewSource.slice(versionListStart, versionListEnd)

  for (const expected of [
    '下载 PDF',
    '@click="downloadBeanListPublication(row)"',
    ':disabled="!beanListPublicationHasContent(row)"',
  ]) {
    assert.ok(versionListSource.includes(expected), `missing version-list download action: ${expected}`)
  }
  for (const expected of [
    'const downloadSourcePublication = ref(null)',
    'downloadSourcePublication.value || currentPriceSourcePublication.value',
    'function downloadBeanListPublication',
    'beanListPublicationPdfOptions(row, pdfOptions.value)',
    'await nextTick()',
    'generateBeanListPdf()',
    'function beanListPublicationHasContent',
  ]) {
    assert.ok(viewSource.includes(expected), `missing publication snapshot download behavior: ${expected}`)
  }
})

test('product bean-list version scope selector lists public and each fulfillment customer', () => {
  const versionListStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const versionListEnd = viewSource.indexOf('<section class="panel">', versionListStart)
  assert.ok(versionListStart > -1 && versionListEnd > versionListStart, 'missing bean-list version panel')
  const versionListSource = viewSource.slice(versionListStart, versionListEnd)

  const pageScopeStart = viewSource.indexOf('<div class="bean-list-global-scope">')
  const pageScopeEnd = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  assert.ok(pageScopeStart > -1 && pageScopeEnd > pageScopeStart, 'missing top-level bean-list scope selector')
  const pageScopeSource = viewSource.slice(pageScopeStart, pageScopeEnd)

  assert.match(pageScopeSource, /v-model="versionListScope"/)
  assert.match(pageScopeSource, /<option value="official">公共豆单<\/option>/)
  assert.match(pageScopeSource, /v-for="customer in customers"/)
  assert.match(pageScopeSource, /:value="`customer:\$\{customer\.id\}`"/)
  assert.match(pageScopeSource, /customerOptionLabel\(customer\)/)
  assert.doesNotMatch(versionListSource, /v-model="versionListScope"/)
  assert.match(versionListSource, /<option value="drip">挂耳豆单<\/option>/)
  assert.doesNotMatch(versionListSource, /fulfillment_customers/)
  assert.doesNotMatch(versionListSource, /所有履约客户豆单/)
  assert.doesNotMatch(versionListSource, /棵凡官方豆单/)
  assert.doesNotMatch(versionListSource, /我的客户豆单/)
  assert.doesNotMatch(versionListSource, /指定客户豆单/)
  assert.doesNotMatch(versionListSource, /version-control-customer/)
  assert.doesNotMatch(versionListSource, /openBeanListDrawer\(pdfTheme\.listType\)/)
  assert.match(viewSource, /function versionListScopeCustomerID/)
  assert.match(viewSource, /function beanListPublicationRequestScope/)
  assert.match(viewSource, /function beanListPublicationCacheKey/)
  assert.match(viewSource, /const versionListCurrentPublication = computed/)
  assert.match(viewSource, /const publicationScopeRows = computed/)
  assert.match(viewSource, /function syncPublicationScopeFromPageContext/)
})

test('product bean-list generate area uses collapsible bean-list sections including green beans', () => {
  for (const expected of [
    'collapsible-bean-section',
    "beanListPreviewCollapsed",
    "toggleBeanListPreviewSection('commercial')",
    "toggleBeanListPreviewSection('drip')",
    "toggleBeanListPreviewSection('retail')",
    "toggleBeanListPreviewSection('green')",
    "greenGroups",
    "green_bean_list",
    "green_bean_sale_tiers",
    "生豆豆单",
  ]) {
    assert.ok(viewSource.includes(expected), `missing collapsible bean-list preview behavior: ${expected}`)
  }
  assert.doesNotMatch(viewSource, /生成挂耳豆单/)
  assert.doesNotMatch(viewSource, /openBeanListDrawer\('drip'\)/)
})

test('product bean-list drawer derives publication owner from current page scope', () => {
  for (const expected of [
    '当前归属',
    'currentPublicationOwnerLabel',
    'currentPublicationScopeDescription',
    'syncPublicationScopeFromPageContext',
    'versionListScopeCustomerID(versionListScope.value)',
    "publicationScope.value = 'customer'",
    "publicationScope.value = 'official'",
  ]) {
    assert.ok(viewSource.includes(expected), `missing derived publication owner behavior: ${expected}`)
  }
  assert.doesNotMatch(viewSource, /<strong>发布归属<\/strong>/)
  assert.doesNotMatch(viewSource, /<strong>客户<\/strong>/)
  assert.doesNotMatch(viewSource, /<SearchableSelect[\s\S]*selectedBeanListCustomerID/)
})

test('product bean-list view maps every bean-list type to its own metadata and tier fields', () => {
  for (const expected of [
    "if (listType === 'green') return 'green_bean_list'",
    "if (listType === 'green') return 'green_bean_sale_tiers'",
    'official: { commercial: [], drip: [], retail: [], green: [] }',
    'selectedProductIDsByType.value = { commercial: [], drip: [], retail: [], green: [] }',
    "if (normalized === 'drip') return '挂耳'",
  ]) {
    assert.ok(viewSource.includes(expected), `missing bean-list type mapping: ${expected}`)
  }
})

test('product bean-list view exposes manual green bean tier price editing', () => {
  for (const expected of [
    'green-tier-price-editor',
    'green-inline-price-editor',
    '梯度按 KG，单价按元/KG',
    '生成并发布新版豆单后，录单才会使用新价格',
    '保存生豆价格',
    'saveGreenBeanPriceDraft',
    'greenTierPriceRows(row)',
    'greenTierPriceRows(item)',
    'setGreenBeanTierPrice(itemProductID(row), tier, $event.target.value)',
    'setGreenBeanTierPrice(itemProductID(item), tier, $event.target.value)',
    'function setGreenBeanTierPrice',
    'greenPriceOverrides',
    "listType: 'green'",
    "'/api/costing/bean-list/drafts'",
  ]) {
    assert.ok(viewSource.includes(expected), `missing green bean tier price editing: ${expected}`)
  }
})

test('product bean-list drawer defaults customer versions from the latest source plus one step', () => {
  for (const expected of [
    'defaultBeanListDraftVersion',
    'defaultBeanListVersionForScope(listType)',
    'pdfOptions.value = { ...pdfOptions.value, listType, version: defaultBeanListVersionForScope(listType) }',
  ]) {
    assert.ok(viewSource.includes(expected), `missing customer bean-list version default behavior: ${expected}`)
  }
})

test('product bean-list warns when a green bean item has no green category template', () => {
  for (const expected of [
    "item?.bom_status === 'missing_green_bean_template'",
    '未挂到带生豆模板的分类，无法生成生豆价格',
    '请在 SKU设置 里把该生豆 SKU 移到带生豆模板的生豆分类',
  ]) {
    assert.ok(viewSource.includes(expected), `missing green bean category warning: ${expected}`)
  }
})
