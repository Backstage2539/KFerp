import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildPublicationSummaryURL,
  createInFlightRequestDeduper,
  createLatestRequestGate,
} from './publication-summary-client.js'

test('identical publication summary requests share one in-flight call', async () => {
  let calls = 0
  let resolveRequest
  const request = createInFlightRequestDeduper(() => {
    calls += 1
    return new Promise((resolve) => { resolveRequest = resolve })
  })
  const first = request('/summary?page=1')
  const second = request('/summary?page=1')
  assert.equal(calls, 1)
  resolveRequest({ rows: [{ id: 1 }] })
  assert.deepEqual(await first, { rows: [{ id: 1 }] })
  assert.deepEqual(await second, { rows: [{ id: 1 }] })
  const third = request('/summary?page=1')
  assert.equal(calls, 2, 'completed responses are not retained as a browser result cache')
  resolveRequest({ rows: [] })
  await third
})

test('latest request gate rejects an older response for the same list', () => {
  const gate = createLatestRequestGate()
  const oldRevision = gate.begin('official:commercial:active')
  const newRevision = gate.begin('official:commercial:active')
  assert.equal(gate.isCurrent('official:commercial:active', oldRevision), false)
  assert.equal(gate.isCurrent('official:commercial:active', newRevision), true)
  assert.equal(gate.isCurrent('official:commercial:archived', newRevision), false)
})

test('publication summary URL sends server paging status and search', () => {
  const url = buildPublicationSummaryURL({
    listType: 'commercial',
    scope: 'customer',
    customerID: 450,
    publicationPurpose: 'factory_supply',
    classificationTemplateID: 8000000000000056,
    status: 'archived',
    page: 3,
    pageSize: 20,
    search: '曲奇',
  })
  const parsed = new URL(url, 'https://erp.example')
  assert.equal(parsed.pathname, '/api/costing/bean-list/publications')
  assert.equal(parsed.searchParams.get('view'), 'summary')
  assert.equal(parsed.searchParams.get('scope'), 'customer')
  assert.equal(parsed.searchParams.get('customer_id'), '450')
  assert.equal(parsed.searchParams.get('status'), 'archived')
  assert.equal(parsed.searchParams.get('page'), '3')
  assert.equal(parsed.searchParams.get('page_size'), '20')
  assert.equal(parsed.searchParams.get('search'), '曲奇')
})
