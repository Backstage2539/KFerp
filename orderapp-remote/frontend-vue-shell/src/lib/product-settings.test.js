import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

import * as productSettings from './product-settings.js'
import {
  orderProductFamilyOptions,
  orderProductKindFilterOptions,
} from './order-entry.js'

import {
  buildAssignCategoryPayload,
  buildCustomerProductRuleBindingPayload,
  buildCustomerProductRuleOverridePayload,
  buildCustomerProductRuleTemplatePayload,
  buildCustomerProductAliasPayload,
  buildCustomerProductAliasBatchPayload,
  customerAliasEffectiveDisplayName,
  buildBusinessGroupAssignmentPayload,
  buildProductCustomerReferencePayload,
  activeProductionBomOptions,
  buildClassificationTemplateUsagePayload,
  buildCustomerProductAliasIndustryFieldPayload,
  classificationAssignmentConflict,
  classificationAssignmentLabel,
  classificationTemplateUnitPriceWarnings,
  productCategoryAssignmentLabel,
  businessGroupAssignmentLabel,
  businessGroupDisplayGroups,
  businessGroupItemMoveOptions,
  businessGroupItemsTree,
  productCatalogGroupOfProduct,
  customerProductAliasRowsForCustomer,
  industryFieldOptionsJSONFromText,
  industryFieldOptionsTextFromJSON,
  industryFieldSummary,
  industryFieldTemplateOptionsForConfig,
  classificationTemplateTabs,
  groupRowsByClassificationCategory,
  buildCustomerPublicUsagePayload,
  buildCustomProductCreatePayload,
  buildProductCategoryConfigPayload,
  buildProductConfigTemplatePayload,
  buildPriceTableRowsFromTemplateResolution,
  buildPriceTierTemplatePayload,
  buildProductPriceRecordPayload,
  buildProductTierPriceSchemePayload,
  buildPricingRulePayload,
  buildPricingRuleCopyPayload,
  buildPricingRuleUpdateFromTrial,
  buildPricingRuleTrialPayload,
  applyPricingRuleTrialToPriceTableRow,
  priceTablePricingRuleTrialPayload,
  buildProductProductionConfigForm,
  productProductionConfigFieldsFromTemplate,
  buildProductProductionConfigBasicsPayload,
  buildProductUnitDefinitionPayload,
  buildProductUnitTemplatePayload,
  buildProductBasicsPayload,
  buildProductBomURL,
  buildChildSkuCreatePayload,
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
  productionBomOptionLabel,
  productCodeLabel,
  priceListRuleFormFromJSON,
  priceListRuleJSONFromForm,
  productBelongsToSkuContext,
  productConfigTemplateBelongsToSkuContext,
  productConfigTemplateNeedsGradientTemplate,
  productCreationActionOptions,
  productDisplayState,
  productPriceRecordLabel,
  pricingRuleTrialDefaultQuoteUnit,
  pricingRuleTrialQuoteUnitOptionsForProduct,
  productCurrentSalesSpecUnit,
  priceTierTemplateRowKey,
  priceTierTemplateUnitCompatibility,
  priceTablePricingResolutionWarning,
  resolvePriceTableTemplateInheritance,
  resolveCreatedProductForConfig,
  productSubtypeCategoryOptionsForType,
  primaryCategoryOptions,
  secondaryCategoryOptions,
  specialAttrSchemaJSONFromRows,
  specialAttrSchemaRowsFromJSON,
  specialAttrValuesFromJSON,
  specialAttrValuesJSONFromForm,
  sortRowsForCustomerSkuPriority,
  productSkuRowsForParent,
  productArchiveRowsWithSkus,
  skuListRowsFromProducts,
  skuGroupTableState,
  skuTableState,
  skuTypeLabel,
  skuTypeOptions,
  unitConversionJSONFromRows,
  unitConversionRowsFromJSON,
  unitRuleFormFromJSON,
  unitRuleJSONFromForm,
  visibleSkuGroupRows,
  visibleNonDeletedRows,
  salesSpecConversionLabel,
  salesSpecRowsFromTemplate,
  selectedSkuRowIDsAfterVisibleToggle,
  storePricingRuleTrialReturnState,
  takePricingRuleTrialReturnState,
} from './product-settings.js'

const rows = [
  { id: 1, name: '乌拉嘎 熟豆', product_kind: 'roasted', primary_name: '咖啡豆', secondary_name: '单品豆', custom_type: '', remark: '常规 SKU' },
  { id: 2, name: '埃塞瑰夏 生豆', product_kind: 'green_bean', primary_name: '生豆', secondary_name: '单品生豆', green_bean_type: 'single_origin', custom_type: 'public_sku_alias', remark: '客户改名' },
  { id: 3, name: '拼配生豆 A', product_kind: 'green_bean', primary_name: '生豆', secondary_name: '拼配生豆', green_bean_type: 'blend', custom_type: 'custom_blend', remark: '特殊拼配说明' },
]

test('current product and pricing writes ignore legacy overall yield and loss fields', () => {
  const productCreate = buildProductCreatePayload({
    name: '曲奇拼配',
    product_kind: 'roasted',
    yield_percent: 80,
  })
  const productUpdate = buildProductBasicsPayload({
    name: '曲奇拼配',
    product_kind: 'roasted',
    yield_percent: 75,
  })
  const customCreate = buildCustomProductCreatePayload(42, {
    name: '客户曲奇拼配',
    product_kind: 'roasted',
    yield_percent: 70,
  })
  assert.equal(Object.hasOwn(productCreate, 'yield_rate'), false)
  assert.equal(Object.hasOwn(productUpdate, 'yield_rate'), false)
  assert.equal(Object.hasOwn(customCreate, 'yield_rate'), false)

  const pricing = buildPricingRulePayload({
    name: '熟豆磅装模板',
    calculation_json: { yield_loss_mode: 'manual', expected_loss_rate: 0.2 },
    yield_loss_mode: 'bom_or_product',
  })
  assert.equal(pricing.calculation_json.yield_loss_mode, 'none')
  assert.equal(Object.hasOwn(pricing.calculation_json, 'expected_loss_rate'), false)

  const trial = buildPricingRuleTrialPayload({
    pricing_rule_id: 12,
    product_id: 643,
    expected_loss_rate: 0.2,
    margin_rate: 0.3,
  })
  assert.equal(Object.hasOwn(trial.overrides, 'expected_loss_rate'), false)

  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const saveConfigBlock = source.match(/async function saveProductProductionConfig\(\)[\s\S]*?\n}\n\nasync function createSku/)?.[0] || ''
  assert.doesNotMatch(source, /pricingRuleForm\.yield_loss_mode/)
  assert.doesNotMatch(source, /pricingRuleTrialForm\.expected_loss_rate/)
  assert.doesNotMatch(source, /<span>损耗\/出率<\/span>/)
  assert.doesNotMatch(source, /<span>临时损耗率<\/span>/)
  assert.doesNotMatch(source, /yield_percent:/)
  assert.doesNotMatch(source, /customForm\.value\.yield_percent/)
  assert.doesNotMatch(saveConfigBlock, /expected_loss_rate/)

  const configForm = buildProductProductionConfigForm({ expected_loss_rate: 0.2 }, { id: 643, yield_rate: 0.8 })
  assert.equal(Object.hasOwn(configForm, 'expected_loss_percent'), false)
})

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

