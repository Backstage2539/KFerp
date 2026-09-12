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
  assert.deepEqual(scheduleViewModes().map((item) => item.value), ['list'])

  assert.deepEqual(buildScheduleAssignmentPayload({
    work_order_id: '88',
    job_card_id: '91',
    bom_id: '11',
    bom_version_id: '101',
    process_route_id: '77',
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
  assert.equal(Object.hasOwn(buildScheduleAssignmentPayload({ bom_id: 11, bom_version_id: 101, process_route_id: 77 }), 'bom_id'), false)
  assert.equal(Object.hasOwn(buildScheduleAssignmentPayload({ bom_id: 11, bom_version_id: 101, process_route_id: 77 }), 'bom_version_id'), false)
  assert.equal(Object.hasOwn(buildScheduleAssignmentPayload({ bom_id: 11, bom_version_id: 101, process_route_id: 77 }), 'process_route_id'), false)

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

test('schedule workspace uses task selection, shared staff, batch preview and explicit conflict confirmation', () => {
  const source = readFileSync(new URL('../views/ProductionScheduleView.vue', import.meta.url), 'utf8')
  for (const marker of ['生产排程工作区', '/api/production-schedule/preview', '/api/production-schedule/batch',
    'ProductionStaffFields', '批量带入默认人员', '确认重叠并保存安排', '可用工时待配置', 'PaginationControls',
    'beforeunload', 'version_conflict', 'request_id', 'returnNavigation', '待安排', '已安排', '生产中', '历史']) {
    assert.ok(source.includes(marker), `missing workflow: ${marker}`)
  }
  assert.doesNotMatch(source, /MRP|甘特|mrpSuggestionsEndpoint|v-model[^>]+assigned_to/)
})
