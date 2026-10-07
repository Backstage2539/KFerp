import { describe, it, expect } from 'vitest'
import { safeLoginReturn } from './loginReturn'
describe('login return', () => {
  it('preserves a fixed price entry and customer order destination', () => {
    expect(safeLoginReturn('/pages/price-list/price-list?entry=abc')).toBe(
      '/pages/price-list/price-list?entry=abc',
    )
    expect(safeLoginReturn('/pages/service/service?key=orders')).toBe(
      '/pages/service/service?key=orders',
    )
  })
  it('returns to all orders for processing customers too', () => {
    expect(
      safeLoginReturn('/pages/service/service?key=orders&source=official'),
    ).toBe('/pages/service/service?key=orders&source=official')
  })
  it('rejects external, employee and recursive login destinations', () => {
    for (const url of [
      'https://evil.test',
      '//evil.test',
      '/pages/employee-orders/employee-orders',
      '/pages/login/login',
      '/pages/service/service?key=orders&customer_id=99',
    ])
      expect(safeLoginReturn(url)).toBe('')
  })
})
