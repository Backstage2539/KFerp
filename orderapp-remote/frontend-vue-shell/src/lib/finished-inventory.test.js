import test from 'node:test'
import assert from 'node:assert/strict'
import { buildFinishedInventoryAdjustmentPayload, finishedInventoryRowQuantity } from './finished-inventory.js'

test('BOM-spec finished inventory writes whole specification units without legacy gram fields', () => {
  const payload = buildFinishedInventoryAdjustmentPayload({
    product_id: 7,
    migration_state: 'cutover',
    bom_spec_id: 91,
    bom_variant_id: 191,
    unit: '袋',
    units: 12,
    spec_g: 227,
    loose_g: 20,
  })

  assert.deepEqual(payload, {
    product_id: 7,
    bom_spec_id: 91,
    bom_variant_id: 191,
    units: 12,
  })
  assert.equal(finishedInventoryRowQuantity({ bom_spec_id: 91, spec_name: '227g 袋', inventory_unit: '袋', units: 12 }), '12 袋')
  assert.equal(finishedInventoryRowQuantity({ spec_g: 227, units: 1, loose_g: 4, total_g: 231 }), '1 件 + 4g（231g）')
})
