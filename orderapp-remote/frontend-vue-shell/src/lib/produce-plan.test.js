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
  productionGapProductRows,
  productionGapMaterialRows,
  productionGapConclusion,
  productionPlanDraftUpdateEndpoint,
  buildProductionPlanDraftUpdatePayload,
  buildProductionPlanCapacityGroups,
  productionPlanCapacityReadiness,
} from './produce-plan.js'

const rows = [
  { product_id: 1, spec_g: 454 },
  { product_id: 2, spec_g: 227 },
  { product_id: 3, spec_g: 100 },
]

test('production planning workspace has two creation steps', () => {
  assert.deepEqual(productionPlanSteps(), [
    { key: 'selectDemand', label: '选需求' },
    { key: 'reviewGap', label: '核对缺口' },
  ])
  assert.equal(currentProductionPlanStep({ selectedCount: 0 }), 'selectDemand')
  assert.equal(currentProductionPlanStep({ selectedCount: 2 }), 'reviewGap')
  assert.equal(currentProductionPlanStep({ selectedCount: 2, plan: { status: 'draft' } }), 'planDetail')
})

test('gap review groups the same real product specification and preserves order traceability', () => {
  const result = productionGapProductRows([
    { parent_product_id: 9, product_id: 91, product: '初晓-商品', bom_spec_id: 11, spec_label: '227g', sales_unit: '袋', sales_spec_count: 10, gap_sales_spec_count: 10, order_details: [{ order_id: 1, order_no: 'SO-1', customer_name: '甲', quantity: 6, sales_unit: '袋' }, { order_id: 1, order_no: 'SO-1', customer_name: '甲', quantity: 4, sales_unit: '袋' }] },
    { parent_product_id: 9, product_id: 92, product: '初晓-商品', bom_spec_id: 11, spec_label: '227g', sales_unit: '袋', sales_spec_count: 8, gap_sales_spec_count: 8, order_details: [{ order_id: 2, order_no: 'SO-2', customer_name: '乙', quantity: 8, sales_unit: '袋' }] },
  ])
  assert.equal(result.length, 1)
  assert.equal(result[0].need_label, '18 袋')
  assert.equal(result[0].gap_label, '18 袋')
  assert.equal(result[0].order_count, 2)
  assert.deepEqual(result[0].order_details.map((row) => row.order_no), ['SO-1', 'SO-2'])
  assert.deepEqual(result[0].order_details.map((row) => row.quantity), [10, 8])

  const sameNameDifferentProducts = productionGapProductRows([
    { parent_product_id: 19, product: '同名商品', bom_spec_id: 11, spec_label: '227g', sales_unit: '袋', sales_spec_count: 1, gap_sales_spec_count: 1 },
    { parent_product_id: 29, product: '同名商品', bom_spec_id: 11, spec_label: '227g', sales_unit: '袋', sales_spec_count: 1, gap_sales_spec_count: 1 },
  ])
  assert.equal(sameNameDifferentProducts.length, 2)
})

test('gap review presents one unified material decision list', () => {
  const payload = { manufacturing_plan: { nodes: [
    { item: { type: 'material', id: 10, name: '初晓熟豆', unit: 'kg' }, required_qty: 4.54, stock_covered_qty: 0, inflight_covered_qty: 0, shortage_qty: 4.54, action: 'manufacture' },
    { item: { type: 'material', id: 20, name: '咖啡豆袋', unit: '条' }, required_qty: 20, stock_covered_qty: 20, shortage_qty: 0, action: 'inventory' },
    { item: { type: 'material', id: 30, name: '在产熟豆', unit: 'kg' }, required_qty: 5, stock_covered_qty: 1, inflight_covered_qty: 4, shortage_qty: 0, action: 'inventory' },
  ] } }
  const rows = productionGapMaterialRows(payload)
  assert.deepEqual(rows.map((row) => row.status), ['manufacture', 'waiting', 'satisfied'])
  assert.equal(rows[0].required_label, '4.54 kg')
  assert.equal(rows[1].inflight_label, '4 kg')
  assert.equal(rows[2].stock_label, '20 条')
  assert.match(productionGapConclusion([{ product: '初晓-商品', gap_label: '20 袋' }], rows), /20 袋.*初晓熟豆 4.54 kg/)

  const mergedShortage = productionGapMaterialRows({ items: [], supply_gaps: [
    { id: 1, item_type: 'material', item_id: 40, item_name: '纸箱', unit: '个', required_units: 3, status: 'open' },
    { id: 2, item_type: 'material', item_id: 40, item_name: '纸箱', unit: '个', required_units: 2, status: 'open' },
  ] })
  assert.equal(mergedShortage.length, 1)
  assert.equal(mergedShortage[0].required_label, '5 件')
})

