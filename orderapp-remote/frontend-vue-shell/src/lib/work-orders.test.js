import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

import {
  buildWorkOrderOperationSplitPayload,
  canEditWorkOrderSplits,
  canStartWorkOrder,
  formatWorkOrderPlannedOutput,
  workOrderPlannedOutput,
  workOrderOperationSplitsEndpoint,
  workOrderStartEndpoint,
  workOrderStatusOptions,
} from './work-orders.js'

test('work orders display frozen route operations from process snapshot when no job-card summary exists', () => {
  const source = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')

  assert.match(source, /processSnapshot\(row\)/)
  assert.match(source, /Array\.isArray\(snapshot\.operations\)/)
  assert.match(source, /operation:\s*item\.operation\s*\|\|\s*item\.operation_name/)
  assert.match(source, /workstation:\s*item\.workstation\s*\|\|\s*item\.workstation_name/)
  assert.match(source, /status:\s*item\.status\s*\|\|\s*'frozen'/)
  assert.match(source, /item\.workstation\s*\|\|\s*item\.workstation_name/)
})

test('work order list keeps query actions only and delegates lifecycle commands to the execution hub', () => {
  const source = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')
  const template = source.slice(0, source.indexOf('<script setup>'))
  const rowActions = template.slice(template.indexOf('<td class="row-actions">'), template.indexOf('</td>', template.indexOf('<td class="row-actions">')) + 5)

  for (const marker of ['执行枢纽', '编辑拆分', '打印']) assert.match(rowActions, new RegExp(marker))
  for (const forbidden of ['开始生产', '完工入库', 'startWorkOrder(row)', "openStockDocument(row, 'finish')"]) {
    assert.doesNotMatch(rowActions, new RegExp(forbidden.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.match(source, /@updated="load"/)
  assert.doesNotMatch(source, /async function startWorkOrder/)
})

test('work orders and job cards surface frozen workstation capacity time and operation cost', () => {
  const workOrderSource = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')
  const jobCardSource = fs.readFileSync(new URL('../views/JobCardsView.vue', import.meta.url), 'utf8')

  for (const marker of [
    '工位产能',
    '计划分钟',
    '计划工序成本',
    'workstation_capacity_name',
    'planned_minutes',
    'planned_operation_cost',
    'operationPlanText',
  ]) {
    assert.match(workOrderSource, new RegExp(marker))
  }
  for (const marker of [
    '工位产能',
    '计划分钟',
    '实际分钟',
    '计划工序成本',
    '实际工序成本',
    'actual_minutes',
    'actual_operation_cost',
  ]) {
    assert.match(jobCardSource, new RegExp(marker))
  }
})

test('job card main table is a read-only execution record', () => {
  const source = fs.readFileSync(new URL('../views/JobCardsView.vue', import.meta.url), 'utf8')
  const template = source.slice(0, source.indexOf('<script setup>'))

  for (const forbidden of [
    '计划投入',
    '实际投入',
    '实际产出',
    'v-model.number="draftFor(row).planned_input_qty"',
    'v-model.number="draftFor(row).actual_input_qty"',
    'v-model.number="draftFor(row).actual_output_qty"',
    '<input',
    '保存实际',
    '>开始<',
    '>暂停<',
    '>继续<',
    '>完成<',
  ]) {
    assert.doesNotMatch(template, new RegExp(forbidden))
  }

  for (const required of [
    '实际分钟',
    '计划工序成本',
    '实际工序成本',
    '实际损耗',
    '损耗原因',
    '异常原因',
    '工序要求',
    '进入工位',
    '执行枢纽',
  ]) {
    assert.match(template, new RegExp(required))
  }

  assert.match(source, /row\.process_requirement \|\| '按冻结工艺路线执行'/)
  assert.match(source, /function openWorkstation\(row\)/)
  assert.match(source, /key: 'workstationView'/)
  assert.match(source, /focus: 'workstation_task'/)
  assert.doesNotMatch(source, /runJobCardAction/)
  assert.doesNotMatch(source, /saveActuals/)
})

test('job cards show product and BOM recipe context with execution navigation', () => {
  const source = fs.readFileSync(new URL('../views/JobCardsView.vue', import.meta.url), 'utf8')
  const template = source.slice(0, source.indexOf('<script setup>'))

  for (const marker of [
    '<th>商品</th>',
    '<th>BOM/配方</th>',
    'row.work_order_no',
    'openExecutionHub',
    'openWorkstation',
    'bomRecipeLabel',
  ]) {
    assert.match(source, new RegExp(marker))
  }
  assert.match(template, /button class="link-button work-order-link"/)
  assert.match(template, /{{ row\.product_name \|\| '-' }}/)
  assert.doesNotMatch(template, /#\{\{ row\.id \}\}/)
  assert.doesNotMatch(source, /job-card-work-order-drawer/)
})

test('work order main table uses generic manufacturing columns instead of roasting advice', () => {
  const source = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')

  for (const want of ['BOM/工艺路线', '工序摘要', '工艺参数', '商品生产配置快照']) {
    assert.match(source, new RegExp(want))
  }
  for (const forbidden of ['工艺建议', '建议设备', '建议锅次', 'suggested_machine', 'suggested_batch_plan', 'suggested_batch_count']) {
    assert.doesNotMatch(source, new RegExp(forbidden))
  }
})

test('current production views remove legacy expected yield and keep actual yield only', () => {
  const runningSource = fs.readFileSync(new URL('../views/ProduceRunningView.vue', import.meta.url), 'utf8')
  const workOrderSource = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')
  const logSource = fs.readFileSync(new URL('../views/ProductionLogsView.vue', import.meta.url), 'utf8')

  assert.doesNotMatch(runningSource, /预期产出率/)
  assert.doesNotMatch(runningSource, /bom_yield_rate/)
  assert.match(runningSource, /实际产出率/)

  assert.doesNotMatch(workOrderSource, /预期产出率|预期损耗率/)
  assert.doesNotMatch(workOrderSource, /expectedYield|expectedLoss\(/)
  assert.match(workOrderSource, /实际损耗率/)
  assert.match(workOrderSource, /实际产出/)

  assert.doesNotMatch(logSource, /BOM预期产出率|row\.bom_yield_rate/)
  assert.match(logSource, /实际产出率/)
  assert.match(logSource, /row\.actual_yield_rate/)
  assert.match(logSource, /colspan="19"/)
})

test('WorkOrdersView filters work orders only by status without BOM demand preview', () => {
  const source = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')
  const template = source.slice(0, source.indexOf('<script setup>'))
  const filterPanel = template.slice(
    template.indexOf('<section class="panel no-print">'),
    template.indexOf('<section class="panel table-wrap no-print">'),
  )
  const escapeRegExp = (value) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

  assert.match(filterPanel, /<span>状态<\/span>/)
  assert.match(filterPanel, /v-model="status"/)
  assert.match(filterPanel, /@click="load"/)

  for (const forbidden of [
    '按 BOM 预览生产需求',
    '生产 BOM',
    '选择 BOM',
    '生产数量',
    '多层展开策略',
    '冻结 BOM',
    '产出商品',
    '产出基准',
    'bom-workbench',
    'workbench-filters',
    'bom-freeze-summary',
    'compact-demand',
    'selectedBomID',
    'productionBoms',
    'productionBomDetails',
    'selectedBomDetail',
    'selectedBomVersion',
    'workOrderDemandRows',
    'loadSelectedBomDetail',
    'buildDemandRows',
    "apiGet('/api/production-boms?status=all')",
  ]) {
    assert.doesNotMatch(source, new RegExp(escapeRegExp(forbidden)))
  }
})

test('workOrderStatusOptions includes draft and released lifecycle states before running', () => {
  assert.deepEqual(workOrderStatusOptions().map((item) => item.value), [
    '',
    'draft',
    'released',
    'running',
    'partially_completed',
    'completed',
    'cancelled',
  ])
})

test('canStartWorkOrder allows only released work orders', () => {
  assert.equal(canStartWorkOrder({ id: 41, status: 'released', running_item_id: 0 }), true)
  assert.equal(canStartWorkOrder({ id: 42, status: 'running', running_item_id: 99 }), false)
  assert.equal(canStartWorkOrder({ id: 43, status: 'draft', running_item_id: 0 }), false)
  assert.equal(canStartWorkOrder({ status: 'released', running_item_id: 0 }), false)
})

test('released work orders expose operation split editing before production starts', () => {
  assert.equal(canEditWorkOrderSplits({ id: 88, status: 'released', running_item_id: 0 }), true)
  assert.equal(canEditWorkOrderSplits({ id: 89, status: 'running', running_item_id: 99 }), false)
  assert.equal(workOrderOperationSplitsEndpoint({ id: 88 }), '/api/work-orders/88/operation-splits')
  assert.equal(workOrderOperationSplitsEndpoint({ id: 0 }), '')

  assert.deepEqual(buildWorkOrderOperationSplitPayload([{
    operation_seq: 1,
    operation_id: 7,
    operation: ' 烘焙 ',
    workstation_capacity_id: 5,
    planned_qty: 72,
    production_plan_item_id: 51,
  }]), {
    items: [{
      operation_seq: 1,
      operation_id: 7,
      operation: '烘焙',
      workstation_capacity_id: 5,
      planned_qty: 72,
      note: '',
    }],
  })
})

test('workOrderStartEndpoint uses formal work order start API', () => {
  assert.equal(workOrderStartEndpoint({ id: 41 }), '/api/produce/work-orders/41/start')
  assert.equal(workOrderStartEndpoint({ id: 0 }), '')
})

test('work order planned output falls back to planned grams and spec when packed counts are absent', () => {
  assert.deepEqual(workOrderPlannedOutput({ planned_g: 55706, spec_g: 454 }), {
    units: 122,
    loose_g: 318,
  })
  assert.equal(formatWorkOrderPlannedOutput({ planned_g: 55706, spec_g: 454 }), '122 袋 + 318g')
  assert.deepEqual(workOrderPlannedOutput({ planned_units: 3, planned_loose_g: 20, planned_g: 0, spec_g: 454 }), {
    units: 3,
    loose_g: 20,
  })
})

test('WorkOrdersView exposes capacity split editor drawer', () => {
  const source = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')
  for (const marker of [
    '编辑拆分',
    'work-order-split-drawer',
    'openWorkOrderSplitDrawer(row)',
    'saveWorkOrderOperationSplits',
    'plannedCapacitySplitMetrics',
    'productionPlanSplitBatchCards',
    'autoSplitWorkOrderOperation',
    'applicableOperationCapacities',
    '自动拆分',
  ]) {
    assert.ok(source.includes(marker), `missing ${marker}`)
  }
  assert.doesNotMatch(source, /assignRemainingWorkOrderSplitQty/)
  assert.doesNotMatch(source, /分配剩余产量/)
  assert.doesNotMatch(source, /分配剩余产能/)

  const autoSplitStart = source.indexOf('function withAutoWorkOrderSplits')
  const autoSplitEnd = source.indexOf('function openWorkOrderSplitDrawer', autoSplitStart)
  const autoSplitSource = source.slice(autoSplitStart, autoSplitEnd)
  for (const frozenOutputField of [
    'planned_output_g',
    'sales_spec_count',
    'inventory_qty_per_sales_unit',
    'inventory_unit',
    'planned_inventory_qty',
  ]) {
    assert.match(autoSplitSource, new RegExp(frozenOutputField))
  }
})
