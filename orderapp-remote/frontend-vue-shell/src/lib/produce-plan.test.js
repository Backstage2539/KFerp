import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import * as producePlan from './produce-plan.js'

import {
  buildProductionPlanCreatePayload,
  buildProductionPlanNextActions,
  buildProductionDemandSelection,
  buildProductionDemandSummaryQuery,
  currentProductionPlanStep,
  productionPlanSubmitEndpoint,
  productionPlanSteps,
  productionPlanOperationSplitsPreviewEndpoint,
  buildInsufficientSelection,
  insufficientSelectionState,
  productionPlanOperationSplitsEndpoint,
  buildProductionPlanOperationSplitPayload,
  buildOperationCapacityAutoSplits,
  capacityDefaultPlannedQty,
  maxAssignableQtyForCapacitySplit,
  operationCapacityAutoSplitError,
  plannedCapacitySplitMetrics,
  productionMaterialQuantity,
  productionPlanSplitBatchCards,
  qtyFromGForCapacityUnit,
  productionDemandSelectable,
  productionDemandSelectionState,
  defaultProductionDemandStatusFilter,
  productionDemandPanelEmptyText,
  productionDemandPanelTitle,
  productionDemandStatusFilterValue,
  productionDemandStatusLabel,
  productionDemandStatusTone,
  operationSplitPreviewStatusLabel,
  operationSplitPreviewStatusTone,
  productionPlanItemQuantitySummary,
  productionPlanItemOutputTargetG,
  productionPlanBomSummary,
  productionPlanLegacyGramLabel,
  productionPlanItemBomSourceLabel,
} from './produce-plan.js'

const rows = [
  { product_id: 1, spec_g: 454 },
  { product_id: 2, spec_g: 227 },
  { product_id: 3, spec_g: 100 },
]

test('production plan item summary uses frozen sales-spec conversion and parent BOM source', () => {
  const item = {
    sales_spec_count: 4,
    inventory_qty_per_sales_unit: 0.454,
    inventory_unit: 'kg',
    planned_inventory_qty: 1.816,
    bom_inherited: true,
    bom_source_product_id: 644,
    bom_version_id: 1337,
  }

  assert.equal(productionPlanItemQuantitySummary(item), '4件、0.454Kg/件、合计1.816Kg')
  assert.equal(productionPlanItemBomSourceLabel(item), 'BOM版本 #1337 · 继承父商品BOM')
  assert.equal(
    productionPlanItemQuantitySummary({ spec_g: 454 }),
    '454g',
    '历史计划行保持旧规格投影',
  )
  assert.equal(productionPlanLegacyGramLabel(1816), '1816g')
  assert.equal(productionPlanLegacyGramLabel(0), '0g')
})

test('production plan BOM summary hides legacy yield and only shows configured BOM loss', () => {
  assert.equal(productionPlanBomSummary({}), '默认 BOM')
  assert.equal(productionPlanBomSummary({ bom_material_loss_rate: 0 }), '默认 BOM')
  assert.equal(productionPlanBomSummary({ bom_material_loss_rate: 0.12 }), 'BOM原料损耗 12.00%')
  assert.equal(productionPlanBomSummary({ bom_material_loss_rate: 0.18 }), 'BOM原料损耗 18.00%')
  assert.equal(productionPlanBomSummary({ bom_material_loss_rate: 0.2 }), 'BOM原料损耗 20.00%')
  assert.equal(productionPlanBomSummary({ bom_material_loss_rate: 1 }), '默认 BOM')
  assert.equal(productionPlanBomSummary({ bom_material_loss_rate: 0, bom_summary_error: 'product BOM not configured' }), 'BOM 配置待完善')
})

