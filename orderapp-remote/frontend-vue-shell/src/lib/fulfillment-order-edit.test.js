import test from 'node:test'
import assert from 'node:assert/strict'
import { fulfillmentEditFamilies, fulfillmentSpecForRow, fulfillmentSpecRowPatch, fulfillmentEditCommand, fulfillmentFeeSummary, fulfillmentItemProductName, fulfillmentListFeeLines, fulfillmentLinePreview, fulfillmentDraftGoodsCents, fulfillmentSavedGoodsCents, mergeFulfillmentDetailItems } from './fulfillment-order-edit.js'

const catalog = { product_families: [{ parent_product_id: 7, name: '客户已购商品', specs: [
  { product_id: 7, bom_spec_id: 19, bom_variant_id: 30, spec_label: '227g袋装', inventory_unit: '袋', available_qty: 5, pending_inbound_qty: 2, unit_price: 35, price_tiers: [{ publication_id: 91, unit_price: 35 }] },
  { product_id: 7, bom_spec_id: 20, bom_variant_id: 31, spec_label: '454g袋装', inventory_unit: '袋', available_qty: 3, unit_price: 60 },
] }] }

test('fulfillment selection preserves the customer catalog scope and separates prepaid stock from authorized product pricing', () => {
  const stock = fulfillmentEditFamilies(catalog, 'stock')
  const current = fulfillmentSpecForRow(stock, { product_id: 7, bom_spec_id: 19, bom_variant_id: 10 })
  assert.equal(current.spec.bom_variant_id, 30)
  assert.equal(current.spec.pending_inbound_qty, 2)
  assert.equal(fulfillmentSpecRowPatch(current.family, current.spec, 'stock').unit_price, 0)
  assert.equal(fulfillmentSpecForRow(stock, { product_id: 8, bom_spec_id: 19 }).spec, null)
  assert.equal(fulfillmentSpecForRow(stock, { product_id: 7, bom_spec_id: 999 }).spec, null)
  const demand = fulfillmentEditFamilies(catalog, 'on_demand')
  assert.equal(demand[0].specs[0].unit_price, 35)
  assert.equal(demand[0].specs[0].publication_id, 91)
  assert.equal(fulfillmentSpecRowPatch(demand[0], demand[0].specs[0], 'on_demand').unit_price, 35)
})

test('reselecting an original on-demand specification preserves its frozen price and variant only for that exact product and stable spec', () => {
  const family = fulfillmentEditFamilies(catalog, 'on_demand')[0]
  const original = { item_id: 21, product_id: 7, bom_spec_id: 19, bom_variant_id: 10, unit_price: 32, publication_id: 80 }
  const preserved = fulfillmentSpecRowPatch(family, family.specs[0], 'on_demand', original)
  assert.equal(preserved.unit_price, 32)
  assert.equal(preserved.bom_variant_id, 10)
  assert.equal(preserved.bean_list_publication_id, 80)
  const discounted = fulfillmentSpecRowPatch(family, family.specs[0], 'on_demand', { ...original, line_amount: 115, base_line_amount: 120, discount_type: 'amount', discount_value: 5, discount_amount: 5 })
  assert.equal(discounted.discount_type, 'amount')
  assert.equal(discounted.discount_amount, 5)
  assert.equal(fulfillmentSpecRowPatch(family, family.specs[1], 'on_demand', original).unit_price, 60)
  assert.equal(fulfillmentSpecRowPatch(family, family.specs[0], 'on_demand', { ...original, product_id: 8 }).unit_price, 35)
})

test('complete fulfillment edit payload keeps row identities and excludes source, customer, payment, price, and implicit fee overrides', () => {
  const row = { item_id: 21, product_id: 7, bom_spec_id: 19, bom_variant_id: 30, qty: 4, unit_price: 0, bean_list_publication_id: 0 }
  const form = { receiver_name: '收件人', receiver_phone: '13800000000', receiver_address: '本单地址', notes: '编辑备注', order_date: '2026-10-22', shipping_province: '云南省', source_warehouse: 'untrusted', customer_id: 99, payment_goods_amount: 888 }
  const command = fulfillmentEditCommand({ edit_revision: 'original-revision' }, form, [row, { product_id: 0 }])
  assert.equal(command.edit_revision, 'original-revision')
  assert.deepEqual(command.items, [{ item_id: 21, product_id: 7, bom_spec_id: 19, bom_variant_id: 30, qty: 4, spec_g: 0, publication_id: 0, price_selection_mode: 'auto', selected_price_row_key: '' }])
  for (const field of ['customer_id', 'source_warehouse', 'payment_goods_amount', 'service_fee_cents']) assert.equal(field in command, false)
  assert.throws(() => fulfillmentEditCommand({}, form, [{ ...row, qty: 0 }]), /正整数/)
  assert.throws(() => fulfillmentEditCommand({}, form, [{ ...row, bom_spec_id: 0 }]), /完整选择商品规格/)
  assert.throws(() => fulfillmentEditCommand({}, form, [row], { enabled: true, amount: 0, reason: '' }), /调整原因/)
  const freeService = fulfillmentEditCommand({}, form, [row], { enabled: true, amount: '0.00', reason: '本单免服务费' })
  assert.equal(freeService.service_fee_cents, 0)
  assert.equal(freeService.adjustment_reason, '本单免服务费')
})

