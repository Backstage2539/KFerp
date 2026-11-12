import test from 'node:test'
import assert from 'node:assert/strict'
import { buildDailyChart, ledgerRoute } from './finance-daily.js'

test('daily chart leaves missing cost and profit as gaps while revenue remains continuous', () => {
  const chart = buildDailyChart([
    { date: '2026-11-01', revenue: 100, cost_expenses: 60, profit: 40 },
    { date: '2026-11-02', revenue: 20, cost_expenses: null, profit: null },
    { date: '2026-11-03', revenue: 0, cost_expenses: 15, profit: -15 },
  ])
  assert.equal(chart.series.revenue.length, 1)
  assert.equal(chart.series.revenue[0].length, 3)
  assert.deepEqual(chart.series.cost_expenses.map(s => s.length), [1, 1])
  assert.deepEqual(chart.series.profit.map(s => s.length), [1, 1])
  assert.ok(chart.min < 0)
})

test('ledger navigation accepts only internal Vue routes and preserves return context', () => {
  assert.deepEqual(ledgerRoute('/app/vue-shell?view=orders&order_id=17'), { key: 'orders', params: { order_id: '17' }, returnNavigation: { key: 'financeDashboard', label: '返回财务首页' } })
  assert.equal(ledgerRoute('https://other.example/vue-shell?view=orders'), null)
})

test('fee and expense sources open the correct workspace and restore finance context', () => {
  const back = { month: '2026-10', date: '2026-10-08' }
  const fee = ledgerRoute('/app/vue-shell?view=customerAccount&customer_id=42', back)
  assert.equal(fee.key, 'customerFulfillment')
  assert.deepEqual(fee.params, { customer_id: '42', tab: 'fees' })
  assert.deepEqual(fee.customerContext, { customerID: 42 })
  assert.deepEqual(fee.returnNavigation.viewContext, { type: 'factory' })
  assert.deepEqual(fee.returnNavigation.params, back)
  assert.equal(ledgerRoute('/app/vue-shell?view=customerAccount&customer_id=no', back), null)
  const expense = ledgerRoute('/app/vue-shell?view=financeExpenses', back, { source_type: 'expense', source_id: 17 })
  assert.deepEqual(expense.params, { month: '2026-10', expense_id: '17' })
})
