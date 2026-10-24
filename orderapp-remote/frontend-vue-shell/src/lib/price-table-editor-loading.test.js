import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { ref, computed, nextTick } from 'vue'
import { publicationEditorPayload, loadPublishedEditorBatch, reconcileEditorBatch, editorTableConflict, resolveEditorTableConflict } from './price-table-editor-restore.js'
import { clonePriceTable, createPriceTableBatch, savePriceTableBatchDraft, readPriceTableBatchDraft, validatePriceTableBatch, publicationTableMetadata } from './price-table-batch.js'
import { defaultPriceListProductSpecSelections, normalizePriceListProductSpecSelections, priceListProductSpecSelectionIssue } from './product-price-list-selection.js'
import { buildPublicationSummaryURL } from './publication-summary-client.js'

const source = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
const block = (start, end) => source.slice(source.indexOf(start), source.indexOf(end, source.indexOf(start)))
function fn(name) {
  const start = source.search(new RegExp(`(?:async )?function ${name}\\(`))
  assert.ok(start >= 0, `missing editor behavior: ${name}`)
  return source.slice(start, source.indexOf('\n}\n', start) + 2)
}
const functions = [
  fn('persistNamedPriceTableBatch'),
  block('async function applyNamedPriceTablePayload()', '\nasync function restoreNamedPriceTableBatch'),
  block('async function restoreNamedPriceTableBatch(', '\nasync function retryNamedPriceTableEditor'),
  block('async function resolveNamedTableDraft(', '\nfunction removeUnavailableEditorSelections'),
  block('async function selectNamedPriceTable(', '\nfunction keepActiveNamedPriceTableTabVisible'),
  fn('saveNamedPriceTableBatch'),
].join('\n')

const spec = { parent_product_id: 940, product_id: 940, bom_id: 18277, bom_version_id: 423, bom_spec_id: 309, bom_variant_id: 607, selection_source: 'explicit' }
const publication = { id: 152, version: 'V3.0.14', status: 'published', release_id: 'release', table_key: 'drip',
  table_name: '挂耳', is_default_table: true,
  config: { product_spec_selections: [spec], price_list_template_selection: { defaults: { pricing_mode: 'fixed_price' } } },
  content: { price_rows: [{ ...spec, pricing_mode: 'fixed_price', final_unit_price: 32.8, price_unit: '盒' }] },
}

