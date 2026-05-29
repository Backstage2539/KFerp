const drafts = new Map()

export const FORM_DRAFT_SCOPES = {
  orderEntry: 'order-entry',
  bom: 'bom',
  skuSettings: 'sku-settings',
}

function cloneDraft(value) {
  if (value === null || typeof value === 'undefined') return null
  return JSON.parse(JSON.stringify(value))
}

export function saveFormDraft(key, value) {
  if (!key) return
  drafts.set(String(key), cloneDraft(value))
}

export function readFormDraft(key) {
  if (!key || !drafts.has(String(key))) return null
  return cloneDraft(drafts.get(String(key)))
}

export function hasFormDraft(key) {
  return Boolean(key && drafts.has(String(key)))
}

export function clearFormDraft(key) {
  if (!key) return
  drafts.delete(String(key))
}
