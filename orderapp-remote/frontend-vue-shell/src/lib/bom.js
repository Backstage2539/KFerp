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

export function filterProductionBomCatalog(rows = [], { status = 'active', query = '', groupID = 0 } = {}) {
  const statusMode = String(status || 'active').trim().toLowerCase()
  const keyword = String(query || '').trim().toLowerCase()
  const selectedGroupID = Number(groupID || 0)
  return rows.filter((row) => {
    const rowStatus = String(row.status || 'active').trim().toLowerCase()
    if (statusMode === 'active' && rowStatus === 'inactive') return false
    if (statusMode === 'inactive' && rowStatus !== 'inactive') return false
    const rowGroupID = Number(row.group_id || row.production_bom_group_id || 0)
    if (selectedGroupID > 0 && rowGroupID !== selectedGroupID) return false
    if (selectedGroupID === -1 && rowGroupID > 0) return false
    if (!keyword) return true
    const haystack = [
      row.code,
      row.name,
      row.group_name,
      row.latest_version_no,
      row.status,
    ].map((value) => String(value || '').toLowerCase()).join(' ')
    return haystack.includes(keyword)
  })
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

export function productionBomLabel(row = {}) {
  const code = String(row.production_bom_code || row.productionBomCode || '').trim()
  const name = String(row.production_bom_name || row.productionBomName || '').trim()
  const version = String(row.production_bom_version_no || row.productionBomVersionNo || '').trim()
  const status = String(row.bom_status || row.status || '').trim()
  const title = [code, name].filter(Boolean).join(' ')
  if (title && version) return `${title} / ${version}`
  if (title) return `${title} / 未绑定版本`
  if (version) return `生产 BOM / ${version}`
  if (status === 'missing') return '无生产 BOM'
  return '无生产 BOM'
}

export function productionBomVersionWarning(row = {}) {
  const current = String(row.production_bom_version_no || row.productionBomVersionNo || '').trim()
  const latest = String(row.latest_bom_version_no || row.latestBomVersionNo || '').trim()
  const rawLatest = row.is_latest_bom_version ?? row.isLatestBomVersion
  const isLatest = rawLatest === true || rawLatest === 'true' || rawLatest === 1
  if (!current || !latest || isLatest || current === latest) return ''
  return `当前引用 ${current}，最新 ${latest}`
}

export function bomSourceLabel(row = {}) {
  return productionBomLabel(row)
}
