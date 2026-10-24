import test from 'node:test'
import { applyCustomerPriceRows } from './customer-price-draft.js'
import { removePriceTable, createPriceTableBatch } from './price-table-batch.js'
import assert from 'node:assert/strict'
import { publicationEditorPayload, loadPublishedEditorBatch, reconcileEditorBatch, editorTableConflict, resolveEditorTableConflict } from './price-table-editor-restore.js'

function publication(id, version, specs) {
  const selections = specs.map(([parent, spec, variant, bom, bomVersion]) => ({
    parent_product_id: parent, product_id: parent, bom_spec_id: spec, bom_variant_id: variant,
    bom_id: bom, bom_version_id: bomVersion, bom_spec_authoritative: true, selection_source: 'explicit',
  }))
  return { id, version, status: 'published', owner_type: 'customer', owner_key: '471',
    release_id: 'release-' + id, table_key: 'table-' + id, table_name: version, is_default_table: true,
    config: { product_spec_selections: selections, customizers: { title: 'test' },
      price_list_display_order: { product_order: ['keep'] },
      price_list_template_selection: { defaults: { pricing_mode: 'tier_template', tier_template_id: 5 },
        group_selections: [{ level: 1, group_item_id: 881, pricing_mode: 'fixed_price' }],
        product_overrides: specs.map(([parent, spec, , , , amount]) => ({ scope: 'sku', parent_product_id: parent, sku_id: spec, fixed_unit_price: amount })) } },
    content: { price_rows: selections.map((s, i) => ({ ...s, row_key: s.bom_spec_id + ':fixed_price',
      pricing_mode: 'fixed_price', final_unit_price: specs[i][5], price_unit: '袋' })) },
  }
}

const beans = publication(151, 'V3.0.66', [
  [1,229,474,7958,402,26.5], [58,68,457,890,401,27], [48,241,406,9614,376,28.5],
  [7,19,558,883,409,31.5], [57,47,490,887,404,48.5], [70,117,959,897,530,58.5],
])
const drips = publication(152, 'V3.0.14', [
  [940,309,607,18277,423,32.8], [957,390,801,26067,512,32.8], [959,393,804,26341,515,32.47],
  [961,399,810,26704,520,35.6], [966,444,1151,29835,551,20],
])
const families = p => p.config.product_spec_selections.map(s => ({ parent_product_id: s.parent_product_id,
  default_sku_id: s.bom_spec_id, sku_options: [{ ...s, sku_id: s.bom_spec_id }] }))

test('published tables restore valid specs at current BOM versions without changing fixed or inherited quotes', () => {
  for (const p of [beans, drips]) {
    const saved = structuredClone(p)
    const current = families(p).map(f => ({ ...f, default_sku_id: 999,
      sku_options: [...f.sku_options.map(s => ({ ...s, bom_version_id: s.bom_version_id + 1, bom_variant_id: s.bom_variant_id + 1 })), { sku_id: 999, bom_spec_id: 999 }] }))
    const result = publicationEditorPayload(p, current)
    assert.ok(result.draft.product_spec_selections.every(s => !s.selection_issue && s.sku_id !== 999))
    for (const old of p.content.price_rows) {
      const selected = result.draft.product_spec_selections.find(s => s.bom_spec_id === old.bom_spec_id)
      assert.equal(selected.bom_version_id, old.bom_version_id + 1)
      assert.equal(result.draft.productOverrides['sku:' + old.bom_spec_id].fixed_unit_price, old.final_unit_price)
    }
    assert.deepEqual(result.draft.customerPriceSeedRows.map(r => r.final_unit_price), p.content.price_rows.map(r => r.final_unit_price))
    assert.deepEqual(p, saved)
  }
})

