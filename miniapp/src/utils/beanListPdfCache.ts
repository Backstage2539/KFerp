import type { BeanListSummary } from '../api/customerPortal'

export type BeanListPDFCacheRecord = {
  publication_id: number
  version_no: string
  cache_key: string
  saved_file_path: string
}

export type BeanListPDFCacheable = Pick<BeanListSummary, 'id' | 'list_type' | 'version_no' | 'cache_key'>

export function beanListCacheStorageKey(customerID: number, item: BeanListPDFCacheable): string {
  return `kferp:bean-list-pdf:${customerID || 0}:${item.list_type || 'default'}`
}

export function beanListVersionChanged(cached: BeanListPDFCacheRecord | null | undefined, item: BeanListPDFCacheable): boolean {
  if (!cached) return false
  return cached.cache_key !== beanListCacheKey(item)
}

export function nextBeanListCacheRecord(item: BeanListPDFCacheable, savedFilePath: string): BeanListPDFCacheRecord {
  return {
    publication_id: item.id,
    version_no: item.version_no || '',
    cache_key: beanListCacheKey(item),
    saved_file_path: savedFilePath,
  }
}

function beanListCacheKey(item: BeanListPDFCacheable): string {
  return item.cache_key || `bean-list:${item.id}:${item.version_no || 'published'}`
}
