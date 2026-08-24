import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

import {
  buildFinishedAdjustmentPayload,
  buildFinishedTransferPayload,
  defaultBOMSpecID,
} from './stock-bom-spec.js'

const here = dirname(fileURLToPath(import.meta.url))

const cutoverProduct = {
  id: 7,
  name: '规格组商品',
  migration_state: 'cutover',
  bom_specs: [
    { bom_spec_id: 91, bom_variant_id: 191, name: '227g袋', unit: '袋', is_default: true },
    { bom_spec_id: 92, bom_variant_id: 192, name: '454g袋', unit: '袋', is_default: false },
  ],
}

test('cutover transfer submits parent plus stable spec and inventory unit without client variant', () => {
  assert.equal(defaultBOMSpecID(cutoverProduct), 91)
  const payload = buildFinishedTransferPayload({
    product_id: 7,
    bom_spec_id: 91,
    spec_g: 227,
    from_warehouse: 'finished_goods',
    to_warehouse: 'finished_shop',
    qty_units: 4,
    qty_loose_g: 12,
    note: '门店备货',
  }, cutoverProduct)
  assert.deepEqual(payload, {
    product_id: 7,
    bom_spec_id: 91,
    unit_code: '袋',
    from_warehouse: 'finished_goods',
    to_warehouse: 'finished_shop',
    qty_units: 4,
    note: '门店备货',
  })
  assert.equal('bom_variant_id' in payload, false)
  assert.equal('spec_g' in payload, false)
})

test('cutover adjustment submits stable spec target units and inventory unit without client variant', () => {
  const payload = buildFinishedAdjustmentPayload({
    item_id: 7,
    bom_spec_id: 92,
    spec_g: 454,
    warehouse: 'finished_goods',
    target_g: 20,
    target_units: 7,
    reason: '规格盘点',
  }, cutoverProduct)
  assert.deepEqual(payload, {
    adjustment_type: 'quantity',
    item_type: 'finished_product',
    item_id: 7,
    bom_spec_id: 92,
    unit_code: '袋',
    warehouse: 'finished_goods',
    target_units: 7,
    reason: '规格盘点',
  })
  assert.equal('bom_variant_id' in payload, false)
})

test('legacy finished writes keep spec_g and loose gram compatibility', () => {
  const legacyProduct = { id: 8, name: '旧规格商品', migration_state: 'legacy', bom_specs: [] }
  assert.deepEqual(buildFinishedTransferPayload({
    product_id: 8, spec_g: 454, from_warehouse: 'finished_goods', to_warehouse: 'finished_shop',
    qty_units: 2, qty_loose_g: 20, note: '',
  }, legacyProduct), {
    product_id: 8, spec_g: 454, from_warehouse: 'finished_goods', to_warehouse: 'finished_shop',
    qty_units: 2, qty_loose_g: 20, note: '',
  })
})

test('finished transfer and adjustment views select BOM specs from current inventory catalog', () => {
  for (const file of ['FinishedTransfersView.vue', 'StockAdjustmentsView.vue']) {
    const source = readFileSync(resolve(here, `../views/${file}`), 'utf8')
    assert.match(source, /\/api\/products\/inventory\?limit=200/)
    assert.match(source, /BOM 规格/)
    assert.doesNotMatch(source, /bom_variant_id/)
  }
  const transfer = readFileSync(resolve(here, '../views/FinishedTransfersView.vue'), 'utf8')
  const adjustment = readFileSync(resolve(here, '../views/StockAdjustmentsView.vue'), 'utf8')
  assert.match(transfer, /buildFinishedTransferPayload/)
  assert.match(adjustment, /buildFinishedAdjustmentPayload/)
})
