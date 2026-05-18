import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildPriceExplanationRequest,
  gradientDisplayUnitLabel,
  normalizeGradientTemplate,
  validateGradientTemplate,
} from './gradient-templates.js'

test('normalizeGradientTemplate keeps display labels separate from system gram ranges', () => {
  const got = normalizeGradientTemplate({
    name: ' 工厂量单 ',
    display_unit: 'kg',
    tiers: [
      { label: '50kg+', min_weight_g: '50000', max_weight_g: '', margin_rate: '0.12', position: 2 },
      { label: '24-49kg', min_weight_g: '24000', max_weight_g: '49000', margin_rate: '0.175', position: 1 },
    ],
  })

  assert.equal(got.name, '工厂量单')
  assert.equal(got.display_unit, 'kg')
  assert.deepEqual(got.tiers.map((tier) => tier.label), ['24-49kg', '50kg+'])
  assert.equal(got.tiers[0].min_weight_g, 24000)
  assert.equal(got.tiers[0].max_weight_g, 49000)
  assert.equal(got.tiers[1].max_weight_g, null)
})

test('validateGradientTemplate rejects missing gram ranges and mixed invalid values', () => {
  assert.deepEqual(validateGradientTemplate({
    name: '',
    display_unit: 'kg',
    tiers: [{ label: '', min_weight_g: 0, max_weight_g: -1, margin_rate: -1 }],
  }), [
    '请填写模板名称',
    '第 1 档请填写区间名',
    '第 1 档最小总克重必须大于 0',
    '第 1 档最大总克重必须大于最小总克重',
    '第 1 档利润率不能为负数',
  ])
})

test('buildPriceExplanationRequest creates a temporary what-if payload without saving settings', () => {
  const item = {
    product_id: 501,
    name: '模板拼配',
    green_bean_cost_per_kg: 51.75,
    gradient_template: { id: 9, name: '工厂量单模板', display_unit: 'kg', tiers: [] },
  }
  const got = buildPriceExplanationRequest(item, { label: '24-49kg' }, { margin_rate: 0.3 })

  assert.equal(got.product.product_id, 501)
  assert.equal(got.tier_label, '24-49kg')
  assert.deepEqual(got.overrides, { margin_rate: 0.3 })
  assert.equal(got.save, undefined)
})

test('gradientDisplayUnitLabel returns operator-facing unit text', () => {
  assert.equal(gradientDisplayUnitLabel('kg'), '元/kg')
  assert.equal(gradientDisplayUnitLabel('lb'), '元/磅')
  assert.equal(gradientDisplayUnitLabel('bad'), '元/磅')
})
