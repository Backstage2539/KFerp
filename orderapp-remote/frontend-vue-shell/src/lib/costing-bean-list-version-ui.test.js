import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const viewSource = readFileSync(resolve(here, '../views/CostingView.vue'), 'utf8')
const priceListWorkflowSource = readFileSync(resolve(here, './costing-price-list-workflow.js'), 'utf8')
const priceListSelectionSource = readFileSync(resolve(here, './product-price-list-selection.js'), 'utf8')
const priceListDraftSource = readFileSync(resolve(here, './product-price-list-draft.js'), 'utf8')
const priceListTypesSource = readFileSync(resolve(here, './product-price-list-types.js'), 'utf8')

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

test('product price list header and published versions use the compact pricing toolbar', () => {
  const pageHeaderStart = viewSource.indexOf('<section class="panel">')
  const versionPanelStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  assert.ok(pageHeaderStart > -1 && versionPanelStart > pageHeaderStart, 'missing price-list page header and version panel')

  const pageHeaderSource = viewSource.slice(pageHeaderStart, versionPanelStart)
  assert.doesNotMatch(pageHeaderSource, /<span>模型<\/span>/)
  assert.doesNotMatch(pageHeaderSource, /<strong>Price List \/ Item Price<\/strong>/)

  const versionPanelEnd = viewSource.indexOf('<section class="panel">', versionPanelStart + 1)
  const versionPanelSource = viewSource.slice(versionPanelStart, versionPanelEnd)
  assert.doesNotMatch(versionPanelSource, /查看当前范围下的已发布价格表、生成新版、撤回和归档。/)
  assert.doesNotMatch(versionPanelSource, /刷新版本/)
  assert.doesNotMatch(versionPanelSource, /refreshBeanListVersionList/)
  assert.match(versionPanelSource, /:aria-label="publicationListCollapsed \? '展开已发布价格表' : '收起已发布价格表'"/)
  assert.match(versionPanelSource, /'⇊'\s*:\s*'⇈'/)

  const collapseButtonIndex = versionPanelSource.indexOf('class="publication-list-collapse-toggle')
  const versionTitleIndex = versionPanelSource.indexOf('<div class="section-title">已发布价格表</div>')
  assert.ok(collapseButtonIndex > -1 && collapseButtonIndex < versionTitleIndex, 'collapse toggle must sit to the left of the title')

  const productTypeIndex = versionPanelSource.indexOf('<span>商品类型</span>')
  const searchIndex = versionPanelSource.indexOf('<span>搜索</span>')
  assert.ok(productTypeIndex > -1 && productTypeIndex < searchIndex, 'product type filter must sit to the left of search')
  assert.match(viewSource, /\.version-controls input,\s*\.version-controls select\s*\{[^}]*min-height:\s*38px/s)

  assert.match(viewSource, /<strong>计价规则<\/strong>/)
  assert.doesNotMatch(viewSource, /<strong>Price List \/ Item Price 生成规则<\/strong>/)
})

test('product price list keeps product count scope and tier template action on one compact row', () => {
  const pageHeaderStart = viewSource.indexOf('<section class="panel">')
  const versionPanelStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const pageHeaderSource = viewSource.slice(pageHeaderStart, versionPanelStart)

  assert.match(pageHeaderSource, /class="price-list-top-toolbar"/)
  const productCountIndex = pageHeaderSource.indexOf('<span>商品数</span>')
  const scopeIndex = pageHeaderSource.indexOf('aria-label="价格表归属"')
  const tierTemplateIndex = pageHeaderSource.indexOf('>管理阶梯模板</button>')
  assert.ok(productCountIndex > -1 && scopeIndex > productCountIndex && tierTemplateIndex > scopeIndex)
  assert.equal((viewSource.match(/>管理阶梯模板<\/button>/g) || []).length, 1)
  assert.match(viewSource, /\.price-list-top-toolbar\s*\{[^}]*display:\s*grid[^}]*grid-template-columns:/s)
})

test('product price list groups all three management actions in the top toolbar with equal summary heights', () => {
  const pageHeaderStart = viewSource.indexOf('<section class="panel">')
  const versionPanelStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const pageHeaderSource = viewSource.slice(pageHeaderStart, versionPanelStart)
  const generatePanelStart = viewSource.indexOf('<div class="bean-list-generate-bar">')
  const generatePanelEnd = viewSource.indexOf('</section>', generatePanelStart)
  const generatePanelSource = viewSource.slice(generatePanelStart, generatePanelEnd)

  assert.doesNotMatch(pageHeaderSource, />刷新<\/button>/)
  assert.match(pageHeaderSource, /class="price-list-toolbar-actions"/)
  const tierTemplateIndex = pageHeaderSource.indexOf('>管理阶梯模板</button>')
  const pricingRulesIndex = pageHeaderSource.indexOf('>计价模式规则</button>')
  const priceListConfigIndex = pageHeaderSource.indexOf('>价格表配置</button>')
  assert.ok(
    tierTemplateIndex > -1 && pricingRulesIndex > tierTemplateIndex && priceListConfigIndex > pricingRulesIndex,
    'top toolbar must contain tier templates, pricing rules and price-list config in order',
  )
  assert.doesNotMatch(generatePanelSource, />计价模式规则<\/button>/)
  assert.doesNotMatch(generatePanelSource, />价格表配置<\/button>/)
  assert.equal((viewSource.match(/>管理阶梯模板<\/button>/g) || []).length, 1)
  assert.equal((viewSource.match(/>计价模式规则<\/button>/g) || []).length, 1)
  assert.equal((viewSource.match(/>价格表配置<\/button>/g) || []).length, 1)
  assert.match(viewSource, /\.price-list-top-toolbar\s*\{[^}]*align-items:\s*stretch/s)
  assert.match(
    viewSource,
    /\.price-list-toolbar-stat,\s*\.price-list-toolbar-scope\s*\{[^}]*min-height:\s*76px/s,
  )
  assert.match(viewSource, /\.price-list-toolbar-actions\s*\{[^}]*display:\s*flex[^}]*flex-wrap:\s*wrap/s)
  assert.match(
    viewSource,
    /@media \(max-width:\s*1200px\)[\s\S]*\.price-list-top-toolbar\s*\{[^}]*grid-template-columns:\s*minmax\(120px,\s*\.35fr\)\s*minmax\(260px,\s*1fr\)[^}]*\}[\s\S]*\.price-list-toolbar-actions\s*\{[^}]*grid-column:\s*1\s*\/\s*-1/s,
  )
  assert.match(viewSource, /@media \(max-width:\s*900px\)[\s\S]*\.price-list-toolbar-actions button\s*\{[^}]*flex:\s*1 1 180px/s)
  assert.match(viewSource, /onMounted\(\(\) => \{[\s\S]*loadBeanList\(\)/s)
})