test('draft recalculation keeps the same plan identity and carries revision', () => {
  assert.equal(productionPlanDraftUpdateEndpoint({ id: 41, status: 'draft' }), '/api/production-plans/41')
  assert.equal(productionPlanDraftUpdateEndpoint({ id: 41, status: 'submitted' }), '')
  assert.deepEqual(buildProductionPlanDraftUpdatePayload({ revision: 3 }, [' a ', 'b'], 'req-1'), {
    revision: 3,
    request_id: 'req-1',
    selected: ['a', 'b'],
  })
})

test('ProducePlanView renders the business-first gap review without dependency jargon', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  for (const marker of ['核对缺口', '商品缺口', '生产用料', '现货覆盖', '在产覆盖', '剩余缺口', '创建草稿并编辑', '返回选需求']) {
    assert.match(source, new RegExp(marker))
  }
  const reviewStart = source.indexOf('class="gap-review-workspace"')
  const reviewEnd = source.indexOf('<!-- Creation ends here;', reviewStart)
  assert.ok(reviewStart > 0 && reviewEnd > reviewStart)
  const review = source.slice(reviewStart, reviewEnd)
  assert.doesNotMatch(review, /多层制造需求与上游依赖|物料需求汇总（预计消耗）|<th>层级<\/th>|BOM版本/)
})

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

