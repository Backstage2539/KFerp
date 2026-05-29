export function normalizedBomProductKind(row = {}) {
  const kind = String(row.product_kind || row.productKind || '').trim().toLowerCase()
  return kind || 'roasted_bean'
}

export function isBomProductCandidate(row = {}) {
  return normalizedBomProductKind(row) !== 'green_bean'
}

export function bomRowCustomerID(row = {}) {
  return Number(row.customer_id ?? row.customerID ?? 0)
}

function bomOrderUsageCount(row = {}) {
  const raw = row.order_usage_count ?? row.orderUsageCount ?? row.order_count ?? row.orderCount ?? 0
  const value = Number(raw || 0)
  return Number.isFinite(value) ? value : 0
}

export function sortBomContextProducts(rows = [], customerID = 0) {
  const selectedCustomerID = Number(customerID || 0)
  return [...rows].sort((a, b) => {
    if (selectedCustomerID > 0) {
      const aOwned = bomRowCustomerID(a) === selectedCustomerID ? 0 : 1
      const bOwned = bomRowCustomerID(b) === selectedCustomerID ? 0 : 1
      if (aOwned !== bOwned) return aOwned - bOwned
    }
    const usageDiff = bomOrderUsageCount(b) - bomOrderUsageCount(a)
    if (usageDiff !== 0) return usageDiff
    return String(a.name || a.product || '').localeCompare(String(b.name || b.product || ''))
  })
}

export function filterBomContextProducts(rows = [], customerID = 0) {
  const selectedCustomerID = Number(customerID || 0)
  return sortBomContextProducts(rows.filter((row) => {
    if (!isBomProductCandidate(row)) return false
    const rowCustomerID = bomRowCustomerID(row)
    return selectedCustomerID > 0 ? rowCustomerID === 0 || rowCustomerID === selectedCustomerID : rowCustomerID === 0
  }), selectedCustomerID)
}

export function filterBomRowsByProductFocus(rows = [], productID = 0) {
  const focusProductID = Number(productID || 0)
  if (focusProductID <= 0) return rows
  return rows.filter((row) => Number(row.product_id || row.id || 0) === focusProductID)
}

export function bomContextCustomerIDs(products = [], bomRows = []) {
  const ids = new Set()
  for (const row of [...products, ...bomRows]) {
    if (!isBomProductCandidate(row)) continue
    const customerID = bomRowCustomerID(row)
    if (customerID > 0) ids.add(customerID)
  }
  return ids
}

export function bomSourceLabel(row = {}) {
  const explicit = String(row.derived_from_label || row.derivedFromLabel || '').trim()
  if (explicit) return explicit
  const sourceType = String(row.bom_source_type || row.bomSourceType || '').trim()
  const code = String(row.source_product_code || row.sourceProductCode || '').trim()
  const name = String(row.source_product_name || row.sourceProductName || '').trim()
  const version = String(row.source_bom_version_no || row.sourceBomVersionNo || '当前BOM').trim() || '当前BOM'
  const source = [code, name].filter(Boolean).join(' ')
  if (sourceType === 'inherit_current') return `继承：${source} / BOM ${version}`
  if (sourceType === 'inherit_version') return `锁定：${source} / BOM ${version}`
  if (sourceType === 'derived_owned') return `自有 BOM，派生自：${source} / BOM ${version}`
  if (sourceType === 'owned') return '自有 BOM'
  if (Number(row.base_product_id || row.baseProductID || 0) > 0 && Number(row.bom_item_count || row.item_count || 0) === 0) {
    return `继承：SKU-${Number(row.base_product_id || row.baseProductID)} / BOM 当前BOM`
  }
  if (String(row.bom_status || row.status || '') === 'missing') return '缺 BOM'
  return '自有 BOM'
}
