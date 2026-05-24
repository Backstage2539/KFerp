import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

import {
  buildAssignCategoryPayload,
  buildCustomerProductRuleBindingPayload,
  buildCustomerProductRuleOverridePayload,
  buildCustomerProductRuleTemplatePayload,
  buildCustomerPublicUsagePayload,
  buildCustomProductCreatePayload,
  buildProductCategoryConfigPayload,
  buildProductBasicsPayload,
  buildProductBomURL,
  buildProductCreatePayload,
  buildSkuConfigOverridePayload,
  buildSkuContextCategoryTree,
  categoryBelongsToSkuContext,
  categoryDisplayState,
  customerSkuCustomerOptions,
  filterSkuRows,
  gradientTemplateBelongsToSkuContext,
  nextSkuContextCustomerID,
  normalizedProductKind,
  paginatedSkuRows,
  greenBeanTypeLabel,
  productBelongsToSkuContext,
  productDisplayState,
  primaryCategoryOptions,
  roastedBomProductOptions,
  secondaryCategoryOptions,
  sortRowsForCustomerSkuPriority,
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
  assert.deepEqual(filterSkuRows([
    ...rows,
    { id: 5, name: '冻干速溶咖啡', product_kind: 'instant_coffee' },
  ], { productKind: 'instant_coffee' }).map((row) => row.id), [5])
})

test('instant coffee product kind is preserved in SKU payloads without roast settings', () => {
  assert.equal(normalizedProductKind({ product_kind: 'instant_coffee' }), 'instant_coffee')

  assert.deepEqual(buildProductCreatePayload({
    name: '冻干速溶咖啡',
    product_kind: 'instant_coffee',
    roast_level: '深烘',
    yield_percent: 80,
    remark: '原料为速溶咖啡',
  }), {
    name: '冻干速溶咖啡',
    product_kind: 'instant_coffee',
    remark: '原料为速溶咖啡',
  })

  assert.deepEqual(buildProductBasicsPayload({
    name: '冻干速溶咖啡',
    product_kind: 'instant_coffee',
    roast_level: '中烘',
    yield_percent: 80,
    remark: '原料为速溶咖啡',
  }, null), {
    name: '冻干速溶咖啡',
    product_kind: 'instant_coffee',
    remark: '原料为速溶咖啡',
    margin_rate_override: null,
  })

  assert.deepEqual(buildCustomProductCreatePayload(42, {
    base_product_id: 8,
    name: '客户A-速溶咖啡',
    product_kind: 'instant_coffee',
    custom_type: 'public_sku_alias',
    copy_bom: true,
    copy_price_tiers: true,
  }), {
    customer_id: 42,
    base_product_id: 8,
    name: '客户A-速溶咖啡',
    remark: '',
    product_kind: 'instant_coffee',
    custom_type: 'public_sku_alias',
    copy_bom: false,
    copy_price_tiers: true,
  })
})

test('product subtype config payload carries templates and lightweight unit rule', () => {
  assert.deepEqual(buildProductCategoryConfigPayload({
    id: 2,
    name: '冻干速溶',
    parent_id: 1,
    position: 3,
    gradient_template_id: 9,
    operation_template_id: 19,
    price_list_rule_json: '{"generator":"instant"}',
    inventory_unit: ' kg ',
    quote_unit: '盒',
    order_unit: '盒',
    unit_conversion_json: '{"盒":{"kg":0.2}}',
    integer_unit: true,
  }), {
    id: 2,
    name: '冻干速溶',
    parent_id: 1,
    position: 3,
    gradient_template_id: 9,
    operation_template_id: 19,
    price_list_rule_json: '{"generator":"instant"}',
    inventory_unit: 'kg',
    quote_unit: '盒',
    order_unit: '盒',
    unit_conversion_json: '{"盒":{"kg":0.2}}',
    integer_unit: true,
  })
})

test('SKU config override payload carries template and unit rule overrides', () => {
  assert.deepEqual(buildSkuConfigOverridePayload({
    gradient_template_id_override: 9,
    operation_template_id_override: 19,
    unit_rule_override_json: '{"order_unit":"盒","integer_unit":true}',
  }), {
    gradient_template_id_override: 9,
    operation_template_id_override: 19,
    unit_rule_override_json: '{"order_unit":"盒","integer_unit":true}',
  })
})

