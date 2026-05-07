import test from 'node:test'
import assert from 'node:assert/strict'

import { clearStoredAuthToken, hasStoredAuthToken } from './auth.js'

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
