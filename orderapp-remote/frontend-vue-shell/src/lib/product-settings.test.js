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
  buildProductConfigTemplatePayload,
  buildProductUnitDefinitionPayload,
  buildProductUnitTemplatePayload,
  buildProductBasicsPayload,
  buildProductBomURL,
  buildProductCreatePayload,
  buildSkuCopyPayload,
  buildSkuCreatePayload,
  buildSkuConfigOverridePayload,
  buildSkuContextCategoryTree,
  categoryBelongsToSkuContext,
  categoryDisplayState,
  customerSkuCustomerOptions,
  filterSkuRows,
  gradientTemplateBelongsToSkuContext,
  inferProductKindFromProductTypeCategory,
  nextSkuContextCustomerID,
  normalizeVisibleSkuFilters,
  normalizedProductKind,
  paginatedSkuRows,
  priceListRuleFormFromJSON,
  priceListRuleJSONFromForm,
  greenBeanTypeLabel,
  productBelongsToSkuContext,
  productConfigTemplateBelongsToSkuContext,
  productDisplayState,
  productSubtypeCategoryOptionsForType,
  primaryCategoryOptions,
  roastedBomProductOptions,
  secondaryCategoryOptions,
  specialAttrSchemaJSONFromRows,
  specialAttrSchemaRowsFromJSON,
  specialAttrValuesFromJSON,
  specialAttrValuesJSONFromForm,
  sortRowsForCustomerSkuPriority,
  skuListRowsFromProducts,
  skuTableState,
  skuTypeLabel,
  skuTypeOptions,
  unitConversionJSONFromRows,
  unitConversionRowsFromJSON,
  unitRuleFormFromJSON,
  unitRuleJSONFromForm,
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

test('instant coffee product kind carries SKU-owned BOM yield and special KV settings', () => {
  assert.equal(normalizedProductKind({ product_kind: 'instant_coffee' }), 'instant_coffee')

  assert.deepEqual(buildProductCreatePayload({
    name: '冻干速溶咖啡',
    product_kind: 'instant_coffee',
    special_attr_values: { roast_level: '深烘' },
    yield_percent: 80,
    remark: '原料为速溶咖啡',
  }), {
    name: '冻干速溶咖啡',
    product_kind: 'instant_coffee',
    remark: '原料为速溶咖啡',
    special_attrs_json: '{"roast_level":"深烘"}',
    yield_rate: 0.8,
  })

  assert.deepEqual(buildProductBasicsPayload({
    name: '冻干速溶咖啡',
    product_kind: 'instant_coffee',
    special_attr_values: { roast_level: '中烘' },
    yield_percent: 80,
    remark: '原料为速溶咖啡',
  }, null), {
    name: '冻干速溶咖啡',
    product_kind: 'instant_coffee',
    remark: '原料为速溶咖啡',
    special_attrs_json: '{"roast_level":"中烘"}',
    yield_rate: 0.8,
    margin_rate_override: null,
  })

  assert.deepEqual(buildCustomProductCreatePayload(42, {
    base_product_id: 8,
    name: '客户A-速溶咖啡',
    product_kind: 'instant_coffee',
    special_attr_values: { roast_level: '中烘' },
    yield_percent: 100,
    custom_type: 'public_sku_alias',
    copy_bom: true,
    copy_price_tiers: true,
  }), {
    customer_id: 42,
    base_product_id: 8,
    name: '客户A-速溶咖啡',
    remark: '',
    product_kind: 'instant_coffee',
    special_attrs_json: '{"roast_level":"中烘"}',
    yield_rate: 1,
    custom_type: 'public_sku_alias',
    copy_bom: false,
    copy_price_tiers: true,
  })
})

test('product category selection infers legacy product kind only as compatibility', () => {
  assert.equal(inferProductKindFromProductTypeCategory({ name: '速溶咖啡' }), 'instant_coffee')
  assert.equal(inferProductKindFromProductTypeCategory({ name: '冻干速溶' }), 'instant_coffee')
  assert.equal(inferProductKindFromProductTypeCategory({ name: '生豆' }), 'green_bean')
  assert.equal(inferProductKindFromProductTypeCategory({ name: '挂耳' }), 'drip_bag')
  assert.equal(inferProductKindFromProductTypeCategory({ name: '意式拼配' }), 'roasted')
  assert.equal(inferProductKindFromProductTypeCategory(null), 'roasted')
})

test('unified SKU create payload is owned by current view and carries no legacy product kind fields', () => {
  const payload = buildSkuCreatePayload(42, {
    name: '客户盒装速溶',
    remark: '10g/条，10条/盒',
    product_type_category_id: 7,
    product_subtype_category_id: 17,
    product_kind: 'instant_coffee',
    custom_type: 'public_sku_alias',
    base_product_id: 99,
    copy_bom: true,
    copy_price_tiers: true,
    special_attr_values: { roast_level: '中深烘' },
  })

  assert.deepEqual(payload, {
    customer_id: 42,
    name: '客户盒装速溶',
    remark: '10g/条，10条/盒',
    product_type_category_id: 7,
    product_subtype_category_id: 17,
    special_attrs_json: '{"roast_level":"中深烘"}',
    active: true,
  })
  assert.equal(Object.hasOwn(payload, 'product_kind'), false)
  assert.equal(Object.hasOwn(payload, 'custom_type'), false)
  assert.equal(Object.hasOwn(payload, 'base_product_id'), false)
})

test('SKU copy payload supports all selected SKU ids and same-name overwrite flow', () => {
  assert.deepEqual(buildSkuCopyPayload({
    target_customer_id: 42,
    source_customer_id: 0,
    source_sku_ids: [7, '8', 0, 7],
  }), {
    target_customer_id: 42,
    source_customer_id: 0,
    source_sku_ids: [7, 8],
  })
})

test('product subtype options are derived from the selected product type only', () => {
  const tree = [{
    id: 1,
    name: '速溶咖啡',
    level: 1,
    children: [
      { id: 11, parent_id: 1, name: '冻干速溶', level: 2 },
      { id: 12, parent_id: 1, name: '喷雾干燥', level: 2 },
    ],
  }, {
    id: 2,
    name: '挂耳',
    level: 1,
    children: [{ id: 21, parent_id: 2, name: '盒装挂耳', level: 2 }],
  }]

  assert.deepEqual(productSubtypeCategoryOptionsForType(tree, 1).map((category) => category.id), [11, 12])
  assert.deepEqual(productSubtypeCategoryOptionsForType(tree, 2).map((category) => category.id), [21])
  assert.deepEqual(productSubtypeCategoryOptionsForType(tree, 0), [])
})

