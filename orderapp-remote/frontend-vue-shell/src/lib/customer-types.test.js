import test from 'node:test'
import assert from 'node:assert/strict'

import {
  customerTypeLabel,
  mergeCustomerTypeOptions,
  normalizeCustomerType,
  validCustomerType,
} from './customer-types.js'

test('customer type helpers include channel customers without treating them as permissions', () => {
  assert.equal(normalizeCustomerType('channel'), 'channel')
  assert.equal(validCustomerType('channel'), true)
  assert.equal(customerTypeLabel('channel'), '渠道客户')
})

test('customer type helpers accept backend custom type options', () => {
  const options = mergeCustomerTypeOptions([
    { value: 'partner_store', label: '联名门店' },
    { value: 'channel', label: '渠道客户' },
  ])
  assert.equal(normalizeCustomerType('partner_store', options), 'partner_store')
  assert.equal(validCustomerType('partner_store', options), true)
  assert.equal(customerTypeLabel('partner_store', options), '联名门店')
  assert.equal(normalizeCustomerType('missing_type', options), 'retail')
})
