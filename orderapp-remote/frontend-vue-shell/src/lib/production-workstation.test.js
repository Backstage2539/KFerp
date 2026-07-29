import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import {
  navItemsWithProductionBadges,
  productionCompletionMetrics,
  productionCompletionOutputQty,
  productionTaskActionEndpoint,
  productionTaskActionErrorMessage,
  productionTopNavItems,
  stockOperationContextParams,
  workstationVisibleActions,
  workstationTaskSections,
} from './production-workstation.js'

test('production top nav lists high-frequency views before legacy production pages', () => {
  assert.deepEqual(productionTopNavItems.map((item) => item.key), [
    'productionOverview',
    'workstationView',
    'productionFlow',
    'produceRunning',
    'produceLogs',
  ])
  assert.equal(productionTopNavItems[0].label, '生产视图')
  assert.equal(productionTopNavItems[1].label, '工位视图')
})

test('workstation task sections answer current task, next task, and blocked reason', () => {
  const sections = workstationTaskSections([
    {
      job_card_id: 91,
      work_order_no: 'WO-001',
      product_name: '桂花乌龙',
      operation: '包装',
      workstation: '包装工位A',
      status: 'running',
      priority: 9,
      planned_start_at: '2026-06-13 09:00',
      blocking_reason: '',
    },
    {
      job_card_id: 92,
      work_order_no: 'WO-001',
      product_name: '桂花乌龙',
      operation: '贴标',
      workstation: '包装工位A',
      status: 'pending',
      priority: 6,
      planned_start_at: '2026-06-13 09:45',
      blocking_reason: '',
    },
    {
      job_card_id: 93,
      work_order_no: 'WO-002',
      product_name: '日晒瑰夏',
      operation: '烘焙',
      workstation: '布勒 18kg',
      status: 'paused',
      priority: 8,
      blocking_reason: '缺少生豆领料',
    },
  ])

  assert.equal(sections.length, 2)
  const pack = sections.find((section) => section.workstation === '包装工位A')
  assert.equal(pack.currentTask.job_card_id, 91)
  assert.equal(pack.nextTask.job_card_id, 92)
  assert.equal(pack.blockingReason, '')

  const roast = sections.find((section) => section.workstation === '布勒 18kg')
  assert.equal(roast.currentTask.job_card_id, 93)
  assert.equal(roast.nextTask, null)
  assert.equal(roast.blockingReason, '缺少生豆领料')
})

test('production task action endpoints stay aligned with workstation action buttons', () => {
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'start'), '/api/job-cards/91/start')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'pause'), '/api/job-cards/91/pause')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'resume'), '/api/job-cards/91/resume')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'complete'), '/api/job-cards/91/complete')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'report_exception'), '/api/production/workstation/tasks/91/exception')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'material_call'), '/api/production/workstation/tasks/91/material-call')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 0 }, 'start'), '')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'unknown'), '')
})

test('production task action failures are explained in Chinese without changing the server state locally', () => {
  assert.equal(
    productionTaskActionErrorMessage(new Error('invalid job card status transition: running -> resume'), 'resume'),
    '当前工序状态不允许继续，请刷新后按最新状态操作',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('job card not found'), 'pause'),
    '工序卡不存在或已失效，请刷新后重试',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('invalid job card action resume from running'), 'resume'),
    '当前工序状态不允许继续，请刷新后按最新状态操作',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('work order must be running before job card start'), 'start'),
    '请先从工单执行枢纽开始生产，再在工位开始本工序',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('permission denied'), 'complete'),
    '当前账号没有执行此操作的权限，请联系管理员',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('work order must be released'), 'start'),
    '工单必须先下达后才能执行工序',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('work order is not running'), 'resume'),
    '工单尚未开始生产，请先从执行枢纽开始生产',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('actual input and output quantity invalid'), 'complete'),
    '实际投入或实际产出数量不正确，请检查后重试',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('实际产出和成品件数只能填写一项'), 'complete'),
    '实际产出和成品件数只能填写一项',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('network unavailable'), 'pause'),
    '暂停失败，请稍后重试；如持续失败请联系管理员',
  )
})

