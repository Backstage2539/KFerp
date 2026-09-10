import test from 'node:test'
import { normalizePriceListGenerationDraft } from './product-price-list-draft.js'
import assert from 'node:assert/strict'
import { capturePriceListDisplayOrder, applyPriceListDisplayOrder, movePriceListCategory, movePriceListProduct } from './price-list-display-order.js'

const product = id => ({ parent_product_id: id, specs: [{ id: id * 10 }] })
const category = (id, parent, ids = []) => ({ code: `c${id}`, group_id: 1, group_item_id: id, parent_group_item_id: parent, items: ids.map(product) })
const rows = [category(1, 0), category(2, 1, [10, 11]), category(3, 1, [12]), category(4, 0, [13])]
test('category arrows move siblings together with descendants and never escape their parent', () => {
  let order = movePriceListCategory(rows, {}, 'c1', 1)
  assert.deepEqual(applyPriceListDisplayOrder(rows, order).map(r => r.code), ['c4', 'c1', 'c2', 'c3'])
  order = movePriceListCategory(rows, order, 'c2', 1)
  assert.deepEqual(applyPriceListDisplayOrder(rows, order).map(r => r.code), ['c4', 'c1', 'c3', 'c2'])
  assert.deepEqual(movePriceListCategory(rows, order, 'c2', 1), order)
})
test('products stay in their category and all specs follow the product', () => {
  const order = movePriceListProduct(rows, {}, 'c2', 10, 1)
  const result = applyPriceListDisplayOrder(rows, order)
  assert.deepEqual(result[1].items.map(p => p.parent_product_id), [11, 10])
  assert.deepEqual(result[1].items[1].specs, [{ id: 100 }])
  assert.deepEqual(movePriceListProduct(rows, order, 'c2', 10, 1), order)
  assert.deepEqual(movePriceListProduct(rows, order, 'c3', 10, 1), order)
})
test('refresh appends new rows and drops absent rows without losing retained ordering; another table is independent', () => {
  const order = movePriceListProduct(rows, {}, 'c2', 10, 1)
  const refreshed = [rows[0], category(2, 1, [10, 14, 11]), rows[2], rows[3]]
  assert.deepEqual(applyPriceListDisplayOrder(refreshed, JSON.parse(JSON.stringify(order)))[1].items.map(p => p.parent_product_id), [11, 10, 14])
  assert.deepEqual(applyPriceListDisplayOrder(refreshed, {})[1].items.map(p => p.parent_product_id), [10, 14, 11])
  assert.deepEqual(applyPriceListDisplayOrder([category(2, 0, [10])], order)[0].items.map(p => p.parent_product_id), [10])
})

test('first refresh captures an older table ordering and appends new siblings and products', () => {
  const saved = capturePriceListDisplayOrder(rows, {})
  const next = [rows[0], category(5, 1), category(2, 1, [14, 10, 11]), rows[2], rows[3]]
  const result = applyPriceListDisplayOrder(next, saved)
  assert.deepEqual(result.map(r => r.code), ['c1', 'c2', 'c3', 'c5', 'c4'])
  assert.deepEqual(result[1].items.map(p => p.parent_product_id), [10, 11, 14])
})

test('optional display order survives persisted draft normalization without changing older tables', () => {
  const order = capturePriceListDisplayOrder(rows, {})
  const draft = normalizePriceListGenerationDraft({ price_list_display_order: order })
  assert.deepEqual(draft.price_list_display_order, order)
  assert.equal(normalizePriceListGenerationDraft({}).price_list_display_order, undefined)
})
