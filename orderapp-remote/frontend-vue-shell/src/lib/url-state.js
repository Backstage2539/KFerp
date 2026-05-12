export function relativeURLForHistory(url) {
  const parsed = url instanceof URL ? url : new URL(String(url), window.location.href)
  return `${parsed.pathname}${parsed.search}${parsed.hash}`
}

const viewScopedParams = [
  'edit_id',
  'mode',
  'q',
  'from',
  'to',
  'void',
  'page',
  'limit',
  'pay_status_id',
  'ship_status_id',
  'process_status_id',
  'customer_id',
  'product_id',
  'status',
  'plan',
  'selected',
  'per_page',
  'warehouse',
  'item_type',
  'batch',
  'ship_ready',
]

export function viewNavigationURL(currentURL, key, params = {}) {
  const url = currentURL instanceof URL ? new URL(currentURL.toString()) : new URL(String(currentURL), window.location.href)
  url.searchParams.set('view', key)
  for (const name of viewScopedParams) {
    url.searchParams.delete(name)
  }
  Object.entries(params || {}).forEach(([name, value]) => {
    if (value !== undefined && value !== null && String(value) !== '') {
      url.searchParams.set(name, String(value))
    }
  })
  return url
}

export function replaceHistoryURL(url) {
  window.history.replaceState({}, '', relativeURLForHistory(url))
}
