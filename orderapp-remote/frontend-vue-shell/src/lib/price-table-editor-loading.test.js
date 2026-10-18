import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { ref, computed, nextTick } from 'vue'
import { publicationEditorPayload, loadPublishedEditorBatch, reconcileEditorBatch, editorTableConflict } from './price-table-editor-restore.js'
import { clonePriceTable, createPriceTableBatch, savePriceTableBatchDraft, readPriceTableBatchDraft } from './price-table-batch.js'
import { defaultPriceListProductSpecSelections } from './product-price-list-selection.js'
import { buildPublicationSummaryURL } from './publication-summary-client.js'

const source = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
const block = (start, end) => source.slice(source.indexOf(start), source.indexOf(end, source.indexOf(start)))
const functions = [
  block('function persistNamedPriceTableBatch()', '\nasync function applyNamedPriceTablePayload'),
  block('async function applyNamedPriceTablePayload()', '\nasync function restoreNamedPriceTableBatch'),
  block('async function restoreNamedPriceTableBatch(', '\nasync function retryNamedPriceTableEditor'),
  block('async function selectNamedPriceTable(', '\nfunction keepActiveNamedPriceTableTabVisible'),
].join('\n')

const spec = { parent_product_id: 940, product_id: 940, bom_id: 18277, bom_version_id: 423, bom_spec_id: 309, bom_variant_id: 607, selection_source: 'explicit' }
const publication = { id: 152, version: 'V3.0.14', status: 'published', release_id: 'release', table_key: 'drip',
  table_name: '挂耳', is_default_table: true,
  config: { product_spec_selections: [spec], price_list_template_selection: { defaults: { pricing_mode: 'fixed_price' } } },
  content: { price_rows: [{ ...spec, pricing_mode: 'fixed_price', final_unit_price: 32.8, price_unit: '盒' }] },
}

function harness() {
  const scope = ref('customer:471:drip'), state = ref({}), storageData = new Map(), writes = [], pending = []
  const storage = { getItem: k => storageData.get(k), setItem: (k,v) => { writes.push(k); storageData.set(k,v) } }
  const refs = {
    priceListCatalogReady: ref(true), loading: ref(false), priceListPublicationTypeOptionsReady: ref(true), namedPriceTableScope: ref(''),
    namedEditorReadyScope: ref(''), namedEditorLoading: ref(false), namedEditorError: ref(''),
    namedPriceTableBatch: ref(null), publishedEditorBatch: ref(null),
    activePriceListTypeKey: ref('drip'), activeBeanListCustomerID: ref(471),
    publicationScope: ref('customer'), pdfOptions: ref({}), pdfTheme: ref({ listType: 'commercial' }), pdfCustomizers: ref({}),
    selectedProductPriceListType: ref({ listType: 'commercial' }), selectedProductPriceListLabel: ref('挂耳'),
    pdfAvailableItems: ref([{ parent_product_id: 940, default_sku_id: 309, sku_options: [{ ...spec, sku_id: 309 }] }]),
    priceListGroupTemplateRows: ref([]), downloadSourcePublication: ref(null), priceSourcePublicationByType: ref({}),
    styleSourcePublicationIDByType: ref({}), priceListTemplateDefaults: ref({}), priceListParentTemplateSelections: ref({}),
    priceListGroupTemplateSelections: ref({}), priceListProductTemplateOverrides: ref({}), priceListFlatRowOverrides: ref({}),
    priceListTierFixedPrices: ref({}), productSpecSelectionsByType: ref({}), beanListPublishing: ref(false),
  }
  refs.activeNamedPriceTable = computed(() => refs.namedPriceTableBatch.value?.tables.find(t => t.key === refs.namedPriceTableBatch.value.active_table_key))
  refs.namedEditorReady = computed(() => refs.namedEditorReadyScope.value === scope.value && !refs.namedEditorLoading.value && !refs.namedEditorError.value)
  refs.activeNamedTableConflict = computed(() => editorTableConflict(refs.activeNamedPriceTable.value,
    refs.publishedEditorBatch.value?.tables.find(t => t.key === refs.activeNamedPriceTable.value?.key)))
  refs.namedEditorBlocked = computed(() => refs.loading.value || !refs.namedEditorReady.value || refs.activeNamedTableConflict.value)
  const bindings = { ...refs, nextTick, clonePriceTable, createPriceTableBatch, publicationEditorPayload,
    loadPublishedEditorBatch, reconcileEditorBatch, buildPublicationSummaryURL, defaultPriceListProductSpecSelections,
    priceListGenerationDraftBaseKey: () => scope.value, activePublicationProductTypeCategoryID: () => 0,
    activePublicationClassificationTemplateID: () => 163, FACTORY_SUPPLY_PUBLICATION_PURPOSE: 'factory_supply',
    publicationRequestOnce: url => new Promise((resolve,reject) => pending.push({ url, resolve, reject })),
    loadBeanListPublicationDetail: async row => row,
    readPriceListGenerationDraft: () => null,
    readPriceTableBatchDraft: key => readPriceTableBatchDraft(key, storage),
    savePriceTableBatchDraft: (key,batch) => savePriceTableBatchDraft(key,batch,storage),
    captureNamedPriceTablePayload: () => ({ draft: clonePriceTable(state.value) }),
    buildProductPriceListTitle: () => '新表', defaultPriceListTemplateSelection: value => value,
    restorePriceListGenerationDraftForActiveType: draft => { state.value = clonePriceTable(draft) },
    keepActiveNamedPriceTableTabVisible: () => {},
  }
  const setup = new Function(...Object.keys(bindings), 'let namedEditorRevision = 0, restoringNamedPriceTable = false;\n' + functions +
    '\nreturn { restoreNamedPriceTableBatch, persistNamedPriceTableBatch, selectNamedPriceTable };')
  return { ...refs, ...setup(...Object.values(bindings)), scope, state, pending, writes, storage, storageData }
}