test('fulfillment fees distinguish estimate, actual invoice, current receivable and prepaid goods', () => {
  const fees = { service_fee_cents: 1400, shipping_fee_cents: 0, freight: { estimate_known: true, estimated_total_cents: 810, current_receivable_cents: 0 } }
  assert.deepEqual(fulfillmentFeeSummary(fees, 0), { goods: 0, service: 1400, shipping: 0, serviceDue: null, estimated: 810, actual: null, billed: null, total: 1400 })
  const billed = fulfillmentFeeSummary(fees, 2000, { actual_cost_cents: 900, billed_cents: 900, service_due_cents: 0 })
  assert.equal(billed.estimated, 810)
  assert.equal(billed.actual, 900)
  assert.equal(billed.serviceDue, 0)
  assert.equal(billed.billed, 900)
  assert.equal(billed.total, 3400, 'an invoice must not become another current order charge')
  const changed = fulfillmentFeeSummary({ ...fees, service_fee_cents: 2000 }, 0, { service_fee_cents: 1400, service_due_cents: 400 })
  assert.equal(changed.serviceDue, 1000, 'preview preserves the historical settled service amount instead of inventing another settlement')
  assert.equal(fulfillmentFeeSummary({ freight: { estimate_known: false } }).estimated, null)
})

test('order detail separates a repeated specification suffix from the product name', () => {
  assert.equal(fulfillmentItemProductName({ product_name: '商品 227g袋装', bom_spec_name: '227g袋装' }), '商品')
  assert.equal(fulfillmentItemProductName({ product_name: '227g特别拼配', bom_spec_name: '227g袋装' }), '227g特别拼配')
})

test('fulfillment list fees expose paid service and estimated freight without inventing receivables', () => {
  const lines = fulfillmentListFeeLines({ total_amount: 0, grand_total: 0, freight: { service_fee_cents: 1400, service_due_cents: 0, estimated_total_cents: 810, actual_cost_cents: 900, billed_cents: 900 } })
  assert.ok(lines.some(line => line.label === '服务费结算' && line.value === '已结清'))
  assert.ok(lines.some(line => line.label === '预估运费' && line.value === '8.10'))
  assert.ok(lines.some(line => line.label === '实际运费' && line.value === '9.00'))
  assert.equal(lines.some(line => /待收/.test(line.value)), false)
  const discounted = fulfillmentListFeeLines({ total_amount: 120, discount_amount: 15, rounding_amount: 0.2, grand_total: 104.8, shipping_amount: 0 })
  assert.ok(discounted.some(line => line.label === '商品货款' && line.value === '104.80'))
  assert.ok(fulfillmentListFeeLines({}).some(line => line.label === '代发服务费' && line.value === '待读取'))
})


test('historical line and order discounts use frozen net amounts without offering a new discount choice', () => {
  const original = { item_id: 21, product_id: 7, bom_spec_id: 19, bom_variant_id: 30, unit_price: 30, qty: 4,
    line_amount: 115, base_line_amount: 120, discount_type: 'amount', discount_value: 5, discount_amount: 5 }
  const context = { fulfillment_mode: 'on_demand', goods_total_cents: 10480, items: [original] }
  assert.deepEqual(fulfillmentLinePreview(original, original), { amount: 115, discountAmount: 5, requiresDiscountReview: false })
  assert.equal(fulfillmentDraftGoodsCents(context, [original]), 10480)
  assert.deepEqual(fulfillmentLinePreview({ ...original, qty: 5 }, original), { amount: 150, discountAmount: 5, requiresDiscountReview: true })
  assert.equal(fulfillmentDraftGoodsCents(context, [{ ...original, qty: 5 }]), 13980)
  assert.equal(fulfillmentLinePreview({ ...original, product_id: 8 }, original).amount, 120)
  assert.equal(fulfillmentLinePreview({ ...original, bom_spec_id: 20, unit_price: 60 }, original).amount, 240)
  assert.equal(fulfillmentLinePreview({ ...original, item_id: 0 }, original).amount, 120)
  assert.equal(fulfillmentLinePreview({ ...original, bom_variant_id: 31 }, original).amount, 120)
  assert.equal(fulfillmentLinePreview(original, original, 'stock').amount, 0)
  assert.equal(fulfillmentDraftGoodsCents({ fulfillment_mode: 'on_demand', items: [original] }, [original]), 11500, 'older edit contexts fall back to their frozen net line sum')
  assert.equal(fulfillmentSavedGoodsCents({ total_amount: 120, grand_total: 104.8, discount_amount: 15, rounding_amount: 0.2, shipping_amount: 0 }), 10480)
  assert.equal(fulfillmentSavedGoodsCents({ total_amount: 120, grand_total: 0, discount_amount: 120 }), 0, 'explicit fully discounted orders stay free')
  assert.equal(fulfillmentSavedGoodsCents({ total_amount: 120, grand_total: 0 }), 12000, 'historical missing grand totals retain their goods amount')
})

test('canonical fulfillment detail rows retain existing weight and price-source snapshots by exact item identity', () => {
  const rich = { item_id: 21, product_id: 7, spec: '旧规格', unit: '旧单位', logistics_unit_weight_g: 270, logistics_item_weight_g: 1080,
    weight_snapshot_json: '{"qty":4}', price_source_json: '{"publication_id":80}' }
  const [merged, added] = mergeFulfillmentDetailItems([rich], [{ item_id: 21, product_id: 7, spec_name: '227g袋装', inventory_unit: '袋', line_amount: 115 }, { item_id: 22, product_id: 8, spec_label: '454g袋装', sales_unit: '袋', line_amount: 60 }])
  assert.equal(merged.spec, '227g袋装')
  assert.equal(merged.unit, '袋')
  assert.equal(merged.logistics_item_weight_g, 1080)
  assert.equal(merged.weight_snapshot_json, rich.weight_snapshot_json)
  assert.equal(merged.price_source_json, rich.price_source_json)
  assert.equal(added.spec, '454g袋装')
  assert.equal(added.unit, '袋')
  assert.equal(added.price_source_json, undefined, 'another item cannot inherit a historical price source or weight')
})