test('instant coffee product kind ignores legacy yield and SKU special attributes on write', () => {
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

test('child SKU create payload carries parent product and concrete spec fields', () => {
  const payload = buildChildSkuCreatePayload(88, {
    name: '埃塞俄比亚 水洗 227g袋装',
    sku_name: ' 227g袋装 ',
    sku_code: ' ETH-227 ',
    barcode: ' 690000000227 ',
    spec_label: ' 227g ',
    net_content_qty: '227',
    net_content_unit: 'g',
    unit_template_id: '12',
    active: true,
    unit_conversion_rows: [{ from_unit: '箱', from_qty: 1, to_unit: '袋', to_qty: 12 }],
  })

  assert.deepEqual(payload, {
    parent_product_id: 88,
    name: '埃塞俄比亚 水洗 227g袋装',
    sku_name: '227g袋装',
    sku_code: 'ETH-227',
    barcode: '690000000227',
    spec_label: '227g',
    net_content_qty: 227,
    net_content_unit: 'g',
    unit_template_id: 12,
    active: true,
  })
})

test('product SKU rows group child SKUs under the selected parent product', () => {
  const products = [
    { id: 88, sku_id: 88, name: '埃塞俄比亚 水洗', parent_product_id: 0, sku_name: '默认规格', is_default_sku: true },
    { id: 101, sku_id: 101, name: '埃塞俄比亚 水洗 227g袋装', parent_product_id: 88, sku_name: '227g袋装', spec_label: '227g' },
    { id: 102, sku_id: 102, name: '埃塞俄比亚 水洗 100g袋装', parent_product_id: 88, sku_name: '100g袋装', spec_label: '100g' },
    { id: 201, sku_id: 201, name: '肯尼亚 水洗 227g袋装', parent_product_id: 200, sku_name: '227g袋装' },
  ]

  assert.deepEqual(productSkuRowsForParent(products, 88).map((row) => [row.sku_id, row.sku_name, row.spec_label]), [
    [88, '默认规格', ''],
    [101, '227g袋装', '227g'],
    [102, '100g袋装', '100g'],
  ])
})

test('product SKU rows project the parent default_sku_id onto one concrete child SKU', () => {
  const products = [
    { id: 88, sku_id: 88, name: '初晓', parent_product_id: 0, default_sku_id: 102, is_default_sku: true },
    { id: 101, sku_id: 101, name: '初晓 227g', parent_product_id: 88, sku_name: '227g', is_default_sku: false },
    { id: 102, sku_id: 102, name: '初晓 454g', parent_product_id: 88, sku_name: '454g', is_default_sku: false },
  ]

  const rows = productSkuRowsForParent(products, 88)
  assert.equal(rows.find((row) => row.sku_id === 88)?.is_default_sku, false)
  assert.equal(rows.find((row) => row.sku_id === 101)?.is_default_sku, false)
  assert.equal(rows.find((row) => row.sku_id === 102)?.is_default_sku, true)
})

test('product archive configuration drops the per-spec default SKU action', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  assert.doesNotMatch(source, /设为默认规格/)
  assert.doesNotMatch(source, /setDefaultProductSalesSpec/)
  assert.match(source, /默认规格/)
})

test('product archive rows keep sales-spec SKUs inside one parent product row', () => {
  const products = [
    { id: 1, name: '金色山脉', parent_product_id: 0, product_code: 'SKU-000001', active: true },
    { id: 5, name: '金色山脉 100g袋装', parent_product_id: 1, sku_name: '100g袋装', spec_label: '100g', active: true },
    { id: 3, name: '金色山脉 227g袋装', parent_product_id: 1, sku_name: '227g袋装', spec_label: '227g', active: true },
    { id: 4, name: '金色山脉 Kg', parent_product_id: 1, sku_name: 'Kg', spec_label: 'Kg', active: true },
    { id: 2, name: '金色山脉 磅', parent_product_id: 1, sku_name: '磅', spec_label: '磅', active: true },
  ]

  const rows = productArchiveRowsWithSkus(products)

  assert.equal(rows.length, 1)
  assert.equal(rows[0].name, '金色山脉')
  assert.deepEqual(rows[0].sku_rows.map((row) => [row.id, row.sku_name]), [
    [5, '100g袋装'],
    [3, '227g袋装'],
    [4, 'Kg'],
    [2, '磅'],
  ])
  assert.match(rows[0].sku_search_text, /100g袋装/)
  assert.match(rows[0].sku_search_text, /SKU-000005/)
  assert.deepEqual(filterSkuRows(rows, { query: '227g袋装' }).map((row) => row.id), [1])
})

test('pricing rule trial picker only exposes unique active parent products and reuses order product-kind filtering', () => {
  const catalogProducts = [
    { id: 100, name: '晨曦拼配', parent_product_id: 0, product_kind: 'roasted', active: true },
    { id: 100, name: '晨曦拼配', parent_product_id: 0, product_kind: 'roasted', active: true },
    { id: 101, name: '晨曦拼配 227g', parent_product_id: 100, product_kind: 'roasted', active: true },
    { id: 200, name: '晨曦挂耳', parent_product_id: 0, product_kind: 'drip_bag', active: true },
    { id: 201, name: '晨曦挂耳 10g', parent_product_id: 200, product_kind: 'drip_bag', active: true },
    { id: 300, name: '失效生豆', parent_product_id: 0, product_kind: 'green_bean', active: false },
    { id: 400, name: '在售生豆', parent_product_id: 0, product_kind: 'green_bean', active: true },
    { id: 501, name: '孤立子规格', parent_product_id: 500, product_kind: 'instant_coffee', active: true },
  ]

  assert.equal(
    typeof productSettings.pricingRuleTrialMainProductOptions,
    'function',
    'price trial should own an explicit parent-product candidate helper',
  )
  const candidates = productSettings.pricingRuleTrialMainProductOptions?.(catalogProducts) || []

  assert.deepEqual(candidates.map((row) => row.id), [100, 200, 400])
  assert.equal(candidates.some((row) => Number(row.parent_product_id || 0) > 0), false)
  assert.deepEqual(filterSkuRows(candidates, { query: '227g' }).map((row) => row.id), [100])
  assert.deepEqual(orderProductKindFilterOptions(candidates), [
    { value: '', label: '全部' },
    { value: 'roasted', label: '熟豆' },
    { value: 'drip_bag', label: '挂耳' },
    { value: 'green_bean', label: '生豆' },
  ])
  assert.deepEqual(orderProductFamilyOptions(candidates, '', 'drip_bag').map((row) => row.id), [200])
})

test('pricing rule trial sales specs come only from the selected parent concrete SKUs', () => {
  const catalogProducts = [
    { id: 58, name: '曜石', parent_product_id: 0, default_sku_id: 560, default_sales_unit: '454g', active: true },
    { id: 558, name: '曜石 227g', parent_product_id: 58, sku_id: 558, spec_label: '227g', derived_sales_unit: '227g', active: true },
    { id: 559, name: '曜石 454g旧规格', parent_product_id: 58, sku_id: 559, spec_label: '454g旧', derived_sales_unit: '454g', derived_spec_status: 'template_removed', active: true },
    { id: 557, name: '曜石 1lb已停用', parent_product_id: 58, sku_id: 557, spec_label: '1lb已停用', derived_sales_unit: 'lb', derived_spec_status: 'template_disabled', active: true },
    { id: 560, name: '曜石 454g', parent_product_id: 58, sku_id: 560, spec_label: '454g', derived_sales_unit: '454g', is_default_sku: true, active: true },
    { id: 561, name: '曜石 礼盒A', parent_product_id: 58, sku_id: 561, spec_label: '礼盒A', derived_sales_unit: '盒', active: true },
    { id: 562, name: '曜石 礼盒B', parent_product_id: 58, sku_id: 562, spec_label: '礼盒B', derived_sales_unit: '盒', active: true },
    { id: 563, name: '曜石 停用规格', parent_product_id: 58, sku_id: 563, spec_label: '停用', derived_sales_unit: '袋', active: false },
    { id: 564, name: '曜石 100g', parent_product_id: 58, sku_id: 564, spec_label: '100g', derived_sales_unit: '100g', active: true },
    { id: 565, name: '曜石 500g', parent_product_id: 58, sku_id: 565, spec_label: '500g', derived_sales_unit: '500g', active: true },
    { id: 566, name: '曜石 1Kg', parent_product_id: 58, sku_id: 566, spec_label: '1Kg', derived_sales_unit: '1Kg', active: true },
    { id: 70, name: '晨曦', parent_product_id: 0, default_sales_unit: 'lb', active: true },
    { id: 80, name: '仅有失效子规格的历史商品', parent_product_id: 0, default_sales_unit: 'kg', active: true },
    { id: 801, name: '历史商品已停用规格', parent_product_id: 80, sku_id: 801, derived_sales_unit: '500g', derived_spec_status: 'template_disabled', active: true },
  ]

  assert.equal(typeof productSettings.pricingRuleTrialProductSpecOptions, 'function')
  assert.equal(typeof productSettings.pricingRuleTrialDefaultProductSpecID, 'function')
  assert.equal(typeof productSettings.pricingRuleTrialProductSpecUnit, 'function')

  const specs = productSettings.pricingRuleTrialProductSpecOptions?.(catalogProducts, 58) || []
  assert.deepEqual(specs.map((row) => row.sku_id), [560, 558, 561, 562, 564, 565, 566])
  assert.deepEqual(specs.map((row) => productSettings.pricingRuleTrialProductSpecUnit(row)), ['454g', '227g', '盒', '盒', '100g', '500g', '1Kg'])
  assert.equal(productSettings.pricingRuleTrialDefaultProductSpecID(specs), 560)
  assert.equal(specs.filter((row) => productSettings.pricingRuleTrialProductSpecUnit(row) === '盒').length, 2, 'same-unit SKUs remain separate sales specs')
  assert.deepEqual(filterSkuRows(productSettings.pricingRuleTrialMainProductOptions(catalogProducts), { query: '454g旧规格' }), [])
  assert.deepEqual(filterSkuRows(productSettings.pricingRuleTrialMainProductOptions(catalogProducts), { query: '1lb已停用' }), [])

  const legacy = productSettings.pricingRuleTrialProductSpecOptions?.(catalogProducts, 70) || []
  assert.deepEqual(legacy.map((row) => row.sku_id), [70])
  assert.equal(productSettings.pricingRuleTrialProductSpecUnit(legacy[0]), 'lb')

  const invalidChildFallback = productSettings.pricingRuleTrialProductSpecOptions?.(catalogProducts, 80) || []
  assert.deepEqual(invalidChildFallback.map((row) => row.sku_id), [80])
  assert.equal(productSettings.pricingRuleTrialProductSpecUnit(invalidChildFallback[0]), 'kg')
})

test('pricing rule trial payload submits the selected concrete SKU while the parent remains UI-only context', () => {
  const payload = buildPricingRuleTrialPayload({
    pricing_rule_id: 15,
    parent_product_id: 58,
    product_id: 560,
    quote_unit: '454g',
    other_cost_rows: [],
  })

  assert.equal(payload.product_id, 560)
  assert.equal(payload.quote_unit, '454g')
})

test('product archive page renders child SKUs inside the parent name cell instead of peer product rows', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.match(source, /productArchiveRowsWithSkus\(publicSkuRowsRaw\.value\)/)
  assert.match(template, /class="product-spec-skus"/)
  assert.match(template, /v-for="sku in row\.sku_rows"/)
  assert.match(template, /\{\{ row\.sku_rows\.length \}\} 个规格 SKU/)
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
    gradient_template_id: '18',
    unit_template_id: '22',
    product_config_template_id: '31',
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

test('customer product aliases no longer submit template or pricing overrides', () => {
  const payload = buildCustomerProductAliasPayload({
    customer_id: 42,
    product_id: 88,
    display_name: 'Karen 精品拼配',
    product_config_template_id: 31,
    gradient_template_id: 18,
    unit_template_id: 22,
    classification_template_id: 91,
  })

  assert.equal(Object.hasOwn(payload, 'product_config_template_id'), false)
  assert.equal(Object.hasOwn(payload, 'gradient_template_id'), false)
  assert.equal(Object.hasOwn(payload, 'unit_template_id'), false)
  assert.equal(Object.hasOwn(payload, 'classification_template_id'), false)
})

test('product settings view exposes group and pricing rule management while retiring customer product and old template entry points', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const appSource = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
  const menuSource = fs.readFileSync(new URL('./menu-ia.js', import.meta.url), 'utf8')
  const productCreateDrawer = source.slice(source.indexOf('product-editor-drawer'), source.indexOf('customer-alias-create-drawer'))
  const aliasCreateDrawer = source.slice(source.indexOf('customer-alias-create-drawer'), source.indexOf('product-production-config-drawer'))
  const productConfigDrawer = source.slice(source.indexOf('product-production-config-drawer'), source.indexOf('<script setup>'))
  const normalFormSurface = [productCreateDrawer, aliasCreateDrawer, productConfigDrawer].join('\n')

  for (const want of ['商品价格管理', '价格计算模板', '客户引用', '价格摘要', '暂无价格表价格', '库存单位', '整数库存']) {
    assert.match(source, new RegExp(want))
  }
  assert.match(appSource, /productPriceManagement/)
  assert.match(appSource, /groupManagement/)
  assert.match(menuSource, /key: 'businessSettings', label: '业务设置'/)
  assert.match(menuSource, /groupManagement:\s*'分组模板'/)
  assert.match(menuSource, /key: 'productPriceManagement', label: '商品价格管理'/)
  assert.doesNotMatch(menuSource, /key: 'customerProductAliases'/)
  assert.doesNotMatch(menuSource, /label: '客户商品'/)
  assert.doesNotMatch(menuSource, /label: '商品分类管理'/)
  assert.doesNotMatch(menuSource, /label: '商品配置和分类模板'/)
  assert.doesNotMatch(menuSource, /label: '单位模板'/)

  for (const forbidden of [
    '商品配置模板',
    '报价单位',
    '录单单位',
    '利润率覆盖',
  ]) {
    assert.doesNotMatch(normalFormSurface, new RegExp(forbidden))
  }
})

test('product customer references replace customer product master data for display only', () => {
  assert.deepEqual(buildProductCustomerReferencePayload({
    id: '12',
    product_id: '88',
    customer_id: '42',
    customer_item_code: ' KAREN-ESP ',
    customer_display_name: ' Karen 精品拼配 ',
    display_name: 'old alias',
    price_unit: 'kg',
    gradient_template_id: '18',
    product_config_template_id: '31',
    active: false,
    remark: ' 客户自己的叫法 ',
  }), {
    id: 12,
    product_id: 88,
    customer_id: 42,
    customer_item_code: 'KAREN-ESP',
    customer_display_name: 'Karen 精品拼配',
    active: false,
    remark: '客户自己的叫法',
  })
})

test('business group assignment payload supports products, BOMs, warehouses, and display labels', () => {
  assert.deepEqual(buildBusinessGroupAssignmentPayload({
    id: '9',
    usage_key: 'product_catalog',
    object_key: 'product',
    object_id: '88',
    object_ref: 'SHOULD_NOT_BE_USED',
    group_id: '5',
    group_item_id: '51',
    sort_order: '20',
  }), {
    id: 9,
    usage_key: 'product_catalog',
    object_key: 'product',
    object_id: 88,
    object_ref: '',
    group_id: 5,
    group_item_id: 51,
    sort_order: 20,
  })

  assert.deepEqual(buildBusinessGroupAssignmentPayload({
    usageKey: 'warehouse_inventory',
    objectKey: 'warehouse',
    objectRef: ' finished_goods ',
    object_id: 0,
    groupID: 7,
    groupItemID: 71,
  }), {
    id: 0,
    usage_key: 'warehouse_inventory',
    object_key: 'warehouse',
    object_id: 0,
    object_ref: 'finished_goods',
    group_id: 7,
    group_item_id: 71,
    sort_order: 100,
  })

  const group = {
    id: 5,
    name: '商品业务线',
    items: [
      { id: 50, name: '咖啡', parent_id: 0 },
      { id: 51, name: '熟豆', parent_id: 50 },
    ],
  }
  assert.equal(businessGroupAssignmentLabel({ group_id: 5, group_item_id: 51 }, [group]), '商品业务线 / 咖啡 / 熟豆')
  assert.equal(businessGroupAssignmentLabel({ group_id: 5, group_item_id: 51 }, [group], { includeGroupName: false }), '咖啡 / 熟豆')
  assert.equal(productCatalogGroupOfProduct({ id: 88 }, [
    { usage_key: 'product_catalog', object_key: 'product', object_id: 88, group_id: 5, group_item_id: 51 },
  ], [group]).label, '商品业务线 / 咖啡 / 熟豆')

  const systemGroup = {
    id: 6,
    name: '商品默认分组',
    code: 'default_product_catalog',
    active: true,
    usages: [{ usage_key: 'product_catalog', active: true }],
    items: [
      { id: 60, group_id: 6, parent_id: 0, name: '咖啡熟豆', active: true, sort_order: 10 },
      { id: 61, group_id: 6, parent_id: 60, name: '意式拼配', active: true, sort_order: 20 },
    ],
  }
  assert.equal(businessGroupAssignmentLabel({ group_id: 6, group_item_id: 61 }, [systemGroup]), '未分组')
  assert.equal(businessGroupAssignmentLabel({ group_id: 6, group_item_id: 0 }, [systemGroup]), '未分组')
  assert.deepEqual(businessGroupItemMoveOptions([systemGroup], 'product_catalog').map((option) => option.label), [])
  assert.deepEqual(businessGroupItemMoveOptions([{
    id: 7,
    name: '生产线分组',
    active: true,
    usages: [{ usage_key: 'product_catalog', active: true }],
    items: [
      { id: 70, group_id: 7, parent_id: 0, name: '速溶线', active: true, sort_order: 10 },
    ],
  }], 'production_bom', { includeGroupsWithoutUsage: true }).map((option) => option.label), ['生产线分组 / 速溶线'])
  assert.deepEqual(businessGroupItemMoveOptions([{
    id: 8,
    name: '商品分组',
    active: true,
    usages: [{ usage_key: 'production_bom', active: true }],
    items: [
      { id: 80, group_id: 8, parent_id: 0, name: 'BOM-咖啡熟豆', active: true, sort_order: 10 },
      { id: 81, group_id: 8, parent_id: 80, name: '拼配豆', active: true, sort_order: 20 },
    ],
  }], 'production_bom', { includeGroupName: false }).map((option) => option.label), ['BOM-咖啡熟豆', 'BOM-咖啡熟豆 / 拼配豆'])
  assert.deepEqual(businessGroupDisplayGroups([
    { id: 88, name: '商品A' },
    { id: 89, name: '商品B' },
  ], [
    { usage_key: 'product_catalog', object_key: 'product', object_id: 88, group_id: 6, group_item_id: 61 },
  ], [systemGroup]).map((row) => ({ label: row.label, count: row.rows.length })), [
    { label: '未分组', count: 2 },
  ])
  assert.deepEqual(businessGroupItemMoveOptions([{
    id: 9,
    name: '商品分组',
    active: true,
    usages: [{ usage_key: 'product_catalog', active: true }],
    items: [
      { id: 90, group_id: 9, parent_id: 0, name: '商品-咖啡熟豆', active: true, sort_order: 10 },
      { id: 91, group_id: 9, parent_id: 90, name: '意式拼配豆', active: true, sort_order: 20 },
    ],
  }], 'product_catalog', { includeGroupName: false }).map((option) => ({
    label: option.label,
    depth: option.depth,
    parent: option.parent_group_item_id,
  })), [
    { label: '商品-咖啡熟豆', depth: 0, parent: 0 },
    { label: '商品-咖啡熟豆 / 意式拼配豆', depth: 1, parent: 90 },
  ])
  assert.deepEqual(businessGroupDisplayGroups([
    { id: 90, name: '熟豆商品' },
    { id: 91, name: '意式商品' },
  ], [
    { usage_key: 'product_catalog', object_key: 'product', object_id: 90, group_id: 9, group_item_id: 90 },
    { usage_key: 'product_catalog', object_key: 'product', object_id: 91, group_id: 9, group_item_id: 91 },
  ], [{
    id: 9,
    name: '商品分组',
    active: true,
    usages: [{ usage_key: 'product_catalog', active: true }],
    items: [
      { id: 90, group_id: 9, parent_id: 0, name: '商品-咖啡熟豆', active: true, sort_order: 10 },
      { id: 91, group_id: 9, parent_id: 90, name: '意式拼配豆', active: true, sort_order: 20 },
    ],
  }]).map((row) => ({
    label: row.label,
    path: row.path_label,
    depth: row.depth,
    count: row.rows.length,
  })), [
    { label: '商品-咖啡熟豆', path: '商品-咖啡熟豆', depth: 0, count: 1 },
    { label: '意式拼配豆', path: '商品-咖啡熟豆 / 意式拼配豆', depth: 1, count: 1 },
  ])
})

test('pricing rules and tier templates are independent templates used by price lists', () => {
  assert.deepEqual(buildPricingRulePayload({
    id: 5,
    name: ' 成本加成含税 ',
    code: ' PR-COST ',
    cost_source_mode: 'product_cost_context',
    margin_rate: '0.32',
    tax_rate: '0.06',
    rounding_mode: 'yuan',
    formula_version: ' v2 ',
    calculation_json: {
      cost_components: ['material', 'operation', 'packaging', 'logistics'],
      yield_loss_mode: 'bom_or_product',
      profit_method: 'gross_margin',
      tax_mode: 'tax_included',
      minimum_margin_rate: '0.18',
      trial_note: '选择商品、报价单位后试算',
      other_costs: { '认证费': '2.5' },
      tier_label: 'should be ignored',
    },
    other_cost_rows: [
      { key: ' 包装贴标 ', value: '1.25' },
      { key: '认证费', value: '2.5' },
      { key: '认证费', value: '3.5' },
      { key: '', value: '99' },
    ],
    product_id: 88,
    customer_product_alias_id: 99,
    final_unit_price: 88,
    tiers: [{ label: '10kg+' }],
    min_qty: 1,
    max_qty: 10,
    tier_label: '1kg+',
    active: false,
    remark: ' 用 BOM 和工艺成本试算 ',
  }), {
    id: 5,
    name: '成本加成含税',
    code: 'PR-COST',
    cost_source_mode: 'bom_current_cost',
    margin_rate: 0.32,
    tax_rate: 0.06,
    rounding_mode: 'yuan',
    formula_version: 'v2',
    calculation_json: {
      yield_loss_mode: 'none',
      profit_method: 'markup',
      tax_mode: 'tax_included',
      minimum_margin_rate: 0.18,
      trial_note: '选择商品、报价单位后试算',
      other_costs: {
        '包装贴标': 1.25,
        '认证费': 3.5,
      },
    },
    active: false,
    remark: '用 BOM 和工艺成本试算',
  })

  assert.deepEqual(buildPriceTierTemplatePayload({
    id: 7,
    name: ' 批发阶梯 ',
    product_id: 88,
    final_unit_price: 88,
    cost_formula: 'cost_plus',
    tiers: [
      { label: '10kg+', min_qty: '10', max_qty: '', quantity_unit: ' kg ', position: 2, pricing_rule_id: '20', final_unit_price: 66 },
      { label: '1kg+', min_qty: '1', max_qty: '9', quantity_unit: 'kg', position: 1, pricing_rule_id: '10' },
    ],
  }), {
    id: 7,
    name: '批发阶梯',
    active: true,
    remark: '',
    tiers: [
      { label: '1kg+', min_qty: 1, max_qty: 9, quantity_unit: 'sales_spec_count', pricing_rule_id: 10, position: 1, active: true, remark: '' },
      { label: '10kg+', min_qty: 10, max_qty: null, quantity_unit: 'sales_spec_count', pricing_rule_id: 20, position: 2, active: true, remark: '' },
    ],
  })
})

test('pricing trial updates only reusable template parameters', () => {
  const rule = {
    id: 5,
    name: '熟豆24磅模板-正常-418',
    code: 'RULE-418',
    cost_source_mode: 'bom_current_cost',
    margin_rate: 0.2,
    tax_rate: 0.06,
    rounding_mode: 'jiao',
    formula_version: 'v2',
    calculation_json: {
      profit_method: 'markup',
      tax_mode: 'tax_included',
      minimum_margin_rate: 0.1,
      trial_note: '保留说明',
      other_costs: { '旧成本': 1 },
    },
    active: true,
    remark: '保留备注',
  }
  const trial = {
    margin_rate: '0.35',
    tax_rate: '',
    other_cost_rows: [{ key: '包装贴标', value: '1.25' }],
    parent_product_id: 659,
    product_id: 660,
    customer_id: 12,
    bom_version_id: 1797,
    process_route_id: 8,
    operation_template_id: 9,
    quote_unit: 'kg',
  }
  const payload = buildPricingRuleUpdateFromTrial(rule, trial)
  assert.equal(payload.margin_rate, 0.35)
  assert.equal(payload.tax_rate, 0.06, 'blank trial tax keeps the template tax')
  assert.deepEqual(payload.calculation_json.other_costs, { '包装贴标': 1.25 })
  assert.equal(payload.calculation_json.tax_mode, 'tax_included')
  assert.equal(payload.calculation_json.minimum_margin_rate, 0.1)
  assert.equal(payload.calculation_json.trial_note, '保留说明')
  assert.equal(payload.rounding_mode, 'jiao')
  for (const forbidden of ['parent_product_id', 'product_id', 'customer_id', 'bom_version_id', 'process_route_id', 'operation_template_id', 'quote_unit']) {
    assert.equal(Object.hasOwn(payload, forbidden), false, `${forbidden} must stay trial-only`)
  }

  const cleared = buildPricingRuleUpdateFromTrial(rule, {
    ...trial,
    tax_rate: 0,
    other_cost_rows: [{ key: '', value: 0 }],
  })
  assert.equal(cleared.tax_rate, 0, 'an explicit zero clears template tax')
  assert.deepEqual(cleared.calculation_json.other_costs, {}, 'blank rows clear template other costs')
})

test('pricing trial BOM return state is frontend-memory only and one-time', () => {
  const snapshot = {
    form: { pricing_rule_id: 5, product_id: 660, bom_version_id: 1797, margin_rate: 0.35 },
    product_kind_filter: 'roasted',
  }
  const key = storePricingRuleTrialReturnState(snapshot)
  assert.match(key, /^pricing-rule-trial-return:/)
  snapshot.form.margin_rate = 9
  assert.deepEqual(takePricingRuleTrialReturnState(key), {
    form: { pricing_rule_id: 5, product_id: 660, bom_version_id: 1797, margin_rate: 0.35 },
    product_kind_filter: 'roasted',
  })
  assert.equal(takePricingRuleTrialReturnState(key), null)
})

test('pricing rule payload normalizes compatible legacy or missing profit methods to markup', () => {
  for (const profitMethod of [undefined, '', 'gross_margin', 'markup']) {
    const payload = buildPricingRulePayload({
      name: '统一加价模板',
      margin_rate: 0.8,
      calculation_json: profitMethod === undefined ? {} : { profit_method: profitMethod },
      profit_method: profitMethod,
    })
    assert.equal(payload.margin_rate, 0.8)
    assert.equal(payload.calculation_json.profit_method, 'markup')
  }
})

test('pricing rule payload preserves unsupported legacy methods so the API can reject unsafe reinterpretation', () => {
  for (const profitMethod of ['fixed_add', 'unexpected_method']) {
    const payload = buildPricingRulePayload({
      name: '旧方式待确认',
      margin_rate: 3,
      calculation_json: { profit_method: profitMethod },
      profit_method: 'markup',
    })
    assert.equal(payload.calculation_json.profit_method, profitMethod)
  }
})

test('pricing rule copy payload creates an active unique template from inactive source', () => {
  assert.deepEqual(buildPricingRuleCopyPayload({
    id: 5,
    name: '停用模板',
    code: 'RULE-OLD',
    cost_source_mode: 'bom_current_cost',
    margin_rate: 0.22,
    tax_rate: 0.13,
    rounding_mode: 'jiao',
    formula_version: 'v2',
    calculation_json: {
      yield_loss_mode: 'manual',
      profit_method: 'markup',
      tax_mode: 'tax_excluded',
      minimum_margin_rate: 0.15,
      other_costs: { '包装': 1.2 },
      trial_note: '复制后试算',
    },
    active: false,
    remark: '原模板已停用',
  }, [
    { name: '停用模板', code: 'RULE-OLD' },
    { name: '停用模板 复制', code: 'RULE-OLD-COPY' },
  ]), {
    id: 0,
    name: '停用模板 复制 2',
    code: 'RULE-OLD-COPY-2',
    cost_source_mode: 'bom_current_cost',
    margin_rate: 0.22,
    tax_rate: 0.13,
    rounding_mode: 'jiao',
    formula_version: 'v2',
    calculation_json: {
      yield_loss_mode: 'none',
      profit_method: 'markup',
      tax_mode: 'tax_excluded',
      minimum_margin_rate: 0.15,
      trial_note: '复制后试算',
      other_costs: { '包装': 1.2 },
    },
    active: true,
    remark: '原模板已停用',
  })
})

test('pricing rule trial payload is temporary and does not save price rows', () => {
  assert.deepEqual(buildPricingRuleTrialPayload({
    pricing_rule_id: '10',
    product_id: '549',
    customer_id: '',
    bom_version_id: '5392',
    process_route_id: '19',
    operation_template_id: '27',
    quote_unit: ' kg ',
    expected_loss_rate: '0.12',
    margin_rate: '0.30',
    tax_rate: '0.06',
    other_cost_rows: [
      { key: ' 包装贴标 ', value: '1.25' },
      { key: '认证费', value: '2.5' },
      { key: '', value: '99' },
    ],
    post_markup_cost_rows: [
      { key: ' 包装 ', value: '1.7' },
      { key: '产品损耗', value: '0.06' },
      { key: '利润税额', value: '1.1996' },
    ],
    final_unit_price: 88,
    price_rows: [{ final_unit_price: 88 }],
  }), {
    pricing_rule_id: 10,
    product_id: 549,
    customer_id: 0,
    bom_version_id: 5392,
    process_route_id: 19,
    operation_template_id: 27,
    quote_unit: 'kg',
    overrides: {
      margin_rate: 0.3,
      tax_rate: 0.06,
      other_costs: {
        '包装贴标': 1.25,
        '认证费': 2.5,
      },
    },
  })
})

test('pricing rule trial sales unit options only include units resolvable for the product', () => {
  const product = {
    id: 573,
    name: '棒巧拼配 227g袋装',
    auto_derived_sku: true,
    derived_sales_unit: '袋',
    default_sales_unit: 'kg',
    quote_unit: 'kg',
    inventory_unit: 'kg',
    sales_units: ['袋', '盒', 'kg'],
    unit_conversion_json: '{"袋":{"kg":0.227}}',
  }
  const globalUnits = [
    { code: 'kg', name: 'kg' },
    { code: 'g', name: 'g' },
    { code: '盒', name: '盒' },
    { code: '袋', name: '袋' },
    { code: '磅', name: '磅' },
    { code: '条', name: '条' },
  ]

  assert.deepEqual(pricingRuleTrialQuoteUnitOptionsForProduct(globalUnits, product).map((unit) => unit.code), ['袋', 'kg', 'g', '磅'])
  assert.equal(pricingRuleTrialDefaultQuoteUnit(product, globalUnits), '袋')
})

test('pricing rule trial resolves derived SKU sales unit from net content when conversion JSON is absent', () => {
  const product = {
    id: 574,
    name: '棒巧拼配 227g袋装',
    auto_derived_sku: true,
    derived_sales_unit: '袋',
    net_content_qty: 227,
    net_content_unit: 'g',
    inventory_unit: 'kg',
    quote_unit: 'kg',
    unit_conversion_json: '{}',
  }
  const globalUnits = [
    { code: 'kg', name: 'kg' },
    { code: 'g', name: 'g' },
    { code: '袋', name: '袋' },
    { code: '盒', name: '盒' },
  ]

  assert.deepEqual(pricingRuleTrialQuoteUnitOptionsForProduct(globalUnits, product).map((unit) => unit.code), ['袋', 'kg', 'g'])
  assert.equal(pricingRuleTrialDefaultQuoteUnit(product, globalUnits), '袋')
})

test('price table resolves pricing mode by parent product, subgroup, parent group, price list', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'fixed_price', tier_template_id: 1, pricing_rule_id: 10, fixed_unit_price: 99 },
    groupAssignments: [
      { group_item_id: 100, pricing_mode: 'pricing_rule', tier_template_id: 2, pricing_rule_id: 20, fixed_unit_price: 0, parent_group_item_id: 0 },
      { group_item_id: 101, pricing_mode: 'tier_template', tier_template_id: 3, pricing_rule_id: 0, fixed_unit_price: 0, parent_group_item_id: 100 },
    ],
    productOverrides: [
      { scope: 'parent_product', product_id: 88, parent_product_id: 88, group_item_id: 101, tier_template_id: 0, pricing_rule_id: 40 },
      { scope: 'sku', product_id: 88, sku_id: 88, parent_product_id: 88 },
    ],
    product: { id: 88, sku_id: 88, parent_product_id: 88, group_item_id: 101 },
  })

  assert.deepEqual(resolved, {
    pricing_mode: 'tier_template',
    pricing_mode_source: 'subgroup',
    pricing_mode_source_group_item_id: 101,
    pricing_mode_source_group_item_name: '',
    pricing_mode_source_group_depth: 0,
    tier_template_id: 3,
    tier_template_source: 'subgroup',
    pricing_rule_id: 40,
    pricing_rule_source: 'parent_product',
    fixed_unit_price: 0,
    fixed_unit_price_source: 'sku',
  })

  assert.equal(resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'fixed_price', fixed_unit_price: 99 },
    groupAssignments: [{ group_item_id: 101, fixed_unit_price: 88 }],
    productOverrides: [{ scope: 'sku', product_id: 88, sku_id: 88, parent_product_id: 88, fixed_unit_price: 59.92 }],
    product: { id: 88, sku_id: 88, parent_product_id: 88, group_item_id: 101 },
  }).fixed_unit_price, 59.92)

  assert.deepEqual(buildPriceTableRowsFromTemplateResolution({
    product: { id: 88, name: '初晓拼配', inventory_unit: 'kg', default_sales_unit: 'kg', unit_conversion_json: '{"kg":{"kg":1}}' },
    resolution: resolved,
    tierTemplate: {
      id: 3,
      name: '批发阶梯',
      tiers: [
        { id: 31, label: '1kg+', min_qty: 1, max_qty: 9, quantity_unit: 'kg', pricing_rule_id: 41 },
        { id: 32, label: '10kg+', min_qty: 10, max_qty: null, quantity_unit: 'kg', pricing_rule_id: 42 },
      ],
    },
    pricingRule: { id: 40, name: '成本加成' },
    pricingRulesByID: {
      41: { id: 41, code: 'PR-1KG' },
      42: { id: 42, code: 'PR-10KG' },
    },
    unitPriceByTier: { '1kg+': 88, '10kg+': 78 },
  }), [
    { product_id: 88, product_name: '初晓拼配', price_unit: 'kg', inventory_unit: 'kg', inventory_conversion_json: { kg: { kg: 1 } }, tier_label: '1kg+', min_qty: 1, max_qty: 9, final_unit_price: 88, pricing_mode: 'tier_template', pricing_mode_source: 'subgroup', tier_template_id: 3, tier_template_source: 'subgroup', template_tier_id: 31, pricing_rule_id: 41, pricing_rule_source: 'subgroup', pricing_rule_version: 'PR-1KG', tier_pricing_rule_id: 41, tier_pricing_rule_version: 'PR-1KG', quantity_basis: 'sales_spec_count', tier_quantity_unit: 'kg', tier_template_name: '批发阶梯' },
    { product_id: 88, product_name: '初晓拼配', price_unit: 'kg', inventory_unit: 'kg', inventory_conversion_json: { kg: { kg: 1 } }, tier_label: '10kg+', min_qty: 10, max_qty: null, final_unit_price: 78, pricing_mode: 'tier_template', pricing_mode_source: 'subgroup', tier_template_id: 3, tier_template_source: 'subgroup', template_tier_id: 32, pricing_rule_id: 42, pricing_rule_source: 'subgroup', pricing_rule_version: 'PR-10KG', tier_pricing_rule_id: 42, tier_pricing_rule_version: 'PR-10KG', quantity_basis: 'sales_spec_count', tier_quantity_unit: 'kg', tier_template_name: '批发阶梯' },
  ])
})

test('price table inheritance resolves shared parent pricing for every SKU and isolates fixed prices per SKU', () => {
  const common = {
    defaults: { pricing_mode: 'fixed_price', tier_template_id: 1, pricing_rule_id: 10, fixed_unit_price: 99 },
    groupAssignments: [
      { group_item_id: 100, pricing_mode: 'pricing_rule', tier_template_id: 2, pricing_rule_id: 20, fixed_unit_price: 77 },
      { group_item_id: 101, pricing_mode: 'tier_template', tier_template_id: 3, pricing_rule_id: 30, fixed_unit_price: 88, parent_group_item_id: 100 },
    ],
    productOverrides: [
      {
        scope: 'parent_product',
        parent_product_id: 500,
        pricing_mode: 'pricing_rule',
        tier_template_id: 4,
        pricing_rule_id: 40,
        fixed_unit_price: 199,
      },
      {
        scope: 'sku',
        sku_id: 501,
        parent_product_id: 500,
        fixed_unit_price: 59.92,
      },
      {
        scope: 'sku',
        product_id: 503,
        parent_product_id: 500,
        pricing_mode: 'tier_template',
        tier_template_id: 5,
        pricing_rule_id: 50,
      },
    ],
  }

  assert.deepEqual(resolvePriceTableTemplateInheritance({
    ...common,
    product: { id: 501, sku_id: 501, parent_product_id: 500, group_item_id: 101 },
  }), {
    pricing_mode: 'pricing_rule',
    pricing_mode_source: 'parent_product',
    pricing_mode_source_group_item_id: 0,
    pricing_mode_source_group_item_name: '',
    pricing_mode_source_group_depth: -1,
    tier_template_id: 4,
    tier_template_source: 'parent_product',
    pricing_rule_id: 40,
    pricing_rule_source: 'parent_product',
    fixed_unit_price: 59.92,
    fixed_unit_price_source: 'sku',
  })

  assert.deepEqual(resolvePriceTableTemplateInheritance({
    ...common,
    product: { id: 502, sku_id: 502, parent_product_id: 500, group_item_id: 101 },
  }), {
    pricing_mode: 'pricing_rule',
    pricing_mode_source: 'parent_product',
    pricing_mode_source_group_item_id: 0,
    pricing_mode_source_group_item_name: '',
    pricing_mode_source_group_depth: -1,
    tier_template_id: 4,
    tier_template_source: 'parent_product',
    pricing_rule_id: 40,
    pricing_rule_source: 'parent_product',
    fixed_unit_price: 0,
    fixed_unit_price_source: 'sku',
  })

  assert.deepEqual(resolvePriceTableTemplateInheritance({
    ...common,
    product: { id: 503, sku_id: 503, parent_product_id: 500, group_item_id: 101 },
  }), {
    pricing_mode: 'pricing_rule',
    pricing_mode_source: 'parent_product',
    pricing_mode_source_group_item_id: 0,
    pricing_mode_source_group_item_name: '',
    pricing_mode_source_group_depth: -1,
    tier_template_id: 4,
    tier_template_source: 'parent_product',
    pricing_rule_id: 40,
    pricing_rule_source: 'parent_product',
    fixed_unit_price: 0,
    fixed_unit_price_source: 'sku',
  })
})

