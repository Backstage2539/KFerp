const PRICE_LIST_GENERATION_DRAFT_PREFIX = 'kferp:price-list-generation'

function cloneDraft(value) {
  if (value === null || typeof value === 'undefined') return null
  return JSON.parse(JSON.stringify(value))
}

function storageOrDefault(storage) {
  if (storage) return storage
  if (typeof window !== 'undefined' && window.localStorage) return window.localStorage
  return null
}

function stringPart(value) {
  return String(value ?? '').trim() || 'default'
}

export function priceListGenerationDraftKey({
  workspace = 'factory',
  scope = 'official',
  customerID = 0,
  typeKey = '',
} = {}) {
  const customer = Number(customerID || 0) || 0
  return [
    PRICE_LIST_GENERATION_DRAFT_PREFIX,
    stringPart(workspace),
    stringPart(scope),
    customer || 'all',
    stringPart(typeKey),
  ].join(':')
}

export function normalizePriceListGenerationDraft(draft = {}) {
  const normalized = {
    defaults: cloneDraft(draft.defaults || {}),
    parentSelections: cloneDraft(draft.parentSelections || {}),
    groupSelections: cloneDraft(draft.groupSelections || {}),
    productOverrides: cloneDraft(draft.productOverrides || {}),
    flatRowOverrides: cloneDraft(draft.flatRowOverrides || {}),
  }
  if (Object.prototype.hasOwnProperty.call(draft, 'product_spec_selections') || Object.prototype.hasOwnProperty.call(draft, 'productSpecSelections')) {
    normalized.product_spec_selections = cloneDraft(draft.product_spec_selections ?? draft.productSpecSelections ?? []) || []
  }
  return normalized
}

export function savePriceListGenerationDraft(key, draft = {}, storage = null) {
  const target = storageOrDefault(storage)
  if (!target || !key) return
  target.setItem(String(key), JSON.stringify(normalizePriceListGenerationDraft(draft)))
}

export function readPriceListGenerationDraft(key, storage = null) {
  const target = storageOrDefault(storage)
  if (!target || !key) return null
  const raw = target.getItem(String(key))
  if (!raw) return null
  try {
    return normalizePriceListGenerationDraft(JSON.parse(raw))
  } catch {
    return null
  }
}
