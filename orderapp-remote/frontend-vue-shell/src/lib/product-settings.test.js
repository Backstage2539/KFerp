import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

import {
  buildAssignCategoryPayload,
  buildCustomerProductRuleBindingPayload,
  buildCustomerProductRuleOverridePayload,
  buildCustomerProductRuleTemplatePayload,
  buildCustomerProductAliasPayload,
  buildCustomerProductAliasBatchPayload,
  buildClassificationTemplateUsagePayload,
  buildCustomerProductAliasIndustryFieldPayload,
  classificationAssignmentConflict,
  classificationAssignmentLabel,
  classificationTemplateUnitPriceWarnings,
  customerProductAliasRowsForCustomer,
  industryFieldOptionsJSONFromText,
  industryFieldOptionsTextFromJSON,
  industryFieldSummary,
  classificationTemplateTabs,
  groupRowsByClassificationCategory,
  buildCustomerPublicUsagePayload,
  buildCustomProductCreatePayload,
  buildProductCategoryConfigPayload,
  buildProductConfigTemplatePayload,
  buildProductUnitDefinitionPayload,
  buildProductUnitTemplatePayload,
  buildProductBasicsPayload,
  buildProductBomURL,
  buildProductCreatePayload,
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
  productCreationActionOptions,
  productDisplayState,
  resolveCreatedProductForConfig,
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

test('instant coffee product kind carries legacy yield without writing SKU special attributes', () => {
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
    active: true,
  })
  assert.equal(Object.hasOwn(payload, 'product_kind'), false)
  assert.equal(Object.hasOwn(payload, 'custom_type'), false)
  assert.equal(Object.hasOwn(payload, 'base_product_id'), false)
  assert.equal(Object.hasOwn(payload, 'product_type_category_id'), false)
  assert.equal(Object.hasOwn(payload, 'product_subtype_category_id'), false)
})

test('customer product alias payload binds a customer-facing name to one product record', () => {
  assert.deepEqual(buildCustomerProductAliasPayload({
    id: '12',
    customer_id: '42',
    product_id: '88',
    display_name: ' Karen 精品拼配 ',
    customer_item_code: ' KAREN-ESP ',
    brand_name: ' ',
    display_category_id: '7',
    sort_order: '30',
    include_in_price_list: true,
    active: false,
    remark: '贴牌只改对外名称',
    classification_template_id: '88',
  }), {
    id: 12,
    customer_id: 42,
    product_id: 88,
    display_name: 'Karen 精品拼配',
    brand_name: '',
    display_category_id: 7,
    sort_order: 30,
    include_in_price_list: true,
    active: false,
    remark: '贴牌只改对外名称',
  })
})

test('customer product alias payloads never bind classification templates to aliases', () => {
  const single = buildCustomerProductAliasPayload({
    customer_id: '42',
    product_id: '88',
    display_name: 'Karen 精品拼配',
    classification_template_id: '88',
  })
  const batch = buildCustomerProductAliasBatchPayload({
    customer_id: '42',
    product_ids: [8, '9'],
    classification_template_id: '88',
  })

  assert.equal(Object.hasOwn(single, 'classification_template_id'), false)
  assert.equal(Object.hasOwn(batch, 'classification_template_id'), false)
})

test('customer product alias batch payload creates many customer-facing names from product records', () => {
  assert.deepEqual(buildCustomerProductAliasBatchPayload({
    customer_id: '42',
    product_ids: [8, '9', 0, 8, 'bad'],
    include_in_price_list: true,
    brand_name: ' ',
    display_category_id: '12',
  }), {
    customer_id: 42,
    product_ids: [8, 9],
    include_in_price_list: true,
    brand_name: '',
    display_category_id: 12,
  })
})

test('customer product alias rows support active filters and search like product archive', () => {
  const aliases = [
    { id: 1, customer_id: 42, display_name: 'Karen 深烘', customer_item_code: 'CPA-000001', product_code: 'SKU-000001', product_name: '意式拼配', brand_name: '', active: true },
    { id: 2, customer_id: 42, display_name: 'Karen 中烘', customer_item_code: 'CPA-000002', product_code: 'SKU-000002', product_name: '甜感拼配', brand_name: '', active: false },
    { id: 3, customer_id: 9, display_name: 'Other', customer_item_code: 'CPA-000003', product_code: 'SKU-000003', product_name: '其他', brand_name: '', active: true },
  ]

  assert.deepEqual(customerProductAliasRowsForCustomer(aliases, 42, { active: 'active' }).map((row) => row.id), [1])
  assert.deepEqual(customerProductAliasRowsForCustomer(aliases, 42, { active: 'inactive' }).map((row) => row.id), [2])
  assert.deepEqual(customerProductAliasRowsForCustomer(aliases, 42, { active: 'all', query: '甜感' }).map((row) => row.id), [2])
  assert.deepEqual(customerProductAliasRowsForCustomer(aliases, 42, { active: 'all', query: 'SKU-000001' }).map((row) => row.id), [1])
})

test('industry field helpers use comma text for select options and build alias field payloads', () => {
  assert.equal(industryFieldOptionsTextFromJSON('["浅烘","中烘","深烘"]'), '浅烘, 中烘, 深烘')
  assert.equal(industryFieldOptionsJSONFromText('浅烘, 中烘，深烘'), '["浅烘","中烘","深烘"]')
  assert.equal(industryFieldSummary([
    { label: '烘焙度', value_text: '深烘' },
    { field_key: 'process', value_text: '水洗' },
    { label: '空值', value_text: '' },
  ]), '烘焙度：深烘；process：水洗')
  assert.deepEqual(buildCustomerProductAliasIndustryFieldPayload({
    fields: [
      { field_key: 'roast_level', value_text: ' 深烘 ' },
      { field_key: '', value_text: '跳过' },
    ],
  }), {
    fields: [
      { field_key: 'roast_level', value_text: '深烘' },
    ],
  })
})

