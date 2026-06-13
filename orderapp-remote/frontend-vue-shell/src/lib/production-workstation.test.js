import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  productionTaskActionEndpoint,
  productionTopNavItems,
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
    'productionCosts',
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