test('BOM-spec production demand uses its inventory quantity and unit instead of grams', () => {
  const row = {
    bom_spec_id: 91,
    inventory_unit: '袋',
    need_inventory_qty: 100,
    available_inventory_qty: 40,
    gap_inventory_qty: 60,
    need_g: 0,
    inv_g: 0,
    gap_g: 0,
    demand_status: 'unplanned',
  }

  assert.equal(producePlan.productionDemandGapQuantity(row), 60)
  assert.equal(producePlan.productionDemandQuantityLabel(row, 'need'), '100 袋')
  assert.equal(producePlan.productionDemandQuantityLabel(row, 'available'), '40 袋')
  assert.equal(producePlan.productionDemandQuantityLabel(row, 'gap'), '60 袋')
  assert.equal(productionDemandSelectable(row), true)
  assert.equal(producePlan.productionDemandQuantityLabel({ need_g: 227 }, 'need'), '227g')
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

test('production plan detail workspace saves all draft sections with one endpoint', () => {
  const plan = {
    id: 41,
    draft_token: 'current-token',
    items: [{ id: 51, target_warehouse: 'finished_goods' }],
    component_sources: [{
      production_plan_item_id: 51,
      component_type: 'material',
      component_id: 7,
      component_bom_spec_id: 0,
      component_spec_g: 0,
      source_warehouse: 'raw_material',
      source_owner_customer_id: 0,
    }],
    operation_splits: [{
      production_plan_item_id: 51,
      operation_seq: 1,
      operation: '包装',
      workstation_capacity_id: 9,
      planned_qty: 20,
    }],
  }

  assert.equal(producePlan.productionPlanDraftEndpoint(plan), '/api/production-plans/41/draft')
  assert.deepEqual(producePlan.buildProductionPlanDraftPayload(plan), {
    draft_token: 'current-token',
    items: [{ id: 51, target_warehouse: 'finished_goods' }],
    component_sources: [{
      production_plan_item_id: 51,
      component_type: 'material',
      component_id: 7,
      component_bom_spec_id: 0,
      component_spec_g: 0,
      source_warehouse: 'raw_material',
      source_owner_customer_id: 0,
    }],
    operation_splits: [{
      production_plan_item_id: 51,
      operation_seq: 1,
      operation: '包装',
      workstation_capacity_id: 9,
      planned_qty: 20,
    }],
  })
})

test('production plan stages keep task identity while connecting a shared upstream item', () => {
  const detail = {
    items: [
      { id: 1, output_type: 'product', output_name: '初晓商品', sales_spec_count: 10, inventory_unit: '袋', order_nos: 'SO-1' },
      { id: 2, output_type: 'product', output_name: '初晓商品', sales_spec_count: 8, inventory_unit: '袋', order_nos: 'SO-2' },
      { id: 3, output_type: 'product', output_name: '初晓商品', sales_spec_count: 2, inventory_unit: '袋', order_nos: 'SO-3' },
      { id: 4, output_type: 'material', output_name: '初晓', output_qty: 4.54, output_unit: 'kg' },
    ],
    manufacturing_plan: {
      edges: [
        { consumer_plan_item_id: 1, supplier_plan_item_id: 4, required_g: 2270 },
        { consumer_plan_item_id: 2, supplier_plan_item_id: 4, required_g: 1816 },
        { consumer_plan_item_id: 3, supplier_plan_item_id: 4, required_g: 454 },
      ],
    },
  }

  const stages = producePlan.buildProductionPlanStages(detail)
  assert.equal(stages.length, 2)
  assert.deepEqual(stages[0].tasks.map((task) => task.id), [4])
  assert.deepEqual(stages[1].tasks.map((task) => task.id), [1, 2, 3])
  assert.equal(stages[0].tasks[0].supplies.length, 3)
  assert.equal(stages[1].tasks[0].quantity_label, '10 袋')
})

test('production plan detail uses a full workspace with fixed header and footer', () => {
  const source = fs.readFileSync(new URL('../components/ProductionPlanDetailWorkspace.vue', import.meta.url), 'utf8')
  assert.match(source, /生产计划详情工作区/)
  assert.match(source, /本次生产安排/)
  assert.match(source, /用料与来源/)
  assert.match(source, /提交前核对/)
  assert.match(source, /保存草稿/)
  assert.match(source, /提交生成工单/)
  assert.match(source, /quantity\(material\.qty, material\.unit\)/)
  assert.match(source, /position:\s*sticky/)
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

test('production plan stepper separates creation from saved documents', () => {
  assert.deepEqual(productionPlanSteps().map((step) => step.label), [
    '选需求',
    '核对缺口',
  ])

  assert.equal(currentProductionPlanStep({ selectedCount: 0, plan: null, splitCount: 0 }), 'selectDemand')
  assert.equal(currentProductionPlanStep({ selectedCount: 2, plan: null, splitCount: 0 }), 'reviewGap')
  assert.equal(currentProductionPlanStep({ selectedCount: 2, plan: { status: 'draft' }, splitCount: 0 }), 'planDetail')
  assert.equal(currentProductionPlanStep({ selectedCount: 2, plan: { status: 'draft' }, splitCount: 2 }), 'planDetail')
  assert.equal(currentProductionPlanStep({ plan: { status: 'submitted' }, splitCount: 2 }), 'planDetail')
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

test('production plan capacity workspace renders live task coverage', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const component = fs.readFileSync(new URL('../components/ProductionPlanCapacityWorkspace.vue', import.meta.url), 'utf8')
  const rendered = `${source}\n${component}`
  for (const want of [
    '拆分核对',
    '计划数量',
    '已安排',
    '还需安排',
    'productionPlanSplitPreview',
    'scheduleProductionPlanSplitPreview',
    'productionPlanCapacityReadiness',
  ]) {
    assert.match(rendered, new RegExp(want))
  }
})

test('ProducePlanView creates draft plans and batch submits checked draft plans', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /创建草稿并编辑/)
  assert.match(source, /提交生成工单/)
  assert.match(source, /apiSend\('\/api\/production-plans'/)
  assert.doesNotMatch(source, /beginDraftRecalculation|draftBeingEdited/)
  assert.match(source, /productionPlanBatchSubmitEndpoint\(\)/)
  assert.match(source, /selectedProductionPlans/)
  assert.doesNotMatch(source, />生成计划</)
  assert.doesNotMatch(source, /@click="buildPlan"/)
  assert.doesNotMatch(source, /submitPlanRow\(plan\)/)
  assert.doesNotMatch(source, /请先选择产品并点击“生成计划”/)
  assert.doesNotMatch(source, /apiSend\('\/api\/produce\/start'/)
})

test('ProducePlanView ends creation at the shared draft workspace', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const workbenchIndex = source.indexOf('planning-workbench')
  const historyIndex = source.indexOf('id="plan-records"')
  assert.ok(workbenchIndex > 0 && historyIndex > workbenchIndex)
  assert.match(source, /@click="runPlanNextStep"/)
  assert.match(source, /await openProductionPlanDetail\(created\)/)
  assert.match(source, /创建草稿并编辑/)
  assert.doesNotMatch(source, /class="schedule-workspace"/)
  assert.doesNotMatch(source, /@click="submitCurrentProductionPlan"/)
  assert.doesNotMatch(source, /@click="cancelProductionPlanDraft\(currentPlan, 'current'\)"/)
})

test('ProducePlanView keeps technical BOM calculations out of the gap decision screen', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const reviewStart = source.indexOf('class="gap-review-workspace"')
  const reviewEnd = source.indexOf('<!-- Creation ends here;', reviewStart)
  const review = source.slice(reviewStart, reviewEnd)
  assert.doesNotMatch(review, /productionPlanBomSummary|bom_summary_error|BOM摘要|工艺路线摘要/)
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

test('ProducePlanView submits through the unified detail action', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  assert.match(source, /@submit="submitProductionPlanDetail"/)
  assert.match(source, /productionPlanSubmitEndpoint\(plan\)/)
  assert.match(source, /productionPlanDetailDirty.value \|\| !plan\?\.readiness\?\.can_submit/)
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
  assert.match(source, /@cancel="cancelProductionPlanDraft/)
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
  const component = fs.readFileSync(new URL('../components/ProductionPlanCapacityWorkspace.vue', import.meta.url), 'utf8')
  const rendered = `${source}\n${component}`

  for (const marker of [
    '工序产能拆分',
    '添加工位',
    'productionPlanOperationSplitsEndpoint',
    'buildProductionPlanOperationSplitPayload',
    'plannedCapacitySplitMetrics',
    'manufacturing-workstation-capacities',
    '承担产量',
    'planned_qty',
    'planned_batch_count',
    'planned_qty_g',
    'planned_minutes',
    'planned_operation_cost',
    'productionPlanSplitBatchCards',
    'batch-summary',
    '尾批',
    'autoSplitProductionPlanDrawerOperation',
    'ensureWorkstationCapacities',
    'applicableOperationCapacities',
    '自动拆分',
  ]) {
    assert.match(rendered, new RegExp(marker))
  }
  assert.doesNotMatch(source, /assignRemainingCurrentPlanSplitQty/)
  assert.doesNotMatch(source, /assignRemainingProductionPlanDrawerSplitQty/)
  assert.doesNotMatch(source, /分配剩余产量/)
  assert.doesNotMatch(source, /分配剩余产能/)
  assert.doesNotMatch(source, /每锅数量/)
  assert.doesNotMatch(source, /推荐机器/)
})

test('ProducePlanView edits draft plan splits in a dedicated full-page workspace', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  for (const marker of [
    '@click="handlePlanStepClick(step.key)"',
    'openCurrentPlanSplitDrawer',
    'openProductionPlanSplitDrawer',
    'closeProductionPlanSplitDrawer',
    'saveProductionPlanSplitDrawer',
    'productionPlanSplitDrawer',
    'ProductionPlanCapacityWorkspace',
    'production-plan-capacity-shell',
    'productionPlanSplitDirty',
    'confirmProductionPlanSplitWorkspace',
    'normalizeProductionPlanDetailForSplitEditor',
    'detailOperationsFallback',
    '编辑拆分',
    'productionPlanSelectable(plan)',
    'productionPlanSplitRows.value = withAutoOperationSplits(savedRows, detail)',
  ]) {
    assert.ok(source.includes(marker), `missing ${marker}`)
  }

  assert.doesNotMatch(source, /production-plan-split-drawer/)
  assert.doesNotMatch(source, /drawer-backdrop production-plan-split-layer/)
  assert.doesNotMatch(source, /operation-split-placeholder/)
  assert.doesNotMatch(source, /operation-split-panel/)
  assert.doesNotMatch(source, /autoSplitCurrentPlanOperation/)
  assert.doesNotMatch(source, /创建草稿生产计划后可填写工序产能拆分/)
  assert.doesNotMatch(source, /先点创建生产计划，生成草稿后再选择工位产能和承担产量/)
  assert.doesNotMatch(source, /@click="loadProductionPlanIntoCurrentEditor\(plan\)"/)
  assert.doesNotMatch(source, /currentPlan\.value = detail/)
})

test('capacity workspace groups by frozen operation names and preserves each source order task', () => {
  const detail = {
    manufacturing_plan: {
      edges: [{ supplier_plan_item_id: 51, consumer_plan_item_id: 61, required_g: 4540 }],
    },
    items: [
      {
        id: 51,
        output_type: 'material',
        output_name: '初晓熟豆',
        output_qty: 4.54,
        output_unit: 'kg',
        process_snapshot_json: JSON.stringify({ operations: [{ seq: 1, operation_id: 701, operation: '咖啡烘焙+除石' }] }),
      },
      {
        id: 61,
        output_type: 'product',
        product_name: '初晓-商品',
        spec_g: 227,
        sales_spec_count: 20,
        sales_spec_snapshot_json: JSON.stringify({ spec_label: '227g', sales_unit: '袋' }),
        process_snapshot_json: JSON.stringify({ operations: [{ seq: 1, operation_id: 702, operation: '包装' }] }),
        demand_sources: [
          { order_item_id: 101, order_no: 'SO-20260907-0003', customer_name: '客户A', quantity: 10, sales_unit: '袋' },
          { order_item_id: 102, order_no: 'SO-20260907-0002', customer_name: '客户B', quantity: 8, sales_unit: '袋' },
          { order_item_id: 103, order_no: 'SO-20260907-0001', customer_name: '客户C', quantity: 2, sales_unit: '袋' },
        ],
      },
    ],
  }
  const preview = {
    operation_coverage: [
      { production_plan_item_id: 51, operation_seq: 1, operation_id: 701, operation: '咖啡烘焙+除石', required_qty: 5.64, arranged_qty: 5.64, diff_qty: 0, unit: 'kg', status: 'matched' },
      { production_plan_item_id: 61, operation_seq: 1, operation_id: 702, operation: '包装', required_qty: 20, arranged_qty: 18, diff_qty: -2, unit: '袋', status: 'short' },
    ],
  }

  const groups = buildProductionPlanCapacityGroups(detail, preview)
  assert.deepEqual(groups.map((group) => group.operation), ['咖啡烘焙+除石', '包装'])
  assert.deepEqual(groups[1].tasks[0].sources.map((source) => [source.order_no, source.quantity_label]), [
    ['SO-20260907-0003', '10袋'],
    ['SO-20260907-0002', '8袋'],
    ['SO-20260907-0001', '2袋'],
  ])
  assert.equal(groups[1].tasks[0].required_label, '20袋')
  assert.equal(groups[1].tasks[0].arranged_label, '18袋')
  assert.equal(groups[1].tasks[0].remaining_label, '2袋')
  assert.equal(groups[1].tasks[0].status, 'short')
})

test('capacity readiness permits short drafts but requires coverage and over-allocation acknowledgement to confirm', () => {
  const short = productionPlanCapacityReadiness({ operation_coverage: [
    { production_plan_item_id: 1, operation: '手工装袋', status: 'short' },
    { production_plan_item_id: 2, operation: '手工装袋', status: 'matched' },
  ] })
  assert.equal(short.can_save_draft, true)
  assert.equal(short.can_confirm, false)
  assert.equal(short.short_count, 1)

  const over = productionPlanCapacityReadiness({ operation_coverage: [
    { production_plan_item_id: 1, operation: '滚筒烘焙', status: 'over' },
    { production_plan_item_id: 2, operation: '手工装袋', status: 'matched' },
  ] })
  assert.equal(over.can_confirm, false)
  assert.equal(over.requires_over_acknowledgement, true)
  assert.equal(productionPlanCapacityReadiness({ operation_coverage: over.rows }, true).can_confirm, true)
})

test('ProducePlanView renders capacity allocation as a full workspace using actual operation names', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const component = fs.readFileSync(new URL('../components/ProductionPlanCapacityWorkspace.vue', import.meta.url), 'utf8')

  assert.match(source, /ProductionPlanCapacityWorkspace/)
  assert.match(source, /:class="\{ 'detail-open': !!\(productionPlanDetail \|\| productionPlanSplitDrawer\), embedded: props.embedded \}"/)
  assert.doesNotMatch(source, /production-plan-split-drawer/)
  assert.match(component, /group\.operation/)
  assert.match(component, /保存草稿/)
  assert.match(component, /确认安排/)
  assert.match(component, /来源订单/)
  assert.doesNotMatch(component, /烘焙熟豆|包装成品/)
})

test('ProducePlanView lets operators search and expand grouped demands and drag horizontal overflow', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  for (const marker of [
    'demandKeyword',
    'filteredStockInsufficientRows',
    '搜索商品、客户或订单号',
    'collapsedDemandGroups',
    'startTableScrollDrag',
    'drag-scroll-wrap',
    'demandPanelTitle',
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
    '配置异常',
  ]) {
    assert.match(source, new RegExp(marker))
  }
  assert.match(source, /row\.demand_status \|\| 'unplanned'/)
  assert.doesNotMatch(source, /selected\[rowKey\(row\)\]/)
})

test('ProducePlanView keeps operation capacity splitting out of gap review', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  const workbenchStart = source.indexOf('class="gap-review-workspace"')
  const listStart = source.indexOf('<!-- Creation ends here;', workbenchStart)
  assert.ok(workbenchStart > 0, 'missing gap review start')
  assert.ok(listStart > workbenchStart, 'missing creation boundary after gap review')
  const currentPlanWorkbench = source.slice(workbenchStart, listStart)

  assert.doesNotMatch(currentPlanWorkbench, /工序产能拆分/)
  assert.doesNotMatch(currentPlanWorkbench, /保存拆分/)
  assert.doesNotMatch(currentPlanWorkbench, /添加拆分/)
  assert.match(source, /production-plan-capacity-shell/)
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

test('draft production plan items choose and freeze a target warehouse through the typed item endpoint', () => {
  assert.equal(
    producePlan.productionPlanItemTargetWarehouseEndpoint({ id: 41 }, { id: 73 }),
    '/api/production-plans/41/items/73/target-warehouse',
  )
  assert.equal(producePlan.productionPlanItemTargetWarehouseEndpoint({ id: 0 }, { id: 73 }), '')
  assert.deepEqual(producePlan.buildProductionPlanItemTargetWarehousePayload(' wip '), { target_warehouse: 'wip' })

  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  assert.match(source, /apiGet\('\/api\/stock\/warehouses'\)/)
  assert.match(source, /v-model="item\.target_warehouse"/)
  assert.match(source, /saveProductionPlanItemTargetWarehouse\(item\)/)
  assert.match(source, /productionPlanItemTargetWarehouseEndpoint\(productionPlanDetail\.value, item\)/)
  assert.match(source, /method:\s*'PATCH'/)
  assert.match(source, /v-if="productionPlanSelectable\(productionPlanDetail\)"[\s\S]*v-else[\s\S]*targetWarehouseLabel\(item\.target_warehouse\)/)
})

test('production plan component source endpoint and payload freeze warehouse owner and BOM identity', () => {
  assert.equal(
    producePlan.productionPlanItemComponentSourcesEndpoint({ id: 41 }, { production_plan_item_id: 73 }),
    '/api/production-plans/41/items/73/component-sources',
  )
  assert.deepEqual(producePlan.buildProductionPlanComponentSourcesPayload([{
    component_type: 'material',
    component_id: 91,
    component_bom_spec_id: 0,
    component_spec_g: 0,
    source_warehouse: 'customer_74_raw',
    source_owner_customer_id: 74,
    material_source_mode: 'customer_supplied',
  }]), {
    sources: [{
      component_type: 'material',
      component_id: 91,
      component_bom_spec_id: 0,
      component_spec_g: 0,
      source_warehouse: 'customer_74_raw',
      source_owner_customer_id: 74,
    }],
  })
})

test('production plan page makes every BOM component source explicit before submit', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  assert.match(source, /组件来源仓/)
  assert.match(source, /productionPlanItemComponentSourcesEndpoint/)
  assert.match(source, /source_owner_customer_id/)
  assert.match(source, /method:\s*'PUT'/)
})

test('submitted production plan cancellation is offered only while every related work order is unstarted', () => {
  assert.equal(producePlan.productionPlanCanCancelSubmitted({
    id: 41,
    status: 'submitted',
    related_work_orders: [
      { id: 71, status: 'released', running_item_id: 0 },
      { id: 72, status: 'released' },
    ],
  }), true)
  assert.equal(producePlan.productionPlanCanCancelSubmitted({
    id: 41,
    status: 'submitted',
    related_work_orders: [{ id: 71, status: 'running', running_item_id: 88 }],
  }), false)
  assert.equal(producePlan.productionPlanCanCancelSubmitted({ id: 41, status: 'submitted', related_work_orders: [] }), false)
  assert.equal(producePlan.productionPlanCanCancelSubmitted({ id: 41, status: 'draft', related_work_orders: [{ id: 71, status: 'released' }] }), false)

  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  assert.match(source, /productionPlanCanCancelSubmitted\(productionPlanDetail\)/)
  assert.match(source, /@click="cancelSubmittedProductionPlan\(productionPlanDetail, 'detail'\)"/)
  assert.match(source, /确认取消已提交生产计划/)
  assert.match(source, /apiSend\(productionPlanCancelEndpoint\(plan\), \{ body: \{\} \}\)/)
})

test('production plan related work orders render the typed output identity', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  assert.match(source, /<th>产出对象<\/th>/)
  assert.match(source, /formatWorkOrderTypedOutput\(wo\)/)
  assert.match(source, /productionPlanDetail\.related_work_orders/)
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

test('multi-level manufacturing plan rows expose typed recursive demand, stock coverage, net shortage and dependencies', () => {
  const payload = {
    nodes: [
      { item: { type: 'product', id: 88, name: '227g 咖啡豆', unit: 'unit' }, required_qty: 100, stock_covered_qty: 0, shortage_qty: 100, action: 'manufacture', bom_version_id: 501 },
      { item: { type: 'material', id: 27, name: '烘焙熟豆', unit: 'kg' }, required_qty: 22.7, stock_covered_qty: 10, shortage_qty: 12.7, action: 'manufacture', bom_version_id: 502 },
      { item: { type: 'material', id: 11, name: '咖啡生豆', unit: 'kg' }, required_qty: 14.4, stock_covered_qty: 0, shortage_qty: 14.4, action: 'purchase', blocking: true },
    ],
    edges: [
      { consumer_key: 'product:88', supplier_key: 'material:27', required_qty: 22.7 },
      { consumer_key: 'material:27', supplier_key: 'material:11', required_qty: 14.4 },
    ],
  }

  const rows = producePlan.manufacturingPlanRows(payload)
  assert.deepEqual(rows.map((row) => [row.key, row.depth]), [
    ['product:88', 0], ['material:27', 1], ['material:11', 2],
  ])
  assert.equal(rows[1].type_label, '物料')
  assert.equal(rows[1].required_qty, 22.7)
  assert.equal(rows[1].stock_covered_qty, 10)
  assert.equal(rows[1].shortage_qty, 12.7)
  assert.equal(rows[1].dependency_label, '供给 商品 · 227g 咖啡豆')
  assert.equal(rows[2].action_label, '采购/补料')
  assert.equal(rows[2].blocking, true)
})

test('multi-level manufacturing plan preserves real backend flat typed node identities', () => {
  const rows = producePlan.manufacturingPlanRows({
    manufacturing_plan: {
      nodes: [
        { key: 'product:1', output_type: 'product', output_product_id: 1, output_material_id: 0, output_name: '227g 咖啡豆', output_unit: '件', required_qty: 100, stock_covered_qty: 0, shortage_qty: 100, action: 'manufacture', depth: 0 },
        { key: 'material:10', output_type: 'material', output_product_id: 0, output_material_id: 10, output_name: '烘焙熟豆', output_unit: 'kg', required_qty: 22.7, stock_covered_qty: 10, shortage_qty: 12.7, action: 'manufacture', depth: 1 },
        { key: 'material:30', output_type: 'material', output_product_id: 0, output_material_id: 30, output_name: '咖啡生豆', output_unit: 'kg', required_qty: 14.4, stock_covered_qty: 0, shortage_qty: 14.4, action: 'purchase', depth: 2, blocking: true },
        { key: 'material:20', output_type: 'material', output_product_id: 0, output_material_id: 20, output_name: '包装袋', output_unit: '件', required_qty: 100, stock_covered_qty: 40, shortage_qty: 60, action: 'purchase', depth: 2, blocking: true },
      ],
      edges: [
        { consumer_key: 'product:1', supplier_key: 'material:10', required_qty: 22.7 },
        { consumer_key: 'material:10', supplier_key: 'material:30', required_qty: 14.4 },
        { consumer_key: 'material:10', supplier_key: 'material:20', required_qty: 100 },
      ],
    },
  })

  assert.deepEqual(rows.map((row) => [row.key, row.depth, row.name]), [
    ['product:1', 0, '227g 咖啡豆'],
    ['material:10', 1, '烘焙熟豆'],
    ['material:30', 2, '咖啡生豆'],
    ['material:20', 2, '包装袋'],
  ])
  assert.equal(rows[1].dependency_label, '供给 商品 · 227g 咖啡豆')
  assert.equal(rows[2].dependency_label, '供给 物料 · 烘焙熟豆')
  assert.equal(rows[3].dependency_label, '供给 物料 · 烘焙熟豆')
})

test('multi-level manufacturing graph uses backend depth and lists every shared-node consumer', () => {
  const rows = producePlan.manufacturingPlanRows({
    manufacturing_plan: {
      nodes: [
        { key: 'product:1', output_type: 'product', output_product_id: 1, output_name: '零售成品', output_unit: '件', required_qty: 10, depth: 0, action: 'manufacture' },
        { key: 'material:30', output_type: 'material', output_material_id: 30, output_name: '拼配熟豆', output_unit: 'kg', required_qty: 5, depth: 1, action: 'manufacture' },
        { key: 'material:10', output_type: 'material', output_material_id: 10, output_name: '共享基底', output_unit: 'kg', required_qty: 6, depth: 1, action: 'manufacture' },
      ],
      edges: [
        { consumer_key: 'product:1', supplier_key: 'material:30' },
        { consumer_key: 'material:30', supplier_key: 'material:10' },
        { consumer_key: 'product:1', supplier_key: 'material:10' },
      ],
    },
  })

  const shared = rows.find((row) => row.key === 'material:10')
  assert.equal(shared.depth, 1)
  assert.equal(shared.dependency_label, '供给 物料 · 拼配熟豆；商品 · 零售成品')
})

test('multi-level manufacturing plan falls back to persisted plan items and supply gaps', () => {
  const rows = producePlan.manufacturingPlanRows({
    items: [
      { id: 501, output_type: 'product', output_product_id: 88, output_name: '227g 咖啡豆', output_qty: 22.7, output_unit: 'kg', bom_version_id: 701 },
      { id: 502, output_type: 'material', output_material_id: 27, output_name: '烘焙熟豆', output_qty: 12.7, output_unit: 'kg', bom_version_id: 702 },
    ],
    supply_gaps: [
      { id: 9, production_plan_item_id: 502, item_type: 'material', item_id: 11, item_name: '咖啡生豆', required_g: 14400, reason: 'no_default_material_bom', status: 'open' },
    ],
  })

  assert.deepEqual(rows.map((row) => [row.key, row.depth]), [
    ['product:88', 0], ['material:27', 1], ['gap:material:11:9', 2],
  ])
  assert.deepEqual(rows.map((row) => [row.name, row.required_qty, row.unit]), [
    ['227g 咖啡豆', 22.7, 'kg'], ['烘焙熟豆', 12.7, 'kg'], ['咖啡生豆', 14400, 'g'],
  ])
  assert.equal(rows[1].action_label, '安排生产')
  assert.equal(rows[1].dependency_label, '供给计划内商品')
  assert.equal(rows[2].action_label, '采购/补料')
  assert.equal(rows[2].dependency_label, '阻断 物料 · 烘焙熟豆')
  assert.equal(rows[2].blocking, true)
})

test('production plan detail drawer retains the typed manufacturing dependency graph for traceability', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  assert.match(source, /manufacturing-dependency-table/)
  assert.match(source, /productionPlanDetailManufacturingRows/)
  assert.match(source, /对象类型/)
  assert.match(source, /库存覆盖/)
  assert.match(source, /净缺口/)
  assert.match(source, /上游依赖/)
  assert.match(source, /manufacturing_plan/)
  assert.doesNotMatch(source, /两段式/)
})
