import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import {
  navItemsWithProductionBadges,
  productionTaskActionEndpoint,
  productionTopNavItems,
  stockOperationContextParams,
  workstationTaskSections,
} from './production-workstation.js'

test('production top nav lists high-frequency views before legacy production pages', () => {
  assert.deepEqual(productionTopNavItems.map((item) => item.key), [
    'productionOverview',
    'workstationView',
    'producePlan',
    'produceRunning',
    'workOrders',
    'jobCards',
    'qualityInspections',
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
    { key: 'producePlan', badge: '' },
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
    tab: 'wip',
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
  assert.match(source, /function openFinishPanel\(task, mode\) \{[\s\S]*issue\.open = false/)
  assert.doesNotMatch(source, /<section v-if="issue\.open" class="panel action-panel">/)
  assert.doesNotMatch(source, /<section v-if="finishPanel\.open" class="panel action-panel">/)
})
