import test from 'node:test'
import assert from 'node:assert/strict'
import { fetchFinanceDailyLedger } from './finance.js'

test('daily ledger sends filters and pagination through authenticated client', async () => {
  const oldWindow = globalThis.window, oldFetch = globalThis.fetch
  let actual
  globalThis.window = { location: { origin: 'https://erp.qacoohee.com' }, localStorage: { getItem: () => '' } }
  globalThis.fetch = async url => { actual = new URL(url); return new Response('{"rows":[]}', { status: 200 }) }
  try {
    await fetchFinanceDailyLedger({ month: '2026-11', date: '2026-11-02', section: 'main_cost', unavailable: true, page: 2, pageSize: 20 })
    assert.equal(actual.pathname, '/api/finance/daily-ledger')
    assert.equal(actual.searchParams.get('month'), '2026-11')
    assert.equal(actual.searchParams.get('date'), '2026-11-02')
    assert.equal(actual.searchParams.get('section'), 'main_cost')
    assert.equal(actual.searchParams.get('unavailable'), 'true')
    assert.equal(actual.searchParams.get('page'), '2')
    assert.equal(actual.searchParams.get('page_size'), '20')
  } finally { globalThis.window = oldWindow; globalThis.fetch = oldFetch }
})
