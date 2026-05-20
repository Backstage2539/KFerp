import { apiGet, apiSend } from './client.js'
import { currentMonth } from '../lib/finance.js'

function monthValue(month) {
  return encodeURIComponent(month || currentMonth())
}

export function fetchFinanceSettings() {
  return apiGet('/api/finance/settings')
}

export function saveFinanceSettings(payload) {
  return apiSend('/api/finance/settings', { body: payload })
}

export function switchFinanceClosingMode(mode) {
  return apiSend('/api/finance/settings/closing-mode', { body: { mode } })
}

export function fetchFinanceDashboard(month) {
  return apiGet(`/api/finance/dashboard?month=${monthValue(month)}`)
}

export function fetchFinanceExpenses(month, employeeId = 0, customerId = 0) {
  const params = new URLSearchParams({ month: month || currentMonth() })
  if (Number(employeeId) > 0) {
    params.set('employee_id', String(Number(employeeId)))
  }
  if (Number(customerId) > 0) {
    params.set('customer_id', String(Number(customerId)))
  }
  return apiGet(`/api/finance/expenses?${params.toString()}`)
}

export function createFinanceExpense(payload) {
  return apiSend('/api/finance/expenses', { body: payload })
}

export function fetchFinanceReport(month) {
  return apiGet(`/api/finance/reports/${monthValue(month)}`)
}

export function fetchFinanceClosingReview(month) {
  return apiGet(`/api/finance/reports/${monthValue(month)}/closing-review`)
}

export function fetchFinanceReportDrilldown(month) {
  return apiGet(`/api/finance/reports/${monthValue(month)}/drilldown`)
}

export function fetchFinanceTaxLedger(month) {
  return apiGet(`/api/finance/tax-ledger?month=${monthValue(month)}`)
}

export function saveFinanceTaxLedgerEntry(payload) {
  return apiSend('/api/finance/tax-ledger', { body: payload })
}

export function closeFinanceMonth(month) {
  return apiSend(`/api/finance/reports/${monthValue(month)}/close`, { body: {} })
}

export function createFinanceAdjustment(payload) {
  return apiSend('/api/finance/adjustments', { body: payload })
}
