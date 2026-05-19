import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildCustomerPublicUsagePayload,
  buildCustomProductCreatePayload,
  buildProductBasicsPayload,
  buildProductCreatePayload,
  categoryBelongsToSkuContext,
  customerSkuCustomerOptions,
  filterSkuRows,
  paginatedSkuRows,
  greenBeanTypeLabel,
  productBelongsToSkuContext,
  primaryCategoryOptions,
  roastedBomProductOptions,
  secondaryCategoryOptions,
  skuTypeLabel,
  skuTypeOptions,
} from './product-settings.js'

const rows = [
  { id: 1, name: '乌拉嘎 熟豆', product_kind: 'roasted', primary_name: '咖啡豆', secondary_name: '单品豆', custom_type: '', remark: '常规 SKU' },
  { id: 2, name: '埃塞瑰夏 生豆', product_kind: 'green_bean', primary_name: '生豆', secondary_name: '单品生豆', green_bean_type: 'single_origin', custom_type: 'public_sku_alias', remark: '客户改名' },
  { id: 3, name: '拼配生豆 A', product_kind: 'green_bean', primary_name: '生豆', secondary_name: '拼配生豆', green_bean_type: 'blend', custom_type: 'custom_blend', remark: '特殊拼配说明' },
]

test('filterSkuRows supports product kind, name, primary category, and secondary category filters', () => {
  assert.deepEqual(filterSkuRows(rows, { productKind: 'green_bean' }).map((row) => row.id), [2, 3])
  assert.deepEqual(filterSkuRows(rows, { query: '瑰夏' }).map((row) => row.id), [2])
  assert.deepEqual(filterSkuRows(rows, { primaryCategory: '生豆', secondaryCategory: '拼配生豆' }).map((row) => row.id), [3])
  assert.deepEqual(filterSkuRows([
    ...rows,
    { id: 4, name: '耶加雪菲挂耳', product_kind: 'drip_bag' },
  ], { productKind: 'drip_bag' }).map((row) => row.id), [4])
})

test('filterSkuRows supports SKU type options and query searches type labels', () => {
  assert.deepEqual(skuTypeOptions.map((option) => option.value), ['all', 'standard', 'public_sku_alias', 'custom_roast', 'custom_blend'])
  assert.equal(skuTypeLabel('custom_roast'), '定制烘焙')
  assert.deepEqual(filterSkuRows(rows, { customType: 'standard' }).map((row) => row.id), [1])
  assert.deepEqual(filterSkuRows(rows, { customType: 'public_sku_alias' }).map((row) => row.id), [2])
  assert.deepEqual(filterSkuRows(rows, { customType: 'custom_blend' }).map((row) => row.id), [3])
  assert.deepEqual(filterSkuRows(rows, { query: '定制拼配' }).map((row) => row.id), [3])
  assert.deepEqual(filterSkuRows(rows, { query: '特殊拼配' }).map((row) => row.id), [3])
})

test('paginatedSkuRows filters all SKU rows before slicing the current page', () => {
  const manyRows = [
    ...Array.from({ length: 12 }, (_, index) => ({
      id: index + 1,
      name: `熟豆 ${index + 1}`,
      product_kind: 'roasted',
    })),
    { id: 13, name: '后页生豆 A', product_kind: 'green_bean' },
    { id: 14, name: '后页生豆 B', product_kind: 'green_bean' },
  ]

  assert.deepEqual(
    paginatedSkuRows(manyRows, { productKind: 'green_bean' }, { page: 1, pageSize: 10 }).map((row) => row.id),
    [13, 14],
  )
})

test('category filter options are derived from current SKU rows', () => {
  assert.deepEqual(primaryCategoryOptions(rows), ['咖啡豆', '生豆'])
  assert.deepEqual(secondaryCategoryOptions(rows, '生豆'), ['单品生豆', '拼配生豆'])
})

test('product create payload carries SKU remark without direct green bean prices', () => {
  const roasted = buildProductCreatePayload({ name: '暖阳拼配', product_kind: 'roasted', roast_level: '中烘', yield_percent: 82, remark: '奶咖主推' })
  assert.deepEqual(roasted, {
    name: '暖阳拼配',
    product_kind: 'roasted',
    remark: '奶咖主推',
    roast_level: '中烘',
    yield_rate: 0.82,
  })

  const green = buildProductCreatePayload({
    name: '巴拿马生豆',
    product_kind: 'green_bean',
    green_bean_type: 'blend',
    green_bean_bom_product_id: 7,
    default_price: 188,
    remark: '新季生豆',
  })
  assert.deepEqual(green, {
    name: '巴拿马生豆',
    product_kind: 'green_bean',
    remark: '新季生豆',
    green_bean_type: 'blend',
    green_bean_bom_product_id: 7,
  })
})

