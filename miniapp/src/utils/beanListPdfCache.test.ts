import { describe, expect, it } from 'vitest'
import {
  beanListCacheStorageKey,
  beanListVersionChanged,
  nextBeanListCacheRecord,
  type BeanListPDFCacheRecord,
} from './beanListPdfCache'

describe('bean list PDF cache helpers', () => {
  const beanList = {
    id: 1,
    list_type: 'commercial',
    version_no: 'V3.0.5',
    cache_key: 'bean-list:1:V3.0.5',
    pdf_url: '/api/mini/bean-lists/1.pdf',
  }

  it('scopes local PDF cache by customer and list type', () => {
    expect(beanListCacheStorageKey(147, beanList)).toBe('kferp:bean-list-pdf:147:commercial')
  })

  it('prompts for update only when the server cache key changes', () => {
    const cached: BeanListPDFCacheRecord = {
      publication_id: 1,
      version_no: 'V3.0.5',
      cache_key: 'bean-list:1:V3.0.5',
      saved_file_path: 'wxfile://old.pdf',
    }
    expect(beanListVersionChanged(cached, beanList)).toBe(false)
    expect(beanListVersionChanged(cached, { ...beanList, id: 2, version_no: 'V3.0.6', cache_key: 'bean-list:2:V3.0.6' })).toBe(true)
  })

  it('stores the saved file path with the current publication version', () => {
    expect(nextBeanListCacheRecord(beanList, 'wxfile://new.pdf')).toEqual({
      publication_id: 1,
      version_no: 'V3.0.5',
      cache_key: 'bean-list:1:V3.0.5',
      saved_file_path: 'wxfile://new.pdf',
    })
  })
})
