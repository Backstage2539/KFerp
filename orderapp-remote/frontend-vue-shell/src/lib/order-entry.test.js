import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

import * as orderEntry from './order-entry.js'
import {
  buildOrderPayload,
  defaultWholesaleSpec,
  defaultStatusID,
  filterOptions,
  filterProductsForCustomer,
  formatSpecLabel,
  dripTierPriceRows,
  lineDiscountAmount,
  lineTotal,
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
  wholesalePriceUnit,
  wholesaleTierPriceRows,
  wholesaleSpecOptions,
} from './order-entry.js'

function orderEntryViewSource() {
  return readFileSync(new URL('../views/OrderEntryView.vue', import.meta.url), 'utf8')
}

function cssBlock(source, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = source.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`, 's'))
  return match?.[1] || ''
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

test('buildOrderPayload saves real custom spec grams', () => {
  const payload = buildOrderPayload({
    form: {
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

  assert.equal(payload.product_id[0], '7')
  assert.equal(payload.spec[0], '300')
  assert.equal(payload.qty[0], '2')
  assert.equal(payload.item_name[0], '橘皮乌龙')
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

  assert.deepEqual(got, { tierID: '50', unitPrice: '10.66' })
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

test('buildOrderPayload includes structured order responsible person', () => {
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

  assert.equal(payload.responsible_type, 'employee')
  assert.equal(payload.responsible_id, 8)
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

test('responsibleOptions groups employees and customer partners for commission ownership', () => {
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
    { type: 'customer', id: 3, name: '测试客户', label: '合作方/客户 - 测试客户', meta: '门店老板 13800000003', search: '合作方 客户 测试客户 门店老板 13800000003' },
  ])
})

test('order entry raises the active combobox above following fields', () => {
  const source = orderEntryViewSource()

  assert.match(source, /<label class="customer-combobox combobox"\s+:class="\{\s*open:\s*customerOpen\s*\}">/)
  assert.match(source, /<label class="responsible-combobox combobox"\s+:class="\{\s*open:\s*responsibleOpen\s*\}">/)
  assert.match(source, /@focus="customerOpen = true; responsibleOpen = false"/)
  assert.match(source, /@focus="responsibleOpen = true; customerOpen = false"/)

  const baseZIndex = zIndexForSelector(source, '.combobox')
  const openZIndex = zIndexForSelector(source, '.combobox.open')
  assert.ok(openZIndex > baseZIndex, `expected active combobox z-index ${openZIndex} to exceed base z-index ${baseZIndex}`)
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
