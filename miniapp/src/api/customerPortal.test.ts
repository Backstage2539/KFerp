import { describe, expect, it } from 'vitest'
import { buildMallOrderPath, buildMallPagePath, buildServicePagePath } from './customerPortal'

describe('customer portal API helpers', () => {
  it('encodes service page filters into the mini service path', () => {
    expect(
      buildServicePagePath('orders', {
        q: '乌拉嘎 上海',
        date_from: '2026-05-01',
        date_to: '2026-05-03',
        process_status: '生产中',
        pay_status: '已收款',
        ship_status: '待发货',
      }),
    ).toBe('/api/mini/services/orders?q=%E4%B9%8C%E6%8B%89%E5%98%8E%20%E4%B8%8A%E6%B5%B7&date_from=2026-05-01&date_to=2026-05-03&process_status=%E7%94%9F%E4%BA%A7%E4%B8%AD&pay_status=%E5%B7%B2%E6%94%B6%E6%AC%BE&ship_status=%E5%BE%85%E5%8F%91%E8%B4%A7')
  })

  it('does not add a query string when no filters are set', () => {
    expect(buildServicePagePath('beanList', {})).toBe('/api/mini/services/beanList')
  })

  it('exposes stable mini mall API paths', () => {
    expect(buildMallPagePath()).toBe('/api/mini/mall')
    expect(buildMallOrderPath()).toBe('/api/mini/mall/orders')
  })
})
