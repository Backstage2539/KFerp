import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

test('ProductionCostsView exposes phase3 trace variance and abnormal loss analytics', () => {
  const source = readFileSync(new URL('../views/ProductionCostsView.vue', import.meta.url), 'utf8')
  for (const marker of [
    '/api/production-trace/analytics',
    '追溯链路',
    '成本差异',
    '异常损耗',
    'trace_links',
    'cost_variance',
    'abnormal_losses',
  ]) {
    assert.ok(source.includes(marker), `ProductionCostsView.vue should include ${marker}`)
  }
})

test('ProductionCostsView includes operation planned versus actual cost markers', () => {
  const source = readFileSync(new URL('../views/ProductionCostsView.vue', import.meta.url), 'utf8')
  for (const marker of [
    '计划工序成本',
    '实际工序成本',
    'planned_operation_cost',
    'actual_operation_cost',
  ]) {
    assert.ok(source.includes(marker), `ProductionCostsView.vue should include ${marker}`)
  }
})
