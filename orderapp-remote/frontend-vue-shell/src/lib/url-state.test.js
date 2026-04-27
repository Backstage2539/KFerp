import test from 'node:test'
import assert from 'node:assert/strict'

import { relativeURLForHistory } from './url-state.js'

test('relativeURLForHistory strips origin and credentials before replaceState', () => {
  const url = new URL('https://order:secret@erp.qacoohee.com/vue-shell?view=reqReview&page=1')
  assert.equal(relativeURLForHistory(url), '/vue-shell?view=reqReview&page=1')
})

test('relativeURLForHistory preserves hash fragments', () => {
  const url = new URL('http://127.0.0.1:18082/vue-shell?view=materials#detail')
  assert.equal(relativeURLForHistory(url), '/vue-shell?view=materials#detail')
})
