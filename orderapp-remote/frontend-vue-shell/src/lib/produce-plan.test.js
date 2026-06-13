import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import * as producePlan from './produce-plan.js'

import {
  buildProductionPlanCreatePayload,
  buildProductionDemandSelection,
  buildProductionDemandSummaryQuery,
  productionPlanSubmitEndpoint,
  buildInsufficientSelection,
  insufficientSelectionState,
  productionPlanOperationSplitsEndpoint,
  buildProductionPlanOperationSplitPayload,
  plannedCapacitySplitMetrics,
  productionPlanSplitBatchCards,
  productionDemandSelectable,
  productionDemandSelectionState,
  productionDemandStatusLabel,
  productionDemandStatusTone,
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

test('production demand status helpers only allow unplanned shortage rows to be selected', () => {
  const demandRows = [
    { product_id: 1, spec_g: 454, gap_g: 454, demand_status: 'unplanned' },
    { product_id: 2, spec_g: 227, gap_g: 227, demand_status: 'in_production' },
    { product_id: 3, spec_g: 100, gap_g: 0, demand_status: 'completed' },
  ]

  assert.equal(productionDemandStatusLabel('unplanned'), '待计划')
  assert.equal(productionDemandStatusLabel('in_production'), '生产中')
  assert.equal(productionDemandStatusLabel('completed'), '生产完成')
  assert.equal(productionDemandStatusTone('in_production'), 'in-production')
  assert.equal(productionDemandSelectable(demandRows[0]), true)
  assert.equal(productionDemandSelectable(demandRows[1]), false)
  assert.equal(productionDemandSelectable(demandRows[2]), false)
  assert.deepEqual(buildProductionDemandSelection(demandRows, true), { '1-454': true })
  assert.deepEqual(productionDemandSelectionState(demandRows, { '1-454': true, '2-227': true }), {
    checked: true,
    indeterminate: false,
    selectedCount: 1,
    total: 1,
  })
})

test('production demand summary query carries demand status filters and selected plan preview keys', () => {
  assert.equal(
    buildProductionDemandSummaryQuery({
      from: '2026-06-01',
      to: '2026-06-13',
      customer_id: '9',
      demand_status: 'in_production',
    }, true, ['1-454', '2-227']),
    '/api/produce/unproduced?from=2026-06-01&to=2026-06-13&customer_id=9&demand_status=in_production&plan=1&selected=1-454%2C2-227',
  )
  assert.equal(
    buildProductionDemandSummaryQuery({ demand_status: 'bad', customer_id: '0' }, false, []),
    '/api/produce/unproduced',
  )
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

test('production plan operation capacity split helpers derive batch count from assigned quantity', () => {
  assert.equal(productionPlanOperationSplitsEndpoint({ id: 41 }), '/api/production-plans/41/operation-splits')
  assert.equal(productionPlanOperationSplitsEndpoint({ id: 0 }), '')

  assert.deepEqual(plannedCapacitySplitMetrics({
    planned_qty: 90,
    batch_size_qty: 18,
    batch_size_unit: 'kg',
    standard_minutes: 15,
    hourly_rate: 300,
  }), {
    planned_batch_count: 5,
    planned_qty: 90,
    planned_qty_g: 90000,
    planned_minutes: 75,
    planned_operation_cost: 375,
  })

  assert.deepEqual(plannedCapacitySplitMetrics({
    planned_qty: 20,
    batch_size_qty: 18,
    batch_size_unit: 'kg',
    standard_minutes: 15,
    hourly_rate: 300,
  }), {
    planned_batch_count: 2,
    planned_qty: 20,
    planned_qty_g: 20000,
    planned_minutes: 30,
    planned_operation_cost: 150,
  })

  assert.deepEqual(buildProductionPlanOperationSplitPayload([
    { production_plan_item_id: 51, operation_seq: 10, operation: '烘焙', workstation_capacity_id: 8, planned_qty: 90 },
    { production_plan_item_id: 51, operation_seq: 10, operation: '烘焙', workstation_capacity_id: 9, planned_qty: 8 },
    { production_plan_item_id: 0, operation: '忽略', planned_qty: 1 },
  ]), {
    items: [
      { production_plan_item_id: 51, operation_seq: 10, operation: '烘焙', workstation_capacity_id: 8, planned_qty: 90 },
      { production_plan_item_id: 51, operation_seq: 10, operation: '烘焙', workstation_capacity_id: 9, planned_qty: 8 },
    ],
  })
})

test('production plan operation capacity split helper renders batch cards without splitting records', () => {
  assert.deepEqual(productionPlanSplitBatchCards({
    workstation_capacity_name: '布勒 18kg',
    planned_qty: 72,
    batch_size_qty: 18,
    batch_size_unit: 'kg',
    standard_minutes: 15,
    hourly_rate: 300,
  }), [
    { label: '第1批', workstation_capacity_name: '布勒 18kg', batch_size_qty: 18, batch_size_unit: 'kg', planned_qty: 18, planned_qty_g: 18000, planned_minutes: 15, underfilled: false },
    { label: '第2批', workstation_capacity_name: '布勒 18kg', batch_size_qty: 18, batch_size_unit: 'kg', planned_qty: 18, planned_qty_g: 18000, planned_minutes: 15, underfilled: false },
    { label: '第3批', workstation_capacity_name: '布勒 18kg', batch_size_qty: 18, batch_size_unit: 'kg', planned_qty: 18, planned_qty_g: 18000, planned_minutes: 15, underfilled: false },
    { label: '第4批', workstation_capacity_name: '布勒 18kg', batch_size_qty: 18, batch_size_unit: 'kg', planned_qty: 18, planned_qty_g: 18000, planned_minutes: 15, underfilled: false },
  ])

  assert.deepEqual(productionPlanSplitBatchCards({
    workstation_capacity_name: '布勒 18kg',
    planned_qty: 20,
    batch_size_qty: 18,
    batch_size_unit: 'kg',
    standard_minutes: 15,
    hourly_rate: 300,
  }), [
    { label: '第1批', workstation_capacity_name: '布勒 18kg', batch_size_qty: 18, batch_size_unit: 'kg', planned_qty: 18, planned_qty_g: 18000, planned_minutes: 15, underfilled: false },
    { label: '第2批', workstation_capacity_name: '布勒 18kg', batch_size_qty: 18, batch_size_unit: 'kg', planned_qty: 2, planned_qty_g: 2000, planned_minutes: 15, underfilled: true },
  ])
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
  assert.match(source, /saveCurrentPlanOperationSplits/)
  assert.match(source, /submitCurrentProductionPlan/)
  assert.match(source, /buildCurrentProductionPlanSubmitPayload\(currentPlan\.value\)/)
  assert.match(source, /apiSend\(productionPlanBatchSubmitEndpoint\(\), \{ body: payload \}\)/)
  assert.doesNotMatch(source, /@click="submitPlanRow\(plan\)"/)
})

test('ProducePlanView owns operation capacity splits after draft plan creation', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  for (const marker of [
    '工序产能拆分',
    '添加拆分',
    'productionPlanOperationSplitsEndpoint',
    'buildProductionPlanOperationSplitPayload',
    'plannedCapacitySplitMetrics',
    'manufacturing-workstation-capacities',
    '承担产量',
    '自动批次数',
    'planned_qty',
    'planned_batch_count',
    'planned_qty_g',
    'planned_minutes',
    'planned_operation_cost',
    '布勒 18kg',
    '智烘 4kg',
    'productionPlanSplitBatchCards',
    'split-batch-cards',
    'split-batch-card',
    '不足标准批量',
  ]) {
    assert.match(source, new RegExp(marker))
  }
  assert.doesNotMatch(source, /每锅数量/)
  assert.doesNotMatch(source, /推荐机器/)
})

test('ProducePlanView lets operators expand planning tables and drag horizontal overflow', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  for (const marker of [
    'demandPanelCollapsed',
    'currentPlanPanelCollapsed',
    'toggleDemandPanelCollapsed',
    'toggleCurrentPlanPanelCollapsed',
    'startTableScrollDrag',
    'drag-scroll-wrap',
    '收起待生产需求',
    '展开待生产需求',
    '收起当前生产计划',
    '展开当前生产计划',
    'demand-collapsed',
    'current-plan-collapsed',
  ]) {
    assert.match(source, new RegExp(marker))
  }
  assert.match(source, /overscroll-behavior:\s*auto/)
  assert.doesNotMatch(source, /overscroll-behavior:\s*contain/)
})

test('ProducePlanView maintains production demand statuses and filters planned rows out of selection', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  for (const marker of [
    'demandStatusFilter',
    'demandStatusOptions',
    'demand_status',
    '需求状态',
    '待计划',
    '生产中',
    '生产完成',
    'productionDemandSelectable',
    'productionDemandStatusLabel',
    'productionDemandStatusTone',
    'productionDemandSelectionKey',
    'isProductionDemandSelected',
    'status-demand-in-production',
    '已进入生产计划的需求不可重复生成计划',
  ]) {
    assert.match(source, new RegExp(marker))
  }
  assert.match(source, /row\.demand_status \|\| 'unplanned'/)
  assert.doesNotMatch(source, /selected\[rowKey\(row\)\]/)
})

test('ProducePlanView explains operation capacity splits before the draft plan exists', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /创建草稿生产计划后可填写工序产能拆分/)
  assert.match(source, /先点创建生产计划，生成草稿后再选择工位产能和承担产量/)
  assert.match(source, /拆分会在提交生成工单前保存/)
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
