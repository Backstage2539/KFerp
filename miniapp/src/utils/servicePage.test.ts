import { describe, expect, it } from 'vitest'
import { serviceCapability, serviceTitle, visibleServiceSections } from './servicePage'

describe('service page helpers', () => {
  it('maps mini service keys to capability codes and titles', () => {
    expect(serviceCapability('directShip')).toBe('direct_ship')
    expect(serviceCapability('processing')).toBe('processing')
    expect(serviceTitle('settlement')).toBe('结算中心')
  })

  it('returns visible data sections for a service payload', () => {
    const sections = visibleServiceSections({
      key: 'shipping',
      title: '物流查询',
      orders: [{ order_no: 'SO-1', ship_tracking_no: 'SF123' }],
      fee_items: [{ fee_type: 'shipping', amount: '12.00', status: 'unsettled' }],
    })

    expect(sections.map((section) => section.title)).toEqual(['订单 / 物流', '费用明细'])
  })
})
