import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import fs from 'node:fs'
import {
  dedupePriceListFlatRows,
  priceListPricingRuleTrialRequestsForRows,
} from './costing-price-list-workflow.js'

describe('costing price-list workflow helpers', () => {
  it('selects uncached pricing-rule trial requests and skips terminal cache rows', () => {
    const rows = [
      { row_key: 'a', product_id: 7, pricing_rule_id: 11 },
      { row_key: 'b', product_id: 8, pricing_rule_id: 12 },
      { row_key: 'c', product_id: 9, pricing_rule_id: 0 },
    ]
    const requests = priceListPricingRuleTrialRequestsForRows(rows, {
      customerID: 3,
      cache: { '8:12:3': { status: 'success' } },
      payloadForRow: (row, context) => (
        row.pricing_rule_id > 0
          ? { product_id: row.product_id, pricing_rule_id: row.pricing_rule_id, customer_id: context.customerID }
          : null
      ),
      cacheKeyForPayload: (payload) => `${payload.product_id}:${payload.pricing_rule_id}:${payload.customer_id}`,
    })

    assert.deepEqual(requests, [{
      row: rows[0],
      payload: { product_id: 7, pricing_rule_id: 11, customer_id: 3 },
      key: '7:11:3',
    }])
  })

  it('collapses duplicate tier-template flat rows for the same product and pricing rule', () => {
    const rows = [
      {
        row_key: '556:tier-template:11:25',
        product_id: 556,
        product_name: '熟豆-白巧坚果拼配',
        pricing_mode: 'tier_template',
        tier_template_id: 11,
        template_tier_id: 25,
        pricing_rule_id: 11,
        tier_pricing_rule_id: 11,
        tier_label: '24kg',
        price_unit: 'kg',
      },
      {
        row_key: '556:tier-template:11:26',
        product_id: 556,
        product_name: '熟豆-白巧坚果拼配',
        pricing_mode: 'tier_template',
        tier_template_id: 11,
        template_tier_id: 26,
        pricing_rule_id: 11,
        tier_pricing_rule_id: 11,
        tier_label: '1kg',
        price_unit: 'kg',
      },
      {
        row_key: '557:tier-template:11:26',
        product_id: 557,
        product_name: '曜石2.0',
        pricing_mode: 'tier_template',
        tier_template_id: 11,
        template_tier_id: 26,
        pricing_rule_id: 11,
        tier_pricing_rule_id: 11,
        tier_label: '1kg',
        price_unit: 'kg',
      },
    ]

    const got = dedupePriceListFlatRows(rows)

    assert.deepEqual(got.map((row) => row.row_key), [
      '556:tier-template:11:25',
      '557:tier-template:11:26',
    ])
  })

  it('keeps tier-template flat rows when tiers use different pricing rules', () => {
    const rows = [
      {
        row_key: '88:tier-template:3:31',
        product_id: 88,
        product_name: '初晓拼配',
        pricing_mode: 'tier_template',
        tier_template_id: 3,
        template_tier_id: 31,
        pricing_rule_id: 41,
        tier_pricing_rule_id: 41,
        price_unit: 'kg',
      },
      {
        row_key: '88:tier-template:3:32',
        product_id: 88,
        product_name: '初晓拼配',
        pricing_mode: 'tier_template',
        tier_template_id: 3,
        template_tier_id: 32,
        pricing_rule_id: 42,
        tier_pricing_rule_id: 42,
        price_unit: 'kg',
      },
    ]

    assert.deepEqual(dedupePriceListFlatRows(rows).map((row) => row.row_key), [
      '88:tier-template:3:31',
      '88:tier-template:3:32',
    ])
  })

  it('treats child SKU id as the price-list row identity', () => {
    const rows = [
      {
        row_key: 'parent88:sku101:tier31',
        product_id: 88,
        sku_id: 101,
        pricing_mode: 'tier_template',
        tier_template_id: 3,
        template_tier_id: 31,
        tier_pricing_rule_id: 41,
        price_unit: '袋',
      },
      {
        row_key: 'parent88:sku102:tier31',
        product_id: 88,
        sku_id: 102,
        pricing_mode: 'tier_template',
        tier_template_id: 3,
        template_tier_id: 31,
        tier_pricing_rule_id: 41,
        price_unit: '袋',
      },
    ]

    assert.deepEqual(dedupePriceListFlatRows(rows).map((row) => row.row_key), [
      'parent88:sku101:tier31',
      'parent88:sku102:tier31',
    ])
  })

  it('product price list flat rows read product master sales unit conversion', () => {
    const source = fs.readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')

    assert.match(source, /default_sales_unit/)
    assert.match(source, /unit_conversion_json/)
    assert.match(source, /itemSkuID/)
    assert.match(source, /sku_snapshot/)
    assert.match(source, /parent_product_id/)
    assert.match(source, /priceListFlatRowUnitSummary/)
    assert.match(source, /商品档案单位/)
  })
})
