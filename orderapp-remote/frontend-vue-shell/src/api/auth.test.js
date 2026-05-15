import test from 'node:test'
import assert from 'node:assert/strict'

import { clearStoredAuthToken, fetchInternalAuthAccounts, hasStoredAuthToken } from './auth.js'

test('hasStoredAuthToken checks localStorage auth token before loading the Vue shell actor', () => {
  const previousWindow = globalThis.window
  globalThis.window = {
    localStorage: { getItem: (key) => (key === 'auth_token' ? 'token-13800138075' : null) },
  }
  try {
    assert.equal(hasStoredAuthToken(), true)
  } finally {
    globalThis.window = previousWindow
  }
})

test('clearStoredAuthToken removes local token so BasicAuth cache cannot reopen the app UI', () => {
  const previousWindow = globalThis.window
  let removed = ''
  globalThis.window = {
    localStorage: { removeItem: (key) => { removed = key } },
  }
  try {
    clearStoredAuthToken()
    assert.equal(removed, 'auth_token')
  } finally {
    globalThis.window = previousWindow
  }
})

test('fetchInternalAuthAccounts reads the internal-only account endpoint', async () => {
  const previousFetch = globalThis.fetch
  const previousWindow = globalThis.window
  let requestedUrl = ''
  globalThis.window = {
    location: { origin: 'http://erp.test', pathname: '/app' },
    localStorage: { getItem: () => 'token-13800138000' },
  }
  globalThis.fetch = async (url, options) => {
    requestedUrl = String(url)
    assert.equal(options.headers.Authorization, 'Bearer token-13800138000')
    return {
      ok: true,
      status: 200,
      json: async () => ({ rows: [] }),
    }
  }
  try {
    const data = await fetchInternalAuthAccounts()
    assert.deepEqual(data, { rows: [] })
    assert.equal(requestedUrl, 'http://erp.test/app/api/auth/internal-accounts')
  } finally {
    globalThis.fetch = previousFetch
    globalThis.window = previousWindow
  }
})