test('shared parent pricing exposes inherited fixed mode while keeping two SKU amounts isolated', () => {
  const productOverrides = [
    { scope: 'sku', product_id: 501, sku_id: 501, parent_product_id: 500, fixed_unit_price: 59.92 },
    { scope: 'sku', product_id: 502, sku_id: 502, parent_product_id: 500, fixed_unit_price: 109.9 },
  ]
  const defaults = { pricing_mode: 'fixed_price' }

  const first = resolvePriceTableTemplateInheritance({
    defaults,
    productOverrides,
    product: { id: 501, sku_id: 501, parent_product_id: 500, group_item_id: 101 },
  })
  const second = resolvePriceTableTemplateInheritance({
    defaults,
    productOverrides,
    product: { id: 502, sku_id: 502, parent_product_id: 500, group_item_id: 101 },
  })

  assert.equal(first.pricing_mode, 'fixed_price')
  assert.equal(first.pricing_mode_source, 'default')
  assert.equal(first.fixed_unit_price, 59.92)
  assert.equal(second.pricing_mode, 'fixed_price')
  assert.equal(second.fixed_unit_price, 109.9)

  const categoryFixed = resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'tier_template', tier_template_id: 8 },
    groupAssignments: [{ group_item_id: 101, pricing_mode: 'fixed_price' }],
    productOverrides,
    product: { id: 501, sku_id: 501, parent_product_id: 500, group_item_id: 101 },
  })
  assert.equal(categoryFixed.pricing_mode, 'fixed_price')
  assert.equal(categoryFixed.pricing_mode_source, 'subgroup')
  assert.equal(categoryFixed.fixed_unit_price, 59.92)
})

test('price table inheritance walks the complete category ancestor chain and reports the effective category', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'tier_template', tier_template_id: 90 },
    groupAssignments: [
      { group_item_id: 10, group_item_name: '生豆', pricing_mode: 'fixed_price' },
      { group_item_id: 20, group_item_name: '产区', parent_group_item_id: 10 },
      { group_item_id: 30, group_item_name: '处理法', parent_group_item_id: 20 },
      { group_item_id: 40, group_item_name: '水洗', parent_group_item_id: 30 },
    ],
    productOverrides: [
      { scope: 'sku', sku_id: 401, fixed_unit_price: 86 },
    ],
    product: { id: 401, sku_id: 401, parent_product_id: 400, group_item_id: 40 },
  })

  assert.equal(resolved.pricing_mode, 'fixed_price')
  assert.equal(resolved.pricing_mode_source, 'parent_group')
  assert.equal(resolved.pricing_mode_source_group_item_id, 10)
  assert.equal(resolved.pricing_mode_source_group_item_name, '生豆')
  assert.equal(resolved.pricing_mode_source_group_depth, 3)
  assert.equal(resolved.fixed_unit_price, 86)
  assert.equal(resolved.fixed_unit_price_source, 'sku')
})

test('price table inheritance lets the nearest configured ancestor override a farther ancestor', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'tier_template', tier_template_id: 90 },
    groupAssignments: [
      { group_item_id: 10, group_item_name: '生豆', pricing_mode: 'fixed_price' },
      { group_item_id: 20, group_item_name: '产区', parent_group_item_id: 10, pricing_mode: 'pricing_rule', pricing_rule_id: 22 },
      { group_item_id: 30, group_item_name: '处理法', parent_group_item_id: 20 },
      { group_item_id: 40, group_item_name: '水洗', parent_group_item_id: 30 },
    ],
    product: { id: 401, sku_id: 401, parent_product_id: 400, group_item_id: 40 },
  })

  assert.equal(resolved.pricing_mode, 'pricing_rule')
  assert.equal(resolved.pricing_mode_source, 'parent_group')
  assert.equal(resolved.pricing_mode_source_group_item_id, 20)
  assert.equal(resolved.pricing_mode_source_group_item_name, '产区')
  assert.equal(resolved.pricing_mode_source_group_depth, 2)
  assert.equal(resolved.pricing_rule_id, 22)
  assert.equal(resolved.pricing_rule_source, 'parent_group')
})

test('price table inheritance coalesces duplicate category nodes without losing configured values or ancestry', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'tier_template', tier_template_id: 90 },
    groupAssignments: [
      { group_item_id: 10, group_item_name: '生豆', pricing_mode: '' },
      { group_item_id: 10, group_item_name: '', pricing_mode: 'fixed_price' },
      { group_item_id: 20, group_item_name: '水洗', parent_group_item_id: 0 },
      { group_item_id: 20, group_item_name: '', parent_group_item_id: 10 },
    ],
    productOverrides: [
      { scope: 'sku', sku_id: 201, fixed_unit_price: 66 },
    ],
    product: { id: 201, sku_id: 201, parent_product_id: 200, group_item_id: 20 },
  })

  assert.equal(resolved.pricing_mode, 'fixed_price')
  assert.equal(resolved.pricing_mode_source_group_item_id, 10)
  assert.equal(resolved.pricing_mode_source_group_item_name, '生豆')
  assert.equal(resolved.pricing_mode_source_group_depth, 1)
  assert.equal(resolved.fixed_unit_price, 66)
})

test('price table inheritance stops at category cycles and falls back to the price table', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'pricing_rule', pricing_rule_id: 99 },
    groupAssignments: [
      { group_item_id: 20, group_item_name: '产区', parent_group_item_id: 30 },
      { group_item_id: 30, group_item_name: '处理法', parent_group_item_id: 20 },
      { group_item_id: 40, group_item_name: '水洗', parent_group_item_id: 30 },
    ],
    product: { id: 401, sku_id: 401, parent_product_id: 400, group_item_id: 40 },
  })

  assert.equal(resolved.pricing_mode, 'pricing_rule')
  assert.equal(resolved.pricing_mode_source, 'default')
  assert.equal(resolved.pricing_rule_id, 99)
  assert.equal(resolved.pricing_mode_source_group_item_id, 0)
  assert.equal(resolved.pricing_mode_source_group_item_name, '')
  assert.equal(resolved.pricing_mode_source_group_depth, -1)
})

test('price table inherited fixed mode never shares category or price-list amounts across SKUs', () => {
  const common = {
    defaults: { pricing_mode: 'fixed_price', fixed_unit_price: 999 },
    groupAssignments: [
      { group_item_id: 10, pricing_mode: 'fixed_price', fixed_unit_price: 888 },
      { group_item_id: 20, parent_group_item_id: 10, fixed_unit_price: 777 },
    ],
    productOverrides: [
      { scope: 'sku', sku_id: 201, fixed_unit_price: 66 },
    ],
  }

  const priced = resolvePriceTableTemplateInheritance({
    ...common,
    product: { id: 201, sku_id: 201, parent_product_id: 200, group_item_id: 20 },
  })
  const empty = resolvePriceTableTemplateInheritance({
    ...common,
    product: { id: 202, sku_id: 202, parent_product_id: 200, group_item_id: 20 },
  })

  assert.equal(priced.pricing_mode, 'fixed_price')
  assert.equal(priced.fixed_unit_price, 66)
  assert.equal(empty.pricing_mode, 'fixed_price')
  assert.equal(empty.fixed_unit_price, 0)
  assert.equal(empty.fixed_unit_price_source, 'sku')
})

test('price table inheritance leaves pricing mode empty when no level configures a method', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: {},
    groupAssignments: [
      { group_item_id: 10 },
      { group_item_id: 20, parent_group_item_id: 10 },
    ],
    product: { id: 201, sku_id: 201, parent_product_id: 200, group_item_id: 20 },
  })

  assert.equal(resolved.pricing_mode, '')
  assert.equal(resolved.pricing_mode_source, 'default')
})

test('a SKU fixed amount never invents a pricing method when every level inherits', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: {},
    groupAssignments: [
      { group_item_id: 10 },
      { group_item_id: 20, parent_group_item_id: 10 },
    ],
    productOverrides: [
      { scope: 'sku', sku_id: 201, fixed_unit_price: 66 },
    ],
    product: { id: 201, sku_id: 201, parent_product_id: 200, group_item_id: 20 },
  })

  assert.equal(resolved.pricing_mode, '')
  assert.equal(resolved.fixed_unit_price, 66)
  assert.equal(priceTablePricingResolutionWarning(resolved), '未设置计价方式')
})

test('legacy template IDs infer the method from the nearest configured level', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: {},
    groupAssignments: [
      { group_item_id: 10, tier_template_id: 9 },
      { group_item_id: 20, parent_group_item_id: 10, pricing_rule_id: 22 },
    ],
    product: { id: 201, sku_id: 201, parent_product_id: 200, group_item_id: 20 },
  })

  assert.equal(resolved.pricing_mode, 'pricing_rule')
  assert.equal(resolved.pricing_mode_source, 'subgroup')
  assert.equal(resolved.pricing_rule_id, 22)
})

test('price table pricing resolution warnings distinguish missing method, amount and templates', () => {
  assert.equal(priceTablePricingResolutionWarning({}), '未设置计价方式')
  assert.equal(priceTablePricingResolutionWarning({ pricing_mode: 'fixed_price', fixed_unit_price: 0 }), '未填写固定价')
  assert.equal(priceTablePricingResolutionWarning({ pricing_mode: 'tier_template', tier_template_id: 0 }), '未选择阶梯模板')
  assert.equal(priceTablePricingResolutionWarning({ pricing_mode: 'pricing_rule', pricing_rule_id: 0 }), '未选择价格计算模板')
  assert.equal(priceTablePricingResolutionWarning({ pricing_mode: 'fixed_price', fixed_unit_price: 68 }), '')
  assert.equal(priceTablePricingResolutionWarning({ pricing_mode: 'tier_template', tier_template_id: 9 }), '')
  assert.equal(priceTablePricingResolutionWarning({ pricing_mode: 'pricing_rule', pricing_rule_id: 11 }), '')
})

test('price table inheritance does not borrow a pricing rule ID when the effective category selects pricing-rule mode without one', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'pricing_rule', pricing_rule_id: 99 },
    groupAssignments: [
      { group_item_id: 10, pricing_mode: 'pricing_rule', pricing_rule_id: 88 },
      { group_item_id: 20, parent_group_item_id: 10, pricing_mode: 'pricing_rule', pricing_rule_id: 0 },
    ],
    product: { id: 201, sku_id: 201, parent_product_id: 200, group_item_id: 20 },
  })

  assert.equal(resolved.pricing_mode, 'pricing_rule')
  assert.equal(resolved.pricing_mode_source, 'subgroup')
  assert.equal(resolved.pricing_rule_id, 0)
  assert.equal(resolved.pricing_rule_source, 'subgroup')
})

test('price table inheritance does not borrow a tier template ID when the effective ancestor selects tier mode without one', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'tier_template', tier_template_id: 99 },
    groupAssignments: [
      { group_item_id: 10, pricing_mode: 'tier_template', tier_template_id: 88 },
      { group_item_id: 20, parent_group_item_id: 10, pricing_mode: 'tier_template', tier_template_id: 0 },
      { group_item_id: 30, parent_group_item_id: 20 },
    ],
    product: { id: 301, sku_id: 301, parent_product_id: 300, group_item_id: 30 },
  })

  assert.equal(resolved.pricing_mode, 'tier_template')
  assert.equal(resolved.pricing_mode_source, 'parent_group')
  assert.equal(resolved.pricing_mode_source_group_item_id, 20)
  assert.equal(resolved.tier_template_id, 0)
  assert.equal(resolved.tier_template_source, 'parent_group')
})

test('price tier template quantities are sales-spec counts and ignore legacy kg/lb tier units', () => {
  const kgTemplate = {
    id: 3,
    name: '咖啡熟豆',
    tiers: [
      { id: 31, label: '1kg+', quantity_unit: 'kg', active: true },
      { id: 32, label: '24kg+', quantity_unit: '公斤', active: true },
    ],
  }

  assert.deepEqual(priceTierTemplateUnitCompatibility({ name: '初晓', default_sales_unit: '磅' }, kgTemplate), {
    compatible: true,
    product_unit: '磅',
    template_units: ['kg'],
    message: '',
  })
  assert.deepEqual(priceTierTemplateUnitCompatibility({
    name: '初晓',
    default_sales_unit: '磅',
    price_unit_snapshot: 'kg',
    price_unit: 'kg',
  }, kgTemplate), {
    compatible: true,
    product_unit: '磅',
    template_units: ['kg'],
    message: '',
  })
  assert.equal(priceTierTemplateUnitCompatibility({
    price_unit: '',
    default_sales_unit: '',
    quote_unit: '磅',
    price_unit_snapshot: 'kg',
  }, kgTemplate).compatible, true)
  assert.equal(priceTablePricingRuleTrialPayload({
    product_id: 550,
    pricing_mode: 'tier_template',
    tier_unit_compatible: false,
    quantity_basis: 'sales_spec_count',
    pricing_rule_id: 41,
    price_unit: 'lb',
  })?.quote_unit, 'lb', 'a sales-spec-count tier row may create a Pricing Rule trial request regardless of legacy tier unit')
  assert.equal(productCurrentSalesSpecUnit({ price_unit: '', quote_unit: '磅', price_unit_snapshot: 'kg' }), '磅')
  const kgOverrideKey = priceTierTemplateRowKey({
    productID: 550, templateID: 3, tierID: 31,
    product: { default_sales_unit: 'kg' }, tier: { quantity_unit: 'kg' },
  })
  const poundOverrideKey = priceTierTemplateRowKey({
    productID: 550, templateID: 3, tierID: 31,
    product: { default_sales_unit: '磅' }, tier: { quantity_unit: 'lb' },
  })
  assert.notEqual(kgOverrideKey, poundOverrideKey, 'a concrete SKU sales-spec transition must invalidate its old manual price override key')
  assert.equal(
    priceTierTemplateRowKey({ productID: 550, templateID: 3, tierID: 31, product: { default_sales_unit: '磅' }, tier: { quantity_unit: 'kg' } }),
    priceTierTemplateRowKey({ productID: 550, templateID: 3, tierID: 31, product: { default_sales_unit: '磅' }, tier: { quantity_unit: 'lb' } }),
    'legacy tier quantity units must not affect a current sales-spec-count row identity',
  )
  const poundRows = buildPriceTableRowsFromTemplateResolution({
    product: { id: 550, name: '初晓', default_sales_unit: '磅', price_unit: 'kg', inventory_unit: 'kg', unit_conversion_json: { 磅: { kg: 0.45359237 } } },
    resolution: { pricing_mode: 'tier_template', tier_template_id: 4, tier_template_source: 'product' },
    tierTemplate: { id: 4, name: '磅装阶梯', tiers: [{ id: 41, label: '1磅+', quantity_unit: 'lb', pricing_rule_id: 41 }] },
    pricingRulesByID: { 41: { id: 41, code: 'PR-LB' } },
    unitPriceByTier: { '1磅+': 68 },
  })
  assert.equal(poundRows[0].price_unit, '磅', 'generated rows must use the current sales spec instead of a stale price unit')
  assert.equal(poundRows[0].tier_quantity_unit, '磅', 'the tier display unit must be the selected SKU sales spec')
  assert.equal(poundRows[0].quantity_basis, 'sales_spec_count')
  assert.equal(priceTierTemplateUnitCompatibility({ default_sales_unit: 'lbs' }, { tiers: [{ quantity_unit: '磅' }] }).compatible, true)
  assert.equal(priceTierTemplateUnitCompatibility({ default_sales_unit: '1Kg' }, { tiers: [{ quantity_unit: '千克' }] }).compatible, true)
  assert.equal(priceTierTemplateUnitCompatibility({ default_sales_unit: '盒（10袋）' }, { tiers: [{ quantity_unit: '盒' }] }).compatible, true)
  assert.deepEqual(priceTierTemplateUnitCompatibility({}, kgTemplate), {
    compatible: false, product_unit: '', template_units: ['kg'], message: '阶梯模板不可用：商品缺少有效默认销售规格',
  })
  assert.deepEqual(priceTierTemplateUnitCompatibility({ default_sales_unit: 'kg' }, { tiers: [{ min_qty: 1, quantity_unit: '' }] }), {
    compatible: true, product_unit: 'kg', template_units: [], message: '',
  })
  assert.deepEqual(priceTierTemplateUnitCompatibility({ default_sales_unit: 'kg' }, { tiers: [] }), {
    compatible: false, product_unit: 'kg', template_units: [], message: '阶梯模板不可用：阶梯模板缺少有效数量档位',
  })
})

test('price table generates sales-spec-count tiers even when legacy template units differ', () => {
  const rows = buildPriceTableRowsFromTemplateResolution({
    product: { id: 88, name: '初晓', inventory_unit: 'kg', default_sales_unit: '磅', unit_conversion_json: '{"磅":{"kg":0.454}}' },
    resolution: { pricing_mode: 'tier_template', tier_template_id: 3, tier_template_source: 'product' },
    tierTemplate: {
      id: 3,
      name: '咖啡熟豆',
      tiers: [{ id: 31, label: '1kg+', min_qty: 1, quantity_unit: 'kg', pricing_rule_id: 41 }],
    },
    pricingRulesByID: { 41: { id: 41, code: 'PR-1KG' } },
    unitPriceByTier: { '1kg+': 88 },
  })
  assert.equal(rows.length, 1)
  assert.equal(rows[0].tier_quantity_unit, '磅')
  assert.equal(rows[0].quantity_basis, 'sales_spec_count')
})

test('price table can generate a single row from pricing rule mode or fixed price mode', () => {
  assert.deepEqual(buildPriceTableRowsFromTemplateResolution({
    product: { id: 88, name: '初晓拼配', inventory_unit: 'kg', default_sales_unit: 'kg' },
    resolution: {
      pricing_mode: 'pricing_rule',
      pricing_mode_source: 'parent_group',
      pricing_rule_id: 40,
      pricing_rule_source: 'parent_group',
    },
    pricingRule: { id: 40, code: 'PR-BASE' },
    unitPriceByTier: { default: 86 },
  }), [
    { product_id: 88, product_name: '初晓拼配', price_unit: 'kg', inventory_unit: 'kg', inventory_conversion_json: { kg: { kg: 1 } }, tier_label: '基础价', min_qty: 0, max_qty: null, final_unit_price: 86, pricing_mode: 'pricing_rule', pricing_mode_source: 'parent_group', tier_template_id: 0, tier_template_source: '', template_tier_id: 0, pricing_rule_id: 40, pricing_rule_source: 'parent_group', pricing_rule_version: 'PR-BASE', tier_pricing_rule_id: 0, tier_pricing_rule_version: '' },
  ])

  assert.deepEqual(buildPriceTableRowsFromTemplateResolution({
    product: { id: 88, name: '初晓拼配', inventory_unit: 'kg', default_sales_unit: 'kg' },
    resolution: {
      pricing_mode: 'fixed_price',
      pricing_mode_source: 'product',
      fixed_unit_price: 73.5,
      fixed_unit_price_source: 'product',
    },
  }), [
    { product_id: 88, product_name: '初晓拼配', price_unit: 'kg', inventory_unit: 'kg', inventory_conversion_json: { kg: { kg: 1 } }, tier_label: '固定价', min_qty: 0, max_qty: null, final_unit_price: 73.5, pricing_mode: 'fixed_price', pricing_mode_source: 'product', tier_template_id: 0, tier_template_source: '', template_tier_id: 0, pricing_rule_id: 0, pricing_rule_source: '', pricing_rule_version: '', tier_pricing_rule_id: 0, tier_pricing_rule_version: '', fixed_unit_price: 73.5 },
  ])
})

test('price table pricing-rule preview row uses the live trial result', () => {
  const row = {
    row_key: '550:pricing_rule',
    product_id: 550,
    product_name: '熟豆-红岩拼配',
    pricing_mode: 'pricing_rule',
    pricing_mode_source: 'product',
    tier_label: '基础价',
    price_unit: 'lb',
    final_unit_price: 0,
    original_final_unit_price: 0,
    inventory_unit: 'kg',
    inventory_conversion_json: {},
    pricing_rule_id: 40,
    pricing_rule_source: 'product',
    pricing_rule_version: '咖啡熟豆磅装模板-v1',
    tier_pricing_rule_id: 0,
    tier_pricing_rule_version: '',
    cost_source_snapshot: {
      pricing_rule_id: 40,
      bom_version_id: 8842,
      process_route_id: 19,
      operation_template_id: 9,
    },
  }

  assert.deepEqual(priceTablePricingRuleTrialPayload(row, { customerID: 0 }), {
    pricing_rule_id: 40,
    product_id: 550,
    customer_id: 0,
    bom_version_id: 8842,
    process_route_id: 19,
    operation_template_id: 9,
    quote_unit: 'lb',
    overrides: {},
  })

  const got = applyPricingRuleTrialToPriceTableRow(row, {
    pricing_rule_id: 40,
    product_id: 550,
    quote_unit: 'lb',
    inventory_unit: 'kg',
    final_unit_price: 68.5,
    bom_version_id: 8842,
    bom_version_no: 'V002',
    process_route_id: 19,
    process_route_name: '标准烘焙路线',
    operation_template_id: 9,
    operation_template_name: '标准烘焙',
    base_cost: 42.3,
    warnings: [],
    base_cost_details: [{
      key: 'operation:bom_snapshot:19',
      type: 'operation',
      name: '烘焙',
      type_label: '标准工序',
      consume_unit: 'per_inventory_unit',
      capacity_selection_source: 'bom_operation_snapshot',
      description: '标准工序成本来自 BOM 工序成本快照：布勒 · 20kg档 · 8.5000/kg',
      amount: 8.5,
      unit_cost: 8.5,
      unit: 'kg',
    }],
  })

  assert.equal(got.product_name, '熟豆-红岩拼配')
  assert.equal(got.final_unit_price, 68.5)
  assert.equal(got.original_final_unit_price, 68.5)
  assert.equal(got.price_unit, 'lb')
  assert.deepEqual(got.inventory_conversion_json, { lb: { kg: 0.454 } })
  assert.equal(got.cost_source_snapshot.bom_version_no, 'V002')
  assert.equal(got.cost_source_snapshot.process_route_name, '标准烘焙路线')
  assert.equal(got.cost_source_snapshot.operation_template_name, '标准烘焙')
  assert.equal(got.cost_source_snapshot.pricing_rule_trial_final_unit_price, 68.5)
  assert.deepEqual(got.cost_source_snapshot.pricing_rule_trial_warnings, [])
  assert.equal(got.cost_source_snapshot.pricing_rule_trial_base_cost_details[0].capacity_selection_source, 'bom_operation_snapshot')
})

test('price table pricing-rule preview keeps a manually adjusted final price while refreshing its automatic baseline', () => {
  const got = applyPricingRuleTrialToPriceTableRow({
    product_id: 550,
    pricing_mode: 'pricing_rule',
    pricing_rule_id: 40,
    price_unit: 'kg',
    inventory_unit: 'kg',
    final_unit_price: 88,
    original_final_unit_price: 80,
    manual_adjusted: true,
    cost_source_snapshot: {},
  }, {
    pricing_rule_id: 40,
    product_id: 550,
    quote_unit: 'kg',
    inventory_unit: 'kg',
    final_unit_price: 84,
  })

  assert.equal(got.final_unit_price, 88)
  assert.equal(got.original_final_unit_price, 84)
  assert.equal(got.manual_adjusted, true)
})

test('price table pricing-rule preview payload falls back to numeric product key', () => {
  const row = {
    row_key: '550:pricing_rule',
    product_id: 0,
    product_key: '550',
    product_name: '熟豆-红岩拼配',
    pricing_mode: 'pricing_rule',
    price_unit: 'kg',
    inventory_unit: 'kg',
    pricing_rule_id: 11,
    cost_source_snapshot: {
      bom_version_id: 723,
      process_route_id: 0,
    },
  }

  assert.deepEqual(priceTablePricingRuleTrialPayload(row, { customerID: 0 }), {
    pricing_rule_id: 11,
    product_id: 550,
    customer_id: 0,
    bom_version_id: 723,
    process_route_id: 0,
    operation_template_id: 0,
    quote_unit: 'kg',
    overrides: {},
  })
})

