import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import * as producePlan from './produce-plan.js'

import {
  buildProductionPlanCreatePayload,
  productionPlanSubmitEndpoint,
  buildInsufficientSelection,
  describeProducePlanRow,
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

test('describeProducePlanRow summarizes drip bag production and upstream shortage', () => {
  const labels = describeProducePlanRow({
    product_name: '蓝山挂耳',
    production_kind: 'drip_bag',
    need_bags: 20,
    upstream_roast_demand_g: 150,
    upstream_shortage_g: 110,
    finished_product_component_shortage_g: 110,
  })

  assert.deepEqual(labels, ['挂耳生产', '需求 20 袋', '熟豆组件缺口 110g', '上游烘焙需求 150g'])
})

test('buildProductionPlanCreatePayload creates a formal draft plan instead of starting production', () => {
  const payload = buildProductionPlanCreatePayload(
    { from: '2026-06-01', to: '2026-06-30', customer_id: '9' },
    ['1-227', '2-454'],
    [{ key: '1-227', final_input_g: 600 }],
    [
      { product_id: 1, spec_g: 227, input_g: 580 },
      { product_id: 2, spec_g: 454, input_g: 1200 },
    ],
  )

  assert.deepEqual(payload, {
    from: '2026-06-01',
    to: '2026-06-30',
    customer_id: 9,
    source_type: 'erp_order',
    selected: ['1-227', '2-454'],
    input_by_key: {
      '1-227': 600,
      '2-454': 1200,
    },
  })
})

test('productionPlanSubmitEndpoint points submit action at the formal production plan API', () => {
  assert.equal(productionPlanSubmitEndpoint({ id: 41 }), '/api/production-plans/41/submit')
  assert.equal(productionPlanSubmitEndpoint({}), '')
})

test('production plan list query includes status and date filters with a 50 row default', () => {
  assert.equal(
    producePlan.buildProductionPlanListQuery({
      status: 'submitted',
      time_field: 'submitted_at',
      from: '2026-06-01',
      to: '2026-06-11',
    }),
    '/api/production-plans?status=submitted&time_field=submitted_at&from=2026-06-01&to=2026-06-11&limit=50',
  )
  assert.equal(
    producePlan.buildProductionPlanListQuery({ status: '', time_field: 'invalid', limit: 20 }),
    '/api/production-plans?time_field=created_at&limit=20',
  )
})

test('production plan status labels and tones are localized for the list', () => {
  assert.equal(producePlan.productionPlanStatusLabel('draft'), '草稿')
  assert.equal(producePlan.productionPlanStatusLabel('submitted'), '已提交工单')
  assert.equal(producePlan.productionPlanStatusLabel('in_progress'), '生产中')
  assert.equal(producePlan.productionPlanStatusLabel('completed'), '已完成')
  assert.equal(producePlan.productionPlanStatusLabel('cancelled'), '已取消')

  assert.equal(producePlan.productionPlanStatusTone('draft'), 'draft')
  assert.equal(producePlan.productionPlanStatusTone('submitted'), 'submitted')
  assert.equal(producePlan.productionPlanStatusTone('in_progress'), 'in-progress')
  assert.equal(producePlan.productionPlanStatusTone('completed'), 'completed')
  assert.equal(producePlan.productionPlanStatusTone('cancelled'), 'cancelled')
})

test('production plan selection only targets draft plans and reports tri-state header state', () => {
  const plans = [
    { id: 41, status: 'draft' },
    { id: 42, status: 'submitted' },
    { id: 43, status: 'draft' },
    { id: 44, status: 'completed' },
  ]

  assert.equal(producePlan.productionPlanSelectable(plans[0]), true)
  assert.equal(producePlan.productionPlanSelectable(plans[1]), false)
  assert.deepEqual(producePlan.buildProductionPlanSelection(plans, true), { 41: true, 43: true })
  assert.deepEqual(producePlan.buildProductionPlanSelection(plans, false), {})
  assert.deepEqual(producePlan.productionPlanSelectionState(plans, { 41: true, 42: true }), {
    checked: false,
    indeterminate: true,
    selectedCount: 1,
    total: 2,
  })
  assert.deepEqual(producePlan.productionPlanSelectionState(plans, { 41: true, 43: true }), {
    checked: true,
    indeterminate: false,
    selectedCount: 2,
    total: 2,
  })
})

test('production plan batch submit payload keeps only positive selected ids', () => {
  assert.equal(producePlan.productionPlanBatchSubmitEndpoint(), '/api/production-plans/submit')
  assert.deepEqual(
    producePlan.buildProductionPlanBatchSubmitPayload({ 41: true, 42: false, 'bad-id': true, 43: true }),
    { ids: [41, 43] },
  )
})

test('ProducePlanView creates draft plans and batch submits checked draft plans', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /创建生产计划/)
  assert.match(source, /提交生成工单/)
  assert.match(source, /apiSend\('\/api\/production-plans'/)
  assert.match(source, /productionPlanBatchSubmitEndpoint\(\)/)
  assert.match(source, /selectedProductionPlans/)
  assert.doesNotMatch(source, />生成计划</)
  assert.doesNotMatch(source, /@click="buildPlan"/)
  assert.doesNotMatch(source, /submitPlanRow\(plan\)/)
  assert.doesNotMatch(source, /请先选择产品并点击“生成计划”/)
  assert.doesNotMatch(source, /apiSend\('\/api\/produce\/start'/)
})

test('ProducePlanView does not leave selected rows with a disabled no-op create button', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.doesNotMatch(source, /:disabled="saving \|\| !planReady"/)
  assert.match(source, /if \(!planReady\.value\) \{[\s\S]*await load\(true\)/)
})
