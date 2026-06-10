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
    'downloadSourcePublication.value?.content?.groups',
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
  assert.match(labelSource, /activeTypeID !== 0/)
  assert.match(labelSource, /selectedProductPriceListLabel\.value/)
})

test('product price-list version list hides factory supply and customer resale purpose filter', () => {
  const versionListStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const versionListEnd = viewSource.indexOf('<section class="panel">', versionListStart)
  assert.ok(versionListStart > -1 && versionListEnd > versionListStart, 'missing bean-list version panel')
  const versionListSource = viewSource.slice(versionListStart, versionListEnd)

  assert.doesNotMatch(versionListSource, /v-model="publicationPurposeFilter"/)
  assert.doesNotMatch(versionListSource, /<span>用途<\/span>/)
  assert.doesNotMatch(versionListSource, /<option value="factory_supply">工厂供货价格表<\/option>/)
  assert.doesNotMatch(versionListSource, /<option value="customer_resale">客户转售价格表<\/option>/)
  assert.doesNotMatch(versionListSource, /<th>用途<\/th>/)
  assert.doesNotMatch(versionListSource, /beanListPublicationPurposeLabel\(row\)/)
  assert.doesNotMatch(viewSource, /publicationPurposeFilter/)
  assert.doesNotMatch(viewSource, /客户转售价格表/)
  assert.doesNotMatch(viewSource, /工厂供货价格表/)
  assert.match(viewSource, /publication_purpose/)
  assert.match(viewSource, /FACTORY_SUPPLY_PUBLICATION_PURPOSE/)
})

test('product bean-list generate area uses inline price-list configuration instead of preview cards', () => {
  for (const expected of [
    'buildProductPriceListTypeOptions',
    'priceListRenderTypeForItem',
    'productPriceListTypeKey',
    'price-list-page-config',
    '<strong>Price List / Item Price 生成规则</strong>',
    '<button class="primary" type="button" :disabled="loading || !visibleCostingItems.length || !productPriceListTypeOptions.length" @click="openBeanListDrawer()">价格表配置</button>',
    'aria-label="价格表配置"',
    "greenTierPriceRows",
    "green_bean_list",
    "green_bean_sale_tiers",
    "beanListTypeLabel(listType)",
  ]) {
    assert.ok(viewSource.includes(expected), `missing inline price-list configuration behavior: ${expected}`)
  }
  assert.doesNotMatch(viewSource, /collapsible-bean-section/)
  assert.doesNotMatch(viewSource, /productPriceListPreviewSections/)
  assert.doesNotMatch(viewSource, /生成挂耳豆单/)
  assert.doesNotMatch(viewSource, /openBeanListDrawer\('drip'\)/)
})

