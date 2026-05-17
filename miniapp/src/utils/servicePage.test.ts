import { describe, expect, it } from 'vitest'
import { serviceCapability, serviceTitle, visibleServiceSections } from './servicePage'

describe('service page helpers', () => {
  it('maps mini service keys to capability codes and titles', () => {
    expect(serviceCapability('directShip')).toBe('direct_ship')
    expect(serviceCapability('orders')).toBe('product_order')
    expect(serviceCapability('processing')).toBe('processing')
    expect(serviceTitle('orders')).toBe('我的订单')
    expect(serviceTitle('settlement')).toBe('结算中心')
  })

  it('maps legacy logistics service links to the order service', () => {
    expect(serviceCapability('shipping')).toBe('product_order')
    expect(serviceTitle('shipping')).toBe('我的订单')
  })

  it('returns visible data sections for a service payload', () => {
    const sections = visibleServiceSections({
      key: 'orders',
      title: '我的订单',
      orders: [{ order_no: 'SO-1', ship_tracking_no: 'SF123' }],
      fee_items: [{ fee_type: 'shipping', amount: '12.00', status: 'unsettled' }],
    })

    expect(sections.map((section) => section.title)).toEqual(['我的订单', '费用明细'])
  })

  it('labels settlement orders as bill rows', () => {
    const sections = visibleServiceSections({
      key: 'settlement',
      title: '结算中心',
      orders: [{ order_no: 'SO-YAN-BILL', grand_total: '4559.00' }],
    })

    expect(sections.map((section) => section.title)).toEqual(['订单账单'])
  })
})
