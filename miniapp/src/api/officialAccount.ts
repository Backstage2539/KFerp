import { miniRequest } from './client'
import type { BeanListSummary } from './customerPortal'
export type OfficialBinding = {
  openid: string
  mini_user_id: number
  customer_id: number
  customer_name: string
  active: boolean
}
export function fetchFixedPriceEntry(key: string, token = '') {
  return miniRequest<{
    entry: { key: string; name: string; visibility: string }
    publication: BeanListSummary
  }>(`/api/mini/price-table-entries/${encodeURIComponent(key)}`, { token })
}
export function fetchOfficialBindings(token: string) {
  return miniRequest<{ rows: OfficialBinding[]; enabled: boolean }>(
    '/api/mini/official-account/binding',
    { token },
  )
}
export function createOfficialBindingCode(token: string) {
  return miniRequest<{
    code: string
    expires_in: number
    customer_name: string
  }>('/api/mini/official-account/binding-codes', { token, method: 'POST' })
}
export function updateOfficialBinding(
  token: string,
  openid: string,
  customerID: number,
  remove = false,
) {
  return miniRequest('/api/mini/official-account/binding', {
    token,
    method: remove ? 'DELETE' : 'PUT',
    data: { openid, customer_id: customerID },
  })
}
