import { miniRequest } from './client'
import type { BeanListSummary } from './customerPortal'
export type PageBlock = { kind: 'heading'|'text'|'image'; text?: string; asset_id?: string; caption?: string }
export type PageDocument = { name: string; kind: 'price'|'article'|'function'; visibility: string; blocks?: PageBlock[] }
export function fetchPageEntry(key: string, token = '') {
 return miniRequest<{ document: PageDocument; publication?: BeanListSummary; target_path?: string }>(`/api/pages/${encodeURIComponent(key)}`,{token})
}
