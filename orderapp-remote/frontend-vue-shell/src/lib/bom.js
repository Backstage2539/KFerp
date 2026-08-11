import { clampPage, normalizePageSize } from './pagination.js'

function productionBomTopLevelGroup(group = {}) {
  return Boolean(group?.is_template_group || group?.unclassified || group?.all)
}

function productionBomTemplateRootKeyByID(groups = []) {
  return new Map((Array.isArray(groups) ? groups : [])
    .filter((group) => group?.is_template_group && Number(group?.group_id || 0) > 0)
    .map((group) => [Number(group.group_id || 0), String(group.key || '')]))
}

function productionBomRootKey(group = {}, templateRootKeyByID = new Map()) {
  if (productionBomTopLevelGroup(group)) return String(group?.key || '')
  return templateRootKeyByID.get(Number(group?.group_id || 0)) || ''
}

function productionBomCategoryHiddenByCollapse(group = {}, groups = [], collapsed = new Set(), includeSelf = true) {
  if (includeSelf && collapsed.has(String(group?.key || ''))) return true
  const groupID = Number(group?.group_id || 0)
  let parentID = Number(group?.parent_group_item_id || 0)
  if (!(groupID > 0) || !(parentID > 0)) return false
  const byItemID = new Map((Array.isArray(groups) ? groups : [])
    .filter((candidate) => Number(candidate?.group_id || 0) === groupID && Number(candidate?.group_item_id || 0) > 0)
    .map((candidate) => [Number(candidate.group_item_id || 0), candidate]))
  const seen = new Set()
  while (parentID > 0 && !seen.has(parentID)) {
    seen.add(parentID)
    const parent = byItemID.get(parentID)
    if (!parent) return false
    if (collapsed.has(String(parent.key || ''))) return true
    parentID = Number(parent.parent_group_item_id || 0)
  }
  return false
}

function productionBomCategoryAncestors(group = {}, groups = []) {
  const groupID = Number(group?.group_id || 0)
  let parentID = Number(group?.parent_group_item_id || 0)
  if (!(groupID > 0) || !(parentID > 0)) return []
  const byItemID = new Map((Array.isArray(groups) ? groups : [])
    .filter((candidate) => Number(candidate?.group_id || 0) === groupID && Number(candidate?.group_item_id || 0) > 0)
    .map((candidate) => [Number(candidate.group_item_id || 0), candidate]))
  const ancestors = []
  const seen = new Set()
  while (parentID > 0 && !seen.has(parentID)) {
    seen.add(parentID)
    const parent = byItemID.get(parentID)
    if (!parent) break
    ancestors.push(parent)
    parentID = Number(parent.parent_group_item_id || 0)
  }
  return ancestors
}

export function productionBomAccordionPageState(flatGroups = [], {
  expandedGroupKey = null,
  collapsedCategoryKeys = [],
  page = 1,
  pageSize = 10,
} = {}) {
  const sourceGroups = Array.isArray(flatGroups) ? flatGroups : []
  const templateRootKeyByID = productionBomTemplateRootKeyByID(sourceGroups)
  const roots = sourceGroups.filter(productionBomTopLevelGroup)
  const rootKeys = new Set(roots.map((group) => String(group.key || '')).filter(Boolean))
  const onlyFlatRoot = roots.length === 1 && roots[0]?.all
  const requestedKey = expandedGroupKey === null || typeof expandedGroupKey === 'undefined'
    ? String(roots[0]?.key || '')
    : String(expandedGroupKey || '')
  const resolvedExpandedGroupKey = onlyFlatRoot
    ? String(roots[0]?.key || '')
    : (requestedKey === '' || rootKeys.has(requestedKey) ? requestedKey : String(roots[0]?.key || ''))
  const normalizedSize = normalizePageSize(pageSize)
  const collapsed = new Set((Array.isArray(collapsedCategoryKeys) ? collapsedCategoryKeys : []).map((key) => String(key || '')))
  const activeSectionGroups = sourceGroups.filter((group) => (
    productionBomRootKey(group, templateRootKeyByID) === resolvedExpandedGroupKey
  ))
  const activeRoot = roots.find((group) => String(group.key || '') === resolvedExpandedGroupKey) || null
  const activeRowGroups = activeRoot?.is_template_group
    ? activeSectionGroups.filter((group) => !group?.is_template_group)
    : activeRoot ? [activeRoot] : []
  const rawTotal = activeRowGroups.reduce((sum, group) => sum + (Array.isArray(group?.rows) ? group.rows.length : 0), 0)
  const entries = []
  for (const group of activeRowGroups) {
    if (!productionBomTopLevelGroup(group) && productionBomCategoryHiddenByCollapse(group, activeRowGroups, collapsed, true)) continue
    for (const row of Array.isArray(group?.rows) ? group.rows : []) {
      entries.push({ groupKey: String(group.key || ''), row })
    }
  }
  const total = entries.length
  const normalizedPage = clampPage(page, total, normalizedSize)
  const start = (normalizedPage - 1) * normalizedSize
  const pageEntries = entries.slice(start, start + normalizedSize)
  const pageRowsByGroupKey = new Map()
  for (const entry of pageEntries) {
    if (!pageRowsByGroupKey.has(entry.groupKey)) pageRowsByGroupKey.set(entry.groupKey, [])
    pageRowsByGroupKey.get(entry.groupKey).push(entry.row)
  }

  const requiredCategoryKeys = new Set(pageEntries.map((entry) => entry.groupKey))
  for (const group of activeRowGroups) {
    const key = String(group?.key || '')
    const isVisibleCollapsedHeader = collapsed.has(key)
      && !productionBomCategoryHiddenByCollapse(group, activeRowGroups, collapsed, false)
    if (isVisibleCollapsedHeader) requiredCategoryKeys.add(key)
    if (!requiredCategoryKeys.has(key)) continue
    for (const ancestor of productionBomCategoryAncestors(group, activeRowGroups)) {
      requiredCategoryKeys.add(String(ancestor.key || ''))
    }
  }
  const groups = []
  for (const group of sourceGroups) {
    const rootKey = productionBomRootKey(group, templateRootKeyByID)
    const isRoot = productionBomTopLevelGroup(group)
    if (!isRoot && rootKey !== resolvedExpandedGroupKey) continue
    if (!isRoot && !requiredCategoryKeys.has(String(group.key || ''))) continue
    const rows = rootKey === resolvedExpandedGroupKey
      ? (pageRowsByGroupKey.get(String(group.key || '')) || [])
      : []
    groups.push({
      ...group,
      rows,
      row_total: Array.isArray(group?.rows) ? group.rows.length : 0,
    })
  }

  return {
    expandedGroupKey: resolvedExpandedGroupKey,
    groups,
    visibleRows: pageEntries.map((entry) => entry.row),
    rawTotal,
    total,
    page: normalizedPage,
    pageSize: normalizedSize,
    needsPagination: total > normalizedSize,
  }
}

