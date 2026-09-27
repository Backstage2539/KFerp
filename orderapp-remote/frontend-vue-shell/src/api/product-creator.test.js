import test from 'node:test'
import assert from 'node:assert/strict'

import {
  getProductCreatorModules,
  saveProductCreatorTemplate,
  saveProductCreatorRunDraft,
  previewProductCreatorRun,
  commitProductCreatorRun,
  executeProductCreatorRunStep,
} from './product-creator.js'

test('product creator API uses the shared client, /app prefix, revisions, and stable inputs', async () => {
  const previousWindow = globalThis.window
  const previousFetch = globalThis.fetch
  const calls = []
  globalThis.window = {
    location: { origin: 'https://erp.example.test', pathname: '/app/vue-shell', href: 'https://erp.example.test/app/vue-shell' },
    localStorage: { getItem: () => 'session-token' },
  }
  globalThis.fetch = async (url, init = {}) => {
    calls.push({ url: String(url), init })
    return new Response(JSON.stringify({ ok: true, rows: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  try {
    await getProductCreatorModules()
    await saveProductCreatorTemplate({ id: 4, revision: 2, name: '装配', workflow: { nodes: [], edges: [] } })
    await saveProductCreatorRunDraft(21, 3, { material: { rows: [{ row_id: 'stable-row-a' }] } })
    await previewProductCreatorRun(21, 4)
    await commitProductCreatorRun(21, 4, 'pc-run-21-rev-4')
    await executeProductCreatorRunStep(21, 'purchase-node', 5, 'create_purchase_order', {}, 'pc-run-21-purchase-node-order-rev-5')
    await executeProductCreatorRunStep(21, 'price-node', 6, 'save_price_draft', {}, 'pc-run-21-price-node-draft-rev-6')

    assert.equal(calls[0].url, 'https://erp.example.test/app/api/product-creator/modules')
    assert.equal(calls[1].url, 'https://erp.example.test/app/api/product-creator/templates/4')
    assert.equal(calls[1].init.method, 'PUT')
    assert.equal(calls[1].init.headers.Authorization, 'Bearer session-token')
    assert.equal(calls[2].url, 'https://erp.example.test/app/api/product-creator/runs/21/draft')
    assert.deepEqual(JSON.parse(calls[2].init.body), { revision: 3, inputs: { material: { rows: [{ row_id: 'stable-row-a' }] } } })
    assert.equal(calls[3].url, 'https://erp.example.test/app/api/product-creator/runs/21/preview')
    assert.deepEqual(JSON.parse(calls[3].init.body), { revision: 4 })
    assert.equal(calls[4].url, 'https://erp.example.test/app/api/product-creator/runs/21/commit')
    assert.equal(calls[4].init.headers['Idempotency-Key'], 'pc-run-21-rev-4')
    assert.deepEqual(JSON.parse(calls[4].init.body), { revision: 4 })
    assert.equal(calls[5].url, 'https://erp.example.test/app/api/product-creator/runs/21/nodes/purchase-node/execute')
    assert.equal(calls[5].init.headers['Idempotency-Key'], 'pc-run-21-purchase-node-order-rev-5')
    assert.deepEqual(JSON.parse(calls[5].init.body), { revision: 5, action: 'create_purchase_order', inputs: {} })
    assert.equal(calls[6].url, 'https://erp.example.test/app/api/product-creator/pricing/runs/21/nodes/price-node/execute')
    assert.equal(calls[6].init.headers['Idempotency-Key'], 'pc-run-21-price-node-draft-rev-6')
    assert.deepEqual(JSON.parse(calls[6].init.body), { revision: 6, action: 'save_price_draft', inputs: {} })
  } finally {
    globalThis.window = previousWindow
    globalThis.fetch = previousFetch
  }
})