test('product subtype config payload carries templates and lightweight unit rule', () => {
  assert.deepEqual(buildProductCategoryConfigPayload({
    id: 2,
    customer_id: 42,
    name: '冻干速溶',
    parent_id: 1,
    position: 3,
    product_config_template_id: 301,
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
    customer_id: 42,
    name: '冻干速溶',
    parent_id: 1,
    position: 3,
    product_config_template_id: 301,
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

test('structured price list rule form stores generation rules without price table inclusion flags', () => {
  const form = priceListRuleFormFromJSON('{"generator":"instant","include_in_price_list":false,"pricing_mode":"fixed_unit_price","display_unit":"盒","fixed_unit_price":15,"rounding":"yuan","tax_included":true}')

  assert.equal(form.price_rule_pricing_mode, 'fixed_unit_price')
  assert.equal(form.price_rule_fixed_unit_price, 15)
  assert.equal(form.price_rule_rounding, 'yuan')
  assert.equal(form.price_rule_tax_included, true)
  assert.deepEqual(form.price_rule_extra, { generator: 'instant' })
  assert.equal(priceListRuleJSONFromForm(form), '{"generator":"instant","pricing_mode":"fixed_unit_price","fixed_unit_price":15,"rounding":"yuan","tax_included":true}')
  assert.equal(priceListRuleJSONFromForm({}), '{"pricing_mode":"inherit_gradient_template","rounding":"none","tax_included":false}')

  const costPlusForm = priceListRuleFormFromJSON('{"pricing_mode":"cost_plus","cost_plus_rate":0.28}')
  assert.equal(costPlusForm.price_rule_cost_plus_percent, 28)
  assert.equal(priceListRuleJSONFromForm(costPlusForm), '{"pricing_mode":"cost_plus","cost_plus_rate":0.28,"rounding":"none","tax_included":false}')
})

test('product config template payload carries template rules and unit settings as one reusable object', () => {
  assert.deepEqual(buildProductConfigTemplatePayload({
    id: 301,
    customer_id: 42,
    name: '客户盒装商品配置',
    gradient_template_id: 8,
    operation_template_id: 9,
    unit_template_id: 12,
    price_rule_pricing_mode: 'fixed_unit_price',
    price_rule_fixed_unit_price: 15,
    price_rule_rounding: 'yuan',
    price_rule_tax_included: true,
    special_attrs_schema_rows: [{
      key: 'roast_level',
      label: '烘焙度',
      value_type: 'select',
      options_text: '浅烘\n中烘\n中深烘\n深烘',
      required: true,
      show_in_price_list: true,
    }],
  }), {
    id: 301,
    customer_id: 42,
    name: '客户盒装商品配置',
    gradient_template_id: 8,
    operation_template_id: 9,
    unit_template_id: 12,
    price_list_rule_json: '{"pricing_mode":"fixed_unit_price","fixed_unit_price":15,"rounding":"yuan","tax_included":true}',
    special_attrs_schema_json: '[{"key":"roast_level","label":"烘焙度","value_type":"select","options":["浅烘","中烘","中深烘","深烘"],"required":true,"show_in_price_list":true,"position":1}]',
    active: true,
  })
})

test('special KV schema and SKU values round trip through structured helpers', () => {
  const schemaJSON = specialAttrSchemaJSONFromRows([
    { key: ' roast_level ', label: ' 烘焙度 ', value_type: 'select', options_text: '浅烘，中烘\n中深烘', required: true, show_in_price_list: true },
    { key: ' caffeine ', label: '咖啡因', value_type: 'text', options_text: '', required: false, show_in_price_list: false },
  ])

  assert.equal(schemaJSON, '[{"key":"roast_level","label":"烘焙度","value_type":"select","options":["浅烘","中烘","中深烘"],"required":true,"show_in_price_list":true,"position":1},{"key":"caffeine","label":"咖啡因","value_type":"text","options":[],"required":false,"show_in_price_list":false,"position":2}]')
  assert.deepEqual(specialAttrSchemaRowsFromJSON(schemaJSON).map((row) => ({
    key: row.key,
    label: row.label,
    value_type: row.value_type,
    options_text: row.options_text,
    required: row.required,
    show_in_price_list: row.show_in_price_list,
    position: row.position,
  })), [{
    key: 'roast_level',
    label: '烘焙度',
    value_type: 'select',
    options_text: '浅烘\n中烘\n中深烘',
    required: true,
    show_in_price_list: true,
    position: 1,
  }, {
    key: 'caffeine',
    label: '咖啡因',
    value_type: 'text',
    options_text: '',
    required: false,
    show_in_price_list: false,
    position: 2,
  }])
  assert.deepEqual(specialAttrValuesFromJSON('{"roast_level":"中深烘","caffeine":"低因"}'), { roast_level: '中深烘', caffeine: '低因' })
  assert.equal(specialAttrValuesJSONFromForm({ roast_level: '中深烘', caffeine: '' }), '{"roast_level":"中深烘"}')
})

test('special KV schema saves edited select options and clears stale options when type changes', () => {
  assert.equal(specialAttrSchemaJSONFromRows([{
    key: 'roast_level',
    label: '烘焙度',
    value_type: 'select',
    options: ['浅烘', '中烘'],
    options_text: '浅烘，深烘\n意式',
    show_in_price_list: true,
  }]), '[{"key":"roast_level","label":"烘焙度","value_type":"select","options":["浅烘","深烘","意式"],"required":false,"show_in_price_list":true,"position":1}]')

  assert.equal(specialAttrSchemaJSONFromRows([{
    key: 'roast_level',
    label: '烘焙度',
    value_type: 'text',
    options: ['浅烘', '中烘'],
    options_text: '浅烘，深烘',
    show_in_price_list: true,
  }]), '[{"key":"roast_level","label":"烘焙度","value_type":"text","options":[],"required":false,"show_in_price_list":true,"position":1}]')
})

test('instant coffee SKU payload carries SKU-owned BOM yield and special KV settings', () => {
  assert.deepEqual(buildProductCreatePayload({
    name: '速溶盒装',
    product_kind: 'instant_coffee',
    special_attr_values: { roast_level: '中烘' },
    yield_percent: 96,
  }), {
    name: '速溶盒装',
    product_kind: 'instant_coffee',
    remark: '',
    special_attrs_json: '{"roast_level":"中烘"}',
    yield_rate: 0.96,
  })

  assert.deepEqual(buildProductBasicsPayload({
    product_kind: 'instant_coffee',
    remark: '条装原料',
    special_attr_values: { roast_level: '中烘' },
    yield_percent: 98,
  }), {
    product_kind: 'instant_coffee',
    remark: '条装原料',
    special_attrs_json: '{"roast_level":"中烘"}',
    yield_rate: 0.98,
    margin_rate_override: null,
  })
})

test('global unit definitions and unit templates build reusable unit payloads', () => {
  assert.deepEqual(buildProductUnitDefinitionPayload({
    code: ' 盒 ',
    name: ' 盒 ',
    unit_type: 'package',
    allow_decimal: false,
    active: true,
  }), {
    code: '盒',
    name: '盒',
    unit_type: 'package',
    allow_decimal: false,
    active: true,
  })

  assert.deepEqual(buildProductUnitTemplatePayload({
    id: 12,
    name: ' 盒装200g ',
    inventory_unit: ' kg ',
    quote_unit: '盒',
    order_unit: '盒',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '盒', to_qty: 0.2, to_unit: 'kg' }],
    integer_unit: true,
  }), {
    id: 12,
    name: '盒装200g',
    inventory_unit: 'kg',
    quote_unit: '盒',
    order_unit: '盒',
    unit_conversion_json: '{"盒":{"kg":0.2}}',
    integer_unit: true,
    active: true,
  })
})

test('structured unit conversion rows round-trip to the existing unit conversion JSON contract', () => {
  const rows = unitConversionRowsFromJSON('{"盒":{"kg":0.2},"箱":{"盒":24}}')

  assert.deepEqual(rows, [
    { from_qty: 1, from_unit: '盒', to_qty: 0.2, to_unit: 'kg' },
    { from_qty: 1, from_unit: '箱', to_qty: 24, to_unit: '盒' },
  ])
  assert.equal(unitConversionJSONFromRows(rows), '{"盒":{"kg":0.2},"箱":{"盒":24}}')
  assert.equal(unitConversionJSONFromRows([{ from_qty: 2, from_unit: '袋', to_qty: 1, to_unit: '盒' }]), '{"袋":{"盒":0.5}}')
  assert.equal(unitConversionJSONFromRows([{ from_qty: 0, from_unit: '盒', to_qty: 0.2, to_unit: 'kg' }]), '{}')
})

test('structured unit rule form builds customer rule override JSON while allowing inheritance', () => {
  const form = unitRuleFormFromJSON('{"order_unit":"箱","unit_conversion_json":{"箱":{"盒":24}},"integer_unit":true}')

  assert.equal(form.inventory_unit, '')
  assert.equal(form.quote_unit, '')
  assert.equal(form.order_unit, '箱')
  assert.equal(form.integer_unit_mode, 'integer')
  assert.deepEqual(form.unit_conversion_rows, [{ from_qty: 1, from_unit: '箱', to_qty: 24, to_unit: '盒' }])
  assert.equal(unitRuleJSONFromForm(form), '{"order_unit":"箱","unit_conversion_json":{"箱":{"盒":24}},"integer_unit":true}')
  assert.equal(unitRuleJSONFromForm({ integer_unit_mode: 'inherit' }), '{}')
  assert.equal(unitRuleJSONFromForm({ integer_unit_mode: 'decimal' }), '{"integer_unit":false}')
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

test('normalizeVisibleSkuFilters drops hidden legacy SKU filters restored from old drafts', () => {
  const publicRows = Array.from({ length: 34 }, (_, index) => ({
    id: index + 1,
    name: `公共 SKU ${index + 1}`,
    product_kind: 'roasted',
    custom_type: 'standard',
    customer_id: 0,
    primary_name: '咖啡烘焙豆',
    secondary_name: '精品意式拼配',
  }))
  const filters = normalizeVisibleSkuFilters({
    productKind: 'green_bean',
    customType: 'custom_blend',
    query: ' ',
    primaryCategory: '咖啡烘焙豆',
    secondaryCategory: '精品意式拼配',
  })

  assert.deepEqual(filters, {
    productKind: 'all',
    customType: 'all',
    query: '',
    primaryCategory: '咖啡烘焙豆',
    secondaryCategory: '精品意式拼配',
  })
  assert.equal(filterSkuRows(publicRows, filters).length, 34)

  const staleCategoryFilters = normalizeVisibleSkuFilters({
    primaryCategory: '已删除产品类型',
    secondaryCategory: '已删除产品子类型',
  }, publicRows)
  assert.deepEqual(staleCategoryFilters, {
    productKind: 'all',
    customType: 'all',
    query: '',
    primaryCategory: '',
    secondaryCategory: '',
  })
  assert.equal(filterSkuRows(publicRows, staleCategoryFilters).length, 34)
})

test('skuListRowsFromProducts keeps the SKU table backed by product rows even when category projection is empty', () => {
  const publicRows = Array.from({ length: 34 }, (_, index) => ({
    id: index + 1,
    name: `公共 SKU ${index + 1}`,
    customer_id: 0,
    product_category_id: index === 0 ? 2 : 0,
  }))
  assert.equal(skuListRowsFromProducts(publicRows, [], (product) => Number(product.customer_id || 0) === 0).length, 34)

  const categorized = skuListRowsFromProducts(publicRows, [{
    id: 1,
    name: '咖啡烘焙豆',
    products: [],
    children: [{
      id: 2,
      name: '精品意式拼配',
      products: [{ id: 1, number: 7 }],
    }],
  }], () => true)
  assert.deepEqual({
    id: categorized[0].id,
    number: categorized[0].number,
    primary_name: categorized[0].primary_name,
    secondary_name: categorized[0].secondary_name,
  }, {
    id: 1,
    number: 7,
    primary_name: '咖啡烘焙豆',
    secondary_name: '精品意式拼配',
  })

  const rowsFromCategoryID = skuListRowsFromProducts(publicRows, [{
    id: 1,
    name: '咖啡烘焙豆',
    products: [],
    children: [{
      id: 2,
      name: '精品意式拼配',
      products: [],
    }],
  }], () => true)
  assert.deepEqual({
    id: rowsFromCategoryID[0].id,
    primary_name: rowsFromCategoryID[0].primary_name,
    secondary_name: rowsFromCategoryID[0].secondary_name,
  }, {
    id: 1,
    primary_name: '咖啡烘焙豆',
    secondary_name: '精品意式拼配',
  })
})

test('skuTableState keeps visible rows, total, and category filters in one consistent calculation', () => {
  const rows = Array.from({ length: 34 }, (_, index) => ({
    id: index + 1,
    name: `公共 SKU ${index + 1}`,
    customer_id: 0,
    primary_name: index < 20 ? '咖啡烘焙豆' : '速溶咖啡',
    secondary_name: index < 20 ? '精品意式拼配' : '盒装',
  }))

  assert.deepEqual(skuTableState(rows, { primaryCategory: '不存在的旧分类' }, { page: 1, pageSize: 10 }), {
    filters: {
      productKind: 'all',
      customType: 'all',
      query: '',
      primaryCategory: '',
      secondaryCategory: '',
    },
    primaryOptions: ['咖啡烘焙豆', '速溶咖啡'],
    secondaryOptions: ['盒装', '精品意式拼配'],
    total: 34,
    page: 1,
    pageSize: 10,
    rows: rows.slice(0, 10),
  })
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
    special_attrs_json: '{}',
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
    special_attrs_json: '{}',
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
    special_attrs_json: '{}',
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
  assert.equal(payload.special_attrs_json, '{}')
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
    special_attrs_json: '{}',
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
    special_attrs_json: '{}',
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
    special_attrs_json: '{}',
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
    special_attr_values: { roast_level: '深烘' },
    custom_type: 'custom_roast',
    copy_bom: true,
    copy_price_tiers: true,
  }), {
    customer_id: 42,
    base_product_id: 0,
    name: '客户A-专属深烘',
    remark: '',
    product_kind: 'roasted',
    special_attrs_json: '{"roast_level":"深烘"}',
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

test('customer category tree does not show public SKUs when only public categories are enabled', () => {
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
    usePublicSkuInCategoryTree: false,
    usePublicSku: false,
    customerProducts: [],
  })

  assert.deepEqual(tree.map((row) => row.name), ['咖啡豆'])
  assert.deepEqual(tree[0].children[0].products.map((row) => row.name), [])
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

test('customer context filters product config templates while allowing public templates to be copied', () => {
  const publicTemplate = { id: 301, name: '盒装商品配置', customer_id: 0, template_state: 'public_template' }
  const customerTemplate = { id: 401, name: '岩师傅 - 盒装商品配置', customer_id: 42, source_template_id: 301, template_state: 'derived_from_public' }

  assert.equal(productConfigTemplateBelongsToSkuContext(publicTemplate, { customerID: 42, customerTemplates: [] }), true)
  assert.equal(productConfigTemplateBelongsToSkuContext(publicTemplate, { customerID: 42, customerTemplates: [customerTemplate] }), true)
  assert.equal(productConfigTemplateBelongsToSkuContext(customerTemplate, { customerID: 42 }), true)
})

test('SKU settings renders special KV template definitions and SKU value editors instead of hardcoded roast selectors', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  for (const expected of [
    '特殊KV定义',
    'special_attrs_schema_rows',
    'specialAttrSchemaForProduct',
    'specialAttrSchemaForForm',
    'special_attr_values',
    'show_in_price_list',
  ]) {
    assert.ok(source.includes(expected), `missing special KV UI marker: ${expected}`)
  }
  assert.doesNotMatch(template, /v-model="row\.roast_level"/)
  assert.doesNotMatch(template, /v-model="productForm\.roast_level"/)
  assert.doesNotMatch(template, /v-model="customForm\.roast_level"/)
})

test('SKU settings makes special KV configuration discoverable from SKU rows and template editor', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const expected of [
    '产品信息字段（特殊属性KV）',
    '展示到价格表/PDF',
    'SKU列表特殊属性列填写具体值',
    '价格表页面、发布快照和 PDF 均展示',
    '未配置字段',
    '配置字段',
    'openSpecialAttrConfigForProduct',
  ]) {
    assert.ok(source.includes(expected), `missing discoverable special KV copy: ${expected}`)
  }
  assert.match(template, /class="[^"]*sku-empty-special-attrs[^"]*"/)
  assert.match(script, /activeSettingsSection\.value = 'templates'[\s\S]*activeConfigTemplateSection\.value = 'product-config'/)
})

test('SKU settings clears special KV dropdown options when the field type is changed away from select', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.ok(script.includes('handleSpecialAttrSchemaTypeChange'), 'missing special KV type-change handler')
  assert.match(template, /@change="handleSpecialAttrSchemaTypeChange\(attr\)"/)
  assert.match(template, /<textarea[\s\S]*v-model\.trim="attr\.options_text"[\s\S]*下拉选项/)
})

test('copying public product config stays a template copy and no longer toggles public SKU references', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const expected of [
    'deriveProductConfigTemplateForCustomer',
    '公共商品配置已复制为客户配置',
  ]) {
    assert.ok(script.includes(expected) || source.includes(expected), `missing product config copy behavior: ${expected}`)
  }
  assert.doesNotMatch(script, /ensurePublicProductReferenceForCustomer/)
  assert.doesNotMatch(script, /use_public_sku:\s*true/)
  assert.doesNotMatch(script, /use_public_categories:\s*true/)
  assert.doesNotMatch(script, /公共SKU引用/)
  assert.doesNotMatch(script, /deriveCustomerProductForCustomer[\s\S]*deriveProductConfigTemplateForCustomer/)
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

test('SKU creation uses one unified SKU form without public customer custom split', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(source, /产品类别/)
  assert.match(source, /产品子类型/)
  assert.match(source, /productTypeCategoryOptions/)
  assert.match(source, /productSubtypeCategoryOptions/)
  assert.match(source, /skuForm\.product_type_category_id/)
  assert.match(source, /skuForm\.product_subtype_category_id/)
  assert.match(source, /@submit\.prevent="createSku"/)
  assert.match(script, /apiSend\('\/api\/product-settings\/skus'/)
  assert.match(source, /停车场/)
  assert.doesNotMatch(template, /新增公共 SKU/)
  assert.doesNotMatch(template, /新增客户专属 SKU/)
  assert.doesNotMatch(template, /基础产品/)
  assert.doesNotMatch(template, /定制类型/)
  assert.doesNotMatch(template, /复制基础产品 BOM/)
  assert.doesNotMatch(template, /创建公共 SKU/)
  assert.doesNotMatch(template, /创建专属 SKU/)
  assert.doesNotMatch(source, /<span>产品形态<\/span>/)
  assert.doesNotMatch(source, /v-model="skuForm\.product_kind"/)
  assert.doesNotMatch(source, /const categoryID = Number\(form\?\.product_type_category_id/)
})

test('SKU settings exposes SKU copy drawer and moves category management under product config', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const expected of [
    'SKU复制',
    'sku-copy-drawer',
    '选择分类和产品',
    'copySourceCustomerID',
    'skuCopySourceOptions',
    'ensureSkuCopySource',
    'copySkuSelection',
    '复制SKU',
    '商品分类管理',
    "activeConfigTemplateSection === 'category-management'",
    '/api/product-settings/skus/copy-options',
    '/api/product-settings/skus/copy',
  ]) {
    assert.ok(source.includes(expected), `missing SKU copy/category management marker: ${expected}`)
  }
  assert.doesNotMatch(template, />分类设置</)
  assert.doesNotMatch(script, /derivePublicSku\(/)
  assert.doesNotMatch(script, /savePublicSkuUsageForCustomer/)
})

test('SKU settings exposes product subtype default unit configuration controls', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  for (const expected of [
    '商品配置',
    '复制为客户配置',
    'template-select',
    'productConfigTemplates',
    'saveProductConfigTemplate',
    'deriveProductConfigTemplateForCustomer',
    '/api/product-settings/product-config-templates',
    '库存单位',
    '报价单位',
    '录单单位',
    '新增换算',
    '整数单位',
    'bindProductConfigTemplateToSubtype',
    'buildProductConfigTemplatePayload',
    'buildProductCategoryConfigPayload',
  ]) {
    assert.ok(source.includes(expected), `missing product config UI marker: ${expected}`)
  }
  assert.doesNotMatch(source, />更换商品配置</)
  assert.doesNotMatch(source, /startProductSubtypeConfigEdit/)
  assert.doesNotMatch(source, /saveProductSubtypeConfig/)
  assert.doesNotMatch(source, /价格表规则 JSON/)
  assert.doesNotMatch(source, /单位换算 JSON/)
  assert.doesNotMatch(source, /单位规则 JSON/)
  assert.doesNotMatch(source, /客户产品规则/)
  assert.doesNotMatch(source, /客户规则模板/)
  assert.doesNotMatch(source, /客户专属覆盖/)
  assert.doesNotMatch(source, /纳入产品价格表/)
})

test('SKU settings initializes product config form after SKU context is available', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const setupSource = source.split('<script setup>')[1] || source
  const contextIndex = setupSource.indexOf('const skuContextCustomerID = computed')
  const formIndex = setupSource.indexOf('const productConfigTemplateForm = ref(defaultProductConfigTemplateForm())')

  assert.notEqual(contextIndex, -1, 'skuContextCustomerID declaration is missing')
  assert.notEqual(formIndex, -1, 'productConfigTemplateForm declaration is missing')
  assert.ok(
    contextIndex < formIndex,
    'productConfigTemplateForm calls a default form that reads skuContextCustomerID, so skuContextCustomerID must be declared first',
  )
})

