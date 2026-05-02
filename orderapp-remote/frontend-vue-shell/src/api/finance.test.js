import test from 'node:test'
import assert from 'node:assert/strict'

import {
  closeFinanceMonth,
  createFinanceAdjustment,
  createFinanceExpense,
  fetchFinanceDashboard,
  fetchFinanceExpenses,
  fetchFinanceReport,
  fetchFinanceSettings,
  saveFinanceSettings,
  switchFinanceClosingMode,
} from './finance.js'

function withMockFetch(fn) {
  const previousWindow = globalThis.window
  const previousFetch = globalThis.fetch
  const requests = []
  globalThis.window = {
    location: { origin: 'https://erp.qacoohee.com' },
    localStorage: { getItem: () => '' },
  }
  globalThis.fetch = async (url, init = {}) => {
    requests.push({ url, init })
    return new Response(JSON.stringify({ ok: true }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  return Promise.resolve()
    .then(() => fn(requests))
    .finally(() => {
      globalThis.window = previousWindow
      globalThis.fetch = previousFetch
    })
}

test('finance API wrappers use the month-scoped dashboard, report and expense endpoints', async () => {
  await withMockFetch(async (requests) => {
    await fetchFinanceDashboard('2026-05')
    await fetchFinanceReport('2026-05')
    await fetchFinanceExpenses('2026-05')
    assert.deepEqual(requests.map((req) => req.url), [
      'https://erp.qacoohee.com/api/finance/dashboard?month=2026-05',
      'https://erp.qacoohee.com/api/finance/reports/2026-05',
      'https://erp.qacoohee.com/api/finance/expenses?month=2026-05',
    ])
  })
})

test('finance API wrappers send settings, close-mode, expenses, close and adjustment commands', async () => {
  await withMockFetch(async (requests) => {
    await fetchFinanceSettings()
    await saveFinanceSettings({ taxpayer_type: 'general' })
    await switchFinanceClosingMode('light_confirmation')
    await createFinanceExpense({ date: '2026-05-02', category: '物流', amount: 100, allocation: 'period_expense' })
    await closeFinanceMonth('2026-05')
    await createFinanceAdjustment({ month: '2026-05', type: 'expense', amount: 50, reason: '补录费用' })

    assert.deepEqual(requests.map((req) => [new URL(req.url).pathname, req.init.method || 'GET']), [
      ['/api/finance/settings', 'GET'],
      ['/api/finance/settings', 'POST'],
      ['/api/finance/settings/closing-mode', 'POST'],
      ['/api/finance/expenses', 'POST'],
      ['/api/finance/reports/2026-05/close', 'POST'],
      ['/api/finance/adjustments', 'POST'],
    ])
    assert.equal(JSON.parse(requests[2].init.body).mode, 'light_confirmation')
  })
})
