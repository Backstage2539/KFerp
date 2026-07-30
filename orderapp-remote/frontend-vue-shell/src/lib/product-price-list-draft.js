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

function pricingSelection(value = {}) {
  return {
    pricing_mode: String(value.pricing_mode ?? value.pricingMode ?? '').trim(),
    tier_template_id: Number(value.tier_template_id ?? value.tierTemplateID ?? 0) || 0,
    pricing_rule_id: Number(value.pricing_rule_id ?? value.pricingRuleID ?? 0) || 0,
  }
}

function sharedPricingSignature(value = {}) {
  const selection = pricingSelection(value)
  return JSON.stringify(selection)
}

function productOverrideScope(key, value = {}) {
  const explicit = String(value.scope ?? value.override_scope ?? value.overrideScope ?? '').trim().toLowerCase()
  if (explicit === 'parent_product' || explicit === 'parent-product') return 'parent_product'
  if (explicit === 'sku' || explicit === 'product_sku') return 'sku'
  if (String(key || '').startsWith('parent:')) return 'parent_product'
  if (String(key || '').startsWith('sku:')) return 'sku'
  const parentID = Number(value.parent_product_id ?? value.parentProductID ?? 0) || 0
  const skuID = Number(value.sku_id ?? value.skuID ?? 0) || 0
  return parentID > 0 && (skuID > 0 || Number(value.product_id ?? value.productID ?? 0) !== parentID)
    ? 'sku'
    : 'parent_product'
}

function overrideParentProductID(scope, value = {}, parentBySku = new Map()) {
  const parentID = Number(value.parent_product_id ?? value.parentProductID ?? 0) || 0
  if (parentID > 0) return parentID
  if (scope === 'parent_product') return Number(value.product_id ?? value.productID ?? value.productId ?? 0) || 0
  const skuID = Number(value.sku_id ?? value.skuID ?? value.product_id ?? value.productID ?? value.productId ?? 0) || 0
  return Number(parentBySku.get(skuID) || 0) || 0
}

function normalizedParentOverride(parentID, value = {}) {
  const selection = pricingSelection(value)
  return {
    scope: 'parent_product',
    product_id: parentID,
    sku_id: 0,
    parent_product_id: parentID,
    product_key: `parent:${parentID}`,
    ...selection,
    fixed_unit_price: 0,
    ...(String(value.product_name ?? value.productName ?? '').trim() ? { product_name: String(value.product_name ?? value.productName).trim() } : {}),
    ...(String(value.parent_product_name ?? value.parentProductName ?? '').trim() ? { parent_product_name: String(value.parent_product_name ?? value.parentProductName).trim() } : {}),
  }
}

function normalizedSKUFixedPriceOverride(value = {}, parentID = 0) {
  const skuID = Number(value.sku_id ?? value.skuID ?? value.product_id ?? value.productID ?? value.productId ?? 0) || 0
  const fixedUnitPrice = Number(value.fixed_unit_price ?? value.fixedUnitPrice ?? 0) || 0
  if (!(skuID > 0) || !(fixedUnitPrice > 0)) return null
  return {
    scope: 'sku',
    product_id: skuID,
    sku_id: skuID,
    parent_product_id: parentID,
    product_key: `sku:${skuID}`,
    pricing_mode: '',
    tier_template_id: 0,
    pricing_rule_id: 0,
    fixed_unit_price: fixedUnitPrice,
    ...(String(value.product_name ?? value.productName ?? '').trim() ? { product_name: String(value.product_name ?? value.productName).trim() } : {}),
    ...(String(value.parent_product_name ?? value.parentProductName ?? '').trim() ? { parent_product_name: String(value.parent_product_name ?? value.parentProductName).trim() } : {}),
    ...(String(value.sku_name ?? value.skuName ?? '').trim() ? { sku_name: String(value.sku_name ?? value.skuName).trim() } : {}),
  }
}

export function normalizeParentSharedPriceListProductOverrides(rows = {}, options = {}) {
  const entries = Array.isArray(rows)
    ? rows.map((value, index) => [String(index), value])
    : Object.entries(rows || {})
  const productSpecSelections = options.productSpecSelections ?? options.product_spec_selections ?? []
  const parentBySku = new Map((Array.isArray(productSpecSelections) ? productSpecSelections : []).map((row) => [
    Number(row?.sku_id ?? row?.skuID ?? 0) || 0,
    Number(row?.parent_product_id ?? row?.parentProductID ?? 0) || 0,
  ]).filter(([skuID, parentID]) => skuID > 0 && parentID > 0))
  const byParent = new Map()
  for (const [key, rawValue] of entries) {
    const value = rawValue && typeof rawValue === 'object' ? rawValue : {}
    const scope = productOverrideScope(key, value)
    const parentID = overrideParentProductID(scope, value, parentBySku)
    if (!(parentID > 0)) continue
    if (!byParent.has(parentID)) byParent.set(parentID, { parent: null, skus: [] })
    if (scope === 'parent_product') byParent.get(parentID).parent = value
    else byParent.get(parentID).skus.push(value)
  }

  const overrides = {}
  const conflicts = []
  for (const [parentID, group] of byParent.entries()) {
    const fixedRows = group.skus.map((row) => normalizedSKUFixedPriceOverride(row, parentID)).filter(Boolean)
    fixedRows.forEach((row) => { overrides[`sku:${row.sku_id}`] = row })

    if (group.parent) {
      overrides[`parent:${parentID}`] = normalizedParentOverride(parentID, group.parent)
      continue
    }

    const configuredSKUs = group.skus.filter((row) => {
      const selection = pricingSelection(row)
      return Boolean(selection.pricing_mode || selection.tier_template_id > 0 || selection.pricing_rule_id > 0)
    })
    if (!configuredSKUs.length) continue
    const signatures = new Set(configuredSKUs.map(sharedPricingSignature))
    if (signatures.size === 1) {
      overrides[`parent:${parentID}`] = normalizedParentOverride(parentID, configuredSKUs[0])
      continue
    }
    conflicts.push({
      parent_product_id: parentID,
      message: `旧草稿中同一商品的规格计价配置不一致，请重新选择商品计价后继续。`,
    })
  }
  return { overrides, conflicts }
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