test('SKU subtype config explains unit impact and stays inside narrow category panels', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  for (const expected of [
    '商品配置',
    '单位模板会影响产品价格表单位',
    '录单默认单位和库存/生产折算',
    '已发布价格表和历史订单不会被回改',
    'unit-impact-help',
    'repeat(auto-fit, minmax',
    'min-width: 0',
    'box-sizing: border-box',
    'grid-column: 1 / -1',
  ]) {
    assert.ok(source.includes(expected), `missing subtype clarity or responsive marker: ${expected}`)
  }
})

test('SKU settings removes public SKU references from the customer SKU list and uses SKU copy instead', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.match(source, /usePublicSkuInCategoryTree:\s*false/)
  assert.match(source, /usePublicSku:\s*false/)
  assert.match(template, /SKU复制/)
  assert.doesNotMatch(template, /是否使用公共SKU/)
  assert.doesNotMatch(source, /savePublicSkuUsageForCustomer/)
})

test('SKU settings renders one unified SKU form as a full-width drawer', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  assert.match(source, /class="settings-drawer product-editor-drawer"/)
  assert.match(source, /class="sku-create-form product-create-form product-drawer-form"/)
  assert.match(source, /@submit\.prevent="createSku"/)
  assert.match(source, /\.product-editor-drawer\s*\{\s*width:\s*min\(820px,\s*94vw\);/)
  assert.match(source, /\.product-drawer-form\s*\{\s*display:\s*grid;\s*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/)
})

test('SKU settings groups master data and template configuration into separate workspaces', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'activeSettingsSection',
    'sku-workspace-tabs',
    'sku-master-workspace',
    'sku-template-workspace',
    'master-data-layout',
    'template-workspace-stack',
    '商品资料',
    '商品配置',
  ]) {
    assert.ok(source.includes(expected), `missing SKU workspace layout marker: ${expected}`)
  }

  assert.ok(
    template.indexOf('class="sku-master-workspace"') < template.indexOf('class="sku-template-workspace"'),
    'master data workspace should be the primary daily-operation workspace before template configuration',
  )
  assert.ok(
    template.indexOf('class="panel product-panel"') < template.indexOf('class="sku-template-workspace"'),
    'SKU list should remain in the daily-operation workspace before template configuration',
  )
  assert.match(template, /class="panel-actions sku-panel-actions"[\s\S]*@click="openProductDrawer"[\s\S]*@click="openSkuCopyDrawer"/)
  assert.match(template, /activeConfigTemplateSection === 'category-management'/)
  assert.match(template, /id="sku-category-management-target"/)
  assert.match(style, /\.master-data-layout\s*\{\s*display:\s*grid;\s*grid-template-columns:\s*minmax\(0,\s*1fr\);/)
  assert.match(style, /\.template-workspace-stack\s*\{\s*display:\s*grid;\s*gap:\s*14px;/)
  assert.match(style, /@media\s*\(max-width:\s*1100px\)/)
  assert.match(style, /\.master-data-layout\s*\{\s*grid-template-columns:\s*1fr;\s*\}/)
})

