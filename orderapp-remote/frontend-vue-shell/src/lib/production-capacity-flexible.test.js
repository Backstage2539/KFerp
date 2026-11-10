import test from 'node:test'
import assert from 'node:assert/strict'
import * as plan from './produce-plan.js'
import { buildWorkOrderOperationSplitPayload } from './work-orders.js'

const item = { id: 11, planned_g: 45113, spec_g: 1000, sales_spec_count: 45.113 }
const capacity = { id: 7, workstation_id: 2, workstation: '布勒', name: '15kg', batch_size_qty: 15, batch_size_unit: 'kg', standard_minutes: 18, hourly_rate: 102, applicable_operation_ids: [9], status: 'active' }
const row = { ...capacity, production_plan_item_id: 11, operation_seq: 1, operation_id: 9, workstation_capacity_id: 7, planned_qty: 45, planned_batch_count: 3, batch_input: true }

test('capacity under and over are advisory while absent operations remain unarranged', () => {
  for (const status of ['short', 'over', 'matched']) assert.equal(plan.productionPlanCapacityReadiness({ operation_coverage: [{ status }] }).can_confirm, true)
  assert.equal(plan.productionPlanCapacityReadiness({ operation_coverage: [{ status: 'missing' }] }).can_confirm, false)
})

test('custom batch calculation recommends ceil and shows the lower whole-batch alternative', () => {
  for (const qty of [18.1, 18.5, 18.7]) {
    const custom = plan.withCustomCapacityBatchSize({ ...row, batch_count_source: 'auto' }, String(qty), [], item, capacity)
    assert.equal(custom.planned_batch_count, 3)
    assert.equal(plan.plannedCapacitySplitMetrics(custom).planned_qty, Number((qty * 3).toFixed(3)))
    assert.equal(plan.productionPlanSplitBatchCards(custom)[0].planned_qty, qty)
  }
  const custom = plan.withCustomCapacityBatchSize({ ...row, batch_count_source: 'auto' }, '18.7', [], item, capacity)
  assert.deepEqual(plan.capacityBatchRecommendation(custom, [custom], item), { batch_count: 3, planned_qty: 56.1, diff_qty: 10.987, lower_batch_count: 2, lower_planned_qty: 37.4, lower_diff_qty: -7.713, remaining_qty: 45.113, unit: 'kg' })
  const second = { ...row, local_key: 'other', planned_batch_count: 1, planned_qty: 15 }
  assert.equal(plan.capacityBatchRecommendation(custom, [custom, second], item).batch_count, 2)
})

test('custom values survive batch edits, preset refresh, plan/work-order payloads and loaded rows', () => {
  const custom = plan.withCustomCapacityBatchSize({ ...row, batch_count_source: 'manual' }, '18.7', [], item, capacity)
  const edited = plan.withCapacityBatchCount(custom, '2', capacity)
  assert.equal(edited.batch_size_qty, 18.7)
  assert.equal(edited.planned_qty, 37.4)
  assert.equal(plan.applyCapacityBatchSnapshot(edited, { ...capacity, batch_size_qty: 16 }, true).batch_size_qty, 18.7)
  const changedSize = plan.withCustomCapacityBatchSize(edited, '18.5', [], item, capacity)
  assert.equal(changedSize.planned_batch_count, '2')
  assert.equal(changedSize.planned_qty, 37)
  for (const build of [plan.buildProductionPlanOperationSplitPayload, buildWorkOrderOperationSplitPayload]) {
    const payload = build([edited]).items[0]
    assert.equal(payload.custom_batch_size_qty, 18.7)
    assert.equal(payload.planned_batch_count, 2)
    assert.equal(Object.hasOwn(payload, 'planned_qty'), false)
  }
  assert.equal(plan.withCustomCapacityBatchSize(row, '18.7', [], item, capacity).planned_batch_count, 3)
})

test('invalid custom quantities are not rounded, omitted or saved', () => {
  for (const value of ['', 0, -1, 'abc', Infinity, '18.7001']) {
    const custom = { ...row, custom_batch_size_qty: value }
    assert.match(plan.capacitySplitInputError(custom), /自定义每批数量/)
    assert.throws(() => plan.buildProductionPlanOperationSplitPayload([custom]), /自定义每批数量/)
    assert.throws(() => buildWorkOrderOperationSplitPayload([custom]), /自定义每批数量/)
  }
})

test('editing a custom size never repairs or replaces a manually cleared batch count', () => {
  const cleared = plan.withCapacityBatchCount({ ...row, custom_batch_size_qty: 18.7, batch_size_qty: 18.7 }, '')
  const resized = plan.withCustomCapacityBatchSize(cleared, '18.5', [], item, capacity)
  assert.equal(resized.planned_batch_count, '')
  assert.match(plan.capacitySplitInputError(resized), /正整数/)
})
