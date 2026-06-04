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

function firstPresent(...values) {
  return values.find((value) => value !== undefined && value !== null && value !== '')
}

export function productionBomDetailAsRecipeDetail(detail = {}, fallback = {}) {
  const bomID = Number(firstPresent(detail.id, detail.production_bom_id, fallback.production_bom_id, fallback.id, 0))
  const versionID = Number(firstPresent(detail.latest_version_id, detail.latest_bom_version_id, fallback.production_bom_version_id, fallback.latest_version_id, 0))
  const versionNo = String(firstPresent(detail.latest_version_no, detail.latest_bom_version_no, fallback.production_bom_version_no, fallback.latest_version_no, fallback.latest_bom_version_no, '')).trim()
  const items = Array.isArray(detail.items) ? detail.items : []
  const totalRatio = items.reduce((sum, item) => {
    if ((item?.component_type || 'material') !== 'material') return sum
    if ((item?.consume_unit || 'ratio_pct') !== 'ratio_pct') return sum
    return sum + Number(item?.ratio_pct || 0)
  }, 0)

  return {
    product_id: 0,
    product: '未绑定商品',
    product_name: '未绑定商品',
    product_kind: 'roasted_bean',
    roast_level: '',
    status: detail.status === 'inactive' ? 'inactive' : 'active',
    items,
    total_ratio: totalRatio,
    expected_loss_rate: Number(firstPresent(detail.expected_loss_rate, fallback.expected_loss_rate, 0)),
    expected_yield_rate: Number(firstPresent(detail.expected_yield_rate, detail.yield_rate, fallback.expected_yield_rate, fallback.yield_rate, 0)),
    yield_rate: Number(firstPresent(detail.yield_rate, detail.expected_yield_rate, fallback.yield_rate, fallback.expected_yield_rate, 0)),
    updated_at: detail.updated_at || fallback.updated_at || '',
    can_edit_bom: true,
    bom_source_type: 'unbound_production_bom',
    effective_product_id: 0,
    effective_bom_version_id: versionID,
    production_bom_id: bomID,
    production_bom_code: detail.code || detail.production_bom_code || fallback.production_bom_code || fallback.code || '',
    production_bom_name: detail.name || detail.production_bom_name || fallback.production_bom_name || fallback.name || '',
    production_bom_group_id: Number(firstPresent(detail.group_id, detail.production_bom_group_id, fallback.production_bom_group_id, fallback.group_id, 0)),
    production_bom_group_name: detail.group_name || detail.production_bom_group_name || fallback.production_bom_group_name || fallback.group_name || '',
    production_bom_version_id: versionID,
    production_bom_version_no: versionNo,
    latest_bom_version_id: versionID,
    latest_bom_version_no: versionNo,
    is_latest_bom_version: true,
    referenced_products: Array.isArray(detail.referenced_products) ? detail.referenced_products : [],
  }
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
      row.product,
      row.code,
      row.name,
      row.production_bom_code,
      row.production_bom_name,
      row.group_name,
      row.production_bom_group_name,
      row.latest_version_no,
      row.production_bom_version_no,
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
  const code = String(row.production_bom_code || row.productionBomCode || row.code || '').trim()
  const name = String(row.production_bom_name || row.productionBomName || row.name || '').trim()
  const version = String(row.production_bom_version_no || row.productionBomVersionNo || row.latest_bom_version_no || row.latestBomVersionNo || row.latest_version_no || row.latestVersionNo || '').trim()
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