test('SKU settings opens SKU creation and SKU copy behind drawers while category management is a config tab', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'skuCopyDrawerOpen',
    'sku-copy-drawer',
    'openSkuCopyDrawer',
    '商品分类管理',
    'categorySearchQuery',
    'visibleCategoryTreeForSkuContext',
    'category-search',
    'category-scroll-list',
    'product-editor-drawer',
    'openProductDrawer',
    'SKU复制',
    '新增SKU',
  ]) {
    assert.ok(source.includes(expected), `missing compact SKU settings marker: ${expected}`)
  }

  assert.doesNotMatch(template, /class="panel public-product-panel"/)
  assert.doesNotMatch(template, /class="panel custom-product-panel"/)
  assert.match(template, /class="panel-actions sku-panel-actions"[\s\S]*@click="openProductDrawer"[\s\S]*@click="openSkuCopyDrawer"/)
  assert.doesNotMatch(template, />分类设置</)
  assert.match(template, /v-for="primary in visibleCategoryTreeForSkuContext"/)
  assert.match(template, /<aside class="settings-drawer sku-copy-drawer"[\s\S]*选择分类和产品/)
  assert.match(template, /当前SKU \{\{ skuDisplayTotal \}\}/)
  assert.match(template, /:total="skuDisplayTotal"/)
  assert.match(template, /<table :key="skuTableKey" class="sku-table"/)
  assert.match(template, /:key="skuPaginationKey"/)
  assert.match(script, /const unfilteredDisplaySkuRows = ref\(\[\]\)/)
  assert.match(script, /const normalizedSkuFilters = ref\(normalizeVisibleSkuFilters\(skuFilters\.value, \[\]\)\)/)
  assert.match(script, /const filteredDisplaySkuRows = ref\(\[\]\)/)
  assert.match(script, /const skuDisplayTotal = ref\(0\)/)
  assert.match(script, /const skuDisplayKey = computed/)
  assert.match(script, /const skuTableKey = computed\(\(\) => `\$\{skuDisplayKey\.value\}:table`\)/)
  assert.match(script, /const skuPaginationKey = computed\(\(\) => `\$\{skuDisplayKey\.value\}:pagination`\)/)
  assert.match(script, /const displaySkuRows = ref\(\[\]\)/)
  assert.match(script, /const skuPrimaryCategoryOptions = ref\(\[\]\)/)
  assert.match(script, /const skuSecondaryCategoryOptions = ref\(\[\]\)/)
  assert.match(script, /function syncVisibleSkuTableState\(\)/)
  assert.match(script, /displaySkuRows\.value = pageState\.rows/)
  assert.match(script, /watch\(\[\s*publicSkuRows,\s*customerSkuRows,\s*skuFilters,\s*skuPage,\s*skuPageSize,\s*selectedCustomerSkuCustomerID,\s*\], syncVisibleSkuTableState, \{ deep: true, immediate: true \}\)/)
  assert.match(script, /applyWorkspaceCustomerContext\(\)\s+syncVisibleSkuTableState\(\)\s+pruneSelectedProducts\(displaySkuRows\.value\)/)
  assert.match(script, /await nextTick\(\)\s+syncVisibleSkuTableState\(\)\s+restoringProductSettingsDraft = false/)
  assert.doesNotMatch(script, /const skuTable = computed/)
  assert.match(style, /\.category-scroll-list\s*\{[^}]*max-height:\s*min\(640px,\s*calc\(100vh - 280px\)\);[^}]*overflow:\s*auto;/s)
  assert.match(style, /\.settings-drawer-mask\s*\{[^}]*position:\s*fixed;/s)
})

