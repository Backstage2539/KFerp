import test from 'node:test'
import assert from 'node:assert/strict'
import { initialStockSelection, stockSelectionPayload, stockSelectionError } from './semi-finished-stock.js'

const items = [{ key: 'material:1', name: '初晓半成品', unit: 'kg', required_qty: 60.112, batches: [
  { batch_id: 3, warehouse: 'raw_materials', available_qty: 15, selected_qty: 10 },
  { batch_id: 4, warehouse: 'wip', available_qty: 20, selected_qty: 0 },
] }]

test('selection preserves selected quantities and emits complete explicit batch choices', () => {
  const values = initialStockSelection(items)
  assert.equal(values['material:1|raw_materials|3'], 10)
  values['material:1|wip|4'] = 5
  assert.deepEqual(stockSelectionPayload(items, values), [
    { key: 'material:1', batch_id: 3, warehouse: 'raw_materials', qty: 10 },
    { key: 'material:1', batch_id: 4, warehouse: 'wip', qty: 5 },
  ])
})

test('selection rejects invalid, over-available and over-demand quantities', () => {
  const values = initialStockSelection(items)
  values['material:1|raw_materials|3'] = 16
  assert.match(stockSelectionError(items, values), /可用/)
  values['material:1|raw_materials|3'] = -1
  assert.match(stockSelectionError(items, values), /数量/)
  const smaller = [{ ...items[0], required_qty: 5 }]
  assert.match(stockSelectionError(smaller, initialStockSelection(smaller)), /需求/)
})

test('the same batch can be selected from two warehouses within their own available quantities', () => {
  const split = [{ ...items[0], batches: [items[0].batches[0], { ...items[0].batches[0], warehouse: 'wip' }] }]
  const values = initialStockSelection(split)
  assert.equal(stockSelectionError(split, values), '')
  assert.deepEqual(stockSelectionPayload(split, values), [
    { key: 'material:1', batch_id: 3, warehouse: 'raw_materials', qty: 10 },
    { key: 'material:1', batch_id: 3, warehouse: 'wip', qty: 10 },
  ])
})
