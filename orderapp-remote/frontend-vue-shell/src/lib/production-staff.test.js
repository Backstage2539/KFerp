import assert from 'node:assert/strict'
import { test } from 'node:test'
import { applyDefaultStaff, staffNames, staffPatch, validateStaff, taskArrangementState } from './production-staff.js'

test('defaults follow each operation and preserve existing assignments', () => {
 const rows = [
  { id: 1, assigned_employee_id: 0, default_employee_id: 11, default_collaborator_ids: [12] },
  { id: 2, assigned_employee_id: 21, collaborator_employee_ids: [22], default_employee_id: 11 },
  { id: 3, assigned_to: '历史姓名', assigned_employee_id: 0, default_employee_id: 11 },
 ]
 const result = applyDefaultStaff(rows)
 assert.equal(result[0].assigned_employee_id, 11)
 assert.deepEqual(result[0].collaborator_employee_ids, [])
 assert.equal(result[1].assigned_employee_id, 21)
 assert.equal(result[2].assigned_employee_id, 0)
 assert.equal(rows[0].assigned_employee_id, 0)
})
test('personnel-only changes send one responsible employee and no collaborator field', () => {
 assert.deepEqual(staffPatch({id:4,work_order_id:2,schedule_version:7,assigned_employee_id:11,collaborator_employee_ids:[12],planned_start_at:'2026-09-12 09:00',shift_code:'早班',workstation:'包装台'}),{job_card_id:4,work_order_id:2,expected_version:7,assigned_employee_id:11})
})
test('selection requires one active eligible responsible employee', () => {
 const eligible=[{id:1,active:true},{id:2,active:true},{id:3,active:false}]
 assert.equal(validateStaff(1,eligible),'')
 assert.ok(validateStaff(3,eligible))
 assert.ok(validateStaff(0,eligible))
 assert.equal(staffNames({ assigned_to: '段其晶', collaborators: [{ name: '历史协作人' }] }), '段其晶')
})
test('schedule readiness differs from manufacturing readiness', () => {
 assert.equal(taskArrangementState({status:'blocked',workstation:'包装台',assigned_employee_id:1,planned_start_at:'2026-09-12 09:00',planned_end_at:'2026-09-12 10:00'}),'scheduled')
 assert.equal(taskArrangementState({status:'pending',assigned_employee_id:1,planned_start_at:'2026-09-12 09:00',planned_end_at:'2026-09-12 10:00'}),'pending')
 assert.equal(taskArrangementState({status:'completed'}),'history')
 assert.equal(taskArrangementState({status:'pending',assigned_employee_id:1}),'pending')
})
