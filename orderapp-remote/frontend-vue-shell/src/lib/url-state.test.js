import test from 'node:test'
import assert from 'node:assert/strict'

import { relativeURLForHistory, viewNavigationURL } from './url-state.js'

test('relativeURLForHistory strips origin and credentials before replaceState', () => {
  const url = new URL('https://order:secret@erp.qacoohee.com/vue-shell?view=reqReview&page=1')
  assert.equal(relativeURLForHistory(url), '/vue-shell?view=reqReview&page=1')
})

test('relativeURLForHistory preserves hash fragments', () => {
  const url = new URL('http://127.0.0.1:18082/vue-shell?view=materials#detail')
  assert.equal(relativeURLForHistory(url), '/vue-shell?view=materials#detail')
})

test('viewNavigationURL clears stale view params when opening a new entry view', () => {
  const url = new URL('https://erp.qacoohee.com/vue-shell?view=customers&edit_id=153&q=%E4%B8%89%E5%BE%84&scope=orders&highlight_order_id=152')

  const next = viewNavigationURL(url, 'order')

  assert.equal(next.searchParams.get('view'), 'order')
  assert.equal(next.searchParams.has('edit_id'), false)
  assert.equal(next.searchParams.has('q'), false)
  assert.equal(next.searchParams.has('scope'), false)
  assert.equal(next.searchParams.has('highlight_order_id'), false)
})