export function normalizedBomProductKind(row = {}) {
  const kind = String(row.product_kind || row.productKind || '').trim().toLowerCase()
  return kind || 'roasted_bean'
}

export function isActiveBomProductOption(row = {}) {
  if (row.active === false || row.active === 0) return false
  const status = String(row.status || row.product_status || row.productStatus || '').trim().toLowerCase()
  if (['inactive', 'disabled', 'deprecated', 'deactivated', 'archived', 'false', '0', '失效'].includes(status)) return false
  return true
}

export function isBomProductCandidate(row = {}) {
  return isActiveBomProductOption(row) && normalizedBomProductKind(row) !== 'green_bean'
}

export function isProductionBomOutputProductCandidate(row = {}) {
  return isActiveBomProductOption(row)
}

export function bomProductCode(row = {}) {
  const explicit = String(row.product_code || row.productCode || row.sku_code || row.skuCode || row.code || '').trim()
  if (explicit) return explicit
  const id = Number(row.product_id || row.productID || row.id || 0)
  return id > 0 ? `SKU-${String(id).padStart(6, '0')}` : ''
}

export function bomProductOptionLabel(row = {}) {
  const name = String(row.name || row.product_name || row.productName || row.product || '').trim()
  const code = bomProductCode(row)
  if (!name) return code
  if (!code || name.toLowerCase().includes(code.toLowerCase())) return name
  return `${code} ${name}`
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
    product_id: Number(firstPresent(detail.output_product_id, fallback.output_product_id, 0)),
    product: detail.output_product_name || fallback.output_product_name || '未设置产出商品',
    product_name: detail.output_product_name || fallback.output_product_name || '未设置产出商品',
    output_product_id: Number(firstPresent(detail.output_product_id, fallback.output_product_id, 0)),
    output_product_name: detail.output_product_name || fallback.output_product_name || '',
    output_product_code: detail.output_product_code || fallback.output_product_code || '',
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
    production_bom_group_id: Number(firstPresent(detail.business_group_id, detail.group_id, detail.production_bom_group_id, fallback.business_group_id, fallback.production_bom_group_id, fallback.group_id, 0)),
    production_bom_group_name: detail.business_group_name || detail.group_name || detail.production_bom_group_name || fallback.business_group_name || fallback.production_bom_group_name || fallback.group_name || '',
    production_bom_group_category_id: Number(firstPresent(detail.group_item_id, detail.business_group_item_id, detail.group_category_id, detail.production_bom_group_category_id, fallback.group_item_id, fallback.business_group_item_id, fallback.production_bom_group_category_id, fallback.group_category_id, 0)),
    production_bom_group_category_name: detail.group_item_name || detail.group_category_name || detail.production_bom_group_category_name || fallback.group_item_name || fallback.production_bom_group_category_name || fallback.group_category_name || '',
    production_bom_version_id: versionID,
    production_bom_version_no: versionNo,
    latest_bom_version_id: versionID,
    latest_bom_version_no: versionNo,
    is_latest_bom_version: true,
    referenced_products: Array.isArray(detail.referenced_products) ? detail.referenced_products : [],
    used_by_boms: Array.isArray(detail.used_by_boms) ? detail.used_by_boms : [],
  }
}