for (const [p, count] of [[beans, 6], [drips, 5]]) test(p.version + ' restores every selected specification and fixed amount from public snapshot', () => {
  const source = structuredClone(p)
  const result = publicationEditorPayload(p, families(p))
  assert.equal(result.draft.product_spec_selections.length, count)
  assert.ok(result.draft.product_spec_selections.every(s => !s.selection_issue && s.sku_id > 0))
  for (const row of p.content.price_rows) {
    assert.equal(result.draft.productOverrides['sku:' + row.bom_spec_id].fixed_unit_price, row.final_unit_price)
  }
  assert.deepEqual(result.draft.price_list_display_order, p.config.price_list_display_order)
  assert.equal(result.draft.parentSelections['881'].pricing_mode, 'fixed_price')
  assert.equal(result.draft.customerPriceCopyFrozen, false)
  assert.deepEqual(p, source)
})

test('canonical BOM overrides for two specifications of one product do not collide', () => {
  const p = publication(153, 'V3.0.15', [[940,309,607,18277,423,32.8],[940,310,608,18277,423,40]])
  p.config.price_list_template_selection.product_overrides = p.content.price_rows.map(row => ({ ...row, scope: 'sku', fixed_unit_price: row.final_unit_price }))
  const f = [{ ...families(p)[0], sku_options: families(p).flatMap(row => row.sku_options) }]
  const result = publicationEditorPayload(p, f)
  assert.equal(result.draft.productOverrides['sku:309'].fixed_unit_price, 32.8)
  assert.equal(result.draft.productOverrides['sku:310'].fixed_unit_price, 40)
  assert.equal(result.draft.productOverrides['sku:940'], undefined)
})

test('explicit empty selections remain empty; legacy missing selections can use saved rows', () => {
  const p = structuredClone(beans)
  p.config.product_spec_selections = []
  assert.deepEqual(publicationEditorPayload(p, families(beans)).draft.product_spec_selections, [])
  delete p.config.product_spec_selections
  assert.equal(publicationEditorPayload(p, families(beans)).draft.product_spec_selections.length, 6)
})

test('loader follows current published release past newer draft pages and loads complete batch', async () => {
  const second = { ...beans, id: 154, table_key: 'second', is_default_table: false }
  const calls = []
  const result = await loadPublishedEditorBatch({
    loadSummary: async page => ({ current: beans, total: 3, page_size: 1, suggested_version: 'V3.0.67',
      rows: page === 1 ? [{ id: 200, status: 'draft', release_id: 'draft' }] : [beans, second] }),
    loadDetail: async row => { calls.push(row.id); return row },
  })
  assert.deepEqual(calls, [151,154])
  assert.equal(result.tables.length, 2)
  assert.equal(result.version, 'V3.0.67')
  assert.equal(result.tables[0].source_publication.id, 151)
})

test('loader aborts late responses and propagates failures instead of initializing empty drafts', async () => {
  let current = true
  assert.equal(await loadPublishedEditorBatch({ isCurrent: () => current,
    loadSummary: async () => { current = false; return { current: beans, rows: [beans] } },
    loadDetail: async () => { throw Error('must not fetch') },
  }), undefined)
  await assert.rejects(loadPublishedEditorBatch({ loadSummary: async () => { throw Error('offline') } }), /offline/)
})

async function publishedBatch() {
  return loadPublishedEditorBatch({ loadSummary: async () => ({ current: beans, rows: [beans], total: 1 }), loadDetail: async row => row })
}

test('same baseline retains deliberate deselection and table-local fixed price edits', async () => {
  const published = await publishedBatch()
  const local = structuredClone(published)
  local.tables[0].payload = { draft: { product_spec_selections: [], productOverrides: { 'sku:68': { fixed_unit_price: 88 } } } }
  const next = reconcileEditorBatch(local, published)
  assert.deepEqual(next.tables[0].payload, local.tables[0].payload)
  assert.equal(editorTableConflict(next.tables[0], published.tables[0]), false)
})

