import { describe, expect, it } from 'vitest'
import { buildServicePagePath } from './customerPortal'

describe('customer portal API helpers', () => {
  it('encodes service page filters into the mini service path', () => {
    expect(
      buildServicePagePath('orders', {
        q: '乌拉嘎 上海',
        date_from: '2026-05-01',
        date_to: '2026-05-03',
      }),
    ).toBe('/api/mini/services/orders?q=%E4%B9%8C%E6%8B%89%E5%98%8E%20%E4%B8%8A%E6%B5%B7&date_from=2026-05-01&date_to=2026-05-03')
  })

  it('does not add a query string when no filters are set', () => {
    expect(buildServicePagePath('beanList', {})).toBe('/api/mini/services/beanList')
  })
})
