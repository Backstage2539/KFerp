import test from 'node:test'
import assert from 'node:assert/strict'
import * as entry from './order-entry.js'
import * as shipping from './order-shipping.js'

const families = [[58, 69, '墨照啡石', 56], [57, 48, '菠浪清甜', 92], [48, 242, '酒心可可', 59]].map(([id, specID, name, price]) => ({
  parent_product_id: id, customer_id: 14, name, migration_state: 'cutover', __order_concrete_price_family: true,
  specs: [{ sku_id: specID, bom_spec_id: specID, bom_variant_id: specID + 100, migration_state: 'cutover',
    inventory_unit: '袋', spec_label: '454g袋装', tiers: [{ publication_id: 51, unit_price: price,
      quantity_basis: 'sales_spec_count', price_unit: '袋' }] }],
}))

test('stored order resolves product and BOM spec in separate namespaces across repeated saves', () => {
  for (const family of families) {
    const stored = { product_id: family.parent_product_id, bom_spec_id: family.specs[0].bom_spec_id,
      qty: 5, unit_price: family.specs[0].tiers[0].unit_price, bean_list_publication_id: 51,
      bean_list_version_no: 'V3.0.12', price_source_json: '{"price_table_name":"报价表","publication_id":51}' }
    for (let round = 0; round < 2; round++) {
      const resolved = entry.orderProductFamilyForStoredItem(families, stored, { customer_id: 14 })
      assert.equal(resolved.name, family.name)
      const spec = entry.orderFamilySpecForStoredItem(resolved, stored, 51)
      const hydrated = entry.orderFamilyHydratedSpecRowPatch(resolved, spec, 51, stored)
      assert.equal(hydrated.product_query, family.name)
      const payload = entry.buildOrderPayload({form: {customer_id: 14}, rows: [hydrated]})
      assert.equal(Number(payload.product_id[0]), stored.product_id)
      assert.equal(Number(payload.bom_spec_id[0]), stored.bom_spec_id)
      assert.equal(Number(payload.qty[0]), 5)
      assert.equal(Number(payload.unit_price[0]), stored.unit_price)
      assert.equal(payload.price_source_json[0], stored.price_source_json)
    }
  }
})

test('missing spec or alias never resolves to an unrelated product or customer', () => {
  assert.equal(entry.orderProductFamilyForStoredItem(families, {product_id: 999, bom_spec_id: 48}, {customer_id: 14}), null)
  assert.equal(entry.orderProductFamilyForStoredItem(families, {product_id: 48, bom_spec_id: 242}, {customer_id: 99}), null)
  assert.equal(entry.orderProductFamilyForStoredItem(families, {product_id: 48, bom_spec_id: 242, customer_product_alias_id: 77}, {customer_id: 14}), null)
  const family = entry.orderProductFamilyForStoredItem(families, {product_id: 48, bom_spec_id: 999}, {customer_id: 14})
  assert.equal(family.name, '酒心可可')
  assert.equal(entry.orderFamilySpecForStoredItem(family, {bom_spec_id: 999}, 51), null)
})

test('quote summary deduplicates a table but preserves different publications and versions', () => {
  const rows = [51,51,52].map(id => ({bean_list_publication_id:id, bean_list_version_no:'V3.0.12',
    price_source_json: JSON.stringify({price_table_name:'报价表',publication_id:id})}))
  assert.equal(entry.orderQuoteSourceSummary(rows).length, 2)
  assert.equal(entry.orderQuoteSourceSummary(rows)[0].label, '报价表 · V3.0.12')
})

test('pickup and local delivery cannot enter courier readiness flow', () => {
  for (const method of ['pickup', 'local_delivery']) {
    assert.equal(shipping.isOrderShipReady({ship_method:method, process_status:'生产完成', ship_status:'未发货'}), false)
    assert.equal(shipping.requiresCourierLogistics(method, '已发货'), false)
  }
  assert.equal(shipping.requiresCourierLogistics('sf_small', '已发货'), true)
  assert.equal(shipping.requiresCourierLogistics('', '未发货'), false)
})