test('classification template usages are page-level tabs instead of object fields', () => {
  assert.deepEqual(buildClassificationTemplateUsagePayload({
    customer_id: '42',
    classification_template_id: '88',
    sort_order: '20',
  }), {
    customer_id: 42,
    classification_template_id: 88,
    sort_order: 20,
  })

  const templates = [
    { id: 88, name: '门店展示', active: true, sort_order: 20 },
    { id: 89, name: '报价分类', active: true, sort_order: 10 },
    { id: 90, name: '停用模板', active: false, sort_order: 1 },
  ]
  const tabs = classificationTemplateTabs(templates, [
    { classification_template_id: 88, active: true, sort_order: 30 },
    { classification_template_id: 89, active: true, sort_order: 10 },
    { classification_template_id: 90, active: true, sort_order: 1 },
  ], { allLabel: '全部客户商品', unclassifiedLabel: '未分类客户商品' })

  assert.deepEqual(tabs.map((tab) => tab.label), ['全部客户商品', '未分类客户商品', '报价分类', '门店展示'])
  assert.deepEqual(groupRowsByClassificationCategory([
    { id: 1, display_name: 'A' },
    { id: 2, display_name: 'B' },
    { id: 3, display_name: 'C' },
  ], {
    id: 88,
    categories: [
      { id: 701, name: '挂耳', sort_order: 20, active: true },
      { id: 700, name: '熟豆', sort_order: 10, active: true },
    ],
    customer_alias_assignments: [
      { alias_id: 2, template_id: 88, category_id: 701, sort_order: 1 },
      { alias_id: 3, template_id: 88, category_id: 0, sort_order: 2 },
    ],
  }, { idKey: 'id', assignmentKey: 'alias_id', assignmentsKey: 'customer_alias_assignments', onlyAssigned: true }).map((group) => [group.label, group.rows.map((row) => row.id)]), [
    ['熟豆', []],
    ['挂耳', [2]],
    ['未分类', [3]],
  ])
})

test('customer product alias rows are scoped to one customer and sorted for price-list display', () => {
  const rows = [
    { id: 3, customer_id: 7, product_id: 101, display_name: '其他客户', sort_order: 1, active: true },
    { id: 2, customer_id: 42, product_id: 88, display_name: '停用商品名', sort_order: 1, active: false },
    { id: 1, customer_id: 42, product_id: 87, display_name: 'Karen A', sort_order: 20, active: true },
    { id: 4, customer_id: 42, product_id: 89, display_name: 'Karen B', sort_order: 10, active: true },
  ]

  assert.deepEqual(customerProductAliasRowsForCustomer(rows, 42).map((row) => row.id), [4, 1])
  assert.deepEqual(customerProductAliasRowsForCustomer(rows, 42, { includeInactive: true }).map((row) => row.id), [2, 4, 1])
})

