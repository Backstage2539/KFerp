import test from 'node:test'
import assert from 'node:assert/strict'

import {
  categorySummaries,
  enrichCostingSetting,
  groupCostingSettings,
} from './costing-settings.js'

const rows = [
  { key: 'retail_tax_rate', label: '零售税率', value: 0.03, unit: 'ratio' },
  { key: 'kg_to_lb_factor', label: 'kg 到 lb 换算', value: 0.454, unit: 'lb/kg' },
  { key: 'wholesale_kg_margin_rate_2', label: '商用熟豆 14包-23包 利润系数', value: 0.38, unit: 'ratio' },
  { key: 'drip_process_cost_per_bag', label: '挂耳加工成本', value: 0.44, unit: '元/袋' },
  { key: 'unknown_adjustment', label: '临时调整', value: 1, unit: '元' },
]

test('groupCostingSettings groups known settings by business category and keeps configured order', () => {
  const groups = groupCostingSettings(rows)

  assert.deepEqual(groups.map((group) => group.key), [
    'base',
    'commercialBeans',
    'retailBeans',
    'dripBags',
    'other',
  ])
  assert.equal(groups[0].title, '基础换算')
  assert.equal(groups[0].rows[0].key, 'kg_to_lb_factor')
  assert.equal(groups[1].rows[0].key, 'wholesale_kg_margin_rate_2')
  assert.equal(groups[4].rows[0].description, '未归类参数，请确认公式用途后再调整。')
})

test('enrichCostingSetting adds operator guidance without changing API values', () => {
  const got = enrichCostingSetting(rows[0])

  assert.equal(got.key, 'retail_tax_rate')
  assert.equal(got.label, '零售税率')
  assert.equal(got.value, 0.03)
  assert.match(got.description, /零售熟豆价格/)
})

test('categorySummaries exposes short category descriptions for UI headers', () => {
  assert.match(categorySummaries.commercialBeans, /商用熟豆/)
  assert.match(categorySummaries.dripBags, /挂耳/)
})
