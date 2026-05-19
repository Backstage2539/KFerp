import { slicePageRows } from './pagination.js'

export const PRODUCT_KIND_ALL = 'all'
export const SKU_CUSTOM_TYPE_ALL = 'all'

export const skuTypeOptions = [
  { value: SKU_CUSTOM_TYPE_ALL, label: '全部类型' },
  { value: 'standard', label: '标准' },
  { value: 'public_sku_alias', label: '公共 SKU 改名' },
  { value: 'custom_roast', label: '定制烘焙' },
  { value: 'custom_blend', label: '定制拼配' },
]

export const greenBeanTypeOptions = [
  { value: 'single_origin', label: '单品' },
  { value: 'blend', label: '拼配' },
]

export function normalizedProductKind(row = {}) {
  const kind = String(row?.product_kind || '').trim()
  if (kind === 'green_bean') return 'green_bean'
  if (kind === 'drip_bag') return 'drip_bag'
  return 'roasted'
}

export function normalizedGreenBeanType(value) {
  return String(value || '').trim() === 'blend' ? 'blend' : 'single_origin'
}

export function greenBeanTypeLabel(value) {
  const normalized = normalizedGreenBeanType(value)
  return greenBeanTypeOptions.find((item) => item.value === normalized)?.label || '单品'
}

export function skuTypeValue(row = {}) {
  return String(row?.custom_type || '').trim() || 'standard'
}

export function skuTypeLabel(value) {
  const normalized = String(value || '').trim() || 'standard'
  return skuTypeOptions.find((item) => item.value === normalized)?.label || '标准'
}