test('SKU settings edits product categories inline inside the category management tab', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'category-inline-toolbar',
    'createPrimaryCategoryInline',
    'togglePrimaryDeleteMode',
    'movePrimaryCategory',
    'startCategoryEdit(primary)',
    'createSecondaryCategoryInline(primary)',
    'toggleSecondaryDeleteMode(primary)',
    'secondaryDeleteModeFor',
    'category-sort-buttons',
    'category-delete-button',
  ]) {
    assert.ok(source.includes(expected), `missing inline category editor marker: ${expected}`)
  }

  assert.match(template, /activeConfigTemplateSection === 'category-management'[\s\S]*class="category-panel category-drawer-panel/)
  assert.doesNotMatch(source, /category-editor-drawer/)
  assert.doesNotMatch(source, /openCategoryDrawer/)
  assert.doesNotMatch(source, /openCategorySettingsDrawer/)
  assert.doesNotMatch(template, />编辑产品类型</)
  assert.doesNotMatch(template, />改名</)
  assert.match(template, /@click(?:\.stop)?="startCategoryEdit\(primary\)"/)
  assert.match(template, /@click(?:\.stop)?="startCategoryEdit\(secondary\)"/)
  assert.match(template, /@keyup\.enter(?:\.prevent)?="saveCategoryName\(primary\)"/)
  assert.match(template, /@keyup\.enter(?:\.prevent)?="saveCategoryName\(secondary\)"/)
  assert.match(script, /apiSend\(`\/api\/product-settings\/categories\/\$\{category\.id\}\/move`/)
  assert.match(style, /\.category-inline-toolbar\s*\{[^}]*display:\s*flex;/s)
})

