import test from 'node:test'
import assert from 'node:assert/strict'

import { apiURL } from './client.js'

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

test('apiURL leaves absolute URLs unchanged', () => {
  assert.equal(apiURL('https://example.com/api'), 'https://example.com/api')
})
