import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import {
  buildCapacityCalendarPayload,
  buildScheduleAssignmentPayload,
  capacityCalendarEndpoint,
  mrpSuggestionsEndpoint,
  productionScheduleEndpoint,
  scheduleAssignEndpoint,
  scheduleViewModes,
} from './production-schedule.js'

test('production schedule helper builds phase3 endpoints and normalized payloads', () => {
  assert.equal(productionScheduleEndpoint({ from: '2026-06-13', to: '2026-06-14', work_center: '印刷线', status: 'released', limit: 50 }), '/api/production-schedule?from=2026-06-13&to=2026-06-14&work_center=%E5%8D%B0%E5%88%B7%E7%BA%BF&status=released&limit=50')
  assert.equal(mrpSuggestionsEndpoint({ from: '2026-06-13', to: '2026-06-14', work_center: '印刷线', status: 'released', material_id: 10, limit: 50 }), '/api/mrp/suggestions?from=2026-06-13&to=2026-06-14&work_center=%E5%8D%B0%E5%88%B7%E7%BA%BF&status=released&material_id=10&limit=50')
  assert.equal(scheduleAssignEndpoint(), '/api/production-schedule/assign')
  assert.equal(capacityCalendarEndpoint(), '/api/production-capacity-calendar')
  assert.deepEqual(scheduleViewModes().map((item) => item.value), ['list', 'calendar', 'gantt', 'capacity'])

  assert.deepEqual(buildScheduleAssignmentPayload({
    work_order_id: '88',
    job_card_id: '91',
    work_center: ' 印刷线 ',
    planned_start_at: ' 2026-06-13 09:00 ',
    planned_end_at: '2026-06-13 11:30',
    shift_code: ' 早班 ',
    assigned_to: ' 王师傅 ',
    priority: '2',
    note: ' 插单优先 ',
  }), {
    work_order_id: 88,
    job_card_id: 91,
    work_center: '印刷线',
    planned_start_at: '2026-06-13 09:00',
    planned_end_at: '2026-06-13 11:30',
    shift_code: '早班',
    assigned_to: '王师傅',
    priority: 2,
    note: '插单优先',
  })

  assert.deepEqual(buildCapacityCalendarPayload({
    work_center: ' 印刷线 ',
    work_date: ' 2026-06-13 ',
    shift_code: ' 早班 ',
    available_minutes: '480',
    downtime_minutes: '30',
    note: ' 设备保养 ',
  }), {
    work_center: '印刷线',
    work_date: '2026-06-13',
    shift_code: '早班',
    available_minutes: 480,
    downtime_minutes: 30,
    note: '设备保养',
  })
})

test('ProductionScheduleView exposes list calendar gantt capacity and conflict workflow', () => {
  const source = readFileSync(new URL('../views/ProductionScheduleView.vue', import.meta.url), 'utf8')
  for (const marker of [
    '生产排程工作台',
    '/api/production-schedule',
    '/api/production-schedule/assign',
    '/api/production-capacity-calendar',
    '/api/mrp/suggestions',
    'MRP',
    '采购建议',
    '调拨建议',
    '列表',
    '日历',
    '甘特',
    '工位负载',
    '冲突',
    '保存排程',
    '保存产能',
  ]) {
    assert.ok(source.includes(marker), `ProductionScheduleView.vue should include ${marker}`)
  }
})
