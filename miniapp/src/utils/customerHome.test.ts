import { expect, it } from 'vitest'
import { customerOrderNeedsHandling } from './customerHome'

it('finishes retained purchases when received without requiring a shipment', () => {
  expect(customerOrderNeedsHandling({ delivery_mode: 'customer_stock', delivery_status: 'received', ship_status: '待发货' })).toBe(false)
  expect(customerOrderNeedsHandling({ delivery_mode: 'customer_stock', delivery_status: 'partially_received', ship_status: '待发货' })).toBe(true)
  expect(customerOrderNeedsHandling({ delivery_mode: 'customer_stock', delivery_status: 'pending_receipt', ship_status: '待发货' })).toBe(true)
})

it('keeps normal unshipped orders pending and finishes shipped orders', () => {
  expect(customerOrderNeedsHandling({ ship_status: '待发货' })).toBe(true)
  expect(customerOrderNeedsHandling({ ship_status: '已发货' })).toBe(false)
  expect(customerOrderNeedsHandling({ ship_status: '已签收' })).toBe(false)
})
