import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

import {
  buildExecutionHubActions,
  buildExecutionHubFocus,
  executionHubTimelineFilters,
  filterExecutionHubTimeline,
  productionContextParams,
  readinessBadgeTone,
} from './production-execution-hub.js'

test('execution hub actions carry production context to work order, WIP, job card, quality, costs and logs pages', () => {
  const hub = {
    work_order: { id: 88, work_order_no: 'WO-00088', running_item_id: 99, batch_id: 'BATCH-WO-88' },
    job_cards: [{ id: 91, work_order_id: 88, status: 'running' }],
    readiness: {
      suggested_action: 'open_wip_issue',
      related_links: [{ key: 'wip', label: '处理 WIP', view: 'stockOperations', params: { tab: 'wip', shortage_g: 1200 } }],
    },
  }
  const actions = buildExecutionHubActions(hub).map((action) => [action.key, action.label, action.view, action.params])

  assert.deepEqual(actions.slice(0, 10), [
    ['startProduction', '开始生产', 'workOrders', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['productionIssue', '生产领料', 'stockOperations', { tab: 'stockEntries', action: 'issue', return_source: 'work_order', work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88', shortage_g: 1200 }],
    ['productionSupplement', '补料', 'stockOperations', { tab: 'stockEntries', action: 'supplement', return_source: 'work_order', work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['productionReturn', '退回未用原料', 'stockOperations', { tab: 'stockEntries', action: 'return', return_source: 'work_order', work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['productionConsume', '记录生产消耗', 'stockOperations', { tab: 'stockEntries', action: 'consume', return_source: 'work_order', work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['finishedReceipt', '完工入库', 'stockOperations', { tab: 'stockEntries', action: 'finish', return_source: 'work_order', work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['openJobCard', '打开工序卡', 'jobCards', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['openQuality', '打开质检', 'qualityInspections', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88', reference_no: 'WO-00088' }],
    ['openCost', '成本', 'productionCosts', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['openLogs', '日志', 'produceLogs', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
  ])
})

test('execution hub readiness and timeline helpers expose filters and focused areas', () => {
  assert.equal(readinessBadgeTone({ severity: 'blocked' }), 'danger')
  assert.equal(readinessBadgeTone({ severity: 'warning' }), 'warning')
  assert.equal(readinessBadgeTone({ can_start: true }), 'success')
  assert.deepEqual(executionHubTimelineFilters().map((item) => item.key), ['all', 'operation', 'inventory', 'quality', 'cost', 'log'])

  const timeline = [
    { type: 'operation', title: '工序开始' },
    { type: 'inventory', title: '领料' },
    { type: 'quality', title: '质检冻结' },
  ]
  assert.deepEqual(filterExecutionHubTimeline(timeline, 'quality').map((item) => item.title), ['质检冻结'])
  assert.equal(buildExecutionHubFocus({ focus: 'blocked' }).section, 'readiness')
  assert.equal(buildExecutionHubFocus({ job_card_id: 91 }).section, 'job_card')
})

test('production context params preserve work order, job card, running item, material and shortage context', () => {
  assert.deepEqual(productionContextParams({
    work_order_id: '88',
    job_card_id: '91',
    running_item_id: '99',
    material_id: '12',
    shortage_g: '1200',
    batch_id: 'BATCH-WO-88',
    tab: 'stockEntries',
    action: 'issue',
    return_source: 'work_order',
    ignored: 'x',
  }), {
    tab: 'stockEntries',
    action: 'issue',
    return_source: 'work_order',
    work_order_id: 88,
    job_card_id: 91,
    running_item_id: 99,
    material_id: 12,
    shortage_g: 1200,
    batch_id: 'BATCH-WO-88',
  })
})

test('production pages mount the shared execution hub drawer instead of separate work order drawers', () => {
  const files = [
    'src/views/ProductionOverviewView.vue',
    'src/views/WorkstationView.vue',
    'src/views/WorkOrdersView.vue',
    'src/views/JobCardsView.vue',
  ]
  for (const file of files) {
    const source = fs.readFileSync(new URL(`../${file.replace(/^src\//, '')}`, import.meta.url), 'utf8')
    assert.match(source, /ProductionExecutionHubDrawer/, `${file} should use shared execution hub drawer`)
  }

  const appSource = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
  for (const key of ['work_order_id', 'job_card_id', 'running_item_id', 'material_id', 'shortage_g', 'reference_no', 'focus', 'batch_id', 'action', 'return_source']) {
    assert.match(appSource, new RegExp(`'${key}'`), `App.vue should preserve ${key} in view params`)
  }
})