test('actual view waits for catalogue and snapshot before applying or persisting selections', async () => {
  const h = harness()
  h.priceListPublicationTypeOptionsReady.value = false
  await h.restoreNamedPriceTableBatch()
  assert.equal(h.pending.length, 0)
  h.priceListPublicationTypeOptionsReady.value = true
  const load = h.restoreNamedPriceTableBatch()
  h.persistNamedPriceTableBatch()
  assert.equal(h.writes.length, 0)
  assert.match(h.pending[0].url, /classification_template_id=163/)
  assert.match(h.pending[0].url, /customer_id=471/)
  h.pending[0].resolve({ current: publication, rows: [publication], total: 1 })
  await load
  assert.equal(h.state.value.product_spec_selections[0].sku_id, 309)
  assert.equal(h.state.value.productOverrides['sku:309'].fixed_unit_price, 32.8)
  assert.equal(h.namedEditorBlocked.value, false)
  assert.ok(h.writes.length > 0)
})

test('actual view rejects late customer responses and preserves cache on failed reload', async () => {
  const h = harness()
  const load = h.restoreNamedPriceTableBatch()
  h.scope.value = 'customer:472:drip'
  h.pending[0].resolve({ current: publication, rows: [publication] })
  await load
  assert.equal(h.namedPriceTableBatch.value, null)
  assert.equal(h.writes.length, 0)
  const retry = h.restoreNamedPriceTableBatch()
  h.pending[1].reject(Error('offline'))
  await retry
  assert.equal(h.namedEditorError.value, 'offline')
  assert.equal(h.namedEditorBlocked.value, true)
  assert.equal(h.writes.length, 0)
})