function harness({ resolveConflict = resolveEditorTableConflict, saveDraft, readDraft, normalizeSpecs = false } = {}) {
  const scope = ref('customer:471:drip'), state = ref({}), storageData = new Map(), writes = [], pending = []
  const storage = { getItem: k => storageData.get(k), setItem: (k,v) => { writes.push(k); storageData.set(k,v) } }
  const refs = {
    priceListCatalogReady: ref(true), loading: ref(false), priceListPublicationTypeOptionsReady: ref(true), namedPriceTableScope: ref(''),
    namedEditorReadyScope: ref(''), namedEditorLoading: ref(false), namedEditorError: ref(''),
    namedPriceTableBatch: ref(null), publishedEditorBatch: ref(null),
    error: ref(''), message: ref(''),
    activePriceListTypeKey: ref('drip'), activeBeanListCustomerID: ref(471),
    publicationScope: ref('customer'), pdfOptions: ref({}), pdfTheme: ref({ listType: 'commercial' }), pdfCustomizers: ref({}),
    selectedProductPriceListType: ref({ listType: 'commercial' }), selectedProductPriceListLabel: ref('挂耳'),
    pdfAvailableItems: ref([{ parent_product_id: 940, default_sku_id: 309, sku_options: [{ ...spec, sku_id: 309 }] }]),
    priceListGroupTemplateRows: ref([]), downloadSourcePublication: ref(null), priceSourcePublicationByType: ref({}),
    styleSourcePublicationIDByType: ref({}), priceListTemplateDefaults: ref({}), priceListParentTemplateSelections: ref({}),
    priceListGroupTemplateSelections: ref({}), priceListProductTemplateOverrides: ref({}), priceListFlatRowOverrides: ref({}),
    priceListTierFixedPrices: ref({}), productSpecSelectionsByType: ref({}), beanListPublishing: ref(false),
    priceListRefresh: ref({ busy: false }), priceListFlatRows: ref([]), activeProductTypeCategoryID: ref(56),
  }
  refs.activeNamedPriceTable = computed(() => refs.namedPriceTableBatch.value?.tables.find(t => t.key === refs.namedPriceTableBatch.value.active_table_key))
  refs.activePublishedEditorTable = computed(() => refs.publishedEditorBatch.value?.tables.find(t => t.key === refs.activeNamedPriceTable.value?.key))
  refs.namedEditorReady = computed(() => refs.namedEditorReadyScope.value === scope.value && !refs.namedEditorLoading.value && !refs.namedEditorError.value)
  refs.activeNamedTableConflict = computed(() => editorTableConflict(refs.activeNamedPriceTable.value,
    refs.publishedEditorBatch.value?.tables.find(t => t.key === refs.activeNamedPriceTable.value?.key)))
  refs.namedEditorBlocked = computed(() => refs.loading.value || !refs.namedEditorReady.value || refs.activeNamedTableConflict.value)
  const bindings = { ...refs, nextTick, clonePriceTable, createPriceTableBatch, publicationEditorPayload,
    loadPublishedEditorBatch, reconcileEditorBatch, buildPublicationSummaryURL, defaultPriceListProductSpecSelections,
    validatePriceTableBatch, publicationTableMetadata,
    resolveEditorTableConflict: resolveConflict,
    priceListGenerationDraftBaseKey: () => scope.value, activePublicationProductTypeCategoryID: () => 0,
    activePublicationClassificationTemplateID: () => 163, FACTORY_SUPPLY_PUBLICATION_PURPOSE: 'factory_supply',
    publicationRequestOnce: url => new Promise((resolve,reject) => pending.push({ url, resolve, reject })),
    loadBeanListPublicationDetail: async row => row,
    readPriceListGenerationDraft: () => null,
    readPriceTableBatchDraft: key => readDraft ? readDraft(key) : readPriceTableBatchDraft(key, storage),
    savePriceTableBatchDraft: (key,batch) => saveDraft ? saveDraft(key,batch) : savePriceTableBatchDraft(key,batch,storage),
    captureNamedPriceTablePayload: () => ({ draft: clonePriceTable(state.value), content: { groups: [{ name: '商品' }] },
      ...(normalizeSpecs ? { config: { product_spec_selections: clonePriceTable(state.value.product_spec_selections || []) },
        blocked_reason: priceListProductSpecSelectionIssue(refs.pdfAvailableItems.value[0], state.value.product_spec_selections)?.message || '' } : {}) }),
    buildProductPriceListTitle: () => '新表', defaultPriceListTemplateSelection: value => value,
    restorePriceListGenerationDraftForActiveType: draft => {
      state.value = clonePriceTable(draft)
      if (normalizeSpecs) state.value.product_spec_selections = normalizePriceListProductSpecSelections(draft.product_spec_selections, refs.pdfAvailableItems.value, { preserveMissing: true })
    },
    keepActiveNamedPriceTableTabVisible: () => {},
    editorTableConflict,
    loadPriceListPricingRuleTrials: async () => {}, currentPriceListPricingRuleTrialRequests: () => [],
    priceListPricingRuleTasks: new Set(), beanListPublicationPayload: () => ({}),
    apiSend: async (url, options) => { requests.push({ url, ...options }); return { tables: [], version: 'V3.0.15' } },
    loadBeanListPublications: async () => {}, defaultBeanListVersionForScope: () => 'V3.0.16',
  }
  const requests = []
  const setup = new Function(...Object.keys(bindings), 'let namedEditorRevision = 0, restoringNamedPriceTable = false;\n' + functions +
    '\nreturn { restoreNamedPriceTableBatch, persistNamedPriceTableBatch, selectNamedPriceTable, resolveNamedTableDraft, saveNamedPriceTableBatch };')
  return { ...refs, ...setup(...Object.values(bindings)), scope, state, pending, writes, storage, storageData, requests }
}

function draftIndexedDB() {
  const records = new Map()
  const db = {
    close() {}, objectStoreNames: { contains: () => true },
    transaction() {
      let operation, write
      const tx = { objectStore: () => ({
        get(key) { operation = { result: records.get(key) }; return operation },
        put(value, key) { operation = {}; write = () => records.set(key, value); return operation },
      }) }
      setImmediate(() => { write?.(); tx.oncomplete?.() })
      return tx
    },
  }
  return { open() {
    const request = { result: db }
    queueMicrotask(() => request.onsuccess())
    return request
  } }
}

