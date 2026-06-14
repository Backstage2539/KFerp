import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

import {
  buildWorkOrderOperationSplitPayload,
  canEditWorkOrderSplits,
  canStartWorkOrder,
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
  assert.match(source, /开始生产/)
  assert.match(source, /startWorkOrder\(row\)/)
  assert.match(source, /workOrderStartEndpoint\(row\)/)
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

test('job card main table hides coffee-specific input and output quantity columns', () => {
  const source = fs.readFileSync(new URL('../views/JobCardsView.vue', import.meta.url), 'utf8')
  const template = source.slice(0, source.indexOf('<script setup>'))

  for (const forbidden of [
    '计划投入',
    '实际投入',
    '实际产出',
    'v-model.number="draftFor(row).planned_input_qty"',
    'v-model.number="draftFor(row).actual_input_qty"',
    'v-model.number="draftFor(row).actual_output_qty"',
  ]) {
    assert.doesNotMatch(template, new RegExp(forbidden))
  }

  for (const required of [
    '实际分钟',
    '计划工序成本',
    '实际工序成本',
    '实际损耗',
    '损耗原因',
    '保存实际',
  ]) {
    assert.match(template, new RegExp(required))
  }

  assert.match(source, /planned_input_qty: Number\(draft\.planned_input_qty \|\| 0\)/)
  assert.match(source, /actual_input_qty: Number\(draft\.actual_input_qty \|\| 0\)/)
  assert.match(source, /actual_output_qty: Number\(draft\.actual_output_qty \|\| 0\)/)
})

test('job cards show product and BOM recipe context with a work order drawer link', () => {
  const source = fs.readFileSync(new URL('../views/JobCardsView.vue', import.meta.url), 'utf8')
  const template = source.slice(0, source.indexOf('<script setup>'))

  for (const marker of [
    '<th>商品</th>',
    '<th>BOM/配方</th>',
    'row.work_order_no',
    'openJobCardWorkOrderDrawer',
    'job-card-work-order-drawer',
    '工单详情',
    '配方物料',
    'materialSnapshotRows',
    'workOrderDrawerRow',
    'bomRecipeLabel',
  ]) {
    assert.match(source, new RegExp(marker))
  }
  assert.match(template, /button class="link-button work-order-link"/)
  assert.match(template, /{{ row\.product_name \|\| '-' }}/)
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
  assert.equal(workOrderStartEndpoint({ id: 41 }), '/api/work-orders/41/start')
  assert.equal(workOrderStartEndpoint({ id: 0 }), '')
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
    'assignRemainingWorkOrderSplitQty',
    'applicableOperationCapacities',
    '分配剩余产量',
    '自动拆分',
  ]) {
    assert.ok(source.includes(marker), `missing ${marker}`)
  }
})
