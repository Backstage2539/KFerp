import { slicePageRows } from './pagination.js'

export const PRODUCT_KIND_ALL = 'all'

export const greenBeanTypeOptions = [
  { value: 'single_origin', label: '单品' },
  { value: 'blend', label: '拼配' },
]

export function normalizedProductKind(row = {}) {
  return row?.product_kind === 'green_bean' ? 'green_bean' : 'roasted'
}

export function normalizedGreenBeanType(value) {
  return String(value || '').trim() === 'blend' ? 'blend' : 'single_origin'
}

export function greenBeanTypeLabel(value) {
  const normalized = normalizedGreenBeanType(value)
  return greenBeanTypeOptions.find((item) => item.value === normalized)?.label || '单品'
}

export function filterSkuRows(rows = [], filters = {}) {
  const productKind = String(filters.productKind || PRODUCT_KIND_ALL).trim()
  const query = String(filters.query || '').trim().toLowerCase()
  const primaryCategory = String(filters.primaryCategory || '').trim()
  const secondaryCategory = String(filters.secondaryCategory || '').trim()
  return (rows || []).filter((row) => {
    if (productKind && productKind !== PRODUCT_KIND_ALL && normalizedProductKind(row) !== productKind) return false
    if (query) {
      const haystack = `${row.name || ''} ${row.number || ''}`.toLowerCase()
      if (!haystack.includes(query)) return false
    }
    if (primaryCategory && String(row.primary_name || '') !== primaryCategory) return false
    if (secondaryCategory && String(row.secondary_name || '') !== secondaryCategory) return false
    return true
  })
}

export function paginatedSkuRows(rows = [], filters = {}, pagination = {}) {
  return slicePageRows(filterSkuRows(rows, filters), pagination)
}

export function customerSkuCustomerOptions(customers = []) {
  const rows = Array.isArray(customers)
    ? customers
    : Array.isArray(customers?.customers)
      ? customers.customers
      : Array.isArray(customers?.rows)
        ? customers.rows
        : []
  return rows
    .filter((customer) => Number(customer?.id || 0) > 0 && customer?.active !== false)
    .slice()
    .sort((a, b) => String(a?.name || '').localeCompare(String(b?.name || '')))
}

export function buildCustomerPublicCopyPayload(customerID, options = {}) {
  return {
    customer_id: Number(customerID || 0),
    use_public_sku: Boolean(options.use_public_sku ?? options.usePublicSku),
    use_public_categories: Boolean(options.use_public_categories ?? options.usePublicCategories),
  }
}

export function primaryCategoryOptions(rows = []) {
  return uniqueSorted((rows || []).map((row) => row.primary_name))
}

export function secondaryCategoryOptions(rows = [], primaryCategory = '') {
  const primary = String(primaryCategory || '').trim()
  return uniqueSorted((rows || [])
    .filter((row) => !primary || String(row.primary_name || '') === primary)
    .map((row) => row.secondary_name))
}

export function roastedBomProductOptions(products = []) {
  return (products || [])
    .filter((row) => Number(row.id || 0) > 0 && String(row?.product_kind || '').trim() === 'roasted')
    .slice()
    .sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')))
}

export function buildProductCreatePayload(form = {}) {
  const kind = normalizedProductKind(form)
  const payload = {
    name: String(form.name || '').trim(),
    product_kind: kind,
  }
  if (kind === 'green_bean') {
    payload.green_bean_type = normalizedGreenBeanType(form.green_bean_type)
    payload.green_bean_bom_product_id = Number(form.green_bean_bom_product_id || 0)
    return payload
  }
  payload.roast_level = String(form.roast_level || '').trim()
  payload.yield_rate = Number((Number(form.yield_percent || 0) / 100).toFixed(4))
  return payload
}

export function buildProductBasicsPayload(row = {}, marginRateOverride = null) {
  const kind = normalizedProductKind(row)
  const payload = {
    product_kind: kind,
  }
  if (kind === 'green_bean') {
    payload.green_bean_type = normalizedGreenBeanType(row.green_bean_type)
    payload.green_bean_bom_product_id = Number(row.green_bean_bom_product_id || 0)
  } else {
    payload.roast_level = String(row.roast_level || '').trim()
    payload.yield_rate = Number((Number(row.yield_percent || 0) / 100).toFixed(4))
  }
  payload.margin_rate_override = marginRateOverride
  return payload
}

function uniqueSorted(values = []) {
  return Array.from(new Set(values.map((value) => String(value || '').trim()).filter(Boolean)))
    .sort((a, b) => a.localeCompare(b))
}