test('actual latest-published loading and server publication work while localStorage is full', async () => {
  const indexedDB = draftIndexedDB()
  let h
  h = harness({
    readDraft: key => readPriceTableBatchDraft(key, h.storage, indexedDB),
    saveDraft: (key, batch) => savePriceTableBatchDraft(key, batch, h.storage, indexedDB),
    resolveConflict: (batch, published, choice) => resolveEditorTableConflict(batch, published, choice, {
      scope: h.scope.value, storage: { setItem() {} },
    }),
  })
  h.storage.setItem = () => { throw new DOMException('quota', 'QuotaExceededError') }
  const load = h.restoreNamedPriceTableBatch()
  for (let attempt = 0; !h.pending.length && attempt < 20; attempt++) await new Promise(resolve => setImmediate(resolve))
  assert.equal(h.pending.length, 1, 'the editor must request the published snapshot')
  h.pending[0].resolve({ current: publication, rows: [publication], total: 1 })
  await load
  h.namedPriceTableBatch.value.tables[0].source_publication.id = 1
  await h.resolveNamedTableDraft('published')
  assert.equal(h.error.value, '')
  assert.match(h.message.value, /已载入最新发布版/)
  assert.equal(h.state.value.productOverrides['sku:309'].fixed_unit_price, 32.8)
  await h.saveNamedPriceTableBatch(true)
  assert.equal(h.requests.length, 1)
  assert.equal(h.requests[0].url, '/api/costing/bean-list/publication-batches')
  assert.equal(h.error.value, '')
  assert.match(h.message.value, /已发布/)
  const restored = await readPriceTableBatchDraft(h.scope.value, h.storage, indexedDB)
  assert.equal(restored.tables[0].payload.draft.productOverrides['sku:309'].fixed_unit_price, 32.8)
})

test('actual server publication awaits the in-flight draft transaction', async () => {
  let gate, finish
  const h = harness({ saveDraft: async () => { if (gate) await gate } })
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
  h.pending[0].resolve({ current: publication, rows: [publication], total: 1 })
  await load
  gate = new Promise(resolve => { finish = resolve })
  const publishing = h.saveNamedPriceTableBatch(true)
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(h.requests.length, 0)
  finish()
  await publishing
  assert.equal(h.requests.length, 1)
})

test('a delayed table switch cannot select an old table in another customer draft', async () => {
  let gate, finish
  const h = harness({ saveDraft: async key => { if (key === 'customer:471:drip' && gate) await gate } })
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
  h.pending[0].resolve({ current: publication, rows: [publication, { ...publication, id: 153, table_key: 'second', is_default_table: false }], total: 1 })
  await load
  gate = new Promise(resolve => { finish = resolve })
  const selecting = h.selectNamedPriceTable('second')
  await nextTick()
  h.scope.value = 'customer:472:drip'
  const otherLoad = h.restoreNamedPriceTableBatch()
  await nextTick()
  const otherPublication = { ...publication, id: 154, table_key: 'other-customer' }
  h.pending[1].resolve({ current: otherPublication, rows: [otherPublication], total: 1 })
  await otherLoad
  finish()
  await selecting
  assert.equal(h.namedPriceTableScope.value, 'customer:472:drip')
  assert.equal(h.namedPriceTableBatch.value.active_table_key, 'other-customer')
  assert.equal(h.state.value.productOverrides['sku:309'].fixed_unit_price, 32.8)
})

test('a delayed publication action cannot publish the next customer after changing scope', async () => {
  let gate, finish
  const h = harness({ saveDraft: async key => { if (key === 'customer:471:drip' && gate) await gate } })
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
  h.pending[0].resolve({ current: publication, rows: [publication], total: 1 })
  await load
  gate = new Promise(resolve => { finish = resolve })
  const publishing = h.saveNamedPriceTableBatch(true)
  await new Promise(resolve => setImmediate(resolve))
  h.scope.value = 'customer:472:drip'
  const otherLoad = h.restoreNamedPriceTableBatch()
  await nextTick()
  const otherPublication = { ...publication, id: 154, table_key: 'other-customer' }
  h.pending[1].resolve({ current: otherPublication, rows: [otherPublication], total: 1 })
  await otherLoad
  finish()
  await publishing
  assert.equal(h.requests.length, 0)
  assert.equal(h.namedPriceTableBatch.value.active_table_key, 'other-customer')
})

