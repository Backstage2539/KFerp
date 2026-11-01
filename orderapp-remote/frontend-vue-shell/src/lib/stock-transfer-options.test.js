import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { loadTransferInventory, transferProductOptions, transferProductSpecs } from './stock-transfer-options.js'

test('loads every page of the selected warehouse inventory and propagates errors', async () => {
  const calls = []
  const rows = await loadTransferInventory('finished_goods', async (path) => {
    const url = new URL(path, 'http://local')
    calls.push(Object.fromEntries(url.searchParams))
    return url.searchParams.get('page') === '1'
      ? { rows: [{ item_id: 7 }], has_next: true }
      : { rows: [{ item_id: 58 }], has_next: false }
  })
  assert.deepEqual(rows.map((r) => r.item_id), [7, 58])
  assert.deepEqual(calls.map((p) => [p.warehouse, p.item_type, p.page]), [['finished_goods', 'finished_product', '1'], ['finished_goods', 'finished_product', '2']])
  await assert.rejects(loadTransferInventory('finished_goods', async () => { throw Error('offline') }), /offline/)
  assert.deepEqual(await loadTransferInventory('', async () => { throw Error('unexpected') }), [])
})

test('inventory options group batches by product and restrict BOM specs to the selected product', () => {
  const rows = [
    { item_id: 7, item_name: '初晓', bom_spec_id: 19, qty_units: 3 },
    { item_id: 58, item_name: '曜石2.0', bom_spec_id: 68, qty_units: 2 },
    { item_id: 58, item_name: '曜石2.0', bom_spec_id: 69, qty_units: 4 },
    { item_id: 58, item_name: '曜石2.0', bom_spec_id: 69, qty_units: 5 },
  ]
  assert.deepEqual(transferProductOptions(rows).map(({id, name}) => [id, name]), [[7, '初晓'], [58, '曜石2.0']])
  const specs = [{ bom_spec_id: 68 }, { bom_spec_id: 69 }, { bom_spec_id: 70 }]
  assert.deepEqual(transferProductSpecs(specs, rows, 58).map(s => s.bom_spec_id), [68, 69])
  assert.deepEqual(transferProductSpecs(specs, rows, 7), [])
})

test('transfer picker uses the chosen source warehouse and clears a selection on warehouse change', () => {
  const view = readFileSync(new URL('../views/StockEntriesView.vue', import.meta.url), 'utf8')
  assert.match(view, /transferProductOptions\(/)
  assert.match(view, /loadTransferInventory\(/)
  assert.match(view, /transferProductSpecs\(/)
  assert.match(view, /@change="handleSourceWarehouseChange\(item\)"/)
  assert.match(view, /function handleSourceWarehouseChange\(item\)[\s\S]*?resetItemObject\(item\)/)
})
