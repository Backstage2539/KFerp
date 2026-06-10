import test from 'node:test'
import assert from 'node:assert/strict'

import {
  priceListGenerationDraftKey,
  readPriceListGenerationDraft,
  savePriceListGenerationDraft,
} from './product-price-list-draft.js'

function memoryStorage() {
  const data = new Map()
  return {
    getItem: (key) => data.has(key) ? data.get(key) : null,
    setItem: (key, value) => data.set(key, String(value)),
    removeItem: (key) => data.delete(key),
  }
}

test('price list generation draft persists pricing selections by scope and product type key', () => {
  const storage = memoryStorage()
  const key = priceListGenerationDraftKey({
    workspace: 'factory',
    scope: 'official',
    customerID: 0,
    typeKey: 'product-catalog:128:3296',
  })

  savePriceListGenerationDraft(key, {
    defaults: { pricing_mode: 'pricing_rule', pricing_rule_id: 40 },
    productOverrides: {
      550: { product_id: 550, pricing_mode: 'tier_template', tier_template_id: 8 },
    },
  }, storage)

  const restored = readPriceListGenerationDraft(key, storage)
  assert.deepEqual(restored.defaults, { pricing_mode: 'pricing_rule', pricing_rule_id: 40 })
  assert.deepEqual(restored.productOverrides['550'], { product_id: 550, pricing_mode: 'tier_template', tier_template_id: 8 })
})
