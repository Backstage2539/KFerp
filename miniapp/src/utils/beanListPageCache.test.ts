import { describe, expect, it } from 'vitest'
import type { BeanListSummary } from '../api/customerPortal'
import {
  beanListPageCacheChanged,
  beanListPageCacheStorageKey,
  nextBeanListPageCacheRecord,
  type BeanListPageCacheRecord,
} from './beanListPageCache'

describe('bean list native page cache helpers', () => {
  const beanList: BeanListSummary = {
    id: 1,
    list_type: 'commercial',
    version_no: 'V3.0.5',
    status: 'published',
    published_at: '2026-05-04 10:00',
    changelog: '新版报价',
    cache_key: 'bean-list:1:V3.0.5',
    title: '棵凡咖啡批发豆单',
    layout_style: 'card',
    cards_per_row: 2,
    groups: [{ category: '原产地精选豆', show_category: true, items: [{ code: '5.2', name: '乌拉嘎' }] }],
  }

  it('scopes the native page cache by customer and list type', () => {
    expect(beanListPageCacheStorageKey(147, beanList, 'development')).toBe('kferp:development:bean-list-page:147:commercial')
    expect(beanListPageCacheStorageKey(147, beanList, 'production')).toBe('kferp:production:bean-list-page:147:commercial')
  })

  it('keeps cached native content until the server cache key changes', () => {
    const cached: BeanListPageCacheRecord = {
      publication_id: 1,
      list_type: 'commercial',
      version_no: 'V3.0.5',
      cache_key: 'bean-list:1:V3.0.5',
      cached_at: 1760000000000,
      page: beanList,
    }
    expect(beanListPageCacheChanged(cached, beanList)).toBe(false)
    expect(beanListPageCacheChanged(cached, { ...beanList, id: 2, version_no: 'V3.0.6', cache_key: 'bean-list:2:V3.0.6' })).toBe(true)
  })

  it('stores the full native page snapshot for instant rendering', () => {
    expect(nextBeanListPageCacheRecord(beanList, 1760000000000)).toEqual({
      publication_id: 1,
      list_type: 'commercial',
      version_no: 'V3.0.5',
      cache_key: 'bean-list:1:V3.0.5',
      cached_at: 1760000000000,
      page: beanList,
    })
  })
})