test('legacy and stale drafts require a per-table decision, backup precedes replacement', async () => {
  const published = await publishedBatch()
  const local = structuredClone(published)
  delete local.tables[0].source_publication
  local.tables[0].payload = { draft: { product_spec_selections: [], marker: 'unsaved' } }
  local.tables.push({ key: 'other', name: 'other', payload: { marker: 'keep' } })
  assert.equal(editorTableConflict(local.tables[0], published.tables[0]), true)
  const saved = new Map()
  const next = await resolveEditorTableConflict(local, published.tables[0], 'published', { scope: 'customer471', storage: { setItem: (k,v) => saved.set(k,v) } })
  assert.equal(JSON.parse([...saved.values()][0]).payload.draft.marker, 'unsaved')
  assert.deepEqual(next.tables[1], local.tables[1])
  const kept = await resolveEditorTableConflict(local, published.tables[0], 'local')
  assert.equal(kept.tables[0].payload.draft.marker, 'unsaved')
  assert.equal(editorTableConflict(kept.tables[0], published.tables[0]), false)
  await assert.rejects(resolveEditorTableConflict(local, published.tables[0], 'published', {
    storage: { setItem: () => { throw Error('storage full') } },
  }), /storage full/)
  assert.equal(local.tables[0].payload.draft.marker, 'unsaved')
})

function backupDatabase({ existing = false } = {}) {
  const records = new Map()
  const stores = new Set(existing ? ['backups'] : [])
  let transaction
  let closed = false
  const database = {
    objectStoreNames: { contains: name => stores.has(name) },
    createObjectStore: name => { assert.equal(name, 'backups'); stores.add(name) },
    transaction(store, mode) {
      assert.equal(store, 'backups')
      assert.equal(mode, 'readwrite')
      assert.ok(stores.has(store))
      transaction = { objectStore: () => ({ put: (value, key) => records.set(key, value) }) }
      return transaction
    },
    close: () => { closed = true },
  }
  return {
    records,
    get transaction() { return transaction },
    get closed() { return closed },
    indexedDB: { open: () => {
      const request = { result: database }
      queueMicrotask(() => { if (!existing) request.onupgradeneeded(); request.onsuccess() })
      return request
    } },
  }
}

test('full localStorage backs up the complete old table in IndexedDB before loading the publication', async () => {
  const published = await publishedBatch()
  const local = structuredClone(published)
  local.tables[0].payload = { draft: { marker: 'unsaved', productOverrides: { 'sku:68': { fixed_unit_price: 88 } } } }
  local.tables.push({ key: 'other', payload: { marker: 'keep' } })
  const original = structuredClone(local)
  const backup = backupDatabase({ existing: true })
  let settled = false
  const pending = Promise.resolve().then(() => resolveEditorTableConflict(local, published.tables[0], 'published', {
    scope: 'factory:official:all:product-catalog:56',
    storage: { setItem: () => { throw new DOMException('Setting the value exceeded the quota.', 'QuotaExceededError') } },
    indexedDB: backup.indexedDB,
  }))
  pending.then(() => { settled = true }, () => { settled = true })
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(settled, false, 'must wait for a durable backup transaction')
  const [key, value] = [...backup.records][0]
  assert.ok(key.startsWith('kferp:price-table-backup:factory:official:all:product-catalog:56:'))
  assert.deepEqual(JSON.parse(value), original.tables[0])
  assert.deepEqual(local, original)
  backup.transaction.oncomplete()
  const next = await pending
  assert.deepEqual(next.tables[0], published.tables[0])
  assert.deepEqual(next.tables[1], original.tables[1])
  assert.deepEqual(local, original)
  assert.equal(backup.closed, true)
})

test('unavailable fallback storage stops replacement without exposing the native quota exception', async () => {
  const published = await publishedBatch()
  const local = structuredClone(published)
  local.tables[0].payload = { draft: { marker: 'unsaved' } }
  await assert.rejects(resolveEditorTableConflict(local, published.tables[0], 'published', {
    storage: { setItem: () => { throw new DOMException('quota', 'QuotaExceededError') } }, indexedDB: null,
  }), /草稿.*保留/)
  assert.equal(local.tables[0].payload.draft.marker, 'unsaved')
})

