import test from 'node:test'
import assert from 'node:assert/strict'
import {
  attendanceLabel,
  buildWeekDays,
  copyPreviousWeekEntries,
  rosterSavePayload,
  previewWorkstationOwner,
  workstationOwnerState,
} from './production-roster.js'

test('week is normalized to Beijing Monday through Sunday', () => {
  assert.deepEqual(buildWeekDays('2026-09-17').map(row => row.date), [
    '2026-09-14', '2026-09-15', '2026-09-16', '2026-09-17', '2026-09-18', '2026-09-19', '2026-09-20',
  ])
})

test('workstation staffing preview follows override primary and ordered backup without treating unplanned as working', () => {
  const employees = [{ id: 1, name: '主负责人', active: true }, { id: 2, name: '替补', active: true }]
  assert.equal(previewWorkstationOwner(1, [2], { 1: 'off', 2: 'working' }, 0, employees).employee_id, 2)
  assert.equal(previewWorkstationOwner(1, [2], { 1: 'working', 2: 'working' }, 0, employees).employee_id, 1)
  assert.equal(previewWorkstationOwner(1, [2], { 1: 'working', 2: 'working' }, 2, employees).source, 'override')
  assert.equal(previewWorkstationOwner(1, [2], { 1: 'unplanned', 2: 'off' }, 0, employees).unattended, true)
  assert.equal(previewWorkstationOwner(1, [2], { 1: 'working', 2: 'working' }, 3, employees).override_invalid, true)
})

test('attendance keeps working off and unplanned distinct', () => {
  assert.equal(attendanceLabel('working'), '上班')
  assert.equal(attendanceLabel('off'), '休息')
  assert.equal(attendanceLabel('unplanned'), '未排班')
  assert.equal(workstationOwnerState({ employee_id: 0, override_invalid: false }), 'unattended')
  assert.equal(workstationOwnerState({ employee_id: 0, override_invalid: true }), 'invalid_override')
})

test('copy previous week produces a reviewable current-week draft', () => {
  const copied = copyPreviousWeekEntries([
    { employee_id: 7, work_date: '2026-09-07', status: 'working' },
    { employee_id: 7, work_date: '2026-09-08', status: 'off' },
  ], '2026-09-14')
  assert.deepEqual(copied, [
    { employee_id: 7, work_date: '2026-09-14', status: 'working' },
    { employee_id: 7, work_date: '2026-09-15', status: 'off' },
  ])
})

test('save payload carries version attendance overrides and idempotency', () => {
  const payload = rosterSavePayload({ week_start: '2026-09-14', version: 3 }, [
    { employee_id: 1, work_date: '2026-09-14', status: 'working' },
  ], [{ workstation_id: 9, work_date: '2026-09-14', employee_id: 1 }], 'req-1')
  assert.deepEqual(payload, {
    week_start: '2026-09-14', expected_version: 3, request_id: 'req-1',
    entries: [{ employee_id: 1, work_date: '2026-09-14', status: 'working' }],
    overrides: [{ workstation_id: 9, work_date: '2026-09-14', employee_id: 1 }],
  })
})