export function filterSkuRows(rows = [], filters = {}) {
  const productKind = String(filters.productKind || PRODUCT_KIND_ALL).trim()
  const customType = String(filters.customType || SKU_CUSTOM_TYPE_ALL).trim()
  const query = String(filters.query || '').trim().toLowerCase()
  const primaryCategory = String(filters.primaryCategory || '').trim()
  const secondaryCategory = String(filters.secondaryCategory || '').trim()
  return (rows || []).filter((row) => {
    if (productKind && productKind !== PRODUCT_KIND_ALL && normalizedProductKind(row) !== productKind) return false
    if (customType && customType !== SKU_CUSTOM_TYPE_ALL && skuTypeValue(row) !== customType) return false
    if (query) {
      const haystack = `${row.name || ''} ${row.number || ''} ${skuTypeLabel(row.custom_type)} ${row.remark || ''}`.toLowerCase()
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

export function buildCustomerPublicUsagePayload(customerID, options = {}) {
  return {
    customer_id: Number(customerID || 0),
    use_public_sku: Boolean(options.use_public_sku ?? options.usePublicSku),
    use_public_categories: Boolean(options.use_public_categories ?? options.usePublicCategories),
  }
}

export function productBelongsToSkuContext(product = {}, context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  const productCustomerID = Number(product.customer_id || 0)
  if (!customerID) return productCustomerID === 0
  if (productCustomerID === customerID) return !isUnmodifiedPublicSkuCopy(product, context.publicProducts)
  return productCustomerID === 0 && Boolean(context.usePublicSku || context.use_public_sku)
}

export function categoryBelongsToSkuContext(category = {}, context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  const categoryCustomerID = Number(category.customer_id || 0)
  if (!customerID) return categoryCustomerID === 0
  if (categoryCustomerID === customerID) return !isDuplicatedPublicCategory(category, context.publicCategories, context.publicProducts)
  return categoryCustomerID === 0 && Boolean(context.usePublicCategories || context.use_public_categories)
}

export function isPublicReferenceRow(row = {}, context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  return customerID > 0 && Number(row.customer_id || 0) === 0
}

function isUnmodifiedPublicSkuCopy(product = {}, publicProducts = []) {
  const baseID = Number(product.base_product_id || 0)
  if (!baseID || product.custom_type !== 'public_sku_alias') return false
  const base = (publicProducts || []).find((row) => Number(row.id || 0) === baseID)
  return Boolean(base && String(base.name || '').trim().toLowerCase() === String(product.name || '').trim().toLowerCase())
}

function isDuplicatedPublicCategory(category = {}, publicCategories = [], publicProducts = []) {
  const matchesPublicCategory = (publicCategories || []).some((row) => (
    Number(row.customer_id || 0) === 0
    && Number(row.level || 0) === Number(category.level || 0)
    && String(row.name || '').trim().toLowerCase() === String(category.name || '').trim().toLowerCase()
  ))
  if (!matchesPublicCategory) return false
  if ((category.products || []).some((product) => !isUnmodifiedPublicSkuCopy(product, publicProducts))) {
    return false
  }
  if ((category.children || []).some((child) => !isDuplicatedPublicCategory(child, publicCategories, publicProducts))) {
    return false
  }
  return true
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

export function roastedBomProductOptions(products = [], { customerID = 0 } = {}) {
  const scopedCustomerID = Number(customerID || 0)
  return (products || [])
    .filter((row) => {
      if (Number(row.id || 0) <= 0 || String(row?.product_kind || '').trim() !== 'roasted') return false
      const rowCustomerID = Number(row.customer_id || 0)
      if (rowCustomerID > 0 && rowCustomerID !== scopedCustomerID) return false
      if (rowCustomerID > 0 && String(row.custom_type || '').trim() === 'public_sku_alias') return false
      return true
    })
    .slice()
    .sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')) || Number(a.customer_id || 0) - Number(b.customer_id || 0) || Number(a.id || 0) - Number(b.id || 0))
}

export function buildProductCreatePayload(form = {}) {
  const kind = normalizedProductKind(form)
  const payload = {
    name: String(form.name || '').trim(),
    product_kind: kind,
    remark: String(form.remark || '').trim(),
  }
  if (kind === 'green_bean') {
    payload.green_bean_type = normalizedGreenBeanType(form.green_bean_type)
    payload.green_bean_bom_product_id = Number(form.green_bean_bom_product_id || 0)
    return payload
  }
  if (kind === 'drip_bag') {
    payload.roast_level = String(form.roast_level || '').trim()
    payload.yield_rate = Number((Number(form.yield_percent || 0) / 100).toFixed(4))
    payload.drip_bag_grams = Number(form.drip_bag_grams || 10)
    payload.drip_box_bag_count = Number(form.drip_box_bag_count || 10)
    return payload
  }
  payload.roast_level = String(form.roast_level || '').trim()
  payload.yield_rate = Number((Number(form.yield_percent || 0) / 100).toFixed(4))
  return payload
}

export function buildCustomProductCreatePayload(customerID, form = {}) {
  const kind = normalizedProductKind(form)
  const payload = {
    customer_id: Number(customerID || form.customer_id || 0),
    base_product_id: Number(form.base_product_id || 0),
    name: String(form.name || '').trim(),
    remark: String(form.remark || '').trim(),
    product_kind: kind,
    custom_type: String(form.custom_type || '').trim(),
    copy_bom: Boolean(form.copy_bom),
    copy_price_tiers: Boolean(form.copy_price_tiers),
  }
  if (kind === 'green_bean') {
    payload.base_product_id = 0
    payload.copy_bom = false
    payload.copy_price_tiers = false
    payload.green_bean_type = normalizedGreenBeanType(form.green_bean_type)
    payload.green_bean_bom_product_id = Number(form.green_bean_bom_product_id || 0)
    return payload
  }
  payload.roast_level = String(form.roast_level || '').trim()
  if (kind === 'drip_bag') {
    payload.drip_bag_grams = Number(form.drip_bag_grams || 10)
    payload.drip_box_bag_count = Number(form.drip_box_bag_count || 10)
  }
  return payload
}

export function buildProductBasicsPayload(row = {}, marginRateOverride = null) {
  const kind = normalizedProductKind(row)
  const payload = {
    product_kind: kind,
    remark: String(row.remark || '').trim(),
  }
  if (kind === 'green_bean') {
    payload.green_bean_type = normalizedGreenBeanType(row.green_bean_type)
    payload.green_bean_bom_product_id = Number(row.green_bean_bom_product_id || 0)
  } else {
    payload.roast_level = String(row.roast_level || '').trim()
    payload.yield_rate = Number((Number(row.yield_percent || 0) / 100).toFixed(4))
    if (kind === 'drip_bag') {
      payload.drip_bag_grams = Number(row.drip_bag_grams || 10)
      payload.drip_box_bag_count = Number(row.drip_box_bag_count || 10)
    }
  }
  payload.margin_rate_override = marginRateOverride
  return payload
}

function uniqueSorted(values = []) {
  return Array.from(new Set(values.map((value) => String(value || '').trim()).filter(Boolean)))
    .sort((a, b) => a.localeCompare(b))
}
