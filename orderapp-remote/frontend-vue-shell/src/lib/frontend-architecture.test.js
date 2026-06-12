import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, it } from 'node:test'

function source(path) {
  return readFileSync(resolve(path), 'utf8')
}

describe('frontend architecture boundaries', () => {
  it('keeps App.vue view-context API calls behind src/api/view-context.js', () => {
    const app = source('src/App.vue')

    assert.match(app, /from '\.\/api\/view-context\.js'/)
    for (const endpoint of [
      '/api/view-context/options?type=customer',
      '/api/view-context/options?type=order',
      '/api/view-context/presets',
    ]) {
      assert.equal(app.includes(endpoint), false, `App.vue should not own ${endpoint}`)
    }
  })

  it('keeps CostingView price-list trial request selection behind a helper module', () => {
    const costing = source('src/views/CostingView.vue')

    assert.match(costing, /costing-price-list-workflow\.js/)
    assert.equal(
      costing.includes('function priceListPricingRuleTrialRequestsForRows'),
      false,
      'CostingView.vue should not own price-list trial request selection',
    )
  })
})
