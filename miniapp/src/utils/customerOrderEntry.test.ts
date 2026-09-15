import { describe, expect, it } from 'vitest'
import { filterRecipientAddresses, recipientAddressSummary, tierQuantityLabel } from './customerOrderEntry'

describe('customer order entry presentation', () => {
  const rows = [
    { id: 1, customer_id: 9, recipient_name: '张三', phone: '13800138000', company: '春风店', province: '云南省', city: '普洱市', district: '思茅区', detail_address: '咖啡路 88 号', is_default: true, revision: 1 },
    { id: 2, customer_id: 9, recipient_name: '李四', phone: '13900139000', company: '', province: '贵州省', city: '贵阳市', district: '', detail_address: '会展路 1 号', is_default: false, revision: 1 },
  ]

  it('searches recipients by name, phone, company and address', () => {
    expect(filterRecipientAddresses(rows, '春风')).toEqual([rows[0]])
    expect(filterRecipientAddresses(rows, '1390013')).toEqual([rows[1]])
    expect(filterRecipientAddresses(rows, '会展路')).toEqual([rows[1]])
  })

  it('formats compact recipient and tier summaries', () => {
    expect(recipientAddressSummary(rows[0])).toContain('张三 13800138000')
    expect(recipientAddressSummary(rows[0])).toContain('云南省普洱市思茅区咖啡路 88 号')
    expect(tierQuantityLabel({ min_qty: 2, max_qty: 7, sales_unit: '袋' })).toBe('2-7 袋')
    expect(tierQuantityLabel({ min_qty: 8, sales_unit: '袋' })).toBe('8+ 袋')
  })
})
