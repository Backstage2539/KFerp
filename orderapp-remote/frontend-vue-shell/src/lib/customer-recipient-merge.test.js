import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  customerPhoneForERPForm,
  customerRecipientFieldSnapshot,
  mergeCustomerRecipientFields,
} from './customer-recipient-merge.js'

test('ERP customer phone falls back to miniapp contact phone for existing records', () => {
  assert.equal(customerPhoneForERPForm({ company_phone: '', phone: '13800138000' }), '13800138000')
  assert.equal(customerPhoneForERPForm({ company_phone: '021-12345678', phone: '13800138000' }), '021-12345678')
})

test('recipient parsing never derives a customer name', () => {
  const merged = mergeCustomerRecipientFields(
    { name: '', contact: '', company_phone: '', address: '' },
    { recipient_name: '张三', phone: '13800138000', address: '云南省普洱市' },
    customerRecipientFieldSnapshot({ contact: '', company_phone: '', address: '' }),
  )

  assert.deepEqual(merged, {
    contact: '张三',
    company_phone: '13800138000',
    address: '云南省普洱市',
  })
  assert.equal(Object.hasOwn(merged, 'name'), false)
})

test('late parsing results update only target fields unchanged since request start', () => {
  const started = customerRecipientFieldSnapshot({
    contact: '原联系人',
    company_phone: '021-12345678',
    address: '原地址',
  })
  const merged = mergeCustomerRecipientFields(
    {
      contact: '手工联系人',
      company_phone: '021-12345678',
      address: '手工地址',
    },
    {
      recipient_name: '解析联系人',
      phone: '13800138000',
      address: '解析地址',
    },
    started,
  )

  assert.deepEqual(merged, {
    contact: '手工联系人',
    company_phone: '13800138000',
    address: '手工地址',
  })
})

test('required contact fields use the personal phone and address on create', async () => {
  const {newCustomerContactError} = await import('./customer-recipient-merge.js')
  const valid = {contact:'测试联系人', phone:'021-12345678', address:'测试路 1 号', company_phone:'13800000000', company_address:'公司地址'}
  for (const [field, label] of Object.entries({contact:'联系人', phone:'联系电话', address:'联系地址'})) {
    assert.ok(newCustomerContactError({...valid, [field]:'　 '}).includes(label))
  }
  assert.equal(newCustomerContactError(valid), '')
})
