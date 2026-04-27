import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildOrderPayload,
  defaultStatusID,
  filterOptions,
  formatSpecLabel,
  lineTotal,
  syncWholesaleTierPrice,
  normalizeSpecG,
  retailPackagePrice,
  retailSpecOptions,
  wholesaleSpecOptions,
} from './order-entry.js'

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

test('normalizeSpecG rejects non-positive custom grams', () => {
  assert.equal(normalizeSpecG({ spec_mode: 'custom', custom_spec_g: 0 }), 0)
  assert.equal(normalizeSpecG({ spec_mode: 'custom', custom_spec_g: 300 }), 300)
})

test('wholesaleSpecOptions includes standard order-entry specs and product tiers', () => {
  const got = wholesaleSpecOptions({
    tiers: [
      { spec_g: 454, min: 1, unit_price: 88 },
      { spec_g: 2500, min: 1, unit_price: 388 },
    ],
  })

  assert.deepEqual(got.map((option) => option.value), ['36', '80', '100', '227', '454', '500', '1000', '2500'])
  assert.equal(formatSpecLabel(2500), '2.5kg')
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

test('lineTotal uses manual unit price even for retail rows', () => {
  assert.equal(lineTotal(product, {
    tier_id: 'manual',
    spec_mode: '227',
    qty: 2,
    unit_price: '45',
  }, true), 90)
})

test('filterOptions searches names, full pinyin, initials, and codes', () => {
  const rows = [
    { name: '橘皮乌龙', py: 'jupiwulong', pyi: 'jpwl' },
    { name: '测试客户', code: 'C-001' },
  ]

  assert.deepEqual(filterOptions(rows, 'jp').map((item) => item.name), ['橘皮乌龙'])
  assert.deepEqual(filterOptions(rows, '001').map((item) => item.name), ['测试客户'])
})

test('defaultStatusID picks paid and unshipped status labels', () => {
  assert.equal(defaultStatusID([{ id: 1, name: '未付款' }, { id: 2, name: '已付款' }], ['已付款']), 2)
  assert.equal(defaultStatusID([{ id: 3, name: '未发货' }], ['未发货']), 3)
})