test('SKU settings uses compact category action controls with right-side direct delete', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''
  const deleteStart = script.indexOf('async function deleteCategory(category)')
  const deleteEnd = script.indexOf('function flattenCategoryNodes', deleteStart)
  const deleteFunction = deleteStart >= 0 && deleteEnd > deleteStart ? script.slice(deleteStart, deleteEnd) : ''

  for (const expected of [
    'category-action-pill',
    'category-action-button',
    'category-sort-pill',
    'category-row-actions',
    'secondary-category-actions',
    'aria-label="上移产品类型"',
    'aria-label="下移产品类型"',
    'aria-label="删除产品类型"',
    'aria-label="删除产品子类型"',
  ]) {
    assert.ok(source.includes(expected), `missing polished category action marker: ${expected}`)
  }

  assert.doesNotMatch(template, /class="icon-action/)
  assert.match(template, /<div class="category-row-actions[^"]*">[\s\S]*category-delete-button/)
  assert.match(template, /<div class="secondary-category-actions">[\s\S]*category-delete-button/)
  assert.doesNotMatch(deleteFunction, /window\.confirm/)
  assert.doesNotMatch(style, /\.icon-action/)
  assert.match(style, /\.category-action-pill\s*\{[^}]*border-radius:\s*999px;/s)
  assert.match(style, /\.category-sort-pill\s*\{[^}]*border-radius:\s*999px;/s)
})

test('SKU category primary row keeps title and sorting on the left with collapse on the right', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'primary-category-left',
    'primary-category-right',
    'category-sort-buttons',
    'category-collapse-button',
  ]) {
    assert.ok(source.includes(expected), `missing primary category layout marker: ${expected}`)
  }

  assert.match(template, /<div class="primary-category-left">[\s\S]*category-sort-pill[\s\S]*primary-name-button/)
  assert.match(template, /<div class="category-row-actions primary-category-right">[\s\S]*category-collapse-button[\s\S]*category-delete-button/)
  assert.match(style, /\.primary-category-left\s*\{[^}]*display:\s*flex;/s)
  assert.match(style, /\.primary-category-right\s*\{[^}]*justify-content:\s*flex-end;/s)
})

