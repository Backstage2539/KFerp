import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

function read(path) {
  return readFileSync(new URL(`../../${path}`, import.meta.url), 'utf8')
}

test('App.vue wires every finance menu key to a Vue view', () => {
  const src = read('src/App.vue')
  for (const name of [
    'FinanceDashboardView',
    'FinanceExpensesView',
    'FinanceClosingView',
    'FinanceReportView',
    'FinanceSettingsView',
    'FinanceManualView',
  ]) {
    assert.ok(src.includes(name), `App.vue missing ${name}`)
  }
  for (const key of ['financeDashboard', 'financeExpenses', 'financeClosing', 'financeReport', 'financeSettings', 'financeManual']) {
    assert.ok(src.includes(`${key}:`), `App.vue missing ${key} view mapping`)
  }
})

test('Finance settings keeps closing mode switch behind can_manage_close_mode', () => {
  const src = read('src/views/FinanceSettingsView.vue')
  assert.ok(src.includes('can_manage_close_mode'))
  assert.ok(src.includes('switchFinanceClosingMode'))
  assert.ok(src.includes('strong_lock'))
  assert.ok(src.includes('light_confirmation'))
})

test('Finance expenses keeps the list month aligned with the expense posting date', () => {
  const src = read('src/views/FinanceExpensesView.vue')
  assert.ok(src.includes('syncMonthFromDate'))
  assert.ok(src.includes('const created = await createFinanceExpense'))
  assert.ok(src.includes('created?.month'))
})

test('Finance expenses associates expenses with employees and filters when an employee is clicked', () => {
  const src = read('src/views/FinanceExpensesView.vue')
  assert.ok(src.includes("apiGet('/api/finance/employees')"))
  assert.ok(src.includes('employee_id: form.employee_id'))
  assert.ok(src.includes('employeeFilter'))
  assert.ok(src.includes('selectEmployeeFilter(row.employee_id)'))
  assert.ok(src.includes('row.employee_name'))
})
