import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import { readFileSync } from 'node:fs'
import { ref, computed, nextTick, watch } from 'vue'
import { normalizePriceListProductSpecSelections } from './product-price-list-selection.js'
import { fetchPriceListRefreshSnapshot } from './price-list-refresh.js'
import { applyPricingRuleTrialToPriceTableRow } from './product-settings.js'

const source = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
const plain = value => JSON.parse(JSON.stringify(value))
function fn(name) {
  const start = source.search(new RegExp(`(?:async )?function ${name}\\(`))
  assert.ok(start >= 0, `missing refresh behavior: ${name}`)
  return source.slice(start, source.indexOf('\n}\n', start) + 2)
}
function deferred() { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no }); return { promise, resolve, reject } }
function setup() {
  const state = {
    ref, computed, watch, nextTick, normalizePriceListProductSpecSelections,
    loading: ref(false), beanListPublishing: ref(false), priceListPricingRuleEditorSaving: ref(false),
    priceListRefresh: ref({ kind: '', busy: false, error: '', message: '' }),
    priceListRefreshContext: ref('official:beans:table-a'),
    activeBeanListCustomerID: ref(0), activePriceListTypeKey: ref('beans'),
    items: ref([{ id: 1, old: true }]), parameters: ref({ old: true }),
    priceListProductBusinessGroups: ref([]), priceListProductBusinessGroupAssignments: ref([]),
    priceListProductCatalogFeatureSelection: ref({}), priceListProductCatalogFeatureSelectionLoaded: ref(true),
    priceTierTemplates: ref([{ id: 1, name: 'old' }]), pricingRules: ref([{ id: 2, name: 'old' }]),
    priceListFlatRowOverrides: ref({ manual: 99 }), customerPriceConfiguredSources: ref({}),
    productSpecSelectionsByType: ref({ beans: [{ parent_product_id: 1, sku_id: 11, selection_source: 'explicit' }], other: [{ sku_id: 90 }] }),
    pdfAvailableItems: ref([{ parent_product_id: 1, default_sku_id: 12, sku_options: [{ sku_id: 12, bom_spec_id: 12, bom_version_id: 2 }] }]),
    pdfProductSpecSelectionIssues: ref([]),
    priceListFlatRows: ref([{ row_key: 'auto', pricing_mode: 'pricing_rule', pricing_rule_id: 2 }]),
    priceListPricingRuleTrialCache: ref({ old: { status: 'loading' }, auto: { status: 'success', result: { price: 10 } } }),
    priceListPricingRuleTrialGeneration: new Map(), priceListPricingRuleTasks: new Set(),
    visibleRowsForProductSpecMigration: rows => rows,
    defaultPriceTierTemplateForm: row => row,
    priceListFlatRowVisibleErrors: () => [], priceListFlatRowPricingTrialStatus: () => 'success',
    currentPriceListPricingRuleTrialRequests: () => [{ key: 'auto' }],
    refreshCalls: [], trialCalls: 0, saves: 0,
    savePriceListGenerationDraftForActiveType: () => state.saves++,
    persistNamedPriceTableBatch: () => {},
    fetchPriceListRefreshSnapshot: async args => { state.refreshCalls.push(args); return {
      items: [{ id: 1, fresh: true }], parameters: { fresh: true },
      groups: [{ id: 3 }], assignments: [{ object_id: 1, group_item_id: 4 }], featureSelection: { group_template_ids: [3] },
      templates: [{ id: 1, name: 'new' }], rules: [{ id: 2, name: 'new' }],
    } },
    apiGet: () => { throw new Error('unexpected direct request') },
    loadPriceListPricingRuleTrials: async () => { state.trialCalls++ },
  }
  const context = vm.createContext(state)
  vm.runInContext(`let priceListRefreshRevision = 0; let priceListTemplateOptionsRevision = 0;\n${fn('refreshPriceListData')}\n${fn('invalidatePriceListTrialCache')}`, context)
  const stop = watch(state.priceListRefreshContext, () => vm.runInContext('priceListRefreshRevision++; priceListRefresh.value = {kind:"",busy:false,error:"",message:""}', context), { flush: 'sync' })
  return { state, run: kind => context.refreshPriceListData(kind), stop }
}

