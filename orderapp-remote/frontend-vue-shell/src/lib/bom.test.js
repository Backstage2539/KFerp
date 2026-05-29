import test from 'node:test'
import assert from 'node:assert/strict'
import {
  bomContextCustomerIDs,
  bomSourceLabel,
  filterBomRowsByProductFocus,
  filterBomContextProducts,
  isBomProductCandidate,
  sortBomContextProducts,
} from './bom.js'

test('BOM context shows public and current-customer SKUs while hiding other customers and green beans', () => {
  const rows = [
    { id: 1, name: '岩师傅熟豆', customer_id: 152, product_kind: 'roasted_bean' },
    { id: 2, name: '兰卡拼配生豆', customer_id: 152, product_kind: 'green_bean', green_bean_bom_product_id: 1 },
    { id: 3, name: '岩师傅挂耳', customer_id: 152, product_kind: 'drip_bag' },
    { id: 4, name: '公共熟豆', customer_id: 0, product_kind: 'roasted_bean' },
    { id: 5, name: '其他客户熟豆', customer_id: 153, product_kind: 'roasted_bean' },
  ]

  assert.equal(isBomProductCandidate(rows[1]), false)
  assert.deepEqual(filterBomContextProducts(rows, 152).map((row) => row.id), [3, 1, 4])
  assert.deepEqual(filterBomContextProducts(rows, 0).map((row) => row.id), [4])
})

test('BOM context sorts customer SKUs first and frequent order products before lower usage rows', () => {
  const rows = [
    { id: 1, name: '公共低频', customer_id: 0, product_kind: 'roasted_bean', order_usage_count: 1 },
    { id: 2, name: '客户低频', customer_id: 152, product_kind: 'roasted_bean', order_usage_count: 1 },
    { id: 3, name: '公共高频', customer_id: 0, product_kind: 'roasted_bean', order_usage_count: 9 },
    { id: 4, name: '客户高频', customer_id: 152, product_kind: 'drip_bag', order_usage_count: 6 },
  ]

  assert.deepEqual(sortBomContextProducts(rows, 152).map((row) => row.id), [4, 2, 3, 1])
  assert.deepEqual(filterBomContextProducts(rows, 152).map((row) => row.id), [4, 2, 3, 1])
})

test('BOM rows can be focused to the SKU product from settings navigation', () => {
  const rows = [
    { product_id: 10, product: '目标 SKU' },
    { product_id: 11, product: '同客户其他 SKU' },
    { product_id: 12, product: '公共 SKU' },
  ]

  assert.deepEqual(filterBomRowsByProductFocus(rows, 10).map((row) => row.product_id), [10])
  assert.deepEqual(filterBomRowsByProductFocus(rows, 0).map((row) => row.product_id), [10, 11, 12])
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

test('BOM source label preserves source SKU code and version snapshots', () => {
  assert.equal(bomSourceLabel({
    bom_source_type: 'inherit_current',
    source_product_code: 'SKU-21',
    source_product_name: 'K001 精品意式拼配',
    source_bom_version_no: 'V003',
  }), '继承：SKU-21 K001 精品意式拼配 / BOM V003')

  assert.equal(bomSourceLabel({
    bom_source_type: 'derived_owned',
    source_product_code: 'SKU-21',
    source_product_name: 'K001 精品意式拼配',
    source_bom_version_no: 'V003',
  }), '自有 BOM，派生自：SKU-21 K001 精品意式拼配 / BOM V003')
})

test('BOM view exposes read-only inherited BOM and explicit derive action', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')

  assert.match(source, /BOM来源/)
  assert.match(source, /派生自有 BOM/)
  assert.match(source, /deriveOwnedBom/)
  assert.match(source, /canEditCurrentBomProduct/)
})
