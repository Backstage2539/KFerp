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
  buildPricingRuleTrialPayload,
  applyPricingRuleTrialToPriceTableRow,
  priceTablePricingRuleTrialPayload,
  buildProductProductionConfigForm,
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
  skuListRowsFromProducts,
  skuTableState,
  skuTypeLabel,
  skuTypeOptions,
  unitConversionJSONFromRows,
  unitConversionRowsFromJSON,
  unitRuleFormFromJSON,
  unitRuleJSONFromForm,
  visibleNonDeletedRows,
  salesSpecConversionLabel,
  salesSpecRowsFromTemplate,
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
  assert.match(menuSource, /key: 'groupTemplates', label: '分组模板'/)
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
      yield_loss_mode: 'bom_or_product',
      profit_method: 'gross_margin',
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
      { label: '1kg+', min_qty: 1, max_qty: 9, quantity_unit: 'kg', pricing_rule_id: 10, position: 1, active: true, remark: '' },
      { label: '10kg+', min_qty: 10, max_qty: null, quantity_unit: 'kg', pricing_rule_id: 20, position: 2, active: true, remark: '' },
    ],
  })
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
      yield_loss_mode: 'manual',
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
    operation_template_id: 27,
    quote_unit: 'kg',
    overrides: {
      expected_loss_rate: 0.12,
      margin_rate: 0.3,
      tax_rate: 0.06,
      other_costs: {
        '包装贴标': 1.25,
        '认证费': 2.5,
      },
    },
  })
})