test('production top nav renders count badges for high-frequency production views', () => {
  const items = navItemsWithProductionBadges(productionTopNavItems, {
    productionOverview: { pending: 2, blocked: 1, running: 3 },
    workstationView: { pending: 2, blocked: 1, running: 3 },
    produceRunning: { running: 3 },
  })

  assert.deepEqual(items.slice(0, 4).map((item) => ({
    key: item.key,
    badge: item.badge,
  })), [
    { key: 'productionOverview', badge: '待2 阻1 中3' },
    { key: 'workstationView', badge: '待2 阻1 中3' },
    { key: 'productionFlow', badge: '' },
    { key: 'produceRunning', badge: '待0 阻0 中3' },
  ])
})

test('stock operation context carries WIP prefill parameters from production tasks', () => {
  assert.deepEqual(stockOperationContextParams({
    work_order_id: 88,
    job_card_id: 91,
    running_item_id: 99,
    material_id: 10,
    shortage_g: 600,
  }), {
    tab: 'stockEntries',
    action: 'issue',
    return_source: 'work_order',
    work_order_id: 88,
    job_card_id: 91,
    running_item_id: 99,
    material_id: 10,
    shortage_g: 600,
  })
})

test('workstation view lets wide task rows scroll and single-station filters fill the work area', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(source, /:class="\{\s*'single-station-grid': singleStationLayout\s*\}"/)
  assert.match(source, /const singleStationLayout = computed\(\(\) => visibleSections\.value\.length === 1\)/)
  assert.match(source, /\.station-grid\.single-station-grid\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\);/s)
  assert.match(source, /\.task-table\s*\{[^}]*overflow-x:\s*auto;[^}]*overflow-y:\s*hidden;[^}]*-webkit-overflow-scrolling:\s*touch;[^}]*overscroll-behavior-inline:\s*contain;/s)
  assert.match(source, /\.task-row\s*\{[^}]*min-width:\s*610px;/s)
  assert.match(source, /@media \(max-width:\s*760px\)\s*\{[\s\S]*\.task-row\s*\{[^}]*grid-template-columns:\s*1fr;[^}]*min-width:\s*0;/)
})

