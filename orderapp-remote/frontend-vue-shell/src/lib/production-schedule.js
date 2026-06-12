export function productionScheduleEndpoint(filters = {}) {
  const params = new URLSearchParams()
  for (const key of ['from', 'to', 'work_center', 'status', 'limit']) {
    const value = String(filters?.[key] ?? '').trim()
    if (value) params.set(key, value)
  }
  const qs = params.toString()
  return qs ? `/api/production-schedule?${qs}` : '/api/production-schedule'
}

export function mrpSuggestionsEndpoint(filters = {}) {
  const params = new URLSearchParams()
  for (const key of ['from', 'to', 'work_center', 'status', 'material_id', 'limit']) {
    const value = String(filters?.[key] ?? '').trim()
    if (value) params.set(key, value)
  }
  const qs = params.toString()
  return qs ? `/api/mrp/suggestions?${qs}` : '/api/mrp/suggestions'
}

export function scheduleAssignEndpoint() {
  return '/api/production-schedule/assign'
}

export function capacityCalendarEndpoint() {
  return '/api/production-capacity-calendar'
}

export function scheduleViewModes() {
  return [
    { value: 'list', label: '列表' },
    { value: 'calendar', label: '日历' },
    { value: 'gantt', label: '甘特' },
    { value: 'capacity', label: '工位负载' },
  ]
}

function intValue(value) {
  const n = Number(value)
  return Number.isFinite(n) ? Math.trunc(n) : 0
}

function textValue(value) {
  return String(value ?? '').trim()
}

export function buildScheduleAssignmentPayload(draft = {}) {
  return {
    work_order_id: intValue(draft.work_order_id),
    job_card_id: intValue(draft.job_card_id),
    work_center: textValue(draft.work_center),
    planned_start_at: textValue(draft.planned_start_at),
    planned_end_at: textValue(draft.planned_end_at),
    shift_code: textValue(draft.shift_code),
    assigned_to: textValue(draft.assigned_to),
    priority: intValue(draft.priority),
    note: textValue(draft.note),
  }
}

export function buildCapacityCalendarPayload(draft = {}) {
  return {
    work_center: textValue(draft.work_center),
    work_date: textValue(draft.work_date),
    shift_code: textValue(draft.shift_code),
    available_minutes: intValue(draft.available_minutes),
    downtime_minutes: intValue(draft.downtime_minutes),
    note: textValue(draft.note),
  }
}

export function scheduleStatusLabel(status) {
  return ({
    draft: '草稿',
    released: '未开工',
    running: '生产中',
    partially_completed: '部分完成',
    completed: '已完成',
    cancelled: '已取消',
    pending: '待执行',
    ready: '可执行',
    paused: '已暂停',
  })[String(status || '').trim()] || status || '-'
}
