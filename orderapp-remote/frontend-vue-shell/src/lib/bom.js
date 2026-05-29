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