test('product creation actions keep only the product archive creation entry', () => {
  assert.deepEqual(productCreationActionOptions({ customerID: 42 }).map((option) => option.label), [
    '创建新商品档案',
  ])
  assert.deepEqual(productCreationActionOptions({ customerID: 0 }).map((option) => option.label), [
    '创建新商品档案',
  ])
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

test('instant coffee SKU payload carries legacy yield without SKU special attributes', () => {
  assert.deepEqual(buildProductCreatePayload({
    name: '速溶盒装',
    product_kind: 'instant_coffee',
    special_attr_values: { roast_level: '中烘' },
    yield_percent: 96,
  }), {
    name: '速溶盒装',
    product_kind: 'instant_coffee',
    remark: '',
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
    active: 'active',
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
    active: 'active',
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
      active: 'active',
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
  assert.equal(Object.hasOwn(payload, 'special_attrs_json'), false)
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

test('product BOM URL carries production BOM id for BOM maintenance jumps', () => {
  const url = buildProductBomURL('https://erp.test/vue-shell?view=productSettings&workspace=customer&customer_id=74', { id: 88, production_bom_id: 19 })
  assert.equal(url.searchParams.get('view'), 'bom')
  assert.equal(url.searchParams.get('production_bom_id'), '19')
  assert.equal(url.searchParams.get('product_id'), null)
  assert.equal(url.searchParams.get('bom_filter_product_id'), null)
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
  assert.equal(productConfigTemplateBelongsToSkuContext(publicTemplate, { customerID: 42, customerTemplates: [customerTemplate] }), false)
  assert.equal(productConfigTemplateBelongsToSkuContext(customerTemplate, { customerID: 42 }), true)
})

test('SKU template panes render context-filtered template lists', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  assert.match(source, /v-for="template in gradientTemplatesForContext"/)
  assert.doesNotMatch(source, /v-for="template in gradientTemplates"/)
  assert.match(source, /v-for="config in productConfigTemplatesForContext"/)
})

test('SKU settings no longer renders special KV template definitions or SKU value editors', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  for (const removed of [
    '特殊KV定义',
    'special_attrs_schema_rows',
    'specialAttrSchemaForProduct',
    'specialAttrSchemaForForm',
    '产品信息字段（特殊属性KV）',
    'special-attr-editor',
    'openSpecialAttrConfigForProduct',
  ]) {
    assert.doesNotMatch(source, new RegExp(removed.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.doesNotMatch(template, /v-model="row\.roast_level"/)
  assert.doesNotMatch(template, /v-model="productForm\.roast_level"/)
  assert.doesNotMatch(template, /v-model="customForm\.roast_level"/)
})

test('product pages split product archive, aliases and config templates without workspace tabs', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const app = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  for (const expected of [
    'productMaster',
    'customerProductAliases',
    'productConfigTemplates',
    '商品档案',
    '客户商品名',
    '商品配置模板',
    '生产配置',
    '商品分类',
    'sectionMode',
  ]) {
    assert.ok(source.includes(expected) || app.includes(expected), `missing split product page marker: ${expected}`)
  }
  assert.doesNotMatch(template, /sku-workspace-tabs/)
  assert.doesNotMatch(template, /activeSettingsSection === 'templates' && activeConfigTemplateSection === 'category-management'/)
})

test('product config template page no longer contains product category management', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const configPageBlock = template.match(/productConfigTemplates[\s\S]*?<\/section>/)?.[0] || template

  assert.match(source, /商品配置模板/)
  assert.match(configPageBlock, /单位模板/)
  assert.match(configPageBlock, /阶梯价模板/)
  assert.doesNotMatch(configPageBlock, />商品分类管理</)
  assert.doesNotMatch(template, /currentSettingsSection === 'master'[\s\S]*class="category-panel category-drawer-panel category-management-panel/)
  assert.match(template, /activeConfigTemplateSection === 'classification-template'/)
})

test('BOM view no longer exposes special attributes from BOM version detail', () => {
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.doesNotMatch(source, /BOM版本与特殊属性/)
  assert.doesNotMatch(source, /特殊属性绑定到 BOM 版本/)
  assert.doesNotMatch(source, /字段定义 special_attrs_schema_json/)
  assert.doesNotMatch(source, /字段值 special_attrs_json/)
  assert.doesNotMatch(source, /保存特殊属性/)
  assert.doesNotMatch(source, /保存预期损耗率/)
  assert.doesNotMatch(source, /预期产出率/)
  assert.doesNotMatch(template, /versionSpecialAttrsSchemaText/)
})

test('product archive config drawer owns template, BOM, route, expected loss and industry fields', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const bomSource = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(source, /商品档案配置/)
  assert.match(source, /商品配置模板/)
  assert.match(source, /行业字段模板/)
  assert.match(source, /预期损耗率/)
  assert.match(source, /工艺路线/)
  assert.match(source, /show_in_price_list/)
  assert.match(source, /\/api\/product-production-configs/)
  assert.match(source, /openProductProductionConfig\(row\)/)
  assert.match(source, /productProductionConfigDrawerOpen/)
  assert.match(source, /保存商品档案配置/)
  assert.doesNotMatch(source, /addProductProductionConfigField/)
  assert.match(source, /productProductionConfigForm\.fields/)
  assert.match(source, /\/api\/process-routes\?status=published/)
  assert.match(source, /维护当前 BOM 明细/)
  assert.match(script, /saveProductProductionConfig/)
  assert.match(script, /kferp:navigate-view/)
  assert.match(source, /product-return-banner/)
  assert.match(source, /productReturnNavigation/)
  assert.match(source, /returnToPreviousView/)
  assert.match(source, /完成商品档案配置后可回到来源操作界面/)
  assert.doesNotMatch(source, /兼容产出因子/)
  assert.doesNotMatch(source, /预期产出/)
  assert.doesNotMatch(bomSource, /expected_loss_rate/)
  assert.doesNotMatch(bomSource, /special_attrs_json/)
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

test('SKU creation uses one unified product archive form without legacy classification fields', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const productDrawer = source.match(/<aside class="settings-drawer product-editor-drawer"[\s\S]*?<\/aside>/)?.[0] || ''

  assert.match(source, /@submit\.prevent="createSku"/)
  assert.match(script, /apiSend\('\/api\/product-settings\/skus'/)
  assert.doesNotMatch(productDrawer, /产品类别/)
  assert.doesNotMatch(productDrawer, /产品子类型/)
  assert.doesNotMatch(productDrawer, /productTypeCategoryOptions/)
  assert.doesNotMatch(productDrawer, /productSubtypeCategoryOptions/)
  assert.doesNotMatch(productDrawer, /skuForm\.product_type_category_id/)
  assert.doesNotMatch(productDrawer, /skuForm\.product_subtype_category_id/)
  assert.doesNotMatch(productDrawer, /停车场/)
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

test('SKU settings removes legacy SKU copy drawer while classification templates replace legacy category management', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const expected of [
    '复制为商品档案',
    '分类模板',
    'productClassificationTabs',
    "currentSettingsSection === 'master'",
    '/api/product-settings/products/${row.id}/copy',
  ]) {
    assert.ok(source.includes(expected), `missing SKU copy/category management marker: ${expected}`)
  }
  for (const removed of [
    '历史SKU复制',
    'sku-copy-drawer',
    'copySourceCustomerID',
    'skuCopySourceOptions',
    'ensureSkuCopySource',
    'copySkuSelection',
    '/api/product-settings/skus/copy-options',
    '/api/product-settings/skus/copy',
  ]) {
    assert.doesNotMatch(source, new RegExp(removed.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
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
    'productConfigTemplates',
    'saveProductConfigTemplate',
    'deriveProductConfigTemplateForCustomer',
    '/api/product-settings/product-config-templates',
    '库存单位',
    '报价单位',
    '录单单位',
    '新增换算',
    '整数单位',
    'buildProductConfigTemplatePayload',
  ]) {
    assert.ok(source.includes(expected), `missing product config UI marker: ${expected}`)
  }
  assert.match(source, /product_config_template_id/)
  assert.match(source, /商品档案配置/)
  assert.doesNotMatch(source, /bindProductConfigTemplateToSubtype/)
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

test('SKU settings removes public SKU references from the customer SKU list and product archive flow', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.match(source, /usePublicSkuInCategoryTree:\s*false/)
  assert.match(source, /usePublicSku:\s*false/)
  assert.match(template, /创建新商品档案/)
  assert.doesNotMatch(template, /SKU复制/)
  assert.doesNotMatch(template, /是否使用公共SKU/)
  assert.doesNotMatch(source, /savePublicSkuUsageForCustomer/)
})

test('product archive list uses the product name as the only production config entry', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.match(template, /生产 BOM/)
  assert.match(source, /productionBomLabel\(row\)/)
  assert.match(source, /productionBomVersionWarning\(row\)/)
  assert.match(template, /当前引用/)
  assert.match(template, /class="[^"]*sku-name-button[^"]*"[\s\S]*@click="openProductProductionConfig\(row\)"/)
  assert.ok(template.indexOf('<th class="sku-name-cell">商品名</th>') < template.indexOf('<th>商品编号</th>'), '商品名 must be the first business column before 商品编号')
  assert.doesNotMatch(template, />生产配置<\/button>/)
  assert.doesNotMatch(template, /更换生产 BOM/)
  assert.doesNotMatch(template, />维护 BOM<\/button>/)
  assert.doesNotMatch(template, /<th>BOM<\/th>/)
  assert.doesNotMatch(template, /product-action-guide/)
  assert.doesNotMatch(template, /production-config-summary/)
  assert.doesNotMatch(source, /BOM已失效/)
  assert.doesNotMatch(source, /缺BOM/)
  assert.doesNotMatch(source, /row\.bom_status/)
  assert.doesNotMatch(template, /特殊属性/)
  assert.doesNotMatch(template, /special-attr-editor/)
  assert.doesNotMatch(template, /产品信息字段（特殊属性KV）/)
  assert.doesNotMatch(template, /新增KV/)
  assert.doesNotMatch(source, /special_attrs_schema_rows/)
  assert.doesNotMatch(template, /派生自有 BOM/)
  assert.doesNotMatch(template, /自有 BOM/)
  assert.doesNotMatch(template, /跟随默认 BOM/)
  assert.doesNotMatch(template, /固定 BOM 版本/)
})

test('new product archive creation opens the created product config drawer with the reloaded product row', () => {
  const reloaded = [
    { id: 8, name: '旧商品', product_config_template_id: 0 },
    { id: 12, name: '新商品', product_config_template_id: 301, production_bom_id: 44 },
  ]
  assert.deepEqual(
    resolveCreatedProductForConfig({ id: '12', name: '新商品' }, reloaded),
    reloaded[1],
  )
  assert.deepEqual(
    resolveCreatedProductForConfig({ product: { id: 12 } }, reloaded),
    reloaded[1],
  )

  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const createSkuBlock = script.match(/async function createSku\(\) \{[\s\S]*?\n\}/)?.[0] || ''

  assert.match(createSkuBlock, /const result = await apiSend\('\/api\/product-settings\/skus'/)
  assert.match(createSkuBlock, /await loadAll\(\)[\s\S]*resolveCreatedProductForConfig\(result/)
  assert.match(createSkuBlock, /await openProductProductionConfig\(createdProductForConfig\)/)
})

test('SKU settings renders one unified SKU form as a full-width drawer', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  assert.match(source, /class="settings-drawer product-editor-drawer"/)
  assert.match(source, /class="sku-create-form product-create-form product-drawer-form"/)
  assert.match(source, /@submit\.prevent="createSku"/)
  assert.match(source, /\.product-editor-drawer\s*\{\s*width:\s*min\(820px,\s*94vw\);/)
  assert.match(source, /\.product-drawer-form\s*\{\s*display:\s*grid;\s*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/)
})

test('product pages group product archive and template configuration into separate pages', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'currentSettingsSection',
    'sectionMode',
    'sku-master-workspace',
    'sku-template-workspace',
    'master-data-layout',
    'template-workspace-stack',
    '商品档案',
    '商品配置模板',
  ]) {
    assert.ok(source.includes(expected), `missing product page layout marker: ${expected}`)
  }

  assert.ok(
    template.indexOf('class="sku-master-workspace"') < template.indexOf('class="sku-template-workspace"'),
    'master data workspace should be the primary daily-operation workspace before template configuration',
  )
  assert.ok(
    template.indexOf('class="panel product-panel"') < template.indexOf('class="sku-template-workspace"'),
    'SKU list should remain in the daily-operation workspace before template configuration',
  )
  assert.match(template, /class="sku-filters product-filter-row"[\s\S]*@click="openProductDrawer"/)
  assert.doesNotMatch(template, /@click="openSkuCopyDrawer"/)
  assert.doesNotMatch(template, /v-if="currentSettingsSection === 'master'"[\s\S]*class="category-panel category-drawer-panel category-management-panel"/)
  assert.match(template, /class="classification-view-toolbar product-classification-tabs"/)
  assert.match(style, /\.master-data-layout\s*\{\s*display:\s*grid;\s*grid-template-columns:\s*minmax\(0,\s*1fr\);/)
  assert.match(style, /\.template-workspace-stack\s*\{\s*display:\s*grid;\s*gap:\s*14px;/)
  assert.match(style, /@media\s*\(max-width:\s*1100px\)/)
  assert.match(style, /\.master-data-layout\s*\{\s*grid-template-columns:\s*1fr;\s*\}/)
})

test('product management exposes customer product names without direct BOM editing', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const expected of [
    '客户商品名',
    'customer-alias-workspace',
    '客户商品编号',
    '品牌名',
    '进入价格表',
    '绑定商品档案',
    'customerProductAliases',
    'buildCustomerProductAliasPayload',
    '/api/customer-product-aliases',
    'saveCustomerProductAlias',
    'disableCustomerProductAlias',
    '客户商品名只维护对外名称、编号、品牌和价格表展示',
    'customer-alias-create-drawer',
    'openCustomerAliasCreateDrawer',
    '绑定商品已失效',
  ]) {
    assert.ok(source.includes(expected), `missing customer product alias marker: ${expected}`)
  }
  assert.match(template, /currentSettingsSection === 'aliases'/)
  assert.match(script, /apiGet\('\/api\/customer-product-aliases\?active=all'\)/)
  assert.match(script, /apiSend\(url,\s*\{ method,\s*body:\s*payload \}\)/)
  assert.match(script, /apiSend\(`\/api\/customer-product-aliases\/\$\{alias\.id\}\/disable`\)/)
  assert.match(script, /apiSend\('\/api\/customer-product-aliases\/batch-disable'/)
  const aliasForm = template.match(/<aside class="settings-drawer customer-alias-create-drawer"[\s\S]*?<\/aside>/)?.[0] || ''
  const inlineAliasArea = template.match(/<section class="panel customer-alias-panel"[\s\S]*?<div class="table-wrap">/)?.[0] || ''
  const aliasFilters = template.match(/<div class="alias-filters alias-filter-row"[\s\S]*?<div class="classification-view-toolbar alias-classification-tabs"/)?.[0] || ''
  assert.doesNotMatch(aliasForm, /customerProductAliasForm\.customer_item_code/)
  assert.doesNotMatch(aliasForm, /customerProductAliasForm\.include_in_price_list/)
  assert.doesNotMatch(aliasForm, />进入价格表</)
  assert.doesNotMatch(inlineAliasArea, /<form class="customer-alias-form"/)
  assert.match(aliasFilters, /新建客户商品/)
  assert.match(aliasFilters, /批量失效/)
  assert.doesNotMatch(aliasFilters, />搜索客户商品</)
  assert.doesNotMatch(template, />编辑<\/button>/)
  assert.doesNotMatch(template, /客户商品名[\s\S]*派生自有 BOM/)
  assert.doesNotMatch(template, /customer-alias-workspace[\s\S]*@click="derive/)
  assert.doesNotMatch(template, /旧客户 SKU 收敛检查/)
  assert.doesNotMatch(source, /aliasMigrationCandidates/)
  assert.doesNotMatch(source, /migration-candidates/)
})

test('SKU settings keeps only the product creation drawer while classification templates drive product tabs', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'productClassificationTabs',
    'displaySkuGroups',
    '增加分类',
    '移动到分类',
    'classification-group-row',
    'product-editor-drawer',
    'openProductDrawer',
    '创建新商品档案',
  ]) {
    assert.ok(source.includes(expected), `missing compact SKU settings marker: ${expected}`)
  }

  assert.doesNotMatch(template, /class="panel public-product-panel"/)
  assert.doesNotMatch(template, /class="panel custom-product-panel"/)
  assert.doesNotMatch(template, /class="panel-actions sku-panel-actions"[\s\S]*@click="openProductDrawer"/)
  assert.match(template, /class="sku-filters product-filter-row"[\s\S]*@click="openProductDrawer"[\s\S]*deactivateProducts/)
  assert.doesNotMatch(template, /@click="openSkuCopyDrawer"/)
  assert.doesNotMatch(template, />分类设置</)
  assert.doesNotMatch(template, /v-for="primary in visibleCategoryManagementTreeForSkuContext"/)
  assert.doesNotMatch(template, /class="category-panel category-drawer-panel category-management-panel"/)
  assert.doesNotMatch(template, /<aside class="settings-drawer sku-copy-drawer"/)
  assert.doesNotMatch(template, /当前SKU \{\{ skuDisplayTotal \}\}/)
  assert.match(template, /:total="skuDisplayTotal"/)
  assert.match(template, /<table :key="skuTableKey" class="sku-table"/)
  assert.match(template, /v-for="group in displaySkuGroups"/)
  assert.match(template, /v-for="row in group\.rows"/)
  assert.match(template, /v-if="!displaySkuRows\.length"/)
  assert.match(template, /:key="skuPaginationKey"/)
  assert.match(script, /const customerID = skuContextCustomerID\.value\s+return sortRowsForCustomerSkuPriority\(/)
  assert.match(script, /product\) => customerID > 0 && skuContextProductFilter\(product\)/)
  assert.match(script, /const currentSkuSourceRows = computed\(\(\) => \(/)
  assert.match(script, /skuContextCustomerID\.value > 0 \? customerSkuRows\.value : publicSkuRows\.value/)
  assert.match(script, /const skuVisibleTableState = computed\(\(\) => skuTableState\(currentSkuSourceRows\.value, skuFilters\.value, \{/)
  assert.match(script, /const normalizedSkuFilters = computed\(\(\) => skuVisibleTableState\.value\.filters\)/)
  assert.match(script, /const skuDisplayTotal = computed\(\(\) => skuVisibleTableState\.value\.total\)/)
  assert.match(script, /const skuDisplayKey = computed/)
  assert.match(script, /const skuTableKey = computed\(\(\) => `\$\{skuDisplayKey\.value\}:table`\)/)
  assert.match(script, /const skuPaginationKey = computed\(\(\) => `\$\{skuDisplayKey\.value\}:pagination`\)/)
  assert.match(script, /const displaySkuRows = computed\(\(\) => skuVisibleTableState\.value\.rows\)/)
  assert.match(script, /const skuPrimaryCategoryOptions = computed\(\(\) => skuVisibleTableState\.value\.primaryOptions\)/)
  assert.match(script, /const skuSecondaryCategoryOptions = computed\(\(\) => skuVisibleTableState\.value\.secondaryOptions\)/)
  assert.doesNotMatch(script, /const skuRenderRows = computed/)
  assert.doesNotMatch(script, /const skuRenderTotal = computed/)
  assert.match(script, /skuDisplayTotal\.value/)
  assert.match(script, /function syncVisibleSkuTableState\(\)/)
  assert.match(script, /const tableState = skuVisibleTableState\.value/)
  assert.doesNotMatch(script, /displaySkuRows\.value = pageState\.rows|const pageState = sliceVisibleSkuRows/)
  assert.match(script, /watch\(\[\s*publicSkuRows,\s*customerSkuRows,\s*skuFilters,\s*skuPage,\s*skuPageSize,\s*selectedCustomerSkuCustomerID,\s*\], syncVisibleSkuTableState, \{ deep: true, immediate: true \}\)/)
  assert.match(script, /applyWorkspaceCustomerContext\(\)\s+syncVisibleSkuTableState\(\)\s+pruneSelectedProducts\(displaySkuRows\.value\)/)
  assert.match(script, /await nextTick\(\)\s+syncVisibleSkuTableState\(\)\s+restoringProductSettingsDraft = false/)
  assert.doesNotMatch(script, /const skuTable = computed/)
  assert.doesNotMatch(source, /debug_sku_table|__kferpSkuTableDebug|skuTableDebugAttr|data-sku-debug|data-top-|data-sku-instance/)
  assert.match(style, /\.category-scroll-list\s*\{[^}]*max-height:\s*min\(640px,\s*calc\(100vh - 280px\)\);[^}]*overflow:\s*auto;/s)
  assert.match(style, /\.settings-drawer-mask\s*\{[^}]*position:\s*fixed;/s)
})

test('legacy SKU category management is not rendered as the product archive classification entry', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.doesNotMatch(template, /<Teleport\s+to="#sku-category-management-target"/)
  assert.doesNotMatch(template, /id="sku-category-management-target"/)
  assert.doesNotMatch(template, /currentSettingsSection === 'master'[\s\S]*class="category-panel category-drawer-panel category-management-panel"/)
  assert.match(template, /class="classification-view-toolbar product-classification-tabs"/)
  assert.match(template, /class="classification-view-toolbar alias-classification-tabs"/)
})

test('classification template page edits only template structure, not object assignments', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'classification-template-pane',
    'classification-template-list',
    'classification-category-editor',
    'classificationCategoryForm',
    'saveClassificationCategory',
    'moveClassificationCategory',
    'deleteClassificationCategory',
    '排序值越小越靠前',
  ]) {
    assert.ok(source.includes(expected), `missing classification template structure marker: ${expected}`)
  }

  assert.match(template, /activeConfigTemplateSection === 'classification-template'/)
  assert.match(template, /点击左侧模板后，在右侧维护分类项；商品归类在商品档案或客户商品名列表中完成/)
  assert.match(template, /<button class="secondary compact-action" type="button" @click="openClassificationTemplateCreateDrawer"/)
  assert.doesNotMatch(source, /category-editor-drawer/)
  assert.doesNotMatch(source, /openCategoryDrawer/)
  assert.doesNotMatch(source, /openCategorySettingsDrawer/)
  assert.doesNotMatch(template, /归属客户/)
  assert.doesNotMatch(template, /对象归类/)
  assert.doesNotMatch(template, /配置分类/)
  assert.match(script, /apiSend\(id \? `\/api\/product-classification-template-categories\/\$\{id\}` : '\/api\/product-classification-template-categories'/)
  assert.match(style, /\.classification-category-editor\s*\{/)
})

test('product and customer alias lists move selected rows within the active classification tab', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'saveSelectedProductClassificationAssignment',
    'saveSelectedAliasClassificationAssignment',
    'confirmProductClassificationTemplateUsage',
    'confirmAliasClassificationTemplateUsage',
    'confirmSelectedProductClassificationMove',
    'confirmSelectedAliasClassificationMove',
    '/api/product-classification-assignments/products',
    '/api/product-classification-assignments/customer-aliases',
    'currentProductClassificationTemplate',
    'currentAliasClassificationTemplate',
    'selectedProductClassificationCategoryID',
    'selectedAliasClassificationCategoryID',
    'UNCLASSIFIED_CATEGORY_MOVE_ID',
    'classificationMoveCategoryID',
  ]) {
    assert.ok(source.includes(expected), `missing classification assignment marker: ${expected}`)
  }

  assert.match(template, /product-classification-tabs[\s\S]*classification-tabs[\s\S]*product-classification-selects[\s\S]*增加分类[\s\S]*移动到分类/)
  assert.match(template, /alias-classification-tabs[\s\S]*classification-tabs[\s\S]*alias-classification-selects[\s\S]*增加分类[\s\S]*移动到分类/)
  assert.match(template, /SearchableSelect[\s\S]*placeholder="增加分类"[\s\S]*@select="confirmProductClassificationTemplateUsage"/)
  assert.match(template, /SearchableSelect[\s\S]*placeholder="移动到分类"[\s\S]*@select="confirmSelectedProductClassificationMove"/)
  assert.match(template, /SearchableSelect[\s\S]*placeholder="增加分类"[\s\S]*@select="confirmAliasClassificationTemplateUsage"/)
  assert.match(template, /SearchableSelect[\s\S]*placeholder="移动到分类"[\s\S]*@select="confirmSelectedAliasClassificationMove"/)
  assert.doesNotMatch(template, /move-classification-card/)
  assert.doesNotMatch(template, /add-classification-card/)
  assert.doesNotMatch(template, /classification-action-card/)
  assert.match(template, /product-filter-row[\s\S]*openProductDrawer[\s\S]*deactivateProducts/)
  assert.match(template, /alias-filter-row[\s\S]*openCustomerAliasCreateDrawer[\s\S]*batchDisableCustomerProductAliases/)
  assert.match(template, /v-for="group in displaySkuGroups"/)
  assert.match(template, /v-for="group in visibleCustomerAliasGroups"/)
  assert.match(style, /\.classification-group-row\s+td\s*\{/)
})

test('classification group rows support collapse and indentation in product and alias lists', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'toggleProductClassificationGroup',
    'toggleAliasClassificationGroup',
    'isProductClassificationGroupCollapsed',
    'isAliasClassificationGroupCollapsed',
    'classification-item-row',
    'classification-group-toggle',
  ]) {
    assert.ok(source.includes(expected), `missing classification group marker: ${expected}`)
  }

  assert.match(template, /isProductClassificationGroupCollapsed\(group\.key\)\s*\?\s*'展开'\s*:\s*'收起'/)
  assert.match(template, /isAliasClassificationGroupCollapsed\(group\.key\)\s*\?\s*'展开'\s*:\s*'收起'/)
  assert.match(style, /\.classification-item-row\s+td:first-child \+ td,[\s\S]*padding-left:/)
  assert.match(style, /\.classification-tab\.active\s*\{/)
})

test('product archive and customer alias pages enable classification templates as page-level tabs', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(source, /product_classification_template_usages/)
  assert.match(source, /aliasClassificationTemplateUsages/)
  assert.match(script, /apiGet\('\/api\/product-classification-template-usages\/products'\)/)
  assert.match(script, /apiGet\('\/api\/product-classification-template-usages\/customer-aliases'\)/)
  assert.match(script, /saveProductClassificationTemplateUsage/)
  assert.match(script, /saveAliasClassificationTemplateUsage/)
  assert.match(template, /productClassificationTabs/)
  assert.match(template, /aliasClassificationTabs/)
  assert.doesNotMatch(template, /复制为客户分类/)
})

test('SKU table groups rows by enabled classification template tabs without product type columns', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'sku-table-wrap',
    'class="sku-table"',
    'classification-view-toolbar',
    'product-classification-tabs',
    'classification-group-row',
    'sku-name-cell',
    'action-cell',
  ]) {
    assert.ok(source.includes(expected), `missing SKU table layout marker: ${expected}`)
  }
  assert.doesNotMatch(template, /<th class="sku-col-product-type">产品类型<\/th>/)
  assert.doesNotMatch(template, /<th class="sku-col-product-subtype">产品子类型<\/th>/)
  assert.match(template, /v-for="group in displaySkuGroups"/)
  assert.match(template, /classification-group-toggle/)
  assert.match(style, /\.sku-table-wrap\s*\{[^}]*overflow-x:\s*auto;/s)
  assert.match(style, /\.sku-table\s*\{[^}]*width:\s*max-content;[^}]*min-width:\s*1600px;/s)
  assert.match(style, /\.sku-table th,\s*\.sku-table td\s*\{[^}]*white-space:\s*nowrap;/s)
})