test('repeat publish clicks share one in-flight action while the draft transaction is pending', async () => {
  let gate, finish
  const h = harness({ saveDraft: async () => { if (gate) await gate } })
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
  h.pending[0].resolve({ current: publication, rows: [publication], total: 1 })
  await load
  gate = new Promise(resolve => { finish = resolve })
  const first = h.saveNamedPriceTableBatch(true)
  await new Promise(resolve => setImmediate(resolve))
  const busyWhileSaving = h.beanListPublishing.value
  const second = h.saveNamedPriceTableBatch(true)
  await nextTick()
  finish()
  await Promise.all([first, second])
  assert.equal(busyWhileSaving, true)
  assert.equal(h.requests.length, 1)
})

test('actual server publication can recover a draft when all browser saving is unavailable', async () => {
  const h = harness()
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
  h.pending[0].resolve({ current: publication, rows: [publication], total: 1 })
  await load
  h.storage.setItem = () => { throw new DOMException('quota', 'QuotaExceededError') }
  await h.saveNamedPriceTableBatch(true)
  assert.equal(h.requests.length, 1)
  assert.match(h.message.value, /已发布/)
  assert.match(h.error.value, /本地草稿保存失败/)
  assert.doesNotMatch(h.error.value, /QuotaExceededError|setItem/)
  assert.equal(h.state.value.productOverrides['sku:309'].fixed_unit_price, 32.8)
})

test('hydrated named editor routes pricing edits through the named batch rather than legacy localStorage', async () => {
  let saved = 0
  const bindings = {
    namedEditorBlocked: ref(false), restoringNamedPriceTable: false, namedEditorReady: ref(true),
    persistNamedPriceTableBatch: async () => { saved++ },
    savePriceListGenerationDraft: () => { throw new DOMException('quota', 'QuotaExceededError') },
  }
  const save = new Function(...Object.keys(bindings), fn('savePriceListGenerationDraftForActiveType') +
    '\nreturn savePriceListGenerationDraftForActiveType;')(...Object.values(bindings))
  await save()
  assert.equal(saved, 1)
})

test('actual view waits for catalogue and snapshot before applying or persisting selections', async () => {
  const h = harness()
  h.priceListPublicationTypeOptionsReady.value = false
  await h.restoreNamedPriceTableBatch()
  assert.equal(h.pending.length, 0)
  h.priceListPublicationTypeOptionsReady.value = true
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
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
  await nextTick()
  h.scope.value = 'customer:472:drip'
  h.pending[0].resolve({ current: publication, rows: [publication] })
  await load
  assert.equal(h.namedPriceTableBatch.value, null)
  assert.equal(h.writes.length, 0)
  const retry = h.restoreNamedPriceTableBatch()
  await nextTick()
  h.pending[1].reject(Error('offline'))
  await retry
  assert.equal(h.namedEditorError.value, 'offline')
  assert.equal(h.namedEditorBlocked.value, true)
  assert.equal(h.writes.length, 0)
})

for (const publish of [false, true]) test(`actual view ${publish ? 'publishes' : 'saves'} all named tables with current spec identities and no obsolete default warnings`, async () => {
  const h = harness({ normalizeSpecs: true })
  const second = { ...publication, id: 153, table_key: 'second', table_name: '另一张挂耳表', is_default_table: false }
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
  h.pending[0].resolve({ current: publication, rows: [publication, second], total: 1 })
  await load
  const nextSpec = { ...spec, sku_id: 309, bom_version_id: 424, bom_variant_id: 608 }
  h.pdfAvailableItems.value = [{ parent_product_id: 940, default_sku_id: 310, sku_options: [nextSpec, { sku_id: 310, bom_spec_id: 310 }] }]
  for (const table of h.namedPriceTableBatch.value.tables) {
    table.payload.draft = { product_spec_selections: [{ ...spec, sku_id: 309, selection_issue: 'default_bom_changed' }], productOverrides: { 'sku:309': { fixed_unit_price: table.key === 'drip' ? 40 : 32.8 } } }
    table.payload.config = { product_spec_selections: [{ ...spec, selection_issue: 'default_changed' }] }
  }
  h.state.value = clonePriceTable(h.namedPriceTableBatch.value.tables[0].payload.draft)
  await h.saveNamedPriceTableBatch(publish)
  assert.equal(h.error.value, '')
  assert.equal(h.requests.length, 1)
  const tables = h.requests[0].body.tables
  assert.equal(tables.length, 2)
  for (const table of tables) {
    const selected = table.config.product_spec_selections[0]
    assert.equal(selected.bom_spec_id, 309)
    assert.equal(selected.bom_version_id, 424)
    assert.equal(selected.bom_variant_id, 608)
    assert.equal(selected.selection_issue, undefined)
  }
  assert.equal(h.namedPriceTableBatch.value.active_table_key, 'drip')
  assert.equal(h.state.value.productOverrides['sku:309'].fixed_unit_price, 40)
})

