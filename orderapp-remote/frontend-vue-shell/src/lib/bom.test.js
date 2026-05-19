import test from 'node:test'
import assert from 'node:assert/strict'
import {
  bomContextCustomerIDs,
  filterBomContextProducts,
  isBomProductCandidate,
} from './bom.js'

test('BOM context filters out green bean SKUs that already bind a roasted BOM', () => {
  const rows = [
    { id: 1, name: '岩师傅熟豆', customer_id: 152, product_kind: 'roasted_bean' },
    { id: 2, name: '兰卡拼配生豆', customer_id: 152, product_kind: 'green_bean', green_bean_bom_product_id: 1 },
    { id: 3, name: '岩师傅挂耳', customer_id: 152, product_kind: 'drip_bag' },
    { id: 4, name: '公共熟豆', customer_id: 0, product_kind: 'roasted_bean' },
  ]

  assert.equal(isBomProductCandidate(rows[1]), false)
  assert.deepEqual(filterBomContextProducts(rows, 152).map((row) => row.id), [1, 3])
  assert.deepEqual(filterBomContextProducts(rows, 0).map((row) => row.id), [4])
})

test('BOM customer selector ignores customers that only have green bean SKUs', () => {
  const products = [
    { id: 1, customer_id: 9, product_kind: 'green_bean' },
    { id: 2, customer_id: 10, product_kind: 'roasted_bean' },
  ]
  const bomRows = [
    { product_id: 3, customer_id: 11, product_kind: 'green_bean' },
    { product_id: 4, customer_id: 12, product_kind: 'drip_bag' },
  ]

  assert.deepEqual([...bomContextCustomerIDs(products, bomRows)].sort((a, b) => a - b), [10, 12])
})