test('price table resolves pricing mode by product, subgroup, parent group, price list', () => {
  const resolved = resolvePriceTableTemplateInheritance({
    defaults: { pricing_mode: 'fixed_price', tier_template_id: 1, pricing_rule_id: 10, fixed_unit_price: 99 },
    groupAssignments: [
      { group_item_id: 100, pricing_mode: 'pricing_rule', tier_template_id: 2, pricing_rule_id: 20, fixed_unit_price: 0, parent_group_item_id: 0 },
      { group_item_id: 101, pricing_mode: 'tier_template', tier_template_id: 3, pricing_rule_id: 0, fixed_unit_price: 0, parent_group_item_id: 100 },
    ],
    productOverrides: [
      { product_id: 88, group_item_id: 101, tier_template_id: 0, pricing_rule_id: 40 },
    ],
    product: { id: 88, group_item_id: 101 },
  })

  assert.deepEqual(resolved, {
    pricing_mode: 'tier_template',
    pricing_mode_source: 'subgroup',
    tier_template_id: 3,
    tier_template_source: 'subgroup',
    pricing_rule_id: 40,
    pricing_rule_source: 'product',
    fixed_unit_price: 99,
    fixed_unit_price_source: 'default',
  })

  assert.deepEqual(buildPriceTableRowsFromTemplateResolution({
    product: { id: 88, name: '初晓拼配', inventory_unit: 'kg', default_sales_unit: '盒', unit_conversion_json: '{"盒":{"kg":0.2}}' },
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
    { product_id: 88, product_name: '初晓拼配', price_unit: '盒', inventory_unit: 'kg', inventory_conversion_json: { 盒: { kg: 0.2 } }, tier_label: '1kg+', min_qty: 1, max_qty: 9, final_unit_price: 88, pricing_mode: 'tier_template', pricing_mode_source: 'subgroup', tier_template_id: 3, tier_template_source: 'subgroup', template_tier_id: 31, pricing_rule_id: 41, pricing_rule_source: 'subgroup', pricing_rule_version: 'PR-1KG', tier_pricing_rule_id: 41, tier_pricing_rule_version: 'PR-1KG' },
    { product_id: 88, product_name: '初晓拼配', price_unit: '盒', inventory_unit: 'kg', inventory_conversion_json: { 盒: { kg: 0.2 } }, tier_label: '10kg+', min_qty: 10, max_qty: null, final_unit_price: 78, pricing_mode: 'tier_template', pricing_mode_source: 'subgroup', tier_template_id: 3, tier_template_source: 'subgroup', template_tier_id: 32, pricing_rule_id: 42, pricing_rule_source: 'subgroup', pricing_rule_version: 'PR-10KG', tier_pricing_rule_id: 42, tier_pricing_rule_version: 'PR-10KG' },
  ])
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
      operation_template_id: 9,
    },
  }

  assert.deepEqual(priceTablePricingRuleTrialPayload(row, { customerID: 0 }), {
    pricing_rule_id: 40,
    product_id: 550,
    customer_id: 0,
    bom_version_id: 8842,
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
    operation_template_id: 9,
    operation_template_name: '标准烘焙',
    base_cost: 42.3,
  })

  assert.equal(got.product_name, '熟豆-红岩拼配')
  assert.equal(got.final_unit_price, 68.5)
  assert.equal(got.original_final_unit_price, 68.5)
  assert.equal(got.price_unit, 'lb')
  assert.deepEqual(got.inventory_conversion_json, { lb: { kg: 0.454 } })
  assert.equal(got.cost_source_snapshot.bom_version_no, 'V002')
  assert.equal(got.cost_source_snapshot.operation_template_name, '标准烘焙')
  assert.equal(got.cost_source_snapshot.pricing_rule_trial_final_unit_price, 68.5)
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
    },
  }

  assert.deepEqual(priceTablePricingRuleTrialPayload(row, { customerID: 0 }), {
    pricing_rule_id: 11,
    product_id: 550,
    customer_id: 0,
    bom_version_id: 723,
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
      operation_template_id: 9,
    },
  }

  assert.deepEqual(priceTablePricingRuleTrialPayload(row, { customerID: 0 }), {
    pricing_rule_id: 40,
    product_id: 550,
    customer_id: 0,
    bom_version_id: 8842,
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

test('product settings exposes pricing rule pane instead of final price records', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const pane = source.match(/<div v-show="showProductPriceManagementPane"[\s\S]*?<p class="muted price-list-flat-row-note"/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const want of ['product-price-management-pane', '商品价格管理', '价格计算模板', 'Pricing Rule', '价格试算', '新建价格计算模板', '基础成本', '生产 BOM 成本（物料+工序）', '其他成本', '成本名', '成本价格', '全局币种配置', '利润方式', '税费方式', '最低毛利', '公式版本', '试算说明', '利润率', '税率', '取整规则', '复制', '失效']) {
    assert.equal(pane.includes(want), true, `product price management pane should expose ${want}`)
  }
  for (const want of ['pricingRules', 'buildPricingRulePayload', 'buildPricingRuleCopyPayload', 'startPricingRuleEdit', 'copyPricingRule', 'deactivatePricingRule', 'addPricingRuleOtherCostRow']) {
    assert.match(script, new RegExp(want))
  }
  assert.match(pane, /@click="openPricingRuleTrial\(\)"[^>]*>价格试算<\/button>[\s\S]*@click="resetPricingRuleForm"[^>]*>新建价格计算模板<\/button>/)
  assert.match(pane, /class="text-button pricing-rule-name-button"[\s\S]*@click="startPricingRuleEdit\(rule\)"/)
  assert.match(pane, /class="secondary compact-action pricing-rule-copy-action"[\s\S]*@click="copyPricingRule\(rule\)"[\s\S]*>复制<\/button>/)
  assert.match(pane, /:class="\['pricing-rule-row', \{ inactive: rule\.active === false \}\]"/)
  assert.doesNotMatch(pane, />编辑模板<\/button>/)
  assert.doesNotMatch(pane, /@click="openPricingRuleTrial\(rule\)"/)
  for (const forbidden of ['商品成本上下文', '成本项配置', '库存成本', '手工成本', '最近采购成本', '成本取数口径', '商品价格记录', '最终单价', '引用价格记录', 'source_price_record_id', '阶梯价模板', 'priceTierTemplateForm', 'savePriceTierTemplate', 'min_qty', 'max_qty', 'tier_label']) {
    assert.equal(pane.includes(forbidden), false, `product price management pane should not expose ${forbidden}`)
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
    '工序',
    '销售单位',
    '临时损耗率',
    '临时利润/加价',
    '临时税率',
    '其他成本',
    '加价后价格',
    '试算单价',
    'BOM+工序成本明细',
    '物料成本明细',
    '工序成本明细',
    '损耗增加',
    '加价增加',
    '税额',
    '取整调整',
    'base_cost_details',
    'tax_in_price_amount',
    'pricing-rule-trial-waterfall',
    'pricing-rule-trial-operator',
    '计算公式',
    'formula_expression_lines',
    '公式步骤',
    'pricingRuleTrialQuoteUnitOptions',
    'pricingRuleTrialBomVersionOptions',
    'pricingRuleTrialOperationTemplateOptions',
    'schedulePricingRuleTrial',
  ]) {
    assert.ok(source.includes(want), `missing pricing rule trial marker: ${want}`)
  }
  for (const forbidden of [
    '售价后附加成本',
    '重新试算',
    'post_markup_cost_rows',
    'addPricingRuleTrialPostMarkupCostRow',
    'removePricingRuleTrialPostMarkupCostRow',
    '来源：',
    '状态：',
    'product_production_config',
    'missing',
    '发布售价快照反推',
  ]) {
    assert.equal(trialDrawer.includes(forbidden), false, `pricing rule trial drawer should not expose ${forbidden}`)
  }
  assert.match(pane, /@click="openPricingRuleTrial\(\)"[^>]*>价格试算<\/button>/)
  assert.doesNotMatch(pane, /@click="openPricingRuleTrial\(rule\)"/)
  assert.match(trialDrawer, /<select v-model\.number="pricingRuleTrialForm\.pricing_rule_id"[\s\S]*activePricingRuleTrialOptions/)
  assert.match(source, /<select v-model\.number="pricingRuleTrialForm\.bom_version_id"[\s\S]*pricingRuleTrialBomVersionOptions/)
  assert.match(source, /<select v-model\.number="pricingRuleTrialForm\.operation_template_id"[\s\S]*pricingRuleTrialOperationTemplateOptions/)
  assert.match(source, /<select v-model="pricingRuleTrialForm\.quote_unit"[\s\S]*pricingRuleTrialQuoteUnitOptions/)
  assert.doesNotMatch(pane, /@click="runPricingRuleTrial"/)
  assert.match(script, /apiSend\('\/api\/costing\/pricing-rule-trial'/)
  assert.match(script, /watch\(\(\) => pricingRuleTrialAutoRunSignature\.value/)
  assert.match(style, /\.pricing-rule-trial-drawer/)
  assert.match(source, /pricing-rule-trial-operator[\s\S]*\+/)
  assert.match(source, /pricing-rule-trial-operator[\s\S]*=/)
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
    '商品 &gt; 子类 &gt; 父类 &gt; 价格表',
    'pricing_rule_id',
  ]) {
    assert.ok(source.includes(want), `CostingView missing marker: ${want}`)
  }
  assert.doesNotMatch(source, /默认阶梯价模板/)
  assert.doesNotMatch(source, /子组/)
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
      { spec_key: 'bag-100g', spec_name: '100g袋装', sales_unit: '袋', net_content_qty: 100, net_content_unit: 'g', active: true },
    ],
  })
  assert.deepEqual(payload, {
    id: 31,
    name: '咖啡袋装销售规格',
    inventory_unit: 'kg',
    default_sales_unit: '袋',
    sales_unit: '袋',
    sales_units: ['袋'],
    quote_unit: '袋',
    order_unit: '袋',
    unit_conversion_json: '{}',
    sales_specs: [
      { spec_key: 'bag-227g', spec_name: '227g袋装', sales_unit: '袋', net_content_qty: 227, net_content_unit: 'g', default: true, active: true },
      { spec_key: 'bag-100g', spec_name: '100g袋装', sales_unit: '袋', net_content_qty: 100, net_content_unit: 'g', default: false, active: true },
    ],
    active: true,
  })
})

