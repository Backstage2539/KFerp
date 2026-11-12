import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

test('orders and expenses present the standard return-to-source control', () => {
  for (const name of ['OrdersView.vue', 'FinanceExpensesView.vue']) {
    const source = readFileSync(new URL(`../views/${name}`, import.meta.url), 'utf8')
    assert.match(source, /<ProductionReturnLink[^>]+return_navigation/)
    assert.match(source, /import ProductionReturnLink/)
  }
  const expenses = readFileSync(new URL('../views/FinanceExpensesView.vue', import.meta.url), 'utf8')
  assert.match(expenses, /viewParams.*default/)
  assert.match(expenses, /sourceExpenseId/)
})
