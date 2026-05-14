import assert from 'node:assert/strict'
import { test } from 'node:test'
import { orderListScopeForRequest, validOrderListScopes } from './order-scope.js'

test('order list scope preserves invalid route values so the API can fail closed', () => {
  assert.deepEqual(validOrderListScopes, ['all', 'mine', 'fulfillment'])
  assert.equal(orderListScopeForRequest('mine'), 'mine')
  assert.equal(orderListScopeForRequest(' fulfillment '), 'fulfillment')
  assert.equal(orderListScopeForRequest(''), 'all')
  assert.equal(orderListScopeForRequest(null), 'all')
  assert.equal(orderListScopeForRequest('fulfillment_typo'), 'fulfillment_typo')
})
