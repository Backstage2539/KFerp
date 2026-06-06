import { describe, expect, it } from 'vitest'
import type { CustomerPriceTableGroup } from '../api/customerPortal'
import { collapsedPriceTableSummaries, priceTableGroupLabel } from './customerProducts'

describe('customer products helpers', () => {
  it('formats price table groups by list type label and counts', () => {
    const group: CustomerPriceTableGroup = {
      list_type: 'green',
      list_type_label: '生豆',
      product_count: 6,
      price_table_count: 1,
      latest_version: { id: 11, list_type: 'green', version_no: 'G-1', status: 'published', published_at: '', changelog: '', cache_key: '' },
    }

    expect(priceTableGroupLabel(group)).toBe('生豆 · 6 个商品 · 1 个价格表')
  })

  it('collapses customer published versions by default', () => {
    const groups: CustomerPriceTableGroup[] = [{
      list_type: 'green',
      list_type_label: '生豆',
      product_count: 2,
      price_table_count: 2,
      latest_version: { id: 21, list_type: 'green', version_no: 'V2', status: 'published', published_at: '', changelog: '', cache_key: '' },
      versions: [
        { id: 21, list_type: 'green', version_no: 'V2', status: 'published', published_at: '', changelog: '', cache_key: '' },
        { id: 20, list_type: 'green', version_no: 'V1', status: 'published', published_at: '', changelog: '', cache_key: '' },
      ],
    }]

    expect(collapsedPriceTableSummaries(groups)).toEqual([{
      list_type: 'green',
      title: '生豆',
      latest_version_no: 'V2',
      total_versions: 2,
      expanded: false,
    }])
  })
})
