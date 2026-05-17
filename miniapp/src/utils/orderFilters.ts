export type OrderDatePreset = 'today' | 'last3' | 'last7' | 'week' | 'month' | 'year'

export type OrderDateRange = {
  date_from?: string
  date_to?: string
}

export type OrderFilterForm = OrderDateRange & {
  keyword?: string
  process_status?: string
  pay_status?: string
  ship_status?: string
}

export type OrderServiceFilters = {
  q?: string
  date_from?: string
  date_to?: string
  process_status?: string
  pay_status?: string
  ship_status?: string
}

export function datePresetRange(preset: OrderDatePreset, now = new Date()): Required<OrderDateRange> {
  const end = startOfLocalDay(now)
  const start = new Date(end)
  if (preset === 'last3') {
    start.setDate(end.getDate() - 2)
  } else if (preset === 'last7') {
    start.setDate(end.getDate() - 6)
  } else if (preset === 'week') {
    const day = end.getDay()
    const mondayOffset = (day + 6) % 7
    start.setDate(end.getDate() - mondayOffset)
  } else if (preset === 'month') {
    start.setDate(1)
  } else if (preset === 'year') {
    start.setMonth(0, 1)
  }
  return {
    date_from: formatDate(start),
    date_to: formatDate(end),
  }
}

export function normalizeDateRange(dateFrom?: string, dateTo?: string): OrderDateRange {
  const from = validDateString(dateFrom)
  const to = validDateString(dateTo)
  if (from && to && from > to) {
    return { date_from: to, date_to: from }
  }
  return {
    ...(from ? { date_from: from } : {}),
    ...(to ? { date_to: to } : {}),
  }
}

export function buildOrderServiceFilters(form: OrderFilterForm): OrderServiceFilters {
  const range = normalizeDateRange(form.date_from, form.date_to)
  const keyword = (form.keyword || '').trim().replace(/\s+/g, ' ')
  const processStatus = normalizeStatusFilter(form.process_status)
  const payStatus = normalizeStatusFilter(form.pay_status)
  const shipStatus = normalizeStatusFilter(form.ship_status)
  return {
    ...(keyword ? { q: keyword } : {}),
    ...range,
    ...(processStatus ? { process_status: processStatus } : {}),
    ...(payStatus ? { pay_status: payStatus } : {}),
    ...(shipStatus ? { ship_status: shipStatus } : {}),
  }
}

function startOfLocalDay(value: Date): Date {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate())
}

function formatDate(value: Date): string {
  const y = value.getFullYear()
  const m = String(value.getMonth() + 1).padStart(2, '0')
  const d = String(value.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function validDateString(value?: string): string {
  const text = (value || '').trim()
  return /^\d{4}-\d{2}-\d{2}$/.test(text) ? text : ''
}

function normalizeStatusFilter(value?: string): string {
  return (value || '').trim().replace(/\s+/g, ' ')
}
