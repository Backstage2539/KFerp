import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

import * as orderEntry from './order-entry.js'
import { buildProductCatalogTemplatePriceListTypeOptions } from './product-price-list-types.js'
import {
  activeBeanListPublicationIDsByType,
  beanListVersionGroupForPublicationID,
  beanListVersionOptionGroups,
  beanListVersionOptionForGroup,
  beanListVersionOptionForProductGroups,
  beanListVersionOptionsForCustomer,
  buildOrderPayload,
  closeOrderProductDropdowns,
  CUSTOM_SPEC_VALUE,
  defaultDripSalesUnit,
  defaultDripSalesUnitSpec,
  defaultWholesaleSpec,
  defaultStatusID,
  filterOptions,
  filterProductsForCustomer,
  formatSpecLabel,
  dripTierPriceRows,
  lineDiscountAmount,
  lineTotal,
  latestBeanListVersionOption,
  latestProductPriceListVersionOption,
  isOrderProductFamily,
  needsTrailingBlankOrderLine,
  normalizeOrderProductFamilies,
  orderFamilyDefaultSpec,
  orderFamilyForSKU,
  orderFamilySearchScopeForPublication,
  orderFamilySpecRowPatch,
  orderFamilySpecsForPublication,
  orderFamilySpecProduct,
  orderFamilySpecOptions,
  orderLegacyProductForPublication,
  orderProductFamilyOptions,
  orderProductFamilyForContext,
  orderProductFamilyIdentity,
  orderProductKindFilterOptions,
  orderSpecSelectionAfterPublicationChange,
  orderRowPriceUnit,
  resolveWholesaleTierPrice,
  syncDripTierPrice,
  syncWholesaleTierPrice,
  normalizeSpecG,
  orderReceiptMethodOptions,
  productKindBadgeClass,
  productKindLabel,
  productBeanListType,
  requiresOrderPaymentMethod,
  requiresOrderPaymentReceipt,
  responsibleOptions,
  retailPackagePrice,
  retailSpecOptions,
  rowUsesStaleBeanListPublication,
  shouldKeepFrozenOrderPublication,
  sortProductsByCustomerUsage,
  wholesalePriceUnit,
  wholesaleTierPriceRows,
  wholesaleSpecOptions,
} from './order-entry.js'

test('order price-list families keep one parent result while matching concrete spec and SKU text', () => {
  const families = normalizeOrderProductFamilies([{
    parent_product_id: 70,
    parent_product_name: '乌拉嘎',
    name: '乌拉嘎',
    product_kind: 'roasted_bean',
    default_sku_id: 702,
    specs: [
      { sku_id: 701, sku_name: 'SKU-ULG-100', spec_label: '100g', tiers: [{ id: 1, publication_id: 91, unit_price: 39 }] },
      { sku_id: 702, sku_name: 'SKU-ULG-227', spec_label: '227g', is_default_sku: true, tiers: [{ id: 2, publication_id: 91, unit_price: 68 }] },
    ],
  }], [])

  assert.equal(families.length, 1)
  assert.equal(families[0].name, '乌拉嘎')
  assert.deepEqual(orderProductFamilyOptions(families, '227g').map((item) => item.parent_product_id), [70])
  assert.deepEqual(orderProductFamilyOptions(families, 'SKU-ULG-100').map((item) => item.parent_product_id), [70])
})

test('explicit legacy product family keeps one product with its concrete specification choices', () => {
  const family = normalizeOrderProductFamilies([{
    parent_product_id: 550,
    parent_product_name: '乌拉嘎',
    name: '客户专属豆',
    alias_name: '客户专属豆',
    __order_concrete_price_family: false,
    default_sku_id: 552,
    specs: [
      { sku_id: 551, spec_label: '227g', tiers: [{ id: 11, publication_id: 901, spec_g: 227, unit_price: 68 }] },
      { sku_id: 552, spec_label: '454g', tiers: [{ id: 12, publication_id: 901, spec_g: 454, unit_price: 118 }] },
    ],
  }], [])[0]

  assert.equal(isOrderProductFamily(family), true)
  assert.equal(orderProductFamilyIdentity(family), '0:550:0')
  assert.equal(family.__order_concrete_price_family, false)
  assert.deepEqual(orderProductFamilyOptions([family], '乌拉嘎').map((item) => item.name), ['客户专属豆'])
  assert.deepEqual(orderProductFamilyOptions([family], '客户专属豆').map((item) => item.parent_product_id), [550])
  assert.deepEqual(orderFamilySpecOptions(family, 901).map((item) => item.label), ['227g', '454g'])
  assert.equal(orderFamilyDefaultSpec(family, 901)?.sku_id, 552)
})

test('product family identity keeps public and customer aliases of one parent separate', () => {
  const families = normalizeOrderProductFamilies([
    { parent_product_id: 550, customer_id: 0, customer_product_alias_id: 0, name: '乌拉嘎', specs: [{ sku_id: 551, spec_label: '227g' }] },
    { parent_product_id: 550, customer_id: 8, customer_product_alias_id: 81, name: '客户专属豆', specs: [{ sku_id: 551, spec_label: '227g' }] },
  ], [])

  assert.deepEqual(families.map((family) => family.family_key), ['0:550:0', '8:550:81'])
  assert.equal(new Set(families.map(orderProductFamilyIdentity)).size, 2)
  assert.equal(orderProductFamilyForContext(families, { customer_id: 8 })?.name, '客户专属豆')
  assert.equal(orderProductFamilyForContext(families, { customer_product_alias_id: 81 })?.name, '客户专属豆')
  assert.equal(orderProductFamilyForContext(families, { customer_id: 8, product_family_key: '0:550:0' })?.name, '乌拉嘎')
})

test('family spec enrichment never crosses public and customer aliases sharing one SKU', () => {
  const families = normalizeOrderProductFamilies([
    { parent_product_id: 550, customer_id: 0, customer_product_alias_id: 0, name: '乌拉嘎', specs: [{ sku_id: 551, spec_label: '227g' }] },
    { parent_product_id: 550, customer_id: 8, customer_product_alias_id: 81, name: '客户专属豆', specs: [{ sku_id: 551, spec_label: '227g' }] },
    { parent_product_id: 550, customer_id: 9, customer_product_alias_id: 82, name: '另一客户豆', specs: [{ sku_id: 551, spec_label: '227g' }] },
  ], [
    { id: 551, sku_id: 551, parent_product_id: 550, customer_id: 0, customer_product_alias_id: 0, product_code: 'PUBLIC-551' },
    { id: 551, sku_id: 551, parent_product_id: 550, customer_id: 8, customer_product_alias_id: 81, product_code: 'ALIAS-551' },
    { id: 551, sku_id: 551, parent_product_id: 550, customer_id: 9, customer_product_alias_id: 82, product_code: 'ALIAS-B-551' },
  ])

  assert.equal(families[0].specs[0].product_code, 'PUBLIC-551')
  assert.equal(families[1].specs[0].product_code, 'ALIAS-551')
  assert.equal(families[2].specs[0].product_code, 'ALIAS-B-551')
  assert.equal(families[0].specs[0].customer_product_alias_id, 0)
  assert.equal(families[1].specs[0].customer_product_alias_id, 81)
  assert.equal(families[2].specs[0].customer_product_alias_id, 82)
})

test('an unpriced explicit product family still exposes its maintained specifications', () => {
  const family = normalizeOrderProductFamilies([{
    parent_product_id: 550,
    parent_product_name: '乌拉嘎',
    name: '乌拉嘎',
    specs: [
      { sku_id: 551, spec_label: '227g', tiers: [] },
      { sku_id: 552, spec_label: '454g', tiers: [] },
    ],
  }], [])[0]

  assert.deepEqual(orderFamilySpecOptions(family, 0).map((item) => item.label), ['227g', '454g'])
})

test('maintained spec choices use current SKU metadata while selected price history keeps its frozen snapshot', () => {
  const family = normalizeOrderProductFamilies([{
    parent_product_id: 550,
    name: '乌拉嘎',
    specs: [{
      sku_id: 551,
      spec_label: '当前454g',
      tiers: [{
        publication_id: 901,
        effective_sales_spec: { sku_id: 551, spec_label: '历史500g' },
      }],
    }],
  }], [])[0]

  assert.deepEqual(orderFamilySpecOptions(family, 901).map((item) => item.label), ['当前454g'])
  assert.equal(orderSpecSelectionAfterPublicationChange(family, 551, 901)?.spec_label, '历史500g')
})

test('order product search only matches specifications sold by the selected publication', () => {
  const family = normalizeOrderProductFamilies([{
    parent_product_id: 550,
    name: '乌拉嘎',
    code: 'PARENT-550 SKU-100 SKU-1KG',
    product_code: 'SKU-1KG',
    specs: [
      { sku_id: 551, sku_code: 'SKU-100', spec_label: '100g', tiers: [{ publication_id: 901, unit_price: 39 }] },
      { sku_id: 552, sku_code: 'SKU-1KG', spec_label: '1Kg', tiers: [{ publication_id: 902, unit_price: 88 }] },
    ],
  }], [])[0]
  const scoped = orderFamilySearchScopeForPublication(family, 901)

  assert.deepEqual(scoped.specs.map((spec) => spec.sku_id), [551])
  assert.deepEqual(orderProductFamilyOptions([scoped], '100g').map((item) => item.parent_product_id), [550])
  assert.deepEqual(orderProductFamilyOptions([scoped], 'SKU-100').map((item) => item.parent_product_id), [550])
  assert.deepEqual(orderProductFamilyOptions([scoped], '1kg'), [])
  assert.deepEqual(orderProductFamilyOptions([scoped], 'SKU-1KG'), [])
})

test('order product options combine visible category and text filters', () => {
  const families = [
    { id: 1, name: '森林瑰夏水洗', product_kind: 'roasted_bean' },
    { id: 2, name: '初晓 挂耳', product_kind: 'drip_bag' },
    { id: 3, name: '红酒日晒 生豆', product_kind: 'green_bean' },
    { id: 4, name: '冷萃速溶', product_kind: 'instant_coffee' },
  ]

  assert.deepEqual(orderProductKindFilterOptions(families), [
    { value: '', label: '全部' },
    { value: 'roasted', label: '熟豆' },
    { value: 'drip_bag', label: '挂耳' },
    { value: 'green_bean', label: '生豆' },
    { value: 'instant_coffee', label: '速溶咖啡' },
  ])
  assert.deepEqual(orderProductFamilyOptions(families, '', 'drip_bag').map((item) => item.id), [2])
  assert.deepEqual(orderProductFamilyOptions(families, '初晓', 'drip_bag').map((item) => item.id), [2])
  assert.deepEqual(orderProductFamilyOptions(families, '森林', 'drip_bag'), [])
})

test('order product dropdown closer keeps only the clicked combobox open', () => {
  const rows = [
    { key: 'a', product_open: true },
    { key: 'b', product_open: true },
  ]

  closeOrderProductDropdowns(rows, 'b')
  assert.deepEqual(rows.map((row) => row.product_open), [false, true])

  closeOrderProductDropdowns(rows)
  assert.deepEqual(rows.map((row) => row.product_open), [false, false])
})

test('order price-list family fallback groups enriched flat products without appending specs to parent name', () => {
  const families = normalizeOrderProductFamilies([], [
    {
      id: 701,
      name: '乌拉嘎 100g',
      parent_product_id: 70,
      parent_product_name: '乌拉嘎',
      effective_sales_spec: '100g',
      tiers: [{
        id: 1,
        publication_id: 91,
        unit_price: 39,
        quantity_basis: 'sales_spec_count',
        effective_sales_spec: { sku_id: 701, spec_label: '100g', sales_unit: '袋' },
      }],
    },
    {
      id: 702,
      name: '乌拉嘎 227g',
      parent_product_id: 70,
      parent_product_name: '乌拉嘎',
      effective_sales_spec: '227g',
      is_default_sku: true,
      tiers: [{
        id: 2,
        publication_id: 91,
        unit_price: 68,
        quantity_basis: 'sales_spec_count',
        effective_sales_spec: { sku_id: 702, spec_label: '227g', sales_unit: '袋' },
      }],
    },
  ])

  assert.equal(families.length, 1)
  assert.equal(families[0].name, '乌拉嘎')
  assert.deepEqual(families[0].specs.map((item) => item.sku_id), [701, 702])
  assert.equal(orderFamilyForSKU(families, 702)?.parent_product_id, 70)
})

test('pure legacy fallback keeps same-parent historical SKU identities and their own price tiers', () => {
  const families = normalizeOrderProductFamilies([], [
    {
      id: 551,
      name: '乌拉嘎 227g',
      parent_product_id: 550,
      parent_product_name: '乌拉嘎',
      tiers: [{ id: 11, publication_id: 901, spec_g: 227, min: 1, unit_price: 68 }],
    },
    {
      id: 552,
      name: '乌拉嘎 454g',
      parent_product_id: 550,
      parent_product_name: '乌拉嘎',
      tiers: [{ id: 12, publication_id: 901, spec_g: 454, min: 1, unit_price: 118 }],
    },
  ])

  assert.deepEqual(families.map((product) => product.id), [551, 552])
  assert.deepEqual(families.map((product) => product.__order_legacy_price_product), [true, true])
  assert.deepEqual(families.map((product) => product.tiers.map((tier) => tier.id)), [[11], [12]])
})

test('pure legacy publication keeps the original common-spec order-entry path', () => {
  const families = normalizeOrderProductFamilies([], [{
    id: 7,
    name: '乌拉嘎',
    product_kind: 'roasted_bean',
    tiers: [{ id: 11, publication_id: 901, spec_g: 454, min: 1, unit_price: 68 }],
  }])

  assert.equal(families.length, 1)
  assert.equal(families[0].__order_concrete_price_family, false)
  assert.ok(wholesaleSpecOptions(families[0]).some((option) => option.value === '227'))
  assert.ok(wholesaleSpecOptions(families[0]).some((option) => option.value === CUSTOM_SPEC_VALUE))
})

test('mixed concrete and legacy history routes each selected publication to its own order-entry path', () => {
  const mixedProduct = {
    id: 551,
    name: '乌拉嘎 227g',
    parent_product_id: 550,
    parent_product_name: '乌拉嘎',
    tiers: [
      {
        id: 21,
        publication_id: 902,
        quantity_basis: 'sales_spec_count',
        effective_sales_spec: { sku_id: 551, spec_label: '227g', sales_unit: '袋' },
        unit_price: 70,
      },
      { id: 11, publication_id: 901, spec_g: 227, min: 1, unit_price: 68 },
    ],
  }

  assert.equal(orderLegacyProductForPublication(mixedProduct, 902), null)
  const legacy = orderLegacyProductForPublication(mixedProduct, 901)
  assert.equal(legacy?.__order_legacy_price_product, true)
  assert.deepEqual(legacy?.tiers.map((tier) => tier.publication_id), [901])
  assert.ok(wholesaleSpecOptions(legacy).some((option) => option.value === CUSTOM_SPEC_VALUE))
})