test('spec refresh immediately reports pending, loads new candidates, preserves draft selections and manual prices', async () => {
  const { state, run, stop } = setup(), gate = deferred()
  const read = state.fetchPriceListRefreshSnapshot
  state.fetchPriceListRefreshSnapshot = async args => { await gate.promise; return read(args) }
  const task = run('products')
  assert.equal(state.priceListRefresh.value.busy, true)
  assert.equal(state.items.value[0].old, true)
  await run('products')
  gate.resolve(); await task
  assert.equal(state.refreshCalls.length, 1)
  assert.equal(state.items.value[0].fresh, true)
  assert.equal(state.productSpecSelectionsByType.value.beans[0].selection_issue, 'invalid_spec')
  assert.deepEqual(plain(state.productSpecSelectionsByType.value.other), [{ sku_id: 90 }])
  assert.deepEqual(plain(state.priceListFlatRowOverrides.value), { manual: 99 })
  assert.match(state.priceListRefresh.value.message, /已刷新/)
  assert.equal(state.priceListRefresh.value.busy, false)
  stop()
})

test('catalog request failure retains every previous value and offers retry in the same operation', async () => {
  const { state, run, stop } = setup()
  state.fetchPriceListRefreshSnapshot = async () => { throw new Error('分类读取失败') }
  await run('products')
  assert.equal(state.items.value[0].old, true)
  assert.deepEqual(plain(state.productSpecSelectionsByType.value.beans), [{ parent_product_id: 1, sku_id: 11, selection_source: 'explicit' }])
  assert.equal(state.priceTierTemplates.value[0].name, 'old')
  assert.match(state.priceListRefresh.value.error, /分类读取失败/)
  assert.equal(state.priceListRefresh.value.busy, false)
  stop()
})

test('price refresh reloads templates, invalidates successful and in-flight costs, waits for the new trials', async () => {
  const { state, run, stop } = setup(), gate = deferred(), started = deferred()
  state.loadPriceListPricingRuleTrials = async () => { state.trialCalls++; started.resolve(); await gate.promise }
  const task = run('prices')
  await started.promise
  assert.equal(state.priceTierTemplates.value[0].name, 'new')
  assert.equal(state.priceListPricingRuleTrialGeneration.get('old'), 1)
  assert.equal(state.priceListPricingRuleTrialGeneration.get('auto'), 1)
  assert.equal(state.priceListRefresh.value.busy, true)
  assert.equal(state.priceListRefresh.value.message, '')
  gate.resolve(); await task
  assert.equal(state.trialCalls, 1)
  assert.match(state.priceListRefresh.value.message, /已刷新/)
  stop()
})

test('trial errors are reported as incomplete refresh, not success', async () => {
  const { state, run, stop } = setup()
  state.priceListFlatRowVisibleErrors = () => ['价格计算失败：缺少 BOM']
  await run('prices')
  assert.match(state.priceListRefresh.value.error, /1.*行/)
  assert.equal(state.priceListRefresh.value.message, '')
  stop()
})

test('late response after switching customer or named table cannot update the new draft, even after switching back', async () => {
  const { state, run, stop } = setup(), gate = deferred()
  const read = state.fetchPriceListRefreshSnapshot
  state.fetchPriceListRefreshSnapshot = async args => { await gate.promise; return read(args) }
  const task = run('prices')
  state.priceListRefreshContext.value = 'customer:102:beans:table-b'
  state.priceListRefreshContext.value = 'official:beans:table-a'
  gate.resolve(); await task
  assert.equal(state.items.value[0].old, true)
  assert.equal(state.priceTierTemplates.value[0].name, 'old')
  assert.equal(state.saves, 0)
  stop()
})

test('refresh is blocked while the initial load or publication is running', async () => {
  const { state, run, stop } = setup()
  state.loading.value = true; await run('products')
  state.loading.value = false; state.beanListPublishing.value = true; await run('prices')
  assert.equal(state.refreshCalls.length, 0)
  stop()
})

test('public refresh reads bean-list, catalog and template API contracts without any business writes', async () => {
  const requests = []
  const responses = {
    '/api/costing/bean-list': { items: [{ product_id: 1, bom_spec_id: 21 }], parameters: { tax: 6 } },
    '/api/business-groups': { rows: [{ id: 3 }] },
    '/api/business-group-assignments?usage_key=product_catalog&object_key=product': { rows: [{ object_id: 1 }] },
    '/api/business-group-feature-selections/product_catalog': { group_template_ids: [3] },
    '/api/price-tier-templates': { templates: [{ id: 4 }, { id: 5, active: false }] },
    '/api/product-pricing-rules': { rules: [{ id: 6, version: 'new' }, { id: 7, active: false }] },
  }
  const result = await fetchPriceListRefreshSnapshot({ apiGet: async url => { requests.push(url); return responses[url] }, prices: true })
  assert.deepEqual(new Set(requests), new Set(Object.keys(responses)))
  assert.equal(result.items[0].bom_spec_id, 21)
  assert.deepEqual(result.templates, [{ id: 4 }])
  assert.deepEqual(result.rules, [{ id: 6, version: 'new' }])
})

