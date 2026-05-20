import test from 'node:test'
import assert from 'node:assert/strict'

import { fetchUISettings, saveUISettings } from './ui-settings.js'

test('fetchUISettings reads the shared UI settings endpoint', async () => {
  const previousFetch = globalThis.fetch
  const previousWindow = globalThis.window
  let requestedUrl = ''
  globalThis.window = {
    location: { origin: 'http://erp.test', pathname: '/app/vue-shell' },
    localStorage: { getItem: () => 'token-1' },
  }
  globalThis.fetch = async (url, options) => {
    requestedUrl = String(url)
    assert.equal(options.headers.Authorization, 'Bearer token-1')
    return {
      ok: true,
      status: 200,
      json: async () => ({ settings: { hide_customer_account_fulfillment: true } }),
    }
  }
  try {
    const data = await fetchUISettings()
    assert.equal(requestedUrl, 'http://erp.test/app/api/ui-settings')
    assert.deepEqual(data.settings, { hide_customer_account_fulfillment: true })
  } finally {
    globalThis.fetch = previousFetch
    globalThis.window = previousWindow
  }
})

test('saveUISettings persists customer account fulfillment visibility', async () => {
  const previousFetch = globalThis.fetch
  const previousWindow = globalThis.window
  let body = ''
  globalThis.window = {
    location: { origin: 'http://erp.test', pathname: '/app/vue-shell' },
    localStorage: { getItem: () => 'token-1' },
  }
  globalThis.fetch = async (url, options) => {
    assert.equal(String(url), 'http://erp.test/app/api/ui-settings')
    assert.equal(options.method, 'PUT')
    body = options.body
    return {
      ok: true,
      status: 200,
      json: async () => ({ settings: { hide_customer_account_fulfillment: false } }),
    }
  }
  try {
    const data = await saveUISettings({ hide_customer_account_fulfillment: false })
    assert.deepEqual(JSON.parse(body), { hide_customer_account_fulfillment: false })
    assert.equal(data.settings.hide_customer_account_fulfillment, false)
  } finally {
    globalThis.fetch = previousFetch
    globalThis.window = previousWindow
  }
})