test('product basics payload preserves remark, green bean type, and BOM binding without direct prices', () => {
  const payload = buildProductBasicsPayload({
    id: 9,
    product_kind: 'green_bean',
    green_bean_type: 'single_origin',
    green_bean_bom_product_id: 7,
    default_price: 188,
    yield_percent: 80,
    remark: '仅作生豆销售',
  }, null)

  assert.deepEqual(payload, {
    product_kind: 'green_bean',
    remark: '仅作生豆销售',
    green_bean_type: 'single_origin',
    green_bean_bom_product_id: 7,
    margin_rate_override: null,
  })
})

test('customer custom SKU payload supports green bean and drip bag product settings', () => {
  assert.deepEqual(buildCustomProductCreatePayload(42, {
    name: '客户A-巴拿马生豆',
    remark: '客户生豆',
    product_kind: 'green_bean',
    green_bean_type: 'blend',
    green_bean_bom_product_id: 9,
    custom_type: 'public_sku_alias',
    copy_bom: true,
    copy_price_tiers: true,
    roast_level: '中烘',
  }), {
    customer_id: 42,
    base_product_id: 0,
    name: '客户A-巴拿马生豆',
    remark: '客户生豆',
    product_kind: 'green_bean',
    green_bean_type: 'blend',
    green_bean_bom_product_id: 9,
    custom_type: 'public_sku_alias',
    copy_bom: false,
    copy_price_tiers: false,
  })

  assert.deepEqual(buildCustomProductCreatePayload('42', {
    base_product_id: '8',
    name: '客户A-耶加挂耳',
    product_kind: 'drip_bag',
    drip_bag_grams: 12,
    drip_box_bag_count: 8,
    roast_level: '中深烘',
    custom_type: 'custom_roast',
  }), {
    customer_id: 42,
    base_product_id: 8,
    name: '客户A-耶加挂耳',
    remark: '',
    product_kind: 'drip_bag',
    drip_bag_grams: 12,
    drip_box_bag_count: 8,
    roast_level: '中深烘',
    custom_type: 'custom_roast',
    copy_bom: false,
    copy_price_tiers: false,
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

test('customer SKU customer options include active customers before they have copied SKUs', () => {
  assert.deepEqual(customerSkuCustomerOptions([
    { id: 9, name: 'Z 客户', active: true },
    { id: 7, name: 'A 客户', active: true },
    { id: 8, name: '停用客户', active: false },
  ]).map((row) => row.id), [7, 9])
})

test('customer SKU customer options use fulfillment customer payload rows', () => {
  assert.deepEqual(customerSkuCustomerOptions({
    rows: [{ id: 1, name: '普通客户', active: true }],
    customers: [
      { id: 9, name: '履约 Z', active: true },
      { id: 7, name: '履约 A', active: true },
      { id: 8, name: '停用履约', active: false },
    ],
  }).map((row) => row.id), [7, 9])
})

test('customer public usage payload saves SKU and category reference switches independently', () => {
  assert.deepEqual(buildCustomerPublicUsagePayload(42, {
    use_public_sku: true,
    use_public_categories: false,
  }), {
    customer_id: 42,
    use_public_sku: true,
    use_public_categories: false,
  })

  assert.deepEqual(buildCustomerPublicUsagePayload('7', {
    usePublicSku: false,
    usePublicCategories: true,
  }), {
    customer_id: 7,
    use_public_sku: false,
    use_public_categories: true,
  })
})

test('customer SKU context treats public SKU and categories as switch-controlled references', () => {
  const publicProduct = { id: 1, name: '公共拼配', customer_id: 0 }
  const customerProduct = { id: 2, name: '客户拼配', customer_id: 42 }
  const otherCustomerProduct = { id: 3, name: '其他客户拼配', customer_id: 7 }
  assert.equal(productBelongsToSkuContext(publicProduct, { customerID: 42, usePublicSku: false }), false)
  assert.equal(productBelongsToSkuContext(publicProduct, { customerID: 42, usePublicSku: true }), true)
  assert.equal(productBelongsToSkuContext(customerProduct, { customerID: 42, usePublicSku: false }), true)
  assert.equal(productBelongsToSkuContext(otherCustomerProduct, { customerID: 42, usePublicSku: true }), false)

  const publicCategory = { id: 10, name: '公共分类', customer_id: 0 }
  const customerCategory = { id: 11, name: '客户分类', customer_id: 42 }
  assert.equal(categoryBelongsToSkuContext(publicCategory, { customerID: 42, usePublicCategories: false }), false)
  assert.equal(categoryBelongsToSkuContext(publicCategory, { customerID: 42, usePublicCategories: true }), true)
  assert.equal(categoryBelongsToSkuContext(customerCategory, { customerID: 42, usePublicCategories: false }), true)
})