test('price table tier-template preview rows use their tier pricing rule trial result', () => {
  const row = {
    row_key: '550:tier-template:8:1',
    product_id: 550,
    product_name: '熟豆-红岩拼配',
    pricing_mode: 'tier_template',
    pricing_mode_source: 'product',
    tier_label: '1磅-9磅',
    min_qty: 1,
    max_qty: 9,
    price_unit: 'lb',
    final_unit_price: 0,
    original_final_unit_price: 0,
    inventory_unit: 'kg',
    inventory_conversion_json: {},
    tier_template_id: 8,
    template_tier_id: 1,
    pricing_rule_id: 40,
    pricing_rule_source: 'tier_template',
    pricing_rule_version: '咖啡熟豆磅装模板-v1',
    tier_pricing_rule_id: 40,
    tier_pricing_rule_version: '咖啡熟豆磅装模板-v1',
    cost_source_snapshot: {
      pricing_rule_id: 40,
      bom_version_id: 8842,
      process_route_id: 19,
      operation_template_id: 9,
    },
  }

  assert.deepEqual(priceTablePricingRuleTrialPayload(row, { customerID: 0 }), {
    pricing_rule_id: 40,
    product_id: 550,
    customer_id: 0,
    bom_version_id: 8842,
    process_route_id: 19,
    operation_template_id: 9,
    quote_unit: 'lb',
    overrides: {},
  })

  const got = applyPricingRuleTrialToPriceTableRow(row, {
    pricing_rule_id: 40,
    product_id: 550,
    quote_unit: 'lb',
    inventory_unit: 'kg',
    final_unit_price: 68.5,
  })

  assert.equal(got.pricing_mode, 'tier_template')
  assert.equal(got.tier_label, '1磅-9磅')
  assert.equal(got.final_unit_price, 68.5)
  assert.equal(got.original_final_unit_price, 68.5)
  assert.equal(got.tier_pricing_rule_id, 40)
  assert.deepEqual(got.inventory_conversion_json, { lb: { kg: 0.454 } })
})

test('product price management edits markup-only pricing rules in a right drawer', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const pane = source.match(/<div v-show="showProductPriceManagementPane"[\s\S]*?<p class="muted price-list-flat-row-note"/)?.[0] || ''
  const editorDrawer = source.match(/<div v-if="pricingRuleEditorDrawerOpen"[\s\S]*?<div v-if="pricingRuleTrialDrawerOpen"/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const want of ['product-price-management-pane', '商品价格管理', '价格计算模板', 'Pricing Rule', '价格试算', '新建价格计算模板', '基础成本', '加价率', '税率', '取整规则', '复制', '失效']) {
    assert.equal(pane.includes(want), true, `product price management pane should expose ${want}`)
  }
  for (const want of ['pricingRules', 'pricingRuleEditorDrawerOpen', 'buildPricingRulePayload', 'buildPricingRuleCopyPayload', 'startPricingRuleEdit', 'closePricingRuleEditor', 'pricingRuleNeedsMarkupConfirmation', 'copyPricingRule', 'deactivatePricingRule', 'addPricingRuleOtherCostRow']) {
    assert.match(script, new RegExp(want))
  }
  assert.match(pane, /@click="openPricingRuleTrial\(\)"[^>]*>价格试算<\/button>[\s\S]*@click="resetPricingRuleForm"[^>]*>新建价格计算模板<\/button>/)
  assert.match(pane, /class="text-button pricing-rule-name-button"[\s\S]*@click="startPricingRuleEdit\(rule\)"/)
  assert.match(pane, /class="secondary compact-action pricing-rule-copy-action"[\s\S]*@click="copyPricingRule\(rule\)"[\s\S]*>复制<\/button>/)
  assert.match(pane, /:disabled="productPriceSaving \|\| pricingRuleNeedsMarkupConfirmation\(rule\)"/)
  assert.match(pane, /:class="\['pricing-rule-row', \{ inactive: rule\.active === false \}\]"/)
  assert.doesNotMatch(pane, /<form class="template-editor pricing-rule-form"/)
  assert.doesNotMatch(pane, />编辑模板<\/button>/)
  assert.doesNotMatch(pane, /@click="openPricingRuleTrial\(rule\)"/)

  for (const want of ['价格计算模板编辑', '编辑价格计算模板', '新建价格计算模板', 'pricing-rule-form', '模板名称', '基础成本', '生产 BOM 成本（物料+工序）', '其他成本', '成本名', '成本价格', '全局币种配置', '加价率（80%=0.8）', '税前价 = 成本基数 × (1 + 加价率)', '最终售价再计算税额和取整', '税费方式', '最低毛利率（仅预警）', '只比较试算结果，不参与售价计算', '公式版本', '试算说明', '税率', '取整规则', '保存价格计算模板']) {
    assert.equal(editorDrawer.includes(want), true, `pricing rule editor drawer should expose ${want}`)
  }
  assert.match(editorDrawer, /class="settings-drawer-mask"[^>]*@click\.self="closePricingRuleEditor"/)
  assert.match(editorDrawer, /class="settings-drawer pricing-rule-editor-drawer"[^>]*aria-label="价格计算模板编辑"/)
  assert.match(editorDrawer, /role="dialog"/)
  assert.match(editorDrawer, /aria-modal="true"/)
  assert.match(editorDrawer, /@keydown\.esc\.stop\.prevent="closePricingRuleEditor"/)
  assert.match(editorDrawer, /@keydown\.tab="trapPricingRuleEditorFocus"/)
  assert.match(editorDrawer, /@click="closePricingRuleEditor"[^>]*>关闭<\/button>/)
  assert.match(editorDrawer, /<form class="template-editor pricing-rule-form"[^>]*@submit\.prevent="savePricingRule"/)
  assert.match(editorDrawer, /v-if="pricingRuleNeedsMarkupConfirmation\(pricingRuleForm\)"[\s\S]*旧价格方式无法安全换算；请新建加价率模板/)
  assert.doesNotMatch(editorDrawer, /v-model="pricingRuleForm\.profit_method"/)
  assert.doesNotMatch(editorDrawer, /value="gross_margin"|value="fixed_add"|>毛利率<|>固定加价</)
  assert.doesNotMatch(editorDrawer, /<div v-if="(?:error|ok)"/)
  assert.match(script, /const pricingRuleEditorDrawerOpen = ref\(false\)/)
  assert.match(script, /function resetPricingRuleForm\(\) \{[\s\S]*?openPricingRuleEditorDrawer\(\)[\s\S]*?\}/)
  assert.match(script, /function startPricingRuleEdit\(rule\) \{[\s\S]*?openPricingRuleEditorDrawer\(\)[\s\S]*?\}/)
  assert.match(script, /function openPricingRuleEditorDrawer\(\) \{[\s\S]*?pricingRuleEditorDrawerOpen\.value = true[\s\S]*?firstField[\s\S]*?focus/)
  assert.match(script, /function closePricingRuleEditor\(\) \{[\s\S]*?pricingRuleEditorDrawerOpen\.value = false[\s\S]*?\}/)
  assert.match(script, /function trapPricingRuleEditorFocus\(event\)/)
  const copyStart = script.indexOf('async function copyPricingRule')
  const copyEnd = script.indexOf('async function deactivatePricingRule', copyStart)
  assert.ok(copyStart > -1 && copyEnd > copyStart, 'copyPricingRule block not found')
  assert.equal(script.slice(copyStart, copyEnd).includes('openPricingRuleEditorDrawer()'), true, 'copied pricing rule should open the editor drawer')
  assert.equal(script.slice(copyStart, copyEnd).includes('pricingRuleNeedsMarkupConfirmation(rule)'), true, 'unsupported legacy pricing rules must not be copied into active markup templates')
  const saveStart = script.indexOf('async function savePricingRule')
  assert.ok(saveStart > -1 && copyStart > saveStart, 'savePricingRule block not found')
  assert.equal(script.slice(saveStart, copyStart).includes("ok.value = '价格计算模板已保存'"), true, 'saving should keep the global success notification feedback')
  assert.match(style, /\.pricing-rule-editor-drawer/)
  assert.match(source, /计价方式：加价率/)
  assert.match(source, /临时加价率/)
  assert.doesNotMatch(source, /利润方式：/)
  assert.doesNotMatch(source, /临时利润\/加价/)
  for (const forbidden of ['商品成本上下文', '成本项配置', '库存成本', '手工成本', '最近采购成本', '成本取数口径', '商品价格记录', '最终单价', '引用价格记录', 'source_price_record_id', '阶梯价模板', 'priceTierTemplateForm', 'savePriceTierTemplate', 'min_qty', 'max_qty', 'tier_label']) {
    assert.equal(`${pane}${editorDrawer}`.includes(forbidden), false, `product price management should not expose ${forbidden}`)
  }
})

