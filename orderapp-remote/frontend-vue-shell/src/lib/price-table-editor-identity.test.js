import test from 'node:test'
import assert from 'node:assert/strict'
import { normalizePriceListProductSpecSelections } from './product-price-list-selection.js'

const original = { parent_product_id: 940, product_id: 940, bom_id: 18277, bom_version_id: 423,
  bom_spec_id: 309, bom_variant_id: 607, spec_identity_mode: 'bom_spec',
  bom_spec_authoritative: true, selection_source: 'explicit' }
const families = [{ parent_product_id: 940, default_sku_id: 309,
  sku_options: [{ ...original, sku_id: 309 }] }]

test('published BOM identity without private sku_id restores its exact picker specification', () => {
  const [restored] = normalizePriceListProductSpecSelections([original], families)
  assert.equal(restored.sku_id, 309)
  assert.equal(restored.selection_issue, undefined)
  assert.equal(restored.bom_variant_id, 607)
})

test('a changed BOM retains the original identity and issue after repeated restoration', () => {
  const changed = [{ ...families[0], sku_options: [{ ...families[0].sku_options[0], bom_version_id: 424 }] }]
  const [first] = normalizePriceListProductSpecSelections([{ ...original, sku_id: 309 }], changed)
  assert.equal(first.bom_version_id, 423)
  assert.equal(first.selection_issue, 'default_bom_changed')
  const [second] = normalizePriceListProductSpecSelections([first], changed)
  assert.equal(second.bom_version_id, 423)
  assert.equal(second.selection_issue, 'default_bom_changed')
})

test('catalogue not ready preserves selections instead of treating loading as removal', () => {
  assert.deepEqual(normalizePriceListProductSpecSelections([original], [], { catalogReady: false }), [original])
})
