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
    product_spec_selections: [
      { parent_product_id: 550, sku_id: 551, selection_source: 'product_default', default_sku_id_at_selection: 551 },
      { parent_product_id: 550, sku_id: 552, selection_source: 'explicit', default_sku_id_at_selection: 551 },
    ],
  }, storage)

  const restored = readPriceListGenerationDraft(key, storage)
  assert.deepEqual(restored.defaults, { pricing_mode: 'pricing_rule', pricing_rule_id: 40 })
  assert.deepEqual(restored.productOverrides['550'], { product_id: 550, pricing_mode: 'tier_template', tier_template_id: 8 })
  assert.deepEqual(restored.product_spec_selections, [
    { parent_product_id: 550, sku_id: 551, selection_source: 'product_default', default_sku_id_at_selection: 551 },
    { parent_product_id: 550, sku_id: 552, selection_source: 'explicit', default_sku_id_at_selection: 551 },
  ])
})

test('legacy price-list drafts do not invent an empty product spec selection field', () => {
  const storage = memoryStorage()
  const key = 'legacy-price-list-draft'
  storage.setItem(key, JSON.stringify({ defaults: { pricing_mode: 'fixed_price' } }))

  const restored = readPriceListGenerationDraft(key, storage)

  assert.equal(Object.hasOwn(restored, 'product_spec_selections'), false)
})