test('legacy product type category drag UI is not present in the new product archive template', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.doesNotMatch(template, /产品类型操作/)
  assert.doesNotMatch(template, /产品子类型操作/)
  assert.doesNotMatch(template, /拖入产品子类型后才参与产品价格表生成/)
  assert.doesNotMatch(template, /v-for="\(secondary, index\) in primary\.children"/)
})

test('SKU settings keeps product config template binding on the product record instead of categories', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.match(source, /productProductionConfigForm\.product_config_template_id/)
  assert.match(template, /商品配置模板/)
  assert.doesNotMatch(template, /class="template-select"[\s\S]*@change\.stop="bindProductConfigTemplateToSubtype/)
  assert.doesNotMatch(source, /bindProductConfigTemplateToSubtype/)
  assert.doesNotMatch(template, />更换商品配置</)
  assert.doesNotMatch(source, /startProductSubtypeConfigEdit/)
  assert.doesNotMatch(source, /saveProductSubtypeConfig/)
  assert.doesNotMatch(source, /editingSubtypeConfigId/)
})

test('customer product aliases support batch adding product records', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(template, /批量添加商品档案/)
  assert.match(source, /customerAliasCreateDrawerOpen/)
  assert.match(source, /customerAliasCreateMode/)
  assert.match(template, /customerAliasCreateMode === 'batch'/)
  assert.match(source, /selectedAliasBatchProductIds/)
  assert.match(script, /\/api\/customer-product-aliases\/batch/)
  assert.match(script, /buildCustomerProductAliasBatchPayload/)
  assert.doesNotMatch(source, /批量创建时客户商品名=商品档案名称，客户商品编号=商品编号/)
  assert.doesNotMatch(source, /aliasBatchForm\.customer_item_code/)
})

