import { describe, expect, it } from 'vitest'
import { visibleHomeEntries } from './capabilities'

describe('visibleHomeEntries', () => {
  it('shows no entries without customer capabilities', () => {
    expect(visibleHomeEntries()).toEqual([])
  })

  it('shows only entries enabled by customer capabilities', () => {
    const entries = visibleHomeEntries([
      { code: 'direct_ship', enabled: true },
      { code: 'mall', enabled: true },
      { code: 'processing', enabled: false },
      { code: 'settlement', enabled: true },
    ])
    expect(entries.map((entry) => entry.key)).toEqual(['mall', 'orders', 'directShip', 'settlement'])
  })

  it('uses the dedicated mall page for customer shopping', () => {
    const entries = visibleHomeEntries([{ code: 'mall', enabled: true }])
    expect(entries).toContainEqual({
      key: 'mall',
      label: '商城下单',
      capability: 'mall',
      url: '/pages/mall/mall',
    })
  })

  it('gives every visible entry a service detail URL for tap navigation', () => {
    const entries = visibleHomeEntries([
      { code: 'bean_list', enabled: true },
      { code: 'direct_ship', enabled: true },
    ])

    expect(entries.map((entry) => entry.url)).toEqual([
      '/pages/service/service?key=beanList',
      '/pages/service/service?key=orders',
      '/pages/service/service?key=directShip',
    ])
  })

  it('shows a direct order entrance for any order-related customer capability', () => {
    for (const code of ['product_order', 'direct_ship', 'shipping_query']) {
      const entries = visibleHomeEntries([{ code, enabled: true }])
      const orders = entries.find((entry) => entry.key === 'orders')
      expect(orders?.label).toBe('我的订单')
      expect(orders?.url).toBe('/pages/service/service?key=orders')
    }
  })

  it('does not expose logistics as a standalone home entry', () => {
    const entries = visibleHomeEntries([{ code: 'shipping_query', enabled: true }])

    expect(entries.map((entry) => entry.key)).toEqual(['orders'])
  })

  it('ignores unknown capability codes', () => {
    const entries = visibleHomeEntries([{ code: 'unknown', enabled: true }])
    expect(entries).toEqual([])
  })
})
