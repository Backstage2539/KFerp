import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  warehouseInventoryItemKey,
  warehouseInventoryQuantityLabel,
  warehouseInventoryRowKey,
  warehouseInventorySpecLabel,
  warehouseInventoryUnitLabel,
} from './warehouse-inventory-spec-identity.js'

const canonical = {
  warehouse: 'finished_goods',
  item_type: 'finished_product',
  item_id: 550,
  bom_spec_id: 91,
  bom_variant_id: 191,
  bom_spec_name: '227g袋',
  inventory_unit: '袋',
  spec_g: 0,
  batch_id: 901,
  qty_g: 0,
  qty_units: 12,
}

test('canonical warehouse rows display BOM specification name, inventory unit and one-to-one quantity', () => {
  assert.equal(warehouseInventorySpecLabel(canonical), '227g袋')
  assert.equal(warehouseInventoryUnitLabel(canonical), '袋')
  assert.equal(warehouseInventoryQuantityLabel(canonical), '12 袋')
})

test('warehouse grouping and selection identity use parent product plus BOM spec while variant remains trace metadata', () => {
  assert.equal(warehouseInventoryItemKey(canonical), 'finished_product:550:bom_spec:91')
  assert.equal(
    warehouseInventoryItemKey({ ...canonical, bom_variant_id: 192 }),
    'finished_product:550:bom_spec:91',
  )
  assert.equal(
    warehouseInventoryItemKey({ ...canonical, bom_spec_id: 92, bom_variant_id: 292 }),
    'finished_product:550:bom_spec:92',
  )
  assert.notEqual(
    warehouseInventoryRowKey(canonical),
    warehouseInventoryRowKey({ ...canonical, batch_id: 902 }),
  )
})

test('legacy warehouse row labels and item identity stay compatible', () => {
  const legacy = {
    warehouse: 'finished_goods',
    item_type: 'finished_product',
    item_id: 551,
    spec_g: 227,
    batch_code: 'LEGACY-227',
    qty_g: 454,
    qty_units: 2,
  }
  assert.equal(warehouseInventoryItemKey(legacy), 'finished_product:551:227')
  assert.equal(warehouseInventorySpecLabel(legacy), '227g')
  assert.equal(warehouseInventoryUnitLabel(legacy), '')
  assert.equal(warehouseInventoryQuantityLabel(legacy), '2 件 / 454g')
})
