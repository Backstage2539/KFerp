import { describe, expect, it } from 'vitest'
import { customerMainTabs } from './beanCenter'
import { visibleHomeEntries } from './capabilities'
import { customerDestinationURL } from './customerNavigation'

describe('customer navigation destinations and access', () => {
  it('keeps inventory reachable with either of the API-supported capabilities', () => {
    for (const code of ['inventory_custody', 'processing']) {
      expect(visibleHomeEntries([{ code, enabled: true }]).filter(entry => entry.key === 'inventory')).toHaveLength(1)
      expect(visibleHomeEntries([{ code, enabled: false }]).some(entry => entry.key === 'inventory')).toBe(false)
    }
    expect(visibleHomeEntries([{ code: 'direct_ship', enabled: true }]).some(entry => entry.key === 'inventory')).toBe(false)
  })

  it('exposes fees only with settlement permission and never exposes customer tabs to employees', () => {
    expect(customerMainTabs(true, []).map(tab => tab.key)).toEqual(['home', 'beans', 'orders', 'mine'])
    expect(customerMainTabs(true, [{ code: 'settlement', enabled: true }]).map(tab => tab.key)).toEqual(['home', 'beans', 'orders', 'billing', 'mine'])
    expect(customerMainTabs(true, [{ code: 'settlement', enabled: false }]).some(tab => tab.key === 'billing')).toBe(false)
    expect(customerMainTabs(true, [{ code: 'settlement', enabled: true }], 'employee')).toEqual([])
    expect(customerMainTabs(false, [{ code: 'settlement', enabled: true }]).map(tab => tab.key)).toEqual(['home', 'beans', 'mine'])
  })

  it('keeps official orders, processing shipments, dropship modes and retained purchase contexts distinct', () => {
    expect(customerDestinationURL('orders', { q: 'SO & 订单' })).toBe('/pages/service/service?key=orders&source=official&q=SO%20%26%20%E8%AE%A2%E5%8D%95')
    expect(customerDestinationURL('processingShipments')).toBe('/pages/service/service?key=orders')
    expect(customerDestinationURL('directShip', { fulfillment_mode: 'stock' })).toBe('/pages/service/service?key=directShip&fulfillment_mode=stock')
    expect(customerDestinationURL('directShip', { fulfillment_mode: 'on_demand' })).toBe('/pages/service/service?key=directShip&fulfillment_mode=on_demand')
    expect(customerDestinationURL('productOrder', { stock_purchase: true })).toBe('/pages/service/service?key=productOrder&stock_purchase=1')
  })
})
