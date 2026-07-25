import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

import {
  buildJobCardActionPayload,
  canCompleteWorkOrder,
  canRunJobCardAction,
  jobCardActionEndpoint,
  jobCardStatusOptions,
  stockEntryEndpoint,
  stockDocumentPreviewEndpoint,
  stockEntryTypeOptions,
  workOrderCancelEndpoint,
  workOrderCompleteEndpoint,
  workOrderIssueMaterialsEndpoint,
  workOrderStartEndpoint,
  workOrderStatusLabel,
} from './manufacturing-execution.js'

test('manufacturing execution helpers expose phase2 endpoints and status labels', () => {
  assert.equal(stockEntryEndpoint(), '/api/stock-documents')
  assert.equal(stockDocumentPreviewEndpoint({ id: 88 }), '/api/produce/work-orders/88/stock-document-preview')
  assert.equal(workOrderStartEndpoint({ id: 88 }), '/api/produce/work-orders/88/start')
  assert.equal(workOrderIssueMaterialsEndpoint({ id: 88 }), '/api/produce/work-orders/88/issue-materials')
  assert.equal(workOrderCompleteEndpoint({ id: 88 }), '/api/produce/work-orders/88/complete')
  assert.equal(workOrderCancelEndpoint({ id: 88 }), '/api/produce/work-orders/88/cancel')
  assert.equal(workOrderStartEndpoint({ id: 0 }), '')
  assert.equal(workOrderCompleteEndpoint({ id: 0 }), '')
  assert.equal(workOrderCancelEndpoint({ id: 0 }), '')
  assert.equal(jobCardActionEndpoint({ id: 91 }, 'start'), '/api/job-cards/91/start')
  assert.equal(jobCardActionEndpoint({ id: 91 }, 'pause'), '/api/job-cards/91/pause')
  assert.equal(jobCardActionEndpoint({ id: 91 }, 'resume'), '/api/job-cards/91/resume')
  assert.equal(jobCardActionEndpoint({ id: 91 }, 'complete'), '/api/job-cards/91/complete')
  assert.equal(jobCardActionEndpoint({ id: 0 }, 'start'), '')

  assert.deepEqual(jobCardStatusOptions().map((item) => item.value), [
    '',
    'pending',
    'ready',
    'running',
    'paused',
    'completed',
    'cancelled',
  ])
  assert.equal(workOrderStatusLabel('released'), '未开工')
  assert.equal(workOrderStatusLabel('running'), '生产中')
  assert.equal(workOrderStatusLabel('partially_completed'), '部分完成')
  assert.equal(workOrderStatusLabel('completed'), '已完成')

  assert.deepEqual(stockEntryTypeOptions().map((item) => item.value), [
    'material_receipt',
    'material_issue',
    'material_transfer',
    'material_transfer_for_manufacture',
    'material_return_from_manufacture',
    'material_consumption_for_manufacture',
    'manufacture',
  ])
})

test('manufacturing execution helpers enforce button availability by state', () => {
  assert.equal(canRunJobCardAction({ id: 91, status: 'pending' }, 'start'), true)
  assert.equal(canRunJobCardAction({ id: 91, status: 'ready' }, 'start'), true)
  assert.equal(canRunJobCardAction({ id: 91, status: 'running' }, 'pause'), true)
  assert.equal(canRunJobCardAction({ id: 91, status: 'paused' }, 'resume'), true)
  assert.equal(canRunJobCardAction({ id: 91, status: 'running' }, 'complete'), true)
  assert.equal(canRunJobCardAction({ id: 91, status: 'completed' }, 'start'), false)
  assert.equal(canRunJobCardAction({ id: 0, status: 'pending' }, 'start'), false)

  assert.equal(canCompleteWorkOrder({ id: 88, status: 'running', running_item_id: 99 }), true)
  assert.equal(canCompleteWorkOrder({ id: 88, status: 'partially_completed', running_item_id: 99 }), true)
  assert.equal(canCompleteWorkOrder({ id: 88, status: 'released', running_item_id: 0 }), false)
})

test('buildJobCardActionPayload includes actual quantities and loss reason without view-only fields', () => {
  assert.deepEqual(buildJobCardActionPayload({
    actual_input_qty: '600',
    actual_output_qty: 540,
    loss_reason: '正常损耗',
    metrics_json: '{"temperature":196}',
  }), {
    actual_input_qty: 600,
    actual_output_qty: 540,
    actual_minutes: 0,
    loss_reason: '正常损耗',
    metrics_json: { temperature: 196 },
  })
})

test('phase2 Vue pages expose work order execution, job-card actions, and stock entry documents', () => {
  const workOrders = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')
  for (const want of ['已领料', '已消耗', '可退料', '工序进度', '成本汇总', "openStockDocument(row, 'finish')", 'stockOperations']) {
    assert.ok(workOrders.includes(want), `WorkOrdersView.vue missing ${want}`)
  }

  const jobCards = fs.readFileSync(new URL('../views/JobCardsView.vue', import.meta.url), 'utf8')
  for (const want of ['jobCardStatusOptions()', 'jobCardActionEndpoint(row, action)', '开始', '暂停', '继续', '完成', '保存实际', '损耗原因']) {
    assert.ok(jobCards.includes(want), `JobCardsView.vue missing ${want}`)
  }

  const stockOperations = fs.readFileSync(new URL('../views/StockOperationsView.vue', import.meta.url), 'utf8')
  assert.match(stockOperations, /StockEntriesView/)
  assert.match(stockOperations, /库存单据/)
  assert.doesNotMatch(stockOperations, /WIP领退\/转仓/)
  assert.doesNotMatch(stockOperations, /成品转仓/)

  const stockEntries = fs.readFileSync(new URL('../views/StockEntriesView.vue', import.meta.url), 'utf8')
  for (const want of ['/api/stock-documents', '盘点调整']) {
    if (want === '/api/stock-documents') {
      assert.ok(stockEntries.includes(want) || stockEntries.includes('stockEntryEndpoint'), `StockEntriesView.vue missing ${want}`)
    } else if (want === '盘点调整') {
      assert.ok(stockOperations.includes(want), `StockOperationsView.vue missing ${want}`)
    }
  }
  assert.deepEqual(stockEntryTypeOptions().map((item) => item.label), ['原料入库', '物料发出 / 报废', '库存转仓', '生产领料', '退回未用原料', '记录生产消耗', '完工入库'])
})
