import type { BeanListSummary } from '../api/customerPortal'
import { configuredMiniappEnvironment, type MiniappBuildEnvironment } from '../config/environment'

export type BeanListPageCacheRecord = {
  schema_version?: number
  publication_id: number
  list_type: string
  version_no: string
  cache_key: string
  cached_at: number
  page: BeanListSummary
}

export type BeanListPageCacheable = Pick<BeanListSummary, 'id' | 'list_type' | 'version_no' | 'cache_key'>

export function beanListPageCacheStorageKey(
  customerID: number,
  item: BeanListPageCacheable,
  environment: MiniappBuildEnvironment = configuredMiniappEnvironment().environment,
): string {
  return `kferp:${environment}:bean-list-page:${customerID || 0}:${item.list_type || 'default'}`
}

export function beanListPageCacheChanged(cached: BeanListPageCacheRecord | null | undefined, item: BeanListPageCacheable): boolean {
  if (!cached) return false
  if (cached.schema_version !== 2) return true
  return cached.cache_key !== beanListPageCacheKey(item)
}

export function nextBeanListPageCacheRecord(item: BeanListSummary, now = Date.now()): BeanListPageCacheRecord {
  return {
    schema_version: 2,
    publication_id: item.id,
    list_type: item.list_type || 'default',
    version_no: item.version_no || '',
    cache_key: beanListPageCacheKey(item),
    cached_at: now,
    page: item,
  }
}

function beanListPageCacheKey(item: BeanListPageCacheable): string {
  return item.cache_key || `bean-list:${item.id}:${item.version_no || 'published'}`
}
