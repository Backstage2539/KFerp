import test from 'node:test'
import assert from 'node:assert/strict'

import {
  customerTypeLabel,
  defaultCapabilityTemplateForCustomerType,
  normalizeCustomerType,
  validCustomerType,
} from './customer-types.js'

test('customer type helpers include channel customers without treating them as permissions', () => {
  assert.equal(normalizeCustomerType('channel'), 'channel')
  assert.equal(validCustomerType('channel'), true)
  assert.equal(customerTypeLabel('channel'), '渠道客户')
  assert.equal(defaultCapabilityTemplateForCustomerType('channel'), 'channel_direct_ship')
  assert.equal(defaultCapabilityTemplateForCustomerType('wholesale'), 'processing_fulfillment')
  assert.equal(defaultCapabilityTemplateForCustomerType('retail'), 'retail_mall')
})