test('product price management exposes pricing rule trial drawer and API wiring', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const pane = source.match(/<div v-show="showProductPriceManagementPane"[\s\S]*?<p class="muted price-list-flat-row-note"/)?.[0] || ''
  const trialDrawer = source.match(/<div v-if="pricingRuleTrialDrawerOpen"[\s\S]*?<div v-if="customerAliasCreateDrawerOpen"/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const want of [
    '价格计算模板试算',
    'openPricingRuleTrial',
    'activePricingRuleTrialOptions',
    'handlePricingRuleTrialRuleChange',
    'pricingRuleTrialDrawerOpen',
    'pricingRuleTrialForm',
    'buildPricingRuleTrialPayload',
    '/api/costing/pricing-rule-trial',
    '试算商品',
    '试算模板',
    '请选择启用的价格计算模板',
    'BOM版本',
    '工艺路线',
    '销售规格',
    '临时加价率',
    '临时税率',
    '其他成本',
    '加价后价格',
    '试算单价',
    '标准制造成本',
    'BOM物料成本',
    '标准工序成本',
    '标准制造成本折算明细',
    '物料成本明细',
    '工序成本明细',
    'BOM组成',
    '原料损耗',
    '损耗后用量',
    '成本单价',
    '折算成本',
    '加价增加',
    '税额',
    '取整调整',
    '税率来源',
    '取整规则：',
    '来自价格计算模板',
    'pricingRuleTrialTaxSourceLabel',
    'pricingRuleTrialHasRoundingAdjustment(pricingRuleTrialResult)',
    'base_cost_details',
    'capacity_selection_source',
    '成本来源',
    'BOM工序成本快照',
    '标准工序成本来自发布 BOM 冻结的工序成本快照',
    'tax_in_price_amount',
    'pricing-rule-trial-waterfall',
    'pricing-rule-trial-operator',
    'pricing-rule-trial-explanation-panel',
    'pricingRuleTrialActiveExplanation',
    'openPricingRuleTrialExplanation',
    'closePricingRuleTrialExplanation',
    'other_cost_details',
    'profit_explanation',
    'cost_unit_cost',
    'cost_unit',
    'recipe_ratio_pct',
    'effective_ratio_pct',
    'ratioPct * (1 - lossRate)',
    'pricingRuleTrialBaseCostRecipeUsage(row)',
    'pricingRuleTrialBaseCostLossRate(row)',
    'pricingRuleTrialBaseCostEffectiveUsage(row)',
    'pricingRuleTrialBaseCostUnitCostValue(row)',
    'pricingRuleTrialBaseCostUnit(row, pricingRuleTrialResult)',
    '试算说明',
    '点击查看试算说明',
    '本次试算抽屉',
    '价格计算模板编辑抽屉',
    '临时加价率',
    '计算公式',
    'formula_expression_lines',
    '公式步骤',
    'pricingRuleTrialSalesSpecOptions',
    'pricingRuleTrialBomVersionOptions',
    'pricingRuleTrialProcessRouteOptions',
    'schedulePricingRuleTrial',
    '草稿，仅供试算',
    '配置BOM',
    'navigatePricingRuleTrialBom',
    '返回价格试算',
    'pricing_rule_trial_return_key',
    'restorePricingRuleTrialReturnState',
    '更新参数到价格计算模板',
    'updatePricingRuleFromTrial',
    'buildPricingRuleUpdateFromTrial',
  ]) {
    assert.ok(source.includes(want), `missing pricing rule trial marker: ${want}`)
  }
  for (const forbidden of [
    '售价后附加成本',
    '重新试算',
    'post_markup_cost_rows',
    'addPricingRuleTrialPostMarkupCostRow',
    'removePricingRuleTrialPostMarkupCostRow',
    '状态：',
    'product_production_config',
    'missing',
    '发布售价快照反推',
    '损耗增加',
    'pricingRuleTrialHasYieldLoss',
    '当前商品损耗率',
    '损耗后成本',
    'ratioPct / (1 + lossRate)',
  ]) {
    assert.equal(trialDrawer.includes(forbidden), false, `pricing rule trial drawer should not expose ${forbidden}`)
  }
  assert.match(pane, /@click="openPricingRuleTrial\(\)"[^>]*>价格试算<\/button>/)
  assert.doesNotMatch(pane, /@click="openPricingRuleTrial\(rule\)"/)
  assert.match(trialDrawer, /<select v-model\.number="pricingRuleTrialForm\.pricing_rule_id"[\s\S]*activePricingRuleTrialOptions/)
  assert.match(trialDrawer, /<th>BOM组成<\/th>[\s\S]*<th>原料损耗<\/th>[\s\S]*<th>损耗后用量<\/th>[\s\S]*<th>成本单价<\/th>[\s\S]*<th>折算成本<\/th>/)
  assert.match(trialDrawer, /pricingRuleTrialBaseCostRecipeUsage\(row\)[\s\S]*pricingRuleTrialBaseCostLossRate\(row\)[\s\S]*pricingRuleTrialBaseCostEffectiveUsage\(row\)/)
  assert.match(trialDrawer, /trialMoneyDisplay\(pricingRuleTrialBaseCostUnitCostValue\(row\), pricingRuleTrialBaseCostUnit\(row, pricingRuleTrialResult\)\)/)
  assert.match(trialDrawer, /trialMoneyDisplay\(row\.amount, row\.unit \|\| pricingRuleTrialResult\.quote_unit\)/)
  assert.match(source, /<select v-model\.number="pricingRuleTrialForm\.bom_version_id"[\s\S]*pricingRuleTrialBomVersionOptions/)
  assert.match(trialDrawer, /试算BOM版本[\s\S]*@click="navigatePricingRuleTrialBom"[\s\S]*配置BOM/)
  assert.match(source, /<select v-model\.number="pricingRuleTrialForm\.process_route_id"[\s\S]*pricingRuleTrialProcessRouteOptions/)
  assert.match(trialDrawer, /<span>试算商品<\/span>[\s\S]*v-model="pricingRuleTrialForm\.parent_product_id"/)
  assert.match(trialDrawer, /<span>销售规格<\/span>[\s\S]*<select v-model\.number="pricingRuleTrialForm\.product_id"[\s\S]*pricingRuleTrialSalesSpecOptions/)
  assert.doesNotMatch(trialDrawer, /<span>销售单位<\/span>/)
  assert.doesNotMatch(trialDrawer, /v-model="pricingRuleTrialForm\.quote_unit"/)
  assert.match(script, /pricingRuleTrialProductSpecOptions/)
  assert.match(script, /pricingRuleTrialDefaultProductSpecID/)
  assert.match(script, /pricingRuleTrialProductSpecUnit/)
  assert.match(script, /parent_product_id:\s*0/)
  assert.match(script, /function schedulePricingRuleTrial\(\) \{[\s\S]*pricingRuleTrialRunID\+\+[\s\S]*pricingRuleTrialLoading\.value = false[\s\S]*runPricingRuleTrial\(\)/)
  assert.match(script, /if \(runID === pricingRuleTrialRunID\) \{[\s\S]*pricingRuleTrialResult\.value = result/)
  assert.doesNotMatch(pane, /@click="runPricingRuleTrial"/)
  assert.match(script, /apiSend\('\/api\/costing\/pricing-rule-trial'/)
  assert.match(script, /function navigatePricingRuleTrialBom\(\)[\s\S]*storePricingRuleTrialReturnState[\s\S]*key:\s*'bom'[\s\S]*production_bom_id[\s\S]*returnNavigation:[\s\S]*key:\s*'productPriceManagement'/)
  assert.match(script, /async function updatePricingRuleFromTrial\(\)[\s\S]*window\.confirm[\s\S]*apiSend\(`\/api\/product-pricing-rules\/\$\{payload\.id\}`[\s\S]*method:\s*'PUT'/)
  assert.match(trialDrawer, /class="drawer-footer pricing-rule-trial-footer"[\s\S]*@click="updatePricingRuleFromTrial"[\s\S]*更新参数到价格计算模板/)
  assert.match(script, /watch\(\(\) => pricingRuleTrialAutoRunSignature\.value/)
  assert.match(script, /const inventoryUnit = String\(option\.inventory_unit \|\| ''\)\.trim\(\)/)
  assert.match(script, /pricingRuleTrialForm\.value\.quote_unit = inventoryUnit/)
  assert.match(script, /resultVariantID[\s\S]{0,180}pricingRuleTrialResult\.value = null/)
  assert.match(script, /watch\(\(\) => pricingRuleTrialForm\.value\.bom_version_id[\s\S]*availableVariants[\s\S]*pricingRuleTrialForm\.value\.bom_variant_id = 0/)
  assert.match(style, /\.pricing-rule-trial-drawer/)
  assert.match(style, /\.pricing-rule-trial-waterfall-card\.interactive/)
  assert.match(style, /\.pricing-rule-trial-explanation-panel/)
  assert.match(trialDrawer, /type="button"[\s\S]*@click="openPricingRuleTrialExplanation\('base_cost'\)"[\s\S]*标准制造成本/)
  assert.match(trialDrawer, /type="button"[\s\S]*@click="openPricingRuleTrialExplanation\('other_cost'\)"[\s\S]*其他成本/)
  assert.match(trialDrawer, /<dt>加价基数<\/dt>/)
  assert.match(trialDrawer, /v-if="pricingRuleTrialHasRoundingAdjustment\(pricingRuleTrialResult\)"[\s\S]*取整调整/)
  assert.match(trialDrawer, /type="button"[\s\S]*@click="openPricingRuleTrialExplanation\('profit_markup'\)"[\s\S]*加价增加/)
  assert.doesNotMatch(trialDrawer, /placeholder="按商品\/BOM"/)
  assert.match(trialDrawer, /v-if="pricingRuleTrialActiveExplanation"[\s\S]*试算说明[\s\S]*@click="closePricingRuleTrialExplanation"/)
  assert.match(trialDrawer, /pricingRuleTrialExplanationTitle\(pricingRuleTrialActiveExplanation\)/)
  assert.match(trialDrawer, /pricingRuleTrialOtherCostRows\(pricingRuleTrialResult\)/)
  assert.match(trialDrawer, /pricingRuleTrialProfitExplanation\(pricingRuleTrialResult\)/)
  assert.match(source, /pricing-rule-trial-operator[\s\S]*\+/)
  assert.match(source, /pricing-rule-trial-operator[\s\S]*=/)
  assert.doesNotMatch(trialDrawer, /点击查看试算说明：BOM\+工序成本/)
})

test('material cost trial exposes manufacturing details, formulas, BOM selection and configuration navigation', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const trialDrawer = source.match(/<div v-if="pricingRuleTrialDrawerOpen"[\s\S]*?<div v-if="customerAliasCreateDrawerOpen"/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const want of [
    '物料成本试算',
    '/api/costing/material-cost-trial-options',
    '/api/costing/material-cost-trial',
    '试算 BOM 版本',
    '物料成本明细',
    '标准工序成本明细',
    '标准制造成本',
    '成本来源',
    '计算公式',
    '公式步骤',
    'base_cost_details',
    'formula_expression_lines',
    'navigateMaterialCostTrialBom',
    '配置 BOM',
  ]) {
    assert.ok(source.includes(want), `missing material cost trial marker: ${want}`)
  }
  assert.match(trialDrawer, /v-model.number="pricingRuleTrialMaterialBomVersionID"[\s\S]*@click="navigateMaterialCostTrialBom"/)
  assert.equal(trialDrawer.includes("pricingRuleTrialBaseCostRows(pricingRuleTrialMaterialResult, 'material')"), true)
  assert.equal(trialDrawer.includes('pricingRuleTrialMaterialResult.formula_expression'), true)
  assert.equal(trialDrawer.includes('pricingRuleTrialMaterialResult.formula_expression_lines'), true)
  assert.equal(script.includes('function navigateMaterialCostTrialBom()'), true)
  assert.equal(script.includes('production_bom_id: bomID'), true)
  assert.equal(script.includes("key: 'productPriceManagement'"), true)
  assert.equal(script.includes("label: '返回物料成本试算'"), true)
})

test('pricing rule trial product picker mirrors the order-entry type filter interaction', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const trialDrawer = source.match(/<div v-if="pricingRuleTrialDrawerOpen"[\s\S]*?<div v-if="customerAliasCreateDrawerOpen"/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(script, /pricingRuleTrialMainProductOptions/)
  assert.match(script, /orderProductKindFilterOptions/)
  assert.match(script, /orderProductFamilyOptions/)
  assert.doesNotMatch(script, /const pricingRuleTrialProductOptions\s*=\s*computed\(\(\)\s*=>\s*productRows\.value/)

  assert.match(trialDrawer, /<template\s+#menu-header>/)
  assert.match(trialDrawer, /:key="`pricing-rule-trial-product-picker:\$\{activePricingRuleTrialProductKindFilter\}`"/)
  assert.match(trialDrawer, /class="product-kind-filter"[^>]*aria-label="商品分类"/)
  assert.match(trialDrawer, /v-for="option in pricingRuleTrialProductKindFilterOptions"/)
  assert.match(trialDrawer, /class="product-kind-filter-option"/)
  assert.match(trialDrawer, /:aria-pressed="activePricingRuleTrialProductKindFilter === option\.value"/)
  assert.match(trialDrawer, /@click\.stop="setPricingRuleTrialProductKindFilter\(option\.value\)"/)
  assert.match(trialDrawer, /<template\s+#option="\{ option \}">/)
  assert.match(trialDrawer, /productKindBadgeClass\(option\)[\s\S]*productKindLabel\(option\)/)
  assert.match(script, /watch\(pricingRuleTrialProductKindFilterOptions,[\s\S]*activePricingRuleTrialProductKindFilter\.value = ''/)
  assert.match(script, /watch\(pricingRuleTrialMainProducts,[\s\S]*pricingRuleTrialForm\.value\.parent_product_id = 0/)
})

test('product price list owns tier template drawer and three pricing modes', () => {
  const source = fs.readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
  for (const want of [
    '管理阶梯模板',
    'tierTemplateDrawerOpen',
    '保存阶梯模板',
    '删除阶梯模板',
    '按阶梯模板计算',
    '按价格计算模板计算',
    '固定价',
    '商品 &gt; 所在分类 &gt; 上级分类逐级向上 &gt; 价格表',
    'pricing_rule_id',
  ]) {
    assert.ok(source.includes(want), `CostingView missing marker: ${want}`)
  }
  assert.doesNotMatch(source, /默认阶梯价模板/)
  assert.doesNotMatch(source, /子组/)
})

test('tier template drawer keeps long template names inside the list column', () => {
  const source = fs.readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
  const style = source.match(/<style scoped>([\s\S]*?)<\/style>/)?.[1] || ''
  const listStyle = style.match(/\.tier-template-list\s*\{([^}]*)\}/)?.[1] || ''
  const rowStyle = style.match(/\.tier-template-list-row\s*\{([^}]*)\}/)?.[1] || ''
  const textStyle = style.match(/\.tier-template-list-row strong,\s*\.tier-template-list-row small\s*\{([^}]*)\}/)?.[1] || ''

  assert.match(listStyle, /grid-template-columns:\s*minmax\(0,\s*1fr\)/)
  assert.match(listStyle, /min-width:\s*0/)
  assert.match(rowStyle, /box-sizing:\s*border-box/)
  assert.match(rowStyle, /min-width:\s*0/)
  assert.match(rowStyle, /white-space:\s*normal/)
  assert.match(textStyle, /overflow-wrap:\s*anywhere/)
  assert.match(textStyle, /white-space:\s*normal/)
})

test('customer alias rename overrides list display while preserving customer product name field', () => {
  const alias = {
    display_name: 'Karen 原客户商品名',
    brand_name: 'Karen 重命名报价名',
  }

  assert.equal(customerAliasEffectiveDisplayName(alias), 'Karen 重命名报价名')
  assert.equal(customerAliasEffectiveDisplayName({ display_name: 'Karen 原客户商品名', brand_name: ' ' }), 'Karen 原客户商品名')
})

test('product config templates only bind a gradient template for gradient pricing mode', () => {
  const gradientForm = {
    name: '按阶梯价报价',
    gradient_template_id: '18',
    unit_template_id: '22',
    price_rule_pricing_mode: 'inherit_gradient_template',
    price_rule_rounding: 'none',
  }
  const fixedForm = {
    name: '固定单价报价',
    gradient_template_id: '18',
    unit_template_id: '22',
    price_rule_pricing_mode: 'fixed_unit_price',
    price_rule_fixed_unit_price: '88',
    price_rule_rounding: 'yuan',
  }

  assert.equal(productConfigTemplateNeedsGradientTemplate(gradientForm), true)
  assert.equal(productConfigTemplateNeedsGradientTemplate(fixedForm), false)
  assert.equal(buildProductConfigTemplatePayload(gradientForm).gradient_template_id, 18)
  assert.equal(buildProductConfigTemplatePayload(fixedForm).gradient_template_id, 0)
})

test('product archive code label uses stable product code instead of list order number', () => {
  assert.equal(productCodeLabel({ id: 88, number: 3 }), 'SKU-000088')
  assert.equal(productCodeLabel({ id: 88, product_code: 'P-2026-001', number: 3 }), 'P-2026-001')
})

test('product archive BOM binding options show active BOMs with version numbers', () => {
  const rows = activeProductionBomOptions([
    { id: 2, code: 'BOM-002', name: '失效配方', status: 'inactive', latest_version_no: 'V003' },
    { id: 1, code: 'BOM-001', name: '初晓拼配', status: 'active', latest_version_no: 'V002', group_name: '拼配' },
  ])

  assert.deepEqual(rows.map((row) => row.id), [1])
  assert.equal(productionBomOptionLabel(rows[0]), 'BOM-001 初晓拼配 / V002')
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

test('product production config has no industry fields without a selected template', () => {
  const legacyFields = [{
    field_key: 'roast_level',
    template_field_key: '',
    label: 'roast_level',
    field_type: 'text',
    value_text: '深烘',
    sort_order: 1,
  }]

  assert.deepEqual(productProductionConfigFieldsFromTemplate(legacyFields, null), [])

  const form = buildProductProductionConfigForm({
    product_id: 556,
    industry_field_template_id: 0,
    fields: legacyFields,
  }, { id: 556, name: '无模板旧商品' })

  assert.deepEqual(form.fields, [])
})

test('product production config form keeps only current template industry fields', () => {
  const roastTemplate = {
    id: 2,
    fields: [{
      field_key: '烘焙度',
      label: '烘焙度',
      field_type: 'select',
      options_json: '["浅烘","中烘","深烘"]',
      sort_order: 1,
    }],
  }

  const form = buildProductProductionConfigForm({
    product_id: 554,
    industry_field_template_id: 2,
    fields: [
      {
        field_key: '烘焙度',
        template_field_key: '烘焙度',
        label: '烘焙度',
        field_type: 'select',
        value_text: '深烘',
        options_json: '["浅烘","中烘","深烘"]',
        sort_order: 1,
      },
      {
        field_key: 'roast_level',
        template_field_key: '',
        label: 'roast_level',
        field_type: 'text',
        value_text: '中烘',
        sort_order: 1,
      },
    ],
  }, { id: 554, name: '榛巧拼配' }, roastTemplate)

  assert.equal(form.fields.length, 1)
  assert.equal(form.fields[0].field_key, '烘焙度')
  assert.equal(form.fields[0].template_field_key, '烘焙度')
  assert.equal(form.fields[0].field_type, 'select')
  assert.equal(form.fields[0].value_text, '深烘')

  const legacyOnly = buildProductProductionConfigForm({
    product_id: 555,
    industry_field_template_id: 2,
    fields: [{
      field_key: 'roast_level',
      template_field_key: '',
      label: 'roast_level',
      field_type: 'text',
      value_text: '中烘',
      sort_order: 1,
    }],
  }, { id: 555, name: '旧烘焙字段商品' }, roastTemplate)

  assert.equal(legacyOnly.fields.length, 1)
  assert.equal(legacyOnly.fields[0].field_key, '烘焙度')
  assert.equal(legacyOnly.fields[0].template_field_key, '烘焙度')
  assert.equal(legacyOnly.fields[0].field_type, 'select')
  assert.equal(legacyOnly.fields[0].value_text, '')
})

test('product production config form projects multiple industry templates in selected order with first-key wins', () => {
  const roastTemplate = {
    id: 2,
    fields: [
      { field_key: 'roast_level', label: '烘焙度', field_type: 'select', options_json: '["浅烘","深烘"]', sort_order: 1 },
      { field_key: 'origin', label: '产地', field_type: 'text', sort_order: 2 },
    ],
  }
  const packageTemplate = {
    id: 3,
    fields: [
      { field_key: 'ROAST_LEVEL', label: '包装烘焙标识', field_type: 'text', sort_order: 1 },
      { field_key: 'bag_count', label: '每盒袋数', field_type: 'number', sort_order: 2 },
    ],
  }
  const form = buildProductProductionConfigForm({
    product_id: 557,
    industry_field_template_ids: [2, 3],
    fields: [
      { field_key: 'roast_level', template_field_key: 'roast_level', value_text: '深烘' },
      { field_key: 'bag_count', template_field_key: 'bag_count', value_number: 10 },
    ],
  }, { id: 557, name: '挂耳拼配' }, [roastTemplate, packageTemplate])

  assert.deepEqual(form.industry_field_template_ids, [2, 3])
  assert.equal(form.industry_field_template_id, 2)
  assert.deepEqual(form.fields.map((field) => [field.field_key, field.label]), [
    ['roast_level', '烘焙度'],
    ['origin', '产地'],
    ['bag_count', '每盒袋数'],
  ])
  assert.equal(form.fields[0].value_text, '深烘')
  assert.equal(form.fields[2].value_number, 10)

  const legacy = buildProductProductionConfigForm({
    product_id: 558,
    industry_field_template_id: 3,
    fields: [],
  }, { id: 558 }, [packageTemplate])
  assert.deepEqual(legacy.industry_field_template_ids, [3])
})

test('industry template selector puts selected templates first in first-wins order and keeps unavailable selections clearable', () => {
  const options = industryFieldTemplateOptionsForConfig([
    { id: 1, name: '咖啡豆字段', status: 'active', sort_order: 20 },
    { id: 2, name: '旧挂耳字段', status: 'inactive', sort_order: 10 },
    { id: 3, name: '包装字段', status: 'active', sort_order: 10 },
  ], {
    industry_field_template_ids: [2, 99],
  })

  assert.deepEqual(options.map((template) => [template.id, template.selected_order, template.unavailable, template.status]), [
    [2, 1, true, 'inactive'],
    [99, 2, true, 'missing'],
    [3, 0, false, 'active'],
    [1, 0, false, 'active'],
  ])
})

test('product production config rejects label-only legacy industry field matches', () => {
  const roastTemplate = {
    id: 2,
    fields: [{
      field_key: '烘焙度',
      label: '烘焙度',
      field_type: 'select',
      options_json: '["浅烘","中烘","深烘"]',
      sort_order: 1,
    }],
  }
  const legacyFields = [{
    field_key: 'legacy_roast',
    template_field_key: '',
    label: '烘焙度',
    field_type: 'text',
    value_text: '中烘',
    sort_order: 1,
  }]

  const projected = productProductionConfigFieldsFromTemplate(legacyFields, roastTemplate)

  assert.equal(projected.length, 1)
  assert.equal(projected[0].field_key, '烘焙度')
  assert.equal(projected[0].template_field_key, '烘焙度')
  assert.equal(projected[0].value_text, '')
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
    gradient_template_id: 0,
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
    gradient_template_id: 0,
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

test('instant coffee SKU payload ignores legacy yield and SKU special attributes', () => {
  assert.deepEqual(buildProductCreatePayload({
    name: '速溶盒装',
    product_kind: 'instant_coffee',
    special_attr_values: { roast_level: '中烘' },
    yield_percent: 96,
  }), {
    name: '速溶盒装',
    product_kind: 'instant_coffee',
    remark: '',
  })

  assert.deepEqual(buildProductBasicsPayload({
    product_kind: 'instant_coffee',
    remark: '条装原料',
    special_attr_values: { roast_level: '中烘' },
    yield_percent: 98,
  }), {
    product_kind: 'instant_coffee',
    remark: '条装原料',
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
    default_sales_unit: '盒',
    unit_conversion_rows: [
      { from_qty: 1, from_unit: '盒', to_qty: 0.2, to_unit: 'kg' },
      { from_qty: 1, from_unit: '磅', to_qty: 0.453592, to_unit: 'kg' },
    ],
    integer_unit: true,
  }), {
    id: 12,
    name: '盒装200g',
    inventory_unit: 'kg',
    sales_unit: '盒',
    default_sales_unit: '盒',
    sales_units: ['kg', '盒', '磅'],
    quote_unit: '盒',
    order_unit: '盒',
    unit_conversion_json: '{"kg":{"kg":1},"盒":{"kg":0.2},"磅":{"kg":0.453592}}',
    integer_unit: true,
    active: true,
  })
})

test('unit template payload exposes default sales unit while dual-writing legacy quote and order units', () => {
  assert.deepEqual(buildProductUnitTemplatePayload({
    id: 18,
    name: ' 盒装10个 ',
    inventory_unit: ' 个 ',
    default_sales_unit: ' 盒 ',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '盒', to_qty: 10, to_unit: '个' }],
    integer_unit: true,
  }), {
    id: 18,
    name: '盒装10个',
    inventory_unit: '个',
    sales_unit: '盒',
    default_sales_unit: '盒',
    sales_units: ['个', '盒'],
    quote_unit: '盒',
    order_unit: '盒',
    unit_conversion_json: '{"个":{"个":1},"盒":{"个":10}}',
    integer_unit: true,
    active: true,
  })
})

test('unit template payload normalizes legacy sales-unit-to-sales-unit conversions to inventory unit', () => {
  assert.deepEqual(buildProductUnitTemplatePayload({
    id: 21,
    name: '箱装咖啡豆',
    inventory_unit: 'kg',
    default_sales_unit: '箱',
    unit_conversion_rows: [
      { from_qty: 1, from_unit: '箱', to_qty: 24, to_unit: '盒' },
      { from_qty: 1, from_unit: '盒', to_qty: 0.2, to_unit: 'kg' },
    ],
  }), {
    id: 21,
    name: '箱装咖啡豆',
    inventory_unit: 'kg',
    sales_unit: '箱',
    default_sales_unit: '箱',
    sales_units: ['kg', '箱', '盒'],
    quote_unit: '箱',
    order_unit: '箱',
    unit_conversion_json: '{"kg":{"kg":1},"箱":{"kg":4.8},"盒":{"kg":0.2}}',
    integer_unit: false,
    active: true,
  })
})

test('sales spec template payload carries template inventory unit target', () => {
  const payload = buildProductUnitTemplatePayload({
    id: 31,
    name: '咖啡袋装销售规格',
    inventory_unit: ' kg ',
    sales_spec_rows: [
      { spec_key: 'bag-227g', spec_name: '227g袋装', sales_unit: '袋', net_content_qty: 227, net_content_unit: 'g', default: true, active: true },
      { spec_key: 'bag-100g', spec_name: '100g袋装', net_content_qty: 0.1 },
    ],
  })
  assert.deepEqual(payload, {
    id: 31,
    name: '咖啡袋装销售规格',
    inventory_unit: 'kg',
    default_sales_unit: '227g袋装',
    sales_unit: '227g袋装',
    sales_units: ['227g袋装', '100g袋装'],
    quote_unit: '227g袋装',
    order_unit: '227g袋装',
    unit_conversion_json: '{}',
    sales_specs: [
      { spec_key: 'bag-227g', spec_name: '227g袋装', sales_unit: '227g袋装', net_content_qty: 0.227, net_content_unit: 'kg', default: true, active: true },
      { spec_key: 'bag-100g', spec_name: '100g袋装', sales_unit: '100g袋装', net_content_qty: 0.1, net_content_unit: 'kg', default: false, active: true },
    ],
    active: true,
  })
})

test('sales spec template payload keeps the selected default spec instead of forcing the first row', () => {
  const payload = buildProductUnitTemplatePayload({
    id: 31,
    name: '咖啡袋装销售规格',
    inventory_unit: 'kg',
    default_spec_key: 'bag-100g',
    sales_spec_rows: [
      { spec_key: 'bag-227g', spec_name: '227g袋装', net_content_qty: 0.227, default: false, active: true },
      { spec_key: 'bag-100g', spec_name: '100g袋装', net_content_qty: 0.1, default: true, active: true },
    ],
  })

  assert.equal(payload.default_sales_unit, '100g袋装')
  assert.deepEqual(payload.sales_specs.map((row) => ({ spec_key: row.spec_key, default: row.default })), [
    { spec_key: 'bag-227g', default: false },
    { spec_key: 'bag-100g', default: true },
  ])
})

test('sales spec rows decorate template specs and preserve derived child SKU status', () => {
  assert.deepEqual(salesSpecRowsFromTemplate({
    sales_specs: [
      { spec_key: 'bag-227g', spec_name: '227g袋装', sales_unit: '袋', net_content_qty: 227, net_content_unit: 'g', default: true, active: true, derived_sku_code: 'SKU-000912' },
      { spec_key: 'bag-100g', spec_name: '100g袋装', sales_unit: '袋', net_content_qty: 100, net_content_unit: 'g', active: false, derived_spec_status: 'template_disabled' },
    ],
  }), [
    { spec_key: 'bag-227g', spec_name: '227g袋装', sales_unit: '227g袋装', net_content_qty: 227, net_content_unit: 'g', default: true, active: true, derived_sku_code: 'SKU-000912', derived_spec_status: 'active' },
    { spec_key: 'bag-100g', spec_name: '100g袋装', sales_unit: '100g袋装', net_content_qty: 100, net_content_unit: 'g', default: false, active: false, derived_sku_code: '', derived_spec_status: 'template_disabled' },
  ])
})

test('sales spec conversion label explains sales unit to parent inventory unit', () => {
  assert.equal(salesSpecConversionLabel({
    spec_name: '227g袋装',
    net_content_qty: 227,
    net_content_unit: 'g',
  }, 'g'), '1 227g袋装 = 227 g')

  assert.equal(salesSpecConversionLabel({
    spec_name: '227g袋装',
    net_content_qty: 227,
    net_content_unit: 'g',
  }, 'kg'), '1 227g袋装 = 0.227 kg')

  assert.equal(salesSpecConversionLabel({
    spec_name: '箱装',
    net_content_qty: 12,
    net_content_unit: '袋',
  }, 'kg'), '1 箱装 = 12 袋（库存单位 kg，无法自动换算）')

  assert.equal(salesSpecConversionLabel({
    spec_name: '默认规格',
  }, 'g'), '换算待补：请填写库存数量')

  assert.equal(salesSpecConversionLabel({
    derived_spec_name: '100g袋装',
    derived_sales_unit: '旧销售单位',
    net_content_qty: 0.1,
    net_content_unit: 'kg',
  }, 'kg'), '1 100g袋装 = 0.1 kg')
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

test('new product ignores legacy unit authority while existing product basics remain compatible', () => {
  assert.deepEqual(buildProductCreatePayload({
    name: ' 盒装速溶 ',
    product_kind: 'instant_coffee',
    remark: ' 新品 ',
    yield_percent: 80,
    unit_rule_override_enabled: true,
    inventory_unit: ' 盒 ',
    integer_inventory_unit: true,
    default_sales_unit: ' 箱 ',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '箱', to_qty: 12, to_unit: '盒', integer_sales_unit: true }],
  }), {
    name: '盒装速溶',
    product_kind: 'instant_coffee',
    remark: '新品',
  })

  assert.deepEqual(buildProductBasicsPayload({
    name: ' 盒装速溶 ',
    product_kind: 'instant_coffee',
    remark: ' 库存按盒 ',
    yield_percent: 80,
    unit_rule_override_enabled: true,
    inventory_unit: ' 个 ',
    integer_inventory_unit: false,
    default_sales_unit: ' 盒 ',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '盒', to_qty: 10, to_unit: '个', integer_sales_unit: true }],
    unit_rule_override_json: '{"order_unit":"箱","legacy_key":"keep"}',
  }), {
    name: '盒装速溶',
    product_kind: 'instant_coffee',
    remark: '库存按盒',
    inventory_unit: '个',
    integer_inventory_unit: false,
    default_sales_unit: '盒',
    unit_conversion_json: { 盒: { 个: 10 } },
    sales_unit_rules: { 盒: { integer_unit: true } },
    unit_rule_override_json: '{"order_unit":"箱","legacy_key":"keep"}',
  })
})

test('new product omits sales spec template while existing product basics keep legacy template compatibility', () => {
  assert.deepEqual(buildProductCreatePayload({
    name: ' 模板咖啡豆 ',
    product_kind: 'roasted',
    remark: ' 引用咖啡豆单位模板 ',
    yield_percent: 80,
    unit_template_id: '7',
    default_sales_unit: '袋',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '袋', to_qty: 0.25, to_unit: 'kg', integer_sales_unit: true }],
    unit_rule_override_enabled: false,
  }), {
    name: '模板咖啡豆',
    product_kind: 'roasted',
    remark: '引用咖啡豆单位模板',
  })

  assert.deepEqual(buildProductCreatePayload({
    name: ' 例外盒装 ',
    product_kind: 'roasted',
    remark: ' 覆盖模板 ',
    yield_percent: 80,
    unit_template_id: '7',
    inventory_unit: ' 盒 ',
    integer_inventory_unit: true,
    default_sales_unit: ' 箱 ',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '箱', to_qty: 12, to_unit: '盒', integer_sales_unit: true }],
    unit_rule_override_enabled: true,
  }), {
    name: '例外盒装',
    product_kind: 'roasted',
    remark: '覆盖模板',
  })

  const inheritedPayload = buildProductBasicsPayload({
    name: ' 模板咖啡豆 ',
    product_kind: 'roasted',
    remark: ' 引用模板 ',
    yield_percent: 80,
    unit_template_id: '7',
    default_sales_unit: '袋',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '袋', to_qty: 0.25, to_unit: 'kg', integer_sales_unit: true }],
    unit_rule_override_enabled: false,
    unit_rule_override_json: '{"legacy_key":"keep"}',
  })
  assert.equal(inheritedPayload.unit_template_id, 7)
  assert.equal(inheritedPayload.unit_rule_override_json, '{"legacy_key":"keep"}')
  assert.equal(Object.hasOwn(inheritedPayload, 'inventory_unit'), false)
  assert.equal(Object.hasOwn(inheritedPayload, 'integer_inventory_unit'), false)
  assert.equal(Object.hasOwn(inheritedPayload, 'default_sales_unit'), false)
  assert.equal(Object.hasOwn(inheritedPayload, 'unit_conversion_json'), false)
  assert.equal(Object.hasOwn(inheritedPayload, 'sales_unit_rules'), false)

  const overridePayload = buildProductBasicsPayload({
    name: ' 例外盒装 ',
    product_kind: 'roasted',
    remark: ' 覆盖模板 ',
    yield_percent: 80,
    unit_template_id: '7',
    inventory_unit: ' 盒 ',
    integer_inventory_unit: true,
    default_sales_unit: ' 箱 ',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '箱', to_qty: 12, to_unit: '盒', integer_sales_unit: true }],
    unit_rule_override_enabled: true,
    unit_rule_override_json: '{"legacy_key":"keep"}',
  })
  assert.equal(overridePayload.unit_template_id, 7)
  assert.equal(overridePayload.inventory_unit, '盒')
  assert.equal(overridePayload.integer_inventory_unit, true)
  assert.equal(overridePayload.default_sales_unit, '箱')
  assert.deepEqual(overridePayload.unit_conversion_json, { 箱: { 盒: 12 } })
  assert.deepEqual(overridePayload.sales_unit_rules, { 箱: { integer_unit: true } })
  assert.equal(overridePayload.unit_rule_override_json, '{"legacy_key":"keep"}')
})

test('product production config save does not write template inventory unit as product override', () => {
  const inheritedProduct = {
    id: 88,
    name: '初晓拼配',
    product_kind: 'roasted',
    unit_template_id: 7,
    inventory_unit: 'kg',
    default_sales_unit: 'kg',
    unit_conversion_json: '{"kg":{"kg":1}}',
    sales_unit_rules: '{}',
    unit_rule_override_json: '{"legacy_key":"keep"}',
  }
  const inheritedForm = buildProductProductionConfigForm(null, inheritedProduct)
  const inheritedPayload = buildProductProductionConfigBasicsPayload(inheritedProduct, inheritedForm)
  assert.equal(Object.hasOwn(inheritedPayload, 'default_sales_unit'), false)
  assert.equal(Object.hasOwn(inheritedPayload, 'unit_conversion_json'), false)
  assert.equal(Object.hasOwn(inheritedPayload, 'sales_unit_rules'), false)
  assert.equal(Object.hasOwn(inheritedPayload, 'inventory_unit'), false)

  const editedForm = {
    ...inheritedForm,
    default_sales_unit: '盒',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '盒', to_qty: 0.2, to_unit: 'kg', integer_sales_unit: true }],
  }
  const editedPayload = buildProductProductionConfigBasicsPayload(inheritedProduct, editedForm)
  assert.equal(Object.hasOwn(editedPayload, 'default_sales_unit'), false)
  assert.equal(Object.hasOwn(editedPayload, 'unit_conversion_json'), false)
  assert.equal(Object.hasOwn(editedPayload, 'sales_unit_rules'), false)
})

