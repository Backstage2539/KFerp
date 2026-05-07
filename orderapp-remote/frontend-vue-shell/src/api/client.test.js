import test from 'node:test'
import assert from 'node:assert/strict'

import { apiFetch, apiGet, apiSend, apiURL } from './client.js'

test('apiURL builds relative API requests from clean origin even when page URL has basic-auth credentials', () => {
  const previousWindow = globalThis.window
  globalThis.window = {
    location: {
      origin: 'https://erp.qacoohee.com',
      href: 'https://order:secret@erp.qacoohee.com/vue-shell?view=userPermissions',
    },
  }
  try {
    assert.equal(apiURL('/api/auth/me'), 'https://erp.qacoohee.com/api/auth/me')
  } finally {
    globalThis.window = previousWindow
  }
})

test('apiURL keeps /app prefix when Vue shell is opened from production app path', () => {
  const previousWindow = globalThis.window
  globalThis.window = {
    location: {
      origin: 'https://erp.qacoohee.com',
      pathname: '/app/vue-shell',
      href: 'https://erp.qacoohee.com/app/vue-shell?view=orders',
    },
  }
  try {
    assert.equal(apiURL('/api/auth/me'), 'https://erp.qacoohee.com/app/api/auth/me')
    const url = new URL('/api/orders?page=1', 'https://erp.qacoohee.com')
    assert.equal(apiURL(url), 'https://erp.qacoohee.com/app/api/orders?page=1')
  } finally {
    globalThis.window = previousWindow
  }
})

test('apiURL leaves absolute URLs unchanged', () => {
  assert.equal(apiURL('https://example.com/api'), 'https://example.com/api')
})

test('apiURL accepts same-origin URL objects from view query builders', () => {
  const previousWindow = globalThis.window
  globalThis.window = { location: { origin: 'https://erp.qacoohee.com' } }
  try {
    const url = new URL('/api/customers?page=1', 'https://erp.qacoohee.com')
    assert.equal(apiURL(url), 'https://erp.qacoohee.com/api/customers?page=1')
  } finally {
    globalThis.window = previousWindow
  }
})

test('apiGet sends Bearer token from localStorage', async () => {
  const previousWindow = globalThis.window
  const previousFetch = globalThis.fetch
  let requestHeaders
  globalThis.window = {
    location: { origin: 'https://erp.qacoohee.com' },
    localStorage: { getItem: (key) => (key === 'auth_token' ? 'token-13800138075' : null) },
  }
  globalThis.fetch = async (_url, init = {}) => {
    requestHeaders = init.headers
    return new Response(JSON.stringify({ ok: true }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  try {
    await apiGet('/api/auth/me')
    assert.equal(requestHeaders.Authorization, 'Bearer token-13800138075')
  } finally {
    globalThis.window = previousWindow
    globalThis.fetch = previousFetch
  }
})

test('apiSend preserves custom headers while sending Bearer token', async () => {
  const previousWindow = globalThis.window
  const previousFetch = globalThis.fetch
  let requestHeaders
  globalThis.window = {
    location: { origin: 'https://erp.qacoohee.com' },
    localStorage: { getItem: (key) => (key === 'auth_token' ? 'token-logout' : null) },
  }
  globalThis.fetch = async (_url, init = {}) => {
    requestHeaders = init.headers
    return new Response(JSON.stringify({ ok: true }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  try {
    await apiSend('/api/auth/logout', { headers: { 'X-Test': '1' } })
    assert.equal(requestHeaders.Authorization, 'Bearer token-logout')
    assert.equal(requestHeaders['X-Test'], '1')
  } finally {
    globalThis.window = previousWindow
    globalThis.fetch = previousFetch
  }
})

test('apiFetch sends Bearer token while returning the raw response for file and form flows', async () => {
  const previousWindow = globalThis.window
  const previousFetch = globalThis.fetch
  let requestURL = ''
  let requestHeaders = {}
  let requestBody
  globalThis.window = {
    location: { origin: 'https://erp.qacoohee.com' },
    localStorage: { getItem: (key) => (key === 'auth_token' ? 'token-download' : null) },
  }
  globalThis.fetch = async (url, init = {}) => {
    requestURL = url
    requestHeaders = init.headers
    requestBody = init.body
    return new Response('xlsx', { status: 200, headers: { 'Content-Type': 'application/octet-stream' } })
  }
  try {
    const form = new FormData()
    form.append('file', new Blob(['a']), 'tracking.xlsx')
    const res = await apiFetch('/api/orders/shipping-tracking-excel', { method: 'POST', body: form })
    assert.equal(requestURL, 'https://erp.qacoohee.com/api/orders/shipping-tracking-excel')
    assert.equal(requestHeaders.Authorization, 'Bearer token-download')
    assert.equal(requestHeaders['Content-Type'], undefined)
    assert.equal(requestBody, form)
    assert.equal(res.status, 200)
  } finally {
    globalThis.window = previousWindow
    globalThis.fetch = previousFetch
  }
})