export function filterProductionBomCatalog(rows = [], { status = 'active', query = '', groupID = 0, groupItemID = 0 } = {}) {
  const statusMode = String(status || 'active').trim().toLowerCase()
  const keyword = String(query || '').trim().toLowerCase()
  const selectedGroupID = Number(groupID || 0)
  const selectedGroupItemID = Number(groupItemID || 0)
  return rows.filter((row) => {
    const rowStatus = String(row.status || 'active').trim().toLowerCase()
    if (statusMode === 'active' && rowStatus === 'inactive') return false
    if (statusMode === 'inactive' && rowStatus !== 'inactive') return false
    const rowGroupID = Number(row.business_group_id || row.group_id || row.production_bom_group_id || 0)
    const rowGroupItemID = Number(row.group_item_id || row.business_group_item_id || row.group_category_id || row.production_bom_group_category_id || 0)
    if (selectedGroupID > 0 && selectedGroupItemID > 0 && (rowGroupID !== selectedGroupID || rowGroupItemID !== selectedGroupItemID)) return false
    if (selectedGroupID > 0 && selectedGroupItemID === -1 && rowGroupID === selectedGroupID && rowGroupItemID > 0) return false
    if (selectedGroupID > 0) {
      if (!keyword) return true
    } else {
      if (selectedGroupItemID > 0 && rowGroupItemID !== selectedGroupItemID) return false
      if (selectedGroupItemID === -1 && rowGroupItemID > 0) return false
      if (selectedGroupID === -1 && rowGroupID > 0) return false
    }
    if (!keyword) return true
    const haystack = [
      row.product,
      row.code,
      row.name,
      row.output_product_name,
      row.output_product_code,
      row.production_bom_code,
      row.production_bom_name,
      row.group_name,
      row.production_bom_group_name,
      row.group_category_name,
      row.production_bom_group_category_name,
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

export function normalizeProductionBomName(value = '') {
  let name = String(value || '').trim()
  name = name.replace(/^BOM-?\d+\s*/i, '')
  name = name.replace(/\s*\/\s*V\d+\s*$/i, '')
  name = name.replace(/\s+(?:生产\s*BOM|Production\s+BOM|BOM)((?:\s+(?:特殊属性)?副本)*)\s*$/i, '$1')
  return name.trim()
}

export function productionBomLabel(row = {}) {
  const code = String(row.production_bom_code || row.productionBomCode || row.code || '').trim()
  const rawName = String(row.production_bom_name || row.productionBomName || row.name || '').trim()
  const name = normalizeProductionBomName(rawName)
  const version = String(row.production_bom_version_no || row.productionBomVersionNo || row.latest_bom_version_no || row.latestBomVersionNo || row.latest_version_no || row.latestVersionNo || '').trim()
  const status = String(row.bom_status || row.status || '').trim()
  if (name) return name
  if (status === 'missing') return '无生产 BOM'
  if (rawName || code || version || Number(row.production_bom_id || row.productionBomID || row.bom_id || row.id || 0) > 0) return '未命名 BOM'
  return '无生产 BOM'
}

export function productionBomListName(row = {}) {
  const name = normalizeProductionBomName(row.production_bom_name || row.productionBomName || row.name || '')
  return name || '未命名 BOM'
}

export function productionBomVersionWarning(row = {}) {
  const current = String(row.production_bom_version_no || row.productionBomVersionNo || '').trim()
  const latest = String(row.latest_bom_version_no || row.latestBomVersionNo || '').trim()
  const rawLatest = row.is_latest_bom_version ?? row.isLatestBomVersion
  const isLatest = rawLatest === true || rawLatest === 'true' || rawLatest === 1
  if (!current || !latest || isLatest || current === latest) return ''
  return `当前引用 ${current}，最新 ${latest}`
}

export const BOM_KIND_PRODUCT = 'product'
export const BOM_KIND_SPEC_PACKAGING = 'spec_packaging'

export function normalizeBomKind(kind) {
  const value = String(kind || '').trim()
  if (value === BOM_KIND_SPEC_PACKAGING) return BOM_KIND_SPEC_PACKAGING
  return BOM_KIND_PRODUCT
}

export function isPackagingBomKind(kind) {
  return normalizeBomKind(kind) === BOM_KIND_SPEC_PACKAGING
}

export function isSemiFinishedProduct(product = {}) {
  return Boolean(product?.is_semi_finished)
}

export function semiFinishedPackagingRequiredError(missingSpecs = []) {
  return {
    code: 'semi_finished_packaging_required',
    missing_specs: Array.isArray(missingSpecs) ? missingSpecs : [],
    message: '半成品规格需要进行包装：' + (Array.isArray(missingSpecs) ? missingSpecs.join('、') : ''),
  }
}

export function specPackagingBomRefKey(unitTemplateId, specKey) {
  return `${unitTemplateId}:${specKey}`
}

export function bomSourceLabel(row = {}) {
  return productionBomLabel(row)
}
