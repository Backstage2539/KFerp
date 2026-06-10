import assert from 'node:assert/strict'
import test from 'node:test'

import {
  priceListCategoryCodesForSelectedProducts,
  priceListCategoryHiddenByCollapsedAncestor,
  priceListCategoryProductIDs,
  priceListVisibleCategoryRows,
} from './product-price-list-selection.js'

const categoryRows = [
  {
    code: 'business-group-9-90',
    label: '咖啡熟豆',
    group_id: 9,
    group_item_id: 90,
    parent_group_item_id: 0,
    items: [],
  },
  {
    code: 'business-group-9-92',
    label: '意式拼配豆',
    group_id: 9,
    group_item_id: 92,
    parent_group_item_id: 90,
    items: [{ product_id: 550, name: '熟豆-红岩拼配' }],
  },
  {
    code: 'business-group-9-91',
    label: '挂耳咖啡',
    group_id: 9,
    group_item_id: 91,
    parent_group_item_id: 0,
    items: [],
  },
  {
    code: 'business-group-unclassified',
    label: '未分类',
    group_id: 9,
    group_item_id: 0,
    parent_group_item_id: 0,
    items: [],
    unclassified: true,
  },
]

test('price-list product picker hides empty categories outside the selected type but keeps ancestors', () => {
  const visibleRows = priceListVisibleCategoryRows(categoryRows)

  assert.deepEqual(visibleRows.map((row) => row.label), ['咖啡熟豆', '意式拼配豆'])
})

test('price-list parent category selection includes descendant products and preview categories', () => {
  assert.deepEqual(
    priceListCategoryProductIDs(categoryRows, 'business-group-9-90'),
    ['550'],
  )
  assert.deepEqual(
    priceListCategoryCodesForSelectedProducts(categoryRows, ['550']),
    ['business-group-9-92'],
  )
})

test('price-list collapsed parent category hides descendant category rows', () => {
  assert.equal(
    priceListCategoryHiddenByCollapsedAncestor(categoryRows, categoryRows[1], {
      'business-group-9-90': true,
    }),
    true,
  )
  assert.equal(
    priceListCategoryHiddenByCollapsedAncestor(categoryRows, categoryRows[1], {
      'business-group-9-92': true,
    }),
    false,
  )
})
