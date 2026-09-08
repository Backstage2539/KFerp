import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { computed, effectScope, nextTick, ref, watch } from 'vue'
import { applyCustomerPriceRows, seedCustomerPriceRows } from './customer-price-draft.js'

const view = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
const sourceFlow = view.slice(view.indexOf('watch([generatedPriceListFlatRows,'), view.indexOf('\nwatch(activeBeanListCustomerID, () => {'))
const rows = [28, 33, 26, 24].map((price, i) => ({
  parent_product_id: 1063, product_id: 1063, bom_spec_id: 3, bom_variant_id: 483,
  row_key: `227g:${i}`, spec_label: '227g', final_unit_price: price, price_unit: '袋',
  min_qty: [14, 2, 24, 48][i], max_qty: [23, 13, 47, null][i],
}))
const official = { id: 122, owner_type: 'official', status: 'published', version: 'V3.0.22', content: { price_rows: rows } }

function harness(t) {
  const scope = effectScope()
  t.after(() => scope.stop())
  const customerID = ref(302), tableKey = ref(''), seeds = ref([]), drafts = new Map(), pending = []
  const baseKey = () => `customer:${customerID.value}:coffee`
  const draftKey = () => baseKey() + (tableKey.value ? `:table:${tableKey.value}` : '')
  const generated = ref([{ ...rows[0], customer_reference_snapshot: { customer_id: 302 }, final_unit_price: 999 }])
  const bindings = {
    watch, generatedPriceListFlatRows: generated, customerPriceSources: ref([]), customerPriceSourcesReadyKey: ref(''),
    activeBeanListCustomerID: customerID, activePriceListTypeKey: ref('coffee'), customerPriceSeedRows: seeds,
    priceListGenerationDraftBaseKey: baseKey, priceListGenerationDraftStorageKey: draftKey,
    seedCustomerPriceRows, pdfTheme: ref({ listType: 'commercial' }), window: { location: { origin: 'https://example.test' } },
    beanListPublicationURL: (type, owner) => `/publications?scope=${owner}`,
    apiGet: url => new Promise(resolve => pending.push({ url: String(url), resolve })), error: ref(''),
    restoreDraft: () => { seeds.value = structuredClone(drafts.get(draftKey()) || []) },
    savePriceListGenerationDraftForActiveType: () => drafts.set(draftKey(), JSON.parse(JSON.stringify(seeds.value))),
  }
  const setup = new Function(...Object.keys(bindings), `
    let customerPriceSourcesRevision = 0, customerPriceSeedScope = '';
    function restorePriceListGenerationDraftForActiveType() {
      customerPriceSeedScope = priceListGenerationDraftStorageKey(); restoreDraft();
    }
    ${sourceFlow}
    return { loadCustomerPriceSources, restorePriceListGenerationDraftForActiveType };
  `)
  const handlers = scope.run(() => setup(...Object.values(bindings)))
  const output = computed(() => applyCustomerPriceRows(generated.value, seeds.value, {}, customerID.value))
  return { ...handlers, customerID, tableKey, seeds, drafts, pending, draftKey, output }
}

test('public quote response remains usable when named customer draft initializes during the request', async t => {
  const h = harness(t)
  const loading = h.loadCustomerPriceSources()
  h.tableKey.value = 'default'
  h.pending[0].resolve({ rows: [] })
  h.pending[1].resolve({ rows: [official] })
  await loading
  await nextTick()
  assert.deepEqual(h.output.value.map(row => row.final_unit_price), [28, 33, 26, 24])
  assert.equal(h.output.value[0].customer_quote_missing, false)
})

test('switching named tables reuses loaded quotes and preserves the other table draft', async t => {
  const h = harness(t)
  h.tableKey.value = 'standard'
  const loading = h.loadCustomerPriceSources()
  h.pending[0].resolve({ rows: [] })
  h.pending[1].resolve({ rows: [official] })
  await loading
  await nextTick()
  h.drafts.set(h.draftKey(), [{ ...rows[0], final_unit_price: 55 }])
  h.tableKey.value = 'dropship'
  h.restorePriceListGenerationDraftForActiveType()
  await nextTick()
  assert.deepEqual(h.output.value.map(row => row.final_unit_price), [28, 33, 26, 24])
  h.tableKey.value = 'standard'
  h.restorePriceListGenerationDraftForActiveType()
  await nextTick()
  assert.deepEqual(h.output.value.map(row => row.final_unit_price), [55])
})

test('a late source response cannot populate a different customer draft', async t => {
  const h = harness(t)
  h.tableKey.value = 'default'
  const loading = h.loadCustomerPriceSources()
  h.customerID.value = 303
  await nextTick()
  h.pending[0].resolve({ rows: [{ ...official, owner_type: 'customer', owner_key: '302' }] })
  h.pending[1].resolve({ rows: [official] })
  await loading
  await nextTick()
  assert.deepEqual(h.seeds.value, [])
})