test('sales spec rows decorate template specs and preserve derived child SKU status', () => {
  assert.deepEqual(salesSpecRowsFromTemplate({
    sales_specs: [
      { spec_key: 'bag-227g', spec_name: '227g袋装', sales_unit: '袋', net_content_qty: 227, net_content_unit: 'g', default: true, active: true, derived_sku_code: 'SKU-000912' },
      { spec_key: 'bag-100g', spec_name: '100g袋装', sales_unit: '袋', net_content_qty: 100, net_content_unit: 'g', active: false, derived_spec_status: 'template_disabled' },
    ],
  }), [
    { spec_key: 'bag-227g', spec_name: '227g袋装', sales_unit: '袋', net_content_qty: 227, net_content_unit: 'g', default: true, active: true, derived_sku_code: 'SKU-000912', derived_spec_status: 'active' },
    { spec_key: 'bag-100g', spec_name: '100g袋装', sales_unit: '袋', net_content_qty: 100, net_content_unit: 'g', default: false, active: false, derived_sku_code: '', derived_spec_status: 'template_disabled' },
  ])
})

test('sales spec conversion label explains sales unit to parent inventory unit', () => {
  assert.equal(salesSpecConversionLabel({
    spec_name: '227g袋装',
    sales_unit: '袋',
    net_content_qty: 227,
    net_content_unit: 'g',
  }, 'g'), '1 袋 = 227 g')

  assert.equal(salesSpecConversionLabel({
    spec_name: '227g袋装',
    sales_unit: '袋',
    net_content_qty: 227,
    net_content_unit: 'g',
  }, 'kg'), '1 袋 = 0.227 kg')

  assert.equal(salesSpecConversionLabel({
    spec_name: '箱装',
    sales_unit: '箱',
    net_content_qty: 12,
    net_content_unit: '袋',
  }, 'kg'), '1 箱 = 12 袋（库存单位 kg，无法自动换算）')

  assert.equal(salesSpecConversionLabel({
    spec_name: '默认规格',
    sales_unit: '袋',
  }, 'g'), '换算待补：请填写净含量')
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

test('legacy explicit product unit override payload carries inventory unit master data', () => {
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
    yield_rate: 0.8,
    inventory_unit: '盒',
    integer_inventory_unit: true,
    default_sales_unit: '箱',
    unit_conversion_json: { 箱: { 盒: 12 } },
    sales_unit_rules: { 箱: { integer_unit: true } },
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
    yield_rate: 0.8,
    inventory_unit: '个',
    integer_inventory_unit: false,
    default_sales_unit: '盒',
    unit_conversion_json: { 盒: { 个: 10 } },
    sales_unit_rules: { 盒: { integer_unit: true } },
    unit_rule_override_json: '{"order_unit":"箱","legacy_key":"keep"}',
  })
})