test('actual view switching tables preserves edited amount and an intentional empty selection', async () => {
  const h = harness()
  const second = { ...publication, id: 153, table_key: 'second', is_default_table: false }
  const load = h.restoreNamedPriceTableBatch()
  h.pending[0].resolve({ current: publication, rows: [publication, second], total: 1 })
  await load
  h.state.value.productOverrides['sku:309'].fixed_unit_price = 40
  h.state.value.product_spec_selections = []
  await h.selectNamedPriceTable('second')
  assert.equal(h.state.value.productOverrides['sku:309'].fixed_unit_price, 32.8)
  await h.selectNamedPriceTable('drip')
  assert.equal(h.state.value.productOverrides['sku:309'].fixed_unit_price, 40)
  assert.deepEqual(h.state.value.product_spec_selections, [])
})


test('a new customer with no publication starts with clean pricing rather than the previous customer prices', async () => {
  const h = harness()
  h.state.value = { customerPriceSeedRows: [{ final_unit_price: 999 }], productOverrides: { old: 999 } }
  const load = h.restoreNamedPriceTableBatch()
  h.pending[0].resolve({ rows: [], total: 0, current: null })
  await load
  assert.deepEqual(h.state.value.customerPriceSeedRows, [])
  assert.deepEqual(h.state.value.productOverrides, {})
  assert.equal(h.state.value.product_spec_selections.length, 1)
})

test('catalogue loader awaits customer aliases before restoring and never restores after alias failure', async () => {
  for (const fail of [false, true]) {
    let resolveAliases, rejectAliases, restored = 0
    const aliases = new Promise((resolve,reject) => { resolveAliases = resolve; rejectAliases = reject })
    const bindings = {
      loading: ref(false), namedEditorReadyScope: ref(''), namedEditorLoading: ref(false), priceListCatalogReady: ref(false),
      error: ref(''), namedEditorError: ref(''), message: ref(''), activeBeanListCustomerID: ref(471),
      items: ref([{ id: 'previous' }]), parameters: ref({}),
      apiGet: async () => ({ items: [{ id: 'new' }], parameters: {} }),
      beanListURLForCustomerRules: () => '/api/costing/bean-list?customer_id=471',
      persistNamedPriceTableBatch: () => {},
    loadPriceListProductBusinessGroups: async () => {},
      loadCustomerProductAliases: () => aliases, loadPriceListTemplateOptions: async () => {},
      visibleRowsForProductSpecMigration: rows => rows, syncSelectedProductTypeCategoryFromOptions: () => {},
      initializePdfDefaults: () => {}, restoreNamedPriceTableBatch: async () => { restored++ },
    }
    const load = new Function(...Object.keys(bindings), 'let beanListLoadRevision = 0, namedEditorRevision = 0;\n' +
      block('async function loadBeanList()', '\nlet customerPriceCatalogRevision') + '\nreturn loadBeanList;')(...Object.values(bindings))
    const pending = load()
    await nextTick()
    assert.equal(restored, 0)
    assert.deepEqual(bindings.items.value, [{ id: 'previous' }])
    if (fail) rejectAliases(Error('aliases offline'))
    else resolveAliases()
    await pending
    assert.equal(restored, fail ? 0 : 1)
    assert.equal(bindings.priceListCatalogReady.value, !fail)
    if (fail) assert.match(bindings.namedEditorError.value, /aliases offline/)
  }
})

test('opening configuration after hydration never reloads a stale legacy selection cache', () => {
  let reads = 0
  const bindings = {
    clonePriceTable, namedEditorBlocked: ref(false), namedEditorReady: ref(true), restoringNamedPriceTable: false,
    priceListLegacyPricingConflicts: ref([]), priceListDisplayOrder: ref({}),
    priceListGenerationDraftStorageKey: () => 'current-table',
    readPriceListGenerationDraft: () => { reads += 1; return null },
  }
  const restore = new Function(...Object.keys(bindings), "let customerPriceSeedScope = 'current-table';\n" +
    block('function restorePriceListGenerationDraftForActiveType(', '\nfunction beanListPublicationTypeKey') +
    '\nreturn restorePriceListGenerationDraftForActiveType;')(...Object.values(bindings))
  assert.equal(restore(), true)
  assert.equal(reads, 0)
})