test('product archive BOM detail navigation uses SPA view events instead of hard refresh', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const appSource = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(source, /kferp:navigate-view/)
  assert.doesNotMatch(source, /window\.location\.href\s*=\s*buildProductBomURL/)
  assert.doesNotMatch(appSource, /isProductSettingsKey\(currentKey\.value\)[\s\S]{0,140}hardNavigateToView/)
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

  assert.match(template, /currentSettingsSection === 'templates'[\s\S]*商品配置/)
  assert.doesNotMatch(template, />模板配置</)
  assert.ok(
    template.indexOf('商品配置模板') < template.indexOf('阶梯价模板'),
    'product config template tab should be the first, frequent template tab before gradient templates',
  )
  assert.match(style, /\.config-template-tabs\s*\{[^}]*display:\s*inline-flex;/s)
})

test('SKU settings separates global unit templates into a peer configuration tab', () => {
  const menuSource = fs.readFileSync(new URL('./menu-ia.js', import.meta.url), 'utf8')
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const settingsSource = fs.readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const expected of [
    'unit-template-pane',
    'showUnitTemplatePane',
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
    menuSource.indexOf("label: '商品配置和分类模板'") < menuSource.indexOf("label: '阶梯价模板'")
      && menuSource.indexOf("label: '阶梯价模板'") < menuSource.indexOf("label: '单位模板'"),
    'unit and gradient templates should be peer product menu functions',
  )
  assert.doesNotMatch(template, /<div class="field-group-title">单位规则<\/div>[\s\S]*productConfigTemplateForm\.unit_conversion_rows/)
  assert.match(script, /buildProductConfigTemplatePayload\([\s\S]*unit_template_id/)
})

test('SKU product config uses display unit from unit dictionary instead of fixed display modes', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const productConfigPane = source.match(/<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'"[\s\S]*?<div v-if="productDrawerOpen"/)?.[0] || ''
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
  const productConfigPane = source.match(/<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'"[\s\S]*?<div v-if="productDrawerOpen"/)?.[0] || ''
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
  const unitTemplatePane = source.match(/<div v-show="showUnitTemplatePane"[\s\S]*?<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'classification-template'"/)?.[0] || ''

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
  const unitTemplatePane = source.match(/<div v-show="showUnitTemplatePane"[\s\S]*?<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'classification-template'"/)?.[0] || ''
  const globalUnitDrawer = source.match(/<div v-if="globalUnitDrawerOpen"[\s\S]*?<\/aside>\s*<\/div>/)?.[0] || ''

  for (const expected of [
    'sku-page-summary',
    'kferp:notify',
  ]) {
    assert.ok(source.includes(expected), `missing compact SKU context marker: ${expected}`)
  }

  assert.match(style, /\.sku-page-summary\s*\{[^}]*padding:\s*8px 12px;/s)
  assert.match(style, /\.sku-page-summary \.panel-head\s*\{[^}]*margin-bottom:\s*0;/s)
  assert.doesNotMatch(template, /SKU归属/)
  assert.doesNotMatch(template, /compact-sku-context/)
  assert.doesNotMatch(template, /<div v-if="error" class="error"/)
  assert.doesNotMatch(template, /<div v-if="ok" class="ok"/)
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
  const unitTemplatePane = source.match(/<div v-show="showUnitTemplatePane"[\s\S]*?<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'classification-template'"/)?.[0] || ''
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

test('product archive config drawer does not bind classification templates or direct category dropdowns', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const drawer = source.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''

  assert.ok(drawer, 'product archive config drawer should exist')
  assert.doesNotMatch(drawer, /classification_template_id/)
  assert.doesNotMatch(drawer, /配置分类/)
  assert.doesNotMatch(drawer, /openClassificationConfigDrawer/)
  assert.doesNotMatch(drawer, /product_subtype_category_id/)
  assert.doesNotMatch(drawer, />产品子类型</)
  assert.doesNotMatch(drawer, />分类<\/span>[\s\S]*<select/)
})

test('product archive industry fields are generated from templates without ad-hoc field definition editing', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const drawer = source.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''

  assert.ok(drawer, 'product archive config drawer should exist')
  assert.match(drawer, /行业字段/)
  assert.match(drawer, /industry_field_template_id/)
  assert.doesNotMatch(drawer, /行业字段值/)
  assert.doesNotMatch(drawer, /新增字段/)
  assert.doesNotMatch(drawer, /删除<\/button>/)
  assert.doesNotMatch(drawer, />字段名</)
  assert.doesNotMatch(drawer, />类型</)
})

test('product settings uses classification tabs and page-level assignment controls', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  for (const expected of [
    'productClassificationTabs',
    'aliasClassificationTabs',
    'activeProductClassificationTab',
    'activeAliasClassificationTab',
    'saveSelectedProductClassificationAssignment',
    'saveSelectedAliasClassificationAssignment',
    '未分类商品',
    '未分类客户商品',
    '增加分类',
    '移动到分类',
  ]) {
    assert.ok(source.includes(expected), `missing classification tab marker: ${expected}`)
  }
  assert.doesNotMatch(source, /classification-config-drawer/)
  assert.doesNotMatch(source, /aria-label="分类配置"/)
})

test('customer product aliases use page-level classification templates, not single or batch fields', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const aliasDrawer = source.match(/<aside class="settings-drawer customer-alias-create-drawer"[\s\S]*?<\/aside>/)?.[0] || ''
  const aliasForm = aliasDrawer.match(/<form class="customer-alias-form"[\s\S]*?<\/form>/)?.[0] || ''
  const aliasBatchMode = aliasDrawer.match(/<div v-else class="customer-alias-batch-mode"[\s\S]*?<\/div>\s*<\/div>\s*<div v-if="customerAliasCreateMode === 'batch'"/)?.[0] || ''
  const aliasTable = source.match(/<table class="customer-alias-table"[\s\S]*?<\/table>/)?.[0] || ''

  assert.doesNotMatch(aliasForm, /classification_template_id/)
  assert.doesNotMatch(aliasForm, /include_in_price_list/)
  assert.doesNotMatch(aliasDrawer, /aliasBatchForm\.classification_template_id/)
  assert.doesNotMatch(aliasDrawer, />默认进入价格表</)
  assert.match(aliasBatchMode, /alias-batch-list-filters[\s\S]*aliasBatchFilters\.query[\s\S]*alias-batch-table/)
  assert.doesNotMatch(aliasDrawer, /默认复制\/复用商品档案分类模板/)
  assert.match(aliasDrawer, /批量添加商品档案/)
  assert.doesNotMatch(source, /customer-alias-batch-drawer/)
  assert.doesNotMatch(aliasTable, />展示分类</)
  assert.doesNotMatch(aliasTable, /navigateProductBom/)
  assert.doesNotMatch(source, /openClassificationConfigDrawer\(\{[\s\S]*objectType:\s*'customer_alias'/)
})

test('product menus split config, gradient, unit templates and rename product price list', () => {
  const menuSource = fs.readFileSync(new URL('./menu-ia.js', import.meta.url), 'utf8')
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const costingSource = fs.readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
  const configWorkspace = source.match(/<div v-show="currentSettingsSection === 'templates'"[\s\S]*?<div v-if="classificationTemplateCreateDrawerOpen"/)?.[0] || ''

  for (const expected of [
    "label: '商品配置和分类模板'",
    "key: 'pricingGradientTemplates'",
    "label: '阶梯价模板'",
    "key: 'productUnitTemplates'",
    "label: '单位模板'",
    "label: '商品价格表'",
  ]) {
    assert.match(menuSource, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.doesNotMatch(menuSource, /label: '产品价格表'/)
  assert.match(costingSource, /<h2>商品价格表<\/h2>/)
  assert.doesNotMatch(costingSource, /<h2>产品价格表<\/h2>/)
  assert.match(configWorkspace, /商品配置模板/)
  assert.match(configWorkspace, /分类模板/)
  assert.doesNotMatch(configWorkspace, /activeConfigTemplateSection === 'gradient'/)
  assert.doesNotMatch(configWorkspace, /activeConfigTemplateSection === 'unit-template'/)
})

test('classification templates and categories reference gradient and unit templates', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const classificationPane = source.match(/<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'classification-template'"[\s\S]*?<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'"/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(classificationPane, /模板默认阶梯价模板/)
  assert.match(classificationPane, /模板默认单位模板/)
  assert.match(classificationPane, /分类项阶梯价模板/)
  assert.match(classificationPane, /分类项单位模板/)
  assert.match(script, /classificationTemplateForm\.value[\s\S]*gradient_template_id/)
  assert.match(script, /classificationTemplateForm\.value[\s\S]*unit_template_id/)
  assert.match(script, /classificationCategoryForm\.value[\s\S]*gradient_template_id/)
  assert.match(script, /classificationCategoryForm\.value[\s\S]*unit_template_id/)
})

test('classification assignment helpers allow direct move overwrite and expose labels', () => {
  const templates = [{
    id: 10,
    name: '报价分类',
    categories: [{ id: 11, name: '意式拼配' }],
    product_assignments: [{ product_id: 88, template_id: 10, category_id: 11 }],
  }]

  assert.equal(classificationAssignmentLabel({ id: 88 }, templates, { assignmentType: 'product' }), '报价分类 / 意式拼配')
  assert.equal(classificationAssignmentLabel({ id: 89 }, templates, { assignmentType: 'product' }), '未分类')
  assert.equal(classificationAssignmentConflict({ id: 88 }, 10, templates, { assignmentType: 'product' }), null)
  assert.equal(classificationAssignmentConflict({ id: 88 }, 10, templates, { assignmentType: 'product', categoryID: 11 }), null)
  assert.equal(classificationAssignmentConflict({ id: 88 }, 12, templates, { assignmentType: 'product' }), null)
  assert.equal(classificationAssignmentConflict({ id: 89 }, 10, templates, { assignmentType: 'product' }), null)
})

test('product archive and customer alias classification UX uses big-category tabs and current classification labels', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(source, /未分类商品/)
  assert.match(source, /未分类客户商品/)
  assert.match(source, /增加分类/)
  assert.match(source, /移动到分类/)
  assert.match(source, /classification-select-row/)
  assert.match(source, /productAddClassificationOptions/)
  assert.match(source, /aliasAddClassificationOptions/)
  assert.match(template, /当前归类/)
  assert.match(script, /selectedProductRowsAlreadyInCurrentCategory/)
  assert.match(script, /selectedAliasRowsAlreadyInCurrentCategory/)
  assert.doesNotMatch(source, /已归类，需先移出当前分类/)
})

test('classification template unit and price mismatch warnings compare product config with assigned category', () => {
  const warnings = classificationTemplateUnitPriceWarnings({
    productConfigTemplate: { id: 7, gradient_template_id: 100, unit_template_id: 200 },
    classificationTemplate: { id: 10, gradient_template_id: 101, unit_template_id: 201 },
    classificationCategory: { id: 11, gradient_template_id: 102, unit_template_id: 202 },
  })
  assert.deepEqual(warnings, [
    '商品配置阶梯价模板与所属分类引用不一致',
    '商品配置单位模板与所属分类引用不一致',
  ])
  assert.deepEqual(classificationTemplateUnitPriceWarnings({
    productConfigTemplate: { id: 7, gradient_template_id: 102, unit_template_id: 202 },
    classificationTemplate: { id: 10, gradient_template_id: 101, unit_template_id: 201 },
    classificationCategory: { id: 11, gradient_template_id: 102, unit_template_id: 202 },
  }), [])
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
