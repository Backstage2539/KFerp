import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildInsufficientSelection,
  gramsToKgString,
  insufficientSelectionState,
  normalizeRoastPlans,
  normalizedYieldRate,
  roastExpectedFinishedG,
  syncRoastPlanRow,
} from './produce-plan.js'

const rows = [
  { product_id: 1, spec_g: 454 },
  { product_id: 2, spec_g: 227 },
  { product_id: 3, spec_g: 100 },
]

test('insufficient selection state shows unchecked, checked, and indeterminate header states', () => {
  assert.deepEqual(insufficientSelectionState(rows, {}), {
    checked: false,
    indeterminate: false,
    selectedCount: 0,
    total: 3,
  })

  assert.deepEqual(insufficientSelectionState(rows, { '1-454': true }), {
    checked: false,
    indeterminate: true,
    selectedCount: 1,
    total: 3,
  })

  assert.deepEqual(insufficientSelectionState(rows, { '1-454': true, '2-227': true, '3-100': true }), {
    checked: true,
    indeterminate: false,
    selectedCount: 3,
    total: 3,
  })
})

test('buildInsufficientSelection selects all insufficient rows or clears them', () => {
  assert.deepEqual(buildInsufficientSelection(rows, true), {
    '1-454': true,
    '2-227': true,
    '3-100': true,
  })
  assert.deepEqual(buildInsufficientSelection(rows, false), {})
})

test('normalizeRoastPlans normalizes batch fields and recomputes final input', () => {
  const plans = normalizeRoastPlans([
    { key: '1-454', machine: '  A机  ', batch_g: 0, batch_count: 0, final_input_g: 999 },
    { key: '2-227', machine: '', batch_g: 1200.2, batch_count: 2.4, final_input_g: 0 },
  ])

  assert.deepEqual(plans, [
    { key: '1-454', machine: 'A机', batch_g: 1, batch_count: 1, final_input_g: 1 },
    { key: '2-227', machine: '', batch_g: 1200, batch_count: 2, final_input_g: 2400 },
  ])
})

test('syncRoastPlanRow allows changing machine and batch count in place', () => {
  const row = { machine: '旧机器', batch_g: 1500, batch_count: 1, final_input_g: 1500 }

  syncRoastPlanRow(row, { machine: '新机器', batch_count: 3 })

  assert.equal(row.machine, '新机器')
  assert.equal(row.batch_g, 1500)
  assert.equal(row.batch_count, 3)
  assert.equal(row.final_input_g, 4500)
})

test('normalizedYieldRate supports ratio and percent style inputs', () => {
  assert.equal(normalizedYieldRate(0.815), 0.815)
  assert.equal(normalizedYieldRate(81.5), 0.815)
  assert.equal(normalizedYieldRate(0), 0)
})

test('roastExpectedFinishedG follows editable final_input_g and yield_rate', () => {
  assert.equal(roastExpectedFinishedG({ final_input_g: 13370, yield_rate: 0.815 }), 10897)
  assert.equal(roastExpectedFinishedG({ final_input_g: 4000, yield_rate: 82 }), 3280)
  assert.equal(roastExpectedFinishedG({ final_input_g: 0, yield_rate: 0.815 }), 0)
})

test('gramsToKgString keeps roast output display stable', () => {
  assert.equal(gramsToKgString(10897), '10.90')
  assert.equal(gramsToKgString(571), '0.57')
  assert.equal(gramsToKgString(0), '0')
})
