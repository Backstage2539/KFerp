import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { priceListPricingRuleTrialRequestsForRows } from './costing-price-list-workflow.js'

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
})