test('SKU table keeps product type columns on one line and relies on horizontal scroll', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'sku-table-wrap',
    'class="sku-table"',
    'sku-category-cell',
    'sku-name-cell',
    'special-attrs-cell',
  ]) {
    assert.ok(source.includes(expected), `missing SKU table layout marker: ${expected}`)
  }
  assert.match(template, /<th class="sku-col-product-type">产品类型<\/th>/)
  assert.match(style, /\.sku-table-wrap\s*\{[^}]*overflow-x:\s*auto;/s)
  assert.match(style, /\.sku-table\s*\{[^}]*width:\s*max-content;[^}]*min-width:\s*1600px;/s)
  assert.match(style, /\.sku-table th,\s*\.sku-table td\s*\{[^}]*white-space:\s*nowrap;/s)
  assert.match(style, /\.sku-category-cell\s*\{[^}]*white-space:\s*nowrap;/s)
})

test('SKU settings collapses category levels and focuses newly created categories', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'collapsedPrimaryCategoryIds',
    'collapsedSecondaryCategoryIds',
    'togglePrimaryCategoryCollapse',
    'toggleSecondaryCategoryCollapse',
    'isPrimaryCategoryCollapsed(primary)',
    'isSecondaryCategoryCollapsed(secondary)',
    'focusCategoryAfterCreate',
    'scrollIntoView',
    'data-secondary-id',
    'category-collapse-button',
  ]) {
    assert.ok(source.includes(expected), `missing category collapse/focus marker: ${expected}`)
  }

  assert.match(template, /v-if="!isPrimaryCategoryCollapsed\(primary\)"[\s\S]*v-for="\(secondary, index\) in primary\.children"/)
  assert.match(template, /v-show="!isSecondaryCategoryCollapsed\(secondary\)"[\s\S]*class="product-chip-list"/)
  assert.match(script, /categorySearchQuery\.value\s*=\s*''[\s\S]*scrollIntoView/)
  assert.match(style, /\.category-collapse-button\s*\{/)
})

test('SKU settings binds subtype product config directly without a separate change button', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.match(template, /class="template-select"[\s\S]*@change\.stop="bindProductConfigTemplateToSubtype\(secondary, \$event\.target\.value\)"/)
  assert.doesNotMatch(template, />更换商品配置</)
  assert.doesNotMatch(source, /startProductSubtypeConfigEdit/)
  assert.doesNotMatch(source, /saveProductSubtypeConfig/)
  assert.doesNotMatch(source, /editingSubtypeConfigId/)
})

test('global unit dictionary is managed from global settings instead of SKU settings', () => {
  const productSettings = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const productTemplate = productSettings.split('<script setup>')[0] || productSettings
  const globalSettings = fs.readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
  const menuSource = fs.readFileSync(new URL('../lib/menu-ia.js', import.meta.url), 'utf8')

  for (const expected of [
    '全局设置',
    '全局单位字典',
    'productUnitDefinitions',
    'saveGlobalUnitDefinition',
    '/api/product-settings/units',
    'unit-definition-form',
  ]) {
    assert.ok(globalSettings.includes(expected), `missing global unit dictionary marker: ${expected}`)
  }

  assert.match(menuSource, /key:\s*'uiSettings'[\s\S]*label:\s*'全局设置'/)
  assert.doesNotMatch(globalSettings, />新建单位</)
  assert.doesNotMatch(productTemplate, /<strong>单位字典<\/strong>/)
  assert.doesNotMatch(productTemplate, /@submit\.prevent="saveProductUnitDefinition"/)
  assert.match(productTemplate, /基础单位在“全局设置”维护/)
})

test('SKU settings splits product config templates and gradient templates into nested tabs', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'activeConfigTemplateSection',
    'config-template-tabs',
    'product-config-template-pane',
    'gradient-template-pane',
    '商品配置模板',
    '阶梯价模板',
  ]) {
    assert.ok(source.includes(expected), `missing template tab marker: ${expected}`)
  }

  assert.match(template, /activeSettingsSection === 'templates'[\s\S]*商品配置/)
  assert.doesNotMatch(template, />模板配置</)
  assert.ok(
    template.indexOf('商品配置模板') < template.indexOf('阶梯价模板'),
    'product config template tab should be the first, frequent template tab before gradient templates',
  )
  assert.match(style, /\.config-template-tabs\s*\{[^}]*display:\s*inline-flex;/s)
})

test('SKU settings separates global unit templates into a peer configuration tab', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const settingsSource = fs.readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const expected of [
    'unit-template-pane',
    "activeConfigTemplateSection === 'unit-template'",
    'productUnitDefinitions',
    'productUnitTemplates',
    'saveProductUnitTemplate',
    'productConfigTemplateForm.unit_template_id',
    '基础单位在“全局设置”维护',
    '/api/product-settings/unit-templates',
  ]) {
    assert.ok(source.includes(expected), `missing global unit template marker: ${expected}`)
  }
  assert.match(settingsSource, /全局单位字典/)
  assert.match(settingsSource, /saveGlobalUnitDefinition/)
  assert.match(settingsSource, /\/api\/product-settings\/units/)
  assert.doesNotMatch(template, /<strong>单位字典<\/strong>/)
  assert.doesNotMatch(source, /saveProductUnitDefinition/)

  assert.ok(
    template.indexOf('商品配置模板') < template.indexOf('单位模板')
      && template.indexOf('单位模板') < template.indexOf('阶梯价模板'),
    'unit template tab should sit between product config templates and gradient templates',
  )
  assert.doesNotMatch(template, /<div class="field-group-title">单位规则<\/div>[\s\S]*productConfigTemplateForm\.unit_conversion_rows/)
  assert.match(script, /buildProductConfigTemplatePayload\([\s\S]*unit_template_id/)
})

test('SKU product config uses display unit from unit dictionary instead of fixed display modes', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const productConfigPane = source.match(/<div v-show="activeConfigTemplateSection === 'product-config'"[\s\S]*?<div v-if="productDrawerOpen"/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.ok(productConfigPane, 'product config pane should exist')
  assert.doesNotMatch(productConfigPane, /价格表展示单位/)
  assert.doesNotMatch(productConfigPane, /price_rule_display_unit/)
  assert.doesNotMatch(productConfigPane, /priceListRuleDisplayUnitOptions/)
  assert.match(productConfigPane, /固定单价/)
  assert.match(productConfigPane, /成本加成/)
  assert.doesNotMatch(productConfigPane, /展示方式/)
  assert.doesNotMatch(source, /盒装\/箱装展示|按重量展示|priceListRuleDisplayModeOptions|price_rule_display_mode/)
  assert.doesNotMatch(script, /priceListRuleDisplayUnitOptions/)
  assert.match(script, /Object\.prototype\.hasOwnProperty\.call\(template,\s*'customer_id'\)/)
})

