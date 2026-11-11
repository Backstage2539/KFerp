import { describe, it, expect } from 'vitest'
import { loginRouteFor, safeLoginReturn } from './loginReturn'
describe('login return', () => {
  it('opens account login explicitly without losing a safe return destination', () => {
    expect(loginRouteFor('', 'password')).toBe('/pages/login/login?mode=password')
    const route = new URL(loginRouteFor('/pages/profile/profile', 'password'), 'https://test.invalid')
    expect(route.searchParams.get('mode')).toBe('password')
    expect(route.searchParams.get('return_to')).toBe('/pages/profile/profile')
    expect(loginRouteFor('https://evil.test', 'password')).toBe('/pages/login/login?mode=password')
  })
  it('preserves unified entries and rejects injected customers or return addresses', () => {
    const path='/pages/page-entry/page-entry?entry=0123456789abcdef0123456789abcdef'
    expect(safeLoginReturn(path)).toBe(path)
    expect(safeLoginReturn(path+'&customer_id=9')).toBe('')
    expect(safeLoginReturn(path+'&return_to=https://evil.test')).toBe('')
  })
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
it('retains a specific notice order and rejects malformed or appended targets',()=>{
 const p='/pages/service/service?key=orders&source=notice&order_id=42&customer_id=8'
 expect(safeLoginReturn(p)).toBe(p)
 for(const bad of [p+'&next=https://evil.test',p.replace('42','0'),p.replace('42','-1'),p.replace('8','8.5')]) expect(safeLoginReturn(bad)).toBe('')
 expect(safeLoginReturn('/pages/order-notifications/order-notifications')).toBe('/pages/order-notifications/order-notifications')
})
it('keeps the employee notice order but rejects arbitrary employee navigation',()=>{
 const p='/pages/employee-order-detail/employee-order-detail?id=42'
 expect(safeLoginReturn(p)).toBe(p)
 expect(safeLoginReturn(p+'&customer_id=8')).toBe('')
 expect(safeLoginReturn(p.replace('42','-1'))).toBe('')
})
it('permits only a numeric receipt or bill notification destination',()=>{
 for(const type of ['receipt','bill']) {
  const path=`/pages/notification-detail/notification-detail?type=${type}&id=42`
  expect(safeLoginReturn(path)).toBe(path)
  for(const bad of [path+'&customer_id=9',path.replace('42','0'),path.replace('42','-1'),path.replace('42','42.1'),path.replace(type,'orders')]) expect(safeLoginReturn(bad)).toBe('')
 }
})