test('product price list uses classification templates and categories instead of legacy product types', () => {
  for (const expected of [
    '<h2>商品价格表</h2>',
    'Price List / Item Price',
    'data-pr440-price-list-model',
    '商品 &gt; 子类 &gt; 父类 &gt; 价格表',
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

test('price list mode rules are opened from a button and not shown as a persistent panel', () => {
  const generatePanelStart = viewSource.indexOf('<div class="bean-list-generate-bar">')
  const pageDrawerStart = viewSource.indexOf('<div v-if="settingsOpen"', generatePanelStart)
  assert.ok(generatePanelStart > -1 && pageDrawerStart > generatePanelStart, 'missing generate page block')
  const pageSource = viewSource.slice(generatePanelStart, pageDrawerStart)

  for (const expected of [
    '计价模式规则',
    '@click="priceListRulesDialogOpen = true"',
    'v-if="priceListRulesDialogOpen"',
    'aria-label="计价模式规则"',
    'price-list-rules-dialog',
    '商品 &gt; 子类 &gt; 父类 &gt; 价格表',
    'group_source=price_list',
  ]) {
    assert.ok(pageSource.includes(expected), `missing modal price-list mode rule behavior: ${expected}`)
  }

  const oldPanelStart = pageSource.indexOf('data-pr442-price-list-group-source')
  assert.equal(oldPanelStart, -1, 'rule table should not remain as a persistent model panel')
  assert.doesNotMatch(pageSource, /模板继承规则/)
  assert.doesNotMatch(pageSource, /<table>[\s\S]*<th>层级<\/th>[\s\S]*<\/table>/)
})

test('price list generation renders the product picker as an indented collapsible tree', () => {
  const productSelectionStart = viewSource.indexOf('<div class="pdf-picker productSelection">')
  const dialogStart = viewSource.indexOf('<div v-if="priceListConfigDialog.open"', productSelectionStart)
  assert.ok(productSelectionStart > -1 && dialogStart > productSelectionStart, 'missing product selection block')

  const selectionSource = viewSource.slice(productSelectionStart, dialogStart)
  const categoryStart = selectionSource.indexOf('v-for="category in categoryProductGroups"')
  const categoryHeadStart = selectionSource.indexOf('class="product-picker-category-head"', categoryStart)
  const productRowStart = selectionSource.indexOf('v-for="row in category.items"', categoryHeadStart)
  const productRowEnd = selectionSource.indexOf('<div v-if="pdfTheme.listType ===', productRowStart)
  assert.ok(categoryStart > -1, 'missing category tree row')
  assert.ok(categoryHeadStart > categoryStart, 'missing category tree head')
  assert.ok(productRowStart > categoryHeadStart && productRowEnd > productRowStart, 'missing product row under category')

  const categorySource = selectionSource.slice(categoryStart, productRowStart)
  const productRowSource = selectionSource.slice(productRowStart, productRowEnd)

  for (const expected of [
    ':style="productPickerCategoryStyle(category)"',
    'isProductPickerCategoryCollapsed(category)',
    'toggleProductPickerCategoryCollapse(category)',
    'category-collapse-toggle',
    'aria-label="收起或展开分类"',
    'selectedCountForCategory(category.code)',
  ]) {
    assert.ok(categorySource.includes(expected), `missing collapsible category tree behavior: ${expected}`)
  }

  for (const expected of [
    'v-if="!isProductPickerCategoryCollapsed(category)"',
    ':style="productPickerRowStyle(category)"',
  ]) {
    assert.ok(productRowSource.includes(expected), `missing indented product row behavior: ${expected}`)
  }

  for (const expected of [
    'const productPickerCollapsedCategories = ref({})',
    'function productPickerCategoryStyle',
    'function productPickerRowStyle',
    'function productPickerCategoryDepth',
    'function productPickerCategoryCollapseKey',
    '--product-picker-category-indent',
    '--product-picker-row-indent',
  ]) {
    assert.ok(viewSource.includes(expected), `missing product picker tree helper/style: ${expected}`)
  }
})

test('price list product picker selection uses product-catalog state and cascades category tree behavior', () => {
  for (const expected of [
    'priceListSelectionStateKey',
    'priceListVisibleCategoryRows',
    'priceListCategoryProductIDs',
    'priceListCategoryCodesForSelectedProducts',
    'priceListCategoryHiddenByCollapsedAncestor',
    'function priceListSelectionKey',
    'return priceListVisibleCategoryRows',
    'priceListCategoryProductIDs(categoryProductGroups.value',
    'priceListCategoryCodesForSelectedProducts(categoryProductGroups.value',
    'priceListCategoryHiddenByCollapsedAncestor(categoryProductGroups.value',
    'v-if="!isProductPickerCategoryHiddenByCollapsedAncestor(category)"',
  ]) {
    assert.ok(viewSource.includes(expected), `missing product-catalog picker selection behavior: ${expected}`)
  }
})

test('price list generation keeps A/B positions as summaries and edits pricing in an anchored popover', () => {
  const builderStart = viewSource.indexOf('<div class="pdf-picker price-list-template-builder"')
  const productSelectionStart = viewSource.indexOf('<div class="pdf-picker productSelection">')
  const dialogStart = viewSource.indexOf('<div v-if="priceListConfigDialog.open"', productSelectionStart)
  const flatRowStart = viewSource.indexOf('<div v-if="priceListFlatRows.length" class="pdf-picker flat-price-row-editor">')
  assert.ok(builderStart > -1 && productSelectionStart > builderStart, 'missing price-list builder followed by product selection')
  assert.ok(dialogStart > productSelectionStart, 'missing display config dialog after product selection')
  assert.ok(flatRowStart > productSelectionStart, 'missing conditional flat-row editor after product selection')

  const builderSource = viewSource.slice(builderStart, productSelectionStart)
  const selectionSource = viewSource.slice(productSelectionStart, dialogStart)
  const dialogSource = viewSource.slice(dialogStart, flatRowStart)
  const categoryHeadStart = selectionSource.indexOf('class="product-picker-category-head"')
  const categoryHeadEnd = selectionSource.indexOf('v-for="row in category.items"', categoryHeadStart)
  const productRowStart = selectionSource.indexOf('v-for="row in category.items"')
  const productRowEnd = selectionSource.indexOf('<div v-if="pdfTheme.listType ===', productRowStart)
  assert.ok(categoryHeadStart > -1 && categoryHeadEnd > categoryHeadStart, 'missing category selection head block')
  assert.ok(productRowStart > -1 && productRowEnd > productRowStart, 'missing product selection row block')

  const categoryHeadSource = selectionSource.slice(categoryHeadStart, categoryHeadEnd)
  const productRowSource = selectionSource.slice(productRowStart, productRowEnd)

  for (const forbidden of ['父类计价配置', '子类计价配置', '商品行', 'product-override-row']) {
    assert.equal(builderSource.includes(forbidden), false, `old standalone pricing config should be removed from builder: ${forbidden}`)
  }

  for (const expected of [
    'category-pricing-summary',
    'priceListCategoryPricingSummary(category)',
    "openPriceListPricingPopover('category', category)",
    "isPriceListPricingPopoverOpen('category', category)",
  ]) {
    assert.ok(categoryHeadSource.includes(expected), `missing A-position category summary: ${expected}`)
  }

  for (const expected of [
    'product-compact-status',
    'priceListProductPricingSummary(priceListProductRowForItem(row))',
    'priceListProductDisplaySummary(itemProductID(row))',
    "openPriceListPricingPopover('product', priceListProductRowForItem(row))",
    "isPriceListPricingPopoverOpen('product', priceListProductRowForItem(row))",
    'openPriceListProductDisplayDialog(itemProductID(row))',
  ]) {
    assert.ok(productRowSource.includes(expected), `missing B-position product summary: ${expected}`)
  }

  for (const forbidden of [
    'category-inline-pricing-config',
    'product-inline-pricing-config',
    'product-display-config',
    '父类计价',
    '子类计价',
    '商品行计价',
  ]) {
    assert.equal(selectionSource.includes(forbidden), false, `selection list should not render inline config: ${forbidden}`)
  }

  for (const expected of [
    'price-list-pricing-popover',
    'priceListPricingPopoverOptions',
    '继承分类',
    '按阶梯模板价计算',
    '按价格模板计算',
    '固定价',
    'setPriceListPricingPopoverMode(option.value)',
    "setPriceListPricingPopoverField('tier_template_id'",
    "setPriceListPricingPopoverField('pricing_rule_id'",
    "setPriceListPricingPopoverField('fixed_unit_price'",
  ]) {
    assert.ok(selectionSource.includes(expected) || viewSource.includes(expected), `missing anchored pricing popover behavior: ${expected}`)
  }

  for (const expected of [
    'price-list-config-dialog',
    '商品展示',
    'setCustomizerField(priceListConfigDialog.productId',
  ]) {
    assert.ok(dialogSource.includes(expected), `missing display dialog behavior: ${expected}`)
  }
  for (const forbidden of [
    "priceListConfigDialog.type === 'category-pricing'",
    "priceListConfigDialog.type === 'product-pricing'",
    '上级分类计价',
    '本分类计价',
    '商品计价',
  ]) {
    assert.equal(dialogSource.includes(forbidden), false, `pricing should not remain in bottom-right dialog: ${forbidden}`)
  }
})

test('price list generation persists pricing drafts and applies tier-template trial results', () => {
  for (const expected of [
    'savePriceListGenerationDraft(',
    'restorePriceListGenerationDraftForActiveType',
    'productCatalogBusinessGroupRowsForPriceList',
  ]) {
    assert.ok(viewSource.includes(expected), `missing price-list draft/group persistence behavior: ${expected}`)
  }

  const flatRowStart = viewSource.indexOf('function priceListFlatRowFromSource')
  const flatRowEnd = viewSource.indexOf('function priceListPricingRuleTrialResultForRow', flatRowStart)
  assert.ok(flatRowStart > -1 && flatRowEnd > flatRowStart, 'priceListFlatRowFromSource block not found')
  const flatRowSource = viewSource.slice(flatRowStart, flatRowEnd)
  assert.match(flatRowSource, /mode === 'pricing_rule' \|\| mode === 'tier_template'/)
})

test('price list product selection summaries avoid parent child wording and inherited rows say category inheritance', () => {
  const productSelectionStart = viewSource.indexOf('<div class="pdf-picker productSelection">')
  const dialogStart = viewSource.indexOf('<div v-if="priceListConfigDialog.open"', productSelectionStart)
  assert.ok(productSelectionStart > -1 && dialogStart > productSelectionStart, 'missing product selection block')

  const selectionSource = viewSource.slice(productSelectionStart, dialogStart)
  const categoryHeadStart = selectionSource.indexOf('class="product-picker-category-head"')
  const categoryHeadEnd = selectionSource.indexOf('v-for="row in category.items"', categoryHeadStart)
  const productRowStart = selectionSource.indexOf('v-for="row in category.items"')
  const productRowEnd = selectionSource.indexOf('<div v-if="pdfTheme.listType ===', productRowStart)
  assert.ok(categoryHeadStart > -1 && categoryHeadEnd > categoryHeadStart, 'missing category selection head block')
  assert.ok(productRowStart > -1 && productRowEnd > productRowStart, 'missing product selection row block')

  const categoryHeadSource = selectionSource.slice(categoryHeadStart, categoryHeadEnd)
  const productRowSource = selectionSource.slice(productRowStart, productRowEnd)

  for (const expected of [
    "return '继承分类'",
    "priceListTemplateSummary(priceListCategoryTemplateSelection(group), '')",
    "priceListTemplateSummary(priceListProductTemplateOverride(row), '继承分类')",
  ]) {
    assert.ok(viewSource.includes(expected), `missing category inheritance summary behavior: ${expected}`)
  }

  for (const forbidden of [
    '父类：',
    '子类：',
    '继承子类',
    '继承父类',
  ]) {
    assert.equal(categoryHeadSource.includes(forbidden), false, `category summary should hide parent/child wording: ${forbidden}`)
    assert.equal(productRowSource.includes(forbidden), false, `product summary should hide parent/child wording: ${forbidden}`)
  }

  for (const expected of [
    'category-pricing-summary',
    'product-compact-status',
    'priceListCategoryPricingSummary(category)',
    'priceListProductPricingSummary(priceListProductRowForItem(row))',
    'priceListProductDisplaySummary(itemProductID(row))',
  ]) {
    assert.ok(selectionSource.includes(expected), `missing compact summary behavior: ${expected}`)
  }
})

test('price list category pricing target helper separates parent, subgroup and product overrides', () => {
  for (const expected of [
    'function priceListCategoryTemplateTarget',
    'function priceListCategoryTemplateSelection',
    'function setPriceListCategoryTemplate',
    'function clearPriceListCategoryTemplate',
    'function priceListParentTemplateKey',
    'function priceListGroupTemplateKey',
    'priceListCategoryTemplateTarget(group).kind ===',
    'priceListParentTemplateSelections.value',
    'priceListGroupTemplateSelections.value',
    'priceListProductTemplateOverrides.value',
    'function setPriceListPricingPopoverMode',
    'function setPriceListPricingPopoverField',
    "openPriceListPricingPopover('category'",
    "openPriceListPricingPopover('product'",
  ]) {
    assert.ok(viewSource.includes(expected), `missing separated pricing target behavior: ${expected}`)
  }

  const categoryOverrideStart = viewSource.indexOf('function priceListCategoryPricingHasOverride')
  const categoryOverrideEnd = viewSource.indexOf('function priceListProductPricingHasOverride', categoryOverrideStart)
  assert.ok(categoryOverrideStart > -1 && categoryOverrideEnd > categoryOverrideStart, 'missing category override helper')
  const categoryOverrideSource = viewSource.slice(categoryOverrideStart, categoryOverrideEnd)
  assert.ok(categoryOverrideSource.includes('priceListCategoryTemplateSelection(group)'), 'category override should use current category target only')
  assert.equal(
    categoryOverrideSource.includes('priceListParentTemplateSelection(group)) || priceListTemplateHasOverride(priceListGroupTemplateSelection(group))'),
    false,
    'category summary must not merge parent and subgroup overrides into one button'
  )

  const assignmentsStart = viewSource.indexOf('function priceListTemplateAssignments()')
  const assignmentsEnd = viewSource.indexOf('function priceListProductOverridesForSnapshot()', assignmentsStart)
  assert.ok(assignmentsStart > -1 && assignmentsEnd > assignmentsStart, 'missing priceListTemplateAssignments')
  const assignmentsSource = viewSource.slice(assignmentsStart, assignmentsEnd)
  for (const expected of [
    'const parentKey = priceListParentTemplateKey(group)',
    'group_item_id: Number(group.group_item_id || 0)',
    'parent_group_item_id: 0',
    'const groupKey = priceListGroupTemplateKey(group)',
  ]) {
    assert.ok(assignmentsSource.includes(expected), `missing assignment mapping detail: ${expected}`)
  }
})

test('price list preview builds from current selected products instead of empty current publication content', () => {
  const groupsStart = viewSource.indexOf('const basePdfGroups = computed(() => {')
  const groupsEnd = viewSource.indexOf('const priceListGroupTemplateRows', groupsStart)
  assert.ok(groupsStart > -1 && groupsEnd > groupsStart, 'missing basePdfGroups computed block')
  const groupsSource = viewSource.slice(groupsStart, groupsEnd)

  assert.ok(groupsSource.includes('downloadSourcePublication.value?.content?.groups'), 'download action should still render stored publication content')
  assert.ok(groupsSource.includes('buildBeanListPdfGroupsFromCategoryRows(categoryProductGroups.value'), 'generate drawer should render from the same category rows as product picker')
  assert.equal(groupsSource.includes('currentPriceSourcePublication.value?.content?.groups'), false, 'current price source must not replace current selected products')
  assert.equal(viewSource.includes('const priceListFlatRows = computed(() => priceListFlatRowsFromGroups(basePdfGroups.value))'), true, 'flat rows should be generated from base preview groups')
  assert.equal(viewSource.includes('applyPriceListFlatRowsToBeanListPdfGroups(basePdfGroups.value, priceListFlatRows.value'), true, 'preview should render flat price rows back into PDF groups')
  assert.equal(viewSource.includes("apiSend('/api/costing/pricing-rule-trial'"), true, 'pricing-rule rows should load live trial prices')
  assert.equal(viewSource.includes('priceTablePricingRuleTrialPayload(row, { customerID: activeBeanListCustomerID.value })'), true, 'pricing-rule trial payload should be scoped to the current customer')
  assert.equal(viewSource.includes("cached?.status === 'error'"), true, 'failed pricing-rule trial requests should not spin in a retry loop')
  assert.equal(viewSource.includes('visibleCategoryCodes: pdfVisibleCategoryCodes.value'), true, 'preview should keep the product picker category codes')
  assert.equal(viewSource.includes('const pdfVisiblePreviewCategoryCodes = computed(() => pdfCategoryCodesForVisibleSelection'), false, 'preview should not translate picker category codes into legacy PDF category codes')
  assert.equal(viewSource.includes('function pdfCategoryCodesForVisibleSelection'), false, 'legacy preview category code mapper should be removed')
  assert.equal(viewSource.includes('<div v-if="priceListFlatRows.length" class="pdf-picker flat-price-row-editor">'), true, 'empty flat price rows should stay hidden')
  assert.equal(viewSource.includes('priceListFlatRows.value.length > 0 && priceListFlatRows.value.every'), true, 'empty flat price rows should not be publish-ready')
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
    '梯度按 KG，单价按元/KG',
    '生成并发布新版价格表后，录单才会使用新价格',
    'greenTierPriceRows(row)',
    'setGreenBeanTierPrice(itemProductID(row), tier, $event.target.value)',
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