test('product price list restores and persists browser scope and product type preferences', () => {
  for (const expected of [
    'readPriceListPagePreferences()',
    'writePriceListPagePreferences(',
    'resolvePriceListScopePreference(',
    'resolveProductTypePreference(',
    'PRICE_LIST_PAGE_PREFERENCES_KEY',
  ]) {
    assert.ok(viewSource.includes(expected), `missing price-list browser preference behavior: ${expected}`)
  }
})

test('product price list waits for product-catalog feature selection before resolving the remembered template', () => {
  assert.match(viewSource, /const priceListProductCatalogFeatureSelectionLoaded = ref\(false\)/)
  assert.match(
    viewSource,
    /const productPriceListTypeOptions = computed\(\(\) => \{\s*if \(!priceListProductCatalogFeatureSelectionLoaded\.value\) return \[\]/,
  )
  assert.match(
    viewSource,
    /watch\(productPriceListTypeOptions, \(options\) => \{\s*if \(!priceListProductCatalogFeatureSelectionLoaded\.value\) \{[\s\S]*?return\s*\}/,
  )
  assert.match(
    viewSource,
    /async function loadPriceListProductBusinessGroups\(\) \{[\s\S]*priceListProductCatalogFeatureSelectionLoaded\.value = false[\s\S]*finally \{\s*priceListProductCatalogFeatureSelectionLoaded\.value = true\s*\}/,
  )
})

test('product price list loads synthetic publication views when remembered template options first become ready', () => {
  assert.match(viewSource, /const priceListPublicationTypeOptionsReady = ref\(false\)/)

  const watcherStart = viewSource.indexOf('watch(productPriceListTypeOptions, (options) => {')
  const watcherEnd = viewSource.indexOf('watch(selectedProductTypeCategoryID', watcherStart)
  assert.ok(watcherStart > -1 && watcherEnd > watcherStart, 'missing product type readiness watcher')
  const watcherSource = viewSource.slice(watcherStart, watcherEnd)
  assert.match(watcherSource, /const optionsJustBecameReady = !priceListPublicationTypeOptionsReady\.value/)
  assert.match(watcherSource, /loadActiveProductTypePublicationViews\(\)/)

  const loaderStart = viewSource.indexOf('function loadActiveProductTypePublicationViews()')
  const loaderEnd = viewSource.indexOf('watch(selectedProductTypeCategoryID', loaderStart)
  assert.ok(loaderStart > -1 && loaderEnd > loaderStart, 'missing active synthetic publication loader')
  const loaderSource = viewSource.slice(loaderStart, loaderEnd)
  assert.match(loaderSource, /const productTypeCategoryID = activeProductTypeCategoryID\.value/)
  assert.match(loaderSource, /selectedProductPriceListType\.value\?\.listType/)
  assert.match(loaderSource, /loadBeanListPublications\(listType, versionListScope\.value, productTypeCategoryID\)/)
  assert.match(loaderSource, /loadBeanListPublications\(listType, 'official', productTypeCategoryID, 'factory_supply'\)/)
  assert.match(loaderSource, /loadBeanListPublications\(listType, 'mine', productTypeCategoryID, 'factory_supply'\)/)
  assert.match(loaderSource, /loadBeanListPublications\(listType, 'customer', productTypeCategoryID, 'factory_supply'\)/)
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

test('product price-list card preview omits legacy bean-list marketing fields and wraps long names', () => {
  const previewCardStart = viewSource.indexOf('<div v-else class="pdf-card-grid">')
  const previewCardEnd = viewSource.indexOf('</section>', previewCardStart)
  assert.ok(previewCardStart > -1 && previewCardEnd > previewCardStart, 'preview card block not found')
  const previewCardSource = viewSource.slice(previewCardStart, previewCardEnd)

  const printCardStart = viewSource.indexOf('<div v-else class="pdf-card-grid">', previewCardEnd)
  const printCardEnd = viewSource.indexOf('</section>', printCardStart)
  assert.ok(printCardStart > -1 && printCardEnd > printCardStart, 'print card block not found')
  const printCardSource = viewSource.slice(printCardStart, printCardEnd)

  for (const source of [previewCardSource, printCardSource]) {
    assert.doesNotMatch(source, /item\.recommendedUse/)
    assert.doesNotMatch(source, /item\.flavor/)
    assert.doesNotMatch(source, /item\.description/)
    assert.doesNotMatch(source, /出品建议/)
    assert.doesNotMatch(source, /风味/)
    assert.doesNotMatch(source, /特点/)
    assert.doesNotMatch(source, /批发价/)
    assert.match(source, /item\.attributeLines/)
    assert.match(source, /pdf-price-block/)
  }

  assert.match(viewSource, /\.pdf-item-head \{[^}]*grid-template-columns: auto minmax\(0, 1fr\)/)
  assert.match(viewSource, /\.pdf-item-head > div \{[^}]*min-width: 0/)
  assert.match(viewSource, /\.pdf-item h3 \{[^}]*overflow-wrap: anywhere/)
})

test('product price-list card preview aligns each row to its tallest card without fixed card height', () => {
  assert.match(viewSource, /\.pdf-card-row \{[^}]*align-items: stretch/)
  assert.doesNotMatch(viewSource, /\.pdf-card-row > \.pdf-item \{[^}]*height: 100%/)
  assert.doesNotMatch(viewSource, /\.pdf-card-row > \.pdf-item \{[^}]*align-self: start/)
  assert.doesNotMatch(viewSource, /\.pdf-price-block \{[^}]*margin-top: auto/)
  assert.doesNotMatch(viewSource, /\.pdf-card-row\.cards-2 \.pdf-item-head,\s*\.pdf-card-row\.cards-3 \.pdf-item-head \{[^}]*min-height/)
  assert.match(viewSource, /\.pdf-price \{[^}]*min-height: 34px/)
})

test('product price-list card preview aligns title attributes and quote slots within each row', () => {
  assert.match(viewSource, /class="pdf-meta-block"/)
  assert.match(viewSource, /\.pdf-card-row \{[^}]*grid-template-rows: auto auto auto/)
  assert.match(viewSource, /\.pdf-card-row \{[^}]*column-gap: 9px/)
  assert.match(viewSource, /\.pdf-card-row \{[^}]*row-gap: 0/)
  assert.match(viewSource, /\.pdf-card-row > \.pdf-item \{[^}]*display: grid/)
  assert.match(viewSource, /\.pdf-card-row > \.pdf-item \{[^}]*grid-template-rows: subgrid/)
  assert.match(viewSource, /\.pdf-card-row > \.pdf-item \{[^}]*grid-row: span 3/)
  assert.match(viewSource, /\.pdf-price-block \{[^}]*align-self: start/)
})

test('product price-list publish action reports blocked reasons instead of doing nothing', () => {
  const publishButtonStart = viewSource.indexOf('@click="publishBeanList"')
  assert.ok(publishButtonStart > -1, 'publish button not found')
  const publishButtonSource = viewSource.slice(viewSource.lastIndexOf('<button', publishButtonStart), viewSource.indexOf('</button>', publishButtonStart))
  const publishTitleStart = viewSource.lastIndexOf('<div class="pdf-preview-title">', publishButtonStart)
  const publishTitleEnd = viewSource.indexOf('<div class="pdf-preview-phone', publishTitleStart)
  assert.ok(publishTitleStart > -1 && publishTitleEnd > publishTitleStart, 'publish action title block not found')
  const publishTitleSource = viewSource.slice(publishTitleStart, publishTitleEnd)

  assert.match(viewSource, /const priceListPublishBlockedReason = computed\(\(\) => \{/)
  assert.match(publishTitleSource, /v-if="error" class="error price-list-publish-feedback"/)
  assert.match(publishTitleSource, /v-if="message" class="ok price-list-publish-feedback"/)
  assert.match(viewSource, /flat-price-row-error-list/)
  assert.doesNotMatch(publishTitleSource, /price-list-publish-guard/)
  assert.match(publishButtonSource, /:disabled="beanListPublishing"/)
  assert.doesNotMatch(publishButtonSource, /!pdfGroups\.length/)
  assert.doesNotMatch(publishButtonSource, /!pdfTheme\.version/)
  assert.doesNotMatch(publishButtonSource, /!customerScopeReady/)
  assert.doesNotMatch(publishButtonSource, /!priceListFlatRowsReady/)

  const publishStart = viewSource.indexOf('async function publishBeanList()')
  const publishEnd = viewSource.indexOf('async function saveBeanListDraft()', publishStart)
  assert.ok(publishStart > -1 && publishEnd > publishStart, 'publishBeanList function not found')
  const publishSource = viewSource.slice(publishStart, publishEnd)
  const blockedReasonStart = viewSource.indexOf('const priceListPublishBlockedReason = computed(() => {')
  const blockedReasonEnd = viewSource.indexOf('const pdfPageStyle', blockedReasonStart)
  assert.ok(blockedReasonStart > -1 && blockedReasonEnd > blockedReasonStart, 'priceListPublishBlockedReason block not found')
  const blockedReasonSource = viewSource.slice(blockedReasonStart, blockedReasonEnd)

  for (const expected of [
    'const blockedReason = priceListPublishBlockedReason.value',
    'error.value = blockedReason',
    '暂无可发布的价格表预览',
    '请填写价格表版本号',
    '请选择客户',
    '平铺价格行存在未完成项目，请按红色行提示补齐。',
  ]) {
    assert.ok(viewSource.includes(expected), `missing publish blocked reason behavior: ${expected}`)
  }
  assert.doesNotMatch(blockedReasonSource, /hasInactiveBomWarning/)
  assert.doesNotMatch(blockedReasonSource, /BOM 提示后再发布价格表/)
  assert.doesNotMatch(publishSource, /hasInactiveBomWarning/)
  assert.doesNotMatch(publishSource, /if \(hasInactiveBomWarning\.value\) \{\s*error\.value = ''\s*await scrollFirstInactiveBomWarningIntoView\(\)\s*return\s*\}/)
  assert.doesNotMatch(publishSource, /if \(!pdfGroups\.value\.length\) return/)
})

test('product price-list inactive BOM warnings stay on product rows with product archive navigation', () => {
  assert.doesNotMatch(viewSource, /inactiveBomWarningCount/)
  assert.doesNotMatch(viewSource, /重新启用 BOM/)
  assert.doesNotMatch(viewSource, /<div v-if="[^"]*inactiveBom[^"]*" class="warning-banner"/)

  for (const expected of [
    'itemBomWarning(row)',
    'class="product-picker-bom-warning"',
    'itemBomProblemLabel(row)',
    '去商品档案重新选择 BOM',
    '@click.stop="openProductArchiveForBom(row)"',
    'function itemHasInactiveBomWarning',
    'function itemBomProblemLabel',
    'function openProductArchiveForBom',
    'production_bom_name',
    'production_bom_version_no',
    'source_bom_version_no',
    "key: 'productMaster'",
    'open_product_config_id',
    'returnNavigation',
    '失效 BOM 不能重新启用；如需沿用旧结构，请先在生产 BOM 复制成新 BOM 后再选择。',
    'scrollFirstInactiveBomWarningIntoView',
  ]) {
    assert.ok(viewSource.includes(expected), `missing inactive BOM product-row behavior: ${expected}`)
  }
})

test('product price-list replaces backend pricing warnings with precise current-draft warnings', () => {
  const selectionStart = viewSource.indexOf('<div class="pdf-picker productSelection">')
  const dialogStart = viewSource.indexOf('<div v-if="priceListConfigDialog.open"', selectionStart)
  assert.ok(selectionStart > -1 && dialogStart > selectionStart, 'missing product selection block')
  const selectionSource = viewSource.slice(selectionStart, dialogStart)

  assert.ok(selectionSource.includes('visibleItemWarnings(row)'), 'product picker warnings should be filtered by effective price-list pricing')
  assert.equal(selectionSource.includes('itemWarnings(row).length'), false, 'raw backend warning count should not drive product picker display')

  for (const expected of [
    'function visibleItemWarnings',
    'function priceListResolvedTemplateForItem',
    'function priceListProductSpecPricingWarning',
    'resolvePriceTableTemplateInheritance({',
    'priceListTemplateAssignments()',
    'priceListProductOverridesForSnapshot()',
    "text !== '未设置计价方式'",
    'priceTablePricingResolutionWarning(priceListProductSpecPricingResolution(family, spec))',
  ]) {
    assert.ok(viewSource.includes(expected), `missing resolved price-list warning fallback behavior: ${expected}`)
  }
})

test('product price-list published versions can be archived and restored from archive list', () => {
  const versionListStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const versionListEnd = viewSource.indexOf('<section class="panel">', versionListStart)
  assert.ok(versionListStart > -1 && versionListEnd > versionListStart, 'missing bean-list version panel')
  const versionListSource = viewSource.slice(versionListStart, versionListEnd)

  for (const expected of [
    '归档选中',
    '归档列表',
    '移出归档',
    'selectedPublicationArchiveIDs',
    'toggleCurrentPagePublicationArchiveSelection',
    'togglePublicationArchiveSelection(row)',
    'archiveSelectedBeanListPublications',
    'restoreArchivedBeanListPublication(row)',
    'publicationArchiveRefreshProductTypeIDs',
    'setBeanListPublicationStatusInCache',
    'beanListPublicationArchivedFromStatus',
    'currentScopeActivePublicationRows',
    'currentScopeArchivedPublicationRows',
    "row.status !== 'archived'",
    "row.status === 'archived'",
    'archivedPublicationListState',
    'paginatedArchivedPublicationRows',
    ':disabled="!selectedPublicationArchiveIDs.length || beanListArchiving"',
    "apiSend('/api/costing/bean-list/publications/archive'",
    "apiSend('/api/costing/bean-list/publications/unarchive'",
  ]) {
    assert.ok(versionListSource.includes(expected) || viewSource.includes(expected), `missing publication archive behavior: ${expected}`)
  }

  assert.ok(viewSource.includes("case 'archived':"), 'archived status should have a visible label')
  assert.ok(viewSource.includes("return '已归档'"), 'archived status should render 已归档')
  assert.ok(viewSource.includes("if (status === 'archived') return 'status-archived'"), 'archived status should have its own pill class')
  assert.ok(viewSource.includes('beanListPublicationIsCurrent(row)'), 'current publication should be protected from accidental archive')
  assert.ok(versionListSource.includes('<th class="select-col">'), 'version list should expose a multi-select column')
  assert.ok(viewSource.includes('activeProductTypeCategoryID.value'), 'archive refresh should update the current visible product type cache')
  assert.ok(viewSource.includes('for (const refreshProductTypeID of publicationArchiveRefreshProductTypeIDs'), 'archive refresh should cover all affected cache keys')
  assert.ok(viewSource.includes("setBeanListPublicationStatusInCache(rows.map((row) => Number(row.id || 0)), 'archived')"), 'archive should update cached rows immediately')
  assert.ok(viewSource.includes('setBeanListPublicationStatusInCache([Number(row.id || 0)], beanListPublicationArchivedFromStatus(row))'), 'unarchive should restore cached rows immediately')
})

test('product price-list version scope selector lists public and each fulfillment customer', () => {
  const versionListStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const versionListEnd = viewSource.indexOf('<section class="panel">', versionListStart)
  assert.ok(versionListStart > -1 && versionListEnd > versionListStart, 'missing bean-list version panel')
  const versionListSource = viewSource.slice(versionListStart, versionListEnd)

  const pageScopeStart = viewSource.indexOf('<div class="price-list-top-toolbar">')
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
    'buildProductCatalogTemplatePriceListTypeOptions',
    'priceListRenderTypeForItem',
    'productPriceListTypeKey',
    'price-list-page-config',
    '<strong>计价规则</strong>',
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

test('product price list uses product archive catalog templates and categories instead of legacy product types', () => {
  for (const expected of [
    '<h2>商品价格表</h2>',
    'Price List / Item Price',
    'data-pr440-price-list-model',
    '商品 &gt; 所在分类 &gt; 上级分类逐级向上 &gt; 价格表',
    '平铺价格行',
    '分组项选品',
    'buildProductCatalogTemplatePriceListTypeOptions',
    'businessGroupRowsForFeatureSelection',
    'groupRowsByBusinessGroupTemplate',
    "usageKey: 'product_catalog'",
  ]) {
    assert.ok(viewSource.includes(expected), `missing classification price-list behavior: ${expected}`)
  }
  assert.doesNotMatch(viewSource, /<h2>产品价格表<\/h2>/)
  assert.doesNotMatch(viewSource, /<span>产品类型<\/span>/)
  assert.doesNotMatch(viewSource, /buildClassificationPriceListTypeOptions/)
  assert.doesNotMatch(viewSource, /按当前价格表归属和商品管理里的产品类型生成/)
  assert.doesNotMatch(viewSource, /按当前价格表归属、商品当前归类和客户商品生成商品价格表/)
})

test('product price list inherits product catalog feature selections instead of owning a price-list group usage', () => {
  assert.match(viewSource, /apiGet\('\/api\/business-group-feature-selections\/product_catalog'\)/)
  assert.match(viewSource, /businessGroupFeatureSelectionIDs/)
  assert.match(viewSource, /businessGroupRowsForFeatureSelection/)
  assert.match(viewSource, /buildProductCatalogTemplatePriceListTypeOptions/)
  assert.doesNotMatch(viewSource, /business-group-feature-selections\/price_list/)
  assert.doesNotMatch(viewSource, /usage_key=price_list/)
  assert.doesNotMatch(viewSource, /selectedProductCatalogGroupTemplateID/)
  assert.doesNotMatch(viewSource, /productSettingsSelectedProductGroupTemplateID/)
})

test('same-list product catalog templates keep publication URLs payloads caches and row selection isolated', () => {
  for (const expected of [
    'publicationTypeIdentityForPriceListType',
    'priceListTypeOptionForPublication',
    'preferredPublicationForPriceListType',
    'activePublicationClassificationTemplateID',
    "params.set('classification_template_id'",
  ]) {
    assert.ok(viewSource.includes(expected), `missing isolated publication identity behavior: ${expected}`)
  }
  for (const expected of [
    'PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE',
    'publicationProductTypeCategoryID',
    'publicationClassificationTemplateID',
  ]) {
    assert.ok(priceListTypesSource.includes(expected), `missing stable publication type field: ${expected}`)
  }
  assert.match(viewSource, /classification_template_id:\s*publicationIdentity\.classificationTemplateID/)
  assert.match(viewSource, /product_type_category_id:\s*publicationIdentity\.productTypeCategoryID/)
  assert.match(viewSource, /const selectedType = priceListTypeOptionForPublication\(productPriceListTypeOptions\.value, row\)/)
  assert.doesNotMatch(viewSource, /const productTypeCategoryID = currentClassificationTemplateIDOfPublication\(currentBeanListPublication\.value\)/)
  assert.doesNotMatch(viewSource, /const productTypeCategoryID = currentClassificationTemplateIDOfPublication\(row\)/)
})

test('price list mode rules are opened from a button and not shown as a persistent panel', () => {
  const generatePanelStart = viewSource.indexOf('<div class="bean-list-generate-bar">')
  const pageDrawerStart = viewSource.indexOf('<div v-if="tierTemplateDrawerOpen"', generatePanelStart)
  assert.ok(generatePanelStart > -1 && pageDrawerStart > generatePanelStart, 'missing generate page block')
  const pageSource = viewSource.slice(generatePanelStart, pageDrawerStart)

  for (const expected of [
    '计价模式规则',
    '@click="priceListRulesDialogOpen = true"',
    'v-if="priceListRulesDialogOpen"',
    'aria-label="计价模式规则"',
    'price-list-rules-dialog',
    '商品 &gt; 所在分类 &gt; 上级分类逐级向上 &gt; 价格表',
    'group_source=price_list',
    '父商品只选择一次计价模式',
    '固定价金额按规格分别录入',
  ]) {
    assert.ok(viewSource.includes(expected), `missing modal price-list mode rule behavior: ${expected}`)
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

test('price list product picker renders one parent row with independently selectable specs and shared parent pricing', () => {
  const productSelectionStart = viewSource.indexOf('<div class="pdf-picker productSelection">')
  const dialogStart = viewSource.indexOf('<div v-if="priceListConfigDialog.open"', productSelectionStart)
  assert.ok(productSelectionStart > -1 && dialogStart > productSelectionStart, 'missing product selection block')
  const selectionSource = viewSource.slice(productSelectionStart, dialogStart)

  for (const expected of [
    'X款/Y规格',
    'row.sku_options',
    'togglePdfProductSpec',
    'isPdfProductSpecSelected',
    'priceListProductSpecLabel(spec)',
    "openPriceListPricingPopover('product', priceListParentProductPricingRow(row))",
    ':indeterminate.prop="isPdfCategoryPartiallySelected(category.code)"',
  ]) {
    assert.ok(selectionSource.includes(expected), `missing parent/spec picker behavior: ${expected}`)
  }

  assert.equal(selectionSource.includes('v-for="row in category.items"'), true)
  assert.equal(selectionSource.includes('priceListProductRowForItem(row)'), false, 'parent row must not be priced as if it were one SKU')
  assert.equal(
    selectionSource.includes('{{ priceListPricingPopover.productRow?.parent_product_name }} · {{ priceListPricingPopover.productRow?.sku_name }}'),
    false,
    'pricing panel must not concatenate the SKU into the product name',
  )
})

test('price list product specs stay compact and expose only the inherited fixed-price amount inline', () => {
  assert.match(viewSource, /\.product-spec-options\s*\{[^}]*display:\s*flex;[^}]*flex-wrap:\s*wrap;/s)
  assert.match(viewSource, /\.product-spec-option\s*\{[^}]*display:\s*flex;/s)
  assert.match(viewSource, /class="product-spec-fixed-price"/)
  assert.match(viewSource, /priceListProductSpecPricingWarning\(row, spec\)/)
  assert.doesNotMatch(viewSource, /class="product-spec-pricing-panel"/)
  assert.doesNotMatch(viewSource, /设置当前规格计价/)
})

test('flat price rows render the unchanged product name with a separate spec description', () => {
  const flatRowStart = viewSource.indexOf('<div v-if="priceListFlatRows.length" class="pdf-picker flat-price-row-editor">')
  const previewStart = viewSource.indexOf('<div class="price-list-preview"', flatRowStart)
  assert.ok(flatRowStart > -1, 'missing flat price row editor')
  const flatRowSource = viewSource.slice(flatRowStart, previewStart > flatRowStart ? previewStart : undefined)

  assert.match(flatRowSource, /<strong>\{\{ priceListFlatRowDisplayTitle\(row\) \}\}<\/strong>/)
  assert.match(flatRowSource, /\{\{ priceListFlatRowSpecDescription\(row\) \}\}/)
  assert.match(viewSource, /product_name:\s*item\.product_name_snapshot \|\| item\.product_name \|\| item\.__price_list_product_name/)
  assert.match(viewSource, /priceListSalesSpecCountTierLabel\(templateTier\)/)
  assert.doesNotMatch(viewSource, /\$\{formatQty\(minQty\)\}-\$\{formatQty\(maxQty\)\}个\$\{spec\}/)
})

test('price list exposes one shared parent-product pricing choice above category inheritance', () => {
  assert.match(viewSource, /商品 &gt; 所在分类 &gt; 上级分类逐级向上 &gt; 价格表/)
  assert.match(viewSource, /商品计价/)
  assert.match(viewSource, /priceListParentProductPricingRow\(row\)/)
  assert.match(viewSource, /scope: 'parent_product'/)
  assert.match(viewSource, /priceListProductTemplateOverrideKey/)
  assert.doesNotMatch(viewSource, /规格 &gt; 商品 &gt;/)
})

test('price list inherits fixed-price mode but requires each selected spec to enter its own amount', () => {
  assert.match(viewSource, /固定价金额按具体规格分别录入/)
  assert.doesNotMatch(viewSource, /:value="priceListTemplateDefaults\.fixed_unit_price"/)
  assert.match(viewSource, /v-for="spec in row\.sku_options"/)
  assert.match(viewSource, /priceListProductSpecPricingResolution\(row, spec\)\.pricing_mode === 'fixed_price'/)
  assert.match(viewSource, /固定价（元\/\{\{ priceListProductSpecLabel\(spec\) \}\}）/)
  assert.match(viewSource, /setPriceListProductFixedPrice\(row, spec/)
  assert.match(viewSource, /function priceListEffectiveProductPricingSelection/)
  assert.match(viewSource, /selectedSpecsForProduct\(row\)\[0\]/)
  const activeSelectionStart = viewSource.indexOf('function priceListActivePricingSelection()')
  const activeSelectionEnd = viewSource.indexOf('function setPriceListPricingPopoverMode', activeSelectionStart)
  assert.ok(activeSelectionStart > -1 && activeSelectionEnd > activeSelectionStart, 'missing active pricing selection helper')
  assert.match(
    viewSource.slice(activeSelectionStart, activeSelectionEnd),
    /priceListProductTemplateOverride\(priceListPricingPopover\.value\.productRow \|\| \{\}\)/,
    'product popover must keep local inherit selected instead of marking inherited fixed price as local',
  )
  assert.ok(viewSource.includes('当前生效：{{ priceListPricingPopoverEffectiveSummary() }}'))
  assert.match(viewSource, /const representative = selectedSpecsForProduct\(item\)\[0\]/)
})

test('flat publication rows resolve pricing ancestry from the concrete selected SKU', () => {
  const flatRowsSource = viewSource.match(/function priceListFlatRowsFromGroups[\s\S]*?function priceListFlatRowFromSource/)?.[0] || ''

  assert.match(flatRowsSource, /const groupRow = priceListGroupForItem\(item\)/)
  assert.doesNotMatch(flatRowsSource, /const groupRow = priceListGroupConfigRow\(group\)/)
})

test('price list generation persists concrete product spec selections and materializes only selected SKUs', () => {
  for (const expected of [
    'productSpecSelectionsByType',
    'product_spec_selections: pdfProductSpecSelections.value',
    'priceListSelectedSkuCategoryRows(categoryProductGroups.value, pdfProductSpecSelections.value)',
    'normalizePriceListProductSpecSelections',
    'defaultPriceListProductSpecSelections',
  ]) {
    assert.ok(viewSource.includes(expected), `missing product spec selection data flow: ${expected}`)
  }
  assert.ok(priceListSelectionSource.includes('selection_source'), 'selection helper must freeze the selection source')
  assert.ok(priceListSelectionSource.includes('default_sku_id_at_selection'), 'selection helper must freeze the default SKU at selection time')
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
    'priceListProductPricingSummary(priceListParentProductPricingRow(row))',
    'priceListProductDisplaySummary(priceListParentProductID(row))',
    "openPriceListPricingPopover('product', priceListParentProductPricingRow(row))",
    "isPriceListPricingPopoverOpen('product', priceListParentProductPricingRow(row))",
    'openPriceListProductDisplayDialog(priceListParentProductID(row))',
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
    'setPriceListProductFixedPrice(',
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
    'priceListProductCatalogFeatureSelection',
    'priceListPricingRuleTrialRequestsForRows',
    'watch(priceListPricingRuleTrialRequests',
    "apiSend('/api/costing/pricing-rule-trials'",
    'mergePriceListPricingRuleTrialCache',
    'executePriceListPricingRuleTrialBatches',
  ]) {
    assert.ok(viewSource.includes(expected), `missing price-list draft/group persistence behavior: ${expected}`)
  }
  assert.match(priceListWorkflowSource, /start \+= chunkSize/, 'batch executor should advance by its tested chunk size')
  assert.equal(viewSource.includes('watch(priceListFlatRows'), false, 'flat price rows must not trigger a duplicate deep trial watcher')

  const flatRowStart = viewSource.indexOf('function priceListFlatRowFromSource')
  const flatRowEnd = viewSource.indexOf('function priceListPricingRuleTrialResultForRow', flatRowStart)
  assert.ok(flatRowStart > -1 && flatRowEnd > flatRowStart, 'priceListFlatRowFromSource block not found')
  const flatRowSource = viewSource.slice(flatRowStart, flatRowEnd)
  assert.match(flatRowSource, /mode === 'pricing_rule' \|\| mode === 'tier_template'/)
})

test('price list restored product template overrides schedule pricing-rule trial refresh', () => {
  assert.ok(
    viewSource.includes('function schedulePriceListPricingRuleTrialRefresh'),
    'restored pricing-rule product overrides need an explicit post-render trial refresh scheduler',
  )

  const restoreStart = viewSource.indexOf('function restorePriceListGenerationDraftForActiveType()')
  const restoreEnd = viewSource.indexOf('function beanListPublicationTypeKey', restoreStart)
  assert.ok(restoreStart > -1 && restoreEnd > restoreStart, 'restorePriceListGenerationDraftForActiveType block not found')
  assert.ok(
    viewSource.slice(restoreStart, restoreEnd).includes('schedulePriceListPricingRuleTrialRefresh()'),
    'restoring saved product template overrides should refresh pricing-rule trial requests',
  )

  const setProductStart = viewSource.indexOf('function setPriceListProductTemplate(row = {}, field, value)')
  const setProductEnd = viewSource.indexOf('function clearPriceListProductTemplate', setProductStart)
  assert.ok(setProductStart > -1 && setProductEnd > setProductStart, 'setPriceListProductTemplate block not found')
  assert.ok(
    viewSource.slice(setProductStart, setProductEnd).includes('schedulePriceListPricingRuleTrialRefresh()'),
    'changing product pricing template should refresh pricing-rule trial requests',
  )

  const flatRowsStart = viewSource.indexOf('function priceListFlatRowsFromGroups(groups = [])')
  const flatRowsEnd = viewSource.indexOf('function priceListFlatRowFromSource', flatRowsStart)
  assert.ok(flatRowsStart > -1 && flatRowsEnd > flatRowsStart, 'priceListFlatRowsFromGroups block not found')
  assert.ok(
    viewSource.slice(flatRowsStart, flatRowsEnd).includes('item?.id'),
    'flat price rows must use item.id as a product id fallback so restored product overrides match preview items',
  )
  assert.ok(
    viewSource.slice(flatRowsStart, flatRowsEnd).includes('itemProductID(item)'),
    'flat price rows must use numeric product keys as a final product id fallback for restored preview snapshots',
  )
})

test('price list pricing-rule preview retries current rows after user pricing changes', () => {
  assert.ok(
    viewSource.includes('function clearPriceListPricingRuleTrialErrorCache'),
    'pricing-rule preview should be able to clear error cache entries for current preview rows',
  )

  const scheduleStart = viewSource.indexOf('function schedulePriceListPricingRuleTrialRefresh')
  const scheduleEnd = viewSource.indexOf('const pdfTotalItems', scheduleStart)
  assert.ok(scheduleStart > -1 && scheduleEnd > scheduleStart, 'schedulePriceListPricingRuleTrialRefresh block not found')
  const scheduleSource = viewSource.slice(scheduleStart, scheduleEnd)
  assert.ok(
    scheduleSource.includes('clearPriceListPricingRuleTrialErrorCache(priceListFlatRows.value)'),
    'scheduled pricing changes should clear current-row errors before collecting retry requests',
  )

  const requestStart = priceListWorkflowSource.indexOf('function priceListPricingRuleTrialRequestsForRows')
  const requestEnd = priceListWorkflowSource.indexOf('export function dedupePriceListFlatRows', requestStart)
  assert.ok(requestStart > -1 && requestEnd > requestStart, 'priceListPricingRuleTrialRequestsForRows block not found')
  const requestSource = priceListWorkflowSource.slice(requestStart, requestEnd)
  assert.ok(
    requestSource.includes("cached?.status === 'error'"),
    'passive watchers should still avoid immediate retry loops after a failed trial',
  )
})

test('price list flat rows collapse only identical tier-template rows before preview and publish', () => {
  assert.ok(
    viewSource.includes('dedupePriceListFlatRows'),
    'price-list flat rows should import the duplicate tier-template row guard',
  )
  assert.ok(
    viewSource.includes('dedupePriceListFlatRows(normalizePriceListPublicationRows('),
    'price-list flat rows should normalize stale parent identities before collapsing identical generated template-tier rows',
  )
})

test('price list pricing-rule preview uses product quote unit before package fallback', () => {
  const unitStart = viewSource.indexOf('function flatRowPriceUnit')
  const unitEnd = viewSource.indexOf('function flatRowInventoryConversion', unitStart)
  assert.ok(unitStart > -1 && unitEnd > unitStart, 'flatRowPriceUnit block not found')
  const unitSource = viewSource.slice(unitStart, unitEnd)

  assert.match(unitSource, /productCurrentSalesSpecUnit\(item\)/, 'flat row price unit should use the shared current-sales-spec resolver')
  assert.match(unitSource, /item\.inventory_unit/)
  assert.match(unitSource, /item\.inventoryUnit/)
  assert.match(unitSource, /return Number\(tier\.spec_g \|\| tier\.specG \|\| 0\) === 1000 \? 'kg' : 'lb'/)
})

test('price list product selection summaries expose category and shared parent-product inheritance', () => {
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
    'priceListProductPricingSummary(priceListParentProductPricingRow(row))',
    'priceListProductDisplaySummary(priceListParentProductID(row))',
  ]) {
    assert.ok(selectionSource.includes(expected), `missing compact summary behavior: ${expected}`)
  }
})

test('price list draft restoration promotes legacy SKU pricing and blocks conflicting configs until parent repricing', () => {
  const restoreStart = viewSource.indexOf('function restorePriceListGenerationDraftForActiveType()')
  const restoreEnd = viewSource.indexOf('function beanListPublicationTypeKey', restoreStart)
  assert.ok(restoreStart > -1 && restoreEnd > restoreStart, 'missing price-list draft restore block')
  const restoreSource = viewSource.slice(restoreStart, restoreEnd)

  assert.match(restoreSource, /normalizeParentSharedPriceListProductOverrides\(draft\.productOverrides/)
  assert.match(restoreSource, /productSpecSelections: draft\.product_spec_selections/)
  assert.match(restoreSource, /migrateLegacyFixedPriceFlatRowOverrides/)
  assert.ok(priceListDraftSource.includes("match(/^([0-9]+):fixed_price$/)"))
  assert.ok(priceListDraftSource.includes('`sku:${skuID}`'))
  assert.match(restoreSource, /priceListLegacyPricingConflicts\.value = migratedProductOverrides\.conflicts/)
  assert.match(viewSource, /旧草稿存在规格级计价冲突，发布已阻止/)
  assert.match(viewSource, /priceListLegacyPricingBlockedReason/)
  assert.match(viewSource, /clearPriceListLegacyPricingConflict\(row\.parent_product_id \|\| row\.product_id\)/)
})

test('price list treats tier templates as shared sales-spec counts and keeps fixed prices labeled per selected spec', () => {
  for (const expected of [
    'priceListSalesSpecCountTierLabel',
    'quantity_basis',
    'tier_quantity_unit',
    'priceListProductSpecPricingResolution(row, spec)',
    'priceListProductSpecLabel(spec)',
  ]) {
    assert.ok(viewSource.includes(expected), `missing sales-spec-count tier UI: ${expected}`)
  }
  assert.doesNotMatch(viewSource, /:disabled="priceListTierTemplateOptionDisabled\(template\)"/)
  assert.match(viewSource, /固定价（元\/\{\{ priceListProductSpecLabel\(spec\) \}\}）/)
  assert.doesNotMatch(viewSource, /<select v-model="tier\.quantity_unit">/)
  assert.match(viewSource, />最小件数</)
  assert.match(viewSource, />最大件数</)
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
  assert.ok(groupsSource.includes('buildBeanListPdfGroupsFromCategoryRows(selectedSkuCategoryProductGroups.value'), 'generate drawer should render materialized selected SKU rows from the picker')
  assert.equal(groupsSource.includes('currentPriceSourcePublication.value?.content?.groups'), false, 'current price source must not replace current selected products')
  assert.equal(viewSource.includes('const priceListFlatRows = computed(() => dedupePriceListFlatRows(normalizePriceListPublicationRows('), true, 'flat rows should normalize concrete identities before collapsing identical generated tier rows')
  assert.equal(viewSource.includes('normalizePriceListPublicationGroups('), true, 'preview should normalize stale parent identities before applying flat price rows')
  assert.equal(viewSource.includes('applyPriceListFlatRowsToBeanListPdfGroups(normalizedPriceListGroups.value, priceListFlatRows.value'), true, 'preview should render flat price rows back into normalized PDF groups')
  assert.equal(viewSource.includes("apiSend('/api/costing/pricing-rule-trials'"), true, 'pricing-rule rows should load live trial prices in one batch')
  assert.equal(viewSource.includes("apiSend('/api/costing/pricing-rule-trial'"), false, 'price-list rows should not send one HTTP request per product')
  assert.equal(viewSource.includes('priceTablePricingRuleTrialPayload(row, { customerID: activeBeanListCustomerID.value })'), true, 'pricing-rule trial payload should be scoped to the current customer')
  assert.equal(priceListWorkflowSource.includes("cached?.status === 'error'"), true, 'failed pricing-rule trial requests should not spin in a retry loop')
  assert.equal(viewSource.includes('visibleCategoryCodes: pdfVisibleCategoryCodes.value'), true, 'preview should keep the product picker category codes')
  assert.equal(viewSource.includes('const pdfVisiblePreviewCategoryCodes = computed(() => pdfCategoryCodesForVisibleSelection'), false, 'preview should not translate picker category codes into legacy PDF category codes')
  assert.equal(viewSource.includes('function pdfCategoryCodesForVisibleSelection'), false, 'legacy preview category code mapper should be removed')
  assert.equal(viewSource.includes('<div v-if="priceListFlatRows.length" class="pdf-picker flat-price-row-editor">'), true, 'empty flat price rows should stay hidden')
  assert.equal(viewSource.includes('trialStatusForRow: priceListFlatRowPricingTrialStatus'), true, 'flat price publish readiness should require successful live trials')
  assert.equal(priceListWorkflowSource.includes('return rows.length > 0 && rows.every'), true, 'empty flat price rows should not be publish-ready')
  assert.equal(viewSource.includes('重新试算失败项'), true, 'failed live trials should expose an explicit retry action')
})

test('price list preview spans the page below flat price rows', () => {
  assert.match(viewSource, /<section class="price-list-preview">[\s\S]*?<div class="pdf-preview-title">[\s\S]*?<div class="pdf-preview-phone bean-list-pdf-surface"/)
  assert.match(viewSource, /<div v-if="priceListFlatRows\.length" class="pdf-picker flat-price-row-editor">[\s\S]*?<\/div>\s*<section class="price-list-preview">/)
  assert.match(viewSource, /\.price-list-page-config\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\)[^}]*min-width:\s*0/)
  assert.match(viewSource, /\.price-list-preview\s*\{[^}]*grid-column:\s*1\s*\/\s*-1[^}]*width:\s*100%[^}]*min-width:\s*0/)
  assert.match(viewSource, /\.price-list-preview\s+\.pdf-preview-phone\s*\{[^}]*width:\s*100%[^}]*max-width:\s*none/)
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
    'productSpecSelectionsByType.value = {}',
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
    'greenTierPriceRows(spec)',
    'setGreenBeanTierPrice(priceListSkuID(spec), tier, $event.target.value)',
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

test('product price-list no longer treats a green-bean category template as a pricing prerequisite', () => {
  assert.equal(viewSource.includes("item?.bom_status === 'missing_green_bean_template'"), false)
  assert.equal(viewSource.includes('未挂到带生豆模板的分类，无法生成生豆价格'), false)
  assert.equal(viewSource.includes('请在商品管理里把该生豆商品移到带生豆模板的生豆分类'), false)
})

test('editing a fixed flat row updates the canonical SKU fixed amount instead of a second override', () => {
  const setterStart = viewSource.indexOf('function setPriceListFlatRowPrice(row = {}, value)')
  const setterEnd = viewSource.indexOf('function defaultPriceTierTemplateForm', setterStart)
  assert.ok(setterStart > -1 && setterEnd > setterStart, 'missing flat-row price setter')
  const setterSource = viewSource.slice(setterStart, setterEnd)
  assert.match(setterSource, /row\.pricing_mode.*fixed_price/)
  assert.match(setterSource, /setPriceListSkuFixedPrice\(row, value\)/)
  assert.match(setterSource, /delete next\[key\]/)
})

test('flat price rows expose the shared pricing-rule editor before retry failures', () => {
  const header = viewSource.match(/<div v-if="priceListFlatRows\.length" class="pdf-picker flat-price-row-editor">[\s\S]*?<div class="flat-price-table"/)?.[0] || ''
  assert.match(header, /@click="openPriceListPricingRuleEditor"[^>]*>编辑价格模板<\/button>/)
  assert.match(header, /编辑价格模板<\/button>[\s\S]*重新试算失败项/)
  assert.match(viewSource, /<PricingRuleEditorForm/)
  assert.match(viewSource, /aria-label="商品价格表价格模板编辑"/)
  assert.match(viewSource, /savePriceListPricingRule/)
})

test('flat price row pricing-rule editor is anchored as a full-height right drawer', () => {
  assert.match(viewSource, /<div v-if="priceListPricingRuleEditorDrawerOpen" class="settings-drawer-mask"/)
  assert.match(viewSource, /\.settings-drawer-mask\s*\{[^}]*position:\s*fixed;[^}]*inset:\s*0;[^}]*display:\s*flex;[^}]*justify-content:\s*flex-end;/s)
  assert.match(viewSource, /\.settings-drawer\s*\{[^}]*box-sizing:\s*border-box;[^}]*height:\s*100vh;/s)
})
