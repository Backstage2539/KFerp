import test from 'node:test'
import assert from 'node:assert/strict'
import { transferSpecs, transferBatches, transferAmount } from './stock-transfer-selector.js'

const rows = [
  { item_type: 'finished_product', item_id: 7, bom_spec_id: 91, bom_variant_id: 191, spec_name: '旧 227g', inventory_unit: '袋', batch_code: 'A', qty_units: 4, available_units: 3 },
  { item_type: 'finished_product', item_id: 7, bom_spec_id: 0, spec_g: 454, batch_code: 'B', qty_g: 958, qty_units: 2, available_g: 50, available_units: 2 },
  { item_type: 'material', item_id: 8, inventory_unit: 'kg', batch_code: 'C', qty_g: 2000, available_g: 1500 },
]

test('actual stock determines historical, packaged and loose specifications', () => {
  const specs = transferSpecs(rows, 'finished_product', 7)
  assert.deepEqual(specs.map(s => s.label), ['旧 227g', '454g', '克（454g库存散装）'])
  assert.deepEqual(transferBatches(rows, 'finished_product', 7, specs[2].key).map(r => r.batch_code), ['B'])
  assert.deepEqual(transferAmount(rows[1], specs[2], 'available'), { amount: 50, unit: '克' })
})

test('material availability uses its real inventory unit', () => {
  const [spec] = transferSpecs(rows, 'material', 8)
  assert.deepEqual(transferAmount(rows[2], spec, 'available'), { amount: 1.5, unit: 'kg' })
})
