import test from 'node:test'
import assert from 'node:assert/strict'

import {
  migrateLegacyFixedPriceFlatRowOverrides,
  normalizeParentSharedPriceListProductOverrides,
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

test('identical legacy SKU pricing configs promote to one parent config while fixed prices stay per SKU', () => {
  const normalized = normalizeParentSharedPriceListProductOverrides({
    'sku:551': { scope: 'sku', product_id: 551, sku_id: 551, parent_product_id: 550, pricing_mode: 'fixed_price', fixed_unit_price: 59.9 },
    'sku:552': { scope: 'sku', product_id: 552, sku_id: 552, parent_product_id: 550, pricing_mode: 'fixed_price', fixed_unit_price: 109.9 },
  })

  assert.deepEqual(normalized.conflicts, [])
  assert.deepEqual(normalized.overrides['parent:550'], {
    scope: 'parent_product',
    product_id: 550,
    sku_id: 0,
    parent_product_id: 550,
    product_key: 'parent:550',
    pricing_mode: 'fixed_price',
    tier_template_id: 0,
    pricing_rule_id: 0,
    fixed_unit_price: 0,
  })
  assert.equal(normalized.overrides['sku:551'].pricing_mode, '')
  assert.equal(normalized.overrides['sku:551'].fixed_unit_price, 59.9)
  assert.equal(normalized.overrides['sku:552'].fixed_unit_price, 109.9)
})

test('parent config wins during draft restore and conflicting legacy SKU configs require reselection', () => {
  const parentWins = normalizeParentSharedPriceListProductOverrides({
    'parent:550': { scope: 'parent_product', product_id: 550, parent_product_id: 550, pricing_mode: 'pricing_rule', pricing_rule_id: 40 },
    'sku:551': { scope: 'sku', product_id: 551, sku_id: 551, parent_product_id: 550, pricing_mode: 'tier_template', tier_template_id: 8, fixed_unit_price: 59.9 },
  })
  assert.equal(parentWins.conflicts.length, 0)
  assert.equal(parentWins.overrides['parent:550'].pricing_mode, 'pricing_rule')
  assert.equal(parentWins.overrides['sku:551'].pricing_mode, '')
  assert.equal(parentWins.overrides['sku:551'].fixed_unit_price, 59.9)

  const conflict = normalizeParentSharedPriceListProductOverrides({
    'sku:551': { scope: 'sku', product_id: 551, sku_id: 551, parent_product_id: 550, pricing_mode: 'tier_template', tier_template_id: 8 },
    'sku:552': { scope: 'sku', product_id: 552, sku_id: 552, parent_product_id: 550, pricing_mode: 'pricing_rule', pricing_rule_id: 40 },
  })
  assert.equal(conflict.conflicts.length, 1)
  assert.equal(conflict.conflicts[0].parent_product_id, 550)
  assert.match(conflict.conflicts[0].message, /旧草稿.*规格.*不一致.*重新选择商品计价/)
  assert.equal(Object.hasOwn(conflict.overrides, 'parent:550'), false)
})

test('legacy SKU configs recover their parent from saved product-spec selections', () => {
  const normalized = normalizeParentSharedPriceListProductOverrides({
    'sku:551': { scope: 'sku', product_id: 551, sku_id: 551, pricing_mode: 'pricing_rule', pricing_rule_id: 40 },
    'sku:552': { scope: 'sku', product_id: 552, sku_id: 552, pricing_mode: 'pricing_rule', pricing_rule_id: 40 },
  }, {
    productSpecSelections: [
      { parent_product_id: 550, sku_id: 551 },
      { parent_product_id: 550, sku_id: 552 },
    ],
  })

  assert.deepEqual(normalized.conflicts, [])
  assert.equal(normalized.overrides['parent:550'].pricing_mode, 'pricing_rule')
  assert.equal(normalized.overrides['parent:550'].pricing_rule_id, 40)
})

test('legacy fixed flat rows migrate one-to-one into the matching concrete SKU override', () => {
  const migrated = migrateLegacyFixedPriceFlatRowOverrides({
    '551:fixed_price': 59.9,
    '552:fixed_price': 109.9,
  }, {
    'parent:550': { scope: 'parent_product', product_id: 550, parent_product_id: 550, pricing_mode: 'fixed_price' },
  }, [
    { parent_product_id: 550, sku_id: 551, product_name: '小菠萝', sku_name: '227g' },
    { parent_product_id: 550, sku_id: 552, product_name: '小菠萝', sku_name: '454g' },
  ])

  assert.equal(migrated.productOverrides['sku:551'].fixed_unit_price, 59.9)
  assert.equal(migrated.productOverrides['sku:552'].fixed_unit_price, 109.9)
  assert.equal(migrated.productOverrides['sku:551'].sku_name, '227g')
  assert.equal(migrated.productOverrides['sku:552'].sku_name, '454g')
  assert.deepEqual(migrated.flatRowOverrides, {})
  assert.equal(migrated.migratedCount, 2)
})

test('a legacy draft containing only fixed flat rows still migrates each exact SKU key', () => {
  const migrated = migrateLegacyFixedPriceFlatRowOverrides({
    '551:fixed_price': 59.9,
    '552:fixed_price': 109.9,
  })

  assert.equal(migrated.productOverrides['sku:551'].fixed_unit_price, 59.9)
  assert.equal(migrated.productOverrides['sku:552'].fixed_unit_price, 109.9)
  assert.equal(migrated.productOverrides['sku:551'].sku_id, 551)
  assert.equal(migrated.productOverrides['sku:552'].sku_id, 552)
  assert.deepEqual(migrated.flatRowOverrides, {})
  assert.equal(migrated.migratedCount, 2)
})

test('a flat-only fixed migration survives the next draft reload without parent metadata', () => {
  const migrated = migrateLegacyFixedPriceFlatRowOverrides({
    '551:fixed_price': 59.9,
  })
  const reloaded = normalizeParentSharedPriceListProductOverrides(migrated.productOverrides, {
    productSpecSelections: [],
  })

  assert.equal(reloaded.overrides['sku:551'].fixed_unit_price, 59.9)
  assert.equal(reloaded.overrides['sku:551'].parent_product_id, 0)
  assert.deepEqual(reloaded.conflicts, [])
})

test('legacy fixed flat rows preserve existing SKU amounts and ignore non-fixed rows', () => {
  const migrated = migrateLegacyFixedPriceFlatRowOverrides({
    '551:fixed_price': 1,
    '999:fixed_price': 88,
    '551:pricing_rule': 66,
  }, {
    'sku:551': { scope: 'sku', sku_id: 551, parent_product_id: 550, fixed_unit_price: 59.9 },
  }, [
    { parent_product_id: 550, sku_id: 551 },
    { parent_product_id: 550, sku_id: 552 },
  ])

  assert.equal(migrated.productOverrides['sku:551'].fixed_unit_price, 59.9)
  assert.equal(migrated.productOverrides['sku:552'], undefined)
  assert.equal(migrated.productOverrides['sku:999'].fixed_unit_price, 88)
  assert.deepEqual(migrated.flatRowOverrides, {
    '551:pricing_rule': 66,
  })
})
