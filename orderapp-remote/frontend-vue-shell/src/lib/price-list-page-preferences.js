export const PRICE_LIST_PAGE_PREFERENCES_KEY = 'kferp:product-price-list:page-preferences:v1'

const DEFAULT_PREFERENCES = Object.freeze({
  scope: 'official',
  productTypeCategoryID: 0,
})

function normalizedScope(value) {
  const scope = String(value || '').trim()
  if (scope === 'official') return scope
  if (/^customer:[1-9]\d*$/.test(scope)) return scope
  return 'official'
}

function normalizedProductTypeCategoryID(value) {
  const id = Number(value || 0)
  return Number.isInteger(id) && id > 0 ? id : 0
}

function defaultStorage() {
  try {
    return globalThis.localStorage || null
  } catch {
    return null
  }
}

export function readPriceListPagePreferences(storage = defaultStorage()) {
  if (!storage?.getItem) return { ...DEFAULT_PREFERENCES }
  try {
    const stored = JSON.parse(storage.getItem(PRICE_LIST_PAGE_PREFERENCES_KEY) || '{}')
    return {
      scope: normalizedScope(stored?.scope),
      productTypeCategoryID: normalizedProductTypeCategoryID(stored?.productTypeCategoryID),
    }
  } catch {
    return { ...DEFAULT_PREFERENCES }
  }
}

export function writePriceListPagePreferences(preferences, storage = defaultStorage()) {
  if (!storage?.setItem) return
  const current = readPriceListPagePreferences(storage)
  const next = {
    scope: normalizedScope(preferences?.scope ?? current.scope),
    productTypeCategoryID: normalizedProductTypeCategoryID(preferences?.productTypeCategoryID ?? current.productTypeCategoryID),
  }
  try {
    storage.setItem(PRICE_LIST_PAGE_PREFERENCES_KEY, JSON.stringify(next))
  } catch {
    // Browser privacy/quota restrictions must not block the price-list page.
  }
}

export function resolvePriceListScopePreference(scope, customers = []) {
  const normalized = normalizedScope(scope)
  if (normalized === 'official') return normalized
  const customerID = Number(normalized.split(':')[1] || 0)
  return customers.some((customer) => Number(customer?.id || 0) === customerID)
    ? normalized
    : 'official'
}

export function resolveProductTypePreference(selectedID, options = []) {
  const normalized = normalizedProductTypeCategoryID(selectedID)
  if (!options.length) return normalized
  if (options.some((option) => Number(option?.id || 0) === normalized)) return normalized
  return Number(options[0]?.id || 0)
}
