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
    'FinanceTaxLedgerView',
    'FinanceSettingsView',
    'OutsourceSettingsView',
    'OperationManualView',
  ]) {
    assert.ok(src.includes(name), `App.vue missing ${name}`)
  }
  for (const key of ['financeDashboard', 'financeExpenses', 'processingBilling', 'financeClosing', 'financeReport', 'financeTaxLedger', 'financeSettings', 'financeManual']) {
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

test('Finance expenses exposes searchable category and payment option lists', () => {
  const src = read('src/views/FinanceExpensesView.vue')
  assert.ok(src.includes('expenseCategoryOptions'))
  assert.ok(src.includes('expensePaymentOptions'))
  assert.ok(src.includes('filterExpenseOptions'))
  assert.ok(src.includes('filteredExpenseCategoryOptions'))
  assert.ok(src.includes('filteredExpensePaymentOptions'))
  assert.ok(src.includes('list="finance-expense-category-options"'))
  assert.ok(src.includes('list="finance-expense-payment-options"'))
  assert.ok(src.includes('<datalist id="finance-expense-category-options"'))
  assert.ok(src.includes('<datalist id="finance-expense-payment-options"'))
})

test('Finance improvements expose closing review, drilldown, tax ledger and accountant handoff in Vue', () => {
  const menu = read('src/lib/menu-ia.js')
  const report = read('src/views/FinanceReportView.vue')
  const closing = read('src/views/FinanceClosingView.vue')
  const expenses = read('src/views/FinanceExpensesView.vue')
  const taxLedger = read('src/views/FinanceTaxLedgerView.vue')
  const api = read('src/api/finance.js')

  assert.ok(menu.includes('financeTaxLedger'))
  assert.ok(report.includes('fetchFinanceReportDrilldown'))
  assert.ok(report.includes('accountant-handoff.xlsx'))
  assert.ok(report.includes('row.payment_method'))
  assert.ok(closing.includes('fetchFinanceClosingReview'))
  assert.ok(expenses.includes('order_id'))
  assert.ok(expenses.includes('dimension_note'))
  assert.ok(taxLedger.includes('saveFinanceTaxLedgerEntry'))
  assert.ok(api.includes('/closing-review'))
  assert.ok(api.includes('/drilldown'))
  assert.ok(api.includes('/tax-ledger'))
})
