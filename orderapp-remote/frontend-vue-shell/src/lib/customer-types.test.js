import test from 'node:test'
import assert from 'node:assert/strict'

import {
  customerTypeLabel,
  normalizeCustomerType,
  validCustomerType,
} from './customer-types.js'

test('customer type helpers include channel customers without treating them as permissions', () => {
  assert.equal(normalizeCustomerType('channel'), 'channel')
  assert.equal(validCustomerType('channel'), true)
  assert.equal(customerTypeLabel('channel'), '渠道客户')
})
