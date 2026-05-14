export const validOrderListScopes = ['all', 'mine', 'fulfillment']

export function orderListScopeForRequest(scope) {
  const normalized = String(scope || '').trim()
  if (!normalized) return 'all'
  if (validOrderListScopes.includes(normalized)) return normalized
  return normalized
}