test('customer selection refresh uses only scoped catalog and bean-list, and does not load unrelated templates', async () => {
  const urls = []
  await fetchPriceListRefreshSnapshot({ customerID: 102, apiGet: async url => { urls.push(url); return {} } })
  assert.deepEqual(new Set(urls), new Set(['/api/product-settings/customer-catalog?customer_id=102', '/api/costing/bean-list?customer_id=102']))
})

test('a single failed dependency rejects the refresh snapshot instead of silently clearing that panel', async () => {
  await assert.rejects(fetchPriceListRefreshSnapshot({ prices: true, apiGet: async url => {
    if (url === '/api/product-pricing-rules') throw new Error('模板加载失败')
    return { items: [{ id: 1 }] }
  } }), /模板加载失败/)
})

test('an explicitly entered price stays fixed after refresh even when it equaled the former template price', () => {
  const context = vm.createContext({
    priceListFlatRowOverrides: ref({ manual: 40 }), tierFlatFinalPrice: () => 40,
    flatRowPriceUnit: () => 'kg', pricingRuleVersion: () => 'new',
    itemSkuID: () => 11, itemParentProductID: () => 1, itemSkuSnapshot: () => ({}), itemProductID: () => 1,
    priceListGroupSnapshot: () => ({}), flatRowInventoryConversion: () => ({}),
    costSourceSnapshotForPriceRow: () => ({}), customerReferenceSnapshotForPriceRow: () => ({}),
    priceListPricingRuleTrialResultForRow: () => ({ final_unit_price: 66, quote_unit: 'kg' }),
    applyPricingRuleTrialToPriceTableRow,
  })
  vm.runInContext(fn('priceListFlatRowFromSource'), context)
  const row = context.priceListFlatRowFromSource({ rowKey: 'manual', resolved: { pricing_mode: 'pricing_rule' } })
  assert.equal(row.final_unit_price, 40)
  assert.equal(row.original_final_unit_price, 66)
})

test('repeated refresh cannot silently confirm an unresolved BOM version change', async () => {
  const { state, run, stop } = setup()
  state.productSpecSelectionsByType.value.beans = [{ parent_product_id: 1, sku_id: 11, bom_spec_id: 11, bom_version_id: 1 }]
  state.pdfAvailableItems.value = [{ parent_product_id: 1, default_sku_id: 11, sku_options: [{ sku_id: 11, bom_spec_id: 11, bom_version_id: 2 }] }]
  await run('products')
  assert.equal(state.productSpecSelectionsByType.value.beans[0].selection_issue, 'default_bom_changed')
  await run('products')
  assert.equal(state.productSpecSelectionsByType.value.beans[0].selection_issue, 'default_bom_changed')
  stop()
})

test('a superseded trial response cannot overwrite the newly requested price', async () => {
  const { state, stop } = setup(), old = deferred(), fresh = deferred()
  state.priceListPricingRuleTrialCache.value = {}
  const context = vm.createContext({ ...state,
    executePriceListPricingRuleTrialBatches: () => old.promise,
  })
  vm.runInContext(fn('mergePriceListPricingRuleTrialCache') + fn('loadPriceListPricingRuleTrials') + fn('invalidatePriceListTrialCache'), context)
  const previous = context.loadPriceListPricingRuleTrials([{ key: 'same-rule-and-spec' }])
  context.invalidatePriceListTrialCache()
  context.executePriceListPricingRuleTrialBatches = () => fresh.promise
  const current = context.loadPriceListPricingRuleTrials([{ key: 'same-rule-and-spec' }])
  old.resolve({ 'same-rule-and-spec': { status: 'success', result: { final_unit_price: 40 } } }); await previous
  assert.equal(state.priceListPricingRuleTrialCache.value['same-rule-and-spec'].status, 'loading')
  fresh.resolve({ 'same-rule-and-spec': { status: 'success', result: { final_unit_price: 66 } } }); await current
  assert.equal(state.priceListPricingRuleTrialCache.value['same-rule-and-spec'].result.final_unit_price, 66)
  stop()
})
