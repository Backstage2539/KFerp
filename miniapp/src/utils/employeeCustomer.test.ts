import { describe, expect, it } from 'vitest'
import {
  mergeEmployeeCustomerRecipientFields,
  snapshotEmployeeCustomerRecipientFields,
} from './employeeCustomer'

describe('employee customer recipient fields', () => {
  it('fills only recipient fields and never derives the customer name from the recipient', () => {
    expect(mergeEmployeeCustomerRecipientFields(
      { name: '', contact: '', phone: '', address: '' },
      {
        recipient_name: '张三',
        phone: '13800138000',
        address: '云南省普洱市思茅区咖啡路 88 号',
      },
    )).toEqual({
      contact: '张三',
      phone: '13800138000',
      address: '云南省普洱市思茅区咖啡路 88 号',
    })
  })

  it('keeps the customer name and preserves fields omitted by a partial parse', () => {
    const current = {
      name: '原客户名',
      contact: '原联系人',
      phone: '021-12345678',
      address: '原地址',
    }
    expect(mergeEmployeeCustomerRecipientFields(
      current,
      { recipient_name: '  李四  ', phone: '', address: '  ' },
    )).toEqual({
      contact: '李四',
      phone: '021-12345678',
      address: '原地址',
    })
    expect(current).toEqual({
      name: '原客户名',
      contact: '原联系人',
      phone: '021-12345678',
      address: '原地址',
    })
  })

  it('does not overwrite target fields changed manually while parsing is pending', () => {
    const started = snapshotEmployeeCustomerRecipientFields({
      contact: '原联系人',
      phone: '021-12345678',
      address: '原地址',
    })
    expect(mergeEmployeeCustomerRecipientFields(
      {
        contact: '手工联系人',
        phone: '021-12345678',
        address: '手工地址',
      },
      {
        recipient_name: '解析联系人',
        phone: '13800138000',
        address: '解析地址',
      },
      started,
    )).toEqual({
      contact: '手工联系人',
      phone: '13800138000',
      address: '手工地址',
    })
  })
})
