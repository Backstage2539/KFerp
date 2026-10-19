import test from 'node:test'
import assert from 'node:assert/strict'
import { fulfillmentSettingsForm, fulfillmentSettingsPayload, shippingTemplateForm, shippingTemplatePayload, customerFinishedWarehouses } from './fulfillment-settings.js'

test('fulfillment settings preserve both modes and require explicit fee amounts', () => {
  const data = { customer_id: 147, revision: 3, modes: [{ mode: 'stock', enabled: true, warehouse_code: 'own', fee_configured: true, fixed_fee_cents: 300, per_package_fee_cents: 50, shipping_template_id: 8 }] }
  const form = fulfillmentSettingsForm(data)
  assert.equal(form.modes.length, 2)
  assert.equal(form.modes[0].fixed_fee, '3.00')
  assert.equal(fulfillmentSettingsPayload(form).modes[0].per_package_fee_cents, 50)
  form.modes[0].fixed_fee = ''
  assert.throws(() => fulfillmentSettingsPayload(form), /明确填写/)
  form.modes[0].fixed_fee = '0'
  assert.equal(fulfillmentSettingsPayload(form).modes[0].fixed_fee_cents, 0)
})

test('shipping templates retain revision and validate monetary precision', () => {
  const form = shippingTemplateForm({ id: 8, revision: 2, name: '云南', active: true, packaging_weight_g: 200, rules: [{ province: '云南省', first_weight_g: 1000, first_fee_cents: 800, additional_weight_g: 1000, additional_fee_cents: 200 }] })
  const payload = shippingTemplatePayload(form)
  assert.equal(payload.expected_revision, 2)
  assert.equal(payload.rules[0].first_fee_cents, 800)
  form.rules[0].first_fee = '1.001'
  assert.throws(() => shippingTemplatePayload(form), /两位小数/)
})

test('warehouse picker only exposes active customer finished warehouses', () => {
  assert.deepEqual(customerFinishedWarehouses([{ code: 'a', customer_id: 147, kind: 'finished', active: true }, { code: 'b', customer_id: 148, kind: 'finished' }, { code: 'c', customer_id: 147, kind: 'raw' }, { code: 'd', customer_id: 147, kind: 'customer_finished', active: false }], 147).map(r => r.code), ['a'])
})