test('publication pricing mode detects a legacy to concrete price-list transition', () => {
  const product = {
    tiers: [
      { id: 11, publication_id: 901, spec_g: 227, min: 1, unit_price: 68 },
      {
        id: 21,
        publication_id: 902,
        quantity_basis: 'sales_spec_count',
        effective_sales_spec: { sku_id: 551, spec_label: '227g', sales_unit: '袋' },
        unit_price: 70,
      },
    ],
  }

  assert.equal(orderEntry.orderProductPublicationMode(product, 901), 'legacy')
  assert.equal(orderEntry.orderProductPublicationMode(product, 902), 'concrete')
  assert.equal(orderEntry.orderProductPublicationMode(product, 999), '')
})

test('an unrelated legacy product remains available when another product has a concrete family', () => {
  const families = normalizeOrderProductFamilies([{
    parent_product_id: 70,
    parent_product_name: '乌拉嘎',
    name: '乌拉嘎',
    specs: [{
      sku_id: 701,
      spec_label: '227g',
      tiers: [{
        id: 21,
        publication_id: 902,
        quantity_basis: 'sales_spec_count',
        effective_sales_spec: { sku_id: 701, spec_label: '227g', sales_unit: '袋' },
        unit_price: 70,
      }],
    }],
  }], [{
    id: 8,
    name: '历史豆',
    product_kind: 'roasted_bean',
    tiers: [{ id: 12, publication_id: 901, spec_g: 454, min: 1, unit_price: 66 }],
  }])

  assert.deepEqual(families.map((family) => [family.parent_product_id, family.__order_concrete_price_family]), [
    [70, true],
    [8, false],
  ])
})

test('order spec choices only include concrete SKUs priced by the selected publication', () => {
  const family = normalizeOrderProductFamilies([{
    parent_product_id: 70,
    parent_product_name: '乌拉嘎',
    name: '乌拉嘎',
    default_sku_id: 702,
    specs: [
      { sku_id: 701, spec_label: '100g', tiers: [{ id: 1, publication_id: 91, unit_price: 39 }] },
      { sku_id: 702, spec_label: '227g', is_default_sku: true, tiers: [{ id: 2, publication_id: 92, unit_price: 68 }] },
      { sku_id: 703, spec_label: '454g', tiers: [] },
    ],
  }], [])[0]

  assert.deepEqual(orderFamilySpecOptions(family, 91), [
    { label: '100g', value: '701', skuID: 701 },
  ])
  assert.equal(orderFamilyDefaultSpec(family, 91)?.sku_id, 701)
  assert.deepEqual(orderFamilySpecsForPublication(family, 92).map((item) => item.sku_id), [702])
  assert.equal(orderFamilyDefaultSpec(family, 92)?.sku_id, 702)
  assert.deepEqual(orderFamilySpecOptions(family, 999), [])
  assert.equal(orderFamilyDefaultSpec(family, 999), null)
  assert.equal(orderSpecSelectionAfterPublicationChange(family, 702, 92)?.sku_id, 702)
  assert.equal(orderSpecSelectionAfterPublicationChange(family, 702, 91), null)
})

test('selecting an order price-list spec freezes concrete SKU identity while keeping the parent product name', () => {
  const family = normalizeOrderProductFamilies([{
    parent_product_id: 70,
    parent_product_name: '乌拉嘎',
    name: '客户别名乌拉嘎',
    customer_product_alias_id: 19,
    customer_product_display_name: '客户别名乌拉嘎',
    product_kind: 'roasted_bean',
    specs: [{
      sku_id: 702,
      sku_name: 'SKU-ULG-227',
      product_code: 'SKU-ULG-227',
      spec_label: '227g',
      net_content_qty: 227,
      net_content_unit: 'g',
      order_unit: '件',
      tiers: [{
        id: 2,
        publication_id: 92,
        version_no: 'V3.0.6',
        unit_price: 68,
        display_unit: '227g',
        quantity_basis: 'sales_spec_count',
      }],
    }],
  }], [])[0]
  const spec = orderFamilyDefaultSpec(family, 92)
  const patch = orderFamilySpecRowPatch(family, spec, 92)
  const product = orderFamilySpecProduct(family, spec, 92)

  assert.equal(patch.parent_product_id, 70)
  assert.equal(patch.product_id, 702)
  assert.equal(patch.product_name, '客户别名乌拉嘎')
  assert.equal(patch.product_record_name, '乌拉嘎')
  assert.equal(patch.product_query, '客户别名乌拉嘎')
  assert.equal(patch.spec_mode, '702')
  assert.equal(patch.spec_label, '227g')
  assert.equal(patch.spec_g, 227)
  assert.equal(patch.unit, '件')
  assert.equal(patch.bean_list_publication_id, 92)
  assert.equal(patch.bean_list_version_no, 'V3.0.6')
  assert.equal(product.id, 702)
  assert.equal(product.name, '客户别名乌拉嘎')
  assert.deepEqual(product.tiers.map((tier) => tier.id), [2])
})

test('the selected publication effective sales spec overrides stale top-level SKU metadata', () => {
  const family = normalizeOrderProductFamilies([{
    parent_product_id: 70,
    parent_product_name: '乌拉嘎',
    name: '乌拉嘎',
    product_kind: 'roasted_bean',
    default_sku_id: 702,
    specs: [{
      sku_id: 702,
      sku_name: '227g袋装',
      spec_label: '227g',
      net_content_qty: 227,
      net_content_unit: 'g',
      sales_unit: '件',
      tiers: [
        {
          id: 21,
          publication_id: 91,
          version_no: 'V1',
          unit_price: 68,
          effective_sales_spec: {
            sku_id: 702,
            spec_key: 'bag-227g',
            spec_name: '227g袋装',
            spec_label: '227g',
            sales_unit: '袋',
            net_content_qty: 227,
            net_content_unit: 'g',
            product_kind: 'roasted_bean',
          },
        },
        {
          id: 22,
          publication_id: 92,
          version_no: 'V2',
          unit_price: 118,
          effective_sales_spec: {
            sku_id: 702,
            spec_key: 'bag-454g',
            spec_name: '454g袋装',
            spec_label: '454g',
            sales_unit: '袋',
            net_content_qty: 454,
            net_content_unit: 'g',
            inventory_unit: 'g',
            inventory_conversion_json: { '袋': { g: 454 } },
            product_kind: 'instant_coffee',
          },
        },
      ],
    }],
  }], [])[0]

  const selected = orderFamilySpecsForPublication(family, 92)[0]
  const patch = orderFamilySpecRowPatch(family, selected, 92)
  const product = orderFamilySpecProduct(family, selected, 92)

  assert.equal(selected.spec_label, '454g')
  assert.equal(selected.sku_name, '454g袋装')
  assert.equal(selected.sales_unit, '袋')
  assert.equal(selected.net_content_qty, 454)
  assert.equal(selected.net_content_unit, 'g')
  assert.equal(selected.product_kind, 'instant_coffee')
  assert.deepEqual(selected.inventory_conversion_json, { '袋': { g: 454 } })
  assert.equal(patch.spec_label, '454g')
  assert.equal(patch.spec_g, 454)
  assert.equal(patch.unit, '袋')
  assert.equal(patch.product_kind, 'instant_coffee')
  assert.equal(patch.bean_list_version_no, 'V2')
  assert.equal(product.effective_sales_spec.spec_key, 'bag-454g')
})

test('order payload submits the authoritative parent and concrete SKU separately', () => {
  const payload = buildOrderPayload({
    form: {},
    rows: [{
      parent_product_id: 70,
      product_id: 702,
      product_name: '乌拉嘎',
      product_record_name: '乌拉嘎',
      product_kind: 'roasted_bean',
      spec_source: 'price_list_sku',
      spec_mode: '702',
      spec_label: '227g',
      spec_g: 227,
      qty: 3,
      unit: '件',
      unit_price: 68,
      tier_id: 2,
    }],
  })

  assert.deepEqual(payload.product_id, ['702'])
  assert.deepEqual(payload.parent_product_id, ['70'])
  assert.deepEqual(payload.product_name_snapshot, ['乌拉嘎'])
  assert.deepEqual(payload.item_name, ['乌拉嘎'])
  assert.deepEqual(payload.spec, ['227'])
})

