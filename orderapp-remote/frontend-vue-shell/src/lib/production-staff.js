export function applyDefaultStaff(rows = []) {
 return rows.map(row => row.assigned_employee_id || row.assigned_to ? { ...row } : {
  ...row, assigned_employee_id: Number(row.default_employee_id || 0),
  collaborator_employee_ids: [],
 })
}
export function staffPatch(row = {}) {
 return { job_card_id: Number(row.job_card_id || row.id), work_order_id: Number(row.work_order_id), expected_version: Number(row.schedule_version || 0), assigned_employee_id: Number(row.assigned_employee_id || 0) }
}
export function validateStaff(lead, eligible = []) {
 const active = new Set(eligible.filter(row => row.active !== false).map(row => Number(row.id)))
 if (!Number(lead)) return '请选择负责人'
 if (!active.has(Number(lead))) return '所选员工已停用或不属于本工序的可执行员工'
 return ''
}
export function taskArrangementState(row = {}) {
 if (['completed', 'cancelled'].includes(row.status) || ['completed', 'cancelled'].includes(row.work_order_status)) return 'history'
 if (['running', 'paused', 'in_progress'].includes(row.status)) return 'running'
 return row.assigned_employee_id && row.planned_start_at && row.planned_end_at && (row.workstation || row.work_center) ? 'scheduled' : 'pending'
}
export function staffNames(row = {}) {
 return row.assigned_to || '待分配负责人'
}
