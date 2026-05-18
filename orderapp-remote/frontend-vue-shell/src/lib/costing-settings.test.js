import test from 'node:test'
import assert from 'node:assert/strict'

import {
  categorySummaries,
  enrichCostingSetting,
  groupCostingSettings,
} from './costing-settings.js'

const rows = [
  { key: 'roast_yield_rate', label: '生豆到熟豆转化率', value: 0.8, unit: 'ratio' },
  { key: 'retail_tax_rate', label: '零售税率', value: 0.03, unit: 'ratio' },
  { key: 'kg_to_lb_factor', label: 'kg 到 lb 换算', value: 0.454, unit: 'lb/kg' },
  { key: 'wholesale_kg_margin_rate_2', label: '商用熟豆 14包-23包 利润系数', value: 0.38, unit: 'ratio' },
  { key: 'retail_bean_margin_rate', label: '零售熟豆利润系数', value: 0.6, unit: 'ratio' },
  { key: 'drip_process_cost_per_bag', label: '挂耳加工成本', value: 0.44, unit: '元/袋' },
  { key: 'retail_drip_multiplier', label: '零售挂耳利润系数', value: 2.5, unit: 'ratio' },
  { key: 'wholesale_drip_multiplier_1', label: '商用挂耳 100 包利润系数', value: 2.1, unit: 'ratio' },
  { key: 'unknown_adjustment', label: '临时调整', value: 1, unit: '元' },
]

test('groupCostingSettings hides deprecated yield and margin settings from quick settings', () => {
  const groups = groupCostingSettings(rows)
  const keys = groups.flatMap((group) => group.rows.map((row) => row.key))

  assert.deepEqual(keys.filter((key) => key.includes('margin_rate') || key.includes('multiplier')), [])
  assert.equal(keys.includes('roast_yield_rate'), false)
})

test('groupCostingSettings groups editable settings by business category and keeps configured order', () => {
  const groups = groupCostingSettings(rows)

  assert.deepEqual(groups.map((group) => group.key), [
    'base',
    'retailBeans',
    'dripBags',
    'other',
  ])
  assert.equal(groups[0].title, '基础换算')
  assert.equal(groups[0].rows[0].key, 'kg_to_lb_factor')
  assert.equal(groups[1].rows[0].key, 'retail_tax_rate')
  assert.equal(groups[3].rows[0].description, '未归类参数，请确认公式用途后再调整。')
})

test('enrichCostingSetting adds operator guidance without changing API values', () => {
  const got = enrichCostingSetting(rows[1])

  assert.equal(got.key, 'retail_tax_rate')
  assert.equal(got.label, '零售税率')
  assert.equal(got.value, 0.03)
  assert.match(got.description, /零售熟豆价格/)
})

test('categorySummaries exposes short category descriptions for UI headers', () => {
  assert.match(categorySummaries.commercialBeans, /商用熟豆/)
  assert.match(categorySummaries.dripBags, /挂耳/)
})