test('OrderEntryView uses parent product families and concrete published SKU specs without global gram choices', () => {
  const source = orderEntryViewSource()

  assert.match(source, /const productFamilies = ref\(\[\]\)/)
  assert.match(source, /normalizeOrderProductFamilies\([\s\S]*?data\.product_families \|\| \[\],[\s\S]*?products\.value,[\s\S]*?data\.product_bom_spec_options \|\| \[\],[\s\S]*?\)/)
  assert.match(source, /orderProductFamilyOptions\(/)
  assert.doesNotMatch(source, /scopedLegacyProducts/)
  assert.match(source, /:key="productOptionKey\(product\)"/)
  assert.match(source, /__order_legacy_price_product\) return `legacy:\$\{Number\(product\.id \|\| 0\)\}`/)
  assert.match(source, /return `family:\$\{orderProductFamilyIdentity\(product\)\}`/)
  assert.match(source, /product_family_key: ''/)
  assert.match(source, /orderProductFamilyForContext\(candidates/)
  assert.match(source, /orderFamilySpecOptions\(/)
  assert.match(source, /function productSpecCountForCurrentList\(family\)[\s\S]*?selectedBeanListVersionOptionForProduct\(family\)[\s\S]*?orderFamilySpecOptions\(family, selected\.id\)\.length/)
  assert.match(source, /const selectedPublicationID = Number\(selected\?\.id \|\| 0\)[\s\S]*?if \(selectedPublicationID <= 0\)[\s\S]*?invalidatePriceListSpecRow/)
  assert.match(source, /const defaultSpec = orderFamilyDefaultSpec\(family, selectedPublicationID\)[\s\S]*?orderSpecSelectionAfterPublicationChange\([\s\S]*?defaultSpec\?\.sku_id,[\s\S]*?selectedPublicationID/)
  assert.match(source, /orderFamilySearchScopeForPublication\(family, selected\.id\)/)
  assert.match(source, /if \(!isOrderProductFamily\(family\)\) return \[family\]/)
  assert.doesNotMatch(source, /if \(!family\?\.__order_concrete_price_family\) return \[family\]/)
  assert.match(source, /isOrderProductFamily\(family\)/)
  assert.match(source, /function onSpecChange\(row\)/)
  assert.match(source, /当前价格表未发布该商品规格，不能保存；请补齐价格表或选择有价规格。/)
  assert.match(source, /const spec = orderSpecSelectionAfterPublicationChange\(/)
  assert.doesNotMatch(source, /if \(!spec\) return productByID\(row\?\.product_id, row\)/)
  assert.match(source, /function invalidatePriceListSpecRow\(row/)
  assert.match(source, /所选价格表不包含该商品当前规格，请重新选择规格/)
  assert.match(source, /row\.spec_source === 'legacy_price_list' && options\.priceListChanged/)
  assert.match(source, /syncBeanListVersionForCustomer\(\{ force: !!copyID \}\)/)
  assert.match(source, /if \(copyID\) syncRowsForType\(\{ priceListChanged: true \}\)/)
  assert.match(source, /orderProductPublicationMode\(legacyProduct \|\| row, selectedPublicationID\)/)
  assert.match(source, /const publicationMode = orderProductPublicationMode\([\s\S]*?family \|\| product \|\| flatProduct \|\| hydrated,[\s\S]*?item\.bean_list_publication_id/)
  assert.match(source, /hydrated\.spec_source = publicationMode === 'legacy' \? 'legacy_price_list' : 'price_list_sku'/)
  assert.match(source, /const keepFrozenPublication = shouldKeepFrozenOrderPublication\([\s\S]*?copyMode\.value/)
  assert.match(source, /hydrated\.historical_spec_readonly = !pricedSpec \|\| keepFrozenPublication/)
  assert.match(source, /已切换到具体规格价格表，请重新选择价格表中的规格/)
  assert.match(source, /isConcretePriceListRow\(row\)[\s\S]*?onSpecChange\(row\)/)
  assert.match(source, /historical_spec_readonly/)
  assert.match(source, /历史规格，当前价格表不可用/)

  const concreteSpecBlock = source.slice(source.indexOf('<div class="spec-control">'), source.indexOf('</div>', source.indexOf('<div class="spec-control">')))
  assert.match(concreteSpecBlock, /isConcretePriceListRow\(row\)/)
  assert.doesNotMatch(concreteSpecBlock, /COMMON_SPEC_GRAMS/)
  assert.match(source, /v-if="!isConcretePriceListRow\(row\) && row\.spec_mode === CUSTOM_SPEC_VALUE"/)
})

test('OrderEntryView manual explains parent products and published price-list specs instead of common grams', () => {
  const source = orderEntryViewSource()
  const manual = source.slice(source.indexOf('<details class="manual">'), source.indexOf('</details>', source.indexOf('<details class="manual">')))

  assert.match(manual, /商品按父商品展示/)
  assert.match(manual, /规格列和规格搜索只使用当前已选已发布价格表中有价的规格/)
  assert.match(manual, /价格表没有的商品档案规格不会出现在新订单候选中/)
  assert.match(manual, /切换价格表版本/)
  assert.doesNotMatch(manual, /常用规格：36g/)
})

test('挂耳新录单统一选择 commercial 商品价格表', () => {
  assert.equal(productBeanListType({ product_kind: 'drip_bag' }), 'commercial')
  assert.equal(productBeanListType({ product_kind: 'roasted' }), 'commercial')
  assert.equal(productBeanListType({ product_kind: 'green_bean' }), 'green')
})

test('挂耳 commercial 阶梯使用 API 的 min_qty/max_qty 且盒价不重复乘袋数', () => {
  const product = {
    product_kind: 'drip_bag',
    drip_bag_grams: 10,
    drip_box_bag_count: 10,
    tiers: [
      { id: 1, product_kind: 'drip_bag', sales_unit: 'box', unit_bag_count: 10, min_qty: 10, max_qty: 99, unit_price: 32.8 },
      { id: 2, product_kind: 'drip_bag', sales_unit: 'box', unit_bag_count: 10, min_qty: 100, unit_price: 30.8 },
    ],
  }
  assert.deepEqual(syncDripTierPrice(product, { sales_unit: 'box', unit_bag_count: 10, qty: 20 }), { tierID: '1', unitPrice: '32.8' })
  assert.equal(dripTierPriceRows(product, { sales_unit: 'box', unit_bag_count: 10 })[0].rangeLabel, '10-99盒')
})

test('新价格表按 concrete SKU 销售规格件数命中阶梯，旧发布继续按重量回退', () => {
	const countProduct = {
	  tiers: [{
	    id: 501,
	    spec_g: 1000,
	    min: 2,
	    max: 4,
	    unit_price: 68,
	    display_unit: '磅',
	    quantity_basis: 'sales_spec_count',
	    tier_quantity_unit: '磅',
	    price_source_json: '{"source":"published_bean_list","quantity_basis":"sales_spec_count"}',
	  }],
	}
	const countRow = { spec_mode: '227', qty: 2, tier_id: 'auto', unit_price: '' }
	const countResult = resolveWholesaleTierPrice(countProduct, countRow)
	assert.equal(countResult.tierID, '501')
	assert.equal(countResult.unitPrice, '68')
	assert.equal(lineTotal(countProduct, {
	  ...countRow,
	  tier_id: countResult.tierID,
	  unit_price: countResult.unitPrice,
	  price_unit: countResult.priceUnit.label,
	  price_unit_suffix: countResult.priceUnit.suffix,
	  price_unit_g: countResult.priceUnit.unitG,
	}, false), 136)

	const legacyProduct = {
	  tiers: [{ id: 502, spec_g: 1000, min_qty: 1, unit_price: 82, display_unit: 'kg' }],
	}
	const legacyResult = resolveWholesaleTierPrice(legacyProduct, { spec_mode: '454', qty: 3, tier_id: 'auto', unit_price: '' })
	assert.equal(legacyResult.tierID, '502')
	assert.equal(legacyResult.unitPrice, '82')
	assert.equal(Number(lineTotal(legacyProduct, {
	  spec_mode: '454',
	  qty: 3,
	  tier_id: legacyResult.tierID,
	  unit_price: legacyResult.unitPrice,
	  price_unit: legacyResult.priceUnit.label,
	  price_unit_suffix: legacyResult.priceUnit.suffix,
	  price_unit_g: legacyResult.priceUnit.unitG,
	}, false).toFixed(3)), 111.684)
})

test('零售订单优先使用 concrete SKU 发布价并按件计价和折扣', () => {
	const countProduct = {
		retail_price_227g: 50,
		tiers: [{
			id: 503,
			spec_g: 227,
			min: 1,
			max: null,
			unit_price: 68,
			display_unit: '227g',
			quantity_basis: 'sales_spec_count',
			price_source_json: '{"source":"published_bean_list","list_type":"retail","quantity_basis":"sales_spec_count"}',
		}],
	}
	const resolved = resolveWholesaleTierPrice(countProduct, { spec_mode: '227', qty: 2, tier_id: 'auto' })
	assert.equal(resolved.tierID, '503')
	assert.equal(resolved.quantityBasis, 'sales_spec_count')
	assert.equal(lineTotal(countProduct, {
		spec_mode: '227',
		qty: 2,
		tier_id: resolved.tierID,
		unit_price: resolved.unitPrice,
		quantity_basis: resolved.quantityBasis,
		discount_type: 'unit_amount',
		discount_value: 10,
	}, true), 116)
})

test('零售订单有新发布件数阶梯但数量无档位时不回退到主数据价', () => {
	const resolved = resolveWholesaleTierPrice({
		tiers: [{
			id: 504,
			spec_g: 227,
			min: 10,
			max: 20,
			unit_price: 68,
			quantity_basis: 'sales_spec_count',
		}],
	}, { spec_mode: '227', qty: 2, tier_id: 'auto' })
	assert.equal(resolved.tierID, 'auto')
	assert.equal(resolved.quantityBasis, 'sales_spec_count')
	assert.equal(resolved.priceMissing, true)
})

test('选中零售发布版本时不回退到另一商用发布版本', () => {
	const resolved = resolveWholesaleTierPrice({
		tiers: [{
			id: 505,
			min: 1,
			max: null,
			unit_price: 68,
			quantity_basis: 'sales_spec_count',
			price_source_json: '{"source":"published_bean_list","list_type":"commercial","publication_id":11,"quantity_basis":"sales_spec_count"}',
		}],
	}, { spec_mode: '227', qty: 2, tier_id: 'auto', bean_list_publication_id: 22 })
	assert.equal(resolved.tierID, 'auto')
	assert.equal(resolved.unitPrice, '')
	assert.equal(resolved.priceMissing, true)
})

test('手动价保留发布价的销售规格件数口径', () => {
	const priceSourceJSON = '{"source":"published_bean_list","publication_id":22,"quantity_basis":"sales_spec_count"}'
	const payload = buildOrderPayload({
		form: {},
		rows: [{
			product_id: 558,
			product_name: '初晓',
			product_kind: 'roasted_bean',
			spec_mode: '227',
			qty: 2,
			tier_id: 'manual',
			unit_price: '68',
			price_source_json: priceSourceJSON,
		}],
	})
	assert.deepEqual(payload.price_source_json, [priceSourceJSON])
})

test('挂耳派生盒 SKU 从录单单位或唯一阶梯推导盒装默认值', () => {
  const boxProduct = {
    product_kind: 'drip_bag',
    order_unit: '盒（10袋）',
    drip_bag_grams: 10,
    drip_box_bag_count: 10,
    tiers: [{ product_kind: 'drip_bag', sales_unit: 'box', unit_bag_count: 10 }],
  }
  assert.equal(defaultDripSalesUnit(boxProduct), 'box')
  const boxSpec = defaultDripSalesUnitSpec(boxProduct)
  assert.deepEqual(boxSpec, {
    salesUnit: 'box', unitBeanG: 10, unitBagCount: 10, unitLabel: '盒', specG: 100, specLabel: '10袋/盒',
  })
  const payload = buildOrderPayload({
    form: {},
    rows: [{
      product_id: 701, product_name: '金色山脉 挂耳 盒（10袋）', product_kind: 'drip_bag', qty: 2,
      sales_unit: boxSpec.salesUnit, unit_bag_count: boxSpec.unitBagCount, unit_bean_g: boxSpec.unitBeanG,
    }],
  })
  assert.deepEqual(payload.sales_unit, ['box'])
  assert.deepEqual(payload.unit_bag_count, ['10'])
  assert.deepEqual(payload.spec, ['100'])
  assert.deepEqual(payload.unit, ['盒'])
  assert.equal(defaultDripSalesUnit({ product_kind: 'drip_bag', tiers: [{ product_kind: 'drip_bag', sales_unit: 'box' }] }), 'box')
  assert.match(orderEntryViewSource(), /const defaultSpec = defaultDripSalesUnitSpec\(product\)/)
  assert.match(orderEntryViewSource(), /row\.unit_bag_count = defaultSpec\.unitBagCount/)
  assert.match(orderEntryViewSource(), /row\.unit = defaultSpec\.unitLabel/)
})

function orderEntryViewSource() {
  return readFileSync(new URL('../views/OrderEntryView.vue', import.meta.url), 'utf8')
}

function ordersViewSource() {
  return readFileSync(new URL('../views/OrdersView.vue', import.meta.url), 'utf8')
}

function salesOrderViewSource() {
  return readFileSync(new URL('../views/SalesOrderView.vue', import.meta.url), 'utf8')
}

function cssBlock(source, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = source.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`, 's'))
  return match?.[1] || ''
}

function sourceAfter(source, marker) {
  const index = source.indexOf(marker)
  return index >= 0 ? source.slice(index) : ''
}

function zIndexForSelector(source, selector) {
  const match = cssBlock(source, selector).match(/z-index:\s*(\d+)/)
  return Number(match?.[1] || 0)
}

const product = {
  id: 7,
  name: '橘皮乌龙',
  retail_price_100g: 42,
  retail_price_200g: 0,
  retail_price_227g: 50,
  retail_price_250g: 56,
  retail_specs: [100, 227, 250],
  tiers: [],
}

test('retailPackagePrice uses exact configured spec price first', () => {
  assert.equal(retailPackagePrice(product, 250), 56)
})

test('retailPackagePrice falls back to 227g retail price for custom grams', () => {
  assert.equal(retailPackagePrice(product, 300), 67)
})

test('retailSpecOptions includes custom sentinel for retail orders', () => {
  assert.deepEqual(retailSpecOptions(product, true), [
    { label: '36g', value: '36' },
    { label: '80g', value: '80' },
    { label: '100g', value: '100' },
    { label: '227g', value: '227' },
    { label: '250g', value: '250' },
    { label: '454g', value: '454' },
    { label: '500g', value: '500' },
    { label: '1000g', value: '1000' },
    { label: '2.5kg', value: '2500' },
    { label: '自定义克数', value: 'custom' },
  ])
})

test('order entry labels instant coffee as its own product kind', () => {
  assert.equal(productKindLabel('instant_coffee'), '速溶咖啡')
  assert.equal(productKindBadgeClass('instant_coffee'), 'kind-instant')
  assert.equal(productKindLabel({ product_kind: 'drip_bag' }), '挂耳')
  assert.equal(productKindBadgeClass({ product_kind: 'drip_bag' }), 'kind-drip')
})

test('buildOrderPayload saves real custom spec grams', () => {
  const payload = buildOrderPayload({
    form: {
      document_date: '2026-05-23',
      order_date: '2026-04-25',
      customer_id: 3,
      source_id: 1,
      order_type_id: 2,
      pay_status_id: 1,
      ship_status_id: 1,
      notes: '',
      shipping_amount: 0,
      discount_amount: 0,
      round_to_int: false,
    },
    rows: [
      {
        product_id: 7,
        product_name: '橘皮乌龙',
        tier_id: 'auto',
        spec_mode: 'custom',
        custom_spec_g: 300,
        qty: 2,
        unit: '件',
        unit_price: 0,
      },
    ],
  })

  assert.equal(payload.document_date, '2026-05-23')
  assert.equal(payload.product_id[0], '7')
  assert.equal(payload.spec[0], '300')
  assert.equal(payload.qty[0], '2')
  assert.equal(payload.item_name[0], '橘皮乌龙')
})

test('buildOrderPayload freezes customer alias and product snapshots on order lines', () => {
  const payload = buildOrderPayload({
    form: {
      document_date: '2026-05-31',
      order_date: '2026-05-31',
      customer_id: 42,
      source_id: 1,
      order_type_id: 2,
      pay_status_id: 1,
      ship_status_id: 1,
    },
    rows: [
      {
        product_id: 77,
        product_name: 'Karen 贴牌意式',
        product_code: 'SKU-77',
        product_record_name: '精品意式拼配',
        customer_product_alias_id: 910,
        customer_item_code: 'KAREN-ESP',
        brand_name: '',
        tier_id: 'auto',
        spec_mode: '454',
        qty: 3,
        unit: '件',
        unit_price: 68,
        bean_list_publication_id: 8001,
        bean_list_version_no: 'KAREN-V1',
        price_source_json: '{"publication_id":8001,"customer_product_alias_id":910}',
      },
    ],
  })

  assert.equal(payload.customer_product_alias_id[0], '910')
  assert.equal(payload.customer_product_display_name_snapshot[0], 'Karen 贴牌意式')
  assert.equal(payload.customer_item_code_snapshot[0], 'KAREN-ESP')
  assert.equal(payload.brand_name_snapshot[0], '')
  assert.equal(payload.product_code_snapshot[0], 'SKU-77')
  assert.equal(payload.product_name_snapshot[0], '精品意式拼配')
  assert.equal(payload.item_bean_list_publication_id[0], '8001')
  assert.equal(payload.item_bean_list_version_no[0], 'KAREN-V1')
  assert.equal(payload.price_source_json[0], '{"publication_id":8001,"customer_product_alias_id":910}')
})

test('order entry exposes document date, order date, and order backfill labels', () => {
  const source = orderEntryViewSource()

  assert.match(source, /订单补录/)
  assert.match(source, /单据日期/)
  assert.match(source, /订单日期/)
  assert.match(source, /form\.document_date/)
  assert.match(source, /form\.order_date/)
})

test('order entry exposes continuous backfill mode and save flow', () => {
  const source = orderEntryViewSource()
  const resetSource = sourceAfter(source, 'function resetForBackfillContinuation')
  const saveSource = sourceAfter(source, 'async function save(')

  assert.match(source, /backfillMode/)
  assert.match(source, /保存并继续补录/)
  assert.match(source, /保存并查看订单/)
  assert.match(source, /save\(\{ continueBackfill: true \}\)/)
  assert.match(source, /resetForBackfillContinuation/)
  assert.match(saveSource, /continueBackfill/)
  assert.match(saveSource, /resetForBackfillContinuation\(\)/)
  assert.match(saveSource, /if \(!props\.embedded && data\.redirect_url\) window\.location\.href = data\.redirect_url/)

  for (const want of [
    "form.ship_tracking_no = ''",
    "form.payment_goods_amount = ''",
    "form.payment_shipping_amount = ''",
    'form.payment_voucher_asset_id = 0',
    "form.notes = ''",
    "form.discount_amount = ''",
    "form.outsource_material_fee = ''",
    "form.outsource_roast_fee = ''",
    "form.outsource_packaging_fee = ''",
    "form.outsource_manual_fee = ''",
    "form.outsource_tax_fee = ''",
    "form.outsource_other_fee = ''",
    'rows.value = [newRow()]',
    'paymentVoucher.value = null',
    'paymentVoucherFile.value = null',
    'saveOrderEntryDraft()',
  ]) {
    assert.ok(resetSource.includes(want), `resetForBackfillContinuation missing ${want}`)
  }
})

test('buildOrderPayload carries per-item notes with order detail rows', () => {
  const payload = buildOrderPayload({
    form: {
      order_date: '2026-05-09',
      customer_id: 3,
      source_id: 1,
      order_type_id: 1,
      pay_status_id: 2,
      ship_status_id: 1,
      notes: '整单备注',
      shipping_amount: 0,
      discount_amount: 0,
      round_to_int: false,
    },
    rows: [
      {
        product_id: 7,
        product_name: '橘皮乌龙',
        tier_id: 'manual',
        spec_mode: '454',
        qty: 2,
        unit: '件',
        unit_price: 88,
        item_note: '贴标：A店',
      },
      {
        product_id: 8,
        product_name: '榛巧拼配',
        tier_id: 'auto',
        spec_mode: '1000',
        qty: 1,
        unit: '件',
        unit_price: 106,
        item_note: '',
      },
    ],
  })

  assert.deepEqual(payload.item_note, ['贴标：A店', ''])
  assert.deepEqual(payload.item_name, ['橘皮乌龙', '榛巧拼配'])
})

test('buildOrderPayload includes selected bean list publication', () => {
  const payload = buildOrderPayload({
    form: {
      order_date: '2026-05-18',
      customer_id: 3,
      source_id: 1,
      order_type_id: 1,
      pay_status_id: 2,
      ship_status_id: 1,
      bean_list_publication_id: 88,
    },
    rows: [
      {
        product_id: 7,
        product_name: '曲奇拼配',
        tier_id: 'auto',
        spec_mode: '454',
        qty: 1,
        unit: '件',
        unit_price: 88,
      },
    ],
  })

  assert.equal(payload.bean_list_publication_id, 88)
})

test('buildOrderPayload includes selected bean list publications by product kind', () => {
  const payload = buildOrderPayload({
    form: {
      order_date: '2026-05-19',
      customer_id: 3,
      commercial_bean_list_publication_id: 81,
      green_bean_list_publication_id: 82,
      drip_bean_list_publication_id: 83,
    },
    rows: [
      {
        product_id: 88,
        product_name: '兰卡拼配生豆',
        product_kind: 'green_bean',
        tier_id: 'auto',
        spec_mode: '1000',
        qty: 1,
        unit: 'kg',
      },
    ],
  })

  assert.equal(payload.commercial_bean_list_publication_id, 81)
  assert.equal(payload.green_bean_list_publication_id, 82)
  assert.equal(payload.drip_bean_list_publication_id, 83)
})

test('order payment receipt fields are required only after actual collection', () => {
  const payStatuses = [
    { id: 1, name: '未付款' },
    { id: 2, name: '已付款' },
    { id: 3, name: '已收款' },
    { id: 4, name: '已支付' },
  ]

  assert.equal(requiresOrderPaymentMethod({ pay_status_id: 2 }, payStatuses), true)
  assert.equal(requiresOrderPaymentMethod({ pay_status_id: 3 }, payStatuses), true)
  assert.equal(requiresOrderPaymentReceipt({ pay_status_id: 2 }, payStatuses), false)
  assert.equal(requiresOrderPaymentReceipt({ pay_status_id: 3 }, payStatuses), true)
  assert.equal(requiresOrderPaymentReceipt({ pay_status_id: 4 }, payStatuses), false)
  assert.equal(requiresOrderPaymentReceipt({ pay_status_id: 1 }, payStatuses), false)
})

test('buildOrderPayload preserves green bean product kind for order pricing', () => {
  const payload = buildOrderPayload({
    form: {
      order_date: '2026-05-19',
      customer_id: 152,
      source_id: 1,
      order_type_id: 1,
      pay_status_id: 2,
      ship_status_id: 1,
    },
    rows: [
      {
        product_id: 414,
        product_name: '兰卡拼配生豆',
        product_kind: 'green_bean',
        tier_id: 'auto',
        spec_mode: '1000',
        qty: 30,
        unit: 'kg',
        unit_price: '',
      },
    ],
  })

  assert.equal(payload.product_kind[0], 'green_bean')
  assert.equal(payload.spec[0], '1000')
  assert.equal(payload.unit[0], 'kg')
})

test('normalizeSpecG rejects non-positive custom grams', () => {
  assert.equal(normalizeSpecG({ spec_mode: 'custom', custom_spec_g: 0 }), 0)
  assert.equal(normalizeSpecG({ spec_mode: 'custom', custom_spec_g: 300 }), 300)
})

test('wholesaleSpecOptions includes standard order-entry specs, product tiers, and custom grams', () => {
  const got = wholesaleSpecOptions({
    tiers: [
      { spec_g: 454, min: 1, unit_price: 88 },
      { spec_g: 2500, min: 1, unit_price: 388 },
    ],
  })

  assert.deepEqual(got.map((option) => option.value), ['36', '80', '100', '227', '454', '500', '1000', '2500', 'custom'])
  assert.equal(got.at(-1).label, '自定义克数')
  assert.equal(formatSpecLabel(2500), '2.5kg')
})

test('product kind helpers label green bean and roasted products distinctly', () => {
  assert.equal(productKindLabel({ product_kind: 'green_bean' }), '生豆')
  assert.equal(productKindLabel({ product_kind: 'roasted' }), '熟豆')
  assert.equal(productKindLabel({}), '熟豆')
  assert.equal(productKindBadgeClass({ product_kind: 'green_bean' }), 'kind-green')
  assert.equal(productKindBadgeClass({ product_kind: 'roasted' }), 'kind-roasted')
})

test('defaultWholesaleSpec uses the product first configured tier spec', () => {
  const got = defaultWholesaleSpec({
    tiers: [
      { id: 9, spec_g: 227, min: 1, unit_price: 49 },
      { id: 10, spec_g: 454, min: 1, unit_price: 88 },
    ],
  })

  assert.equal(got, '227')
})

test('order entry derives default order-unit spec from lightweight unit conversion', () => {
  const product = {
    order_unit: '盒',
    unit_conversion_json: '{"盒":{"kg":0.2}}',
    tiers: [{ spec_g: 1000 }],
  }

  const options = wholesaleSpecOptions(product)
  assert.equal(defaultWholesaleSpec(product), '200')
  assert.deepEqual(options[0], { label: '盒（200g）', value: '200', orderUnit: '盒' })
})

test('wholesaleTierPriceRows exposes every configured gradient price', () => {
  const got = wholesaleTierPriceRows({
    tiers: [
      { id: 1, spec_g: 227, min: 1, max: 7, unit_price: 49 },
      { id: 2, spec_g: 227, min: 8, max: null, unit_price: 42 },
      { id: 3, spec_g: 2500, min: 1, max: null, unit_price: 388 },
    ],
  })

  assert.deepEqual(got, [
    { id: '1', specG: 227, specLabel: '227g', rangeLabel: '1-7件', unitPrice: 98, priceUnit: { label: '元/磅', suffix: '/磅', unitG: 454 } },
    { id: '2', specG: 227, specLabel: '227g', rangeLabel: '8件+', unitPrice: 84, priceUnit: { label: '元/磅', suffix: '/磅', unitG: 454 } },
    { id: '3', specG: 2500, specLabel: '2.5kg', rangeLabel: '1kg+', unitPrice: 155, priceUnit: { label: '元/kg', suffix: '/kg', unitG: 1000 } },
  ])
})

test('wholesaleTierPriceRows keeps kg ranges while showing published lb unit prices', () => {
  const got = wholesaleTierPriceRows({
    tiers: [
      { id: 50, spec_g: 1000, min: 1, max: 59, unit_price: 23.49, display_unit: 'lb' },
      { id: 51, spec_g: 1000, min: 60, max: null, unit_price: 23.49, display_unit: 'lb' },
    ],
  }, { spec_mode: '80', qty: 1 })

  assert.deepEqual(got, [
    { id: '50', specG: 1000, specLabel: '1000g', rangeLabel: '1-59kg', unitPrice: 23.49, priceUnit: { label: '元/磅', suffix: '/磅', unitG: 454 } },
    { id: '51', specG: 1000, specLabel: '1000g', rangeLabel: '60kg+', unitPrice: 23.49, priceUnit: { label: '元/磅', suffix: '/磅', unitG: 454 } },
  ])
})

test('wholesaleTierPriceRows keeps custom quote units from product price list snapshots', () => {
  const got = wholesaleTierPriceRows({
    tiers: [
      { id: 70, spec_g: 100, min: 10, max: null, unit_price: 15, display_unit: '盒' },
    ],
  })

  assert.deepEqual(got, [
    { id: '70', specG: 100, specLabel: '100g', rangeLabel: '10件+', unitPrice: 15, priceUnit: { label: '元/盒', suffix: '/盒', unitG: 100 } },
  ])
})

test('syncWholesaleTierPrice matches tier by selected spec and package quantity', () => {
  const row = { spec_mode: '454', qty: 12, tier_id: 'auto', unit_price: '' }
  const got = syncWholesaleTierPrice({
    tiers: [
      { id: 1, spec_g: 454, min: 1, max: 9, unit_price: 99 },
      { id: 2, spec_g: 454, min: 10, max: null, unit_price: 86 },
      { id: 3, spec_g: 1000, min: 1, max: null, unit_price: 170 },
    ],
  }, row)

  assert.deepEqual(got, { tierID: '2', unitPrice: '86' })
})

test('syncWholesaleTierPrice falls back to bean-list weight tiers when selected spec has no exact tier', () => {
  const row = { spec_mode: '1000', qty: 30, tier_id: 'auto', unit_price: '' }
  const got = syncWholesaleTierPrice({
    tiers: [
      { id: 1, spec_g: 454, min: 2, max: 13, unit_price: 63 },
      { id: 2, spec_g: 454, min: 14, max: 23, unit_price: 57 },
      { id: 3, spec_g: 454, min: 24, max: 48, unit_price: 51 },
      { id: 4, spec_g: 454, min: 49, max: null, unit_price: 48 },
    ],
  }, row)

  assert.deepEqual(got, { tierID: '4', unitPrice: '106' })
  assert.deepEqual(wholesalePriceUnit(row), { label: '元/kg', suffix: '/kg', unitG: 1000 })
  assert.equal(lineTotal({ tiers: [] }, { ...row, tier_id: got.tierID, unit_price: got.unitPrice }, false), 106 * 30)
})

test('isOrderTierActive highlights fallback wholesale tier by matched id when selected spec differs', () => {
  assert.equal(typeof orderEntry.isOrderTierActive, 'function')
  const row = { spec_mode: '1000', qty: 20, tier_id: '58', unit_price: '117' }

  assert.equal(orderEntry.isOrderTierActive(row, { id: '58', specG: 454 }), true)
  assert.equal(orderEntry.isOrderTierActive(row, { id: '57', specG: 454 }), false)
  assert.equal(orderEntry.isOrderTierActive({ ...row, tier_id: 'manual' }, { id: '58', specG: 454 }), false)
  assert.equal(orderEntry.isOrderTierActive({ ...row, tier_id: 'auto' }, { id: '58', specG: 454 }), false)
})

test('syncWholesaleTierPrice matches kg-priced exact specs by total kg instead of package count', () => {
  const row = { spec_mode: '2500', qty: 10, tier_id: 'auto', unit_price: '' }
  const got = syncWholesaleTierPrice({
    tiers: [
      { id: 1, spec_g: 2500, min: 1, max: 23, unit_price: 550 },
      { id: 2, spec_g: 2500, min: 24, max: 49, unit_price: 512.5 },
      { id: 3, spec_g: 2500, min: 50, max: null, unit_price: 475 },
    ],
  }, row)

  assert.deepEqual(got, { tierID: '2', unitPrice: '205' })
  assert.equal(lineTotal({ tiers: [] }, { ...row, tier_id: got.tierID, unit_price: got.unitPrice }, false), 205 * 25)
})

test('syncWholesaleTierPrice preserves kg display-unit prices for kg rows', () => {
  const row = { spec_mode: '1000', qty: 10, tier_id: 'auto', unit_price: '' }
  const got = syncWholesaleTierPrice({
    tiers: [
      { id: 50, spec_g: 1000, min: 1, max: 59, unit_price: 23.49, display_unit: 'kg' },
    ],
  }, row)

  assert.deepEqual(got, { tierID: '50', unitPrice: '23.49' })
  assert.equal(Number(lineTotal({ tiers: [] }, { ...row, tier_id: got.tierID, unit_price: got.unitPrice }, false).toFixed(2)), 234.9)
})

test('syncWholesaleTierPrice converts kg display-unit prices for lb rows', () => {
  const row = { spec_mode: '80', qty: 13, tier_id: 'auto', unit_price: '' }
  const got = syncWholesaleTierPrice({
    tiers: [
      { id: 50, spec_g: 1000, min: 1, max: 59, unit_price: 23.49, display_unit: 'kg' },
    ],
  }, row)

  assert.deepEqual(got, { tierID: '50', unitPrice: '23.49' })
})

test('resolveWholesaleTierPrice keeps kg tier unit and source version for small packages inside the published range', () => {
  const row = { spec_mode: '80', qty: 313, tier_id: 'auto', unit_price: '' }
  const got = resolveWholesaleTierPrice({
    tiers: [
      {
        id: 64,
        spec_g: 1000,
        min: 25,
        max: 49,
        unit_price: 82,
        display_unit: 'kg',
        price_source_json: '{"source":"published_bean_list","list_type":"commercial","publication_id":9909,"version_no":"V3.0.9","price_unit":"kg"}',
      },
    ],
  }, row)

  assert.equal(got.tierID, '64')
  assert.equal(got.unitPrice, '82')
  assert.deepEqual(got.priceUnit, { label: '元/kg', suffix: '/kg', unitG: 1000 })
  assert.equal(got.tierPriceLabel, '82/kg')
  assert.equal(got.beanListPublicationID, 9909)
  assert.equal(got.beanListVersionNo, 'V3.0.9')
  assert.equal(got.belowMinTier, false)
  assert.equal(got.priceMissing, false)

  const pricedRow = {
    ...row,
    tier_id: got.tierID,
    unit_price: got.unitPrice,
    price_unit: got.priceUnit.label,
    price_unit_suffix: got.priceUnit.suffix,
    price_unit_g: got.priceUnit.unitG,
  }
  assert.deepEqual(orderRowPriceUnit(pricedRow), { label: '元/kg', suffix: '/kg', unitG: 1000 })
  assert.equal(Number(lineTotal({ tiers: [] }, pricedRow, false).toFixed(2)), 2053.28)
})

test('resolveWholesaleTierPrice leaves price blank below minimum, above finite maximum, and in tier gaps', () => {
  const product = {
    tiers: [
      {
        id: 64,
        spec_g: 1000,
        min: 25,
        max: 49,
        unit_price: 82,
        display_unit: 'kg',
        price_source_json: '{"source":"published_bean_list","list_type":"commercial","publication_id":9909,"version_no":"V3.0.9","price_unit":"kg"}',
      },
      {
        id: 65,
        spec_g: 1000,
        min: 51,
        max: 60,
        unit_price: 78,
        display_unit: 'kg',
        price_source_json: '{"source":"published_bean_list","list_type":"commercial","publication_id":9909,"version_no":"V3.0.9","price_unit":"kg"}',
      },
    ],
  }

  for (const qty of [1, 50, 61]) {
    const got = resolveWholesaleTierPrice(product, { spec_mode: '1000', qty, tier_id: 'auto', unit_price: '' })
    assert.equal(got.tierID, 'auto')
    assert.equal(got.unitPrice, '')
    assert.equal(got.priceMissing, true)
    assert.equal(got.belowMinTier, false)
  }
})

test('OrderEntryView shows explicit missing published price and blocks save without a manual override', () => {
  const source = orderEntryViewSource()

  assert.match(source, /tier_price_label/)
  assert.match(source, /当前数量无已发布价格，不能保存/)
  assert.match(source, /row\.price_missing/)
  assert.match(source, /hasUnpricedPublishedRow/)
  assert.match(source, /rowHasBlockingPrice/)
  assert.match(source, /手动价必须大于0，不能保存/)
  assert.match(source, /missingPublishedPriceRowIndex/)
  assert.match(source, /repriceHydratedRows/)
  assert.match(source, /const draftRestored = restoreOrderEntryDraft\(\)/)
  assert.match(source, /if \(draftRestored\) \{\s*syncBeanListVersionForCustomer\(\{ force: true \}\)\s*syncRowsForType\(\{ priceListChanged: true \}\)\s*\} else \{\s*repriceHydratedRows\(\)\s*\}/)
  assert.match(source, /function selectTier[\s\S]*?isDripRow[\s\S]*?syncPrice\(row, \{ force: true \}\)/)
  assert.match(source, /const publishedPrice = resolveWholesaleTierPrice\(product, row\)/)
  assert.match(source, /publishedPrice\.quantityBasis === 'sales_spec_count'[\s\S]*?applyResolvedWholesalePrice\(row, publishedPrice\)/)
  assert.match(source, /price_source_json: item\.price_source_json \|\| ''/)
  assert.match(source, /manual_price: item\.price_override === true \|\| item\.tier_id === 'manual'/)
  assert.match(source, /if \(retailOrder\.value\) return listType === 'retail' \|\| listType === 'drip'/)
  assert.match(source, /currentOrderBeanListTypeForProductKind/)
  assert.match(source, /报价来源：价格表/)
  assert.doesNotMatch(source, /豆单版本：\{\{\s*row\.bean_list_version_no\s*\|\|\s*'未记录'\s*\}\}/)
})

test('OrdersView detail shows read-only quote source and production source trace blocks', () => {
  const source = ordersViewSource()
  for (const expected of [
    '报价来源',
    '生产来源',
    'quote_source_trace',
    'production_source_trace',
    'orderTraceLineLabel',
    'orderTraceSourceLines',
  ]) {
    assert.match(source, new RegExp(expected))
  }
})

test('SalesOrderView omits the sales order trace panel and its detail request', () => {
  const source = salesOrderViewSource()
  for (const forbidden of [
    '销售单追溯',
    '刷新追溯',
    'sales-trace-panel',
    'quote_source_trace',
    'production_source_trace',
    'salesOrderTraceLineLabel',
    'salesOrderTraceLines',
    'loadSalesOrderTrace',
    `/api/orders/\${orderID.value}/detail`,
  ]) {
    assert.doesNotMatch(source, new RegExp(forbidden))
  }
})

test('rowUsesStaleBeanListPublication flags product rows whose publication is not the latest version', () => {
  const options = [
    { id: 31, list_type: 'commercial', version_no: 'V3.0.6', is_default: false },
    { id: 33, list_type: 'commercial', version_no: 'V3.0.9', is_default: true },
    { id: 41, list_type: 'green', version_no: 'V2.0.1', is_default: true },
  ]

  assert.equal(rowUsesStaleBeanListPublication({ product_id: 7, product_kind: 'roasted_bean', bean_list_publication_id: 31 }, options), true)
  assert.equal(rowUsesStaleBeanListPublication({ product_id: 7, product_kind: 'roasted_bean', bean_list_publication_id: 33 }, options), false)
  assert.equal(rowUsesStaleBeanListPublication({ product_id: 8, product_kind: 'green_bean', bean_list_publication_id: 41 }, options), false)
  assert.equal(rowUsesStaleBeanListPublication({ product_id: 0, bean_list_publication_id: 31 }, options), false)
})

test('rowUsesStaleBeanListPublication compares versions inside the same classification group', () => {
  const options = [
    {
      id: 91,
      list_type: 'commercial',
      classification_template_id: 221,
      classification_template_name: '熟豆',
      version_no: 'V3.0.19',
      published_at: '2026-07-22 10:00',
    },
    {
      id: 92,
      list_type: 'commercial',
      classification_template_id: 2,
      classification_template_name: '挂耳',
      version_no: 'V3.0.20',
      published_at: '2026-07-22 11:00',
    },
  ]

  const row = {
    product_id: 7,
    product_kind: 'roasted_bean',
    bean_list_publication_id: 91,
  }
  assert.equal(rowUsesStaleBeanListPublication(row, options), false)

  options.push({
    id: 94,
    list_type: 'commercial',
    classification_template_id: 221,
    classification_template_name: '熟豆',
    version_no: 'V3.0.21',
    published_at: '2026-07-22 12:00',
  })
  assert.equal(rowUsesStaleBeanListPublication(row, options), true)
})

test('latestBeanListVersionOption uses the newest published version instead of default flag alone', () => {
  const options = [
    { id: 31, list_type: 'commercial', version_no: 'V3.0.5', published_at: '2026-05-18 18:51', is_default: true },
    { id: 33, list_type: 'commercial', version_no: 'V3.0.9', published_at: '2026-05-22 20:47', is_default: false },
    { id: 41, list_type: 'green', version_no: 'V2.0.1', published_at: '2026-05-19 10:00', is_default: true },
  ]

  assert.equal(latestBeanListVersionOption(options, 'commercial').id, 33)
  assert.equal(rowUsesStaleBeanListPublication({ product_id: 7, product_kind: 'roasted_bean', bean_list_publication_id: 31 }, options), true)
})

test('latestProductPriceListVersionOption prefers product type category over legacy list type', () => {
  const options = [
    { id: 31, list_type: 'commercial', product_type_category_id: 0, product_type_name: '熟豆', version_no: 'V3.0.9', published_at: '2026-05-22 20:47' },
    { id: 51, list_type: 'instant', product_type_category_id: 12, product_type_name: '速溶咖啡', version_no: 'V1.0.0', published_at: '2026-05-23 08:00' },
    { id: 52, list_type: 'instant', product_type_category_id: 12, product_type_name: '速溶咖啡', version_no: 'V1.0.1', published_at: '2026-05-24 08:00' },
  ]

  assert.equal(latestProductPriceListVersionOption(options, { product_type_category_id: 12 })?.id, 52)
  assert.equal(latestProductPriceListVersionOption(options, { product_type_name: '速溶咖啡' })?.id, 52)
  assert.equal(latestProductPriceListVersionOption(options, { product_kind: 'roasted_bean' })?.id, 31)
})

test('beanListVersionOptionGroups groups price lists by custom product category before legacy type', () => {
  const groups = beanListVersionOptionGroups([
    { id: 1, list_type: 'commercial', product_type_category_id: 9, product_type_name: '冷萃类', version_no: 'v1' },
    { id: 2, list_type: 'commercial', product_type_category_id: 9, product_type_name: '冷萃类', version_no: 'v2' },
    { id: 3, list_type: 'green', version_no: 'g1' },
  ])

  assert.deepEqual(groups.map((group) => [group.key, group.label, group.listType, group.options.map((item) => item.id)]), [
    ['classification:9:commercial', '冷萃类', 'commercial', [1, 2]],
    ['legacy:green', '生豆豆单', 'green', [3]],
  ])
})

test('价格表分组使用分类模板 ID，不因价格表名称变化拆组', () => {
  const groups = beanListVersionOptionGroups([
    {
      id: 91,
      list_type: 'commercial',
      classification_template_id: 221,
      classification_template_name: '熟豆',
      product_type_name: '咖啡豆',
      version_no: 'V3.0.19',
    },
    {
      id: 94,
      list_type: 'commercial',
      classification_template_id: 221,
      classification_template_name: '熟豆',
      product_type_name: '咖啡熟豆新名称',
      version_no: 'V3.0.21',
    },
  ])

  assert.equal(groups.length, 1)
  assert.equal(groups[0].key, 'classification:221:commercial')
  assert.deepEqual(groups[0].options.map((item) => item.id), [91, 94])
})

test('录单仅自动启用权威分类价格表，旧分类名价格表保持显式可选', () => {
  const groups = beanListVersionOptionGroups([
    {
      id: 90,
      list_type: 'commercial',
      product_type_category_id: 0,
      product_type_name: 'KMM商品供应售价',
      version_no: 'V3.0.18',
      published_at: '2026-07-20 09:00',
      is_default: true,
    },
    {
      id: 91,
      list_type: 'commercial',
      product_type_category_id: 0,
      product_type_name: '咖啡豆',
      classification_template_id: 221,
      classification_template_name: '熟豆',
      version_no: 'V3.0.19',
      published_at: '2026-07-22 14:43',
      is_default: true,
    },
    {
      id: 92,
      list_type: 'commercial',
      product_type_category_id: 0,
      product_type_name: '挂耳咖啡',
      classification_template_id: 2,
      classification_template_name: '挂耳',
      version_no: 'V3.0.20',
      published_at: '2026-07-22 15:10',
      is_default: true,
    },
  ])

  assert.deepEqual(groups.map((group) => [group.key, group.label, group.autoSelect]), [
    ['legacy:commercial', 'KMM商品供应售价', false],
    ['classification:221:commercial', '熟豆', true],
    ['classification:2:commercial', '挂耳', true],
  ])
  assert.equal(beanListVersionOptionForGroup(groups[0], 0), null)
  assert.deepEqual(activeBeanListPublicationIDsByType(groups, {}), {
    commercial: [91, 92],
  })
  assert.deepEqual(activeBeanListPublicationIDsByType(groups, {
    [groups[0].key]: 90,
  }), {
    commercial: [90, 91, 92],
  })

  const products = [
    { id: 884, tiers: [{ publication_id: 91, list_type: 'commercial' }] },
    { id: 986, tiers: [{ publication_id: 91, list_type: 'commercial' }] },
    { id: 551, tiers: [{ publication_id: 90, list_type: 'commercial' }] },
  ]
  const activeIDs = activeBeanListPublicationIDsByType(groups, {})
  assert.deepEqual(filterProductsForCustomer(products, 0, activeIDs).map((item) => item.id), [884, 986])
})

test('V3.0.19 熟豆目录按父商品聚合后只保留四款，不混入 V3.0.18 多规格商品', () => {
  const currentNames = ['白月光瑰夏', '风味孟连', '果皮茶', '黑巧炸弹']
  const currentFamilies = currentNames.map((name, index) => ({
    parent_product_id: 800 + index,
    parent_product_name: name,
    name,
    product_kind: 'roasted_bean',
    specs: [{
      sku_id: 900 + index,
      sku_name: `${name} SKU`,
      spec_label: '227g',
      tiers: [{
        id: 1000 + index,
        publication_id: 91,
        list_type: 'commercial',
        quantity_basis: 'sales_spec_count',
        sku_id: 900 + index,
        unit_price: 60 + index,
      }],
    }],
  }))
  const legacyProducts = [
    { id: 701, parent_product_id: 70, parent_product_name: '旧拼配', name: '旧拼配 227g', product_kind: 'roasted_bean', tiers: [{ publication_id: 90, list_type: 'commercial', unit_price: 55 }] },
    { id: 702, parent_product_id: 70, parent_product_name: '旧拼配', name: '旧拼配 454g', product_kind: 'roasted_bean', tiers: [{ publication_id: 90, list_type: 'commercial', unit_price: 105 }] },
  ]
  const families = normalizeOrderProductFamilies(currentFamilies, legacyProducts)
  const scoped = filterProductsForCustomer(families, 0, { commercial: [91] })

  assert.deepEqual(scoped.map((item) => item.name), currentNames)
  assert.equal(scoped.length, 4)
  assert.equal(scoped.some((item) => item.parent_product_name === '旧拼配'), false)
})

test('纯历史价格表环境继续自动启用原有分组', () => {
  const groups = beanListVersionOptionGroups([
    { id: 80, list_type: 'commercial', product_type_name: '历史熟豆', version_no: 'V2.0.0' },
    { id: 81, list_type: 'commercial', product_type_name: '历史挂耳', version_no: 'V2.0.1' },
  ])

  assert.deepEqual(groups.map((group) => group.autoSelect), [true])
  assert.deepEqual(activeBeanListPublicationIDsByType(groups, {}), { commercial: [81] })
})

test('历史编辑精确恢复冻结发布，复制订单改按当前分类发布校验', () => {
  const groups = beanListVersionOptionGroups([
    { id: 90, list_type: 'commercial', version_no: 'V3.0.18', is_default: false },
    {
      id: 91,
      list_type: 'commercial',
      classification_template_id: 221,
      classification_template_name: '熟豆',
      version_no: 'V3.0.19',
      is_default: true,
    },
  ])
  const legacyGroup = beanListVersionGroupForPublicationID(groups, 90)
  const currentGroup = beanListVersionGroupForPublicationID(groups, 91)
  const overlappingFamily = {
    list_type: 'commercial',
    tiers: [
      { publication_id: 90, sku_id: 7001 },
      { publication_id: 91, sku_id: 7001 },
    ],
  }

  assert.equal(legacyGroup?.key, 'legacy:commercial')
  assert.equal(currentGroup?.key, 'classification:221:commercial')
  assert.equal(beanListVersionOptionForProductGroups(groups, {
    [legacyGroup.key]: 90,
    [currentGroup.key]: 91,
  }, overlappingFamily, 90)?.id, 90)
  assert.equal(beanListVersionOptionForProductGroups(groups, {
    [legacyGroup.key]: 0,
    [currentGroup.key]: 91,
  }, overlappingFamily, 90)?.id, 91)
})

test('客户当前分类替换旧公共版本时，历史编辑保留冻结发布而复制订单使用当前版本', () => {
  const groups = beanListVersionOptionGroups([{
    id: 9951,
    customer_id: 3,
    list_type: 'commercial',
    classification_template_id: 221,
    classification_template_name: '熟豆',
    version_no: 'CUSTOMER-CURRENT',
    is_customer_owned: true,
    is_default: true,
  }])

  assert.equal(shouldKeepFrozenOrderPublication(groups, 9952, false), true)
  assert.equal(shouldKeepFrozenOrderPublication(groups, 9952, true), false)
  assert.equal(shouldKeepFrozenOrderPublication(groups, 9951, false), false)

  const family = {
    parent_product_id: 700,
    parent_product_name: '重叠规格商品',
    name: '重叠规格商品',
  }
  const oldSpec = {
    sku_id: 7001,
    sku_name: 'SKU-7001',
    spec_label: '227g',
    tiers: [{ publication_id: 9952, unit_price: 68 }],
  }
  const hydrated = orderEntry.orderFamilyHydratedSpecRowPatch(
    family,
    oldSpec,
    9952,
    {
      unit_price: '68',
      price_source_json: '{"publication_id":9952}',
      bean_list_publication_id: 9952,
    },
    true,
  )
  assert.equal(hydrated.bean_list_publication_id, 9952)
  assert.equal(hydrated.unit_price, '68')
  assert.equal(hydrated.historical_spec_readonly, true)
})

test('订单价格表分组不合并同分类的商用和零售发布', () => {
  const groups = beanListVersionOptionGroups([
    { id: 11, list_type: 'commercial', product_type_category_id: 9, product_type_name: '咖啡熟豆' },
    { id: 22, list_type: 'retail', product_type_category_id: 9, product_type_name: '咖啡熟豆' },
  ])
  assert.deepEqual(groups.map((group) => [group.key, group.listType, group.options.map((item) => item.id)]), [
    ['classification:9:commercial', 'commercial', [11]],
    ['classification:9:retail', 'retail', [22]],
  ])
})

test('stored cutover order resolves its priced BOM spec by bom_spec_id instead of parent product_id', () => {
  const family = normalizeOrderProductFamilies([{
    parent_product_id: 700,
    parent_product_name: '规格商品',
    migration_state: 'cutover',
    bom_specs: [
      { bom_spec_id: 91, bom_variant_id: 191, spec_label: '227g 袋', inventory_unit: '袋', is_default_sku: true, tiers: [{ publication_id: 99, unit_price: 48 }] },
      { bom_spec_id: 92, bom_variant_id: 192, spec_label: '454g 袋', inventory_unit: '袋', tiers: [{ publication_id: 99, unit_price: 88 }] },
    ],
  }], [])[0]

  const selected = orderEntry.orderFamilySpecForStoredItem(family, {
    product_id: 700,
    parent_product_id: 700,
    bom_spec_id: 92,
    bom_variant_id: 192,
  }, 99)

  assert.equal(selected?.bom_spec_id, 92)
  assert.equal(selected?.bom_variant_id, 192)
})

test('beanListVersionOptionsForCustomer keeps public fallback versions for the selected customer', () => {
  const options = [
    { customer_id: 74, list_type: 'commercial', id: 31, version_no: 'V3.0.6', is_customer_owned: false, is_default: false },
    { customer_id: 74, list_type: 'commercial', id: 33, version_no: 'V3.0.9', is_customer_owned: false, is_default: true },
    { customer_id: 152, list_type: 'commercial', id: 41, version_no: 'V2.0.1', is_customer_owned: true, is_default: true },
  ]

  assert.deepEqual(
    beanListVersionOptionsForCustomer(options, 74).map((item) => [item.id, item.version_no, item.is_default]),
    [
      [31, 'V3.0.6', false],
      [33, 'V3.0.9', true],
    ],
  )
})

test('beanListVersionOptionsForCustomer exposes public published versions before a customer is selected', () => {
  const options = [
    { customer_id: 0, list_type: 'commercial', id: 45, version_no: 'V3.0.14', is_customer_owned: false, is_default: true },
    { customer_id: 0, list_type: 'commercial', id: 44, version_no: 'V3.0.13', is_customer_owned: false, is_default: false },
    { customer_id: 0, list_type: 'green', id: 25, version_no: 'V3.0.5', is_customer_owned: false, is_default: true },
    { customer_id: 74, list_type: 'commercial', id: 31, version_no: 'V3.0.6', is_customer_owned: true, is_default: true },
  ]

  assert.deepEqual(
    beanListVersionOptionsForCustomer(options, 0).map((item) => [item.id, item.list_type, item.version_no, item.is_default]),
    [
      [45, 'commercial', 'V3.0.14', true],
      [44, 'commercial', 'V3.0.13', false],
      [25, 'green', 'V3.0.5', true],
    ],
  )
})

test('beanListVersionOptionsForCustomer deduplicates repeated public fallbacks for no-customer order entry', () => {
  const options = [
    { customer_id: 2, list_type: 'commercial', id: 45, version_no: 'V3.0.14', is_customer_owned: false, is_default: true },
    { customer_id: 3, list_type: 'commercial', id: 45, version_no: 'V3.0.14', is_customer_owned: false, is_default: true },
    { customer_id: 2, list_type: 'commercial', id: 44, version_no: 'V3.0.13', is_customer_owned: false, is_default: false },
    { customer_id: 74, list_type: 'commercial', id: 31, version_no: 'V3.0.6', is_customer_owned: true, is_default: true },
  ]

  assert.deepEqual(
    beanListVersionOptionsForCustomer(options, 0).map((item) => item.id),
    [45, 44],
  )
})

test('resolveWholesaleTierPrice and tier rows use the selected bean list publication when product carries multiple versions', () => {
  const multiVersionProduct = {
    tiers: [
      {
        id: 57,
        spec_g: 454,
        min: 2,
        max: 13,
        unit_price: 70,
        display_unit: 'lb',
        price_source_json: '{"source":"published_bean_list","list_type":"commercial","publication_id":33,"version_no":"V3.0.9"}',
      },
      {
        id: 56,
        spec_g: 454,
        min: 2,
        max: 13,
        unit_price: 65,
        display_unit: 'lb',
        price_source_json: '{"source":"published_bean_list","list_type":"commercial","publication_id":31,"version_no":"V3.0.6"}',
      },
    ],
  }
  const row = { spec_mode: '454', qty: 2, bean_list_publication_id: 31 }

  const price = resolveWholesaleTierPrice(multiVersionProduct, row)
  assert.equal(price.unitPrice, '65')
  assert.equal(price.tierID, '56')
  assert.equal(price.beanListPublicationID, 31)
  assert.equal(price.beanListVersionNo, 'V3.0.6')
  assert.deepEqual(wholesaleTierPriceRows(multiVersionProduct, row).map((item) => item.unitPrice), [65])
})

test('needsTrailingBlankOrderLine only requests one empty detail row after product selection', () => {
  assert.equal(needsTrailingBlankOrderLine([
    { product_id: 7, product_query: '兰卡拼配', item_note: '' },
  ]), true)
  assert.equal(needsTrailingBlankOrderLine([
    { product_id: 7, product_query: '兰卡拼配', item_note: '' },
    { product_id: 0, product_query: '', item_note: '', unit_price: '' },
  ]), false)
  assert.equal(needsTrailingBlankOrderLine([
    { product_id: 7, product_query: '兰卡拼配', item_note: '' },
    { product_id: 0, product_query: '正在搜索', item_note: '', unit_price: '' },
  ]), true)
})

test('OrderEntryView puts add detail below the list and renders stale price-list warning icon', () => {
  const source = orderEntryViewSource()
  const headerBlock = source.slice(source.indexOf('<section class="panel"'), source.indexOf('<div class="line-list">'))
  assert.doesNotMatch(headerBlock, /新增明细/)
  assert.ok(source.indexOf('class="line-actions"') > source.indexOf('<div class="line-list">'))
  assert.match(source, /rowUsesStaleBeanListPublication/)
  assert.match(source, /非最新价格表/)
  assert.match(source, /bean-list-version-warning/)
})

test('OrderEntryView shows selected bean lists as readable rows and refreshes row versions from selection', () => {
  const source = orderEntryViewSource()
  const lineSection = source.slice(source.indexOf('<section class="panel" :class'), source.indexOf('<div class="line-list">'))
  const syncPriceBlock = source.slice(source.indexOf('function syncPrice(row'), source.indexOf('function clearWholesalePriceMetadata'))

  assert.match(lineSection, /selectedBeanListSummaryItems/)
  assert.match(lineSection, /bean-list-summary-list/)
  assert.match(lineSection, /v-for="item in selectedBeanListSummaryItems"/)
  assert.doesNotMatch(lineSection, /\{\{\s*selectedBeanListSummary\s*\}\}/)
  assert.match(source, /function syncRowBeanListVersionFromSelection\(row\)/)
  assert.match(syncPriceBlock, /syncRowBeanListVersionFromSelection\(row\)[\s\S]*resolveWholesaleTierPrice\(product,\s*row\)/)
  assert.match(lineSection, /:disabled="!canOpenBeanListDrawer"/)
  assert.doesNotMatch(lineSection, /:disabled="!form\.customer_id"/)
  assert.match(source, /const canOpenBeanListDrawer = computed/)
  assert.match(source, /不使用该历史价格表/)

  const summaryListStyles = cssBlock(source, '.bean-list-summary-list')
  const summaryStyles = cssBlock(source, '.bean-list-summary')
  assert.match(summaryListStyles, /display:\s*grid/)
  assert.doesNotMatch(summaryStyles, /text-overflow:\s*ellipsis/)
  assert.doesNotMatch(summaryStyles, /white-space:\s*nowrap/)
})

test('buildOrderPayload preserves manual unit price override', () => {
  const payload = buildOrderPayload({
    form: {
      order_date: '2026-04-27',
      customer_id: 3,
      source_id: 1,
      order_type_id: 1,
      pay_status_id: 2,
      ship_status_id: 1,
    },
    rows: [
      {
        product_id: 7,
        product_name: '橘皮乌龙',
        tier_id: 'manual',
        spec_mode: '454',
        qty: 3,
        unit: '件',
        unit_price: '92',
      },
    ],
  })

  assert.equal(payload.tier_id[0], 'manual')
  assert.equal(payload.unit_price[0], '92')
})

test('buildOrderPayload leaves order responsible person to customer profile defaults', () => {
  const payload = buildOrderPayload({
    form: {
      order_date: '2026-05-06',
      customer_id: 3,
      source_id: 1,
      order_type_id: 1,
      pay_status_id: 2,
      ship_status_id: 1,
      responsible_type: 'employee',
      responsible_id: 8,
    },
    rows: [
      {
        product_id: 7,
        product_name: '橘皮乌龙',
        tier_id: 'manual',
        spec_mode: '454',
        qty: 1,
        unit: '件',
        unit_price: '88',
      },
    ],
  })

  assert.equal(Object.prototype.hasOwnProperty.call(payload, 'responsible_type'), false)
  assert.equal(Object.prototype.hasOwnProperty.call(payload, 'responsible_id'), false)
})

test('buildOrderPayload carries selected receipt method into order saves', () => {
  const payload = buildOrderPayload({
    form: {
      order_date: '2026-05-15',
      customer_id: 3,
      source_id: 1,
      order_type_id: 1,
      pay_status_id: 2,
      payment_method: ' 银行转账 ',
      ship_status_id: 1,
    },
    rows: [
      {
        product_id: 7,
        product_name: '橘皮乌龙',
        tier_id: 'manual',
        spec_mode: '454',
        qty: 1,
        unit: '件',
        unit_price: '88',
      },
    ],
  })

  assert.equal(payload.payment_method, '银行转账')
})

test('buildOrderPayload carries logistics and payment receipt fields', () => {
  const payload = buildOrderPayload({
    form: {
      order_date: '2026-05-23',
      customer_id: 3,
      source_id: 1,
      order_type_id: 1,
      pay_status_id: 3,
      payment_method: '微信支付',
      ship_status_id: 4,
      logistics_company_id: 9,
      logistics_product_id: 10,
      payment_goods_amount: '120.00',
      payment_shipping_amount: '8.00',
      payment_voucher_asset_id: 88,
    },
    rows: [
      {
        product_id: 7,
        product_name: '橘皮乌龙',
        tier_id: 'manual',
        spec_mode: '454',
        qty: 1,
        unit: '件',
        unit_price: '88',
      },
    ],
  })

  assert.equal(payload.logistics_company_id, 9)
  assert.equal(payload.logistics_product_id, 10)
  assert.equal(payload.payment_goods_amount, '120.00')
  assert.equal(payload.payment_shipping_amount, '8.00')
  assert.equal(payload.payment_voucher_asset_id, 88)
})

test('requiresOrderPaymentMethod only triggers for paid receipt statuses', () => {
  const payStatuses = [
    { id: 1, name: '未付款' },
    { id: 2, name: '已付款' },
    { id: 3, name: '已收款' },
  ]

  assert.equal(requiresOrderPaymentMethod({ pay_status_id: 1 }, payStatuses), false)
  assert.equal(requiresOrderPaymentMethod({ pay_status_id: 2 }, payStatuses), true)
  assert.equal(requiresOrderPaymentMethod({ pay_status_id: 3 }, payStatuses), true)
  assert.ok(orderReceiptMethodOptions.includes('银行转账'))
  assert.ok(orderReceiptMethodOptions.includes('微信支付'))
})

test('responsibleOptions only returns employee choices for customer ownership', () => {
  const got = responsibleOptions({
    employees: [
      { id: 8, name: '销售小王', department: '销售', phone: '13800000008' },
    ],
    customers: [
      { id: 3, name: '测试客户', contact: '门店老板', phone: '13800000003' },
    ],
  })

  assert.deepEqual(got, [
    { type: 'employee', id: 8, name: '销售小王', label: '员工 - 销售小王', meta: '销售 13800000008', search: '员工 销售小王 销售 13800000008' },
  ])
})

test('order entry raises the active combobox above following fields', () => {
  const source = orderEntryViewSource()

  assert.match(source, /<label[^>]*class="customer-combobox combobox"[^>]*:class="\{\s*open:\s*customerOpen\s*\}"[^>]*>/)
  assert.match(source, /<label\s+class="product-combobox combobox product-cell"\s+:class="\{\s*open:\s*row\.product_open\s*\}"\s+:data-product-combobox-key="row\.key"\s*>/)
  assert.match(source, /<span>客户负责人<\/span>/)
  assert.doesNotMatch(source, /responsible-combobox/)
  assert.doesNotMatch(source, /responsibleOpen/)
  assert.doesNotMatch(source, /<span>订单负责人<\/span>/)

  const baseZIndex = zIndexForSelector(source, '.combobox')
  const openZIndex = zIndexForSelector(source, '.combobox.open')
  const productZIndex = zIndexForSelector(source, '.product-cell')
  assert.ok(openZIndex > baseZIndex, `expected active combobox z-index ${openZIndex} to exceed base z-index ${baseZIndex}`)
  assert.ok(openZIndex > productZIndex, `expected active combobox z-index ${openZIndex} to exceed product cell z-index ${productZIndex}`)
})

test('order entry product dropdown exposes category filters and closes on outside pointerdown', () => {
  const source = orderEntryViewSource()

  assert.match(source, /:data-product-combobox-key="row\.key"/)
  assert.match(source, /class="product-kind-filter"[^>]*aria-label="商品分类"/)
  assert.match(source, /v-if="productKindFilterOptions\(row\)\.length > 1"/)
  assert.match(source, /v-for="option in productKindFilterOptions\(row\)"/)
  assert.match(source, /@mousedown\.prevent/)
  assert.match(source, /@click\.stop="row\.product_kind_filter = option\.value"/)
  assert.match(source, /@focus="openProductDropdown\(row\)"/)
  assert.match(source, /@keydown\.down\.prevent="openProductDropdown\(row\)"/)
  assert.match(source, /function openProductDropdown\(row\)[\s\S]*?closeOrderProductDropdowns\(rows\.value, row\?\.key\)[\s\S]*?row\.product_open = true/)
  assert.match(source, /document\.addEventListener\('pointerdown', handleOrderProductPointerDown\)/)
  assert.match(source, /document\.removeEventListener\('pointerdown', handleOrderProductPointerDown\)/)
  assert.match(source, /onBeforeUnmount\(saveOrderEntryDraft\)/)
})

test('order entry shows save errors in a fixed global alert', () => {
  const source = orderEntryViewSource()

  assert.match(source, /<div\s+v-if="error"\s+class="global-error-toast notice error"\s+role="alert">/)
  assert.match(source, /<button class="toast-close" type="button" aria-label="关闭错误提示" @click="error = ''">/)

  const toastStyles = cssBlock(source, '.global-error-toast')
  assert.match(toastStyles, /position:\s*fixed/)
  assert.match(toastStyles, /z-index:\s*80/)
})

test('order entry exposes selected customer edit drawer beside new customer', () => {
  const source = orderEntryViewSource()

  assert.match(source, /@click="openCustomerDrawer"[^>]*>新增客户<\/button>/)
  assert.match(source, /@click="openCustomerEditDrawer"[^>]*:disabled="!form\.customer_id"[^>]*>编辑客户<\/button>/s)
  assert.match(source, /<h3>\{\{ customerDrawerMode === 'edit' \? '编辑客户' : '新增客户' \}\}<\/h3>/)
  assert.match(source, /apiGet\(`\/api\/customers\/\$\{form\.customer_id\}`\)/)
  assert.match(source, /method: customerDrawerMode\.value === 'edit' \? 'PUT' : 'POST'/)
  assert.match(source, /apiSend\(customerDrawerMode\.value === 'edit' \? `\/api\/customers\/\$\{customerForm\.id\}` : '\/api\/customers'/)
  for (const field of ['company_name', 'company_address', 'company_phone', 'responsible_employee_id']) {
    assert.match(source, new RegExp(field))
  }
})

test('order entry shows customer defaults instead of editable source and order type controls', () => {
  const source = orderEntryViewSource()
  const orderInfoBlock = source.slice(source.indexOf('<section class="panel order-fields"'), source.indexOf('<section class="panel" :class'))

  assert.match(orderInfoBlock, /客户类型/)
  assert.match(orderInfoBlock, /来源/)
  assert.match(orderInfoBlock, /订单类型/)
  assert.match(orderInfoBlock, /selectedCustomerProfileSummary/)
  assert.doesNotMatch(orderInfoBlock, /v-model\.number="form\.source_id"/)
  assert.doesNotMatch(orderInfoBlock, /v-model\.number="form\.order_type_id"/)
})

test('order entry customer drawer requires customer type source and order type fields', () => {
  const source = orderEntryViewSource()
  const drawerStart = source.indexOf('<div v-if="customerDrawerOpen"')
  const drawerBlock = source.slice(drawerStart, source.indexOf('</aside>', drawerStart))

  assert.match(drawerBlock, /客户类型/)
  assert.match(drawerBlock, /v-model="customerForm\.customer_type"/)
  assert.match(drawerBlock, /开通客户门户\/工作台/)
  assert.match(drawerBlock, /v-model="customerForm\.portal_enabled"/)
  assert.doesNotMatch(drawerBlock, /v-model="customerForm\.capability_template_key"/)
  assert.doesNotMatch(source, /defaultCapabilityTemplateForCustomerType/)
  assert.doesNotMatch(drawerBlock, /<option :value="0">未设置<\/option>/)
  assert.match(source, /请选择客户类型/)
  assert.match(source, /请选择客户来源/)
  assert.match(source, /请选择客户订单类型/)
  assert.doesNotMatch(source, /请选择能力模板/)
})

test('order entry moves price-list selection from order information to product details drawer', () => {
  const source = orderEntryViewSource()
  const orderInfoBlock = source.slice(source.indexOf('<section class="panel order-fields"'), source.indexOf('<section class="panel" :class'))
  const lineSection = source.slice(source.indexOf('<section class="panel" :class'))

  assert.doesNotMatch(orderInfoBlock, /showBeanListVersionPickerByType/)
  assert.match(lineSection, /选择价格表/)
  assert.match(source, /bean-list-drawer/)
  assert.match(source, /openBeanListDrawer/)
})

test('order entry save validation scrolls to invalid fields and marks them until corrected', () => {
  const source = orderEntryViewSource()

  assert.match(source, /const fieldErrors = reactive\(\{\}\)/)
  assert.match(source, /function raiseSaveError\(message,\s*fieldKey/)
  assert.match(source, /scrollIntoView\(\{\s*behavior:\s*'smooth',\s*block:\s*'center'\s*\}\)/)
  assert.match(source, /function clearFieldErrorIfValid\(fieldKey\)/)
  assert.match(source, /watch\(\(\) => form\.customer_id,\s*\(\) => clearFieldErrorIfValid\('customer_id'\)\)/)
  assert.match(source, /:class="\{ 'field-invalid': hasFieldError\('customer_id'\) \}"/)
  assert.match(source, /data-error-field="customer_id"/)
  assert.match(source, /data-error-field="payment_method"/)
  assert.match(source, /data-error-field="logistics_company_id"/)
  assert.match(source, /data-error-field="payment_voucher_asset_id"/)
  assert.match(source, /data-error-field="product_items"/)
  assert.match(source, /raiseSaveError\('请选择客户', 'customer_id'\)/)
  assert.match(source, /raiseSaveError\('请上传收款凭证', 'payment_voucher_asset_id'\)/)

  const invalidStyles = cssBlock(source, '.field-invalid input, .field-invalid select, .field-invalid textarea, .field-invalid .file-upload-control')
  assert.match(invalidStyles, /border-color:\s*#f43f5e/)
})

test('order entry shows clickable realtime payment amount suggestions without locking edits', () => {
  const source = orderEntryViewSource()

  assert.match(source, /const paymentGoodsAmountSuggestion = computed\(\(\) => money\(itemsTotal\.value\)\)/)
  assert.match(source, /const paymentShippingAmountSuggestion = computed\(\(\) => money\(toNumber\(form\.shipping_amount\)\)\)/)
  assert.match(source, /const showPaymentGoodsAmountSuggestion = computed/)
  assert.match(source, /const showPaymentShippingAmountSuggestion = computed/)
  assert.match(source, /function applyPaymentGoodsAmountSuggestion\(\)/)
  assert.match(source, /function applyPaymentShippingAmountSuggestion\(\)/)
  assert.match(source, /form\.payment_goods_amount = paymentGoodsAmountSuggestion\.value/)
  assert.match(source, /form\.payment_shipping_amount = paymentShippingAmountSuggestion\.value/)
  assert.match(source, /class="amount-suggestion-popover"/)
  assert.match(source, /@click="applyPaymentGoodsAmountSuggestion"/)
  assert.match(source, /@click="applyPaymentShippingAmountSuggestion"/)
  assert.match(source, /实时价格提示\s*货款\s*\{\{ paymentGoodsAmountSuggestion \}\}/)
  assert.match(source, /实时价格提示\s*运费\s*\{\{ paymentShippingAmountSuggestion \}\}/)
  assert.doesNotMatch(source, /form\.payment_goods_amount = money\(itemsTotal\.value\)/)

  const suggestionStyles = cssBlock(source, '.amount-suggestion-popover')
  assert.match(suggestionStyles, /position:\s*absolute/)
  assert.match(suggestionStyles, /z-index:\s*6/)
})

test('order entry mobile layout keeps conditional panels and errors inside the viewport', () => {
  const source = orderEntryViewSource()
  const mobileStyles = sourceAfter(source, '@media (max-width: 760px)')

  assert.match(mobileStyles, /\.conditional-panel\s*\{[^}]*grid-template-columns:\s*1fr/s)
  assert.match(mobileStyles, /\.global-error-toast\s*\{[^}]*left:\s*max\(12px,\s*env\(safe-area-inset-left\)\)/s)
  assert.match(mobileStyles, /\.global-error-toast\s*\{[^}]*right:\s*max\(12px,\s*env\(safe-area-inset-right\)\)/s)
  assert.match(mobileStyles, /\.global-error-toast\s*\{[^}]*width:\s*auto/s)
  assert.match(mobileStyles, /\.global-error-toast\s*\{[^}]*--notice-stack-offset:\s*var\(--kferp-notice-stack-space,\s*0px\)/s)
  assert.match(mobileStyles, /\.global-error-toast\s*\{[^}]*top:\s*calc\(max\(12px,\s*env\(safe-area-inset-top\)\)\s*\+\s*var\(--notice-stack-offset\)\)/s)
})

test('order entry payment voucher upload uses a mobile-safe file control', () => {
  const source = orderEntryViewSource()

  assert.match(source, /class="file-upload-control"/)
  assert.match(source, /class="file-button"/)
  assert.match(source, /class="file-name"/)

  const uploadStyles = cssBlock(source, '.file-upload-control')
  const fileNameStyles = cssBlock(source, '.file-name')
  assert.match(uploadStyles, /grid-template-columns:\s*auto\s+minmax\(0,\s*1fr\)/)
  assert.match(fileNameStyles, /text-overflow:\s*ellipsis/)
})

test('order entry total preview includes shipping and exposes goods/logistics hints', () => {
  assert.deepEqual(orderEntry.orderTotalPreview({
    itemsTotal: 2340,
    shippingAmount: '18.5',
    discountAmount: '10',
    roundToInt: false,
  }), {
    goodsAmount: 2330,
    logisticsAmount: 18.5,
    grandTotal: 2348.5,
  })

  const source = orderEntryViewSource()
  assert.match(source, /orderTotalPreview/)
  assert.match(source, /orderTotalHintText/)
  assert.match(source, /货款/)
  assert.match(source, /物流/)
})

test('order entry payment voucher collapses after upload and can open a large preview', () => {
  const source = orderEntryViewSource()

  assert.match(source, /paymentVoucherCollapsed/)
  assert.match(source, /voucher-preview-overlay/)
  assert.match(source, /openPaymentVoucherPreview/)
  assert.match(source, /paymentVoucherPreviewOpen/)
  assert.match(source, /paymentVoucherImageURL/)
})

test('lineTotal uses manual unit price even for retail rows', () => {
  assert.equal(lineTotal(product, {
    tier_id: 'manual',
    spec_mode: '227',
    qty: 2,
    unit_price: '45',
  }, true), 90)
})

test('lineTotal uses manual wholesale unit price in the selected spec display unit', () => {
  assert.equal(lineTotal(product, {
    tier_id: 'manual',
    spec_mode: '1000',
    qty: 30,
    unit_price: '106',
  }, false), 106 * 30)
})

test('lineTotal applies per-row discount amount percent free and unit amount modes', () => {
	const row = {
		tier_id: 'manual',
		spec_mode: '454',
		qty: 2,
		unit_price: '88',
	}

	assert.equal(lineTotal(product, { ...row, discount_type: 'amount', discount_value: '16' }, false), 160)
	assert.equal(lineTotal(product, { ...row, discount_type: 'percent', discount_value: '50' }, false), 88)
	assert.equal(lineTotal(product, { ...row, discount_type: 'free' }, false), 0)
	assert.equal(lineTotal(product, { ...row, discount_type: 'unit_amount', discount_value: '10' }, false), 156)
	assert.equal(lineDiscountAmount(176, { discount_type: 'amount', discount_value: '300' }), 176)
})

test('lineTotal applies unit amount discount by the active price unit', () => {
	assert.equal(lineTotal(product, {
		tier_id: 'manual',
		spec_mode: '1000',
		qty: 30,
		unit_price: '106',
		discount_type: 'unit_amount',
		discount_value: '6',
	}, false), 3000)
})

test('buildOrderPayload carries per-item discount fields', () => {
	const payload = buildOrderPayload({
		form: {
      order_date: '2026-05-15',
      customer_id: 3,
      source_id: 1,
      order_type_id: 1,
      pay_status_id: 2,
      ship_status_id: 1,
      shipping_amount: 0,
      discount_amount: 0,
      round_to_int: false,
    },
    rows: [
      {
        product_id: 7,
        product_name: '橘皮乌龙',
        tier_id: 'manual',
        spec_mode: '454',
        qty: 2,
        unit: '件',
        unit_price: 88,
        discount_type: 'free',
        discount_value: '',
      },
    ],
  })

	assert.deepEqual(payload.discount_type, ['free'])
	assert.deepEqual(payload.discount_value, [''])
})

test('buildOrderPayload carries unit amount discount values', () => {
	const payload = buildOrderPayload({
		form: {
			order_date: '2026-05-22',
			customer_id: 3,
			source_id: 1,
			order_type_id: 1,
			pay_status_id: 2,
			ship_status_id: 1,
		},
		rows: [
			{
				product_id: 7,
				product_name: '橘皮乌龙',
				tier_id: 'manual',
				spec_mode: '454',
				qty: 2,
				unit: '件',
				unit_price: 88,
				discount_type: 'unit_amount',
				discount_value: '10',
			},
		],
	})

	assert.deepEqual(payload.discount_type, ['unit_amount'])
	assert.deepEqual(payload.discount_value, ['10'])
})

test('wholesalePriceUnit keeps 454g rows priced as yuan per lb', () => {
	assert.deepEqual(wholesalePriceUnit({ spec_mode: '454' }), { label: '元/磅', suffix: '/磅', unitG: 454 })
})

test('filterOptions searches names, full pinyin, initials, and codes', () => {
  const rows = [
    { name: '橘皮乌龙', py: 'jupiwulong', pyi: 'jpwl' },
    { name: '测试客户', code: 'C-001' },
  ]

  assert.deepEqual(filterOptions(rows, 'jp').map((item) => item.name), ['橘皮乌龙'])
  assert.deepEqual(filterOptions(rows, '001').map((item) => item.name), ['测试客户'])
})

test('filterProductsForCustomer keeps public and selected customer products only', () => {
  const rows = [
    { id: 1, name: '公共拼配', customer_id: 0, visibility: 'public' },
    { id: 2, name: '客户A深烘', customer_id: 3, visibility: 'customer_only' },
    { id: 3, name: '客户B深烘', customer_id: 4, visibility: 'customer_only' },
  ]

  assert.deepEqual(filterProductsForCustomer(rows, 3).map((item) => item.name), ['公共拼配', '客户A深烘'])
  assert.deepEqual(filterProductsForCustomer(rows, 0).map((item) => item.name), ['公共拼配'])
})

test('filterProductsForCustomer prefers current customer aliases over base product duplicates', () => {
  const rows = [
    { id: 7, name: '系统意式', customer_id: 0, visibility: 'public', tiers: [] },
    {
      id: 7,
      name: 'Karen 贴牌意式',
      customer_id: 42,
      visibility: 'customer_alias',
      customer_product_alias_id: 910,
      customer_item_code: 'KAREN-ESP',
      tiers: [],
    },
    {
      id: 7,
      name: '其他客户贴牌意式',
      customer_id: 43,
      visibility: 'customer_alias',
      customer_product_alias_id: 911,
      tiers: [],
    },
  ]

  const scoped = filterProductsForCustomer(rows, 42)

  assert.deepEqual(scoped.map((item) => item.name), ['Karen 贴牌意式'])
  assert.equal(scoped[0].customer_product_alias_id, 910)
  assert.equal(scoped[0].id, 7)
})

test('filterProductsForCustomer hides public products when customer commercial bean list owns the scope', () => {
  const rows = [
    {
      id: 1,
      name: '公共拼配',
      customer_id: 0,
      visibility: 'public',
      product_kind: 'roasted',
      tiers: [{ id: 11, price_source_json: '{}' }],
    },
    {
      id: 2,
      name: '芬纳定制-红酒日晒-中深烘',
      customer_id: 74,
      visibility: 'customer_only',
      product_kind: 'roasted',
      tiers: [{ id: 56, price_source_json: '{"source":"published_bean_list","list_type":"commercial","publication_id":31}' }],
    },
    {
      id: 3,
      name: '芬纳未发布定制',
      customer_id: 74,
      visibility: 'customer_only',
      product_kind: 'roasted',
      tiers: [],
    },
  ]

  assert.deepEqual(
    filterProductsForCustomer(rows, 74, { commercial: [31] }).map((item) => item.name),
    ['芬纳定制-红酒日晒-中深烘'],
  )
})

test('filterProductsForCustomer limits public fallback products to the selected public publication', () => {
  const rows = [
    {
      id: 1,
      name: '公共新版拼配',
      customer_id: 0,
      visibility: 'public',
      product_kind: 'roasted',
      tiers: [{ id: 57, price_source_json: '{"source":"published_bean_list","list_type":"commercial","publication_id":33}' }],
    },
    {
      id: 2,
      name: '公共旧版拼配',
      customer_id: 0,
      visibility: 'public',
      product_kind: 'roasted',
      tiers: [{ id: 56, price_source_json: '{"source":"published_bean_list","list_type":"commercial","publication_id":31}' }],
    },
    {
      id: 3,
      name: '无豆单公共商品',
      customer_id: 0,
      visibility: 'public',
      product_kind: 'roasted',
      tiers: [],
    },
  ]

  assert.deepEqual(
    filterProductsForCustomer(rows, 74, { commercial: [31] }).map((item) => item.name),
    ['公共旧版拼配'],
  )
})

test('filterProductsForCustomer hides public products when customer disables public SKU usage', () => {
  const rows = [
    { id: 1, name: '公共熟豆', customer_id: 0, visibility: 'public', product_kind: 'roasted' },
    { id: 2, name: '岩师傅红酒日晒生豆', customer_id: 0, visibility: 'public', product_kind: 'green_bean' },
    { id: 3, name: '芬纳定制-红酒日晒-中深烘', customer_id: 74, visibility: 'customer_only', product_kind: 'roasted' },
  ]

  assert.deepEqual(
    filterProductsForCustomer(rows, 74, {}, [{ customer_id: 74, use_public_sku: false }]).map((item) => item.name),
    ['芬纳定制-红酒日晒-中深烘'],
  )
})

test('customer-owned publication scope stays separate from public fallback when public SKU usage is disabled', () => {
  const groups = beanListVersionOptionGroups([
    {
      id: 9951,
      customer_id: 74,
      list_type: 'commercial',
      classification_template_id: 221,
      is_customer_owned: true,
      is_default: true,
    },
    {
      id: 9953,
      customer_id: 74,
      list_type: 'commercial',
      classification_template_id: 222,
      is_customer_owned: false,
      is_default: true,
    },
  ])
  const available = activeBeanListPublicationIDsByType(groups, {})
  const owned = orderEntry.activeCustomerOwnedBeanListPublicationIDsByType(groups, {}, 74)
  const rows = [
    { id: 1, name: '客户发布中的公共档案', visibility: 'public', tiers: [{ publication_id: 9951, list_type: 'commercial' }] },
    { id: 2, name: '公共回退商品', visibility: 'public', tiers: [{ publication_id: 9953, list_type: 'commercial' }] },
    { id: 3, name: '客户专属商品', customer_id: 74, visibility: 'customer_only', tiers: [{ publication_id: 9951, list_type: 'commercial' }] },
  ]

  assert.deepEqual(available, { commercial: [9951, 9953] })
  assert.deepEqual(owned, { commercial: [9951] })
  assert.deepEqual(
    filterProductsForCustomer(rows, 74, available, [{ customer_id: 74, use_public_sku: false }], owned).map((item) => item.name),
    ['客户发布中的公共档案', '客户专属商品'],
  )
  assert.deepEqual(
    filterProductsForCustomer(rows, 74, available, [{ customer_id: 74, use_public_sku: true }], owned).map((item) => item.name),
    ['客户发布中的公共档案', '公共回退商品', '客户专属商品'],
  )
})

test('publication filtering uses frozen tier list type and exact active publication IDs', () => {
  const rows = [
    { id: 1, name: '旧零售', product_kind: 'roasted', tiers: [{ publication_id: 99, list_type: 'retail' }] },
    { id: 2, name: '新零售', product_kind: 'roasted', tiers: [{ publication_id: 101, list_type: 'retail' }] },
    { id: 3, name: '旧挂耳', product_kind: 'drip_bag', tiers: [{ publication_id: 90, list_type: 'drip' }] },
    { id: 4, name: '新挂耳', product_kind: 'drip_bag', tiers: [{ publication_id: 92, list_type: 'drip' }] },
    { id: 5, name: '分类挂耳', product_kind: 'drip_bag', tiers: [{ publication_id: 93, list_type: 'commercial' }] },
    { id: 6, name: '当前生豆', product_kind: 'green_bean', tiers: [{ publication_id: 94, list_type: 'green' }] },
  ]

  assert.deepEqual(filterProductsForCustomer(rows, 0, { retail: [101] }).map((item) => item.name), ['新零售'])
  assert.deepEqual(filterProductsForCustomer(rows, 0, { drip: [92] }).map((item) => item.name), ['新挂耳'])
  assert.deepEqual(filterProductsForCustomer(rows, 0, { commercial: [93] }).map((item) => item.name), ['分类挂耳'])
  assert.deepEqual(filterProductsForCustomer(rows, 0, { green: [94] }).map((item) => item.name), ['当前生豆'])
  assert.deepEqual(
    filterProductsForCustomer(rows, 0, { commercial: [93], drip: [92] }).map((item) => item.name),
    ['新挂耳', '分类挂耳'],
  )
})

test('latest classified publication comparison stays inside the requested list type', () => {
  const options = [
    { id: 11, list_type: 'commercial', classification_template_id: 9, version_no: 'V3.0.9', published_at: '2026-07-21 09:00' },
    { id: 22, list_type: 'retail', classification_template_id: 9, version_no: 'V9.0.0', published_at: '2026-07-22 09:00' },
  ]
  const row = {
    product_id: 7,
    product_kind: 'roasted_bean',
    product_type_category_id: 9,
    bean_list_publication_id: 11,
  }

  assert.equal(latestProductPriceListVersionOption(options, row, 'commercial')?.id, 11)
  assert.equal(rowUsesStaleBeanListPublication(row, options, 'commercial'), false)
})

test('sortProductsByCustomerUsage moves customer common products first without losing original fallback order', () => {
  const rows = [
    { id: 1, name: 'A 公共豆' },
    { id: 2, name: 'B 老客户常订' },
    { id: 3, name: 'C 高频产品' },
    { id: 4, name: 'D 新品' },
  ]
  const usage = [
    { customer_id: 3, product_id: 2, order_count: 2, item_count: 3, last_order_date: '2026-05-01' },
    { customer_id: 3, product_id: 3, order_count: 5, item_count: 5, last_order_date: '2026-05-02' },
    { customer_id: 4, product_id: 1, order_count: 99, item_count: 99, last_order_date: '2026-05-03' },
  ]

  assert.deepEqual(
    sortProductsByCustomerUsage(rows, 3, usage).map((item) => item.id),
    [3, 2, 1, 4],
  )
  assert.deepEqual(
    sortProductsByCustomerUsage(rows, 0, usage).map((item) => item.id),
    [1, 2, 3, 4],
  )
})

test('sortProductsByCustomerUsage aggregates concrete SKU history onto one parent family', () => {
  const rows = [
    { id: 10, name: 'A', specs: [{ sku_id: 101 }, { sku_id: 102 }] },
    { id: 20, name: 'B', specs: [{ sku_id: 201 }] },
  ]
  const usage = [
    { customer_id: 3, product_id: 102, order_count: 9, item_count: 12, last_order_date: '2026-07-20' },
  ]

  assert.deepEqual(sortProductsByCustomerUsage(rows, 3, usage).map((item) => item.id), [10, 20])
})

test('order entry product dropdown applies customer product usage after filtering customer scope', () => {
  const source = orderEntryViewSource()
  assert.match(source, /const customerProductUsages = ref\(\[\]\)/)
  assert.match(source, /customerProductUsages\.value = data\.customer_product_usages \|\| \[\]/)
  assert.match(source, /function scopedOrderProductOptions\(\) \{\s*return filterProductsForCustomer\(/s)
  assert.doesNotMatch(source, /scopedLegacyProducts/)
  assert.match(source, /sortProductsByCustomerUsage\(\s*orderProductFamilyOptions\(scopedOrderProductOptions\(\), row\.product_query, activeProductKindFilter\(row\)\)/s)
  assert.match(source, /form\.customer_id,\s*customerProductUsages\.value/s)
})

test('defaultStatusID picks paid and unshipped status labels', () => {
  assert.equal(defaultStatusID([{ id: 1, name: '未付款' }, { id: 2, name: '已付款' }], ['已付款']), 2)
  assert.equal(defaultStatusID([{ id: 3, name: '未发货' }], ['未发货']), 3)
})

test('syncDripTierPrice matches bag tiers by bag quantity', () => {
  const dripProduct = {
    id: 21,
    name: '耶加雪菲挂耳',
    product_kind: 'drip_bag',
    drip_bag_grams: 10,
    drip_box_bag_count: 10,
    tiers: [
      { id: 81, product_kind: 'drip_bag', sales_unit: 'bag', min: 1, max: 99, unit_price: 2.4, unit_bag_count: 1 },
      { id: 82, product_kind: 'drip_bag', sales_unit: 'bag', min: 100, max: null, unit_price: 2.15, unit_bag_count: 1 },
    ],
  }

  const got = syncDripTierPrice(dripProduct, { sales_unit: 'bag', qty: 120 })

  assert.deepEqual(got, { tierID: '82', unitPrice: '2.15' })
  assert.equal(lineTotal(dripProduct, { product_kind: 'drip_bag', sales_unit: 'bag', qty: 120, unit_price: '2.15' }, false), 258)
})

test('syncDripTierPrice uses an explicit box tier before bag conversion', () => {
  const dripProduct = {
    id: 22,
    name: '哥伦比亚挂耳',
    product_kind: 'drip_bag',
    drip_bag_grams: 10,
    drip_box_bag_count: 10,
    tiers: [
      { id: 91, product_kind: 'drip_bag', sales_unit: 'bag', min: 1, max: 99, unit_price: 2.4, unit_bag_count: 1 },
      { id: 92, product_kind: 'drip_bag', sales_unit: 'bag', min: 100, max: null, unit_price: 2.15, unit_bag_count: 1 },
      { id: 93, product_kind: 'drip_bag', sales_unit: 'box', min: 10, max: null, unit_price: 30, unit_bag_count: 10 },
    ],
  }

  const row = { product_kind: 'drip_bag', sales_unit: 'box', unit_bag_count: 10, unit_bean_g: 10, qty: 12 }
  const got = syncDripTierPrice(dripProduct, row)

  assert.deepEqual(got, { tierID: '93', unitPrice: '30' })
  assert.equal(lineTotal(dripProduct, { ...row, unit_price: got.unitPrice }, false), 360)
})

test('syncDripTierPrice converts boxes to bag tiers only when no box tiers are published', () => {
  const dripProduct = {
    product_kind: 'drip_bag',
    drip_bag_grams: 10,
    drip_box_bag_count: 10,
    tiers: [
      { id: 91, product_kind: 'drip_bag', sales_unit: 'bag', min: 1, max: 99, unit_price: 2.4, unit_bag_count: 1 },
      { id: 92, product_kind: 'drip_bag', sales_unit: 'bag', min: 100, max: null, unit_price: 2.15, unit_bag_count: 1 },
    ],
  }

  const got = syncDripTierPrice(dripProduct, { sales_unit: 'box', unit_bag_count: 10, qty: 12 })
  assert.deepEqual(got, { tierID: '92', unitPrice: '21.5' })
})

test('syncDripTierPrice leaves price blank outside legal bag and explicit box tiers', () => {
  const dripProduct = {
    product_kind: 'drip_bag',
    drip_bag_grams: 10,
    drip_box_bag_count: 10,
    tiers: [
      { id: 91, product_kind: 'drip_bag', sales_unit: 'bag', min: 1, max: 99, unit_price: 2.4, unit_bag_count: 1 },
      { id: 92, product_kind: 'drip_bag', sales_unit: 'bag', min: 100, max: 199, unit_price: 2.15, unit_bag_count: 1 },
      { id: 94, product_kind: 'drip_bag', sales_unit: 'bag', min: 300, max: null, unit_price: 2, unit_bag_count: 1 },
      { id: 93, product_kind: 'drip_bag', sales_unit: 'box', min: 20, max: 29, unit_price: 30, unit_bag_count: 10 },
    ],
  }

  assert.deepEqual(syncDripTierPrice(dripProduct, { sales_unit: 'bag', qty: 250 }), { tierID: 'auto', unitPrice: '' })
  assert.deepEqual(syncDripTierPrice(dripProduct, { sales_unit: 'box', unit_bag_count: 10, qty: 10 }), { tierID: 'auto', unitPrice: '' })
})

test('dripTierPriceRows exposes unit labels for bag and box quotation', () => {
  const got = dripTierPriceRows({
    product_kind: 'drip_bag',
    drip_bag_grams: 10,
    drip_box_bag_count: 10,
    tiers: [
      { id: 101, product_kind: 'drip_bag', sales_unit: 'bag', min: 100, max: null, unit_price: 2.15, unit_bag_count: 1 },
      { id: 102, product_kind: 'drip_bag', sales_unit: 'box', min: 10, max: null, unit_price: 21.5, unit_bag_count: 10 },
    ],
  }, { sales_unit: 'box', unit_bag_count: 10, qty: 12 })

  assert.deepEqual(got, [
    { id: '101', salesUnit: 'bag', specLabel: '10g/袋', rangeLabel: '100袋+', unitPrice: 21.5, priceUnit: { label: '元/盒', suffix: '/盒' } },
    { id: '102', salesUnit: 'box', specLabel: '10袋/盒', rangeLabel: '10盒+', unitPrice: 21.5, priceUnit: { label: '元/盒', suffix: '/盒' } },
  ])
})

test('buildOrderPayload carries drip product unit metadata', () => {
  const payload = buildOrderPayload({
    form: {
      order_date: '2026-05-18',
      customer_id: 3,
      source_id: 1,
      order_type_id: 1,
      pay_status_id: 2,
      ship_status_id: 1,
    },
    rows: [
      {
        product_id: 22,
        product_name: '哥伦比亚挂耳',
        product_kind: 'drip_bag',
        tier_id: '92',
        sales_unit: 'box',
        unit_bag_count: 10,
        unit_bean_g: 10,
        qty: 12,
        unit_price: '21.5',
      },
    ],
  })

  assert.equal(payload.product_id[0], '22')
  assert.equal(payload.product_kind[0], 'drip_bag')
  assert.equal(payload.sales_unit[0], 'box')
  assert.equal(payload.unit_bag_count[0], '10')
  assert.equal(payload.unit_bean_g[0], '10')
  assert.equal(payload.unit[0], '盒')
  assert.equal(payload.spec[0], '100')
})

// Realistic product-catalog type objects (as built by buildProductCatalogTemplatePriceListTypeOptions)
// carry a negative sentinel id (-2000000-groupID); the publication identity used to match published
// price lists lives in publicationClassificationTemplateID (8e15+groupID).
const coffeeBeanPriceListType = {
  id: -2001532,
  categoryID: 0,
  key: 'product-catalog:1532',
  label: '咖啡豆',
  listType: 'commercial',
  productCatalogGroupID: 1532,
  publicationProductTypeCategoryID: 8000000000001532,
  publicationClassificationTemplateID: 8000000000001532,
}

test('filterBeanListVersionOptionsToCurrentTypes keeps current price-list type and hides legacy groups', () => {
  const options = [
    { id: 108, list_type: 'commercial', classification_template_id: 8000000000001532 }, // 咖啡豆 current
    { id: 107, list_type: 'commercial', classification_template_id: 221 },              // 熟豆 orphaned
    { id: 106, list_type: 'green', classification_template_id: 222 },                   // 生豆 orphaned
    { id: 90, list_type: 'green', classification_template_id: 0 },                       // 生豆 legacy
  ]
  const kept = orderEntry.filterBeanListVersionOptionsToCurrentTypes(options, [coffeeBeanPriceListType])
  assert.deepEqual(kept.map((o) => o.id), [108])
})

test('filterBeanListVersionOptionsToCurrentTypes matches real builder output end-to-end', () => {
  const types = buildProductCatalogTemplatePriceListTypeOptions(
    [{ id: 1, name: '曜石' }],
    {
      templates: [{ id: 1532, name: '咖啡豆', active: true, items: [{ id: 24122, name: '意式咖啡' }] }],
      assignments: [],
    },
  )
  const options = [
    { id: 108, list_type: 'commercial', classification_template_id: 8000000000001532 },
    { id: 107, list_type: 'commercial', classification_template_id: 221 },
    { id: 90, list_type: 'green', classification_template_id: 0 },
  ]
  const kept = orderEntry.filterBeanListVersionOptionsToCurrentTypes(options, types)
  assert.deepEqual(kept.map((o) => o.id), [108])
})

test('filterBeanListVersionOptionsToCurrentTypes hides legacy options once classified current types exist', () => {
  const options = [
    { id: 108, list_type: 'commercial', classification_template_id: 8000000000001532 },
    { id: 50, list_type: 'drip', classification_template_id: 0 }, // legacy drip, not a current type -> hide
  ]
  const kept = orderEntry.filterBeanListVersionOptionsToCurrentTypes(options, [coffeeBeanPriceListType])
  assert.deepEqual(kept.map((o) => o.id), [108])
})

test('filterBeanListVersionOptionsToCurrentTypes falls back to all options when no current types loaded', () => {
  const options = [{ id: 108, list_type: 'commercial', classification_template_id: 8000000000001532 }]
  assert.deepEqual(orderEntry.filterBeanListVersionOptionsToCurrentTypes(options, []), options)
  assert.deepEqual(orderEntry.filterBeanListVersionOptionsToCurrentTypes(options, [{ id: 0, listType: 'commercial', label: '全部商品' }]), options)
})
