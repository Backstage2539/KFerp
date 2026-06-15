import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { visibleProductionTraceLinks } from './production-costs.js'

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

test('visibleProductionTraceLinks hides empty work-order cross-product rows', () => {
  const rows = visibleProductionTraceLinks([
    { work_order_id: 88, job_card_id: 12, stock_entry_id: 0, entry_no: '', entry_type: '', material_name: '', batch_code: '', qty_g: 0 },
    { work_order_id: 88, job_card_id: 12, stock_entry_id: 9, entry_no: 'SE-9', entry_type: 'material_issue', material_name: '豆袋', batch_code: 'MB-BAG', qty_g: 0 },
    { work_order_id: 88, job_card_id: 12, stock_entry_id: 10, entry_no: 'SE-10', entry_type: 'finished_receipt', material_name: '', batch_code: 'FP-10', qty_g: 55706 },
  ])

  assert.deepEqual(rows.map((row) => row.stock_entry_id), [9, 10])
})