test('workstation action buttons reveal their forms inside the clicked task row', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(source, /<div v-for="task in section\.tasks"[\s\S]*v-if="isIssuePanelForTask\(task\)"[\s\S]*v-if="isFinishPanelForTask\(task\)"/)
  assert.match(source, /class="task-action-panel"/)
  assert.match(source, /function isIssuePanelForTask\(task\)/)
  assert.match(source, /function isFinishPanelForTask\(task\)/)
  assert.match(source, /function openIssue\(task, mode\) \{[\s\S]*finishPanel\.open = false/)
  assert.match(source, /function openFinishPanel\(task\) \{[\s\S]*issue\.open = false/)
  assert.doesNotMatch(source, /<section v-if="issue\.open" class="panel action-panel">/)
  assert.doesNotMatch(source, /<section v-if="finishPanel\.open" class="panel action-panel">/)
})

test('workstation is the only job-card execution surface and completes with actual records in one submission', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(source, /:disabled="Boolean\(busyKey\) \|\| loading"/)
  assert.match(source, /工序要求：\{\{ task\.process_requirement \|\| '按冻结工艺路线执行' \}\}/)
  for (const field of [
    'finishPanel.actual_minutes',
    'finishPanel.actual_input_qty',
    'finishPanel.actual_output_qty',
    'finishPanel.leftover_qty',
    'finishPanel.loss_reason',
    'finishPanel.exception_reason',
  ]) {
    assert.match(source, new RegExp(field.replace('.', '\\.')))
  }
  assert.match(source, /actual_minutes: Number\(finishPanel\.actual_minutes/)
  assert.match(source, /actual_input_qty: Number\(finishPanel\.actual_input_qty/)
  assert.match(source, /actual_output_qty: actualOutputQty/)
  assert.match(source, /task\.planned_input_inventory_qty/)
  assert.match(source, /inventory_qty_per_sales_unit/)
  assert.match(source, /finishPanel\.inventory_unit/)
  assert.match(source, /loss_reason: finishPanel\.loss_reason/)
  assert.match(source, /exception_reason: finishPanel\.exception_reason/)
  assert.match(source, /productionCompletionMetrics/)
  assert.match(source, /实际投入（\{\{ finishPanel\.inventory_unit \|\| '-' \}\}）/)
  assert.match(source, /实际产出（\{\{ finishPanel\.inventory_unit \|\| '-' \}\}）/)
  assert.doesNotMatch(source, /Number\(finishPanel\.finished_loose_g \|\| 0\)[\s\S]*actualOutputG/)
  assert.doesNotMatch(source, /finishRunningProduction/)
  assert.doesNotMatch(source, /partial_finish/)
})

test('workstation entry focuses the requested task instead of reopening the execution hub', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(source, /focus === 'workstation_task'/)
  assert.match(source, /requestedJobCardID/)
  assert.match(source, /selectedWorkstation\.value = matchedTask\.workstation/)
  assert.match(source, /:class="\{ focused: isRequestedTask\(task\) \}"/)
  assert.match(source, /load\(\{ focusRequested: true \}\)/)
  assert.match(source, /if \(options\?\.focusRequested === true\) focusRequestedTask\(\)/)
  assert.doesNotMatch(source, /^\s*focusRequestedTask\(\)\s*$/m)
  assert.doesNotMatch(source, /if \(id > 0\) \{\s*executionHub\.workOrderId = id/s)
})

test('workstation completion keeps input and output in the frozen inventory unit', () => {
  assert.equal(productionCompletionOutputQty({
    actualOutputQty: 0,
    finishedUnits: 14,
    inventoryQtyPerSalesUnit: 0.454,
  }), 6.356)
  assert.equal(productionCompletionOutputQty({
    actualOutputQty: 6.2,
    finishedUnits: 0,
    inventoryQtyPerSalesUnit: 0.454,
  }), 6.2)
  assert.throws(
    () => productionCompletionOutputQty({
      actualOutputQty: 6.2,
      finishedUnits: 14,
      inventoryQtyPerSalesUnit: 0.454,
    }),
    /实际产出和成品件数只能填写一项/,
  )
  assert.throws(
    () => productionCompletionOutputQty({
      actualOutputQty: 0,
      finishedUnits: 0,
      inventoryQtyPerSalesUnit: 0.454,
    }),
    /请填写实际产出或成品件数/,
  )
  assert.throws(
    () => productionCompletionOutputQty({
      actualOutputQty: 0,
      finishedUnits: 14,
      inventoryQtyPerSalesUnit: 0,
    }),
    /缺少库存单位换算/,
  )
  assert.throws(
    () => productionCompletionOutputQty({
      actualOutputQty: 0,
      finishedUnits: 14.5,
      inventoryQtyPerSalesUnit: 0.454,
    }),
    /成品件数必须为整数/,
  )
})

test('workstation completion freezes inventory-unit interpretation and hides legacy partial finish', () => {
  assert.deepEqual(productionCompletionMetrics({
    inventoryUnit: 'Kg',
    leftoverQty: 0.2,
    note: '包装抽检',
    warehouse: 'finished_goods',
    finishedUnits: 14,
  }), {
    quantity_basis: 'inventory_unit',
    inventory_unit: 'Kg',
    leftover_qty: 0.2,
    note: '包装抽检',
    warehouse: 'finished_goods',
    finished_units: 14,
  })
  assert.deepEqual(workstationVisibleActions({
    available_actions: ['pause', 'complete', 'partial_finish', 'report_exception', 'material_call'],
  }), ['pause', 'complete', 'report_exception', 'material_call'])
})

test('workstation distinguishes a submitted command whose state refresh failed', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(source, /const refreshed = await load\(\)/)
  assert.match(source, /if \(!refreshed\)/)
  assert.match(source, /已提交，但状态刷新失败，请手动刷新/)
  assert.match(source, /return true/)
  assert.match(source, /return false/)
  assert.doesNotMatch(source, /await load\(\)\s*\n\s*message\.value = `\$\{actionLabel\(action\)\}成功`/)
})
