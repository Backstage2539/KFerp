import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildProductBasicsPayload,
  buildProductCreatePayload,
  filterSkuRows,
  greenBeanTypeLabel,
  primaryCategoryOptions,
  roastedBomProductOptions,
  secondaryCategoryOptions,
} from './product-settings.js'

const rows = [
  { id: 1, name: '乌拉嘎 熟豆', product_kind: 'roasted', primary_name: '咖啡豆', secondary_name: '单品豆' },
  { id: 2, name: '埃塞瑰夏 生豆', product_kind: 'green_bean', primary_name: '生豆', secondary_name: '单品生豆', green_bean_type: 'single_origin' },
  { id: 3, name: '拼配生豆 A', product_kind: 'green_bean', primary_name: '生豆', secondary_name: '拼配生豆', green_bean_type: 'blend' },
]

test('filterSkuRows supports product kind, name, primary category, and secondary category filters', () => {
  assert.deepEqual(filterSkuRows(rows, { productKind: 'green_bean' }).map((row) => row.id), [2, 3])
  assert.deepEqual(filterSkuRows(rows, { query: '瑰夏' }).map((row) => row.id), [2])
  assert.deepEqual(filterSkuRows(rows, { primaryCategory: '生豆', secondaryCategory: '拼配生豆' }).map((row) => row.id), [3])
})

test('category filter options are derived from current SKU rows', () => {
  assert.deepEqual(primaryCategoryOptions(rows), ['咖啡豆', '生豆'])
  assert.deepEqual(secondaryCategoryOptions(rows, '生豆'), ['单品生豆', '拼配生豆'])
})

test('product create payload does not carry default sale price or direct green bean tiers', () => {
  const roasted = buildProductCreatePayload({ name: '暖阳拼配', product_kind: 'roasted', roast_level: '中烘', yield_percent: 82 })
  assert.deepEqual(roasted, {
    name: '暖阳拼配',
    product_kind: 'roasted',
    roast_level: '中烘',
    yield_rate: 0.82,
  })

  const green = buildProductCreatePayload({
    name: '巴拿马生豆',
    product_kind: 'green_bean',
    green_bean_type: 'blend',
    green_bean_bom_product_id: 7,
    default_price: 188,
  })
  assert.deepEqual(green, {
    name: '巴拿马生豆',
    product_kind: 'green_bean',
    green_bean_type: 'blend',
    green_bean_bom_product_id: 7,
  })
})

test('product basics payload preserves green bean type and BOM binding without direct prices', () => {
  const payload = buildProductBasicsPayload({
    id: 9,
    product_kind: 'green_bean',
    green_bean_type: 'single_origin',
    green_bean_bom_product_id: 7,
    default_price: 188,
    yield_percent: 80,
  }, null)

  assert.deepEqual(payload, {
    product_kind: 'green_bean',
    green_bean_type: 'single_origin',
    green_bean_bom_product_id: 7,
    margin_rate_override: null,
  })
})

test('green bean labels and BOM product options stay fused with existing product model', () => {
  assert.equal(greenBeanTypeLabel('blend'), '拼配')
  assert.equal(greenBeanTypeLabel('single_origin'), '单品')
  assert.deepEqual(roastedBomProductOptions([
    ...rows,
    { id: 4, name: '历史缺形态 SKU', product_kind: '' },
    { id: 5, name: '异常缺形态生豆', product_kind: '', green_bean_bom_product_id: 1 },
  ]).map((row) => row.id), [1])
})
