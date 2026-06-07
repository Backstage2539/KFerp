import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const viewSource = readFileSync(resolve(here, '../views/CostingView.vue'), 'utf8')

test('product bean-list view exposes publication versions without pricing trial workspace', () => {
  const versionListIndex = viewSource.indexOf('已发布价格表')

  assert.ok(versionListIndex > -1, 'missing visible product price-list version section')
  assert.equal(viewSource.indexOf('价格试算'), -1, '产品价格表 should not expose the pricing trial workspace')
  assert.equal(viewSource.indexOf('pricingCollapsed'), -1, 'pricing trial collapse state should be removed from 产品价格表')

  for (const expected of [
    'v-model.number="selectedProductTypeCategoryID"',
    'productPriceListTypeOptions',
    'selectedProductPriceListType',
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
    "apiSend(`/api/costing/bean-list/publications/${row.id}/pdf?${params.toString()}`",
    'await downloadBeanListPublicationPDF(document)',
    'async function downloadBeanListPublicationPDF',
    'apiFetch(document.download_url)',
    'URL.createObjectURL(blob)',
    'function beanListPublicationHasContent',
  ]) {
    assert.ok(viewSource.includes(expected), `missing publication snapshot download behavior: ${expected}`)
  }
  const downloadStart = viewSource.indexOf('async function downloadBeanListPublication(row)')
  const downloadEnd = viewSource.indexOf('function beanListPublicationDownloadParams', downloadStart)
  assert.ok(downloadStart > -1 && downloadEnd > downloadStart, 'downloadBeanListPublication function not found')
  const downloadSource = viewSource.slice(downloadStart, downloadEnd)
  assert.doesNotMatch(downloadSource, /nextTick\(\)/)
  assert.doesNotMatch(downloadSource, /generateBeanListPdf\(\)/)
})

test('product bean-list generate PDF saves preview snapshot through backend instead of printing', () => {
  const generateStart = viewSource.indexOf('async function generateBeanListPdf()')
  const generateEnd = viewSource.indexOf('async function publishBeanList()', generateStart)
  assert.ok(generateStart > -1 && generateEnd > generateStart, 'generateBeanListPdf function not found')
  const generateSource = viewSource.slice(generateStart, generateEnd)

  for (const expected of [
    "apiSend('/api/costing/bean-list/drafts'",
    'beanListPublicationPayload()',
    "apiSend(`/api/costing/bean-list/publications/${row.id}/pdf?${params.toString()}`",
    'await downloadBeanListPublicationPDF(document)',
    "await loadBeanListPublications(listType, publicationScope.value, productTypeCategoryID, 'factory_supply')",
    'await loadBeanListPublications(listType, versionListScope.value, productTypeCategoryID)',
  ]) {
    assert.ok(generateSource.includes(expected), `missing backend preview PDF generation behavior: ${expected}`)
  }
  assert.doesNotMatch(generateSource, /window\.print/)
  assert.doesNotMatch(generateSource, /pdfPrinting\.value = true/)
  assert.doesNotMatch(generateSource, /bean-list-pdf-printing/)
})