test('product basics payload preserves legacy product unit override without visible override controls', () => {
  const payload = buildProductBasicsPayload({
    name: '磅装咖啡豆',
    unit_template_id: 3,
    unit_rule_source: 'product_override',
    unit_rule_override_json: '{"inventory_unit":"袋","default_sales_unit":"磅","legacy_key":"keep"}',
    inventory_unit: '袋',
    integer_inventory_unit: true,
    default_sales_unit: '磅',
    unit_conversion_json: { 磅: { 袋: 1 } },
    sales_unit_rules: { 磅: { integer_unit: true } },
  })

  assert.equal(payload.unit_template_id, 3)
  assert.equal(payload.inventory_unit, '袋')
  assert.equal(payload.integer_inventory_unit, true)
  assert.equal(payload.default_sales_unit, '磅')
  assert.deepEqual(payload.unit_conversion_json, { 磅: { 袋: 1 } })
  assert.deepEqual(payload.sales_unit_rules, { 磅: { integer_unit: true } })
  assert.equal(payload.unit_rule_override_json, '{"inventory_unit":"袋","default_sales_unit":"磅","legacy_key":"keep"}')
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

test('new product drawer delegates specs to BOM while legacy config edits retain template compatibility', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const createForm = source.match(/<form class="sku-create-form product-create-form product-drawer-form"[\s\S]*?<\/form>/)?.[0] || ''
  const configDrawer = source.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''
  const baseSection = configDrawer.match(/<strong>基础信息<\/strong>[\s\S]*?<\/section>/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(createForm, /建档后[\s\S]*BOM[\s\S]*至少一个规格/)
  assert.doesNotMatch(createForm, /skuForm\.unit_template_id/)
  assert.doesNotMatch(createForm, /<span>销售规格模板<\/span>/)
  assert.doesNotMatch(createForm, /销售规格模板明细/)
  for (const removed of [
    '销售规格模板',
    'productProductionConfigForm.unit_template_id',
    '库存单位：来自销售规格模板',
    '销售规格模板明细',
  ]) {
    assert.doesNotMatch(baseSection, new RegExp(removed.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `config base section must not contain ${removed}`)
  }
  for (const removed of [
    '不引用单位模板',
    '高级单位覆盖',
    '清除覆盖',
    '<span>库存单位</span>',
    '<span>整数库存</span>',
    '销售单位换算',
    'skuForm.inventory_unit',
    'skuForm.unit_rule_override_enabled',
    'skuForm.integer_inventory_unit',
    'skuForm.default_sales_unit',
    'skuForm.unit_conversion_rows',
    'productProductionConfigForm.inventory_unit',
    'productProductionConfigForm.unit_rule_override_enabled',
    'productProductionConfigForm.integer_inventory_unit',
    'productProductionConfigForm.default_sales_unit',
    'productProductionConfigForm.unit_conversion_rows',
  ]) {
    assert.doesNotMatch(createForm, new RegExp(removed.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `create form should not contain ${removed}`)
    assert.doesNotMatch(baseSection, new RegExp(removed.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `config base section should not contain ${removed}`)
  }
  const createBlock = script.match(/async function createSku\(\) \{[\s\S]*?\n\}/)?.[0] || ''
  assert.doesNotMatch(createBlock, /请选择销售规格模板/)
})

test('product archive config drawer shows BOM specs directly without per-spec SKU maintenance', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const configDrawer = source.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''

  assert.match(configDrawer, /BOM 规格（只读）/)
  assert.match(configDrawer, /bom-spec-readonly-panel/)
  assert.match(configDrawer, /到 BOM 维护规格/)
  assert.match(configDrawer, /productProductionBomSpecs/)
  assert.doesNotMatch(configDrawer, /销售规格模板明细/)
  assert.doesNotMatch(configDrawer, /显示历史规格/)
  assert.doesNotMatch(configDrawer, /SKU 编号/)
  assert.doesNotMatch(configDrawer, /derivedSkuCodeLabel\(row\)/)
  assert.doesNotMatch(configDrawer, /设为默认规格/)
  assert.doesNotMatch(configDrawer, /setDefaultProductSalesSpec/)
  assert.doesNotMatch(configDrawer, /v-if="productProductionConfigUsesBomSpecs" class="sales-spec-template-detail bom-spec-readonly-panel"/)
  assert.match(source, /尚未绑定默认制造 BOM|暂无可用规格/)
})

test('product archive drops per-spec SKU row maintenance from the config drawer', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  for (const retired of ['productProductionDerivedSkuRows', 'productProductionSalesSpecRows', 'productProductionVisibleSalesSpecRows', 'showProductProductionHistoricalSpecs', 'setDefaultProductSalesSpec', 'createChildSkuForProduct', 'childSkuForm']) {
    assert.ok(!source.includes(retired), `config drawer must not keep retired SKU helper ${retired}`)
  }
})

test('sales spec template controls remain only for legacy product configuration', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const createForm = source.match(/<form class="sku-create-form product-create-form product-drawer-form"[\s\S]*?<\/form>/)?.[0] || ''
  const configDrawer = source.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''
  const productListToolbar = source.match(/<div class="filter-actions sku-list-actions"[\s\S]*?<\/div>/)?.[0] || ''

  assert.doesNotMatch(createForm, /skuForm\.unit_template_id/)
  assert.doesNotMatch(createForm, /<span>销售规格模板<\/span>/)
  assert.match(createForm, /商品档案不再维护销售规格模板或派生子 SKU/)

  for (const removed of [
    '销售规格模板',
    'productProductionConfigForm.unit_template_id',
    '库存单位：来自销售规格模板',
  ]) {
    assert.doesNotMatch(configDrawer, new RegExp(removed.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `config drawer must not contain ${removed}`)
  }

  assert.doesNotMatch(productListToolbar, /设置销售规格模板/)
  assert.doesNotMatch(productListToolbar, /维护销售规格模板/)
  assert.doesNotMatch(productListToolbar, /openProductUnitTemplateManagement/)
  assert.doesNotMatch(productListToolbar, /batchProductUnitTemplateID/)
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

test('skuGroupTableState paginates every product category independently', () => {
  const coffeeRows = Array.from({ length: 12 }, (_, index) => ({
    id: `coffee-${index + 1}`,
    name: `咖啡豆 ${index + 1}`,
  }))
  const dripRows = Array.from({ length: 13 }, (_, index) => ({
    id: `drip-${index + 1}`,
    name: `挂耳咖啡 ${index + 1}`,
  }))

  const state = skuGroupTableState([
    { key: 'coffee', label: '咖啡豆', rows: coffeeRows },
    { key: 'drip', label: '挂耳咖啡', rows: dripRows },
  ], {
    coffee: { page: 2, pageSize: 10 },
    drip: { page: 1, pageSize: 10 },
  })

  assert.equal(state.groups[0].total, 12)
  assert.equal(state.groups[0].page, 2)
  assert.deepEqual(state.groups[0].rows.map((row) => row.id), ['coffee-11', 'coffee-12'])
  assert.equal(state.groups[1].total, 13)
  assert.equal(state.groups[1].page, 1)
  assert.deepEqual(state.groups[1].rows.map((row) => row.id), dripRows.slice(0, 10).map((row) => row.id))
  assert.deepEqual(state.pagination, {
    coffee: { page: 2, pageSize: 10 },
    drip: { page: 1, pageSize: 10 },
  })
  assert.equal(state.visibleRows.length, 12)
})

test('skuGroupTableState keeps full totals, clamps pages, and counts parent products only', () => {
  const parentRows = [{
    id: 1,
    name: '金色山脉',
    sku_rows: Array.from({ length: 6 }, (_, index) => ({ id: 100 + index })),
  }]
  const state = skuGroupTableState([
    { key: 'coffee', label: '咖啡豆', rows: parentRows },
    { key: 'empty', label: '空分类', rows: [] },
  ], {
    coffee: { page: 9, pageSize: 10 },
    empty: { page: 3, pageSize: 10 },
  })

  assert.equal(state.groups[0].total, 1)
  assert.equal(state.groups[0].page, 1)
  assert.equal(state.groups[0].rows.length, 1)
  assert.equal(state.groups[0].needsPagination, false)
  assert.equal(state.groups[1].total, 0)
  assert.equal(state.groups[1].page, 1)
  assert.equal(state.groups[1].needsPagination, false)
})

test('visibleSkuGroupRows excludes collapsed categories from visible bulk selection', () => {
  const groups = [
    { key: 'coffee', rows: [{ id: 1 }, { id: 2 }] },
    { key: 'drip', rows: [{ id: 3 }] },
  ]

  assert.deepEqual(visibleSkuGroupRows(groups, ['coffee']).map((row) => row.id), [3])
  assert.deepEqual(visibleSkuGroupRows(groups, []).map((row) => row.id), [1, 2, 3])
})

test('visibleSkuGroupRows excludes descendants of a collapsed parent without collapsing siblings', () => {
  const groups = [
    { key: 'business-group-9-90', group_id: 9, group_item_id: 90, parent_group_item_id: 0, rows: [{ id: 1 }] },
    { key: 'business-group-9-92', group_id: 9, group_item_id: 92, parent_group_item_id: 90, rows: [{ id: 2 }] },
    { key: 'business-group-9-93', group_id: 9, group_item_id: 93, parent_group_item_id: 92, rows: [{ id: 3 }] },
    { key: 'business-group-9-91', group_id: 9, group_item_id: 91, parent_group_item_id: 0, rows: [{ id: 4 }] },
    { key: 'business-group-unclassified', group_id: 0, group_item_id: 0, parent_group_item_id: 0, rows: [{ id: 5 }] },
  ]

  assert.deepEqual(
    visibleSkuGroupRows(groups, ['business-group-9-90']).map((row) => row.id),
    [4, 5],
  )
  assert.deepEqual(
    visibleSkuGroupRows(groups, ['business-group-9-92']).map((row) => row.id),
    [1, 4, 5],
  )
})

test('collapsing one selected template parent does not hide the other selected template tree', () => {
  const groups = [
    { key: 'business-group-9-90', group_id: 9, group_item_id: 90, parent_group_item_id: 0, rows: [{ id: 1 }] },
    { key: 'business-group-9-92', group_id: 9, group_item_id: 92, parent_group_item_id: 90, rows: [{ id: 2 }] },
    { key: 'business-group-10-90', group_id: 10, group_item_id: 90, parent_group_item_id: 0, rows: [{ id: 3 }] },
    { key: 'business-group-10-92', group_id: 10, group_item_id: 92, parent_group_item_id: 90, rows: [{ id: 4 }] },
  ]

  assert.deepEqual(
    visibleSkuGroupRows(groups, ['business-group-9-90']).map((row) => row.id),
    [3, 4],
  )
})

test('collapsing a group template header hides all of its categories but not other templates', () => {
  const groups = [
    { key: 'business-template-9', group_id: 9, group_item_id: 0, is_template_group: true, rows: [] },
    { key: 'business-group-9-90', group_id: 9, group_item_id: 90, parent_group_item_id: 0, rows: [{ id: 1 }] },
    { key: 'business-group-9-92', group_id: 9, group_item_id: 92, parent_group_item_id: 90, rows: [{ id: 2 }] },
    { key: 'business-template-10', group_id: 10, group_item_id: 0, is_template_group: true, rows: [] },
    { key: 'business-group-10-90', group_id: 10, group_item_id: 90, parent_group_item_id: 0, rows: [{ id: 3 }] },
    { key: 'business-group-unclassified', group_id: 0, group_item_id: 0, parent_group_item_id: 0, rows: [{ id: 4 }] },
  ]

  assert.deepEqual(
    visibleSkuGroupRows(groups, ['business-template-9']).map((row) => row.id),
    [3, 4],
  )
  assert.deepEqual(
    visibleSkuGroupRows(groups, []).map((row) => row.id),
    [1, 2, 3, 4],
  )
})

test('visible bulk selection preserves hidden descendant selections when a parent group is collapsed', () => {
  const visibleRows = [{ id: 4 }, { id: 5 }]

  assert.deepEqual(selectedSkuRowIDsAfterVisibleToggle([2], visibleRows, true), [2, 4, 5])
  assert.deepEqual(selectedSkuRowIDsAfterVisibleToggle([2, 4, 5], visibleRows, false), [2])
})

test('category filter options are derived from current SKU rows', () => {
  assert.deepEqual(primaryCategoryOptions(rows), ['咖啡豆', '生豆'])
  assert.deepEqual(secondaryCategoryOptions(rows, '生豆'), ['单品生豆', '拼配生豆'])
})

test('product create payload carries SKU remark without direct green bean prices or hard-coded green bean attributes', () => {
  const roasted = buildProductCreatePayload({ name: '暖阳拼配', product_kind: 'roasted', roast_level: '中烘', yield_percent: 82, remark: '奶咖主推' })
  assert.deepEqual(roasted, {
    name: '暖阳拼配',
    product_kind: 'roasted',
    remark: '奶咖主推',
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
  })
  assert.equal(Object.hasOwn(green, 'green_bean_type'), false)
  assert.equal(Object.hasOwn(green, 'green_bean_bom_product_id'), false)
})

test('product basics payload preserves remark without hard-coded green bean attributes', () => {
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
  })
  assert.equal(Object.hasOwn(payload, 'green_bean_type'), false)
  assert.equal(Object.hasOwn(payload, 'green_bean_bom_product_id'), false)
})

test('product basics payload no longer writes hard-coded drip bag package fields', () => {
  const payload = buildProductBasicsPayload({
    id: 10,
    product_kind: 'drip_bag',
    drip_bag_grams: 12,
    drip_box_bag_count: 8,
    yield_percent: 82,
    remark: '挂耳由商品配置模板承载规格',
  }, null)

  assert.equal(payload.product_kind, 'drip_bag')
  assert.equal(payload.remark, '挂耳由商品配置模板承载规格')
  assert.equal(Object.hasOwn(payload, 'yield_rate'), false)
  assert.equal(Object.hasOwn(payload, 'drip_bag_grams'), false)
  assert.equal(Object.hasOwn(payload, 'drip_box_bag_count'), false)
})

test('product basics payload no longer carries customer SKU margin override', () => {
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
  assert.equal(Object.hasOwn(payload, 'yield_rate'), false)
  assert.equal(Object.hasOwn(payload, 'margin_rate_override'), false)
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

test('customer custom SKU payload supports green bean and drip bag without hard-coded product archive fields', () => {
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

test('product archive list does not expose hard-coded green bean attributes or bound roasted controls', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const helperSource = fs.readFileSync(new URL('./product-settings.js', import.meta.url), 'utf8')

  for (const removed of [
    '生豆属性',
    '绑定熟豆',
    'greenBeanTypeOptions',
    'roastedBomProductsForRow',
    'roastedBomProductOptions',
  ]) {
    assert.doesNotMatch(source, new RegExp(removed.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.doesNotMatch(template, /v-model="row\.green_bean_type"/)
  assert.doesNotMatch(template, /v-model="row\.green_bean_bom_product_id"/)
  assert.doesNotMatch(helperSource, /greenBeanTypeOptions/)
  assert.doesNotMatch(helperSource, /greenBeanTypeLabel/)
  assert.doesNotMatch(helperSource, /roastedBomProductOptions/)
})

test('product archive list does not expose BOM usage column or hard-coded drip package editors', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.match(template, /class="[^"]*sku-name-button[^"]*"[\s\S]*@click="openProductProductionConfig\(row\)"/)
  assert.match(template, /被哪些 BOM 使用/)
  assert.doesNotMatch(template, /<th>BOM 使用<\/th>/)
  assert.doesNotMatch(template, /查看使用关系/)
  assert.doesNotMatch(template, /bom-source-cell/)
  assert.doesNotMatch(template, /每袋克[重数]/)
  assert.doesNotMatch(template, /每盒袋数/)
  assert.doesNotMatch(template, /v-model\.number="row\.drip_bag_grams"/)
  assert.doesNotMatch(template, /v-model\.number="row\.drip_box_bag_count"/)
  assert.doesNotMatch(template, /row\.product_kind === 'drip_bag'/)
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

test('product pages split product archive, aliases, category management and pricing without workspace tabs', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const app = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  for (const expected of [
    'productMaster',
    'customerProductAliases',
    'productCategoryManagement',
    'productPriceManagement',
    'productConfigTemplates',
    '商品档案',
    '客户商品',
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
  assert.doesNotMatch(configPageBlock, /productConfigTemplateForm\.unit_template_id/)
  assert.match(configPageBlock, /价格表生成规则/)
  assert.match(configPageBlock, /productConfigTemplateNeedsGradientTemplate\(productConfigTemplateForm\)[\s\S]*阶梯价模板/)
  assert.doesNotMatch(configPageBlock, /<label>\s*<span>阶梯价模板<\/span>\s*<select v-model\.number="productConfigTemplateForm\.gradient_template_id"/)
  assert.doesNotMatch(configPageBlock, />商品分类管理</)
  assert.doesNotMatch(template, /currentSettingsSection === 'master'[\s\S]*class="category-panel category-drawer-panel category-management-panel/)
  assert.doesNotMatch(template, /activeConfigTemplateSection === 'classification-template'/)
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

test('product archive config drawer owns template, industry fields and split BOM production/default relations', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const bomSource = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(source, /商品档案配置/)
  assert.match(source, /商品配置模板/)
  assert.match(source, /行业字段模板/)
  assert.match(source, /可生产该商品的 BOM/)
  assert.match(source, /作为组件被哪些 BOM 使用/)
  assert.match(source, /productProductionConfigUsedByBomRows/)
  assert.match(source, /productProductionConfigProduceBomRows/)
  assert.match(source, /setDefaultProductionBom/)
  assert.match(source, /\/api\/products\/\$\{productID\}\/default-production-bom/)
  assert.match(source, /default_production_bom_id:\s*Number\(row\.bom_id/)
  assert.doesNotMatch(source, /production_bom_version_id:\s*Number\(row\.current_published_version_id/)
  assert.match(source, /bomUsageRelationLabel/)
  assert.match(source, /bomUsageStatusLabel/)
  assert.match(source, /BOM状态/)
  assert.match(source, /默认状态/)
  assert.match(source, /启用状态/)
  assert.match(source, /失效状态/)
  assert.match(source, /ensureProductBomUsage/)
  assert.match(source, /\/api\/production-bom-product-usage\/\$\{id\}/)
  assert.match(source, /show_in_price_list/)
  assert.match(source, /\/api\/product-production-configs/)
  assert.match(source, /openProductProductionConfig\(row\)/)
  assert.match(source, /productProductionConfigDrawerOpen/)
  assert.match(source, /保存商品档案配置/)
  assert.doesNotMatch(source, /addProductProductionConfigField/)
  assert.match(source, /productProductionConfigForm\.fields/)
  assert.doesNotMatch(source, /维护当前 BOM 明细/)
  assert.doesNotMatch(source, /placeholder="搜索有效生产 BOM"/)
  assert.doesNotMatch(source, /<span>BOM版本<\/span>/)
  assert.doesNotMatch(source, /绑定生产 BOM/)
  assert.doesNotMatch(source, /解绑生产 BOM/)
  assert.doesNotMatch(source, /生产反查/)
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

test('SKU settings labels group levels as large and small group items', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  assert.match(source, /新增大类/)
  assert.match(source, /个小类/)
  assert.doesNotMatch(source, /一级分类/)
  assert.doesNotMatch(source, /二级分类/)
})

test('SKU creation uses one unified product archive form without legacy classification fields', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const productDrawer = source.match(/<aside class="settings-drawer product-editor-drawer"[\s\S]*?<\/aside>/)?.[0] || ''

  assert.match(source, /@submit\.prevent="createSku"/)
  assert.match(script, /apiSend\('\/api\/product-settings\/products'/)
  assert.doesNotMatch(script, /\/api\/product-settings\/skus/)
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
    'productUnitTemplateSummary',
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
  assert.doesNotMatch(source, />报价单位</)
  assert.doesNotMatch(source, />录单单位</)
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
    '配置模板不再单独引用规格模板',
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
  const skuTable = template.match(/<table[^>]*class="sku-table"[\s\S]*?<\/table>/)?.[0] || template

  assert.match(template, /生产 BOM/)
  assert.match(template, /class="[^"]*sku-name-button[^"]*"[\s\S]*@click="openProductProductionConfig\(row\)"/)
  assert.ok(template.indexOf('<th class="sku-name-cell">商品名</th>') < template.indexOf('<th>商品编号</th>'), '商品名 must be the first business column before 商品编号')
  assert.doesNotMatch(skuTable, /BOM 使用/)
  assert.doesNotMatch(skuTable, /查看使用关系/)
  assert.doesNotMatch(source, /productionBomLabel\(row\)/)
  assert.doesNotMatch(source, /productionBomVersionWarning\(row\)/)
  assert.doesNotMatch(template, /当前引用/)
  assert.doesNotMatch(template, />生产配置<\/button>/)
  assert.doesNotMatch(template, /更换生产 BOM/)
  assert.doesNotMatch(template, />维护 BOM<\/button>/)
  assert.doesNotMatch(template, /<th>BOM<\/th>/)
  assert.doesNotMatch(template, /product-action-guide/)
  assert.doesNotMatch(template, /production-config-summary/)
  assert.match(source, /bomUsageStatusLabel/)
  assert.match(template, /BOM状态/)
  assert.doesNotMatch(source, /缺BOM/)
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

  assert.match(createSkuBlock, /const result = await apiSend\('\/api\/product-settings\/products'/)
  assert.match(createSkuBlock, /buildProductCreatePayload\(skuForm\.value\)/)
  assert.doesNotMatch(createSkuBlock, /请选择销售规格模板/)
  assert.match(createSkuBlock, /await loadAll\(\)[\s\S]*resolveCreatedProductForConfig\(result/)
  assert.match(createSkuBlock, /await openProductProductionConfig\(createdProductForConfig\)/)
})

test('new product archive delegates all specification authority to BOM', () => {
  assert.deepEqual(buildProductCreatePayload({
    name: ' BOM 规格新品 ',
    product_kind: 'roasted',
    remark: ' 建档后配 BOM ',
    unit_template_id: 77,
    unit_rule_override_enabled: true,
    inventory_unit: '袋',
    integer_inventory_unit: true,
    default_sales_unit: '箱',
    unit_conversion_rows: [{ from_qty: 1, from_unit: '箱', to_qty: 12, to_unit: '袋' }],
  }), {
    name: 'BOM 规格新品',
    product_kind: 'roasted',
    remark: '建档后配 BOM',
  })

  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const createForm = source.match(/<form class="sku-create-form product-create-form product-drawer-form"[\s\S]*?<\/form>/)?.[0] || ''
  assert.doesNotMatch(createForm, /<span>销售规格模板<\/span>/)
  assert.doesNotMatch(createForm, /skuForm\.unit_template_id/)
  assert.match(createForm, /建档后[\s\S]*BOM[\s\S]*至少一个规格/)
})

test('product production config form tolerates newly created products without production config rows', () => {
  const form = buildProductProductionConfigForm(null, {
    id: 812,
    name: '新建商品',
    remark: '刚创建',
    product_kind: 'roasted',
    production_bom_id: 0,
  })

  assert.equal(form.product_id, 812)
  assert.equal(form.name, '新建商品')
  assert.equal(form.remark, '刚创建')
  assert.equal(Object.hasOwn(form, 'product_config_template_id'), false)
  assert.equal(Object.hasOwn(form, 'expected_loss_percent'), false)
  assert.deepEqual(form.fields, [])
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
    '价格摘要',
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
  assert.match(template, /<BusinessGroupInlineWorkspace[\s\S]*v-model:collapsed-keys="collapsedProductClassificationGroups"/)
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
    '客户商品',
    'customer-alias-workspace',
    '客户商品编号',
    '重命名',
    '进入价格表',
    '绑定商品档案',
    '价格摘要',
    'customerProductAliases',
    'buildCustomerProductAliasPayload',
    '/api/customer-product-aliases',
    'saveCustomerProductAlias',
    'disableCustomerProductAlias',
    '客户商品只维护对外名称、编号、重命名和价格表展示',
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
  assert.doesNotMatch(aliasForm, /customerProductAliasForm\.gradient_template_id/)
  assert.doesNotMatch(aliasForm, /customerProductAliasForm\.unit_template_id/)
  assert.doesNotMatch(aliasForm, /customerProductAliasForm\.product_config_template_id/)
  assert.doesNotMatch(inlineAliasArea, /<th>品牌名<\/th>/)
  assert.doesNotMatch(inlineAliasArea, /alias\.brand_name\s*\|\|\s*'-'/)
  assert.doesNotMatch(aliasForm, />进入价格表</)
  assert.doesNotMatch(inlineAliasArea, /<form class="customer-alias-form"/)
  assert.match(aliasFilters, /新建客户商品/)
  assert.match(aliasFilters, /批量失效/)
  assert.doesNotMatch(aliasFilters, />搜索客户商品</)
  assert.doesNotMatch(template, />编辑<\/button>/)
  assert.doesNotMatch(template, /客户商品名/)
  assert.doesNotMatch(template, /客户商品[\s\S]*派生自有 BOM/)
  assert.doesNotMatch(template, /customer-alias-workspace[\s\S]*@click="derive/)
  assert.doesNotMatch(template, /旧客户 SKU 收敛检查/)
  assert.doesNotMatch(source, /aliasMigrationCandidates/)
  assert.doesNotMatch(source, /migration-candidates/)
})

test('SKU settings keeps only the product creation drawer while business groups drive the category workspace', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''
  const productArchiveWorkspace = template.match(/<div v-show="currentSettingsSection === 'master'"[\s\S]*?<div v-show="currentSettingsSection === 'templates'"/)?.[0] || template

  for (const expected of [
    'BusinessGroupInlineWorkspace',
    'collapsedProductClassificationGroups',
    'productCategoryMoveActive',
    'handleProductCategoryMoveTarget',
    'businessGroupInlineListState',
    'handleProductGroupPaginationChange',
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
  assert.doesNotMatch(template, /data-section-mode="groupManagement"/)
  assert.doesNotMatch(productArchiveWorkspace, /v-for="primary in visibleCategoryManagementTreeForSkuContext"/)
  assert.doesNotMatch(productArchiveWorkspace, /class="category-panel category-drawer-panel category-management-panel"/)
  assert.doesNotMatch(template, /<aside class="settings-drawer sku-copy-drawer"/)
  assert.doesNotMatch(template, /当前SKU \{\{ skuDisplayTotal \}\}/)
  assert.match(template, /<BusinessGroupInlineWorkspace[\s\S]*:groups="displaySkuGroups"/)
  assert.match(template, /#group="\{ group \}"[\s\S]*<table[^>]*class="sku-table"[\s\S]*<thead>/)
  assert.match(template, /v-for="row in group\.rows"/)
  assert.match(template, /<PaginationControls[\s\S]*group\.needsPagination[\s\S]*handleProductGroupPaginationChange\(group\.key, \$event\)/)
  assert.match(template, /当前分类暂无商品档案/)
  assert.doesNotMatch(productArchiveWorkspace, /:key="skuPaginationKey"/)
  assert.doesNotMatch(productArchiveWorkspace, /:total="skuDisplayTotal"/)
  assert.match(script, /const customerID = skuContextCustomerID\.value\s+return sortRowsForCustomerSkuPriority\(/)
  assert.match(script, /product\) => customerID > 0 && skuContextProductFilter\(product\)/)
  assert.match(script, /const currentSkuSourceRows = computed\(\(\) => \(/)
  assert.match(script, /skuContextCustomerID\.value > 0 \? customerSkuRows\.value : publicSkuRows\.value/)
  assert.match(script, /const normalizedSkuFilters = computed\(\(\) => normalizeVisibleSkuFilters\(skuFilters\.value, currentSkuSourceRows\.value\)\)/)
  assert.match(script, /const filteredSkuRows = computed\(\(\) => filterSkuRows\(currentSkuSourceRows\.value, normalizedSkuFilters\.value\)\)/)
  assert.match(script, /const skuDisplayKey = computed/)
  const skuDisplayKeyBlock = script.slice(script.indexOf('const skuDisplayKey = computed'), script.indexOf('const skuTableKey = computed'))
  assert.doesNotMatch(skuDisplayKeyBlock, /selectedProductGroupTemplateID/)
  assert.match(script, /const skuTableKey = computed\(\(\) => `\$\{skuDisplayKey\.value\}:table`\)/)
  assert.match(script, /const fullDisplaySkuGroups = computed\(\(\) => groupRowsByBusinessGroupTemplates\(filteredSkuRows\.value, \{/)
  assert.match(script, /templates: productCatalogBusinessGroups\.value/)
  assert.match(script, /const productInlineGroupState = computed\(\(\) => businessGroupInlineListState\(fullDisplaySkuGroups\.value, skuGroupPagination\.value, \{/)
  assert.match(script, /const displaySkuGroups = computed\(\(\) => productInlineGroupState\.value\.groups\)/)
  assert.match(script, /const visibleDisplaySkuRows = computed\(\(\) => businessGroupVisibleRows\(displaySkuGroups\.value, collapsedProductClassificationGroups\.value\)\)/)
  assert.match(script, /const skuPrimaryCategoryOptions = computed\(\(\) => primaryCategoryOptions\(currentSkuSourceRows\.value\)\)/)
  assert.match(script, /const skuSecondaryCategoryOptions = computed\(\(\) => secondaryCategoryOptions\(currentSkuSourceRows\.value, normalizedSkuFilters\.value\.primaryCategory\)\)/)
  assert.doesNotMatch(script, /const skuRenderRows = computed/)
  assert.doesNotMatch(script, /const skuRenderTotal = computed/)
  assert.match(script, /function syncVisibleSkuTableState\(\)/)
  assert.match(script, /function handleProductGroupPaginationChange\(groupKey, \{ page, pageSize \}\)/)
  assert.match(script, /function resetSkuGroupPages\(\) \{\s+if \(restoringProductSettingsDraft\) return/)
  assert.match(script, /watch\(skuFilters, resetSkuGroupPages, \{ deep: true \}\)/)
  assert.doesNotMatch(script, /selectedProductBusinessGroupCategoryKey|businessGroupGroupsForCategorySelection/)
  assert.match(script, /async function handleProductCategoryMoveTarget\(target\)/)
  assert.match(script, /skuGroupPagination: skuGroupPagination\.value/)
  assert.match(script, /watch\(filteredSkuRows, \(rows\) => \{\s+pruneSelectedProducts\(rows\)/)
  assert.match(script, /function toggleProductGroupRows\(group, checked\) \{\s+selectedProductIds\.value = selectedSkuRowIDsAfterVisibleToggle\(/)
  assert.doesNotMatch(script, /displaySkuRows\.value = pageState\.rows|const pageState = sliceVisibleSkuRows/)
  assert.match(script, /watch\(\[\s*publicSkuRows,\s*customerSkuRows,\s*skuFilters,\s*selectedCustomerSkuCustomerID,\s*\], syncVisibleSkuTableState, \{ deep: true, immediate: true \}\)/)
  assert.match(script, /applyWorkspaceCustomerContext\(\)\s+syncVisibleSkuTableState\(\)\s+pruneSelectedProducts\(filteredSkuRows\.value\)/)
  assert.match(script, /await nextTick\(\)\s+syncVisibleSkuTableState\(\)\s+restoringProductSettingsDraft = false/)
  assert.doesNotMatch(script, /const skuTable = computed/)
  assert.doesNotMatch(source, /debug_sku_table|__kferpSkuTableDebug|skuTableDebugAttr|data-sku-debug|data-top-|data-sku-instance/)
  assert.match(workspaceSource, /data-business-group-inline-workspace/)
  assert.doesNotMatch(workspaceSource, /business-group-category-tree/)
  assert.match(style, /\.settings-drawer-mask\s*\{[^}]*position:\s*fixed;/s)
})

test('legacy SKU category management is not rendered as the product archive classification entry', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.doesNotMatch(template, /<Teleport\s+to="#sku-category-management-target"/)
  assert.doesNotMatch(template, /id="sku-category-management-target"/)
  assert.doesNotMatch(template, /currentSettingsSection === 'master'[\s\S]*class="category-panel category-drawer-panel category-management-panel"/)
  assert.match(template, /<BusinessGroupInlineWorkspace[\s\S]*v-model:collapsed-keys="collapsedProductClassificationGroups"/)
  assert.match(template, /class="classification-view-toolbar alias-classification-tabs"/)
})

test('legacy classification template editors are not rendered in product settings UI', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.doesNotMatch(template, /activeConfigTemplateSection === 'classification-template'/)
  assert.doesNotMatch(template, /classification-template-pane/)
  assert.doesNotMatch(template, /classification-category-editor/)
  assert.doesNotMatch(template, /openClassificationTemplateCreateDrawer/)
  assert.doesNotMatch(template, /saveClassificationCategory/)
  assert.doesNotMatch(template, /product-classification-template-categories/)
  assert.doesNotMatch(template, /新建分类模板/)
  assert.doesNotMatch(source, /category-editor-drawer/)
  assert.doesNotMatch(source, /openCategoryDrawer/)
  assert.doesNotMatch(source, /openCategorySettingsDrawer/)
  assert.doesNotMatch(template, /归属客户/)
  assert.doesNotMatch(template, /对象归类/)
  assert.doesNotMatch(template, /配置分类/)
})

test('product list moves selected rows through business group assignments while alias classification stays legacy-compatible', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'data-pr442-product-group-assignments',
    'saveSelectedProductBusinessGroupAssignment',
    '/api/business-group-assignments',
    "usage_key: 'product_catalog'",
    "object_key: 'product'",
    'collapsedProductClassificationGroups',
    'BusinessGroupInlineWorkspace',
    'handleProductCategoryMoveTarget',
    'businessGroupMoveAssignmentPayload',
    'groupRowsByBusinessGroupTemplates',
  ]) {
    assert.ok(source.includes(expected), `missing product business group marker: ${expected}`)
  }
  assert.match(componentSource, /移动到分类/)
  assert.match(workspaceSource, /请选择要移动到的分类/)

  for (const expected of [
    'saveSelectedAliasClassificationAssignment',
    'confirmAliasClassificationTemplateUsage',
    'confirmSelectedAliasClassificationMove',
    '/api/product-classification-assignments/customer-aliases',
    'currentAliasClassificationTemplate',
    'selectedAliasClassificationCategoryID',
    'UNCLASSIFIED_CATEGORY_MOVE_ID',
    'classificationMoveCategoryID',
  ]) {
    assert.ok(source.includes(expected), `missing alias legacy classification marker: ${expected}`)
  }

  const productToolbar = template.match(/<BusinessGroupInlineWorkspace[\s\S]*?>/)?.[0] || ''
  assert.match(productToolbar, /data-pr442-product-group-assignments/)
  assert.match(productToolbar, /@target="handleProductCategoryMoveTarget"/)
  assert.match(productToolbar, /@move="productCategoryMoveActive = true"/)
  assert.doesNotMatch(productToolbar, /分组集 \/ 父组 \/ 子组/)
  assert.doesNotMatch(productToolbar, /placeholder="增加分类"/)
  assert.doesNotMatch(productToolbar, /placeholder="移动到分类"/)
  assert.doesNotMatch(productToolbar, /confirmProductClassificationTemplateUsage/)
  assert.doesNotMatch(productToolbar, /confirmSelectedProductClassificationMove/)

  assert.match(template, /alias-classification-tabs[\s\S]*classification-tabs[\s\S]*alias-classification-selects[\s\S]*增加分类[\s\S]*移动到分类/)
  assert.match(template, /SearchableSelect[\s\S]*placeholder="增加分类"[\s\S]*@select="confirmAliasClassificationTemplateUsage"/)
  assert.match(template, /SearchableSelect[\s\S]*placeholder="移动到分类"[\s\S]*@select="confirmSelectedAliasClassificationMove"/)
  assert.doesNotMatch(template, /move-classification-card/)
  assert.doesNotMatch(template, /add-classification-card/)
  assert.doesNotMatch(template, /classification-action-card/)
  assert.match(template, /product-filter-row[\s\S]*openProductDrawer[\s\S]*deactivateProducts/)
  assert.match(template, /alias-filter-row[\s\S]*openCustomerAliasCreateDrawer[\s\S]*batchDisableCustomerProductAliases/)
  assert.match(template, /#group="\{ group \}"[\s\S]*v-for="row in group\.rows"/)
  assert.match(template, /v-for="group in visibleCustomerAliasGroups"/)
  assert.match(workspaceSource, /business-group-inline-heading/)
})

test('classification group rows support collapse and indentation in product and alias lists', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'collapsedProductClassificationGroups',
    'BusinessGroupInlineWorkspace',
    'toggleAliasClassificationGroup',
    'isAliasClassificationGroupCollapsed',
    'classification-item-row',
  ]) {
    assert.ok(source.includes(expected), `missing classification group marker: ${expected}`)
  }

  assert.match(template, /<BusinessGroupInlineWorkspace[\s\S]*v-model:collapsed-keys="collapsedProductClassificationGroups"/)
  assert.match(template, /#group="\{ group \}"[\s\S]*class="sku-table"/)
  assert.match(workspaceSource, /--business-group-inline-depth/)
  assert.match(workspaceSource, /businessGroupHiddenByCollapsedAncestor/)
  assert.match(workspaceSource, /@click\.stop="toggleGroup\(group\.key\)"/)
  assert.match(template, /isAliasClassificationGroupCollapsed\(group\.key\)\s*\?\s*'展开'\s*:\s*'收起'/)
  assert.match(style, /\.classification-tab\.active\s*\{/)
})

test('product archive uses business groups while customer alias keeps legacy page-level classification tabs', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(source, /businessGroupAssignments/)
  assert.match(script, /apiGet\('\/api\/product-settings'\)/)
  assert.match(source, /business_groups/)
  assert.match(source, /productCatalogBusinessGroups/)
  assert.match(source, /businessGroupRowsForFeatureSelection/)
  assert.match(source, /businessGroupFeatureSelectionIDs/)
  assert.match(source, /businessGroupFeatureSelectionPayload/)
  assert.match(script, /apiGet\('\/api\/business-group-feature-selections\/product_catalog'\)/)
  assert.match(script, /apiSend\('\/api\/business-group-feature-selections\/product_catalog'/)
  assert.match(source, /businessGroupMoveAssignmentPayload/)
  assert.match(source, /groupRowsByBusinessGroupTemplates/)
  assert.match(source, /apiSend\('\/api\/business-group-assignments'/)
  assert.match(template, /data-pr442-product-group-assignments/)
  assert.match(template, /BusinessGroupInlineWorkspace/)
  assert.match(componentSource, /移动到分类/)
  assert.match(workspaceSource, /前往分组模板/)
  assert.doesNotMatch(script, /function productClassificationLabel/)
  assert.doesNotMatch(template.match(/<BusinessGroupInlineWorkspace[\s\S]*?<div class="table-wrap sku-table-wrap product-inline-group-table">/)?.[0] || '', /增加分类/)

  assert.match(source, /aliasClassificationTemplateUsages/)
  assert.match(script, /apiGet\('\/api\/product-classification-template-usages\/customer-aliases'\)/)
  assert.match(script, /saveAliasClassificationTemplateUsage/)
  assert.match(template, /aliasClassificationTabs/)
  assert.doesNotMatch(template, /复制为客户分类/)
})

test('SKU table groups rows by every referenced business group template without product type columns', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'sku-table-wrap',
    'class="sku-table"',
    'BusinessGroupInlineWorkspace',
    'collapsedProductClassificationGroups',
    'product-inline-group-table',
    'sku-name-cell',
    'action-cell',
  ]) {
    assert.ok(source.includes(expected), `missing SKU table layout marker: ${expected}`)
  }
  assert.doesNotMatch(template, /<th class="sku-col-product-type">产品类型<\/th>/)
  assert.doesNotMatch(template, /<th class="sku-col-product-subtype">产品子类型<\/th>/)
  assert.match(template, /#group="\{ group \}"[\s\S]*<table[^>]*class="sku-table"[\s\S]*<thead>/)
  assert.match(template, /v-for="row in group\.rows"/)
  assert.match(template, /handleProductGroupPaginationChange\(group\.key, \$event\)/)
  assert.doesNotMatch(template, /class="classification-template-label"/)
  assert.match(template, /v-if="!productCatalogBusinessGroups\.length"/)
  assert.match(template, /商品档案尚未选择分组模板，当前按全部商品平铺展示/)
  assert.match(workspaceSource, /设置分组模板/)
  assert.match(template, /productGroupTemplateDrawerOpen/)
  assert.match(template, /toggleProductGroupTemplate/)
  assert.match(template, /saveAndCloseProductGroupTemplateDrawer/)
  assert.match(template, /:groups="displaySkuGroups"/)
  assert.match(template, /@target="handleProductCategoryMoveTarget"/)
  assert.match(workspaceSource, /business-group-inline-toggle/)
  assert.match(style, /\.sku-table-wrap\s*\{[^}]*overflow-x:\s*auto;/s)
  assert.match(style, /\.sku-table\s*\{[^}]*width:\s*max-content;[^}]*min-width:\s*1600px;/s)
  assert.match(style, /\.sku-table th,\s*\.sku-table td\s*\{[^}]*white-space:\s*nowrap;/s)
})

test('legacy product type category drag UI is not present in the new product archive template', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const productArchiveWorkspace = template.match(/<div v-show="currentSettingsSection === 'master'"[\s\S]*?<div v-show="currentSettingsSection === 'templates'"/)?.[0] || template

  assert.doesNotMatch(productArchiveWorkspace, /产品类型操作/)
  assert.doesNotMatch(productArchiveWorkspace, /产品子类型操作/)
  assert.doesNotMatch(productArchiveWorkspace, /拖入产品子类型后才参与产品价格表生成/)
  assert.doesNotMatch(productArchiveWorkspace, /v-for="\(secondary, index\) in primary\.children"/)
})

test('SKU settings no longer binds product config templates on the product record', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const productConfigDrawer = template.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''

  assert.doesNotMatch(productConfigDrawer, /productProductionConfigForm\.product_config_template_id/)
  assert.doesNotMatch(productConfigDrawer, /商品配置模板/)
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

test('global unit dictionary is managed from business settings instead of SKU settings', () => {
  const productSettings = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const productTemplate = productSettings.split('<script setup>')[0] || productSettings
  const globalSettings = fs.readFileSync(new URL('../views/GlobalUnitDefinitionsView.vue', import.meta.url), 'utf8')
  const menuSource = fs.readFileSync(new URL('../lib/menu-ia.js', import.meta.url), 'utf8')

  for (const expected of [
    '全局单位字典',
    'productUnitDefinitions',
    'saveGlobalUnitDefinition',
    '/api/product-settings/units',
    'unit-definition-form',
  ]) {
    assert.ok(globalSettings.includes(expected), `missing global unit dictionary marker: ${expected}`)
  }

  assert.match(menuSource, /key:\s*'businessSettings'[\s\S]*label:\s*'业务设置'/)
  assert.doesNotMatch(globalSettings, />新建单位</)
  assert.doesNotMatch(productTemplate, /<strong>单位字典<\/strong>/)
  assert.doesNotMatch(productTemplate, /@submit\.prevent="saveProductUnitDefinition"/)
  assert.match(productTemplate, /这里维护库存单位和销售规格换算/)
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
  const settingsSource = fs.readFileSync(new URL('../views/GlobalUnitDefinitionsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const expected of [
    'unit-template-pane',
    'showUnitTemplatePane',
    'productUnitDefinitions',
    'productUnitTemplates',
    'saveProductUnitTemplate',
    '这里维护库存单位和销售规格换算',
    '/api/product-settings/unit-templates',
  ]) {
    assert.ok(source.includes(expected), `missing global unit template marker: ${expected}`)
  }
  assert.match(settingsSource, /全局单位字典/)
  assert.match(settingsSource, /saveGlobalUnitDefinition/)
  assert.match(settingsSource, /\/api\/product-settings\/units/)
  assert.doesNotMatch(template, /<strong>单位字典<\/strong>/)
  assert.doesNotMatch(source, /saveProductUnitDefinition/)

  assert.match(menuSource, /key: 'businessSettings', label: '业务设置'/)
  assert.match(menuSource, /groupManagement:\s*'分组模板'/)
  assert.match(menuSource, /key: 'productPriceManagement', label: '商品价格管理'/)
  assert.doesNotMatch(menuSource, /key: 'productCategoryManagement', label: '商品分类管理'/)
  assert.doesNotMatch(menuSource, /label: '商品配置和分类模板'/)
  assert.doesNotMatch(menuSource, /label: '阶梯价模板'/)
  assert.doesNotMatch(menuSource, /label: '单位模板'/)
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
  const unitTemplatePane = source.match(/<div v-show="showUnitTemplatePane"[\s\S]*?<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'"/)?.[0] || ''

  assert.ok(unitTemplatePane, 'unit template pane should exist')
  assert.doesNotMatch(unitTemplatePane, />新建模板</)
  assert.match(unitTemplatePane, /@click="resetProductUnitTemplateForm"[\s\S]*新增销售规格模板/)
  assert.match(source, /function resetProductUnitTemplateForm\(\)/)
  assert.match(source, /await apiSend\(url, \{ method, body: payload \}\)/)
  assert.match(source, /await loadAll\(\)\s+resetProductUnitTemplateForm\(\)/)
})

test('existing SKU sales spec template locks inventory unit after create', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const unitTemplatePane = source.match(/<div v-show="showUnitTemplatePane"[\s\S]*?<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'"/)?.[0] || ''

  assert.match(unitTemplatePane, /:disabled="productUnitTemplateInventoryUnitLocked"/)
  assert.match(unitTemplatePane, /库存单位保存后不可修改/)
  assert.match(source, /const productUnitTemplateInventoryUnitLocked = computed/)
  assert.match(source, /original_inventory_unit/)
  assert.match(source, /payload\.inventory_unit\s*=\s*productUnitTemplateForm\.value\.original_inventory_unit/)
})

test('SKU settings compacts context area and uses create edit labels for unit dictionaries', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const settingsSource = fs.readFileSync(new URL('../views/GlobalUnitDefinitionsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''
  const unitTemplatePane = source.match(/<div v-show="showUnitTemplatePane"[\s\S]*?<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'"/)?.[0] || ''
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

  assert.match(unitTemplatePane, /@click="resetProductUnitTemplateForm"[\s\S]*新增销售规格模板/)
  assert.match(unitTemplatePane, /productUnitTemplateForm\.id\s*\?\s*'保存'\s*:\s*'新增'/)
  assert.match(unitTemplatePane, />销售规格模板名称</)
  assert.match(unitTemplatePane, />库存单位</)
  assert.match(unitTemplatePane, /productUnitTemplateForm\.inventory_unit/)
  assert.match(unitTemplatePane, />销售规格明细</)
  assert.match(unitTemplatePane, /sales_spec_rows/)
  assert.match(unitTemplatePane, /class="sales-spec-row"/)
  assert.match(unitTemplatePane, />1<\/span>[\s\S]*row\.spec_name[\s\S]*>=[\s\S]*productUnitTemplateForm\.inventory_unit/)
  assert.match(unitTemplatePane, />默认规格</)
  assert.match(unitTemplatePane, /setSalesSpecDefault\(productUnitTemplateForm, rowIndex\)/)
  assert.match(unitTemplatePane, /row\.default/)
  assert.doesNotMatch(unitTemplatePane, />启用</)
  assert.doesNotMatch(unitTemplatePane, /v-model="row\.sales_unit"/)
  assert.doesNotMatch(unitTemplatePane, /v-model="row\.net_content_unit"/)
  assert.doesNotMatch(unitTemplatePane, />销售单位换算</)
  assert.doesNotMatch(unitTemplatePane, /productUnitTemplateSalesUnitOptions/)
  assert.doesNotMatch(unitTemplatePane, />报价单位</)
  assert.doesNotMatch(unitTemplatePane, />录单单位</)
  assert.doesNotMatch(unitTemplatePane, /成品库存单位/)

  assert.match(script, /const globalUnitEditingCode = ref\(''\)/)
  assert.match(globalUnitDrawer, /@click="resetGlobalUnitDefinitionForm"[\s\S]*新增基础单位/)
  assert.match(globalUnitDrawer, /globalUnitEditingCode\s*\?\s*'保存'\s*:\s*'新增'/)

  assert.match(settingsSource, /const unitEditingCode = ref\(''\)/)
  assert.match(settingsSource, /@click="resetGlobalUnitDefinitionForm"[\s\S]*新增基础单位/)
  assert.match(settingsSource, /unitEditingCode\s*\?\s*'保存'\s*:\s*'新增'/)
})

test('SKU unit template workspace uses left list right editor and opens global unit dictionary drawer', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const unitTemplatePane = source.match(/<div v-show="showUnitTemplatePane"[\s\S]*?<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'"/)?.[0] || ''
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
  assert.match(style, /\.unit-template-layout\s*\{[^}]*grid-template-columns:\s*minmax\(280px,\s*340px\)\s+minmax\(520px,\s*1fr\);/s)
  assert.match(style, /\.unit-template-layout\s*\{[^}]*align-items:\s*stretch;/s)
  assert.match(style, /\.compact-template-list\s*\{[^}]*max-height:\s*none;/s)
  assert.match(style, /\.sales-spec-row\s*\{[^}]*grid-template-columns:\s*auto minmax\(180px,\s*1fr\) auto minmax\(110px,\s*140px\) auto auto auto;/s)
  assert.doesNotMatch(style, /\.sales-spec-row\s*\{[^}]*grid-template-columns:[^}]*minmax\(82px,\s*\.7fr\)/s)
})

test('SKU unit templates and global unit dictionary expose delete actions', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const settingsSource = fs.readFileSync(new URL('../views/GlobalUnitDefinitionsView.vue', import.meta.url), 'utf8')
  const unitTemplatePane = source.match(/<div v-show="showUnitTemplatePane"[\s\S]*?<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'"/)?.[0] || ''
  const globalUnitDrawer = source.match(/<div v-if="globalUnitDrawerOpen"[\s\S]*?<\/aside>\s*<\/div>/)?.[0] || ''

  assert.match(unitTemplatePane, /deleteProductUnitTemplate/)
  assert.match(source, /\/api\/product-settings\/unit-templates\/\$\{templateID\}/)
  assert.match(unitTemplatePane, />删除<\/button>/)
  assert.match(source, /async function deleteProductUnitTemplate\(template\)/)
  assert.match(source, /method:\s*'DELETE'/)

  assert.match(globalUnitDrawer, /deleteGlobalUnitDefinitionFromDrawer/)
  assert.match(globalUnitDrawer, /globalUnitEditingCode[\s\S]*删除/)
  assert.match(source, /\/api\/product-settings\/units\/\$\{encodeURIComponent\(editingCode\)\}/)

  assert.match(settingsSource, /deleteGlobalUnitDefinition/)
  assert.match(settingsSource, /unitEditingCode[\s\S]*删除/)
  assert.match(settingsSource, /\/api\/product-settings\/units\/\$\{encodeURIComponent\(editingCode\)\}/)
  assert.match(settingsSource, /method:\s*'DELETE'/)
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
  assert.match(drawer, /industry_field_template_ids/)
  assert.match(drawer, /type="checkbox"/)
  assert.match(drawer, /productProductionConfigIndustryTemplateOptions/)
  assert.match(drawer, /industryFieldTemplateOptionLabel\(template\)/)
  assert.match(source, /已停用，可取消/)
  assert.match(source, /不可用，可取消/)
  assert.match(source, /优先级/)
  assert.match(drawer, /勾选顺序决定同名字段优先级；取消后重新勾选可调整顺序/)
  assert.doesNotMatch(drawer, /行业字段值/)
  assert.doesNotMatch(drawer, /新增字段/)
  assert.doesNotMatch(drawer, /删除<\/button>/)
  assert.doesNotMatch(drawer, />字段名</)
  assert.doesNotMatch(drawer, />类型</)
})

test('product archive displays and saves the ordered union of selected industry templates', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const sourceBetween = (start, end) => {
    const startIndex = script.indexOf(start)
    const endIndex = script.indexOf(end, startIndex + start.length)
    assert.ok(startIndex >= 0, `missing source block start: ${start}`)
    assert.ok(endIndex > startIndex, `missing source block end: ${end}`)
    return script.slice(startIndex, endIndex)
  }
  const listBlock = sourceBetween('function productionConfigPriceListFields(', 'const customerSkuRowsRaw')
  const openBlock = sourceBetween('async function openProductProductionConfig(', 'async function loadIndustryFieldTemplates(')
  const applyBlock = sourceBetween('function applyIndustryFieldTemplateToProductionConfig(', 'function closeProductProductionConfigDrawer(')
  const closeBlock = sourceBetween('function closeProductProductionConfigDrawer(', 'async function refreshClassificationTemplates(')
  const saveBlock = sourceBetween('async function saveProductProductionConfig(', 'async function createSku(')

  assert.match(listBlock, /productProductionConfigFieldsFromTemplates\(config\.fields \|\| \[\], industryFieldTemplatesForConfig\(config\)\)/)
  assert.match(applyBlock, /const industryFieldTemplateIDs = industryFieldTemplateIDsFromConfig\(productProductionConfigForm\.value\)/)
  assert.match(applyBlock, /productProductionConfigForm\.value\.industry_field_template_ids = industryFieldTemplateIDs/)
  assert.match(applyBlock, /productProductionConfigForm\.value\.fields = productProductionConfigFieldsFromTemplates/)
  assert.match(saveBlock, /const industryFieldTemplateIDs = industryFieldTemplateIDsFromConfig\(productProductionConfigForm\.value\)/)
  assert.match(saveBlock, /const fields = productProductionConfigFieldsFromTemplates/)
  assert.match(saveBlock, /industry_field_template_ids: industryFieldTemplateIDs/)

  assert.match(script, /^let productProductionConfigOpenGeneration = 0$/m)
  assert.match(openBlock, /const openGeneration = \+\+productProductionConfigOpenGeneration/)
  assert.match(openBlock, /const productID = Number\(row\?\.id \|\| config\?\.product_id \|\| 0\)/)
  assert.match(openBlock, /const industryFieldTemplateIDs = industryFieldTemplateIDsFromConfig\(config\)/)
  assert.match(openBlock, /const industryFieldTemplateSignature = industryFieldTemplateIDs\.join\(','\)/)
  assert.match(openBlock, /const industryFieldTemplatesAvailableAtOpen = industryFieldTemplatesForConfig\(config\)\.length === industryFieldTemplateIDs\.length/)
  assert.match(openBlock, /let industryFieldTemplatesPromise = loadIndustryFieldTemplates\(\)/)
  assert.match(openBlock, /if \(!industryFieldTemplatesAvailableAtOpen && industryFieldTemplateIDs\.length\) \{\s*industryFieldTemplatesPromise = industryFieldTemplatesPromise\.then/)
  assert.match(openBlock, /industryFieldTemplatesPromise = industryFieldTemplatesPromise\.then\(\(\) => \{[\s\S]*?isCurrentProductProductionConfigIndustryProjection\(openGeneration, productID, industryFieldTemplateSignature\)[\s\S]*?productProductionConfigForm\.value\.fields = productProductionConfigFieldsFromTemplates/)
  assert.match(openBlock, /let industryFieldTemplatesPromise = loadIndustryFieldTemplates\(\)[\s\S]*?industryFieldTemplatesPromise = industryFieldTemplatesPromise\.then[\s\S]*?await Promise\.all\(/)
  assert.match(openBlock, /Promise\.all\(\[[\s\S]*?industryFieldTemplatesPromise,[\s\S]*?\]\)/)
  assert.doesNotMatch(openBlock, /await Promise\.all\([\s\S]*?\]\)\s*productProductionConfigForm\.value\.fields\s*=/)
  assert.match(openBlock, /await Promise\.all\([\s\S]*?\]\)\s*if \(!isCurrentProductProductionConfigOpen\(openGeneration, productID\)\) return\s*await ensureProductBomUsage\(productID\)\s*if \(!isCurrentProductProductionConfigOpen\(openGeneration, productID\)\) return/)
  assert.match(openBlock, /await ensureProductionBomDetail\(productProductionConfigForm\.value\.production_bom_id\)\s*if \(!isCurrentProductProductionConfigOpen\(openGeneration, productID\)\) return/)
  assert.match(openBlock, /catch \(err\) \{\s*if \(!isCurrentProductProductionConfigOpen\(openGeneration, productID\)\) return\s*error\.value =/)
  assert.equal((openBlock.match(/isCurrentProductProductionConfigIndustryProjection\(openGeneration, productID, industryFieldTemplateSignature\)/g) || []).length, 1)
  assert.equal((openBlock.match(/isCurrentProductProductionConfigOpen\(openGeneration, productID\)/g) || []).length, 4)
  assert.doesNotMatch(openBlock, /isCurrentProductProductionConfigOpen\(openGeneration, productID, industryFieldTemplateSignature\)/)
  assert.doesNotMatch(openBlock, /\t/)

  const drawerGuardBlock = sourceBetween('function isCurrentProductProductionConfigOpen(', 'function isCurrentProductProductionConfigIndustryProjection(')
  const projectionGuardBlock = sourceBetween('function isCurrentProductProductionConfigIndustryProjection(', 'async function openProductProductionConfig(')
  assert.match(drawerGuardBlock, /generation === productProductionConfigOpenGeneration/)
  assert.match(drawerGuardBlock, /productProductionConfigDrawerOpen\.value/)
  assert.match(drawerGuardBlock, /currentProductID === Number\(productID \|\| 0\)/)
  assert.doesNotMatch(drawerGuardBlock, /industryFieldTemplateIDs|industry_field_template_ids/)
  assert.match(projectionGuardBlock, /isCurrentProductProductionConfigOpen\(generation, productID\)/)
  assert.match(projectionGuardBlock, /industryFieldTemplateIDsFromConfig\(productProductionConfigForm\.value\)\.join\(','\) === String\(industryFieldTemplateSignature \|\| ''\)/)
  assert.match(closeBlock, /productProductionConfigOpenGeneration \+= 1\s*productProductionConfigDrawerOpen\.value = false/)
})

test('product settings uses product business groups instead of product classification page controls', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const productToolbar = template.match(/<BusinessGroupInlineWorkspace[\s\S]*?>/)?.[0] || ''
  const groupManagementWorkspace = template.match(/<div v-show="currentSettingsSection === 'category-management'"[\s\S]*?<div v-show="currentSettingsSection === 'master'"/)?.[0] || ''

  for (const expected of [
    'businessGroupAssignments',
    'businessGroups',
    'productGroupFeatureSelectionIDs',
    'productGroupFeatureSelectionDraft',
    'saveProductGroupFeatureSelection',
    'productCatalogBusinessGroups',
    'collapsedProductClassificationGroups',
    'productCategoryMoveActive',
    'handleProductCategoryMoveTarget',
    'saveSelectedProductBusinessGroupAssignment',
    'BusinessGroupInlineWorkspace',
    'businessGroupInlineListState',
    'groupRowsByBusinessGroupTemplates',
    'businessGroupMoveAssignmentPayload',
    'data-pr442-product-group-assignments',
	  ]) {
    assert.ok(source.includes(expected), `missing product business group marker: ${expected}`)
  }
  assert.match(componentSource, /移动到分类/)
  assert.match(workspaceSource, /请选择要移动到的分类/)
  assert.match(productToolbar, /@target="handleProductCategoryMoveTarget"/)
  assert.match(productToolbar, /@move="productCategoryMoveActive = true"/)
  assert.doesNotMatch(productToolbar, /placeholder="增加分类"/)
  assert.doesNotMatch(productToolbar, /placeholder="移动到分类"/)
  assert.equal(groupManagementWorkspace, '')
  const settingsSource = fs.readFileSync(new URL('../views/GroupTemplatesView.vue', import.meta.url), 'utf8')
  assert.match(settingsSource, /data-section-mode="groupTemplates"/)
  assert.match(settingsSource, /新增大类/)
  assert.match(settingsSource, /新增小类/)
  assert.match(source, /\/api\/business-group-items/)
  assert.match(source, /\/api\/business-group-feature-selections\/product_catalog/)
  assert.doesNotMatch(source, /商品默认分组/)
  assert.doesNotMatch(settingsSource, /\/api\/product-settings\/categories/)
  assert.doesNotMatch(source, /classification-config-drawer/)
  assert.doesNotMatch(source, /aria-label="分类配置"/)
})

test('business group management rebuilds flat parent-child items for subcategory display', () => {
  const tree = businessGroupItemsTree([
    { id: 11, group_id: 6, parent_id: 0, name: '熟豆', sort_order: 20, active: true },
    { id: 12, group_id: 6, parent_id: 11, name: '意式拼配', sort_order: 10, active: true },
    { id: 13, group_id: 6, parent_id: 0, name: '生豆', sort_order: 10, active: true },
    { id: 14, group_id: 6, parent_id: 11, name: '单品豆', sort_order: 20, active: false },
  ])

  assert.deepEqual(tree.map((item) => item.name), ['生豆', '熟豆'])
  assert.deepEqual(tree.find((item) => item.id === 11)?.children.map((item) => item.name), ['意式拼配'])
  assert.equal(tree.find((item) => item.id === 12), undefined, 'child items must not render as top-level big groups')
})

test('customer product aliases use page-level classification templates, not single or batch fields', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const aliasDrawer = source.match(/<aside class="settings-drawer customer-alias-create-drawer"[\s\S]*?<\/aside>/)?.[0] || ''
  const aliasForm = aliasDrawer.match(/<form[^>]*class="[^"]*customer-alias-form[^"]*"[\s\S]*?<\/form>/)?.[0] || ''
  const aliasBatchMode = aliasDrawer.match(/<div v-else class="customer-alias-batch-mode"[\s\S]*?<\/div>\s*<\/div>\s*<div v-if="customerAliasCreateMode === 'batch'"/)?.[0] || ''
  const aliasTable = source.match(/<table class="customer-alias-table"[\s\S]*?<\/table>/)?.[0] || ''

  assert.doesNotMatch(aliasForm, /classification_template_id/)
  assert.doesNotMatch(aliasForm, /include_in_price_list/)
  assert.doesNotMatch(aliasForm, /customerProductAliasForm\.gradient_template_id/)
  assert.doesNotMatch(aliasForm, /customerProductAliasForm\.unit_template_id/)
  assert.doesNotMatch(aliasForm, /customerProductAliasForm\.product_config_template_id/)
  assert.match(aliasTable, /openCustomerProductAliasEditor\(alias\)/)
  assert.match(aliasTable, />价格摘要</)
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

test('product archive config drawer separates producible BOM defaults from component where-used lookup', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const drawer = source.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''

  assert.match(drawer, /可生产该商品的 BOM/)
  assert.match(drawer, /作为组件被哪些 BOM 使用/)
  assert.match(drawer, /productProductionConfigProduceBomRows/)
  assert.match(drawer, /productProductionConfigUsedByBomRows/)
  assert.match(drawer, /setDefaultProductionBom/)
  assert.match(drawer, /is_default/)
  assert.match(drawer, /can_set_default/)
  assert.match(drawer, /bomUsageRelationLabel/)
  assert.match(drawer, /bomUsageStatusLabel/)
  assert.match(drawer, /BOM状态/)
  assert.match(drawer, /BOM版本：\{\{\s*bomUsageVersionLabel\(row\)\s*\}\}/)
  assert.match(drawer, /bomUsageRowKey\(row\)/)
  assert.match(source, /function bomUsageVersionLabel\(row = \{\}\)/)
  assert.match(source, /productBomUsageByProductID/)
  assert.doesNotMatch(drawer, /生产反查/)
  assert.doesNotMatch(drawer, /产出或消耗/)
  assert.match(drawer, /产出该商品/)
  assert.match(source, /产出商品/)
  assert.match(source, /作为组件/)
  assert.match(source, /current_published_version_no/)
  assert.doesNotMatch(drawer, /<span>被哪些 BOM 使用<\/span>/)
  assert.doesNotMatch(drawer, /<select v-model\.number="productProductionConfigForm\.production_bom_id"/)
  assert.doesNotMatch(drawer, /SearchableSelect[\s\S]*productProductionConfigActiveBomOptions/)
  assert.doesNotMatch(drawer, /placeholder="搜索有效生产 BOM"/)
  assert.doesNotMatch(drawer, /维护当前 BOM 明细/)
})

test('product menus expose direct category, price management and renamed product price list', () => {
  const menuSource = fs.readFileSync(new URL('./menu-ia.js', import.meta.url), 'utf8')
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const costingSource = fs.readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
  const configWorkspace = source.match(/<div v-show="currentSettingsSection === 'templates'"[\s\S]*?<div v-if="productDrawerOpen"/)?.[0] || ''

  for (const expected of [
    "key: 'businessSettings'",
    "label: '业务设置'",
    "key: 'productPriceManagement'",
    "label: '商品价格管理'",
    "label: '商品价格表'",
  ]) {
    assert.match(menuSource, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.match(menuSource, /groupManagement:\s*'分组模板'/)
  assert.doesNotMatch(menuSource, /key: 'productCategoryManagement', label: '商品分类管理'/)
  assert.doesNotMatch(menuSource, /label: '商品配置和分类模板'/)
  assert.doesNotMatch(menuSource, /label: '阶梯价模板'/)
  assert.doesNotMatch(menuSource, /label: '单位模板'/)
  assert.doesNotMatch(menuSource, /label: '产品价格表'/)
  assert.match(costingSource, /<h2>商品价格表<\/h2>/)
  assert.doesNotMatch(costingSource, /<h2>产品价格表<\/h2>/)
  assert.match(configWorkspace, /商品配置模板/)
  assert.doesNotMatch(configWorkspace, /分类模板/)
  assert.doesNotMatch(configWorkspace, /classification-template/)
  assert.doesNotMatch(configWorkspace, /activeConfigTemplateSection === 'gradient'/)
  assert.doesNotMatch(configWorkspace, /activeConfigTemplateSection === 'unit-template'/)
})

test('legacy classification template tab is not restored from saved product settings drafts', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(script, /activeConfigTemplateSection\.value = \['product-config', 'product-price-management'\]\.includes\(draft\.activeConfigTemplateSection\) \? draft\.activeConfigTemplateSection : 'product-config'/)
  assert.doesNotMatch(script, /activeConfigTemplateSection\.value = \[[^\]]*'classification-template'/)
})

test('deleted template rows are hidden without treating inactive rows as deleted', () => {
  const rows = [
    { id: 1, name: '启用模板', active: true },
    { id: 2, name: '停用模板', active: false },
    { id: 3, name: '删除模板', active: false, deleted_at: '2026-06-06T10:00:00Z' },
    { id: 4, name: '删除模板状态', active: true, template_state: 'deleted' },
  ]

  assert.deepEqual(visibleNonDeletedRows(rows).map((row) => row.id), [1, 2])
})

test('gradient templates choose display units from global unit dictionary instead of unit templates', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const gradientPane = source.match(/<div v-show="showGradientTemplatePane"[\s\S]*?<div v-show="showUnitTemplatePane"/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(gradientPane, /展示单位/)
  assert.match(gradientPane, /v-for="unit in gradientDisplayUnitOptions"/)
  assert.match(script, /for \(const unit of activeProductUnitDefinitions\.value\)/)
  assert.doesNotMatch(gradientPane, /单位模板/)
  assert.doesNotMatch(gradientPane, /templateForm\.unit_template_id/)
  assert.doesNotMatch(gradientPane, /syncGradientDisplayUnitFromUnitTemplate/)
})

test('product settings UI hides deleted dictionaries and supports product config template delete', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const productConfigPane = source.match(/<div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'"[\s\S]*?<div v-show="showProductPriceManagementPane"/)?.[0] || ''

  assert.match(script, /const visibleProductUnitDefinitions = computed\(\(\) => visibleNonDeletedRows\(productUnitDefinitions\.value\)\)/)
  assert.match(script, /const visibleProductUnitTemplates = computed\(\(\) => visibleNonDeletedRows\(productUnitTemplates\.value\)\)/)
  assert.match(script, /const visibleProductConfigTemplates = computed\(\(\) => visibleNonDeletedRows\(productConfigTemplates\.value\)\)/)
  assert.match(script, /const visibleProductClassificationTemplates = computed\(\(\) => visibleNonDeletedRows\(productClassificationTemplates\.value\)\)/)
  assert.match(productConfigPane, /@click="deleteProductConfigTemplate\(productConfigTemplateForm\.id\)"/)
  assert.match(script, /async function deleteProductConfigTemplate/)
  assert.match(script, /\/api\/product-settings\/product-config-templates\/\$\{templateID\}/)
  assert.match(script, /method: 'DELETE'/)
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

test('product category assignment label prefers direct product category ownership', () => {
  const categoryTree = [{
    id: 3,
    name: '咖啡烘焙豆',
    children: [{ id: 7, name: '精品意式拼配' }],
  }]

  assert.equal(
    productCategoryAssignmentLabel({ product_category_id: 7 }, categoryTree),
    '咖啡烘焙豆 / 精品意式拼配',
  )
  assert.equal(
    productCategoryAssignmentLabel({ primary_name: '咖啡烘焙豆', secondary_name: '工厂量单' }, categoryTree),
    '咖啡烘焙豆 / 工厂量单',
  )
  assert.equal(productCategoryAssignmentLabel({}, categoryTree), '未分类')
  assert.equal(productCategoryAssignmentLabel({}, categoryTree, ''), '')
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
  assert.doesNotMatch(script, /productCategoryAssignmentLabel\(row,\s*categoryTreeForSkuContext\.value,\s*''\)/)
  assert.match(script, /selectedProductRowsAlreadyInCurrentCategory/)
  assert.match(script, /selectedAliasRowsAlreadyInCurrentCategory/)
  assert.doesNotMatch(source, /已归类，需先移出当前分类/)
})

test('classification template warnings explain that product config templates override category and template defaults', () => {
  const warnings = classificationTemplateUnitPriceWarnings({
    productConfigTemplate: { id: 7 },
    classificationTemplate: { id: 10, product_config_template_id: 101 },
    classificationCategory: { id: 11, product_config_template_id: 102 },
  })
  assert.deepEqual(warnings, [
    '商品已选择商品配置模板，将覆盖所属分类引用的商品配置模板',
  ])
  assert.deepEqual(classificationTemplateUnitPriceWarnings({
    productConfigTemplate: { id: 102 },
    classificationTemplate: { id: 10, product_config_template_id: 101 },
    classificationCategory: { id: 11, product_config_template_id: 102 },
  }), [])
  assert.deepEqual(classificationTemplateUnitPriceWarnings({
    productConfigTemplate: { id: 0 },
    classificationTemplate: { id: 10, product_config_template_id: 101 },
    classificationCategory: { id: 11, product_config_template_id: 102 },
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

test('product archive grouping is template-driven without category tabs or category column', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const productArchiveBlock = source.slice(
    source.indexOf('data-section-mode="productMaster"'),
    source.indexOf('aria-label="客户商品分类模板视图"'),
  )

  assert.match(source, /BusinessGroupInlineWorkspace/)
  assert.match(source, /groupRowsByBusinessGroupTemplates/)
  assert.match(workspaceSource, /设置分组模板/)
  assert.match(source, /saveAndCloseProductGroupTemplateDrawer/)
  assert.match(componentSource, /移动到分类/)
  assert.match(workspaceSource, /business-group-inline-sections/)
  assert.doesNotMatch(workspaceSource, /business-group-category-tree/)
  assert.doesNotMatch(productArchiveBlock, /<div v-if="selectedProductGroupTemplate" class="classification-tabs">/)
  assert.doesNotMatch(productArchiveBlock, /<button[^>]*class="classification-tab"/)
  assert.doesNotMatch(productArchiveBlock, /<th>分类<\/th>/)
  assert.doesNotMatch(productArchiveBlock, /productClassificationLabel\(row\)/)
})
