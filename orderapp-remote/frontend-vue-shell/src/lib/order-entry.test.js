import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

import * as orderEntry from './order-entry.js'
import {
  beanListVersionOptionGroups,
  beanListVersionOptionsForCustomer,
  buildOrderPayload,
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
  needsTrailingBlankOrderLine,
  orderRowPriceUnit,
  resolveWholesaleTierPrice,
  syncDripTierPrice,
  syncWholesaleTierPrice,
  normalizeSpecG,
  orderReceiptMethodOptions,
  productKindBadgeClass,
  productKindLabel,
  requiresOrderPaymentMethod,
  responsibleOptions,
  retailPackagePrice,
  retailSpecOptions,
  rowUsesStaleBeanListPublication,
  sortProductsByCustomerUsage,
  wholesalePriceUnit,
  wholesaleTierPriceRows,
  wholesaleSpecOptions,
} from './order-entry.js'

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
  const row = { spec_mode: '80', qty: 10, tier_id: 'auto', unit_price: '' }
  const got = syncWholesaleTierPrice({
    tiers: [
      { id: 50, spec_g: 1000, min: 1, max: 59, unit_price: 23.49, display_unit: 'kg' },
    ],
  }, row)

  assert.deepEqual(got, { tierID: '50', unitPrice: '23.49' })
})

test('resolveWholesaleTierPrice keeps kg tier unit, source version, and below-min warning for small package orders', () => {
  const row = { spec_mode: '80', qty: 1, tier_id: 'auto', unit_price: '' }
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
  assert.equal(got.belowMinTier, true)

  const pricedRow = {
    ...row,
    tier_id: got.tierID,
    unit_price: got.unitPrice,
    price_unit: got.priceUnit.label,
    price_unit_suffix: got.priceUnit.suffix,
    price_unit_g: got.priceUnit.unitG,
  }
  assert.deepEqual(orderRowPriceUnit(pricedRow), { label: '元/kg', suffix: '/kg', unitG: 1000 })
  assert.equal(Number(lineTotal({ tiers: [] }, pricedRow, false).toFixed(2)), 6.56)
})

test('OrderEntryView shows tier unit price, price-list source without unrecorded fallback, and below-min warning', () => {
  const source = orderEntryViewSource()

  assert.match(source, /tier_price_label/)
  assert.match(source, /低于最低梯度/)
  assert.match(source, /\.tier-warning/)
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

test('SalesOrderView shows read-only quote source and production source trace blocks', () => {
  const source = salesOrderViewSource()
  for (const expected of [
    '报价来源',
    '生产来源',
    'quote_source_trace',
    'production_source_trace',
    'salesOrderTraceLineLabel',
    'salesOrderTraceLines',
    'loadSalesOrderTrace',
  ]) {
    assert.match(source, new RegExp(expected))
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
    ['category:9', '冷萃类', 'commercial', [1, 2]],
    ['legacy:green', '生豆豆单', 'green', [3]],
  ])
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
  assert.match(source, /<label class="product-combobox combobox product-cell"\s+:class="\{\s*open:\s*row\.product_open\s*\}">/)
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

test('order entry product dropdown applies customer product usage after filtering customer scope', () => {
  const source = orderEntryViewSource()
  assert.match(source, /const customerProductUsages = ref\(\[\]\)/)
  assert.match(source, /customerProductUsages\.value = data\.customer_product_usages \|\| \[\]/)
  assert.match(source, /sortProductsByCustomerUsage\(\s*filterOptions\(\s*filterProductsForCustomer\(/s)
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

test('syncDripTierPrice converts box orders to bag tiers before pricing', () => {
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

  assert.deepEqual(got, { tierID: '92', unitPrice: '21.5' })
  assert.equal(lineTotal(dripProduct, { ...row, unit_price: got.unitPrice }, false), 258)
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