test('product price-list version scope selector lists public and each fulfillment customer', () => {
  const versionListStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const versionListEnd = viewSource.indexOf('<section class="panel">', versionListStart)
  assert.ok(versionListStart > -1 && versionListEnd > versionListStart, 'missing bean-list version panel')
  const versionListSource = viewSource.slice(versionListStart, versionListEnd)

  const pageScopeStart = viewSource.indexOf('<div class="bean-list-global-scope">')
  const pageScopeEnd = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  assert.ok(pageScopeStart > -1 && pageScopeEnd > pageScopeStart, 'missing top-level bean-list scope selector')
  const pageScopeSource = viewSource.slice(pageScopeStart, pageScopeEnd)

  assert.match(pageScopeSource, /v-model="versionListScope"/)
  assert.match(pageScopeSource, /<option value="official">公共价格表<\/option>/)
  assert.match(pageScopeSource, /v-for="customer in customers"/)
  assert.match(pageScopeSource, /:value="`customer:\$\{customer\.id\}`"/)
  assert.match(pageScopeSource, /customerOptionLabel\(customer\)/)
  assert.doesNotMatch(versionListSource, /v-model="versionListScope"/)
  assert.match(versionListSource, /v-model\.number="selectedProductTypeCategoryID"/)
  assert.match(versionListSource, /v-for="type in productPriceListTypeOptions"/)
  assert.match(versionListSource, /:value="type\.id"/)
  assert.match(versionListSource, /beanListPublicationTypeLabel\(row\)/)
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

test('product bean-list legacy global publication row displays as current selected product type', () => {
  const labelStart = viewSource.indexOf('function beanListPublicationTypeLabel(row)')
  const labelEnd = viewSource.indexOf('function selectProductTypeFromPublication', labelStart)
  assert.ok(labelStart > -1 && labelEnd > labelStart, 'beanListPublicationTypeLabel function not found')
  const labelSource = viewSource.slice(labelStart, labelEnd)

  assert.match(labelSource, /activeProductTypeCategoryID\.value/)
  assert.match(labelSource, /matchesCurrentPublicationProductType\(row, activeTypeID\)/)
  assert.match(labelSource, /selectedProductPriceListLabel\.value/)
})

test('product price-list version list supports factory supply and customer resale purpose filter', () => {
  const versionListStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const versionListEnd = viewSource.indexOf('<section class="panel">', versionListStart)
  assert.ok(versionListStart > -1 && versionListEnd > versionListStart, 'missing bean-list version panel')
  const versionListSource = viewSource.slice(versionListStart, versionListEnd)

  for (const expected of [
    'v-model="publicationPurposeFilter"',
    '<option value="factory_supply">工厂供货价格表</option>',
    '<option value="customer_resale">客户转售价格表</option>',
    'function beanListPublicationPurposeLabel',
    'publication_purpose',
    "params.set('publication_purpose', publicationPurposeFilter.value)",
  ]) {
    assert.ok(viewSource.includes(expected) || versionListSource.includes(expected), `missing publication purpose filter behavior: ${expected}`)
  }
})

test('product bean-list generate area uses dynamic collapsible product-type sections including green beans', () => {
  for (const expected of [
    'collapsible-bean-section',
    "beanListPreviewCollapsed",
    'productPriceListPreviewSections',
    'buildProductPriceListTypeOptions',
    'priceListRenderTypeForItem',
    'productPriceListTypeKey',
    'toggleBeanListPreviewSection(section.key)',
    "section.listType === 'green'",
    "greenTierPriceRows",
    "green_bean_list",
    "green_bean_sale_tiers",
    "beanListTypeLabel(listType)",
  ]) {
    assert.ok(viewSource.includes(expected), `missing collapsible bean-list preview behavior: ${expected}`)
  }
  assert.doesNotMatch(viewSource, /生成挂耳豆单/)
  assert.doesNotMatch(viewSource, /openBeanListDrawer\('drip'\)/)
})

test('product price list uses classification templates and categories instead of legacy product types', () => {
  for (const expected of [
    '<h2>商品价格表</h2>',
    'Price List / Item Price',
    'data-pr440-price-list-model',
    '商品 &gt; 子组 &gt; 父组 &gt; 默认',
    '平铺价格行',
    '分组项选品',
    'classification_template_id',
    'classification_template_name',
    'classification_category_id',
    'classification_category_name',
    'buildClassificationPriceListTypeOptions',
    'classificationCategoryIDOfItem',
    'classificationTemplateNameOfItem',
  ]) {
    assert.ok(viewSource.includes(expected), `missing classification price-list behavior: ${expected}`)
  }
  assert.doesNotMatch(viewSource, /<h2>产品价格表<\/h2>/)
  assert.doesNotMatch(viewSource, /<span>产品类型<\/span>/)
  assert.doesNotMatch(viewSource, /按当前价格表归属和商品管理里的产品类型生成/)
  assert.doesNotMatch(viewSource, /按当前价格表归属、商品当前归类和客户商品生成商品价格表/)
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

test('product bean-list view maps green and commercial fields without dedicated drip inference', () => {
  for (const expected of [
    "if (listType === 'green') return 'green_bean_list'",
    "if (listType === 'green') return 'green_bean_sale_tiers'",
    'function priceListRenderTypeForItem',
    'function beanListPublicationTypeKey',
    'product_type_category_id',
    'product_type_name',
    'selectedProductIDsByType.value = {}',
  ]) {
    assert.ok(viewSource.includes(expected), `missing bean-list type mapping: ${expected}`)
  }
  for (const forbidden of [
    "if (kind === 'drip_bag') return 'drip'",
    "categoryHint.includes('挂耳')",
    "section.listType === 'drip'",
    "openDripPriceExplanation",
  ]) {
    assert.doesNotMatch(viewSource, new RegExp(forbidden.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
})

test('product bean-list view exposes manual green bean tier price editing', () => {
  for (const expected of [
    'green-tier-price-editor',
    'green-inline-price-editor',
    '梯度按 KG，单价按元/KG',
    '生成并发布新版价格表后，录单才会使用新价格',
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
    'defaultBeanListVersionForScope(listType, productTypeCategoryID = activeProductTypeCategoryID.value)',
    'version: defaultBeanListVersionForScope(resolvedListType, activeProductTypeCategoryID.value)',
  ]) {
    assert.ok(viewSource.includes(expected), `missing customer bean-list version default behavior: ${expected}`)
  }
})

test('product bean-list warns when a green bean item has no green category template', () => {
  for (const expected of [
    "item?.bom_status === 'missing_green_bean_template'",
    '未挂到带生豆模板的分类，无法生成生豆价格',
    '请在商品管理里把该生豆商品移到带生豆模板的生豆分类',
  ]) {
    assert.ok(viewSource.includes(expected), `missing green bean category warning: ${expected}`)
  }
})
