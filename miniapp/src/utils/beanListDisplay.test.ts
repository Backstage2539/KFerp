import { describe, expect, it } from 'vitest'
import type { BeanListProductSummary, BeanListSummary } from '../api/customerPortal'
import { beanListCardRows, beanListDisplayStyle, beanListQualityLines, splitBeanListHighlight } from './beanListDisplay'

describe('bean list native display helpers', () => {
  it('builds the miniapp surface style from the published ERP config', () => {
    const item: BeanListSummary = {
      id: 1,
      list_type: 'commercial',
      version_no: 'V3.0.5',
      status: 'published',
      published_at: '',
      changelog: '',
      cache_key: 'bean-list:1:V3.0.5',
      background_color: '#f8f1e5',
      font_color: '#171717',
      background_image: '/uploads/bean-bg.png',
    }

    expect(beanListDisplayStyle(item)).toEqual({
      backgroundColor: '#f8f1e5',
      color: '#171717',
      backgroundImage: 'url("/uploads/bean-bg.png")',
    })
  })

  it('chunks products by the published cards-per-row setting', () => {
    const items: BeanListProductSummary[] = [
      { code: '1.1', name: 'A' },
      { code: '1.2', name: 'B' },
      { code: '1.3', name: 'C' },
    ]

    expect(beanListCardRows(items, 2)).toEqual([[items[0], items[1]], [items[2]]])
    expect(beanListCardRows(items, 0)).toEqual([[items[0]], [items[1]], [items[2]]])
  })

  it('splits highlighted terms for native red text rendering', () => {
    expect(splitBeanListHighlight('乌拉嘎 柑橘莓果', ['乌拉嘎', '柑橘'])).toEqual([
      { text: '乌拉嘎', red: true },
      { text: ' ', red: false },
      { text: '柑橘', red: true },
      { text: '莓果', red: false },
    ])
  })

  it('builds quality rows from the latest passed bean-list inspection', () => {
    const item: BeanListProductSummary = {
      code: 'G.1',
      name: '埃塞瑰夏生豆',
      bean_list_quality: {
        factory_flavor_description: '茉莉花、柑橘',
        moisture: '10.8%',
        density: '780g/L',
        inspection_created_at: '2026-05-18 09:30',
      },
    }

    expect(beanListQualityLines(item)).toEqual([
      { label: '工厂风味', value: '茉莉花、柑橘' },
      { label: '水分', value: '10.8%' },
      { label: '密度', value: '780g/L' },
      { label: '质检时间', value: '2026-05-18 09:30' },
    ])
  })
})