test('actual view switching tables preserves edited amount and an intentional empty selection', async () => {
  const h = harness()
  const second = { ...publication, id: 153, table_key: 'second', is_default_table: false }
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
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

test('actual view waits for backup, blocks repeat clicks and only then applies the publication', async () => {
  let finishBackup, calls = 0
  const backup = new Promise(resolve => { finishBackup = resolve })
  const h = harness({ resolveConflict: () => { calls++; return backup } })
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
  h.pending[0].resolve({ current: publication, rows: [publication], total: 1 })
  await load
  const published = clonePriceTable(h.namedPriceTableBatch.value)
  h.namedPriceTableBatch.value.tables[0].source_publication.id = 1
  const old = h.namedPriceTableBatch.value
  const resolving = h.resolveNamedTableDraft('published')
  await h.resolveNamedTableDraft('published')
  assert.equal(calls, 1)
  assert.equal(h.namedPriceTableBatch.value, old)
  assert.equal(h.namedEditorLoading.value, true)
  finishBackup(published)
  await resolving
  assert.equal(h.namedPriceTableBatch.value.tables[0].source_publication.id, publication.id)
  assert.equal(h.namedEditorLoading.value, false)
  assert.match(h.message.value, /已载入最新发布版/)
  assert.equal(h.error.value, '')
})

test('a late backup result cannot overwrite another customer or unlock its pending editor load', async () => {
  let finishBackup
  const backup = new Promise(resolve => { finishBackup = resolve })
  const h = harness({ resolveConflict: () => backup })
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
  h.pending[0].resolve({ current: publication, rows: [publication], total: 1 })
  await load
  const published = clonePriceTable(h.namedPriceTableBatch.value)
  h.namedPriceTableBatch.value.tables[0].source_publication.id = 1
  const old = h.namedPriceTableBatch.value
  const resolving = h.resolveNamedTableDraft('published')
  h.scope.value = 'customer:472:drip'
  const otherLoad = h.restoreNamedPriceTableBatch()
  await nextTick()
  finishBackup(published)
  await resolving
  assert.equal(h.namedPriceTableBatch.value, old)
  assert.equal(h.namedEditorLoading.value, true)
  assert.equal(h.message.value, '')
  h.pending[1].resolve({ current: null, rows: [], total: 0 })
  await otherLoad
  assert.equal(h.namedPriceTableScope.value, h.scope.value)
  assert.equal(h.namedEditorLoading.value, false)
})


test('a new customer with no publication starts with clean pricing rather than the previous customer prices', async () => {
  const h = harness()
  h.state.value = { customerPriceSeedRows: [{ final_unit_price: 999 }], productOverrides: { old: 999 } }
  const load = h.restoreNamedPriceTableBatch()
  await nextTick()
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

test('generate-new-version entry preserves the hydrated customer table brand', () => {
  const pdfOptions = ref({ listType: 'drip', brandName: '客户自定义抬头' })
  const bindings = { pdfOptions, namedEditorReady: ref(true), downloadSourcePublication: ref(null),
    selectedProductPriceListType: ref({ listType: 'drip' }), activeCostingScope: ref('customer'), activeProductTypeCategoryID: ref(163), pdfDrawerOpen: ref(false),
    syncPublicationScopeFromPageContext: () => {}, normalizeBeanListType: v => v,
    defaultBeanListVersionForScope: () => 'V3.0.15', initializePdfDefaultsForType: () => {},
    restorePriceListGenerationDraftForActiveType: () => {}, loadActiveProductTypePublicationViews: () => {},
  }
  const open = new Function(...Object.keys(bindings), block('function openBeanListDrawer(', '\nfunction defaultBeanListVersionForScope') + '\nreturn openBeanListDrawer;')(...Object.values(bindings))
  open()
  assert.equal(pdfOptions.value.brandName, '客户自定义抬头')
})