test('SKU product config template list and price rule controls are visually structured', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const productConfigPane = source.match(/<div v-show="activeConfigTemplateSection === 'product-config'"[\s\S]*?<div v-if="productDrawerOpen"/)?.[0] || ''
  const priceRuleBlock = productConfigPane.match(/<div class="rule-config-block price-rule-grid"[\s\S]*?<div class="rule-config-block">/)?.[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  assert.ok(productConfigPane, 'product config pane should exist')
  for (const expected of [
    'product-config-row',
    'product-config-row-title',
    'template-state-pill',
    'product-config-row-subtitle',
    'template-meta-chips',
    'productConfigUnitChips(config.unit_template_id)',
  ]) {
    assert.ok(productConfigPane.includes(expected), `missing product config list marker: ${expected}`)
  }
  for (const expected of [
    'rule-config-block price-rule-grid',
    'rule-config-field',
    'field-label-with-help',
    'type="button" class="field-help-icon"',
    'field-help-tooltip',
    'role="tooltip"',
  ]) {
    assert.ok(priceRuleBlock.includes(expected), `missing price rule layout marker: ${expected}`)
  }
  assert.doesNotMatch(priceRuleBlock, /<small>默认继承单位模板/)
  assert.match(style, /\.price-rule-grid \{[^}]*grid-template-columns: repeat\(3, minmax\(0, 1fr\)\)/)
  assert.match(style, /\.price-rule-grid \.rule-config-field \{[^}]*grid-template-rows: 22px auto/)
  assert.match(style, /\.field-help-tooltip/)
  assert.match(style, /\.field-help-icon \{[^}]*min-height: 16px/)
  assert.match(style, /\.product-config-row\.active/)
})

test('SKU unit template save creates or updates without a separate new-template button', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const unitTemplatePane = source.match(/<div v-show="activeConfigTemplateSection === 'unit-template'"[\s\S]*?<div v-show="activeConfigTemplateSection === 'product-config'"/)?.[0] || ''

  assert.ok(unitTemplatePane, 'unit template pane should exist')
  assert.doesNotMatch(unitTemplatePane, />新建模板</)
  assert.match(unitTemplatePane, /@click="resetProductUnitTemplateForm"[\s\S]*新增单位模板/)
  assert.match(source, /function resetProductUnitTemplateForm\(\)/)
  assert.match(source, /await apiSend\(url, \{ method, body: payload \}\)/)
  assert.match(source, /await loadAll\(\)\s+resetProductUnitTemplateForm\(\)/)
})

test('SKU settings compacts context area and uses create edit labels for unit dictionaries', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const settingsSource = fs.readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''
  const unitTemplatePane = source.match(/<div v-show="activeConfigTemplateSection === 'unit-template'"[\s\S]*?<div v-show="activeConfigTemplateSection === 'product-config'"/)?.[0] || ''
  const globalUnitDrawer = source.match(/<div v-if="globalUnitDrawerOpen"[\s\S]*?<\/aside>\s*<\/div>/)?.[0] || ''

  for (const expected of [
    'sku-page-summary',
    'compact-sku-context',
    'sku-context-title-line',
  ]) {
    assert.ok(source.includes(expected), `missing compact SKU context marker: ${expected}`)
  }

  assert.match(style, /\.sku-page-summary\s*\{[^}]*padding:\s*10px 12px;/s)
  assert.match(style, /\.compact-sku-context\s*\{[^}]*padding:\s*10px 12px;/s)
  assert.doesNotMatch(template, /产品列表、商品分类和商品配置会按当前归属切换。/)

  assert.match(unitTemplatePane, /@click="resetProductUnitTemplateForm"[\s\S]*新增单位模板/)
  assert.match(unitTemplatePane, /productUnitTemplateForm\.id\s*\?\s*'保存'\s*:\s*'新增'/)
  assert.match(unitTemplatePane, /成品库存单位/)
  assert.doesNotMatch(unitTemplatePane, />库存单位</)

  assert.match(script, /const globalUnitEditingCode = ref\(''\)/)
  assert.match(globalUnitDrawer, /@click="resetGlobalUnitDefinitionForm"[\s\S]*新增基础单位/)
  assert.match(globalUnitDrawer, /globalUnitEditingCode\s*\?\s*'保存'\s*:\s*'新增'/)

  assert.match(settingsSource, /const unitEditingCode = ref\(''\)/)
  assert.match(settingsSource, /@click="resetGlobalUnitDefinitionForm"[\s\S]*新增基础单位/)
  assert.match(settingsSource, /unitEditingCode\s*\?\s*'保存'\s*:\s*'新增'/)
})

test('SKU unit template workspace uses left list right editor and opens global unit dictionary drawer', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const unitTemplatePane = source.match(/<div v-show="activeConfigTemplateSection === 'unit-template'"[\s\S]*?<div v-show="activeConfigTemplateSection === 'product-config'"/)?.[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'unit-template-list-panel',
    'unit-template-editor-panel',
    'globalUnitDrawerOpen',
    'global-unit-dictionary-drawer',
    'openGlobalUnitDictionaryDrawer',
    'saveGlobalUnitDefinitionFromDrawer',
    'buildProductUnitDefinitionPayload',
    '全局单位字典',
  ]) {
    assert.ok(source.includes(expected), `missing unit template workspace marker: ${expected}`)
  }

  assert.ok(
    unitTemplatePane.indexOf('unit-template-list-panel') < unitTemplatePane.indexOf('unit-template-editor-panel'),
    'unit template list should be left of the editor in source order',
  )
  assert.match(unitTemplatePane, /@click="openGlobalUnitDictionaryDrawer"/)
  assert.match(source, /<aside class="settings-drawer global-unit-dictionary-drawer"/)
  assert.match(source, /@submit\.prevent="saveGlobalUnitDefinitionFromDrawer"/)
  assert.match(style, /\.unit-template-layout\s*\{[^}]*grid-template-columns:\s*minmax\(220px,\s*280px\)\s+minmax\(0,\s*1fr\);/s)
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