test('customer product rule payloads carry template items, overrides, and bindings', () => {
  assert.deepEqual(buildCustomerProductRuleTemplatePayload({
    id: 3,
    customer_id: 42,
    name: '贴牌客户模板',
    items: [{
      product_subtype_category_id: 12,
      gradient_template_id: 9,
      operation_template_id: 19,
      price_list_rule_json: '{"generator":"instant"}',
      unit_rule_json: '{"order_unit":"盒","integer_unit":true}',
    }],
  }), {
    id: 3,
    customer_id: 42,
    name: '贴牌客户模板',
    active: true,
    items: [{
      product_subtype_category_id: 12,
      gradient_template_id: 9,
      operation_template_id: 19,
      price_list_rule_json: '{"generator":"instant"}',
      unit_rule_json: '{"order_unit":"盒","integer_unit":true}',
      active: true,
    }],
  })

  assert.deepEqual(buildCustomerProductRuleOverridePayload({
    id: 4,
    customer_id: 42,
    product_subtype_category_id: 12,
    gradient_template_id: 10,
    operation_template_id: 20,
    price_list_rule_json: '',
    unit_rule_json: '{"order_unit":"箱"}',
  }), {
    id: 4,
    customer_id: 42,
    product_subtype_category_id: 12,
    gradient_template_id: 10,
    operation_template_id: 20,
    price_list_rule_json: '{}',
    unit_rule_json: '{"order_unit":"箱"}',
    active: true,
  })

  assert.deepEqual(buildCustomerProductRuleBindingPayload(42, 3), {
    customer_id: 42,
    template_id: 3,
  })
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

test('product basics payload carries customer SKU margin override', () => {
  const payload = buildProductBasicsPayload({
    id: 17,
    name: '芬纳咖啡-曲奇拼配-深烘',
    customer_id: 74,
    product_kind: 'roasted',
    roast_level: '深烘',
    yield_percent: 80,
    remark: '客户定制烘焙',
  }, 0.33)

  assert.equal(payload.product_kind, 'roasted')
  assert.equal(payload.roast_level, '深烘')
  assert.equal(payload.yield_rate, 0.8)
  assert.equal(payload.margin_rate_override, 0.33)
  assert.equal(payload.remark, '客户定制烘焙')
})

test('product basics payload carries editable SKU name', () => {
  const payload = buildProductBasicsPayload({
    id: 17,
    name: '  芬纳定制-红酒日晒-中深烘  ',
    customer_id: 74,
    product_kind: 'roasted',
    roast_level: '中深烘',
    yield_percent: 81.5,
    remark: '客户定制',
  }, null)

  assert.equal(payload.name, '芬纳定制-红酒日晒-中深烘')
})

test('product BOM URL carries SKU focus filter for BOM maintenance jumps', () => {
  const url = buildProductBomURL('https://erp.test/vue-shell?view=productSettings&workspace=customer&customer_id=74', { id: 88 })
  assert.equal(url.searchParams.get('view'), 'bom')
  assert.equal(url.searchParams.get('product_id'), '88')
  assert.equal(url.searchParams.get('bom_filter_product_id'), '88')
  assert.equal(url.searchParams.get('workspace'), 'customer')
  assert.equal(url.searchParams.get('customer_id'), '74')
})

test('customer SKU rows sort customer-owned products before frequent public products', () => {
  const sorted = sortRowsForCustomerSkuPriority([
    { id: 1, name: '公共低频', customer_id: 0, order_usage_count: 1 },
    { id: 2, name: '客户低频', customer_id: 74, order_usage_count: 1 },
    { id: 3, name: '公共高频', customer_id: 0, order_usage_count: 10 },
    { id: 4, name: '客户高频', customer_id: 74, order_usage_count: 8 },
  ], 74)

  assert.deepEqual(sorted.map((row) => row.id), [4, 2, 3, 1])
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
    base_product_id: 0,
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

test('customer custom roast SKU payload does not carry base product or copy flags', () => {
  assert.deepEqual(buildCustomProductCreatePayload(42, {
    base_product_id: '8',
    name: '客户A-专属深烘',
    product_kind: 'roasted',
    roast_level: '深烘',
    custom_type: 'custom_roast',
    copy_bom: true,
    copy_price_tiers: true,
  }), {
    customer_id: 42,
    base_product_id: 0,
    name: '客户A-专属深烘',
    remark: '',
    product_kind: 'roasted',
    roast_level: '深烘',
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

test('roasted BOM options exclude duplicate public SKU aliases and other customers', () => {
  const candidates = [
    { id: 21, name: '初晓', product_kind: 'roasted', customer_id: 0, visibility: 'public' },
    { id: 386, name: '初晓', product_kind: 'roasted', customer_id: 149, visibility: 'customer_only', custom_type: 'public_sku_alias', base_product_id: 21 },
    { id: 420, name: '初晓-客户拼配', product_kind: 'roasted', customer_id: 149, visibility: 'customer_only', custom_type: 'custom_blend', base_product_id: 21 },
    { id: 421, name: '初晓-别的客户', product_kind: 'roasted', customer_id: 150, visibility: 'customer_only', custom_type: 'custom_blend', base_product_id: 21 },
    { id: 422, name: '初晓生豆', product_kind: 'green_bean', customer_id: 149 },
  ]

  assert.deepEqual(roastedBomProductOptions(candidates).map((row) => row.id), [21])
  assert.deepEqual(roastedBomProductOptions(candidates, { customerID: 149 }).map((row) => row.id), [21, 420])
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
    use_public_gradient_templates: true,
  }), {
    customer_id: 42,
    use_public_sku: true,
    use_public_categories: false,
    use_public_gradient_templates: true,
  })

  assert.deepEqual(buildCustomerPublicUsagePayload('7', {
    usePublicSku: false,
    usePublicCategories: true,
    usePublicGradientTemplates: false,
  }), {
    customer_id: 7,
    use_public_sku: false,
    use_public_categories: true,
    use_public_gradient_templates: false,
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

test('customer SKU context prefers derived categories over public templates', () => {
  const publicPrimary = { id: 1, name: '咖啡豆', level: 1, customer_id: 0, source_category_id: 0, template_state: 'public_template' }
  const derivedPrimary = { id: 101, name: '咖啡豆', level: 1, customer_id: 42, source_category_id: 1, template_state: 'derived_from_public' }
  const publicSecondary = { id: 17, parent_id: 1, name: '客户定制', level: 2, customer_id: 0, source_category_id: 0, template_state: 'public_template' }
  const derivedSecondary = { id: 117, parent_id: 101, name: '客户定制', level: 2, customer_id: 42, source_category_id: 17, template_state: 'derived_from_public' }
  const context = {
    customerID: 42,
    usePublicCategories: true,
    customerCategories: [derivedPrimary, derivedSecondary],
  }

  assert.equal(categoryBelongsToSkuContext(publicPrimary, context), false)
  assert.equal(categoryBelongsToSkuContext(publicSecondary, context), false)
  assert.equal(categoryBelongsToSkuContext(derivedPrimary, context), true)
  assert.equal(categoryBelongsToSkuContext(derivedSecondary, context), true)
  assert.equal(categoryDisplayState(publicSecondary, context).label, '公共模板')
  assert.equal(categoryDisplayState(derivedSecondary, context).label, '来自公共模板')
})

test('customer category tree keeps public sibling categories after deriving one public secondary category', () => {
  const publicPrimary = { id: 1, name: '咖啡豆', level: 1, customer_id: 0, products: [], children: [] }
  const publicSingle = {
    id: 11,
    parent_id: 1,
    name: '单品豆',
    level: 2,
    customer_id: 0,
    products: [{ id: 101, name: '花魁', customer_id: 0, product_category_id: 11 }],
    children: [],
  }
  const publicBlend = {
    id: 12,
    parent_id: 1,
    name: '意式拼配',
    level: 2,
    customer_id: 0,
    products: [{ id: 102, name: '暖阳拼配', customer_id: 0, product_category_id: 12 }],
    children: [],
  }
  publicPrimary.children = [publicSingle, publicBlend]
  const derivedPrimary = {
    id: 201,
    source_category_id: 1,
    name: '咖啡豆',
    level: 1,
    customer_id: 42,
    products: [],
    children: [{
      id: 212,
      parent_id: 201,
      source_category_id: 12,
      name: '意式拼配',
      level: 2,
      customer_id: 42,
      products: [],
      children: [],
    }],
  }

  const tree = buildSkuContextCategoryTree([publicPrimary, derivedPrimary], {
    customerID: 42,
    usePublicCategories: true,
    usePublicSkuInCategoryTree: true,
    customerProducts: [],
  })

  assert.equal(tree.length, 1)
  assert.equal(tree[0].id, 201)
  assert.deepEqual(tree[0].children.map((row) => row.name), ['单品豆', '意式拼配'])
  assert.deepEqual(tree[0].children.map((row) => row.products.map((product) => product.name)), [['花魁'], ['暖阳拼配']])
  assert.deepEqual(tree[0].children[1].products[0], {
    id: 102,
    name: '暖阳拼配',
    customer_id: 0,
    product_category_id: 12,
    number: 1,
    primary_name: '咖啡豆',
    secondary_name: '意式拼配',
  })
})

test('customer category tree shows public SKU references when public categories are enabled', () => {
  const publicPrimary = {
    id: 1,
    name: '咖啡豆',
    level: 1,
    customer_id: 0,
    products: [],
    children: [{
      id: 11,
      parent_id: 1,
      name: '单品豆',
      level: 2,
      customer_id: 0,
      products: [{ id: 101, name: '花魁', customer_id: 0, product_category_id: 11 }],
      children: [],
    }],
  }

  const tree = buildSkuContextCategoryTree([publicPrimary], {
    customerID: 42,
    usePublicCategories: true,
    usePublicSkuInCategoryTree: true,
    usePublicSku: false,
    customerProducts: [],
  })

  assert.deepEqual(tree.map((row) => row.name), ['咖啡豆'])
  assert.deepEqual(tree[0].children[0].products.map((row) => row.name), ['花魁'])
})

test('customer category tree keeps empty owned categories that share a public category name', () => {
  const publicPrimary = {
    id: 1,
    name: '咖啡豆',
    level: 1,
    customer_id: 0,
    products: [],
    children: [],
  }
  const customerPrimary = {
    id: 134,
    name: '咖啡豆',
    level: 1,
    customer_id: 74,
    source_category_id: 0,
    template_state: 'customer_owned',
    products: [],
    children: [],
  }

  const tree = buildSkuContextCategoryTree([publicPrimary, customerPrimary], {
    customerID: 74,
    usePublicCategories: false,
    publicCategories: [publicPrimary],
    publicProducts: [],
    customerCategories: [customerPrimary],
    customerProducts: [],
  })

  assert.equal(tree.length, 1)
  assert.equal(tree[0].id, 134)
  assert.equal(tree[0].name, '咖啡豆')
})

test('customer category tree keeps owned categories with customer SKUs when public categories are toggled', () => {
  const publicPrimary = {
    id: 1,
    name: '咖啡豆',
    level: 1,
    customer_id: 0,
    products: [],
    children: [{
      id: 11,
      parent_id: 1,
      name: '定制咖啡熟豆',
      level: 2,
      customer_id: 0,
      products: [{ id: 101, name: '公共定制熟豆', customer_id: 0, custom_type: 'standard' }],
      children: [],
    }],
  }
  const customerPrimary = {
    id: 139,
    name: '咖啡豆',
    level: 1,
    customer_id: 74,
    source_category_id: 0,
    template_state: 'customer_owned',
    products: [],
    children: [{
      id: 140,
      parent_id: 139,
      name: '定制咖啡熟豆',
      level: 2,
      customer_id: 74,
      source_category_id: 0,
      template_state: 'customer_owned',
      products: [{ id: 425, name: '芬纳定制-红酒日晒-中深烘', customer_id: 74, custom_type: 'custom_roast' }],
      children: [],
    }],
  }

  for (const usePublicCategories of [true, false]) {
    const tree = buildSkuContextCategoryTree([publicPrimary, customerPrimary], {
      customerID: 74,
      usePublicCategories,
      usePublicSkuInCategoryTree: usePublicCategories,
      publicCategories: [publicPrimary, publicPrimary.children[0]],
      publicProducts: publicPrimary.children[0].products,
      customerCategories: [customerPrimary, customerPrimary.children[0]],
      customerProducts: customerPrimary.children[0].products,
    })

    const ownedPrimary = tree.find((row) => Number(row.id) === 139)
    assert.ok(ownedPrimary, `owned primary should stay visible when usePublicCategories=${usePublicCategories}`)
    assert.deepEqual(ownedPrimary.children.map((row) => row.name), ['定制咖啡熟豆'])
    assert.deepEqual(ownedPrimary.children[0].products.map((row) => row.name), ['芬纳定制-红酒日晒-中深烘'])
  }
})

test('factory workspace forces SKU settings context back to public SKU', () => {
  assert.equal(nextSkuContextCustomerID(74, { workspaceMode: 'factory', customerContextID: 74 }), 0)
  assert.equal(nextSkuContextCustomerID(0, { workspaceMode: 'factory', customerContextID: 0 }), 0)
  assert.equal(nextSkuContextCustomerID(0, { workspaceMode: 'customer', customerContextID: 74 }), 74)
  assert.equal(nextSkuContextCustomerID(149, { workspaceMode: 'customer', customerContextID: 74 }), 74)
})

test('customer SKU context labels public and derived product ownership', () => {
  const publicProduct = { id: 21, name: '初晓', customer_id: 0, visibility: 'public' }
  const derivedProduct = { id: 421, name: '岩师傅初晓', customer_id: 42, base_product_id: 21, visibility: 'customer_only', custom_type: 'public_sku_alias' }
  const context = {
    customerID: 42,
    usePublicSku: true,
    customerProducts: [derivedProduct],
  }

  assert.equal(productDisplayState(publicProduct, context).label, '公共模板')
  assert.equal(productDisplayState(derivedProduct, context).label, '来自公共 SKU')
})

test('customer context filters gradient templates by ownership and public template switch', () => {
  const publicTemplate = { id: 2, name: '正常磅价模板', customer_id: 0, template_state: 'public_template' }
  const customerTemplate = { id: 102, name: '岩师傅 - 正常磅价模板', customer_id: 42, source_template_id: 2, template_state: 'derived_from_public' }

  assert.equal(gradientTemplateBelongsToSkuContext(publicTemplate, { customerID: 42, usePublicGradientTemplates: false }), false)
  assert.equal(gradientTemplateBelongsToSkuContext(publicTemplate, { customerID: 42, usePublicGradientTemplates: true, customerTemplates: [customerTemplate] }), false)
  assert.equal(gradientTemplateBelongsToSkuContext(customerTemplate, { customerID: 42, usePublicGradientTemplates: false }), true)
})

test('SKU settings exposes an explicit copy action for public gradient templates', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  assert.match(source, /复制为客户模板/)
  assert.match(source, /deriveGradientTemplateForCustomer/)
  assert.match(source, /customer-gradient-templates\/derive/)
})

test('SKU settings labels category levels as product type and subtype', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  assert.match(source, /产品类型/)
  assert.match(source, /产品子类型/)
  assert.doesNotMatch(source, /一级分类/)
  assert.doesNotMatch(source, /二级分类/)
})

test('SKU settings exposes customer product rule template operations', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  for (const expected of [
    '客户产品规则',
    'customerProductRuleTemplates',
    'customerProductRuleOverrides',
    'saveCustomerProductRuleTemplate',
    'saveCustomerProductRuleOverride',
    'bindCustomerProductRuleTemplate',
    '/api/product-settings/customer-rule-templates',
    '/api/product-settings/customer-rule-overrides',
    '/api/product-settings/customers/${customerID}/rule-template',
  ]) {
    assert.ok(source.includes(expected), `missing customer product rule UI behavior: ${expected}`)
  }
})

test('SKU settings renders the customer-only SKU form as a full-width workspace', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  assert.match(source, /\.custom-product-panel\s*\{\s*grid-column:\s*1\s*\/\s*-1;\s*\}/)
  assert.match(source, /\.custom-product-form\s*\{\s*display:\s*grid;\s*grid-template-columns:\s*repeat\(4,\s*minmax\(160px,\s*1fr\)\);/)
  assert.match(source, /@media\s*\(max-width:\s*1100px\)\s*\{[^}]*\.custom-product-form\s*\{\s*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/s)
})

test('assign category payload carries customer context for public template derivation', () => {
  assert.deepEqual(buildAssignCategoryPayload({
    product: { id: 421, customer_id: 42 },
    category: { id: 17, customer_id: 0 },
    customerID: 42,
    position: 3,
  }), {
    category_id: 17,
    customer_id: 42,
    position: 3,
    derive_public_category: true,
    derive_public_product: false,
  })
  assert.deepEqual(buildAssignCategoryPayload({
    product: { id: 21, customer_id: 0 },
    category: { id: 117, customer_id: 42 },
    customerID: 42,
    position: 1,
  }), {
    category_id: 117,
    customer_id: 42,
    position: 1,
    derive_public_category: false,
    derive_public_product: true,
  })
})
