import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

import {
  buildProductionPlanCreatePayload,
  productionPlanSubmitEndpoint,
  buildInsufficientSelection,
  insufficientSelectionState,
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

test('buildProductionPlanCreatePayload creates a generic draft plan and lets backend default input', () => {
  const payload = buildProductionPlanCreatePayload(
    { from: '2026-06-01', to: '2026-06-30', customer_id: '9' },
    ['1-227', '2-454'],
  )

  assert.deepEqual(payload, {
    from: '2026-06-01',
    to: '2026-06-30',
    customer_id: 9,
    source_type: 'erp_order',
    selected: ['1-227', '2-454'],
  })
  assert.equal(Object.prototype.hasOwnProperty.call(payload, 'input_by_key'), false)
})

test('productionPlanSubmitEndpoint points submit action at the formal production plan API', () => {
  assert.equal(productionPlanSubmitEndpoint({ id: 41 }), '/api/production-plans/41/submit')
  assert.equal(productionPlanSubmitEndpoint({}), '')
})

test('ProducePlanView creates and submits formal production plans before work order start', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /创建生产计划/)
  assert.match(source, /提交生成工单/)
  assert.match(source, /apiSend\('\/api\/production-plans'/)
  assert.match(source, /productionPlanSubmitEndpoint\(plan\)/)
  assert.doesNotMatch(source, /apiSend\('\/api\/produce\/start'/)
})

test('ProducePlanView no longer consumes roasting capacity suggestions in the main flow', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  for (const forbidden of [
    '生产建议',
    '推荐机器',
    '每锅数量',
    '锅数',
    '最终投料数',
    '预计成品',
    '/api/produce/machines',
    'roastPlans',
    'machineRows',
    'syncRoastPlan',
  ]) {
    assert.doesNotMatch(source, new RegExp(forbidden))
  }
})

test('ProducePlanView does not leave selected rows with a disabled no-op create button', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.doesNotMatch(source, /:disabled="saving \|\| !planReady"/)
  assert.match(source, /if \(!planReady\.value\) \{[\s\S]*await load\(true\)/)
})
