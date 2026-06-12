import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import * as producePlan from './produce-plan.js'

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

test('production plan detail endpoint targets the formal plan document', () => {
  assert.equal(producePlan.productionPlanDetailEndpoint({ id: 41 }), '/api/production-plans/41')
  assert.equal(producePlan.productionPlanDetailEndpoint({ id: '42' }), '/api/production-plans/42')
  assert.equal(producePlan.productionPlanDetailEndpoint({}), '')
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

test('current production plan submit payload reuses the batch submit contract with one id', () => {
  assert.equal(typeof producePlan.buildCurrentProductionPlanSubmitPayload, 'function')
  assert.deepEqual(producePlan.buildCurrentProductionPlanSubmitPayload({ id: 41 }), { ids: [41] })
  assert.deepEqual(producePlan.buildCurrentProductionPlanSubmitPayload({ id: '42' }), { ids: [42] })
  assert.deepEqual(producePlan.buildCurrentProductionPlanSubmitPayload({ id: 0 }), { ids: [] })
  assert.deepEqual(producePlan.buildCurrentProductionPlanSubmitPayload(null), { ids: [] })
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

test('ProducePlanView uses an ERPNext-style current plan workspace instead of top create and bottom details', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  const workbenchIndex = source.indexOf('planning-workbench')
  const currentPlanIndex = source.indexOf('当前生产计划')
  const createIndex = source.indexOf('创建生产计划')
  const historyIndex = source.indexOf('生产计划单据')
  const topPanel = source.slice(source.indexOf('<h2>生产计划</h2>'), workbenchIndex)

  assert.ok(workbenchIndex > 0, 'production page should have a planning workbench')
  assert.match(source, /待生产需求/)
  assert.ok(currentPlanIndex > 0, 'current plan workspace should be visible')
  assert.ok(createIndex > currentPlanIndex, 'create action should live in the current plan workspace')
  assert.ok(historyIndex > currentPlanIndex, 'history list should be below the current plan workspace')
  assert.doesNotMatch(topPanel, /创建生产计划/)
  assert.doesNotMatch(source, /选择库存不足商品后点击“创建生产计划”/)
  assert.match(source, /勾选库存不足商品后生成计划预览/)
})

test('ProducePlanView automatically loads selected demand into the current plan preview', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /loadSelectedPlanPreview/)
  assert.match(source, /schedulePlanPreview/)
  assert.match(source, /selectedSignature/)
  assert.match(source, /url\.searchParams\.set\('plan', '1'\)/)
  assert.match(source, /url\.searchParams\.set\('selected', keys\.join\(','\)\)/)
  assert.match(source, /previewError/)
})

test('ProducePlanView submits the current draft plan through the batch submit API', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /提交当前计划生成工单/)
  assert.match(source, /submitCurrentProductionPlan/)
  assert.match(source, /buildCurrentProductionPlanSubmitPayload\(currentPlan\.value\)/)
  assert.match(source, /apiSend\(productionPlanBatchSubmitEndpoint\(\), \{ body: payload \}\)/)
  assert.doesNotMatch(source, /@click="submitPlanRow\(plan\)"/)
})

test('ProducePlanView opens an ERPNext-style production plan detail drawer from the compact list', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /production-plan-detail-drawer/)
  assert.match(source, /openProductionPlanDetail/)
  assert.match(source, /productionPlanDetailEndpoint\(plan\)/)
  assert.match(source, /apiGet\(productionPlanDetailEndpoint\(plan\)\)/)
  assert.match(source, /单据头/)
  assert.match(source, /计划行/)
  assert.match(source, /物料需求汇总/)
  assert.match(source, /工艺路线摘要/)
  assert.match(source, /工艺参数 \/ 商品生产配置快照/)
  assert.match(source, /生成结果/)
  assert.match(source, />详情</)
  assert.doesNotMatch(source, /submitPlanRow\(plan\)/)
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
  assert.match(source, /if \(!planReady\.value\) \{[\s\S]*await loadSelectedPlanPreview\(\)/)
})
