const weekdayLabels = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']

function localDate(value) {
  const [year, month, day] = String(value || '').split('-').map(Number)
  return new Date(year, month - 1, day, 12, 0, 0)
}

function dateText(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function buildWeekDays(value) {
  const date = localDate(value || dateText(new Date()))
  const offset = (date.getDay() + 6) % 7
  date.setDate(date.getDate() - offset)
  return weekdayLabels.map((label, index) => {
    const next = new Date(date)
    next.setDate(date.getDate() + index)
    return { date: dateText(next), label, short: `${next.getMonth() + 1}/${next.getDate()}` }
  })
}

export function attendanceLabel(status) {
  return ({ working: '上班', off: '休息', unplanned: '未排班' })[status] || '未排班'
}

export function attendanceClass(status) {
  return ({ working: 'working', off: 'off', unplanned: 'unplanned' })[status] || 'unplanned'
}

export function workstationOwnerState(row = {}) {
  if (row.override_invalid) return 'invalid_override'
  if (!Number(row.employee_id || 0)) return 'unattended'
  return row.source === 'override' ? 'override' : 'automatic'
}

export function previewWorkstationOwner(primaryEmployeeID, backupEmployeeIDs = [], attendance = {}, overrideEmployeeID = 0, employees = []) {
  const ids = [Number(primaryEmployeeID || 0), ...backupEmployeeIDs.map(Number)].filter((id, index, rows) => id > 0 && rows.indexOf(id) === index)
  const employeeByID = new Map(employees.map(row => [Number(row.id), row]))
  const eligible = id => ids.includes(Number(id)) && employeeByID.get(Number(id))?.active !== false && employeeByID.get(Number(id))?.account_type !== 'channel_customer'
  const working = id => attendance[Number(id)] === 'working'
  const overrideID = Number(overrideEmployeeID || 0)
  if (overrideID > 0) {
    if (!eligible(overrideID) || !working(overrideID)) return { employee_id: 0, employee_name: '', source: 'override', unattended: true, override_invalid: true }
    return { employee_id: overrideID, employee_name: employeeByID.get(overrideID)?.name || '', source: 'override', unattended: false, override_invalid: false }
  }
  for (const [index, id] of ids.entries()) {
    if (eligible(id) && working(id)) return { employee_id: id, employee_name: employeeByID.get(id)?.name || '', source: index === 0 ? 'primary' : 'backup', unattended: false, override_invalid: false }
  }
  return { employee_id: 0, employee_name: '', source: 'automatic', unattended: true, override_invalid: false }
}

export function copyPreviousWeekEntries(entries = [], targetWeekStart) {
  const targetDays = buildWeekDays(targetWeekStart)
  const sourceDates = [...new Set(entries.map(row => row.work_date))].sort()
  const sourceIndex = new Map(sourceDates.map((date, index) => [date, index]))
  return entries.map(row => ({
    employee_id: Number(row.employee_id),
    work_date: targetDays[sourceIndex.get(row.work_date) || 0]?.date,
    status: row.status || 'unplanned',
  })).filter(row => row.work_date)
}

export function rosterSavePayload(week, entries, overrides, requestID) {
  return {
    week_start: week.week_start,
    expected_version: Number(week.version || 0),
    request_id: requestID,
    entries: entries.map(row => ({ employee_id: Number(row.employee_id), work_date: row.work_date, status: row.status || 'unplanned' })),
    overrides: overrides.map(row => ({ workstation_id: Number(row.workstation_id), work_date: row.work_date, employee_id: Number(row.employee_id) })),
  }
}

export function nextAttendance(status) {
  return ({ unplanned: 'working', working: 'off', off: 'unplanned' })[status] || 'working'
}
