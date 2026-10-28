import { shanghaiToday } from './employeeOrder'

export type CustomerFeePeriod = 'current' | 'previous' | 'custom' | 'unpaid'

export function customerFeeRange(mode: 'current' | 'previous' | 'unpaid', today = shanghaiToday()): { date_from?: string; date_to?: string; unpaid_only?: boolean } {
  if (mode === 'unpaid') return { unpaid_only: true }
  const [year, month] = today.split('-').map(Number)
  const offset = mode === 'previous' ? -1 : 0
  const from = new Date(Date.UTC(year, month - 1 + offset, 1))
  const to = new Date(Date.UTC(year, month + offset, 0))
  return { date_from: from.toISOString().slice(0, 10), date_to: to.toISOString().slice(0, 10) }
}

export function validCustomerFeeDates(from: string, to: string): boolean {
  const valid = (value: string) => /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(`${value}T00:00:00Z`)) && new Date(`${value}T00:00:00Z`).toISOString().slice(0, 10) === value
  return valid(from) && valid(to) && from <= to
}
