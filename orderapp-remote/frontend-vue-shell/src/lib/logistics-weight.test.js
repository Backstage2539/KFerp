import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { logisticsWeightDraft, logisticsWeightGrams, validateLogisticsWeights, formatWeightG, orderWeightLines } from './logistics-weight.js'

const source = path => readFileSync(new URL(path, import.meta.url), 'utf8')

test('BOM logistics weight keeps manual kilograms and decimal grams without deriving from labels', () => {
  assert.equal(logisticsWeightGrams({ logistics_weight_qty: '0.1', logistics_weight_unit: 'kg' }), 100)
  assert.equal(logisticsWeightGrams({ logistics_weight_qty: '227.125', logistics_weight_unit: 'g' }), 227.125)
  assert.equal(logisticsWeightGrams({ logistics_weight_g: 0.001 }), 0.001)
  assert.equal(logisticsWeightGrams({ name: '454g袋装' }), null)
  assert.equal(logisticsWeightGrams({ logistics_weight_qty: '', logistics_weight_unit: 'kg' }), null)
  assert.deepEqual(logisticsWeightDraft(100), { logistics_weight_qty: 0.1, logistics_weight_unit: 'kg' })
  assert.deepEqual(logisticsWeightDraft(null), { logistics_weight_qty: '', logistics_weight_unit: 'kg' })
  assert.equal(logisticsWeightGrams({ logistics_weight_qty: '1000000', logistics_weight_unit: 'kg' }), 1e9)
  for (const value of ['0.0000001', '1000000000.1']) assert.throws(() => logisticsWeightGrams({ logistics_weight_qty: value, logistics_weight_unit: 'g' }), /物流重量/)
  for (const value of ['0', '-1', 'abc', Infinity, NaN]) assert.throws(() => logisticsWeightGrams({ logistics_weight_qty: value, logistics_weight_unit: 'g' }), /物流重量/)
})

test('publication requires all manual product specification weights and permits incomplete drafts', () => {
  assert.doesNotThrow(() => validateLogisticsWeights([{ name: '盒装', logistics_weight_g: 100 }, { name: '袋装', logistics_weight_g: 227.125 }]))
  assert.throws(() => validateLogisticsWeights([{ name: '盒装', logistics_weight_g: 100 }, { name: '未填规格', logistics_weight_g: null }]), /未填规格.*物流重量/)
})

test('order weights use only frozen values and distinguish reference material weight from logistics weight', () => {
  const item = { qty: 99, material_unit_weight_g: 90, material_item_weight_g: 180, logistics_unit_weight_g: 100, logistics_item_weight_g: 200 }
  assert.deepEqual(orderWeightLines(item), [
    { label: '用料重量（参考）', value: '单件 90g · 本项 180g' },
    { label: '物流重量', value: '单件 100g · 本项 200g' },
  ])
  assert.deepEqual(orderWeightLines({}).map(row => row.value), ['未记录', '未记录'])
  assert.deepEqual(orderWeightLines({ weight_snapshot_json: '{"rule_version":1,"historical_unknown":true}' }).map(row => row.value), ['未记录', '未记录'])
  assert.equal(formatWeightG(null), '未记录')
  assert.equal(formatWeightG(227.125), '227.125g')
  assert.deepEqual(orderWeightLines({ weight_snapshot_json: '{"version":1}' }).map(row => row.value), ['未计算', '未配置'])
  assert.deepEqual(orderWeightLines({ weight_snapshot_json: '{}' }).map(row => row.value), ['未记录', '未记录'])
})

test('manual BOM and specification template editors save weight while ProductCreator inherits read-only', () => {
  const bom = source('../views/BomView.vue')
  assert.ok(/LogisticsWeightEditor[\s\S]*?variant/.test(bom), 'manual/template editors include logistics input')
  assert.ok(/LogisticsWeightEditor[\s\S]*?selectedBomVariant/.test(bom), 'existing BOM variant includes logistics input')
  assert.match(bom, /logistics_weight_g:\s*logisticsWeightGrams\(variant\)/)
  assert.match(bom, /validateLogisticsWeights\(variants\)/)
  for (const path of ['../views/ProductCreatorView.vue', '../views/ProductCreatorRunView.vue']) {
    assert.match(source(path), /物流重量[\s\S]*?formatWeightG\(variant.logistics_weight_g\)/)
  }
})

test('ERP, customer web, fulfillment detail and inline rows show the same read-only frozen weight component', () => {
  for (const path of ['../views/OrdersView.vue', '../views/CustomerAccountView.vue', '../views/CustomerFulfillmentView.vue']) assert.ok(source(path).includes('<OrderItemWeights'), path + ' includes frozen weights')
  assert.match(source('./order-list-inline.js'), /weightSnapshot:\s*item/)
  const component = source('../components/OrderItemWeights.vue')
  assert.match(component, /orderWeightLines/)
  assert.doesNotMatch(component, /v-model|apiSend|fetch\(/)
})

test('ordinary-order freight explains missing logistics weight without showing a zero estimate', () => {
 const component = source('../components/OrderFreightEstimate.vue')
 assert.ok(component.includes('!quote.estimate_known && quote.cost_pending_reason'))
})