test('production plan detail labels legacy gram projections instead of showing ambiguous bare numbers', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  assert.match(source, /productionPlanLegacyGramLabel\(item\.gap_g\)/)
  assert.match(source, /productionPlanLegacyGramLabel\(item\.planned_g\)/)
  assert.match(source, /productionPlanLegacyGramLabel\(item\.planned_output_g\)/)
  assert.doesNotMatch(source, /<td>\{\{\s*item\.(?:gap_g|planned_g|planned_output_g)\s*\|\|\s*0\s*\}\}<\/td>/)
})

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
    {
      product_id: 4,
      spec_g: 0,
      gap_g: 100,
      demand_status: 'unplanned',
      demand_selectable: false,
      blocking_reason: '销售单位“件”无法换算到库存单位“盒”',
    },
  ]

  assert.equal(productionDemandStatusLabel('unplanned'), '待计划')
  assert.equal(productionDemandStatusLabel('in_production'), '生产中')
  assert.equal(productionDemandStatusLabel('completed'), '生产完成')
  assert.equal(productionDemandStatusTone('in_production'), 'in-production')
  assert.equal(defaultProductionDemandStatusFilter(), 'unplanned')
  assert.equal(productionDemandStatusFilterValue('bad', defaultProductionDemandStatusFilter()), 'unplanned')
  assert.equal(productionDemandPanelTitle(''), '生产需求')
  assert.equal(productionDemandPanelTitle('unplanned'), '待计划需求')
  assert.equal(productionDemandPanelEmptyText('completed'), '暂无生产完成需求')
  assert.equal(productionDemandSelectable(demandRows[0]), true)
  assert.equal(productionDemandSelectable(demandRows[1]), false)
  assert.equal(productionDemandSelectable(demandRows[2]), false)
  assert.equal(productionDemandSelectable(demandRows[3]), false)
  assert.deepEqual(buildProductionDemandSelection(demandRows, true), { '1-454': true })
  assert.deepEqual(productionDemandSelectionState(demandRows, { '1-454': true, '2-227': true, '4-0': true }), {
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

test('production plan cancel endpoint only targets a concrete plan', () => {
  assert.equal(
    producePlan.productionPlanCancelEndpoint({ id: 41 }),
    '/api/production-plans/41/cancel',
  )
  assert.equal(producePlan.productionPlanCancelEndpoint({ id: 0 }), '')
  assert.equal(producePlan.productionPlanCancelEndpoint(null), '')
})

test('production plan cancel only resets a workbench showing the same draft', () => {
  assert.equal(producePlan.productionPlanCancelTargetsCurrentPlan({ id: 41 }, { id: 41 }), true)
  assert.equal(producePlan.productionPlanCancelTargetsCurrentPlan({ id: 41 }, { id: 42 }), false)
  assert.equal(producePlan.productionPlanCancelTargetsCurrentPlan({ id: 41 }, null), false)
  assert.equal(producePlan.productionPlanCancelTargetsCurrentPlan({ id: 0 }, { id: 0 }), false)
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

test('production plan stepper shows the current next operation without changing backend workflow', () => {
  assert.deepEqual(productionPlanSteps().map((step) => step.label), [
    '选需求',
    '生成草稿',
    '拆分产能',
    '提交工单',
    '开始生产',
  ])

  assert.equal(currentProductionPlanStep({ selectedCount: 0, plan: null, splitCount: 0 }), 'selectDemand')
  assert.equal(currentProductionPlanStep({ selectedCount: 2, plan: null, splitCount: 0 }), 'createDraft')
  assert.equal(currentProductionPlanStep({ selectedCount: 2, plan: { status: 'draft' }, splitCount: 0 }), 'splitCapacity')
  assert.equal(currentProductionPlanStep({ selectedCount: 2, plan: { status: 'draft' }, splitCount: 2 }), 'submitWorkOrders')
  assert.equal(currentProductionPlanStep({ plan: { status: 'submitted' }, splitCount: 2 }), 'startProduction')
})

test('submitted production plan exposes next-step actions to work orders, job cards, assignment, and WIP issue', () => {
  const actions = buildProductionPlanNextActions({
    success: [{
      plan: { id: 41, plan_no: 'PP-0000000041', status: 'submitted' },
      work_orders: [{ id: 88, work_order_no: 'WO-PP-41' }],
      job_cards: [{ id: 91, work_order_id: 88 }],
    }],
  })

  assert.deepEqual(actions.map((action) => [action.key, action.label, action.view, action.params]), [
    ['workOrders', '打开工单', 'workOrders', { work_order_id: 88 }],
    ['jobCards', '打开工序卡', 'jobCards', { job_card_id: 91, work_order_id: 88 }],
    ['assignWorkstation', '分配工位', 'productionOverview', { work_order_id: 88, job_card_id: 91 }],
    ['issueWip', '生产领料', 'stockOperations', { tab: 'stockEntries', action: 'issue', return_source: 'work_order', work_order_id: 88, job_card_id: 91 }],
  ])
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

test('piece-costed operation uses completed pieces instead of batches or workstation hours', () => {
  assert.deepEqual(plannedCapacitySplitMetrics({
    cost_method: 'piece',
    piece_rate: 0.5,
    planned_qty: 100,
    batch_size_qty: 20,
    batch_size_unit: '件',
    standard_minutes: 10,
    hourly_rate: 300,
    spec_g: 227,
  }), {
    planned_batch_count: 5,
    planned_qty: 100,
    planned_qty_g: 22700,
    planned_minutes: 50,
    planned_operation_cost: 50,
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

test('operation capacity auto split only uses capacities applicable to the current operation', () => {
  const operation = { seq: 1, operation_id: 7, operation: '烘焙' }
  const item = { id: 51, planned_g: 23000, spec_g: 1000 }
  const capacities = [
    { id: 10, workstation_id: 1, name: '布勒10kg', status: 'active', batch_size_qty: 10, batch_size_unit: 'kg', standard_minutes: 10, hourly_rate: 300, applicable_operation_ids: [7] },
    { id: 3, workstation_id: 2, name: '智烘3kg', status: 'active', batch_size_qty: 3, batch_size_unit: 'kg', standard_minutes: 15, hourly_rate: 180, applicable_operation_ids: [7] },
    { id: 100, workstation_id: 3, name: '包装100袋', status: 'active', batch_size_qty: 100, batch_size_unit: '袋', standard_minutes: 20, hourly_rate: 90, applicable_operation_ids: [8] },
    { id: 99, workstation_id: 4, name: '旧未配置产能', status: 'active', batch_size_qty: 23, batch_size_unit: 'kg', standard_minutes: 20, hourly_rate: 120 },
    { id: 98, workstation_id: 4, name: '停用产能', status: 'inactive', batch_size_qty: 23, batch_size_unit: 'kg', standard_minutes: 20, hourly_rate: 120, applicable_operation_ids: [7] },
  ]

  assert.deepEqual(buildOperationCapacityAutoSplits(item, operation, capacities).map((row) => ({
    capacity_id: row.workstation_capacity_id,
    planned_qty: row.planned_qty,
    planned_qty_g: plannedCapacitySplitMetrics(row).planned_qty_g,
  })), [
    { capacity_id: 10, planned_qty: 20, planned_qty_g: 20000 },
    { capacity_id: 3, planned_qty: 3, planned_qty_g: 3000 },
  ])
})

test('operation capacity auto split uses closest capacity for the last underfilled batch', () => {
  const rows = buildOperationCapacityAutoSplits(
    { id: 51, planned_g: 21000, spec_g: 1000 },
    { seq: 1, operation_id: 7, operation: '烘焙' },
    [
      { id: 10, workstation_id: 1, name: '布勒10kg', status: 'active', batch_size_qty: 10, batch_size_unit: 'kg', applicable_operation_ids: [7] },
      { id: 3, workstation_id: 2, name: '智烘3kg', status: 'active', batch_size_qty: 3, batch_size_unit: 'kg', applicable_operation_ids: [7] },
    ],
  )

  assert.deepEqual(rows.map((row) => [row.workstation_capacity_id, row.planned_qty]), [[10, 20], [3, 1]])
  assert.equal(productionPlanSplitBatchCards(rows[1])[0].underfilled, true)
})

test('operation capacity auto split falls back to planned output or demand gap when planned grams are absent', () => {
  const operation = { seq: 1, operation_id: 7, operation: '烘焙' }
  const capacities = [
    { id: 10, workstation_id: 1, name: '布勒10kg', status: 'active', batch_size_qty: 10, batch_size_unit: 'kg', applicable_operation_ids: [7] },
    { id: 3, workstation_id: 2, name: '智烘3kg', status: 'active', batch_size_qty: 3, batch_size_unit: 'kg', applicable_operation_ids: [7] },
  ]

  assert.deepEqual(
    buildOperationCapacityAutoSplits({ id: 51, planned_output_g: 23000, spec_g: 1000 }, operation, capacities)
      .map((row) => [row.workstation_capacity_id, row.planned_qty]),
    [[10, 20], [3, 3]],
  )

  assert.deepEqual(
    buildOperationCapacityAutoSplits({ id: 51, gap_g: 21000, spec_g: 1000 }, operation, capacities)
      .map((row) => [row.workstation_capacity_id, row.planned_qty]),
    [[10, 20], [3, 1]],
  )
})

test('operation capacity auto split reports why no rows were generated', () => {
  const operation = { seq: 1, operation_id: 7, operation: '烘焙' }
  const capacities = [
    { id: 10, status: 'active', batch_size_qty: 10, batch_size_unit: 'kg', applicable_operation_ids: [7] },
  ]

  assert.equal(
    operationCapacityAutoSplitError({ id: 51, planned_g: 0, spec_g: 454 }, operation, capacities),
    '当前计划行缺少计划产量，无法自动拆分',
  )
  assert.equal(
    operationCapacityAutoSplitError({ id: 51, planned_g: 1000, spec_g: 454 }, { seq: 2, operation_id: 8, operation: '包装' }, capacities),
    '当前工序没有可用的工位产能，或工位产能未绑定该工序',
  )
})

test('operation capacity auto split supports count-based packaging capacity through spec grams', () => {
  const rows = buildOperationCapacityAutoSplits(
    { id: 52, planned_g: 10442, spec_g: 454 },
    { seq: 2, operation_id: 8, operation: '包装' },
    [
      { id: 100, workstation_id: 3, name: '包装10袋', status: 'active', batch_size_qty: 10, batch_size_unit: '袋', applicable_operation_ids: [8] },
      { id: 30, workstation_id: 4, name: '手工3袋', status: 'active', batch_size_qty: 3, batch_size_unit: '袋', applicable_operation_ids: [8] },
      { id: 10, workstation_id: 1, name: '布勒10kg', status: 'active', batch_size_qty: 10, batch_size_unit: 'kg', applicable_operation_ids: [7] },
    ],
  )

  assert.deepEqual(rows.map((row) => [row.workstation_capacity_id, row.planned_qty, plannedCapacitySplitMetrics(row).planned_qty_g]), [
    [100, 20, 9080],
    [30, 3, 1362],
  ])
  assert.equal(qtyFromGForCapacityUnit(10442, '袋', 454), 23)
})

test('operation capacity auto split freezes piece costing fields', () => {
  const rows = buildOperationCapacityAutoSplits(
    { id: 52, planned_g: 22700, spec_g: 227 },
    { seq: 2, operation_id: 8, operation: '包装' },
    [{
      id: 100,
      workstation_id: 3,
      name: '包装100件',
      status: 'active',
      cost_method: 'piece',
      piece_rate: 0.5,
      batch_size_qty: 100,
      batch_size_unit: '件',
      standard_minutes: 20,
      hourly_rate: 90,
      applicable_operation_ids: [8],
    }],
  )

  assert.equal(rows.length, 1)
  assert.equal(rows[0].cost_method, 'piece')
  assert.equal(rows[0].piece_rate, 0.5)
  assert.equal(plannedCapacitySplitMetrics(rows[0]).planned_operation_cost, 50)
})

test('piece capacity uses frozen sales-spec count when legacy spec grams are unavailable', () => {
  const rows = buildOperationCapacityAutoSplits(
    { id: 53, planned_g: 200, spec_g: 0, sales_spec_count: 100 },
    { seq: 3, operation_id: 9, operation: '包装' },
    [{
      id: 101,
      workstation_id: 3,
      name: '包装20件',
      status: 'active',
      cost_method: 'piece',
      piece_rate: 0.5,
      batch_size_qty: 20,
      batch_size_unit: '件',
      standard_minutes: 5,
      applicable_operation_ids: [9],
    }],
  )

  assert.equal(rows.length, 1)
  assert.equal(rows[0].planned_qty, 100)
  assert.equal(rows[0].sales_spec_count, 100)
  assert.equal(plannedCapacitySplitMetrics(rows[0]).planned_qty_g, 200)
  assert.equal(plannedCapacitySplitMetrics(rows[0]).planned_operation_cost, 50)
})

test('count capacity projects finished output while weight capacity keeps loss-adjusted input', () => {
  const item = {
    id: 54,
    planned_g: 25795,
    planned_output_g: 22700,
    sales_spec_count: 100,
    inventory_qty_per_sales_unit: 0.227,
    inventory_unit: 'kg',
    planned_inventory_qty: 22.7,
    spec_g: 0,
  }
  assert.equal(productionPlanItemOutputTargetG(item), 22700)

  const packageRows = buildOperationCapacityAutoSplits(item, {
    seq: 3,
    operation_id: 9,
    operation: '包装',
  }, [{
    id: 101,
    workstation_id: 3,
    name: '包装20件',
    status: 'active',
    cost_method: 'piece',
    piece_rate: 0.5,
    batch_size_qty: 20,
    batch_size_unit: '件',
    standard_minutes: 5,
    applicable_operation_ids: [9],
  }])
  assert.equal(packageRows.length, 1)
  assert.equal(packageRows[0].planned_qty, 100)
  assert.equal(packageRows[0].item_target_g, 22700)
  assert.equal(plannedCapacitySplitMetrics(packageRows[0]).planned_qty_g, 22700)
  assert.equal(plannedCapacitySplitMetrics(packageRows[0]).planned_operation_cost, 50)

  const roastRows = buildOperationCapacityAutoSplits(item, {
    seq: 1,
    operation_id: 7,
    operation: '烘焙',
  }, [{
    id: 102,
    workstation_id: 1,
    name: '烘焙10kg',
    status: 'active',
    batch_size_qty: 10,
    batch_size_unit: 'kg',
    standard_minutes: 30,
    hourly_rate: 24,
    applicable_operation_ids: [7],
  }])
  assert.deepEqual(roastRows.map((row) => row.planned_qty), [25.795])
  assert.equal(roastRows.reduce((sum, row) => sum + plannedCapacitySplitMetrics(row).planned_qty_g, 0), 25795)
})

test('production material quantities keep non-weight purchase suggestions in material units', () => {
  assert.equal(productionMaterialQuantity({ unit: 'g', purchase_suggestion_g: 1500, qty: 21 }, 'purchase_suggestion_g'), 1500)
  assert.equal(productionMaterialQuantity({ unit: '个', purchase_suggestion_g: 0, available_g: 0, raw_g: 0, qty: 21 }, 'purchase_suggestion_g'), 21)
  assert.equal(productionMaterialQuantity({ unit: '个', purchase_suggestion_g: 5, qty: 21 }, 'purchase_suggestion_g'), 5)
})

test('single split capacity allocation fills the maximum full batches from remaining quantity', () => {
  const target = { planned_g: 23000, spec_g: 1000 }
  const existing = [{
    production_plan_item_id: 51,
    operation_seq: 1,
    operation_id: 7,
    operation: '烘焙',
    workstation_capacity_id: 10,
    batch_size_qty: 10,
    batch_size_unit: 'kg',
    planned_qty: 10,
    spec_g: 1000,
  }]
  const split = {
    production_plan_item_id: 51,
    operation_seq: 1,
    operation_id: 7,
    operation: '烘焙',
    workstation_capacity_id: 10,
    batch_size_qty: 10,
    batch_size_unit: 'kg',
    spec_g: 1000,
  }

  assert.equal(maxAssignableQtyForCapacitySplit(split, existing, target), 10)
  assert.equal(maxAssignableQtyForCapacitySplit({ ...split, batch_size_qty: 3, batch_size_unit: 'kg' }, existing, target), 12)
  assert.equal(maxAssignableQtyForCapacitySplit({ ...split, batch_size_qty: 30, batch_size_unit: '袋' }, [], { planned_g: 10442, spec_g: 454 }), 23)
})

test('production plan drawer follows selected workstation capacity batch size', () => {
  assert.equal(capacityDefaultPlannedQty({ batch_size_qty: 12, batch_size_unit: 'kg' }), 12)
  assert.equal(capacityDefaultPlannedQty({ batch_size_qty: '1.412', batch_size_unit: 'kg' }), 1.412)
  assert.equal(capacityDefaultPlannedQty({ batch_size_qty: 0, batch_size_unit: 'kg' }), 0)

  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  assert.match(source, /split\.planned_qty = capacityDefaultPlannedQty\(capacity\)/)
  assert.doesNotMatch(source, /if \(Number\(split\.planned_qty \|\| 0\) <= 0\)[\s\S]{0,160}split\.planned_qty/)
})

test('production plan operation split preview endpoint and status display are explicit', () => {
  assert.equal(productionPlanOperationSplitsPreviewEndpoint({ id: 41 }), '/api/production-plans/41/operation-splits/preview')
  assert.equal(productionPlanOperationSplitsPreviewEndpoint({}), '')
  assert.equal(operationSplitPreviewStatusLabel('matched'), '已覆盖')
  assert.equal(operationSplitPreviewStatusLabel('short'), '不足')
  assert.equal(operationSplitPreviewStatusLabel('over'), '超排')
  assert.equal(operationSplitPreviewStatusLabel('missing'), '未安排')
  assert.equal(operationSplitPreviewStatusTone('matched'), 'matched')
  assert.equal(operationSplitPreviewStatusTone('short'), 'short')
  assert.equal(operationSplitPreviewStatusTone('over'), 'over')
  assert.equal(operationSplitPreviewStatusTone('missing'), 'missing')
})

test('production plan split drawer renders live demand gap preview', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  for (const want of [
    '产能安排总览',
    '用料需求差距',
    '实际需求',
    '已安排',
    '差距',
    'productionPlanSplitPreview',
    'scheduleProductionPlanSplitPreview',
    'operationSplitPreviewStatusTone',
  ]) {
    assert.match(source, new RegExp(want))
  }
})

test('ProducePlanView creates draft plans and batch submits checked draft plans', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /生成草稿/)
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

test('ProducePlanView uses the top workflow action and removes the duplicate current-plan create button', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  const workbenchIndex = source.indexOf('planning-workbench')
  const currentPlanIndex = source.indexOf('当前生产计划')
  const historyIndex = source.indexOf('生产计划单据')

  assert.ok(workbenchIndex > 0, 'production page should have a planning workbench')
  assert.match(source, /待生产需求/)
  assert.ok(currentPlanIndex > 0, 'current plan workspace should be visible')
  assert.ok(historyIndex > currentPlanIndex, 'history list should be below the current plan workspace')
  assert.match(source, /@click="runPlanNextStep"/)
  assert.doesNotMatch(source, /<button[^>]*@click="createProductionPlan"[^>]*>创建生产计划<\/button>/)
  assert.match(source, /@click="submitCurrentProductionPlan"[^>]*>提交当前计划生成工单<\/button>/)
  assert.match(source, /@click="cancelProductionPlanDraft\(currentPlan, 'current'\)"[^>]*>撤销草稿<\/button>/)
  assert.doesNotMatch(source, /选择库存不足商品后点击“创建生产计划”/)
  assert.match(source, /勾选库存不足商品后生成计划预览/)
})

test('ProducePlanView opens the existing split-capacity drawer immediately after draft creation', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const createStart = source.indexOf('async function createProductionPlan()')
  const createEnd = source.indexOf('async function submitCurrentProductionPlan()', createStart)
  const createSource = source.slice(createStart, createEnd)

  assert.ok(createStart > 0 && createEnd > createStart, 'createProductionPlan function should exist')
  assert.match(createSource, /currentPlan\.value = await apiSend\('\/api\/production-plans'/)
  assert.match(createSource, /await openCurrentPlanSplitDrawer\(\)/)
  assert.doesNotMatch(createSource, /loadProductionPlanOperationSplits/, 'unsaved auto splits must not advance the step before the drawer opens')
})

test('ProducePlanView renders BOM expected loss without the removed expected-yield label', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /\{\{\s*productionPlanBomSummary\(row\)\s*\}\}/)
  assert.match(source, /:title="row\.bom_summary_error \|\| ''"/)
  assert.doesNotMatch(source, /<th>计划投料\(g\)<\/th>/)
  assert.doesNotMatch(source, /<td>\{\{\s*row\.input_g\s*\}\}<\/td>/)
  assert.doesNotMatch(source, /预期产出率/)
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

test('ProducePlanView cancels draft plans and refreshes returned production demand', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  for (const marker of [
    '撤销草稿',
    'cancelProductionPlanDraft',
    'productionPlanCancelEndpoint(plan)',
    'productionPlanCancelTargetsCurrentPlan',
    'window.confirm',
    'replaceSelected({})',
    'defaultProductionDemandStatusFilter()',
    'refreshProductionDemandAfterDraftCancel',
    'loadProductionPlans()',
  ]) {
    assert.ok(source.includes(marker), `missing ${marker}`)
  }
  assert.match(source, /v-if="currentPlanDraft"[\s\S]*@click="cancelProductionPlanDraft\(currentPlan, 'current'\)"/)
  assert.match(source, /v-if="productionPlanSelectable\(plan\)"[\s\S]*@click="cancelProductionPlanDraft\(plan, 'list'\)"/)
  assert.match(source, /v-if="productionPlanSelectable\(productionPlanDetail\)"[\s\S]*@click="cancelProductionPlanDraft\(productionPlanDetail, 'detail'\)"/)
  assert.match(source, /previewError\.value = err\.message \|\| '撤销生产计划草稿失败'/)
  assert.match(source, /productionPlanDetailError\.value = err\.message \|\| '撤销生产计划草稿失败'/)
  assert.match(source, /if \(cancelledCurrentPlan\) \{[\s\S]*replaceSelected\(\{\}\)[\s\S]*planRows\.value = \[\]/)
  assert.match(source, /refreshProductionDemandAfterDraftCancel\(!cancelledCurrentPlan\)/)
  assert.match(source, /let demandRequestSeq = 0/)
  assert.match(source, /if \(requestID !== demandRequestSeq\) return/)
  assert.match(source, /:disabled="saving \|\| loading">撤销草稿/)
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
    'autoSplitProductionPlanDrawerOperation',
    'ensureWorkstationCapacities',
    'applicableOperationCapacities',
    '自动拆分',
  ]) {
    assert.match(source, new RegExp(marker))
  }
  assert.doesNotMatch(source, /assignRemainingCurrentPlanSplitQty/)
  assert.doesNotMatch(source, /assignRemainingProductionPlanDrawerSplitQty/)
  assert.doesNotMatch(source, /分配剩余产量/)
  assert.doesNotMatch(source, /分配剩余产能/)
  assert.doesNotMatch(source, /每锅数量/)
  assert.doesNotMatch(source, /推荐机器/)
})

test('ProducePlanView edits draft plan splits in a drawer instead of the current plan workspace', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  for (const marker of [
    '@click="handlePlanStepClick(step.key)"',
    'openCurrentPlanSplitDrawer',
    'openProductionPlanSplitDrawer',
    'closeProductionPlanSplitDrawer',
    'saveProductionPlanSplitDrawer',
    'productionPlanSplitDrawer',
    'production-plan-split-drawer',
    'normalizeProductionPlanDetailForSplitEditor',
    'detailOperationsFallback',
    '编辑拆分',
    'productionPlanSelectable(plan)',
    'productionPlanSplitRows.value = withAutoOperationSplits((detail.operation_splits || []).map(normalizeOperationSplit), detail)',
  ]) {
    assert.ok(source.includes(marker), `missing ${marker}`)
  }

  assert.doesNotMatch(source, /operation-split-placeholder/)
  assert.doesNotMatch(source, /operation-split-panel/)
  assert.doesNotMatch(source, /autoSplitCurrentPlanOperation/)
  assert.doesNotMatch(source, /创建草稿生产计划后可填写工序产能拆分/)
  assert.doesNotMatch(source, /先点创建生产计划，生成草稿后再选择工位产能和承担产量/)
  assert.doesNotMatch(source, /@click="loadProductionPlanIntoCurrentEditor\(plan\)"/)
  assert.doesNotMatch(source, /currentPlan\.value = detail/)
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
    'demandPanelTitle',
    '收起\\$\\{demandPanelTitle\\}',
    '展开\\$\\{demandPanelTitle\\}',
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
    'blocking_reason',
    '资料待完善',
  ]) {
    assert.match(source, new RegExp(marker))
  }
  assert.match(source, /row\.demand_status \|\| 'unplanned'/)
  assert.doesNotMatch(source, /selected\[rowKey\(row\)\]/)
})

test('ProducePlanView keeps operation capacity splitting out of the current plan preview', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  const workbenchStart = source.indexOf("<section :class=\"['planning-workbench'")
  const listStart = source.indexOf('<section class="panel">\n      <div class="section-title">库存充足')
  assert.ok(workbenchStart > 0, 'missing planning workbench start')
  assert.ok(listStart > workbenchStart, 'missing inventory-sufficient panel after planning workbench')
  const currentPlanWorkbench = source.slice(workbenchStart, listStart)

  assert.doesNotMatch(currentPlanWorkbench, /工序产能拆分/)
  assert.doesNotMatch(currentPlanWorkbench, /保存拆分/)
  assert.doesNotMatch(currentPlanWorkbench, /添加拆分/)
  assert.match(source, /草稿计划在这里补充或调整工位产能拆分，不占用当前生产计划工作台。/)
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
