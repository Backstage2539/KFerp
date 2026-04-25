import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildOrderPayload,
  normalizeSpecG,
  retailPackagePrice,
  retailSpecOptions,
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
    { label: '100g', value: '100' },
    { label: '227g', value: '227' },
    { label: '250g', value: '250' },
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