test('failed fallback backup leaves the entire current draft intact and gives a readable error', async () => {
  const published = await publishedBatch()
  const local = structuredClone(published)
  local.tables[0].payload = { draft: { marker: 'unsaved' } }
  const original = structuredClone(local)
  const backup = backupDatabase()
  const pending = Promise.resolve().then(() => resolveEditorTableConflict(local, published.tables[0], 'published', {
    storage: { setItem: () => { throw new DOMException('quota', 'QuotaExceededError') } },
    indexedDB: backup.indexedDB,
  }))
  const rejected = assert.rejects(pending, /草稿.*保留/)
  await new Promise(resolve => setImmediate(resolve))
  backup.transaction.error = new DOMException('quota', 'QuotaExceededError')
  backup.transaction.onabort()
  await rejected
  assert.deepEqual(local, original)
  assert.equal(backup.closed, true)
})

test('published fixed prices retain exact amounts and units while new products use editable pricing', () => {
  const { draft } = publicationEditorPayload(drips, families(drips))
  const generated = drips.content.price_rows.map(row => ({ ...row, final_unit_price: 999, price_unit: 'other' }))
  const added = { product_id: 2000, parent_product_id: 2000, bom_spec_id: 2001, bom_variant_id: 2002,
    row_key: 'new', pricing_mode: 'fixed_price', final_unit_price: 45, price_unit: '盒' }
  const rows = applyCustomerPriceRows([...generated, added], draft.customerPriceSeedRows, {}, 471,
    { configuredSources: { ...draft.customerPriceConfiguredSources, 'product:2000': true } })
  assert.deepEqual(rows.slice(0, 5).map(row => [row.final_unit_price, row.price_unit]), drips.content.price_rows.map(row => [row.final_unit_price, row.price_unit]))
  assert.equal(rows.at(-1).final_unit_price, 45)
  const edited = applyCustomerPriceRows([{ ...generated[0], final_unit_price: 50 }], draft.customerPriceSeedRows, {}, 471,
    { configuredSources: { 'product:940': true } })
  assert.equal(edited[0].final_unit_price, 50)
})

test('deleting a table in a local draft survives refreshing the published batch', async () => {
  const published = await publishedBatch()
  published.tables.push({ ...structuredClone(published.tables[0]), key: 'second' })
  const local = removePriceTable(published, 'second')
  assert.deepEqual(reconcileEditorBatch(local, published).tables.map(t => t.key), [published.tables[0].key])
})

test('unproven single legacy table is matched to its published table and requires a decision', async () => {
  const published = await publishedBatch()
  const local = structuredClone(published)
  local.tables[0].key = local.active_table_key = local.default_table_key = 'legacy-browser-key'
  delete local.tables[0].source_publication
  local.tables[0].payload = { draft: { product_spec_selections: [] } }
  const next = reconcileEditorBatch(local, published)
  assert.equal(next.tables.length, 1)
  assert.equal(next.active_table_key, published.active_table_key)
  assert.equal(editorTableConflict(next.tables[0], published.tables[0]), true)
  assert.deepEqual(next.tables[0].payload.draft.product_spec_selections, [])
  const created = createPriceTableBatch({ draft: { product_spec_selections: [] } })
  assert.equal(reconcileEditorBatch(created, published).tables.length, 2)
})

test('legacy non-named cache is retained as an unproven draft instead of silently replaced', async () => {
  const published = await publishedBatch()
  const draft = { product_spec_selections: [], productOverrides: { 'sku:229': { fixed_unit_price: 38 } } }
  const next = reconcileEditorBatch(null, published, draft)
  assert.equal(editorTableConflict(next.tables[0], published.tables[0]), true)
  assert.deepEqual(next.tables[0].payload.draft, draft)
  assert.deepEqual(reconcileEditorBatch(null, null, draft).tables[0].payload.draft, draft)
})