test('product create and basics payload inherit inventory unit from sales spec template', () => {
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
    unit_template_id: 7,
    yield_rate: 0.8,
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
    unit_template_id: 7,
    inventory_unit: '盒',
    integer_inventory_unit: true,
    default_sales_unit: '箱',
    unit_conversion_json: { 箱: { 盒: 12 } },
    sales_unit_rules: { 箱: { integer_unit: true } },
    yield_rate: 0.8,
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

test('product drawers require sales spec templates and read inventory unit from template', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const createForm = source.match(/<form class="sku-create-form product-create-form product-drawer-form"[\s\S]*?<\/form>/)?.[0] || ''
  const configDrawer = source.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''
  const baseSection = configDrawer.match(/<strong>基础信息<\/strong>[\s\S]*?<\/section>/)?.[0] || ''
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  for (const marker of [
    '销售规格模板',
    'skuForm.unit_template_id',
    '库存单位：来自销售规格模板',
    '销售规格模板明细',
  ]) {
    assert.match(createForm, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  for (const marker of [
    '销售规格模板',
    'productProductionConfigForm.unit_template_id',
    '库存单位：来自销售规格模板',
    '销售规格模板明细',
  ]) {
    assert.match(baseSection, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
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
  assert.match(script, /请选择销售规格模板/)
})

test('product archive config drawer shows derived child SKUs from sales spec template', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const configDrawer = source.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''

  assert.match(configDrawer, /销售规格 \/ SKU/)
  assert.match(configDrawer, /销售规格模板明细/)
  assert.match(configDrawer, /salesSpecConversionLabel\(row, productUnitTemplateInventoryUnit\(productProductionConfigForm\.unit_template_id\)\)/)
  assert.match(configDrawer, /SKU 编号/)
  assert.match(configDrawer, /derivedSkuCodeLabel\(row\)/)
  assert.doesNotMatch(configDrawer, /继承父 SKU/)
  assert.doesNotMatch(configDrawer, />父 SKU</)
  assert.match(configDrawer, /derived_spec_status/)
  assert.doesNotMatch(configDrawer, /childSkuForm\.sku_name/)
  assert.doesNotMatch(configDrawer, /createChildSkuForProduct/)
  assert.doesNotMatch(configDrawer, /class="child-sku-form"/)
})

test('product archive derived SKU rows reuse template net content for conversion labels', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const derivedRowsBlock = source.match(/const productProductionDerivedSkuRows = computed\([\s\S]*?const skuFormSalesSpecRows/)?.[0] || ''

  assert.match(derivedRowsBlock, /productUnitTemplateSalesSpecRows\(productProductionConfigForm\.value\.unit_template_id\)/)
  assert.match(derivedRowsBlock, /net_content_qty:\s*row\.net_content_qty \|\| spec\.net_content_qty/)
  assert.match(derivedRowsBlock, /net_content_unit:\s*row\.net_content_unit \|\| spec\.net_content_unit/)
  assert.match(derivedRowsBlock, /sales_unit:\s*row\.derived_sales_unit \|\| spec\.sales_unit/)
})

test('sales spec template controls are required in product drawers', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const createForm = source.match(/<form class="sku-create-form product-create-form product-drawer-form"[\s\S]*?<\/form>/)?.[0] || ''
  const configDrawer = source.match(/<aside class="settings-drawer product-production-config-drawer"[\s\S]*?<\/aside>/)?.[0] || ''
  const productListToolbar = source.match(/<div class="sku-list-actions"[\s\S]*?<\/div>/)?.[0] || source

  for (const marker of [
    '销售规格模板',
    'skuForm.unit_template_id',
    '库存单位：来自销售规格模板',
  ]) {
    assert.match(createForm, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }

  for (const marker of [
    '销售规格模板',
    'productProductionConfigForm.unit_template_id',
    '库存单位：来自销售规格模板',
  ]) {
    assert.match(configDrawer, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }

  assert.match(productListToolbar, /设置销售规格模板/)
  assert.match(productListToolbar, /维护销售规格模板/)
  assert.match(productListToolbar, /openProductUnitTemplateManagement/)
  assert.match(source, /key: 'productUnitTemplates'/)
  assert.match(source, /label: '返回商品档案'/)
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

test('product create payload carries SKU remark without direct green bean prices or hard-coded green bean attributes', () => {
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
  assert.equal(payload.yield_rate, 0.82)
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
  assert.equal(payload.yield_rate, 0.8)
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
  assert.match(configPageBlock, /销售规格模板/)
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
    '销售规格模板',
    'productConfigTemplateForm.unit_template_id',
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
    '销售规格模板会影响商品价格表规格行',
    '录单默认销售规格和后续派生子 SKU',
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
  const skuTable = template.match(/<table :key="skuTableKey"[\s\S]*?<\/table>/)?.[0] || template

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

  assert.match(createSkuBlock, /const result = await apiSend\('\/api\/product-settings\/skus'/)
  assert.match(createSkuBlock, /await loadAll\(\)[\s\S]*resolveCreatedProductForConfig\(result/)
  assert.match(createSkuBlock, /await openProductProductionConfig\(createdProductForConfig\)/)
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
  assert.equal(form.expected_loss_percent, 0)
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
  assert.match(template, /class="classification-view-toolbar product-business-group-controls"/)
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

test('SKU settings keeps only the product creation drawer while classification templates drive product tabs', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''
  const productArchiveWorkspace = template.match(/<div v-show="currentSettingsSection === 'master'"[\s\S]*?<div v-show="currentSettingsSection === 'templates'"/)?.[0] || template

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
  assert.doesNotMatch(template, /data-section-mode="groupManagement"/)
  assert.doesNotMatch(productArchiveWorkspace, /v-for="primary in visibleCategoryManagementTreeForSkuContext"/)
  assert.doesNotMatch(productArchiveWorkspace, /class="category-panel category-drawer-panel category-management-panel"/)
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
  assert.match(template, /class="classification-view-toolbar product-business-group-controls"/)
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
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'data-pr442-product-group-assignments',
    'saveSelectedProductBusinessGroupAssignment',
    '/api/business-group-assignments',
    "usage_key: 'product_catalog'",
    "object_key: 'product'",
    'productBusinessGroupItemOptions',
    'BusinessGroupControls',
    'businessGroupMoveAssignmentPayload',
    'groupRowsByBusinessGroupTemplate',
  ]) {
    assert.ok(source.includes(expected), `missing product business group marker: ${expected}`)
  }
  assert.match(componentSource, /选择分组模板/)
  assert.match(componentSource, /移动到分类/)

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

  const productToolbar = template.match(/<BusinessGroupControls[\s\S]*?\/>/)?.[0] || ''
  assert.match(productToolbar, /data-pr442-product-group-assignments/)
  assert.match(productToolbar, /@move="saveSelectedProductBusinessGroupAssignment"/)
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
  assert.match(template, /v-for="group in displaySkuGroups"/)
  assert.match(template, /v-for="group in visibleCustomerAliasGroups"/)
  assert.match(style, /\.classification-group-row\s+td\s*\{/)
})

test('classification group rows support collapse and indentation in product and alias lists', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const helperSource = fs.readFileSync(new URL('./business-grouping.js', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'toggleProductClassificationGroup',
    'toggleAliasClassificationGroup',
    'isProductClassificationGroupCollapsed',
    'isAliasClassificationGroupCollapsed',
    'classificationGroupIndentStyle',
    'classification-subgroup-row',
    '--classification-group-indent',
    'classification-item-row',
    'classification-group-toggle',
  ]) {
    assert.ok(source.includes(expected), `missing classification group marker: ${expected}`)
  }

  assert.match(template, /isProductClassificationGroupCollapsed\(group\.key\)\s*\?\s*'展开'\s*:\s*'收起'/)
  assert.match(template, /:class="\['classification-group-row', \{ 'classification-subgroup-row': Number\(group\.depth \|\| 0\) > 0 \}\]"/)
  assert.match(template, /:style="classificationGroupIndentStyle\(group\)"/)
  assert.match(template, /<strong\s+:title="group\.path_label \|\| group\.label">\{\{ group\.label \}\}<\/strong>/)
  assert.match(template, /<tr\s+:class="\[\{ 'inactive-sku': row\.active === false, 'sku-highlight': row\.id === highlightedSkuId \}, 'classification-item-row'\]"\s+:style="classificationItemIndentStyle\(group\)"/)
  assert.match(template, /isAliasClassificationGroupCollapsed\(group\.key\)\s*\?\s*'展开'\s*:\s*'收起'/)
  assert.match(script, /function classificationGroupIndentStyle\(group = \{\}\)/)
  assert.match(script, /function classificationItemIndentStyle\(group = \{\}\)/)
  assert.match(script, /businessGroupHeaderIndentStyle\(group\)/)
  assert.match(script, /businessGroupItemIndentStyle\(group\)/)
  assert.ok(helperSource.includes('toNumber(group.depth) * 24'), 'missing shared classification group depth indent calculation')
  assert.ok(helperSource.includes("'--classification-group-indent'"), 'missing shared classification group indent variable')
  assert.match(style, /\.classification-item-row\s+td:first-child \+ td\s*\{[^}]*padding-left:\s*var\(--classification-item-indent,\s*18px\);/s)
  assert.match(style, /\.classification-group-row\s+td\s*\{[^}]*padding-left:\s*var\(--classification-group-indent,\s*16px\);/s)
  assert.match(style, /\.classification-subgroup-row\s+td\s*\{/)
  assert.match(style, /\.classification-tab\.active\s*\{/)
})

test('product archive uses business groups while customer alias keeps legacy page-level classification tabs', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(source, /businessGroupAssignments/)
  assert.match(script, /apiGet\('\/api\/product-settings'\)/)
  assert.match(source, /business_groups/)
  assert.match(source, /productCatalogBusinessGroups/)
  assert.match(source, /businessGroupRowsForUsage/)
  assert.match(source, /businessGroupMoveAssignmentPayload/)
  assert.match(source, /groupRowsByBusinessGroupTemplate/)
  assert.match(source, /apiSend\('\/api\/business-group-assignments'/)
  assert.match(template, /data-pr442-product-group-assignments/)
  assert.match(template, /BusinessGroupControls/)
  assert.match(componentSource, /选择分组模板/)
  assert.match(componentSource, /移动到分类/)
  assert.doesNotMatch(script, /function productClassificationLabel/)
  assert.doesNotMatch(template.match(/<BusinessGroupControls[\s\S]*?<div class="table-wrap sku-table-wrap">/)?.[0] || '', /增加分类/)

  assert.match(source, /aliasClassificationTemplateUsages/)
  assert.match(script, /apiGet\('\/api\/product-classification-template-usages\/customer-aliases'\)/)
  assert.match(script, /saveAliasClassificationTemplateUsage/)
  assert.match(template, /aliasClassificationTabs/)
  assert.doesNotMatch(template, /复制为客户分类/)
})

test('SKU table groups rows by selected business group template without product type columns', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const style = source.split('<style scoped>')[1] || ''

  for (const expected of [
    'sku-table-wrap',
    'class="sku-table"',
    'classification-view-toolbar',
    'product-business-group-controls',
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

test('global unit dictionary is managed from global settings instead of SKU settings', () => {
  const productSettings = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const productTemplate = productSettings.split('<script setup>')[0] || productSettings
  const globalSettings = fs.readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
  const menuSource = fs.readFileSync(new URL('../lib/menu-ia.js', import.meta.url), 'utf8')

  for (const expected of [
    '系统设置',
    '全局单位字典',
    'productUnitDefinitions',
    'saveGlobalUnitDefinition',
    '/api/product-settings/units',
    'unit-definition-form',
  ]) {
    assert.ok(globalSettings.includes(expected), `missing global unit dictionary marker: ${expected}`)
  }

  assert.match(menuSource, /key:\s*'uiSettings'[\s\S]*label:\s*'系统设置'/)
  assert.doesNotMatch(globalSettings, />新建单位</)
  assert.doesNotMatch(productTemplate, /<strong>单位字典<\/strong>/)
  assert.doesNotMatch(productTemplate, /@submit\.prevent="saveProductUnitDefinition"/)
  assert.match(productTemplate, /这里维护库存单位、销售规格和销售单位/)
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
    '这里维护库存单位、销售规格和销售单位',
    '/api/product-settings/unit-templates',
  ]) {
    assert.ok(source.includes(expected), `missing global unit template marker: ${expected}`)
  }
  assert.match(settingsSource, /全局单位字典/)
  assert.match(settingsSource, /saveGlobalUnitDefinition/)
  assert.match(settingsSource, /\/api\/product-settings\/units/)
  assert.doesNotMatch(template, /<strong>单位字典<\/strong>/)
  assert.doesNotMatch(source, /saveProductUnitDefinition/)

  assert.match(menuSource, /key: 'groupTemplates', label: '分组模板'/)
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

test('SKU settings compacts context area and uses create edit labels for unit dictionaries', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const settingsSource = fs.readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
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
  assert.match(unitTemplatePane, />默认规格</)
  assert.match(unitTemplatePane, />销售规格明细</)
  assert.match(unitTemplatePane, /sales_spec_rows/)
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
  assert.match(style, /\.unit-template-layout\s*\{[^}]*grid-template-columns:\s*minmax\(220px,\s*280px\)\s+minmax\(0,\s*1fr\);/s)
})

test('SKU unit templates and global unit dictionary expose delete actions', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const settingsSource = fs.readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
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
  assert.match(drawer, /industry_field_template_id/)
  assert.doesNotMatch(drawer, /行业字段值/)
  assert.doesNotMatch(drawer, /新增字段/)
  assert.doesNotMatch(drawer, /删除<\/button>/)
  assert.doesNotMatch(drawer, />字段名</)
  assert.doesNotMatch(drawer, />类型</)
})

test('product settings uses product business groups instead of product classification page controls', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const productToolbar = template.match(/<BusinessGroupControls[\s\S]*?\/>/)?.[0] || ''
  const groupManagementWorkspace = template.match(/<div v-show="currentSettingsSection === 'category-management'"[\s\S]*?<div v-show="currentSettingsSection === 'master'"/)?.[0] || ''

  for (const expected of [
    'businessGroupAssignments',
    'businessGroups',
    'productCatalogBusinessGroups',
    'productBusinessGroupItemOptions',
    'selectedProductBusinessGroupItemID',
    'saveSelectedProductBusinessGroupAssignment',
    'BusinessGroupControls',
    'groupRowsByBusinessGroupTemplate',
    'businessGroupMoveAssignmentPayload',
    'data-pr442-product-group-assignments',
	  ]) {
    assert.ok(source.includes(expected), `missing product business group marker: ${expected}`)
  }
  assert.match(componentSource, /选择分组模板/)
  assert.match(componentSource, /移动到分类/)
  assert.match(productToolbar, /@move="saveSelectedProductBusinessGroupAssignment"/)
  assert.doesNotMatch(productToolbar, /placeholder="增加分类"/)
  assert.doesNotMatch(productToolbar, /placeholder="移动到分类"/)
  assert.equal(groupManagementWorkspace, '')
  const settingsSource = fs.readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
  assert.match(settingsSource, /data-section-mode="groupTemplates"/)
  assert.match(settingsSource, /新增大类/)
  assert.match(settingsSource, /新增小类/)
  assert.match(source, /\/api\/business-group-items/)
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
    "key: 'groupTemplates'",
    "label: '分组模板'",
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
  const productArchiveBlock = source.slice(
    source.indexOf('data-section-mode="productMaster"'),
    source.indexOf('aria-label="客户商品分类模板视图"'),
  )

  assert.match(source, /BusinessGroupControls/)
  assert.match(source, /groupRowsByBusinessGroupTemplate/)
  assert.match(componentSource, /选择分组模板/)
  assert.match(componentSource, /移动到分类/)
  assert.doesNotMatch(productArchiveBlock, /<div v-if="selectedProductGroupTemplate" class="classification-tabs">/)
  assert.doesNotMatch(productArchiveBlock, /<button[^>]*class="classification-tab"/)
  assert.doesNotMatch(productArchiveBlock, /<th>分类<\/th>/)
  assert.doesNotMatch(productArchiveBlock, /productClassificationLabel\(row\)/)
})
