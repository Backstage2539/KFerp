import { clampPage, normalizePageSize, slicePageRows } from './pagination.js'

export const PRODUCT_KIND_ALL = 'all'
export const SKU_CUSTOM_TYPE_ALL = 'all'

export const skuTypeOptions = [
  { value: SKU_CUSTOM_TYPE_ALL, label: '全部类型' },
  { value: 'standard', label: '标准' },
  { value: 'public_sku_alias', label: '公共 SKU 改名' },
  { value: 'custom_roast', label: '定制烘焙' },
  { value: 'custom_blend', label: '定制拼配' },
]

export const priceListRulePricingModeOptions = [
  { value: 'inherit_gradient_template', label: '按阶梯价模板' },
  { value: 'fixed_unit_price', label: '固定单价' },
  { value: 'cost_plus', label: '成本加成' },
]

export const priceTablePricingModeOptions = [
  { value: 'tier_template', label: '按阶梯模板计算' },
  { value: 'pricing_rule', label: '按价格计算模板计算' },
  { value: 'fixed_price', label: '固定价' },
]

export const priceListRuleRoundingOptions = [
  { value: 'none', label: '不取整' },
  { value: 'jiao', label: '保留到角' },
  { value: 'yuan', label: '保留到元' },
]

export function rowIsDeleted(row = {}) {
  if (row?.deleted === true) return true
  if (row?.deleted_at || row?.deletedAt) return true
  const state = String(row?.template_state || row?.templateState || '').trim().toLowerCase()
  return state === 'deleted'
}

export function visibleNonDeletedRows(rows = []) {
  return (Array.isArray(rows) ? rows : []).filter((row) => !rowIsDeleted(row))
}

export const integerUnitModeOptions = [
  { value: 'inherit', label: '继承子类型' },
  { value: 'integer', label: '只允许整数' },
  { value: 'decimal', label: '允许小数' },
]

export function normalizedProductKind(row = {}) {
  const kind = String(row?.product_kind || '').trim()
  if (kind === 'green_bean') return 'green_bean'
  if (kind === 'drip_bag') return 'drip_bag'
  if (kind === 'instant_coffee' || kind === 'instant') return 'instant_coffee'
  return 'roasted'
}

export function inferProductKindFromProductTypeCategory(category = {}) {
  const text = `${category?.name || ''} ${category?.source_name || ''}`.trim().toLowerCase()
  if (!text) return 'roasted'
  if (text.includes('速溶') || text.includes('冻干') || text.includes('instant')) return 'instant_coffee'
  if (text.includes('挂耳') || text.includes('drip')) return 'drip_bag'
  if (text.includes('生豆') || text.includes('green')) return 'green_bean'
  return 'roasted'
}

export function productKindRequiresRoast(kindOrRow = {}) {
  const kind = typeof kindOrRow === 'object' ? normalizedProductKind(kindOrRow) : normalizedProductKind({ product_kind: kindOrRow })
  return kind !== 'green_bean'
}

export function productKindSupportsBomParams(kindOrRow = {}) {
  const kind = typeof kindOrRow === 'object' ? normalizedProductKind(kindOrRow) : normalizedProductKind({ product_kind: kindOrRow })
  return kind !== 'green_bean'
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
  const active = String(filters.active || 'all').trim()
  return (rows || []).filter((row) => {
    if (productKind && productKind !== PRODUCT_KIND_ALL && normalizedProductKind(row) !== productKind) return false
    if (customType && customType !== SKU_CUSTOM_TYPE_ALL && skuTypeValue(row) !== customType) return false
    if (query) {
      const haystack = `${row.name || ''} ${row.number || ''} ${skuTypeLabel(row.custom_type)} ${row.remark || ''} ${row.sku_search_text || ''}`.toLowerCase()
      if (!haystack.includes(query)) return false
    }
    if (primaryCategory && String(row.primary_name || '') !== primaryCategory) return false
    if (secondaryCategory && String(row.secondary_name || '') !== secondaryCategory) return false
    if (active === 'active' && row.active === false) return false
    if (active === 'inactive' && row.active !== false) return false
    return true
  })
}

export function normalizeVisibleSkuFilters(filters = {}, rows = null) {
  const normalized = {
    productKind: PRODUCT_KIND_ALL,
    customType: SKU_CUSTOM_TYPE_ALL,
    active: String(filters.active || 'active').trim(),
    query: String(filters.query || '').trim(),
    primaryCategory: String(filters.primaryCategory || '').trim(),
    secondaryCategory: String(filters.secondaryCategory || '').trim(),
  }
  if (Array.isArray(rows)) {
    const primaryOptions = primaryCategoryOptions(rows)
    if (normalized.primaryCategory && !primaryOptions.includes(normalized.primaryCategory)) {
      normalized.primaryCategory = ''
    }
    const secondaryOptions = secondaryCategoryOptions(rows, normalized.primaryCategory)
    if (normalized.secondaryCategory && !secondaryOptions.includes(normalized.secondaryCategory)) {
      normalized.secondaryCategory = ''
    }
  }
  return normalized
}

export function paginatedSkuRows(rows = [], filters = {}, pagination = {}) {
  return slicePageRows(filterSkuRows(rows, filters), pagination)
}

export function skuTableState(rows = [], filters = {}, pagination = {}) {
  const sourceRows = Array.isArray(rows) ? rows : []
  const normalizedFilters = normalizeVisibleSkuFilters(filters, sourceRows)
  const filteredRows = filterSkuRows(sourceRows, normalizedFilters)
  const pageSize = normalizePageSize(pagination.pageSize)
  const page = clampPage(pagination.page, filteredRows.length, pageSize)
  const start = (page - 1) * pageSize
  return {
    filters: normalizedFilters,
    primaryOptions: primaryCategoryOptions(sourceRows),
    secondaryOptions: secondaryCategoryOptions(sourceRows, normalizedFilters.primaryCategory),
    total: filteredRows.length,
    page,
    pageSize,
    rows: filteredRows.slice(start, start + pageSize),
  }
}

export function skuGroupTableState(groups = [], paginationByGroup = {}, options = {}) {
  const sourceGroups = Array.isArray(groups) ? groups : []
  const sourcePagination = paginationByGroup && typeof paginationByGroup === 'object'
    ? paginationByGroup
    : {}
  const defaultPageSize = normalizePageSize(options.defaultPageSize)
  const pagination = {}
  const visibleRows = []

  const paginatedGroups = sourceGroups.map((group, index) => {
    const key = String(group?.key || `sku-group-${index}`)
    const sourceRows = Array.isArray(group?.rows) ? group.rows : []
    const requested = sourcePagination[key] || {}
    const pageSize = normalizePageSize(requested.pageSize || defaultPageSize)
    const page = clampPage(requested.page, sourceRows.length, pageSize)
    const rows = slicePageRows(sourceRows, { page, pageSize })
    pagination[key] = { page, pageSize }
    visibleRows.push(...rows)
    return {
      ...group,
      key,
      total: sourceRows.length,
      page,
      pageSize,
      needsPagination: sourceRows.length > pageSize,
      rows,
    }
  })

  return {
    groups: paginatedGroups,
    pagination,
    visibleRows,
    total: sourceGroups.reduce((sum, group) => sum + (Array.isArray(group?.rows) ? group.rows.length : 0), 0),
  }
}

export function visibleSkuGroupRows(groups = [], collapsedGroupKeys = []) {
  const collapsed = new Set((Array.isArray(collapsedGroupKeys) ? collapsedGroupKeys : []).map((key) => String(key || '')))
  return (Array.isArray(groups) ? groups : []).flatMap((group) => (
    collapsed.has(String(group?.key || ''))
      || skuGroupHiddenByCollapsedAncestor(groups, group, collapsedGroupKeys)
      || !Array.isArray(group?.rows)
      ? []
      : group.rows
  ))
}

export function selectedSkuRowIDsAfterVisibleToggle(selectedRowIDs = [], visibleRows = [], checked = false) {
  const selected = [...new Set((Array.isArray(selectedRowIDs) ? selectedRowIDs : [])
    .map((id) => Number(id || 0))
    .filter((id) => id > 0))]
  const visibleIDs = [...new Set((Array.isArray(visibleRows) ? visibleRows : [])
    .map((row) => Number(row?.id || 0))
    .filter((id) => id > 0))]
  const visibleSet = new Set(visibleIDs)
  if (!checked) return selected.filter((id) => !visibleSet.has(id))
  const selectedSet = new Set(selected)
  return [...selected, ...visibleIDs.filter((id) => !selectedSet.has(id))]
}

export function skuGroupHiddenByCollapsedAncestor(groups = [], group = {}, collapsedGroupKeys = []) {
  const collapsed = new Set((Array.isArray(collapsedGroupKeys) ? collapsedGroupKeys : []).map((key) => String(key || '')))
  const groupID = Number(group?.group_id || 0)
  if (!collapsed.size || !(groupID > 0)) return false
  if (!group?.is_template_group) {
    const templateHeader = (Array.isArray(groups) ? groups : [])
      .find((candidate) => Number(candidate?.group_id || 0) === groupID && candidate?.is_template_group)
    if (templateHeader && collapsed.has(String(templateHeader.key || ''))) return true
  }
  let parentID = Number(group?.parent_group_item_id || 0)
  if (!(parentID > 0)) return false
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

export function skuListRowsFromProducts(products = [], categoryTree = [], filterFn = () => true) {
  const categoryMetaByProductID = categoryProductMetaByID(categoryTree)
  const categoryMetaByCategoryID = categoryPathMetaByID(categoryTree)
  return (products || [])
    .filter((product) => {
      try {
        return filterFn(product)
      } catch (_) {
        return false
      }
    })
    .map((product) => ({
      ...product,
      ...(categoryMetaByProductID.get(Number(product?.id || 0))
        || categoryMetaByCategoryID.get(Number(product?.product_category_id || 0))
        || {}),
    }))
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

export function buildCustomerProductAliasPayload(form = {}) {
  return {
    id: Number(form.id || 0),
    customer_id: Number(form.customer_id || form.customerID || 0),
    product_id: Number(form.product_id || form.productID || 0),
    display_name: String(form.display_name ?? form.displayName ?? '').trim(),
    brand_name: String(form.brand_name ?? form.brandName ?? '').trim(),
    display_category_id: Number(form.display_category_id || form.displayCategoryID || 0),
    sort_order: Number(form.sort_order || form.sortOrder || 0),
    include_in_price_list: Boolean(form.include_in_price_list ?? form.includeInPriceList ?? true),
    active: Boolean(form.active ?? true),
    remark: String(form.remark ?? '').trim(),
  }
}

export function buildProductCustomerReferencePayload(form = {}) {
  return {
    id: Number(form.id || 0),
    product_id: Number(form.product_id || form.productID || 0),
    customer_id: Number(form.customer_id || form.customerID || 0),
    customer_item_code: String(form.customer_item_code ?? form.customerItemCode ?? form.ref_code ?? '').trim(),
    customer_display_name: String(form.customer_display_name ?? form.customerDisplayName ?? form.display_name ?? '').trim(),
    material_source_mode: String(form.material_source_mode ?? form.materialSourceMode ?? 'factory').trim().toLowerCase() === 'customer' ? 'customer' : 'factory',
    active: Boolean(form.active ?? true),
    remark: String(form.remark ?? '').trim(),
  }
}

export function buildBusinessGroupAssignmentPayload(form = {}) {
  const objectID = Number(form.object_id ?? form.objectID ?? 0) || 0
  return {
    id: Number(form.id || 0),
    usage_key: String(form.usage_key ?? form.usageKey ?? '').trim(),
    object_key: String(form.object_key ?? form.objectKey ?? '').trim(),
    object_id: objectID,
    object_ref: objectID > 0 ? '' : String(form.object_ref ?? form.objectRef ?? '').trim(),
    group_id: Number(form.group_id ?? form.groupID ?? 0) || 0,
    group_item_id: Number(form.group_item_id ?? form.groupItemID ?? 0) || 0,
    sort_order: Number(form.sort_order ?? form.sortOrder ?? 100) || 100,
  }
}

function flattenBusinessGroupItems(items = [], parent = null, out = []) {
  for (const item of Array.isArray(items) ? items : []) {
    const row = { ...item, parent_id: Number(item.parent_id ?? item.parentID ?? parent?.id ?? 0) || 0 }
    out.push(row)
    flattenBusinessGroupItems(item.children || item.Children || [], row, out)
  }
  return out
}

export function businessGroupItemsTree(items = []) {
  const flat = flattenBusinessGroupItems(items)
    .filter((item) => item?.active !== false)
    .map((item) => ({
      ...item,
      id: Number(item.id || 0),
      group_id: Number(item.group_id ?? item.groupID ?? 0) || 0,
      parent_id: Number(item.parent_id ?? item.parentID ?? 0) || 0,
      sort_order: Number(item.sort_order ?? item.sortOrder ?? item.position ?? 100) || 100,
      children: [],
    }))
    .filter((item) => item.id > 0)
  const byID = new Map(flat.map((item) => [Number(item.id || 0), item]))
  const roots = []
  for (const item of flat) {
    const parent = byID.get(Number(item.parent_id || 0))
    if (parent && Number(parent.id || 0) !== Number(item.id || 0)) {
      parent.children.push(item)
    } else {
      roots.push(item)
    }
  }
  const sortItems = (rows = []) => {
    rows.sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || Number(a.id || 0) - Number(b.id || 0))
    for (const row of rows) sortItems(row.children || [])
    return rows
  }
  return sortItems(roots)
}

function businessGroupByID(groups = [], groupID = 0) {
  const id = Number(groupID || 0)
  return (Array.isArray(groups) ? groups : []).find((group) => Number(group?.id || 0) === id) || null
}

export function isSystemDefaultBusinessGroup(group = {}) {
  const code = String(group.code || '').trim().toLowerCase()
  if (code.startsWith('default_')) return true
  return ['商品默认分组', '生产 BOM 默认分组', '仓库库存默认分组'].includes(String(group.name || '').trim())
}

export function businessGroupVisibleName(group = {}) {
  if (!group || isSystemDefaultBusinessGroup(group)) return ''
  return String(group.name || '').trim()
}

function businessGroupItemPath(group = {}, groupItemID = 0) {
  return businessGroupItemInfo(group, groupItemID).path
}

function businessGroupItemInfo(group = {}, groupItemID = 0) {
  const items = flattenBusinessGroupItems(businessGroupItemsTree(group.items || []))
  const byID = new Map(items.map((item) => [Number(item.id || 0), item]))
  let cursor = byID.get(Number(groupItemID || 0))
  if (!cursor) return { item: null, path: [], depth: 0, order: 9999, parent_group_item_id: 0 }
  const path = []
  const seen = new Set()
  const order = items.findIndex((item) => Number(item.id || 0) === Number(groupItemID || 0))
  let item = cursor
  while (cursor && Number(cursor.id || 0) > 0 && !seen.has(Number(cursor.id || 0))) {
    seen.add(Number(cursor.id || 0))
    const name = String(cursor.name || '').trim()
    if (name) path.unshift(name)
    cursor = byID.get(Number(cursor.parent_id || cursor.parentID || 0))
  }
  return {
    item,
    path,
    depth: Math.max(path.length - 1, 0),
    order: order >= 0 ? order : 9999,
    parent_group_item_id: Number(item.parent_id || item.parentID || 0),
  }
}

export function businessGroupItemLabel(group = {}, groupItemID = 0, options = {}) {
  const includeGroupName = options.includeGroupName !== false
  const groupName = businessGroupVisibleName(group)
  const path = businessGroupItemPath(group, groupItemID)
  if (!path.length) return includeGroupName ? (groupName || '未分组') : '未分组'
  return [includeGroupName ? groupName : '', ...path].filter(Boolean).join(' / ')
}

export function businessGroupAssignmentLabel(assignment = {}, groups = [], options = {}) {
  const group = businessGroupByID(groups, assignment.group_id ?? assignment.groupID)
  if (!group) return '未分组'
  if (isSystemDefaultBusinessGroup(group)) return '未分组'
  const groupItemID = Number(assignment.group_item_id ?? assignment.groupItemID ?? 0)
  if (groupItemID <= 0) return businessGroupVisibleName(group) || '未分组'
  return businessGroupItemLabel(group, groupItemID, options)
}

export function businessGroupItemMoveOptions(groups = [], usageKey = '', options = {}) {
  const normalizedUsage = String(usageKey || '').trim().toLowerCase()
  const includeGroupsWithoutUsage = Boolean(options.includeGroupsWithoutUsage)
  const out = []
  for (const group of (Array.isArray(groups) ? groups : [])
    .filter((row) => row?.active !== false)
    .filter((row) => !isSystemDefaultBusinessGroup(row))
    .filter((row) => {
      if (!normalizedUsage) return true
      if (includeGroupsWithoutUsage) return true
      const usages = Array.isArray(row.usages) ? row.usages : []
      return usages.some((usage) => String(usage.usage_key || usage.usageKey || '').toLowerCase() === normalizedUsage && usage.active !== false)
    })
    .slice()
    .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || Number(a.id || 0) - Number(b.id || 0))) {
    for (const item of flattenBusinessGroupItems(businessGroupItemsTree(group.items || []))) {
      const itemID = Number(item.id || 0)
      if (!itemID) continue
      const itemInfo = businessGroupItemInfo(group, itemID)
      out.push({
        id: itemID,
        group_id: Number(group.id || 0),
        group_item_id: itemID,
        parent_group_item_id: Number(item.parent_id || item.parentID || 0),
        label: businessGroupItemLabel(group, itemID, { includeGroupName: options.includeGroupName !== false }),
        path_label: businessGroupItemLabel(group, itemID, { includeGroupName: false }),
        title_label: itemInfo.path[itemInfo.path.length - 1] || businessGroupItemLabel(group, itemID, { includeGroupName: false }),
        depth: itemInfo.depth,
      })
    }
  }
  return out
}

export function businessGroupDisplayGroups(rows = [], assignments = [], groups = [], {
  usageKey = 'product_catalog',
  objectKey = 'product',
  objectIDForRow = (row = {}) => Number(row.id || row.product_id || row.productID || 0),
} = {}) {
  const normalizedUsage = String(usageKey || '').trim()
  const normalizedObjectKey = String(objectKey || '').trim()
  const byRowID = new Map()
  for (const assignment of Array.isArray(assignments) ? assignments : []) {
    if (String(assignment.usage_key || assignment.usageKey || '') !== normalizedUsage) continue
    if (String(assignment.object_key || assignment.objectKey || '') !== normalizedObjectKey) continue
    const objectID = Number(assignment.object_id || assignment.objectID || 0)
    if (!objectID) continue
    byRowID.set(objectID, assignment)
  }
  const displayGroups = new Map()
  for (const row of Array.isArray(rows) ? rows : []) {
    const rawAssignment = byRowID.get(Number(objectIDForRow(row) || 0)) || null
    const assignmentGroup = rawAssignment ? businessGroupByID(groups, rawAssignment.group_id ?? rawAssignment.groupID) : null
    const assignment = assignmentGroup && !isSystemDefaultBusinessGroup(assignmentGroup) ? rawAssignment : null
    const groupItemID = Number(assignment?.group_item_id ?? assignment?.groupItemID ?? 0)
    const key = groupItemID ? `business-group-${Number(assignment.group_id || assignment.groupID || 0)}-${groupItemID}` : 'business-group-unassigned'
    const itemInfo = groupItemID ? businessGroupItemInfo(assignmentGroup, groupItemID) : { path: [], depth: 0, order: 9999, parent_group_item_id: 0 }
    const pathLabel = groupItemID ? businessGroupItemLabel(assignmentGroup, groupItemID, { includeGroupName: false }) : '未分组'
    const label = groupItemID ? (itemInfo.path[itemInfo.path.length - 1] || pathLabel) : '未分组'
    if (!displayGroups.has(key)) {
      displayGroups.set(key, {
        key,
        label,
        path_label: pathLabel,
        depth: itemInfo.depth,
        parent_group_item_id: itemInfo.parent_group_item_id,
        rows: [],
        all: false,
        sort_order: groupItemID ? (Number(assignmentGroup?.sort_order || 0) * 10000 + itemInfo.order) : 999999,
      })
    }
    displayGroups.get(key).rows.push(row)
  }
  return [...displayGroups.values()].sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || String(a.label).localeCompare(String(b.label), 'zh-Hans-CN'))
}

export function productCatalogGroupOfProduct(product = {}, assignments = [], groups = []) {
  const productID = Number(product.id || product.product_id || product.productID || 0)
  const assignment = (Array.isArray(assignments) ? assignments : []).find((row) => (
    String(row.usage_key || row.usageKey || '') === 'product_catalog'
    && String(row.object_key || row.objectKey || '') === 'product'
    && Number(row.object_id || row.objectID || 0) === productID
  )) || null
  return {
    assignment,
    label: assignment ? businessGroupAssignmentLabel(assignment, groups) : '未分组',
  }
}

export function buildPricingRulePayload(form = {}) {
  const calculationJSON = pricingRuleCalculationJSONFromForm(form)
  return {
    id: Number(form.id || 0),
    name: String(form.name ?? '').trim(),
    code: String(form.code ?? '').trim(),
    cost_source_mode: normalizePricingRuleCostSourceMode(form.cost_source_mode ?? form.costSourceMode),
    margin_rate: Number(form.margin_rate ?? form.marginRate ?? 0) || 0,
    tax_rate: Number(form.tax_rate ?? form.taxRate ?? 0) || 0,
    rounding_mode: String(form.rounding_mode ?? form.roundingMode ?? 'none').trim() || 'none',
    formula_version: String(form.formula_version ?? form.formulaVersion ?? 'v1').trim() || 'v1',
    calculation_json: calculationJSON,
    active: Boolean(form.active ?? true),
    remark: String(form.remark ?? '').trim(),
  }
}

export function pricingRuleEditorForm(rule = {}) {
  const calculation = pricingRuleCalculationObject(rule)
  const otherCosts = calculation.other_costs ?? calculation.otherCosts ?? {}
  const otherCostRows = otherCosts && typeof otherCosts === 'object' && !Array.isArray(otherCosts)
    ? Object.entries(otherCosts).map(([key, value]) => ({ key: String(key || '').trim(), value: Number(value || 0) })).filter((row) => row.key)
    : []
  return {
    id: Number(rule.id || 0),
    name: String(rule.name || ''),
    code: String(rule.code || ''),
    cost_source_mode: normalizePricingRuleCostSourceMode(rule.cost_source_mode ?? rule.costSourceMode),
    margin_rate: Number(rule.margin_rate ?? rule.marginRate ?? 0) || 0,
    tax_rate: Number(rule.tax_rate ?? rule.taxRate ?? 0) || 0,
    rounding_mode: String(rule.rounding_mode ?? rule.roundingMode ?? 'none') || 'none',
    formula_version: String(rule.formula_version ?? rule.formulaVersion ?? 'v1') || 'v1',
    calculation_json: calculation,
    other_cost_rows: otherCostRows.length ? otherCostRows : [{ key: '', value: 0 }],
    profit_method: 'markup',
    tax_mode: String(calculation.tax_mode || 'tax_included'),
    minimum_margin_rate: Number(calculation.minimum_margin_rate || 0),
    trial_note: String(calculation.trial_note || ''),
    active: rule.active !== false,
    remark: String(rule.remark || ''),
  }
}

export function pricingRuleEditorLegacyBlocked(rule = {}) {
  const calculation = pricingRuleCalculationObject(rule)
  const rawMethod = String(calculation.profit_method ?? rule.profit_method ?? '').trim().toLowerCase()
  return Boolean(String(calculation.legacy_profit_method || '').trim()
    || String(calculation.migration_warning || '').trim()
    || (rawMethod && !['markup', 'gross_margin'].includes(rawMethod)))
}

export function pricingRuleEditorLegacyMethodLabel(rule = {}) {
  const calculation = pricingRuleCalculationObject(rule)
  return String(calculation.legacy_profit_method || calculation.profit_method || '未知').trim() || '未知'
}

export function pricingRuleEditorLegacyValueLabel(rule = {}) {
  const calculation = pricingRuleCalculationObject(rule)
  const value = calculation.legacy_margin_rate ?? calculation.legacy_fixed_amount ?? rule.margin_rate
  return value === null || typeof value === 'undefined' || String(value).trim() === '' ? '未记录' : String(value)
}

function pricingRuleCalculationObject(rule = {}) {
  const raw = rule.calculation_json ?? rule.calculationJSON ?? {}
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) return { ...raw }
  if (typeof raw === 'string' && raw.trim()) {
    try {
      const parsed = JSON.parse(raw)
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
    } catch {
      return {}
    }
  }
  return {}
}

export function buildPricingRuleCopyPayload(rule = {}, existingRules = []) {
  const source = buildPricingRulePayload(rule)
  return {
    ...source,
    id: 0,
    name: pricingRuleCopyName(source.name || source.code || '价格计算模板', existingRules),
    code: source.code ? pricingRuleCopyCode(source.code, existingRules) : '',
    active: true,
  }
}

export function buildPricingRuleUpdateFromTrial(rule = {}, trialForm = {}) {
  const base = buildPricingRulePayload(rule)
  const next = {
    ...base,
    calculation_json: { ...(base.calculation_json || {}) },
  }
  const marginRate = optionalNumberFromForm(trialForm.margin_rate ?? trialForm.marginRate)
  if (marginRate !== null) next.margin_rate = marginRate
  const taxRate = optionalNumberFromForm(trialForm.tax_rate ?? trialForm.taxRate)
  if (taxRate !== null) next.tax_rate = taxRate
  const otherCostRows = trialForm.other_cost_rows ?? trialForm.otherCostRows
  if (Array.isArray(otherCostRows)) {
    next.other_cost_rows = otherCostRows.map((row) => ({ ...row }))
  } else {
    const otherCosts = trialForm.other_costs ?? trialForm.otherCosts
    if (otherCosts && typeof otherCosts === 'object' && !Array.isArray(otherCosts)) {
      next.other_costs = { ...otherCosts }
    }
  }
  return buildPricingRulePayload(next)
}

let pricingRuleTrialReturnStateSequence = 0
const pricingRuleTrialReturnStates = new Map()

function clonePricingRuleTrialReturnState(state = {}) {
  return JSON.parse(JSON.stringify(state || {}))
}

export function storePricingRuleTrialReturnState(state = {}) {
  const key = `pricing-rule-trial-return:${++pricingRuleTrialReturnStateSequence}`
  pricingRuleTrialReturnStates.set(key, clonePricingRuleTrialReturnState(state))
  while (pricingRuleTrialReturnStates.size > 10) {
    pricingRuleTrialReturnStates.delete(pricingRuleTrialReturnStates.keys().next().value)
  }
  return key
}

export function takePricingRuleTrialReturnState(key = '') {
  const normalized = String(key || '').trim()
  if (!normalized || !pricingRuleTrialReturnStates.has(normalized)) return null
  const state = pricingRuleTrialReturnStates.get(normalized)
  pricingRuleTrialReturnStates.delete(normalized)
  return clonePricingRuleTrialReturnState(state)
}

function pricingRuleTrialUnitKey(value = '') {
  return String(value || '').trim().toLowerCase()
}

function pricingRuleTrialUnitsEqual(a = '', b = '') {
  const left = normalizeOptionalUnitText(a)
  const right = normalizeOptionalUnitText(b)
  return Boolean(left && right && pricingRuleTrialUnitKey(left) === pricingRuleTrialUnitKey(right))
}

function pricingRuleTrialMassConversionFactor(fromUnit = '', toUnit = '') {
  const sourceGram = salesSpecWeightFactor(fromUnit)
  const targetGram = salesSpecWeightFactor(toUnit)
  if (!(sourceGram > 0 && targetGram > 0)) return 0
  return trimDecimal(sourceGram / targetGram)
}

function pricingRuleTrialAddConversionEdge(graph, fromUnit = '', toUnit = '', factorValue = 0) {
  const from = normalizeOptionalUnitText(fromUnit)
  const to = normalizeOptionalUnitText(toUnit)
  const factor = normalizePositiveNumber(factorValue)
  if (!from || !to || factor <= 0) return
  const fromKey = pricingRuleTrialUnitKey(from)
  if (!graph[fromKey]) graph[fromKey] = {}
  graph[fromKey][pricingRuleTrialUnitKey(to)] = trimDecimal(factor)
}

function pricingRuleTrialAddNetContentConversion(graph, salesUnit = '', netContentQty = 0, netContentUnit = '', inventoryUnit = '') {
  const unit = normalizeOptionalUnitText(salesUnit)
  const qty = normalizePositiveNumber(netContentQty)
  const sourceUnit = normalizeOptionalUnitText(netContentUnit)
  const targetUnit = normalizeOptionalUnitText(inventoryUnit)
  if (!unit || qty <= 0 || !sourceUnit) return
  if (!targetUnit || pricingRuleTrialUnitsEqual(sourceUnit, targetUnit)) {
    pricingRuleTrialAddConversionEdge(graph, unit, sourceUnit, qty)
    return
  }
  const factor = pricingRuleTrialMassConversionFactor(sourceUnit, targetUnit)
  if (factor > 0) {
    pricingRuleTrialAddConversionEdge(graph, unit, targetUnit, qty * factor)
    return
  }
  pricingRuleTrialAddConversionEdge(graph, unit, sourceUnit, qty)
}

function pricingRuleTrialProductConversionGraph(product = {}, inventoryUnit = '') {
  const graph = {}
  const targetInventoryUnit = normalizeOptionalUnitText(inventoryUnit) || normalizeOptionalUnitText(product.inventory_unit ?? product.inventoryUnit) || 'kg'
  const direct = parseJSONObject(product.unit_conversion_json ?? product.unitConversionJSON)
  const rule = parseJSONObject(product.unit_rule_override_json ?? product.unitRuleOverrideJSON)
  const conversion = Object.keys(direct).length
    ? direct
    : parseJSONObject(rule.unit_conversion_json ?? rule.conversion_json ?? {})
  for (const [fromUnit, rawTargets] of Object.entries(conversion)) {
    const directFactor = normalizePositiveNumber(rawTargets)
    if (directFactor > 0) {
      pricingRuleTrialAddConversionEdge(graph, fromUnit, targetInventoryUnit, directFactor)
      continue
    }
    const targets = parseJSONObject(rawTargets)
    for (const [toUnit, factorValue] of Object.entries(targets)) {
      pricingRuleTrialAddConversionEdge(graph, fromUnit, toUnit, factorValue)
    }
  }

  const salesSpecs = Array.isArray(product.sales_specs ?? product.salesSpecs)
    ? (product.sales_specs ?? product.salesSpecs)
    : parseJSONArray(product.sales_specs ?? product.salesSpecs)
  for (const row of salesSpecs) {
    const specUnit = row?.spec_name ?? row?.specName ?? row?.sales_unit ?? row?.salesUnit
    pricingRuleTrialAddNetContentConversion(
      graph,
      specUnit,
      row?.net_content_qty ?? row?.netContentQty,
      row?.net_content_unit ?? row?.netContentUnit,
      targetInventoryUnit,
    )
  }
  pricingRuleTrialAddNetContentConversion(
    graph,
    product.derived_sales_unit ?? product.derivedSalesUnit ?? product.default_sales_unit ?? product.defaultSalesUnit,
    product.net_content_qty ?? product.netContentQty,
    product.net_content_unit ?? product.netContentUnit,
    targetInventoryUnit,
  )
  return graph
}

function pricingRuleTrialResolveUnitFactor(fromUnit = '', toUnit = '', graph = {}, seen = new Set()) {
  const from = normalizeOptionalUnitText(fromUnit)
  const to = normalizeOptionalUnitText(toUnit)
  if (!from || !to) return 0
  if (pricingRuleTrialUnitsEqual(from, to)) return 1
  const massFactor = pricingRuleTrialMassConversionFactor(from, to)
  if (massFactor > 0) return massFactor
  const fromKey = pricingRuleTrialUnitKey(from)
  if (seen.has(fromKey)) return 0
  seen.add(fromKey)
  const targets = graph[fromKey] || {}
  const direct = normalizePositiveNumber(targets[pricingRuleTrialUnitKey(to)])
  if (direct > 0) return direct
  for (const [targetUnit, factorValue] of Object.entries(targets)) {
    const factor = normalizePositiveNumber(factorValue)
    if (factor <= 0) continue
    const targetFactor = pricingRuleTrialResolveUnitFactor(targetUnit, to, graph, seen)
    if (targetFactor > 0) return trimDecimal(factor * targetFactor)
  }
  return 0
}

function pricingRuleTrialUnitConversionFactor(product = {}, unit = '') {
  const sourceUnit = normalizeOptionalUnitText(unit)
  if (!sourceUnit) return 0
  const inventoryUnit = normalizeOptionalUnitText(product.inventory_unit ?? product.inventoryUnit) || 'kg'
  const graph = pricingRuleTrialProductConversionGraph(product, inventoryUnit)
  return pricingRuleTrialResolveUnitFactor(sourceUnit, inventoryUnit, graph)
}

function pricingRuleTrialProductUnitCandidates(product = {}) {
  const out = []
  const push = (value) => {
    const code = String(value || '').trim()
    if (code && !out.includes(code)) out.push(code)
  }
  push(product.derived_sales_unit ?? product.derivedSalesUnit)
  push(product.default_sales_unit ?? product.defaultSalesUnit)
  push(product.sales_unit ?? product.salesUnit)
  push(product.quote_unit ?? product.quoteUnit)
  push(product.order_unit ?? product.orderUnit)
  if (Array.isArray(product.sales_units)) product.sales_units.forEach(push)
  if (Array.isArray(product.salesUnits)) product.salesUnits.forEach(push)
  const salesSpecs = Array.isArray(product.sales_specs ?? product.salesSpecs)
    ? (product.sales_specs ?? product.salesSpecs)
    : parseJSONArray(product.sales_specs ?? product.salesSpecs)
  for (const row of salesSpecs) {
    push(row?.spec_name ?? row?.specName ?? row?.sales_unit ?? row?.salesUnit)
  }
  for (const fromUnit of Object.keys(parseJSONObject(product.unit_conversion_json ?? product.unitConversionJSON))) push(fromUnit)
  push(product.inventory_unit ?? product.inventoryUnit)
  return out
}

export function pricingRuleTrialQuoteUnitOptionsForProduct(unitOptions = [], product = {}) {
  const out = []
  const seen = new Set()
  const labels = new Map()
  const globalCodes = []
  const push = (code, name = '') => {
    const normalized = String(code || '').trim()
    if (!normalized || seen.has(normalized)) return
    if (pricingRuleTrialUnitConversionFactor(product, normalized) <= 0) return
    seen.add(normalized)
    out.push({ code: normalized, name: String(name || normalized).trim() || normalized })
  }
  for (const option of Array.isArray(unitOptions) ? unitOptions : []) {
    const code = String((option?.code ?? option?.value ?? option) || '').trim()
    if (!code) continue
    const name = String((option?.name ?? option?.label ?? option?.code ?? option) || code).trim() || code
    labels.set(code, name)
    globalCodes.push(code)
  }
  for (const code of pricingRuleTrialProductUnitCandidates(product)) {
    push(code, labels.get(code) || code)
  }
  for (const code of globalCodes) {
    push(code, labels.get(code) || code)
  }
  return out
}

export function pricingRuleTrialDefaultQuoteUnit(product = {}, unitOptions = []) {
  const options = pricingRuleTrialQuoteUnitOptionsForProduct(unitOptions, product)
  const available = new Set(options.map((unit) => unit.code))
  for (const code of pricingRuleTrialProductUnitCandidates(product)) {
    if (available.has(code)) return code
  }
  if (available.has('kg')) return 'kg'
  return options[0]?.code || ''
}

export function buildPricingRuleTrialPayload(form = {}) {
  const overrides = {}
  const baseCost = optionalNumberFromForm(form.base_cost ?? form.baseCost)
  if (baseCost !== null) overrides.base_cost = baseCost
  const marginRate = optionalNumberFromForm(form.margin_rate ?? form.marginRate)
  if (marginRate !== null) overrides.margin_rate = marginRate
  const taxRate = optionalNumberFromForm(form.tax_rate ?? form.taxRate)
  if (taxRate !== null) overrides.tax_rate = taxRate
  const otherCosts = pricingRuleTrialOtherCostMapFromForm(form)
  if (Object.keys(otherCosts).length) overrides.other_costs = otherCosts

  const bomSpecID = Number(form.bom_spec_id ?? form.bomSpecID ?? 0) || 0
  const bomVariantID = Number(form.bom_variant_id ?? form.bomVariantID ?? 0) || 0
  const parentProductID = Number(form.parent_product_id ?? form.parentProductID ?? 0) || 0
  const payload = {
    pricing_rule_id: Number(form.pricing_rule_id ?? form.pricingRuleID ?? form.rule_id ?? form.ruleID ?? 0) || 0,
    // PR-608: product_id always means the main product.  BOM specification
    // identity is carried by bom_id/version/spec/variant and never by a child
    // SKU or default_sku_id.
    product_id: parentProductID > 0
      ? parentProductID
      : (Number(form.product_id ?? form.productID ?? 0) || 0),
    customer_id: Number(form.customer_id ?? form.customerID ?? 0) || 0,
    bom_version_id: Number(form.bom_version_id ?? form.bomVersionID ?? 0) || 0,
    process_route_id: Number(form.process_route_id ?? form.processRouteID ?? 0) || 0,
    operation_template_id: Number(form.operation_template_id ?? form.operationTemplateID ?? 0) || 0,
    quote_unit: String(form.quote_unit ?? form.quoteUnit ?? '').trim(),
    overrides,
  }
  const bomID = Number(form.bom_id ?? form.bomID ?? 0) || 0
  if (bomID > 0) payload.bom_id = bomID
  if (bomSpecID > 0) payload.bom_spec_id = bomSpecID
  if (bomVariantID > 0) payload.bom_variant_id = bomVariantID
  return payload
}

export function priceTablePricingRuleTrialPayload(row = {}, options = {}) {
  const quantityBasis = String(row.quantity_basis ?? row.quantityBasis ?? '').trim()
  if (quantityBasis !== 'sales_spec_count' && (row.tier_unit_compatible === false || row.tierUnitCompatible === false)) return null
  const pricingMode = normalizePriceTablePricingMode(row.pricing_mode ?? row.pricingMode)
  const pricingRuleID = [
    row.tier_pricing_rule_id,
    row.tierPricingRuleID,
    row.pricing_rule_id,
    row.pricingRuleID,
  ].map(normalizePositiveNumber).find((value) => value > 0) || 0
  const bomSpecID = [
    row.bom_spec_id,
    row.bomSpecID,
    row.default_bom_spec_id,
    row.defaultBOMSpecID,
  ].map(normalizePositiveNumber).find((value) => value > 0) || 0
  const bomVariantID = [
    row.bom_variant_id,
    row.bomVariantID,
    row.default_bom_variant_id,
    row.defaultBOMVariantID,
  ].map(normalizePositiveNumber).find((value) => value > 0) || 0
  const parentProductID = [
    row.parent_product_id,
    row.parentProductID,
    row.effective_parent_product_id,
    row.effectiveParentProductID,
  ].map(normalizePositiveNumber).find((value) => value > 0) || 0
  const productID = [
    ...(bomSpecID > 0 && parentProductID > 0 ? [parentProductID] : []),
    row.product_id,
    row.productID,
    row.productId,
    row.product_key,
    row.productKey,
  ].map(normalizePositiveNumber).find((value) => value > 0) || 0
  if (!['pricing_rule', 'tier_template'].includes(pricingMode) || pricingRuleID <= 0 || productID <= 0) return null
  const costSource = parseJSONObject(row.cost_source_snapshot ?? row.costSourceSnapshot)
  const costSourceBomSpecID = Number(costSource.bom_spec_id ?? costSource.bomSpecID ?? 0) || 0
  const costSourceBomVariantID = Number(costSource.bom_variant_id ?? costSource.bomVariantID ?? 0) || 0
  const skuID = [
    row.sku_id,
    row.skuID,
    row.skuId,
    row.product_id,
    row.productID,
    row.productId,
  ].map(normalizePositiveNumber).find((value) => value > 0) || 0
  const hasBOMSpecIdentity = bomSpecID > 0 || bomVariantID > 0 || costSourceBomSpecID > 0 || costSourceBomVariantID > 0
  const legacyChildSKU = !hasBOMSpecIdentity && parentProductID > 0 && skuID > 0 && skuID !== parentProductID
  // Legacy flat rows and BOM-spec rows may identify the specification with a
  // display label (for example “1Kg”) while the selected production BOM owns
  // the authoritative inventory unit (for example “袋”). Leave the unit empty
  // in both paths so the backend resolves it from the selected BOM spec.
  const quoteUnit = hasBOMSpecIdentity || legacyChildSKU
    ? ''
    : [
        row.price_unit,
        row.priceUnit,
        row.quote_unit,
        row.quoteUnit,
        costSource.quote_unit,
        costSource.quoteUnit,
        row.inventory_unit,
        row.inventoryUnit,
      ].map((value) => String(value || '').trim()).find(Boolean) || ''
  return buildPricingRuleTrialPayload({
    pricing_rule_id: pricingRuleID,
    product_id: productID,
    customer_id: Number(options.customerID ?? options.customer_id ?? row.customer_id ?? row.customerID ?? 0) || 0,
    bom_id: Number(row.bom_id ?? row.bomID ?? costSource.bom_id ?? costSource.bomID ?? 0) || 0,
    bom_version_id: Number(row.bom_version_id ?? row.bomVersionID ?? costSource.bom_version_id ?? costSource.bomVersionID ?? 0) || 0,
    bom_spec_id: bomSpecID || Number(costSource.bom_spec_id ?? costSource.bomSpecID ?? 0) || 0,
    bom_variant_id: Number(row.bom_variant_id ?? row.bomVariantID ?? costSource.bom_variant_id ?? costSource.bomVariantID ?? 0) || 0,
    process_route_id: Number(row.process_route_id ?? row.processRouteID ?? costSource.process_route_id ?? costSource.processRouteID ?? 0) || 0,
    operation_template_id: Number(row.operation_template_id ?? row.operationTemplateID ?? costSource.operation_template_id ?? costSource.operationTemplateID ?? 0) || 0,
    quote_unit: quoteUnit,
  })
}

export function priceTablePricingRuleTrialCacheKey(payload = {}) {
  if (!payload) return ''
  return [
    Number(payload.pricing_rule_id || 0),
    Number(payload.product_id || 0),
    Number(payload.customer_id || 0),
    Number(payload.bom_id || 0),
    Number(payload.bom_version_id || 0),
    Number(payload.bom_spec_id || 0),
    Number(payload.bom_variant_id || 0),
    Number(payload.process_route_id || 0),
    Number(payload.operation_template_id || 0),
    String(payload.quote_unit || '').trim(),
  ].join(':')
}

function priceTableTrialProductID(row = {}) {
  const bomSpecID = [
    row.bom_spec_id,
    row.bomSpecID,
    row.default_bom_spec_id,
    row.defaultBOMSpecID,
  ].map(normalizePositiveNumber).find((value) => value > 0) || 0
  const productCandidates = bomSpecID > 0
    ? [
        row.parent_product_id,
        row.parentProductID,
        row.effective_parent_product_id,
        row.effectiveParentProductID,
        row.product_id,
        row.productID,
        row.productId,
        row.product_key,
        row.productKey,
      ]
    : [
        row.product_id,
        row.productID,
        row.productId,
        row.product_key,
        row.productKey,
      ]
  return productCandidates.map(normalizePositiveNumber).find((value) => value > 0) || 0
}

export function applyPricingRuleTrialToPriceTableRow(row = {}, trial = {}) {
  const pricingMode = normalizePriceTablePricingMode(row.pricing_mode ?? row.pricingMode)
  if (!['pricing_rule', 'tier_template'].includes(pricingMode)) return row
  const trialCostStatus = String(trial.cost_status ?? trial.costStatus ?? '').trim().toLowerCase()
  if (trialCostStatus === 'incomplete' || trialCostStatus === 'error') {
    const sourceSnapshot = parseJSONObject(row.cost_source_snapshot ?? row.costSourceSnapshot)
    return {
      ...row,
      final_unit_price: 0,
      original_final_unit_price: 0,
      cost_status: trialCostStatus,
      unresolved_components: Array.isArray(trial.unresolved_components ?? trial.unresolvedComponents)
        ? (trial.unresolved_components ?? trial.unresolvedComponents)
        : [],
      partial_cost: Number(trial.partial_cost ?? trial.partialCost ?? 0) || 0,
      cost_source_snapshot: {
        ...sourceSnapshot,
        bom_version_id: Number(trial.bom_version_id ?? trial.bomVersionID ?? sourceSnapshot.bom_version_id ?? 0) || 0,
        bom_version_no: String(trial.bom_version_no ?? trial.bomVersionNo ?? sourceSnapshot.bom_version_no ?? '').trim(),
        bom_spec_id: Number(trial.bom_spec_id ?? trial.bomSpecID ?? sourceSnapshot.bom_spec_id ?? 0) || 0,
        bom_variant_id: Number(trial.bom_variant_id ?? trial.bomVariantID ?? sourceSnapshot.bom_variant_id ?? 0) || 0,
        pricing_rule_trial_cost_status: trialCostStatus,
        pricing_rule_trial_partial_cost: Number(trial.partial_cost ?? trial.partialCost ?? 0) || 0,
        pricing_rule_trial_unresolved_components: Array.isArray(trial.unresolved_components ?? trial.unresolvedComponents)
          ? (trial.unresolved_components ?? trial.unresolvedComponents)
          : [],
      },
    }
  }
  const trialPrice = normalizePositiveNumber(trial.final_unit_price ?? trial.finalUnitPrice)
  if (trialPrice <= 0) return row
  const rowRuleID = Number(row.pricing_rule_id ?? row.pricingRuleID ?? 0) || 0
  const trialRuleID = Number(trial.pricing_rule_id ?? trial.pricingRuleID ?? rowRuleID) || 0
  if (rowRuleID > 0 && trialRuleID > 0 && rowRuleID !== trialRuleID) return row
  const rowProductID = priceTableTrialProductID(row)
  const trialProductID = Number(trial.product_id ?? trial.productID ?? trial.productId ?? rowProductID) || 0
  if (rowProductID > 0 && trialProductID > 0 && rowProductID !== trialProductID) return row
  const sourceSnapshot = parseJSONObject(row.cost_source_snapshot ?? row.costSourceSnapshot)
  const rowBomSpecID = Number(row.bom_spec_id ?? row.bomSpecID ?? 0) || 0
  const rowBomVariantID = Number(row.bom_variant_id ?? row.bomVariantID ?? 0) || 0
  const sourceBomSpecID = Number(sourceSnapshot.bom_spec_id ?? sourceSnapshot.bomSpecID ?? 0) || 0
  const sourceBomVariantID = Number(sourceSnapshot.bom_variant_id ?? sourceSnapshot.bomVariantID ?? 0) || 0
  const trialBomSpecID = Number(trial.bom_spec_id ?? trial.bomSpecID ?? 0) || 0
  const trialBomVariantID = Number(trial.bom_variant_id ?? trial.bomVariantID ?? 0) || 0
  const hasBOMSpecIdentity = rowBomSpecID > 0 || rowBomVariantID > 0 || sourceBomSpecID > 0 || sourceBomVariantID > 0 || trialBomSpecID > 0 || trialBomVariantID > 0
  const rowPriceUnit = String(row.price_unit ?? row.priceUnit ?? '').trim()
  const trialQuoteUnit = String(trial.quote_unit ?? trial.quoteUnit ?? '').trim()
  const rowInventoryUnit = String(row.inventory_unit ?? row.inventoryUnit ?? '').trim()
  const trialInventoryUnit = String(trial.inventory_unit ?? trial.inventoryUnit ?? '').trim()
  const priceUnit = hasBOMSpecIdentity
    ? (trialQuoteUnit || trialInventoryUnit || rowInventoryUnit || rowPriceUnit || 'kg')
    : (rowPriceUnit || trialQuoteUnit || 'kg')
  const inventoryUnit = hasBOMSpecIdentity
    ? (trialInventoryUnit || trialQuoteUnit || rowInventoryUnit || priceUnit)
    : (rowInventoryUnit || trialInventoryUnit || priceUnit)
  const conversion = hasBOMSpecIdentity
    ? (priceUnit && inventoryUnit ? { [priceUnit]: { [inventoryUnit]: 1 } } : {})
    : priceTableInventoryConversion(row.inventory_conversion_json ?? row.inventoryConversionJSON, priceUnit, inventoryUnit)
  const trialWarnings = Array.isArray(trial.warnings) ? trial.warnings.map((item) => String(item || '').trim()).filter(Boolean) : []
  const trialBaseCostDetails = Array.isArray(trial.base_cost_details ?? trial.baseCostDetails)
    ? (trial.base_cost_details ?? trial.baseCostDetails)
        .filter((detail) => String(detail?.type || '') === 'operation')
        .map((detail) => ({
          key: String(detail?.key || '').trim(),
          name: String(detail?.name || '').trim(),
          capacity_name: String(detail?.capacity_name ?? detail?.capacityName ?? '').trim(),
          workstation_name: String(detail?.workstation_name ?? detail?.workstationName ?? '').trim(),
          capacity_selection_source: String(detail?.capacity_selection_source ?? detail?.capacitySelectionSource ?? '').trim(),
          warning: String(detail?.warning || '').trim(),
          amount: Number(detail?.amount || 0) || 0,
          unit: String(detail?.unit || '').trim(),
        }))
    : []
  const manualFinal = row.manual_adjusted === true && normalizePositiveNumber(row.final_unit_price ?? row.finalUnitPrice) > 0
    ? normalizePositiveNumber(row.final_unit_price ?? row.finalUnitPrice)
    : trialPrice
  return {
    ...row,
    price_unit: priceUnit,
    final_unit_price: manualFinal,
    original_final_unit_price: trialPrice,
    inventory_unit: inventoryUnit,
    inventory_conversion_json: conversion,
    cost_source_snapshot: {
      ...sourceSnapshot,
      bom_version_id: Number(trial.bom_version_id ?? trial.bomVersionID ?? sourceSnapshot.bom_version_id ?? 0) || 0,
      bom_version_no: String(trial.bom_version_no ?? trial.bomVersionNo ?? sourceSnapshot.bom_version_no ?? '').trim(),
      bom_spec_id: Number(trial.bom_spec_id ?? trial.bomSpecID ?? sourceSnapshot.bom_spec_id ?? 0) || 0,
      bom_variant_id: Number(trial.bom_variant_id ?? trial.bomVariantID ?? sourceSnapshot.bom_variant_id ?? 0) || 0,
      process_route_id: Number(trial.process_route_id ?? trial.processRouteID ?? sourceSnapshot.process_route_id ?? 0) || 0,
      process_route_name: String(trial.process_route_name ?? trial.processRouteName ?? sourceSnapshot.process_route_name ?? '').trim(),
      operation_template_id: Number(trial.operation_template_id ?? trial.operationTemplateID ?? sourceSnapshot.operation_template_id ?? 0) || 0,
      operation_template_name: String(trial.operation_template_name ?? trial.operationTemplateName ?? sourceSnapshot.operation_template_name ?? '').trim(),
      pricing_rule_trial_final_unit_price: trialPrice,
      pricing_rule_trial_quote_unit: trialQuoteUnit || priceUnit,
      pricing_rule_trial_inventory_unit: trialInventoryUnit || inventoryUnit,
      pricing_rule_trial_base_cost: Number(trial.base_cost ?? trial.baseCost ?? 0) || 0,
      pricing_rule_trial_warnings: trialWarnings,
      pricing_rule_trial_base_cost_details: trialBaseCostDetails,
      pricing_rule_trial_cost_status: String(trial.cost_status ?? trial.costStatus ?? 'complete').trim(),
      pricing_rule_trial_partial_cost: Number(trial.partial_cost ?? trial.partialCost ?? 0) || 0,
      pricing_rule_trial_unresolved_components: Array.isArray(trial.unresolved_components ?? trial.unresolvedComponents)
        ? (trial.unresolved_components ?? trial.unresolvedComponents)
        : [],
    },
  }
}

export function normalizePricingRuleCostSourceMode(value) {
  return 'bom_current_cost'
}

function optionalNumberFromForm(value) {
  if (value === '' || value === null || typeof value === 'undefined') return null
  const n = Number(value)
  return Number.isFinite(n) ? n : null
}

function pricingRuleCopyName(baseName, existingRules = []) {
  const base = String(baseName || '').trim() || '价格计算模板'
  const used = new Set((Array.isArray(existingRules) ? existingRules : []).map((rule) => String(rule?.name || '').trim()).filter(Boolean))
  return nextPricingRuleCopyValue(`${base} 复制`, used, ' ')
}

function pricingRuleCopyCode(baseCode, existingRules = []) {
  const base = String(baseCode || '').trim()
  if (!base) return ''
  const used = new Set((Array.isArray(existingRules) ? existingRules : []).map((rule) => String(rule?.code || '').trim()).filter(Boolean))
  return nextPricingRuleCopyValue(`${base}-COPY`, used, '-')
}

function nextPricingRuleCopyValue(firstCandidate, used, separator) {
  if (!used.has(firstCandidate)) return firstCandidate
  for (let index = 2; index < 1000; index += 1) {
    const candidate = `${firstCandidate}${separator}${index}`
    if (!used.has(candidate)) return candidate
  }
  return `${firstCandidate}${separator}${Date.now()}`
}

function pricingRuleTrialOtherCostMapFromForm(form = {}) {
  return pricingRuleTrialCostMapFromForm(form, ['other_cost_rows', 'otherCostRows'], ['other_costs', 'otherCosts'])
}

function pricingRuleTrialCostMapFromForm(form = {}, rowKeys = [], mapKeys = []) {
  const rowSource = rowKeys.map((key) => form[key]).find((value) => typeof value !== 'undefined')
  if (Array.isArray(rowSource)) {
    return rowSource.reduce((acc, row) => {
      const key = String(row?.key ?? row?.name ?? row?.cost_name ?? row?.costName ?? '').trim()
      const value = Number(row?.value ?? row?.price ?? row?.cost_price ?? row?.costPrice ?? row?.cost ?? 0)
      if (key && Number.isFinite(value)) acc[key] = value
      return acc
    }, {})
  }
  const mapSource = mapKeys.map((key) => form[key]).find((value) => typeof value !== 'undefined')
  if (!mapSource || typeof mapSource !== 'object' || Array.isArray(mapSource)) return {}
  return Object.entries(mapSource).reduce((acc, [rawKey, rawValue]) => {
    const key = String(rawKey || '').trim()
    const value = Number(rawValue)
    if (key && Number.isFinite(value)) acc[key] = value
    return acc
  }, {})
}

function pricingRuleCalculationJSONFromForm(form = {}) {
  const raw = form.calculation_json ?? form.calculationJSON ?? {}
  const base = raw && typeof raw === 'object' && !Array.isArray(raw) ? raw : {}
  const rawProfitMethod = String(base.profit_method ?? form.profit_method ?? form.profitMethod ?? '').trim().toLowerCase()
  const profitMethod = !rawProfitMethod || ['gross_margin', 'markup'].includes(rawProfitMethod) ? 'markup' : rawProfitMethod
  const normalized = {
    ...stripPricingRuleQuantityFields(base),
    yield_loss_mode: 'none',
    profit_method: profitMethod,
    tax_mode: String(form.tax_mode ?? form.taxMode ?? base.tax_mode ?? 'tax_included').trim() || 'tax_included',
    minimum_margin_rate: Number(form.minimum_margin_rate ?? form.minimumMarginRate ?? base.minimum_margin_rate ?? 0) || 0,
    trial_note: String(form.trial_note ?? form.trialNote ?? base.trial_note ?? '').trim(),
  }
  delete normalized.yield_rate
  delete normalized.yieldRate
  delete normalized.expected_yield_rate
  delete normalized.expectedYieldRate
  delete normalized.expected_loss_rate
  delete normalized.expectedLossRate
  normalized.other_costs = pricingRuleOtherCostMapFromForm(form, base)
  return stripPricingRuleQuantityFields(normalized)
}

function pricingRuleOtherCostMapFromForm(form = {}, base = {}) {
  const rowSource = form.other_cost_rows ?? form.otherCostRows
  if (Array.isArray(rowSource)) {
    return rowSource.reduce((acc, row) => {
      const key = String(row?.key ?? row?.name ?? row?.cost_name ?? row?.costName ?? '').trim()
      const value = Number(row?.value ?? row?.price ?? row?.cost_price ?? row?.costPrice ?? row?.cost ?? 0)
      if (key && Number.isFinite(value)) acc[key] = value
      return acc
    }, {})
  }
  const mapSource = form.other_costs ?? form.otherCosts ?? base.other_costs ?? base.otherCosts
  if (!mapSource || typeof mapSource !== 'object' || Array.isArray(mapSource)) return {}
  return Object.entries(mapSource).reduce((acc, [rawKey, rawValue]) => {
    const key = String(rawKey || '').trim()
    const value = Number(rawValue)
    if (key && Number.isFinite(value)) acc[key] = value
    return acc
  }, {})
}

function stripPricingRuleQuantityFields(value) {
  if (Array.isArray(value)) return value.map((item) => stripPricingRuleQuantityFields(item))
  if (!value || typeof value !== 'object') return value
  const forbidden = new Set(['min_qty', 'minQty', 'max_qty', 'maxQty', 'tier_label', 'tierLabel', 'tier_name', 'tierName', 'tiers', 'quantity_unit', 'quantityUnit', 'position', 'final_unit_price', 'finalUnitPrice', 'customer_tiers', 'customerTiers', 'cost_components', 'costComponents'])
  return Object.fromEntries(Object.entries(value)
    .filter(([key]) => !forbidden.has(String(key || '').trim()))
    .map(([key, child]) => [key, stripPricingRuleQuantityFields(child)]))
}

export function buildPriceTierTemplatePayload(form = {}) {
  const tiers = Array.isArray(form.tiers) ? form.tiers : []
  return {
    id: Number(form.id || 0),
    name: String(form.name ?? '').trim(),
    active: Boolean(form.active ?? true),
    remark: String(form.remark ?? '').trim(),
    tiers: tiers
      .map((tier, index) => ({
        label: String(tier.label ?? '').trim(),
        min_qty: Number(tier.min_qty ?? tier.minQty ?? 0) || 0,
        max_qty: tier.max_qty === '' || tier.max_qty === null || tier.max_qty === undefined
          ? null
          : Number(tier.max_qty ?? tier.maxQty ?? 0),
        // The persisted column is kept for backward reads, but new templates are
        // always interpreted as counts of the concrete sales spec selected later.
        quantity_unit: 'sales_spec_count',
        pricing_rule_id: Number(tier.pricing_rule_id ?? tier.pricingRuleID ?? 0) || 0,
        position: Number(tier.position || index + 1),
        active: Boolean(tier.active ?? true),
        remark: String(tier.remark ?? '').trim(),
      }))
      .sort((a, b) => a.position - b.position || a.min_qty - b.min_qty),
  }
}

function normalizePriceTableProductOverrideScope(row = {}) {
  const raw = String(row.scope ?? row.override_scope ?? row.overrideScope ?? '').trim().toLowerCase()
  if (raw === 'sku' || raw === 'product_sku') return 'sku'
  if (raw === 'parent_product' || raw === 'parent-product') return 'parent_product'
  return ''
}

function priceTableSKUOverride(productOverrides = [], skuID = 0) {
  const rows = Array.isArray(productOverrides) ? productOverrides : []
  const explicit = rows.find((row) => {
    if (normalizePriceTableProductOverrideScope(row) !== 'sku') return false
    return Number(row.sku_id || row.skuID || row.product_id || row.productID || 0) === skuID
  })
  if (explicit) return { row: explicit, source: 'sku' }

  // Historical snapshots only had product_id. Keep treating an unscoped row
  // as an override of the concrete SKU so old price lists retain semantics.
  const legacy = rows.find((row) => {
    if (normalizePriceTableProductOverrideScope(row)) return false
    return Number(row.product_id || row.productID || 0) === skuID
  })
  return { row: legacy, source: 'product' }
}

function priceTableParentProductOverride(productOverrides = [], parentProductID = 0) {
  if (!(parentProductID > 0)) return undefined
  const rows = Array.isArray(productOverrides) ? productOverrides : []
  return rows.find((row) => {
    if (normalizePriceTableProductOverrideScope(row) !== 'parent_product') return false
    return Number(row.parent_product_id || row.parentProductID || row.product_id || row.productID || 0) === parentProductID
  })
}

function priceTableGroupAssignmentID(row = {}) {
  return Number(row.group_item_id || row.groupItemID || 0)
}

function priceTableGroupAssignmentParentID(row = {}) {
  return Number(row.parent_group_item_id || row.parentGroupItemID || 0)
}

function priceTableGroupAssignmentName(row = {}) {
  return String(row.group_item_name ?? row.groupItemName ?? row.name ?? row.label ?? '').trim()
}

function coalescePriceTableGroupAssignments(groupAssignments = []) {
  const assignmentsByID = new Map()
  ;(Array.isArray(groupAssignments) ? groupAssignments : []).forEach((row) => {
    const groupItemID = priceTableGroupAssignmentID(row)
    if (!(groupItemID > 0)) return
    const current = assignmentsByID.get(groupItemID) || {
      group_item_id: groupItemID,
      group_item_name: '',
      parent_group_item_id: 0,
      pricing_mode: '',
      tier_template_id: 0,
      pricing_rule_id: 0,
    }
    // The price-list editor can emit both the configured parent row and an
    // empty structural row for the same category. Coalesce meaningful values
    // instead of letting row order make an empty duplicate hide configuration.
    if (!current.group_item_name) current.group_item_name = priceTableGroupAssignmentName(row)
    if (!(current.parent_group_item_id > 0)) current.parent_group_item_id = priceTableGroupAssignmentParentID(row)
    if (!current.pricing_mode) current.pricing_mode = normalizePriceTablePricingMode(row.pricing_mode ?? row.pricingMode)
    if (!(current.tier_template_id > 0)) current.tier_template_id = Number(row.tier_template_id || row.tierTemplateID || 0)
    if (!(current.pricing_rule_id > 0)) current.pricing_rule_id = Number(row.pricing_rule_id || row.pricingRuleID || 0)
    assignmentsByID.set(groupItemID, current)
  })
  return assignmentsByID
}

function priceTableGroupAssignmentChain(groupAssignments = [], groupItemID = 0, fallbackParentID = 0) {
  const assignmentsByID = coalescePriceTableGroupAssignments(groupAssignments)
  const chain = []
  const visited = new Set()
  const startingGroupItemID = Number(groupItemID || 0)
  let currentID = startingGroupItemID || Number(fallbackParentID || 0)
  let depth = startingGroupItemID > 0 ? 0 : 1
  while (currentID > 0 && !visited.has(currentID)) {
    visited.add(currentID)
    const row = assignmentsByID.get(currentID) || {
      group_item_id: currentID,
      group_item_name: '',
      parent_group_item_id: 0,
      pricing_mode: '',
      tier_template_id: 0,
      pricing_rule_id: 0,
    }
    chain.push({ row, depth })
    const rowParentID = priceTableGroupAssignmentParentID(row)
    currentID = rowParentID > 0
      ? rowParentID
      : (depth === 0 ? Number(fallbackParentID || 0) : 0)
    depth += 1
  }
  return chain
}

function priceTableInheritanceCandidate(source, value, row = {}, depth = -1) {
  const groupSource = source === 'subgroup' || source === 'parent_group'
  return {
    source,
    value,
    groupItemID: groupSource ? priceTableGroupAssignmentID(row) : 0,
    groupItemName: groupSource ? priceTableGroupAssignmentName(row) : '',
    groupDepth: groupSource ? depth : -1,
  }
}

function priceTableInheritanceCandidateIsSameLevel(left = {}, right = {}) {
  if (left.source !== right.source) return false
  if (left.source !== 'subgroup' && left.source !== 'parent_group') return true
  return Number(left.groupItemID || 0) === Number(right.groupItemID || 0) &&
    Number(left.groupDepth ?? -1) === Number(right.groupDepth ?? -1)
}

export function resolvePriceTableTemplateInheritance({
  defaults = {},
  groupAssignments = [],
  productOverrides = [],
  product = {},
} = {}) {
  const skuID = Number(product.sku_id || product.skuID || product.id || product.product_id || 0)
  const parentProductID = Number(product.parent_product_id || product.parentProductID || 0)
  const groupItemID = Number(product.group_item_id || product.groupItemID || 0)
  const skuOverrideMatch = priceTableSKUOverride(productOverrides, skuID)
  const skuOverride = skuOverrideMatch.row
  const skuSource = skuOverrideMatch.source
  const parentProductOverride = priceTableParentProductOverride(productOverrides, parentProductID)
  const fallbackParentID = Number(product.parent_group_item_id || product.parentGroupItemID || 0)
  const categoryChain = priceTableGroupAssignmentChain(groupAssignments, groupItemID, fallbackParentID)
  const categoryCandidates = categoryChain.map(({ row, depth }) => ({
    row,
    depth,
    source: depth === 0 ? 'subgroup' : 'parent_group',
  }))

  const modeCandidates = [
    priceTableInheritanceCandidate('parent_product', normalizePriceTablePricingMode(parentProductOverride?.pricing_mode ?? parentProductOverride?.pricingMode)),
    ...categoryCandidates.map(({ row, depth, source }) => priceTableInheritanceCandidate(
      source,
      normalizePriceTablePricingMode(row.pricing_mode ?? row.pricingMode),
      row,
      depth,
    )),
    priceTableInheritanceCandidate('default', normalizePriceTablePricingMode(defaults.pricing_mode ?? defaults.pricingMode)),
  ]
  const tierCandidates = [
    priceTableInheritanceCandidate('parent_product', Number(parentProductOverride?.tier_template_id || parentProductOverride?.tierTemplateID || 0)),
    ...categoryCandidates.map(({ row, depth, source }) => priceTableInheritanceCandidate(
      source,
      Number(row.tier_template_id || row.tierTemplateID || 0),
      row,
      depth,
    )),
    priceTableInheritanceCandidate('default', Number(defaults.tier_template_id || defaults.tierTemplateID || 0)),
  ]
  const pricingCandidates = [
    priceTableInheritanceCandidate('parent_product', Number(parentProductOverride?.pricing_rule_id || parentProductOverride?.pricingRuleID || 0)),
    ...categoryCandidates.map(({ row, depth, source }) => priceTableInheritanceCandidate(
      source,
      Number(row.pricing_rule_id || row.pricingRuleID || 0),
      row,
      depth,
    )),
    priceTableInheritanceCandidate('default', Number(defaults.pricing_rule_id || defaults.pricingRuleID || 0)),
  ]
  // The fixed-price mode may inherit, but the amount is authoritative only at
  // the concrete SKU level. Reusing a category/default amount across package
  // sizes would silently make unlike sales specifications share one price.
  const fixed = {
    source: skuOverride ? skuSource : 'sku',
    value: Number(skuOverride?.fixed_unit_price ?? skuOverride?.fixedUnitPrice ?? 0) || 0,
  }
  const emptyCandidate = () => priceTableInheritanceCandidate('default', 0)
  let tier = tierCandidates.find((item) => item.value > 0) || emptyCandidate()
  let pricing = pricingCandidates.find((item) => item.value > 0) || emptyCandidate()
  let mode = modeCandidates.find((item) => item.value)
  if (!mode) {
    // Legacy drafts and published snapshots may carry a template ID without
    // an explicit method. Infer that method by the same level ordering as a
    // modern configuration; do not let a farther tier ID beat a nearer
    // pricing-rule ID. A SKU amount is data for fixed mode, never the mode
    // itself.
    for (let index = 0; index < modeCandidates.length; index += 1) {
      if (Number(tierCandidates[index]?.value || 0) > 0) {
        mode = { ...tierCandidates[index], value: 'tier_template' }
        break
      }
      if (Number(pricingCandidates[index]?.value || 0) > 0) {
        mode = { ...pricingCandidates[index], value: 'pricing_rule' }
        break
      }
    }
    if (!mode) mode = priceTableInheritanceCandidate('default', '')
  }
  // Choosing a method at a nearer level is also an explicit decision about
  // that method's required template. A blank ID must remain blank so the UI
  // can report the missing template instead of silently borrowing one from a
  // farther ancestor or the price-list default.
  if (mode.value === 'tier_template') {
    tier = tierCandidates.find((item) => priceTableInheritanceCandidateIsSameLevel(mode, item)) || tier
  } else if (mode.value === 'pricing_rule') {
    pricing = pricingCandidates.find((item) => priceTableInheritanceCandidateIsSameLevel(mode, item)) || pricing
  }
  return {
    pricing_mode: mode.value,
    pricing_mode_source: mode.source,
    pricing_mode_source_group_item_id: Number(mode.groupItemID || 0),
    pricing_mode_source_group_item_name: String(mode.groupItemName || ''),
    pricing_mode_source_group_depth: Number.isInteger(mode.groupDepth) ? mode.groupDepth : -1,
    tier_template_id: tier.value,
    tier_template_source: tier.source,
    pricing_rule_id: pricing.value,
    pricing_rule_source: pricing.source,
    fixed_unit_price: fixed.value,
    fixed_unit_price_source: fixed.source,
  }
}

export function priceTablePricingResolutionWarning(resolution = {}) {
  const mode = normalizePriceTablePricingMode(resolution.pricing_mode ?? resolution.pricingMode)
  if (!mode) return '未设置计价方式'
  if (mode === 'fixed_price' && !(Number(resolution.fixed_unit_price ?? resolution.fixedUnitPrice ?? 0) > 0)) return '未填写固定价'
  if (mode === 'tier_template' && !(Number(resolution.tier_template_id ?? resolution.tierTemplateID ?? 0) > 0)) return '未选择阶梯模板'
  if (mode === 'pricing_rule' && !(Number(resolution.pricing_rule_id ?? resolution.pricingRuleID ?? 0) > 0)) return '未选择价格计算模板'
  if (!['fixed_price', 'tier_template', 'pricing_rule'].includes(mode)) return '未设置计价方式'
  return ''
}

export function buildPriceTableRowsFromTemplateResolution({
  product = {},
  tierTemplate = {},
  pricingRule = {},
  pricingRulesByID = {},
  resolution = {},
  unitPriceByTier = {},
} = {}) {
  const productID = Number(product.id || product.product_id || 0)
  const productName = String(product.name || product.product_name || '').trim()
  const unitSnapshot = productSalesUnitSnapshot(product)
  const priceUnit = unitSnapshot.price_unit
  const customerProductAliasID = Number(product.customer_product_alias_id || product.customerProductAliasID || 0)
  const tierTemplateID = Number(tierTemplate.id || 0)
  const mode = normalizePriceTablePricingMode(resolution.pricing_mode ?? resolution.pricingMode) || (tierTemplateID > 0 ? 'tier_template' : Number(pricingRule.id || resolution.pricing_rule_id || 0) > 0 ? 'pricing_rule' : Number(resolution.fixed_unit_price || 0) > 0 ? 'fixed_price' : 'tier_template')
  const modeSource = String(resolution.pricing_mode_source || resolution.pricingModeSource || 'default').trim() || 'default'
  const tierSource = String(resolution.tier_template_source || resolution.tierTemplateSource || '').trim()
  const pricingSource = String(resolution.pricing_rule_source || resolution.pricingRuleSource || modeSource).trim()
  const versionForRule = (rule) => String(rule?.code || rule?.version || rule?.name || (rule?.id ? `PR-${rule.id}` : '')).trim()
  const baseRow = {
    product_id: productID,
    product_name: productName,
    price_unit: priceUnit,
    inventory_unit: unitSnapshot.inventory_unit,
    inventory_conversion_json: unitSnapshot.inventory_conversion_json,
    min_qty: 0,
    max_qty: null,
    pricing_mode: mode,
    pricing_mode_source: modeSource,
    tier_template_id: 0,
    tier_template_source: '',
    template_tier_id: 0,
    pricing_rule_id: 0,
    pricing_rule_source: '',
    pricing_rule_version: '',
    tier_pricing_rule_id: 0,
    tier_pricing_rule_version: '',
  }
  if (customerProductAliasID > 0) baseRow.customer_product_alias_id = customerProductAliasID
  if (mode === 'pricing_rule') {
    const ruleID = Number(resolution.pricing_rule_id || pricingRule.id || 0)
    const rule = pricingRule.id ? pricingRule : pricingRulesByID[ruleID]
    return [{
      ...baseRow,
      tier_label: '基础价',
      final_unit_price: Number(unitPriceByTier.default ?? unitPriceByTier['基础价'] ?? pricingRule.final_unit_price ?? 0) || 0,
      pricing_rule_id: ruleID,
      pricing_rule_source: pricingSource,
      pricing_rule_version: versionForRule(rule),
    }]
  }
  if (mode === 'fixed_price') {
    const fixedPrice = Number(resolution.fixed_unit_price ?? resolution.fixedUnitPrice ?? 0) || 0
    return [{
      ...baseRow,
      tier_label: '固定价',
      final_unit_price: fixedPrice,
      fixed_unit_price: fixedPrice,
    }]
  }
  if (!priceTierTemplateUnitCompatibility(product, tierTemplate).compatible) return []
  return (Array.isArray(tierTemplate.tiers) ? tierTemplate.tiers : []).map((tier) => {
    const label = String(tier.label || '').trim()
    const tierPricingRuleID = Number(tier.pricing_rule_id ?? tier.pricingRuleID ?? resolution.pricing_rule_id ?? pricingRule.id ?? 0) || 0
    const tierRule = pricingRulesByID[tierPricingRuleID] || (Number(pricingRule.id || 0) === tierPricingRuleID ? pricingRule : null)
    const version = versionForRule(tierRule)
    return {
      ...baseRow,
      tier_label: label,
      min_qty: Number(tier.min_qty ?? tier.minQty ?? 0) || 0,
      max_qty: tier.max_qty === null || tier.max_qty === undefined || tier.max_qty === ''
        ? null
        : Number(tier.max_qty ?? tier.maxQty ?? 0),
      final_unit_price: Number(unitPriceByTier[label] ?? tier.final_unit_price ?? tier.finalUnitPrice ?? 0) || 0,
      tier_template_id: tierTemplateID,
      tier_template_name: String(tierTemplate.name || '').trim(),
      tier_template_source: tierSource,
      template_tier_id: Number(tier.id ?? tier.template_tier_id ?? tier.templateTierID ?? 0) || 0,
      quantity_basis: 'sales_spec_count',
      tier_quantity_unit: priceUnit,
      pricing_rule_id: tierPricingRuleID,
      pricing_rule_source: tierSource || pricingSource,
      pricing_rule_version: version,
      tier_pricing_rule_id: tierPricingRuleID,
      tier_pricing_rule_version: version,
    }
  })
}

export function priceTierTemplateUnitCompatibility(product = {}, tierTemplate = {}) {
  const productUnit = productCurrentSalesSpecUnit(product)
  const normalizedProductUnit = normalizePriceTierUnitIdentity(productUnit)
  const tiers = (Array.isArray(tierTemplate.tiers) ? tierTemplate.tiers : []).filter((tier) => tier?.active !== false)
  const templateUnits = uniqueInOrder(tiers
    .map((tier) => normalizePriceTierUnitIdentity(tier?.quantity_unit ?? tier?.quantityUnit))
    .filter(Boolean))
  const compatible = Boolean(normalizedProductUnit) && tiers.length > 0
  let message = ''
  if (!normalizedProductUnit) {
    message = '阶梯模板不可用：商品缺少有效默认销售规格'
  } else if (!tiers.length) {
    message = '阶梯模板不可用：阶梯模板缺少有效数量档位'
  }
  return {
    compatible,
    product_unit: productUnit,
    template_units: templateUnits,
    message,
  }
}

export function priceTierTemplateRowKey({ productID = '', templateID = 0, tierID = 0, product = {}, tier = {}, suffix = '' } = {}) {
  const productUnit = normalizePriceTierUnitIdentity(productCurrentSalesSpecUnit(product)) || '-'
  const base = `${String(productID || '')}:tier-template:${Number(templateID || 0)}:${String(tierID || '')}:${productUnit}`
  return suffix ? `${base}:${String(suffix)}` : base
}

export function productCurrentSalesSpecUnit(product = {}) {
  const candidates = [
    product.default_sales_unit,
    product.defaultSalesUnit,
    product.derived_sales_unit,
    product.derivedSalesUnit,
    product.sales_unit,
    product.salesUnit,
    product.quote_unit,
    product.quoteUnit,
    product.order_unit,
    product.orderUnit,
    product.spec_label,
    product.specLabel,
    product.price_unit,
    product.priceUnit,
    product.price_unit_snapshot,
    product.priceUnitSnapshot,
  ]
  return candidates.map((value) => String(value ?? '').trim()).find(Boolean) || ''
}

function normalizePriceTierUnitIdentity(value = '') {
  const raw = String(value || '').trim()
  if (!raw) return ''
  const compact = raw.toLowerCase().replace(/\s+/g, '')
  const packageMatch = compact.match(/^(盒|袋|条)(?:[（(]|$)/)
  if (packageMatch) return packageMatch[1]
  const massMatch = compact.match(/^(?:\d+(?:\.\d+)?)?(kg|kgs|公斤|千克|g|克|lb|lbs|磅)(?:袋装)?$/i)
  if (massMatch) {
    const unit = String(massMatch[1] || '').toLowerCase()
    if (unit === 'kg' || unit === 'kgs' || unit === '公斤' || unit === '千克') return 'kg'
    if (unit === 'lb' || unit === 'lbs' || unit === '磅') return 'lb'
    if (unit === 'g' || unit === '克') return 'g'
  }
  return compact
}

function productSalesUnitSnapshot(product = {}) {
  const inventoryUnit = normalizeUnitText(product.inventory_unit ?? product.inventoryUnit, 'kg')
  const priceUnit = normalizeUnitText(
    productCurrentSalesSpecUnit(product),
    inventoryUnit,
  )
  const conversion = productSalesUnitConversion(product, priceUnit, inventoryUnit)
  return {
    price_unit: priceUnit,
    inventory_unit: inventoryUnit,
    inventory_conversion_json: conversion,
  }
}

function productSalesUnitConversion(product = {}, priceUnit = '', inventoryUnit = '') {
  const direct = parseJSONObject(product.unit_conversion_json ?? product.unitConversionJSON)
  const rule = parseJSONObject(product.unit_rule_override_json ?? product.unitRuleOverrideJSON)
  const conversion = Object.keys(direct).length
    ? direct
    : parseJSONObject(rule.unit_conversion_json ?? rule.conversion_json ?? {})
  const normalizedPriceUnit = normalizeOptionalUnitText(priceUnit)
  const normalizedInventoryUnit = normalizeOptionalUnitText(inventoryUnit)
  if (normalizedPriceUnit && normalizedInventoryUnit) {
    const rawTargets = conversion[normalizedPriceUnit]
    const directFactor = normalizePositiveNumber(rawTargets)
    if (directFactor > 0) return { [normalizedPriceUnit]: { [normalizedInventoryUnit]: trimDecimal(directFactor) } }
    const targets = parseJSONObject(rawTargets)
    const factor = normalizePositiveNumber(targets[normalizedInventoryUnit])
    if (factor > 0) return { [normalizedPriceUnit]: { [normalizedInventoryUnit]: trimDecimal(factor) } }
    if (normalizedPriceUnit === normalizedInventoryUnit) return { [normalizedPriceUnit]: { [normalizedInventoryUnit]: 1 } }
  }
  return conversion
}

function normalizePriceTablePricingMode(value) {
  const raw = String(value ?? '').trim()
  if (raw === 'tier_template' || raw === 'inherit_gradient_template') return 'tier_template'
  if (raw === 'pricing_rule' || raw === 'cost_plus') return 'pricing_rule'
  if (raw === 'fixed_price' || raw === 'fixed_unit_price') return 'fixed_price'
  return ''
}

export function customerAliasEffectiveDisplayName(alias = {}) {
  const renamed = String(alias.brand_name ?? alias.brandName ?? '').trim()
  if (renamed) return renamed
  return String(alias.display_name ?? alias.displayName ?? '').trim()
}

export function productCodeLabel(row = {}) {
  const explicit = String(row.product_code ?? row.productCode ?? row.code ?? '').trim()
  if (explicit) return explicit
  const id = Number(row.id ?? row.product_id ?? row.productID ?? 0)
  return id > 0 ? `SKU-${String(id).padStart(6, '0')}` : ''
}

export function activeProductionBomOptions(rows = []) {
  return (Array.isArray(rows) ? rows : [])
    .filter((row) => Number(row?.id || 0) > 0)
    .filter((row) => String(row?.status || 'active').trim().toLowerCase() === 'active')
    .map((row) => ({
      ...row,
      id: Number(row.id || 0),
      code: String(row.code || '').trim(),
      name: String(row.name || '').trim(),
      latest_version_no: String(row.latest_version_no || row.production_bom_version_no || '').trim(),
      latest_version_status: String(row.latest_version_status || '').trim(),
      group_name: String(row.group_name || '').trim(),
    }))
    .sort((a, b) => String(a.code || '').localeCompare(String(b.code || '')) || String(a.name || '').localeCompare(String(b.name || '')))
}

export function productionBomOptionLabel(row = {}) {
  const code = String(row.code || '').trim()
  const name = String(row.name || '').trim()
  const version = String(row.latest_version_no || row.production_bom_version_no || '').trim()
  return [code, name].filter(Boolean).join(' ') + (version ? ` / ${version}` : '')
}

export function buildCustomerProductAliasBatchPayload(form = {}) {
  const ids = []
  const seen = new Set()
  for (const raw of form.product_ids || form.productIDs || []) {
    const id = Number(raw || 0)
    if (!id || seen.has(id)) continue
    seen.add(id)
    ids.push(id)
  }
  return {
    customer_id: Number(form.customer_id || form.customerID || 0),
    product_ids: ids,
    include_in_price_list: Boolean(form.include_in_price_list ?? form.includeInPriceList ?? true),
    brand_name: String(form.brand_name ?? form.brandName ?? '').trim(),
    display_category_id: Number(form.display_category_id || form.displayCategoryID || 0),
  }
}

export function buildClassificationTemplateUsagePayload(form = {}) {
  const payload = {
    classification_template_id: Number(form.classification_template_id || form.classificationTemplateID || form.template_id || form.templateID || 0),
    sort_order: Number(form.sort_order || form.sortOrder || 100),
  }
  const customerID = Number(form.customer_id || form.customerID || 0)
  if (customerID > 0) payload.customer_id = customerID
  return payload
}

export function classificationTemplateTabs(templates = [], usages = [], options = {}) {
  const activeTemplateByID = new Map((templates || [])
    .filter((template) => template?.active !== false)
    .map((template) => [Number(template.id || 0), template]))
  const seen = new Set()
  const tabs = [{
    key: 'all',
    id: 0,
    template_id: 0,
    label: String(options.allLabel || '全部商品'),
    all: true,
  }]
  if (options.unclassifiedLabel) {
    tabs.push({
      key: 'unclassified',
      id: -1,
      template_id: 0,
      label: String(options.unclassifiedLabel),
      unclassified: true,
      all: false,
      sort_order: -1,
    })
  }
  for (const usage of usages || []) {
    if (usage?.active === false) continue
    const templateID = Number(usage.classification_template_id || usage.template_id || 0)
    const template = activeTemplateByID.get(templateID)
    if (!template || seen.has(templateID)) continue
    seen.add(templateID)
    tabs.push({
      key: `template-${templateID}`,
      id: templateID,
      template_id: templateID,
      label: template.name || `分类模板 #${templateID}`,
      sort_order: Number(usage.sort_order || template.sort_order || 100),
      template,
      all: false,
    })
  }
  const fixedTabs = tabs.filter((tab) => tab.all || tab.unclassified)
  return fixedTabs.concat(tabs.filter((tab) => !tab.all && !tab.unclassified)
    .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || String(a.label || '').localeCompare(String(b.label || ''))))
}

export function groupRowsByClassificationCategory(rows = [], template = {}, options = {}) {
  const idKey = options.idKey || 'id'
  const assignmentKey = options.assignmentKey || 'product_id'
  const assignmentsKey = options.assignmentsKey || 'product_assignments'
  const assignmentsByObjectID = new Map((template?.[assignmentsKey] || [])
    .filter((assignment) => Number(assignment.template_id || template.id || 0) === Number(template.id || 0))
    .map((assignment) => [Number(assignment[assignmentKey] || 0), assignment]))
  const categories = (template?.categories || [])
    .filter((category) => category?.active !== false)
    .slice()
    .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || Number(a.id || 0) - Number(b.id || 0))
  const groups = categories.map((category) => ({
    key: `category-${Number(category.id || 0)}`,
    id: Number(category.id || 0),
    label: category.name || '未命名分类',
    rows: [],
    category,
  }))
  const groupByCategoryID = new Map(groups.map((group) => [group.id, group]))
  const uncategorized = { key: 'uncategorized', id: 0, label: '未分类', rows: [], category: null }
  const onlyAssigned = Boolean(options.onlyAssigned)
  for (const row of rows || []) {
    const objectID = Number(row?.[idKey] || 0)
    const assignment = assignmentsByObjectID.get(objectID)
    if (onlyAssigned && !assignment) continue
    const categoryID = Number(assignment?.category_id || 0)
    const target = groupByCategoryID.get(categoryID) || uncategorized
    target.rows.push({
      ...row,
      classification_category_id: target.id,
      classification_sort_order: Number(assignment?.sort_order || row?.sort_order || 100),
    })
  }
  for (const group of [...groups, uncategorized]) {
    group.rows.sort((a, b) => Number(a.classification_sort_order || 0) - Number(b.classification_sort_order || 0) || Number(a.id || 0) - Number(b.id || 0))
  }
  return [...groups, uncategorized]
}

export function classificationAssignmentForRow(row = {}, templates = [], options = {}) {
  const assignmentType = String(options.assignmentType || 'product')
  const rowID = Number(row?.[options.idKey || 'id'] || row?.product_id || row?.alias_id || 0)
  if (!rowID) return null
  const assignmentsKey = options.assignmentsKey || (assignmentType === 'alias' ? 'customer_alias_assignments' : 'product_assignments')
  const assignmentIDKey = options.assignmentKey || (assignmentType === 'alias' ? 'alias_id' : 'product_id')
  for (const template of templates || []) {
    const templateID = Number(template?.id || template?.template_id || 0)
    if (!templateID || template?.active === false) continue
    const assignment = (template?.[assignmentsKey] || []).find((item) => Number(item?.[assignmentIDKey] || 0) === rowID)
    if (!assignment) continue
    const categoryID = Number(assignment.category_id || 0)
    const category = (template.categories || []).find((item) => Number(item.id || 0) === categoryID)
    return { assignment, template, category }
  }
  return null
}

export function classificationAssignmentLabel(row = {}, templates = [], options = {}) {
  const found = classificationAssignmentForRow(row, templates, options)
  if (!found) return '未分类'
  const templateName = found.template?.name || `分类模板 #${Number(found.template?.id || 0)}`
  const categoryName = found.category?.name || '未分类'
  return `${templateName} / ${categoryName}`
}

export function productCategoryAssignmentLabel(row = {}, categoryTree = [], fallback = '未分类') {
  const primaryName = String(row?.primary_name || '').trim()
  const secondaryName = String(row?.secondary_name || '').trim()
  if (primaryName && secondaryName) return `${primaryName} / ${secondaryName}`
  if (primaryName) return primaryName

  const categoryID = Number(row?.product_category_id || row?.productCategoryID || row?.category_id || 0)
  if (categoryID > 0) {
    const meta = categoryPathMetaByID(categoryTree).get(categoryID)
    const metaPrimaryName = String(meta?.primary_name || '').trim()
    const metaSecondaryName = String(meta?.secondary_name || '').trim()
    if (metaPrimaryName && metaSecondaryName) return `${metaPrimaryName} / ${metaSecondaryName}`
    if (metaPrimaryName) return metaPrimaryName
  }

  return fallback
}

export function classificationAssignmentConflict(row = {}, targetTemplateID = 0, templates = [], options = {}) {
  return null
}

export function classificationTemplateUnitPriceWarnings(input = {}) {
  const productConfigTemplate = input.productConfigTemplate || input.product_config_template || {}
  const classificationTemplate = input.classificationTemplate || input.classification_template || {}
  const classificationCategory = input.classificationCategory || input.classification_category || {}
  const effectiveProductConfigID = Number(classificationCategory.product_config_template_id || classificationTemplate.product_config_template_id || 0)
  const productConfigID = Number(productConfigTemplate.id || productConfigTemplate.product_config_template_id || 0)
  const warnings = []
  if (productConfigID > 0 && effectiveProductConfigID > 0 && productConfigID !== effectiveProductConfigID) {
    warnings.push('商品已选择商品配置模板，将覆盖所属分类引用的商品配置模板')
  }
  return warnings
}

export function customerProductAliasRowsForCustomer(rows = [], customerID = 0, options = {}) {
  const selectedCustomerID = Number(customerID || 0)
  const active = String(options.active || (options.includeInactive ? 'all' : 'active')).trim()
  const query = String(options.query || '').trim().toLowerCase()
  return (rows || [])
    .filter((row) => {
      if (selectedCustomerID > 0 && Number(row?.customer_id || 0) !== selectedCustomerID) return false
      if (active === 'active' && row?.active === false) return false
      if (active === 'inactive' && row?.active !== false) return false
      if (query) {
        const haystack = [
          row?.display_name,
          row?.customer_item_code,
          row?.brand_name,
          row?.product_code,
          row?.product_name,
        ].join(' ').toLowerCase()
        if (!haystack.includes(query)) return false
      }
      return true
    })
    .slice()
    .sort((a, b) => Number(a?.sort_order || 0) - Number(b?.sort_order || 0) || Number(a?.id || 0) - Number(b?.id || 0))
}

export function industryFieldOptionsTextFromJSON(raw = '[]') {
  if (Array.isArray(raw)) return raw.map((value) => String(value || '').trim()).filter(Boolean).join(', ')
  try {
    const parsed = JSON.parse(String(raw || '[]'))
    if (Array.isArray(parsed)) return parsed.map((value) => String(value || '').trim()).filter(Boolean).join(', ')
  } catch (_) {
    return String(raw || '').trim()
  }
  return ''
}

export function industryFieldOptionsJSONFromText(raw = '') {
  const seen = new Set()
  const values = String(raw || '')
    .split(/[,，]/)
    .map((value) => value.trim())
    .filter((value) => {
      if (!value || seen.has(value)) return false
      seen.add(value)
      return true
    })
  return JSON.stringify(values)
}

export function industryFieldSummary(fields = []) {
  const parts = (fields || [])
    .filter((field) => String(field?.field_key || field?.label || '').trim())
    .map((field) => {
      const label = String(field?.label || field?.field_key || '').trim()
      const value = String(field?.value_text ?? field?.value ?? '').trim()
      return value ? `${label}：${value}` : ''
    })
    .filter(Boolean)
  return parts.length ? parts.join('；') : '-'
}

export function buildCustomerProductAliasIndustryFieldPayload(form = {}) {
  const fields = Array.isArray(form?.fields) ? form.fields : []
  const seen = new Set()
  return {
    fields: fields
      .map((field) => ({
        field_key: String(field?.field_key || '').trim(),
        value_text: String(field?.value_text ?? field?.value ?? '').trim(),
      }))
      .filter((field) => {
        const key = field.field_key.toLowerCase()
        if (!key || seen.has(key)) return false
        seen.add(key)
        return true
      }),
  }
}

export function productCreationActionOptions(context = {}) {
  return [{
    key: 'product_record',
    label: '创建新商品档案',
    description: '配方、包装、生产方式、库存对象或成本口径变化时使用，后续维护独立生产 BOM。',
  }]
}

export function customerProductAliasMigrationCandidateSummary(row = {}) {
  const product = [row.product_code, row.product_name].map((value) => String(value || '').trim()).filter(Boolean).join(' ')
  const base = [row.base_product_code, row.base_product_name].map((value) => String(value || '').trim()).filter(Boolean).join(' ')
  const reason = String(row.suggested_reason || '').trim()
  if (row.suggested_action === 'convert_to_customer_product_alias') {
    return `建议转为客户商品：${product || '当前客户商品'} → 绑定 ${base || '来源商品档案'}${reason ? `；${reason}` : ''}`
  }
  return `建议保留商品档案：${product || '当前客户商品'}${reason ? `；${reason}` : ''}`
}

export function buildCustomerPublicUsagePayload(customerID, options = {}) {
  return {
    customer_id: Number(customerID || 0),
    use_public_sku: Boolean(options.use_public_sku ?? options.usePublicSku),
    use_public_categories: Boolean(options.use_public_categories ?? options.usePublicCategories),
    use_public_gradient_templates: Boolean(options.use_public_gradient_templates ?? options.usePublicGradientTemplates),
  }
}

export function productBelongsToSkuContext(product = {}, context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  const productCustomerID = Number(product.customer_id || 0)
  if (!customerID) return true
  if (productCustomerID === customerID) return true
  if (productCustomerID !== 0) return false
  return (context.references || context.productCustomerReferences || []).some((reference) => (
    reference?.active !== false
    && Number(reference?.product_id || 0) === Number(product?.id || 0)
    && Number(reference?.customer_id || 0) === customerID
  ))
}

export function categoryBelongsToSkuContext(category = {}, context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  const categoryCustomerID = Number(category.customer_id || 0)
  if (!customerID) return categoryCustomerID === 0
  if (categoryCustomerID === customerID) return true
  if (categoryCustomerID !== 0 || !Boolean(context.usePublicCategories || context.use_public_categories)) return false
  return !hasCustomerDerivedCategory(category, context.customerCategories)
}

export function nextSkuContextCustomerID(currentCustomerID = 0, { workspaceMode = '', customerContextID = 0, customerContextId = 0 } = {}) {
  if (String(workspaceMode || '').trim() !== 'customer') return 0
  const lockedCustomerID = Number(customerContextID || customerContextId || 0)
  if (lockedCustomerID > 0) return lockedCustomerID
  return Number(currentCustomerID || 0)
}

export function gradientTemplateBelongsToSkuContext(template = {}, context = {}) {
	const customerID = Number(context.customerID || context.customer_id || 0)
	const templateCustomerID = Number(template.customer_id || 0)
	if (!customerID) return templateCustomerID === 0
  if (templateCustomerID === customerID) return true
  if (templateCustomerID !== 0 || !Boolean(context.usePublicGradientTemplates || context.use_public_gradient_templates)) return false
	return !hasCustomerDerivedTemplate(template, context.customerTemplates)
}

export function productConfigTemplateBelongsToSkuContext(template = {}, context = {}) {
	const customerID = Number(context.customerID || context.customer_id || 0)
	const templateCustomerID = Number(template.customer_id || 0)
	if (!customerID) return templateCustomerID === 0
	if (templateCustomerID === customerID) return true
	if (templateCustomerID !== 0) return false
	if (context.usePublicProductConfigTemplates === false || context.use_public_product_config_templates === false) return false
	return !hasCustomerDerivedTemplate(template, context.customerTemplates)
}

export function categoryDisplayState(category = {}, context = {}) {
  if (Number(category.customer_id || 0) === 0 && Number(context.customerID || context.customer_id || 0) > 0) {
    return { label: '公共模板', tone: 'template' }
  }
  if (Number(category.source_category_id || 0) > 0 || category.template_state === 'derived_from_public') {
    return { label: '来自公共模板', tone: 'derived' }
  }
  return { label: '客户自有', tone: 'owned' }
}

export function productDisplayState(product = {}, context = {}) {
  if (Number(product.customer_id || 0) === 0 && Number(context.customerID || context.customer_id || 0) > 0) {
    return { label: '公共模板', tone: 'template' }
  }
  if (Number(product.base_product_id || 0) > 0 && product.custom_type === 'public_sku_alias') {
    return { label: '来自公共 SKU', tone: 'derived' }
  }
  return { label: '客户自有', tone: 'owned' }
}

export function buildAssignCategoryPayload({ product = {}, category = {}, customerID = 0, position = 0 } = {}) {
  const scopedCustomerID = Number(customerID || 0)
  const productCustomerID = Number(product.customer_id || 0)
  const categoryCustomerID = Number(category.customer_id || 0)
  const payload = {
    category_id: Number(category.id || 0),
    position: Number(position || 0),
  }
  if (scopedCustomerID > 0) {
    payload.customer_id = scopedCustomerID
    payload.derive_public_category = Number(category.id || 0) > 0 && categoryCustomerID === 0
    payload.derive_public_product = productCustomerID === 0
  }
  return payload
}

export function buildProductCategoryConfigPayload(category = {}) {
	return {
		id: Number(category.id || 0),
		customer_id: Number(category.customer_id || 0),
		name: String(category.name || '').trim(),
		parent_id: Number(category.parent_id || 0),
		position: Number(category.position || 0),
		product_config_template_id: Number(category.product_config_template_id || 0),
		gradient_template_id: Number(category.gradient_template_id || 0),
		operation_template_id: Number(category.operation_template_id || 0),
    price_list_rule_json: hasStructuredPriceRuleFields(category)
      ? priceListRuleJSONFromForm(category)
      : normalizeJSONString(category.price_list_rule_json),
    inventory_unit: normalizeUnitText(category.inventory_unit, 'kg'),
    quote_unit: normalizeUnitText(category.quote_unit, normalizeUnitText(category.inventory_unit, 'kg')),
    order_unit: normalizeUnitText(category.order_unit, normalizeUnitText(category.quote_unit, normalizeUnitText(category.inventory_unit, 'kg'))),
    unit_conversion_json: Array.isArray(category.unit_conversion_rows)
      ? unitConversionJSONFromRows(category.unit_conversion_rows)
      : normalizeJSONString(category.unit_conversion_json),
    integer_unit: Boolean(category.integer_unit),
	}
}

export function buildProductConfigTemplatePayload(form = {}) {
	return {
		id: Number(form.id || 0),
		customer_id: Number(form.customer_id || 0),
		name: String(form.name || '').trim(),
		gradient_template_id: productConfigTemplateNeedsGradientTemplate(form) ? Number(form.gradient_template_id || 0) : 0,
		operation_template_id: Number(form.operation_template_id || 0),
		unit_template_id: Number(form.unit_template_id || 0),
		price_list_rule_json: hasStructuredPriceRuleFields(form)
			? priceListRuleJSONFromForm(form)
			: normalizeJSONString(form.price_list_rule_json),
		special_attrs_schema_json: Array.isArray(form.special_attrs_schema_rows)
			? specialAttrSchemaJSONFromRows(form.special_attrs_schema_rows)
			: normalizeJSONArrayString(form.special_attrs_schema_json),
		active: form.active === false ? false : true,
	}
}

export function productConfigTemplateNeedsGradientTemplate(form = {}) {
  const ruleForm = hasStructuredPriceRuleFields(form) ? form : priceListRuleFormFromJSON(form.price_list_rule_json || '{}')
  return optionValue(ruleForm.price_rule_pricing_mode, priceListRulePricingModeOptions, 'inherit_gradient_template') === 'inherit_gradient_template'
}

export function buildProductUnitDefinitionPayload(form = {}) {
  return {
    code: String(form.code || '').trim(),
    name: String(form.name || '').trim(),
    unit_type: String(form.unit_type || '').trim() || 'other',
    allow_decimal: Boolean(form.allow_decimal),
    active: form.active === false ? false : true,
  }
}

export function buildProductUnitTemplatePayload(form = {}) {
  if (Array.isArray(form.sales_spec_rows) || Array.isArray(form.sales_specs)) {
    const inventoryUnit = normalizeUnitText(form.inventory_unit, 'kg')
    const salesSpecs = normalizeSalesSpecRows(form.sales_spec_rows ?? form.sales_specs, inventoryUnit, {
      forceActive: true,
      defaultSpecKey: form.default_spec_key ?? form.defaultSpecKey,
    })
    const defaultSpec = salesSpecs.find((row) => row.default) || salesSpecs.find((row) => row.active !== false) || salesSpecs[0] || null
    const defaultSalesUnit = normalizeOptionalUnitText(
      defaultSpec?.sales_unit ?? form.default_sales_unit ?? form.defaultSalesUnit ?? form.sales_unit ?? form.order_unit ?? form.quote_unit,
    ) || defaultSpec?.sales_unit || ''
    return {
      id: Number(form.id || 0),
      name: String(form.name || '').trim(),
      inventory_unit: inventoryUnit,
      default_sales_unit: defaultSalesUnit,
      sales_unit: defaultSalesUnit,
      sales_units: uniqueInOrder(salesSpecs.map((row) => row.sales_unit)),
      quote_unit: defaultSalesUnit,
      order_unit: defaultSalesUnit,
      unit_conversion_json: '{}',
      sales_specs: salesSpecs,
      active: form.active === false ? false : true,
    }
  }
  const inventoryUnit = normalizeUnitText(form.inventory_unit, 'kg')
  const defaultSalesUnit = normalizeUnitText(
    form.default_sales_unit ?? form.defaultSalesUnit ?? form.sales_unit ?? form.order_unit ?? form.quote_unit,
    inventoryUnit,
  )
  const conversion = unitTemplateConversionPayload(form, inventoryUnit, defaultSalesUnit)
  return {
    id: Number(form.id || 0),
    name: String(form.name || '').trim(),
    inventory_unit: inventoryUnit,
    sales_unit: defaultSalesUnit,
    default_sales_unit: defaultSalesUnit,
    sales_units: conversion.sales_units,
    quote_unit: defaultSalesUnit,
    order_unit: defaultSalesUnit,
    unit_conversion_json: conversion.unit_conversion_json,
    integer_unit: Boolean(form.integer_unit),
    active: form.active === false ? false : true,
  }
}

export function salesSpecRowsFromTemplate(template = {}, inventoryUnit = '') {
  const rawRows = Array.isArray(template.sales_specs ?? template.salesSpecs ?? template.sales_spec_rows)
    ? (template.sales_specs ?? template.salesSpecs ?? template.sales_spec_rows)
    : parseJSONArray(template.sales_specs ?? template.salesSpecs ?? template.sales_spec_rows)
  const rows = normalizeSalesSpecRows(rawRows, inventoryUnit || template.inventory_unit || template.inventoryUnit || '', {
    defaultSpecKey: template.default_spec_key ?? template.defaultSpecKey,
  })
  return rows.map((row) => ({
    ...row,
    ...salesSpecDerivedMeta(rawRows.find((source) => String(source?.spec_key ?? source?.specKey ?? '').trim() === row.spec_key) || rawRows[rows.indexOf(row)] || {}),
    derived_spec_status: String((rawRows.find((source) => String(source?.spec_key ?? source?.specKey ?? '').trim() === row.spec_key) || rawRows[rows.indexOf(row)] || {})?.derived_spec_status ?? (rawRows.find((source) => String(source?.spec_key ?? source?.specKey ?? '').trim() === row.spec_key) || rawRows[rows.indexOf(row)] || {})?.derivedSpecStatus ?? '').trim() || (row.active === false ? 'template_disabled' : 'active'),
  }))
}

function salesSpecDerivedMeta(source = {}) {
  const derivedSkuID = Number(source?.derived_sku_id ?? source?.derivedSKUID ?? 0)
  const derivedSkuCode = String(source?.derived_sku_code ?? source?.derivedSKUCode ?? '').trim()
  const out = {}
  if (derivedSkuID > 0) out.derived_sku_id = derivedSkuID
  out.derived_sku_code = derivedSkuCode
  return out
}

const salesSpecWeightUnitGrams = {
  g: 1,
  kg: 1000,
  lb: 453.59237,
  lbs: 453.59237,
  '磅': 453.59237,
}

export function salesSpecConversionLabel(row = {}, inventoryUnit = '') {
  const salesUnit = String(row?.spec_name ?? row?.specName ?? row?.derived_spec_name ?? row?.derivedSpecName ?? '').trim()
    || normalizeOptionalUnitText(row?.sales_unit ?? row?.salesUnit ?? row?.derived_sales_unit ?? row?.derivedSalesUnit ?? row?.unit)
  const netContentQty = normalizePositiveNumber(row?.net_content_qty ?? row?.netContentQty ?? row?.content_qty)
  const netContentUnit = normalizeOptionalUnitText(row?.net_content_unit ?? row?.netContentUnit ?? row?.content_unit)
  const targetUnit = normalizeOptionalUnitText(inventoryUnit) || netContentUnit
  if (!salesUnit || !netContentQty || !netContentUnit) return '换算待补：请填写库存数量'
  if (!targetUnit || targetUnit === netContentUnit) return `1 ${salesUnit} = ${trimDecimal(netContentQty)} ${netContentUnit}`
  const sourceGram = salesSpecWeightFactor(netContentUnit)
  const targetGram = salesSpecWeightFactor(targetUnit)
  if (sourceGram > 0 && targetGram > 0) {
    return `1 ${salesUnit} = ${trimDecimal((netContentQty * sourceGram) / targetGram)} ${targetUnit}`
  }
  return `1 ${salesUnit} = ${trimDecimal(netContentQty)} ${netContentUnit}（库存单位 ${targetUnit}，无法自动换算）`
}

function salesSpecWeightFactor(unit = '') {
  const text = String(unit || '').trim()
  return salesSpecWeightUnitGrams[text] || salesSpecWeightUnitGrams[text.toLowerCase()] || 0
}

function convertSalesSpecQuantity(value, fromUnit = '', toUnit = '') {
  const qty = Number(value)
  if (!Number.isFinite(qty) || qty <= 0) return 0
  const sourceGram = salesSpecWeightFactor(fromUnit)
  const targetGram = salesSpecWeightFactor(toUnit)
  if (!(sourceGram > 0 && targetGram > 0)) return 0
  return trimDecimal((qty * sourceGram) / targetGram)
}

function normalizeSalesSpecRows(rows = [], inventoryUnit = '', options = {}) {
  const sourceRows = Array.isArray(rows) ? rows : parseJSONArray(rows)
  const targetInventoryUnit = normalizeOptionalUnitText(inventoryUnit)
  const defaultSpecKey = String(options.defaultSpecKey || '').trim()
  const normalized = []
  let defaultIndex = -1
  for (const source of sourceRows) {
    const specName = String(source?.spec_name ?? source?.specName ?? source?.name ?? '').trim()
    if (!specName) continue
    const salesUnit = specName
    const specKey = String(source?.spec_key ?? source?.specKey ?? '').trim() || generatedSalesSpecKey(specName, salesUnit, normalized.length)
    const netContentQty = Number(source?.net_content_qty ?? source?.netContentQty ?? source?.content_qty ?? 0)
    const sourceNetContentUnit = normalizeOptionalUnitText(source?.net_content_unit ?? source?.netContentUnit ?? source?.content_unit) || targetInventoryUnit
    let normalizedQty = Number.isFinite(netContentQty) ? trimDecimal(netContentQty) : 0
    let normalizedUnit = targetInventoryUnit || sourceNetContentUnit
    if (normalizedQty > 0 && sourceNetContentUnit && targetInventoryUnit && sourceNetContentUnit !== targetInventoryUnit) {
      const convertedQty = convertSalesSpecQuantity(normalizedQty, sourceNetContentUnit, targetInventoryUnit)
      if (convertedQty > 0) {
        normalizedQty = convertedQty
        normalizedUnit = targetInventoryUnit
      } else {
        normalizedUnit = sourceNetContentUnit
      }
    }
    const isSourceDefault = Boolean(source?.default ?? source?.is_default ?? source?.isDefault)
    const row = {
      spec_key: specKey,
      spec_name: specName,
      sales_unit: salesUnit,
      net_content_qty: normalizedQty,
      net_content_unit: normalizedUnit,
      default: isSourceDefault,
      active: options.forceActive ? true : (source?.active === false ? false : true),
    }
    if (defaultSpecKey && specKey === defaultSpecKey) defaultIndex = normalized.length
    if (defaultIndex < 0 && isSourceDefault) defaultIndex = normalized.length
    normalized.push(row)
  }
  if (normalized.length) {
    if (defaultIndex < 0) defaultIndex = normalized.findIndex((row) => row.active !== false)
    if (defaultIndex < 0) defaultIndex = 0
  }
  normalized.forEach((row, index) => { row.default = index === defaultIndex })
  return normalized
}

function generatedSalesSpecKey(specName = '', salesUnit = '', index = 0) {
  const raw = `${specName}-${salesUnit}-${index + 1}`.toLowerCase()
  const ascii = raw.replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  return ascii || `spec-${index + 1}`
}

function unitTemplateConversionPayload(form = {}, inventoryUnit = 'kg', defaultSalesUnit = '') {
  const graph = {}
  const conversionKeys = []
  if (Array.isArray(form.unit_conversion_rows)) {
    for (const row of form.unit_conversion_rows || []) {
      const fromQty = normalizePositiveNumber(row?.from_qty || 1)
      const toQty = normalizePositiveNumber(row?.to_qty)
      const fromUnit = normalizeOptionalUnitText(row?.from_unit ?? row?.sales_unit ?? row?.unit)
      const toUnit = normalizeOptionalUnitText(row?.to_unit) || inventoryUnit
      if (fromQty <= 0 || toQty <= 0 || !fromUnit || !toUnit) continue
      if (!graph[fromUnit]) graph[fromUnit] = {}
      graph[fromUnit][toUnit] = trimDecimal(toQty / fromQty)
      conversionKeys.push(fromUnit)
    }
  } else {
    const conversion = parseJSONObject(form.unit_conversion_json ?? form.unitConversionJSON)
    for (const [from, targets] of Object.entries(conversion)) {
      const fromUnit = normalizeOptionalUnitText(from)
      if (!fromUnit) continue
      if (!graph[fromUnit]) graph[fromUnit] = {}
      conversionKeys.push(fromUnit)
      const directFactor = normalizePositiveNumber(targets)
      if (directFactor > 0) {
        graph[fromUnit][inventoryUnit] = trimDecimal(directFactor)
        continue
      }
      const targetMap = parseJSONObject(targets)
      for (const [to, factorValue] of Object.entries(targetMap)) {
        const toUnit = normalizeOptionalUnitText(to)
        const factor = normalizePositiveNumber(factorValue)
        if (!toUnit || factor <= 0) continue
        graph[fromUnit][toUnit] = trimDecimal(factor)
      }
    }
  }

  const orderedUnits = uniqueInOrder([
    inventoryUnit,
    defaultSalesUnit || inventoryUnit,
    ...(Array.isArray(form.sales_units) ? form.sales_units : []),
    ...conversionKeys,
  ])
  const direct = {}
  for (const unit of orderedUnits) {
    if (!unit) continue
    if (unit === inventoryUnit) {
      direct[unit] = { [inventoryUnit]: 1 }
      continue
    }
    const factor = resolveUnitTemplateConversionFactor(unit, inventoryUnit, graph)
    if (factor > 0) {
      direct[unit] = { [inventoryUnit]: trimDecimal(factor) }
    }
  }
  return {
    sales_units: Object.keys(direct),
    unit_conversion_json: JSON.stringify(direct),
  }
}

function resolveUnitTemplateConversionFactor(unit = '', inventoryUnit = '', graph = {}, seen = new Set()) {
  const sourceUnit = normalizeOptionalUnitText(unit)
  const targetInventoryUnit = normalizeOptionalUnitText(inventoryUnit)
  if (!sourceUnit || !targetInventoryUnit) return 0
  if (sourceUnit === targetInventoryUnit) return 1
  if (seen.has(sourceUnit)) return 0
  seen.add(sourceUnit)
  const targets = graph[sourceUnit] || {}
  const direct = normalizePositiveNumber(targets[targetInventoryUnit])
  if (direct > 0) return direct
  for (const [targetUnit, factorValue] of Object.entries(targets)) {
    const factor = normalizePositiveNumber(factorValue)
    if (factor <= 0) continue
    const targetFactor = resolveUnitTemplateConversionFactor(targetUnit, targetInventoryUnit, graph, seen)
    if (targetFactor > 0) return trimDecimal(factor * targetFactor)
  }
  return 0
}

export function buildProductPriceRecordPayload(form = {}) {
  return {
    id: Number(form.id || 0),
    product_id: Number(form.product_id || 0),
    customer_product_alias_id: Number(form.customer_product_alias_id || 0),
    final_unit_price: Number(form.final_unit_price || 0),
    price_unit: normalizeUnitText(form.price_unit, 'kg'),
    currency: String(form.currency || '').trim().toUpperCase() || 'CNY',
    price_group_id: Number(form.price_group_id || 0),
    price_group_name: String(form.price_group_name || '').trim(),
    inventory_unit: normalizeUnitText(form.inventory_unit, 'kg'),
    inventory_conversion_json: normalizeInventoryConversionValue(form.inventory_conversion_json),
    status: String(form.status || '').trim() || 'draft',
    remark: String(form.remark || '').trim(),
  }
}

export function buildProductTierPriceSchemePayload(form = {}) {
  const tiers = (Array.isArray(form.tiers) ? form.tiers : [])
    .map((tier, index) => ({
      label: String(tier.label || '').trim(),
      min_qty: Number(tier.min_qty || 0),
      max_qty: tier.max_qty === '' || tier.max_qty === null || typeof tier.max_qty === 'undefined' ? null : Number(tier.max_qty),
      source_price_record_id: Number(tier.source_price_record_id || 0),
      position: Number(tier.position || 0) || index + 1,
    }))
    .sort((a, b) => {
      if (a.position !== b.position) return a.position - b.position
      return a.min_qty - b.min_qty
    })
  return {
    id: Number(form.id || 0),
    name: String(form.name || '').trim(),
    product_id: Number(form.product_id || 0),
    customer_product_alias_id: Number(form.customer_product_alias_id || 0),
    price_group_id: Number(form.price_group_id || 0),
    active: form.active === false ? false : true,
    remark: String(form.remark || '').trim(),
    tiers,
  }
}

export function productPriceRecordLabel(record = {}) {
  const group = String(record.price_group_name || '').trim() || '未分组'
  const currency = String(record.currency || '').trim().toUpperCase() || 'CNY'
  const price = formatProductFinalUnitPrice(record.final_unit_price)
  const unit = normalizeUnitText(record.price_unit, 'kg')
  return `${group} · ${currency} ${price}/${unit}`
}

export function buildSkuConfigOverridePayload(row = {}) {
  return {
    gradient_template_id_override: Number(row.gradient_template_id_override || 0),
    operation_template_id_override: Number(row.operation_template_id_override || 0),
    unit_rule_override_json: hasStructuredUnitRuleFields(row)
      ? unitRuleJSONFromForm(row)
      : normalizeJSONString(row.unit_rule_override_json),
  }
}

export function buildCustomerProductRuleTemplatePayload(form = {}) {
  return {
    id: Number(form.id || 0),
    customer_id: Number(form.customer_id || 0),
    name: String(form.name || '').trim(),
    active: form.active === false ? false : true,
    items: (form.items || []).map(buildCustomerProductRuleTemplateItemPayload),
  }
}

export function buildCustomerProductRuleTemplateItemPayload(row = {}) {
  return {
    product_subtype_category_id: Number(row.product_subtype_category_id || 0),
    gradient_template_id: Number(row.gradient_template_id || 0),
    operation_template_id: Number(row.operation_template_id || 0),
    price_list_rule_json: hasStructuredPriceRuleFields(row)
      ? priceListRuleJSONFromForm(row)
      : normalizeJSONString(row.price_list_rule_json),
    unit_rule_json: hasStructuredUnitRuleFields(row)
      ? unitRuleJSONFromForm(row)
      : normalizeJSONString(row.unit_rule_json),
    active: row.active === false ? false : true,
  }
}

export function buildCustomerProductRuleOverridePayload(row = {}) {
  return {
    id: Number(row.id || 0),
    customer_id: Number(row.customer_id || 0),
    product_subtype_category_id: Number(row.product_subtype_category_id || 0),
    gradient_template_id: Number(row.gradient_template_id || 0),
    operation_template_id: Number(row.operation_template_id || 0),
    price_list_rule_json: hasStructuredPriceRuleFields(row)
      ? priceListRuleJSONFromForm(row)
      : normalizeJSONString(row.price_list_rule_json),
    unit_rule_json: hasStructuredUnitRuleFields(row)
      ? unitRuleJSONFromForm(row)
      : normalizeJSONString(row.unit_rule_json),
    active: row.active === false ? false : true,
  }
}

export function buildCustomerProductRuleBindingPayload(customerID, templateID) {
  return {
    customer_id: Number(customerID || 0),
    template_id: Number(templateID || 0),
  }
}

export function buildSkuContextCategoryTree(categories = [], context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  const publicRoots = (categories || []).filter((category) => Number(category.customer_id || 0) === 0)
  if (!customerID) {
    return numberCategoryTree(publicRoots.map((primary) => projectCategoryNode(primary, context, null)))
  }

  const customerRoots = (categories || []).filter((category) => Number(category.customer_id || 0) === customerID)
  const customerRootBySource = new Map(customerRoots
    .filter((category) => Number(category.source_category_id || 0) > 0)
    .map((category) => [Number(category.source_category_id || 0), category]))
  const usedCustomerRootIDs = new Set()
  const out = []

  for (const publicRoot of publicRoots) {
    const derivedRoot = customerRootBySource.get(Number(publicRoot.id || 0))
    if (derivedRoot) {
      usedCustomerRootIDs.add(Number(derivedRoot.id || 0))
      out.push(projectMergedCategoryNode(derivedRoot, publicRoot, context, null))
      continue
    }
    if (categoryBelongsToSkuContext(publicRoot, context)) {
      out.push(projectCategoryNode(publicRoot, context, null))
    }
  }

  for (const customerRoot of customerRoots) {
    if (usedCustomerRootIDs.has(Number(customerRoot.id || 0))) continue
    if (categoryBelongsToSkuContext(customerRoot, context)) {
      out.push(projectCategoryNode(customerRoot, context, null))
    }
  }

  return numberCategoryTree(out)
}

export function isPublicReferenceRow(row = {}, context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  return customerID > 0 && Number(row.customer_id || 0) === 0
}

function projectMergedCategoryNode(customerCategory = {}, publicCategory = {}, context = {}, parentName = null) {
  const primaryName = parentName === null ? customerCategory.name || publicCategory.name || '' : parentName
  const secondaryName = parentName === null ? '' : customerCategory.name || publicCategory.name || ''
  const mergedProducts = [
    ...contextProductsForCategory(customerCategory, context),
    ...contextProductsForCategory(publicCategory, context),
  ]
  const customerChildren = customerCategory.children || []
  const publicChildren = publicCategory.children || []
  const customerChildBySource = new Map(customerChildren
    .filter((category) => Number(category.source_category_id || 0) > 0)
    .map((category) => [Number(category.source_category_id || 0), category]))
  const usedCustomerChildIDs = new Set()
  const children = []

  for (const publicChild of publicChildren) {
    const derivedChild = customerChildBySource.get(Number(publicChild.id || 0))
    if (derivedChild) {
      usedCustomerChildIDs.add(Number(derivedChild.id || 0))
      children.push(projectMergedCategoryNode(derivedChild, publicChild, context, primaryName))
      continue
    }
    if (categoryBelongsToSkuContext(publicChild, context)) {
      children.push(projectCategoryNode(publicChild, context, customerCategory.name || publicCategory.name || ''))
    }
  }

  for (const customerChild of customerChildren) {
    if (usedCustomerChildIDs.has(Number(customerChild.id || 0))) continue
    if (categoryBelongsToSkuContext(customerChild, context)) {
      children.push(projectCategoryNode(customerChild, context, customerCategory.name || publicCategory.name || ''))
    }
  }

  return {
    ...customerCategory,
    products: numberProducts(dedupeRowsByID(mergedProducts), primaryName, secondaryName),
    children: numberCategoryChildren(children),
  }
}

function projectCategoryNode(category = {}, context = {}, parentName = null) {
  const primaryName = parentName === null ? category.name || '' : parentName
  const secondaryName = parentName === null ? '' : category.name || ''
  return {
    ...category,
    products: numberProducts(contextProductsForCategory(category, context), primaryName, secondaryName),
    children: numberCategoryChildren((category.children || [])
      .filter((child) => categoryBelongsToSkuContext(child, context))
      .map((child) => projectCategoryNode(child, context, category.name || ''))),
  }
}

function contextProductsForCategory(category = {}, context = {}) {
  return (category.products || [])
    .filter((product) => productBelongsToCategoryTree(product, context))
}

function productBelongsToCategoryTree(product = {}, context = {}) {
  const allowPublicInCategoryTree = Boolean(context.usePublicSku || context.use_public_sku || context.usePublicSkuInCategoryTree)
  return productBelongsToSkuContext(product, {
    ...context,
    usePublicSku: allowPublicInCategoryTree,
    use_public_sku: allowPublicInCategoryTree,
  })
}

function numberCategoryTree(nodes = []) {
  return numberCategoryChildren(nodes)
}

function numberCategoryChildren(nodes = []) {
  return (nodes || []).map((node, index) => ({
    ...node,
    number: index + 1,
    children: numberCategoryChildren(node.children || []),
  }))
}

function numberProducts(products = [], primaryName = '', secondaryName = '') {
  return (products || []).map((product, index) => ({
    ...product,
    number: index + 1,
    primary_name: primaryName,
    secondary_name: secondaryName,
  }))
}

function categoryProductMetaByID(categoryTree = []) {
  const out = new Map()
  for (const primary of categoryTree || []) {
    const primaryName = primary?.name || ''
    for (const product of primary?.products || []) {
      const id = Number(product?.id || 0)
      if (!id) continue
      out.set(id, {
        number: product.number || '',
        primary_name: primaryName,
        secondary_name: '',
      })
    }
    for (const secondary of primary?.children || []) {
      const secondaryName = secondary?.name || ''
      for (const product of secondary?.products || []) {
        const id = Number(product?.id || 0)
        if (!id) continue
        out.set(id, {
          number: product.number || '',
          primary_name: primaryName,
          secondary_name: secondaryName,
        })
      }
    }
  }
  return out
}

function categoryPathMetaByID(categoryTree = []) {
  const out = new Map()
  function visit(category = {}, primaryName = '', secondaryName = '') {
    const id = Number(category?.id || 0)
    if (id) {
      out.set(id, {
        primary_name: primaryName || category?.name || '',
        secondary_name: secondaryName,
      })
    }
    const nextPrimaryName = primaryName || category?.name || ''
    for (const child of category?.children || []) {
      visit(child, nextPrimaryName, child?.name || '')
    }
  }
  for (const primary of categoryTree || []) {
    visit(primary, primary?.name || '', '')
  }
  return out
}

function dedupeRowsByID(rows = []) {
  const seen = new Set()
  const out = []
  for (const row of rows || []) {
    const id = Number(row?.id || 0)
    if (id && seen.has(id)) continue
    if (id) seen.add(id)
    out.push(row)
  }
  return out
}

function isUnmodifiedPublicSkuCopy(product = {}, publicProducts = []) {
  const baseID = Number(product.base_product_id || 0)
  if (!baseID || product.custom_type !== 'public_sku_alias') return false
  const base = (publicProducts || []).find((row) => Number(row.id || 0) === baseID)
  return Boolean(base && String(base.name || '').trim().toLowerCase() === String(product.name || '').trim().toLowerCase())
}

function hasCustomerDerivedProduct(product = {}, customerProducts = []) {
  const productID = Number(product.id || 0)
  if (!productID) return false
  return (customerProducts || []).some((row) => Number(row.base_product_id || 0) === productID && String(row.custom_type || '').trim() === 'public_sku_alias')
}

function hasCustomerDerivedCategory(category = {}, customerCategories = []) {
  const categoryID = Number(category.id || 0)
  if (!categoryID) return false
  return (customerCategories || []).some((row) => Number(row.source_category_id || 0) === categoryID)
}

function hasCustomerDerivedTemplate(template = {}, customerTemplates = []) {
  const templateID = Number(template.id || 0)
  if (!templateID) return false
  return (customerTemplates || []).some((row) => Number(row.source_template_id || 0) === templateID)
}

function isDuplicatedPublicCategory(category = {}, publicCategories = [], publicProducts = []) {
  if (!(category.products || []).length && !(category.children || []).length) return false
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

export function productSubtypeCategoryOptionsForType(categoryTree = [], productTypeCategoryID = 0) {
  const typeID = Number(productTypeCategoryID || 0)
  if (!typeID) return []
  const productType = (categoryTree || []).find((category) => Number(category?.id || 0) === typeID)
  if (!productType) return []
  return (productType.children || [])
    .filter((category) => Number(category?.id || 0) > 0)
    .map((category) => ({
      id: Number(category.id || 0),
      parent_id: Number(category.parent_id || typeID),
      name: category.name || '',
      customer_id: Number(category.customer_id || 0),
		source_category_id: Number(category.source_category_id || 0),
		product_config_template_id: Number(category.product_config_template_id || 0),
		template_state: category.template_state || '',
	}))
}

export function buildProductCreatePayload(form = {}) {
	const kind = normalizedProductKind(form)
	const payload = {
		name: String(form.name || '').trim(),
		product_kind: kind,
		remark: String(form.remark || '').trim(),
	}
	const ownershipType = String(form.ownership_type || '').trim().toLowerCase()
	if (ownershipType === 'factory' || ownershipType === 'customer') {
		payload.ownership_type = ownershipType
	}
	if (ownershipType === 'customer') {
		payload.customer_id = Number(form.customer_id || 0)
		payload.customer_display_name = String(form.customer_display_name || '').trim()
		payload.customer_item_code = String(form.customer_item_code || '').trim()
		payload.material_source_mode = String(form.material_source_mode || '').trim().toLowerCase() === 'customer' ? 'customer' : 'factory'
	}
	if (kind === 'green_bean') return payload
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
    return payload
  }
  if (payload.custom_type === 'custom_roast') {
    payload.base_product_id = 0
    payload.copy_bom = false
    payload.copy_price_tiers = false
  }
  if (kind === 'instant_coffee') {
    payload.copy_bom = false
    return payload
  }
  return payload
}

export function buildSkuCreatePayload(customerID, form = {}) {
	const payload = {
		customer_id: Number(customerID || form.customer_id || 0),
		name: String(form.name || '').trim(),
		remark: String(form.remark || '').trim(),
		active: form.active === false ? false : true,
	}
	const unitTemplateID = normalizedProductUnitTemplateID(form)
	const shouldSaveUnitOverride = productUnitOverrideShouldSave(form)
	if (Object.prototype.hasOwnProperty.call(form, 'unit_template_id')) {
		payload.unit_template_id = unitTemplateID
	}
	if ((shouldSaveUnitOverride || unitTemplateID <= 0) && Object.prototype.hasOwnProperty.call(form, 'inventory_unit')) {
		payload.inventory_unit = String(form.inventory_unit || 'kg').trim() || 'kg'
	}
	if ((shouldSaveUnitOverride || unitTemplateID <= 0) && Object.prototype.hasOwnProperty.call(form, 'integer_inventory_unit')) {
		payload.integer_inventory_unit = Boolean(form.integer_inventory_unit)
	}
  if (shouldSaveUnitOverride) appendProductSalesUnitPayload(payload, form)
	return payload
}

export function buildChildSkuCreatePayload(parentProductID, form = {}) {
  const payload = {
    parent_product_id: Number(parentProductID || form.parent_product_id || form.parentProductID || 0),
    name: String(form.name || '').trim(),
    sku_name: String(form.sku_name || form.skuName || '').trim(),
    sku_code: String(form.sku_code || form.skuCode || '').trim(),
    barcode: String(form.barcode || '').trim(),
    spec_label: String(form.spec_label || form.specLabel || '').trim(),
    net_content_qty: Number(form.net_content_qty || form.netContentQty || 0),
    net_content_unit: String(form.net_content_unit || form.netContentUnit || '').trim(),
    unit_template_id: normalizedProductUnitTemplateID(form),
    active: form.active === false ? false : true,
  }
  if (Number(form.customer_id || form.customerID || 0) > 0) payload.customer_id = Number(form.customer_id || form.customerID || 0)
  if (String(form.remark || '').trim()) payload.remark = String(form.remark || '').trim()
  return payload
}

export function productSkuRowsForParent(products = [], parentProductID = 0) {
  const parentID = Number(parentProductID || 0)
  if (!parentID) return []
  const rows = (Array.isArray(products) ? products : [])
    .filter((row) => {
      const id = Number(row?.id || row?.product_id || 0)
      const skuID = Number(row?.sku_id || id || 0)
      const directParentID = Number(row?.parent_product_id || row?.parentProductID || 0)
      const effectiveParentID = Number(row?.effective_parent_product_id || row?.effectiveParentProductID || directParentID || id || 0)
      return id === parentID || skuID === parentID || directParentID === parentID || effectiveParentID === parentID
    })

  const parentRow = rows.find((row) => Number(row?.id || row?.product_id || 0) === parentID) || {}
  const authoritativeDefaultSkuID = Number(
    parentRow?.default_sku_id
      || parentRow?.defaultSkuID
      || parentRow?.effective_default_sku_id
      || parentRow?.effectiveDefaultSkuID
      || rows.find((row) => Number(row?.effective_default_sku_id || row?.effectiveDefaultSkuID || 0) > 0)?.effective_default_sku_id
      || rows.find((row) => row?.is_default_sku === true && Number(row?.parent_product_id || 0) === parentID)?.sku_id
      || 0,
  )

  return rows
    .map((row) => {
      const id = Number(row?.id || row?.product_id || 0)
      const skuID = Number(row?.sku_id || id || 0)
      const directParentID = Number(row?.parent_product_id || row?.parentProductID || 0)
      const isDefault = authoritativeDefaultSkuID > 0
        ? skuID === authoritativeDefaultSkuID
        : (row?.is_default_sku === true || row?.isDefaultSKU === true || id === parentID || directParentID === 0)
      const skuName = String(row?.sku_name || row?.skuName || (isDefault ? '默认规格' : row?.name || '')).trim() || '默认规格'
      return {
        ...row,
        sku_id: skuID,
        parent_product_id: directParentID,
        effective_parent_product_id: Number(row?.effective_parent_product_id || row?.effectiveParentProductID || directParentID || id || 0),
        sku_name: skuName,
        spec_label: String(row?.spec_label || row?.specLabel || '').trim(),
        is_default_sku: isDefault,
      }
    })
    .sort((a, b) => {
      if (a.is_default_sku !== b.is_default_sku) return a.is_default_sku ? -1 : 1
      return Number(a.sku_id || a.id || 0) - Number(b.sku_id || b.id || 0)
    })
}

export function productArchiveRowsWithSkus(products = []) {
  const source = Array.isArray(products) ? products : []
  const parentIDs = new Set(source
    .filter((row) => Number(row?.parent_product_id || row?.parentProductID || 0) === 0)
    .map((row) => Number(row?.id || row?.product_id || 0))
    .filter(Boolean))
  const childrenByParentID = new Map()
  const parents = []

  for (const row of source) {
    const parentID = Number(row?.parent_product_id || row?.parentProductID || 0)
    if (!parentID || !parentIDs.has(parentID)) {
      parents.push(row)
      continue
    }
    if (String(row?.derived_spec_status || row?.derivedSpecStatus || '').trim() === 'template_removed') continue
    if (!childrenByParentID.has(parentID)) childrenByParentID.set(parentID, [])
    childrenByParentID.get(parentID).push(row)
  }

  return parents.map((parent) => {
    const parentID = Number(parent?.id || parent?.product_id || 0)
    const bomAuthoritative = parent?.bom_spec_authoritative === true
      || parent?.bomSpecAuthoritative === true
      || parent?.legacy_catalog_product === false
      || String(parent?.migration_state || parent?.migrationState || '').trim().toLowerCase() === 'cutover'
    if (bomAuthoritative) {
      const bomSpecs = Array.isArray(parent?.bom_specs) ? parent.bom_specs : []
      const skuSearchText = bomSpecs.map((spec) => [
        spec?.spec_name,
        spec?.spec_key,
        spec?.inventory_unit,
      ].filter(Boolean).join(' ')).join(' ')
      return { ...parent, sku_rows: [], sku_search_text: skuSearchText, bom_specs: bomSpecs }
    }
    const skuRows = (childrenByParentID.get(parentID) || [])
      .map((row) => ({
        ...row,
        sku_name: String(row?.sku_name || row?.skuName || row?.spec_label || row?.specLabel || row?.name || '').trim(),
      }))
    const skuSearchText = skuRows.map((row) => [
      row.name,
      row.sku_name,
      row.spec_label,
      productCodeLabel(row),
    ].filter(Boolean).join(' ')).join(' ')
    return { ...parent, sku_rows: skuRows, sku_search_text: skuSearchText }
  })
}

export function pricingRuleTrialProductSpecUnit(product = {}) {
  const candidates = [
    product?.derived_sales_unit,
    product?.derivedSalesUnit,
    product?.sales_unit,
    product?.salesUnit,
    product?.default_sales_unit,
    product?.defaultSalesUnit,
    product?.quote_unit,
    product?.quoteUnit,
    product?.order_unit,
    product?.orderUnit,
    product?.derived_spec_name,
    product?.derivedSpecName,
    product?.spec_label,
    product?.specLabel,
  ]
  return candidates.map((value) => String(value ?? '').trim()).find(Boolean) || ''
}

function pricingRuleTrialProductSpecIsActive(product = {}) {
  if (product?.active === false || product?.active === 0) return false
  const status = String(product?.status || product?.product_status || '').trim().toLowerCase()
  if (['inactive', 'disabled', 'deprecated', 'archived'].includes(status)) return false
  const derivedStatus = String(product?.derived_spec_status || product?.derivedSpecStatus || '').trim().toLowerCase()
  return derivedStatus === '' || derivedStatus === 'active'
}

export function pricingRuleTrialProductSpecOptions(products = [], parentProductID = 0) {
  const source = Array.isArray(products) ? products : []
  const parentID = Number(parentProductID || 0)
  if (!(parentID > 0)) return []

  const parent = source.find((row) => Number(row?.id || row?.product_id || 0) === parentID) || {}
  const authoritative = parent?.bom_spec_authoritative === true
    || parent?.legacy_catalog_product === false
    || String(parent?.migration_state || parent?.migrationState || '').trim().toLowerCase() === 'cutover'
  const bomSpecs = Array.isArray(parent?.bom_specs) ? parent.bom_specs : []
  if (authoritative && bomSpecs.length) {
    return bomSpecs
      .map((spec) => {
        const specID = Number(spec?.bom_spec_id || spec?.bomSpecID || 0)
        const variantID = Number(spec?.bom_variant_id || spec?.bomVariantID || 0)
        const unit = String(spec?.inventory_unit || spec?.unit || '').trim()
        const name = String(spec?.spec_name || spec?.name || spec?.spec_key || '').trim()
        return {
          ...parent,
          ...spec,
          id: specID,
          product_id: parentID,
          sku_id: specID,
          parent_product_id: parentID,
          effective_parent_product_id: parentID,
          bom_id: Number(spec?.bom_id || spec?.bomID || 0),
          bom_version_id: Number(spec?.bom_version_id || spec?.bomVersionID || 0),
          bom_spec_id: specID,
          bom_variant_id: variantID,
          sku_name: name,
          spec_label: name,
          derived_sales_unit: unit,
          default_sales_unit: unit,
          inventory_unit: unit,
          is_default_sku: spec?.is_default === true || spec?.isDefault === true,
          migration_state: parent?.migration_state || parent?.migrationState || 'preparing',
          spec_identity_mode: 'bom_spec',
          bom_spec_authoritative: true,
        }
      })
      .filter((row) => Number(row.bom_spec_id || 0) > 0 && Number(row.bom_variant_id || 0) > 0 && row.sku_name)
  }

  const rows = productSkuRowsForParent(source, parentID)
  const concreteSpecs = rows.filter((row) => {
    const rowID = Number(row?.id || row?.product_id || row?.sku_id || 0)
    const directParentID = Number(row?.parent_product_id || row?.parentProductID || 0)
    return directParentID === parentID
      && rowID !== parentID
      && pricingRuleTrialProductSpecIsActive(row)
      && Boolean(pricingRuleTrialProductSpecUnit(row))
  })
  const parentFallback = rows.find((row) => {
    const rowID = Number(row?.id || row?.product_id || row?.sku_id || 0)
    return rowID === parentID
      && pricingRuleTrialProductSpecIsActive(row)
      && Boolean(pricingRuleTrialProductSpecUnit(row))
  })
  const candidates = concreteSpecs.length ? concreteSpecs : (parentFallback ? [parentFallback] : [])

  const seen = new Set()
  return candidates.filter((row) => {
    const skuID = Number(row?.sku_id || row?.id || row?.product_id || 0)
    if (!(skuID > 0) || seen.has(skuID)) return false
    seen.add(skuID)
    return true
  })
}

export function pricingRuleTrialDefaultProductSpecID(options = []) {
  const rows = Array.isArray(options) ? options : []
  const selected = rows.find((row) => row?.is_default_sku === true || row?.isDefaultSKU === true) || rows[0]
  return Number(selected?.sku_id || selected?.id || selected?.product_id || 0)
}

export function pricingRuleTrialProductSpecLabel(product = {}) {
  const label = String(
    product?.spec_label
      || product?.specLabel
      || product?.derived_spec_name
      || product?.derivedSpecName
      || product?.sku_name
      || product?.skuName
      || product?.name
      || '',
  ).trim()
  const unit = pricingRuleTrialProductSpecUnit(product)
  const text = label && unit && label.toLowerCase() !== unit.toLowerCase() ? `${label} / ${unit}` : (label || unit || '未命名规格')
  return product?.is_default_sku === true || product?.isDefaultSKU === true ? `${text}（默认）` : text
}

export function pricingRuleTrialMainProductOptions(products = []) {
  const source = Array.isArray(products) ? products : []
  const seen = new Set()
  const rows = []
  for (const product of source) {
    const productID = Number(product?.id || product?.product_id || 0)
    if (!(productID > 0) || seen.has(productID)) continue
    if (Number(product?.parent_product_id || product?.parentProductID || 0) !== 0) continue
    if (product?.active === false || product?.active === 0) continue
    const status = String(product?.status || product?.product_status || '').trim().toLowerCase()
    if (['inactive', 'disabled', 'deprecated', 'archived'].includes(status)) continue
    seen.add(productID)
    rows.push(product)
  }
  const parentIDs = new Set(rows.map((product) => Number(product?.id || product?.product_id || 0)))
  const childrenByParentID = new Map()
  for (const product of source) {
    const parentID = Number(product?.parent_product_id || product?.parentProductID || 0)
    if (!(parentID > 0) || !parentIDs.has(parentID) || !pricingRuleTrialProductSpecIsActive(product)) continue
    if (!childrenByParentID.has(parentID)) childrenByParentID.set(parentID, [])
    childrenByParentID.get(parentID).push(product)
  }
  return rows.map((product) => {
    const productID = Number(product?.id || product?.product_id || 0)
    const skuRows = childrenByParentID.get(productID) || []
    return {
      ...product,
      sku_rows: skuRows,
      sku_search_text: skuRows.map((row) => [
        row?.name,
        row?.sku_name,
        row?.spec_label,
        productCodeLabel(row),
      ].filter(Boolean).join(' ')).join(' '),
    }
  })
}

export function resolveCreatedProductForConfig(result = {}, products = []) {
  const createdProduct = result?.product || result?.sku || result || {}
  const createdID = Number(createdProduct.id || createdProduct.product_id || 0)
  if (createdID > 0) {
    const product = products.find(row => Number(row?.id || row?.product_id || 0) === createdID)
    if (product) return product
  }

  const createdCode = String(createdProduct.code || createdProduct.product_code || createdProduct.number || '').trim()
  if (createdCode) {
    const product = products.find(row => String(row?.code || row?.product_code || row?.number || '').trim() === createdCode)
    if (product) return product
  }

  const createdName = String(createdProduct.name || createdProduct.product_name || '').trim()
  if (createdName) {
    const product = products.find(row => String(row?.name || row?.product_name || '').trim() === createdName)
    if (product) return product
  }

  return createdID > 0 || createdName ? createdProduct : null
}

export function buildProductProductionConfigField(row = {}, index = 0) {
  const source = row && typeof row === 'object' ? row : {}
  const rawType = String(source.field_type || '').trim()
  const type = ['text', 'textarea', 'number', 'ratio', 'select', 'checkbox', 'date', 'bool'].includes(rawType) ? (rawType === 'bool' ? 'checkbox' : rawType) : 'text'
  return {
    local_id: `${Number(source.id || 0) || 'new'}-${Date.now()}-${index}`,
    id: Number(source.id || 0),
    field_key: String(source.field_key || '').trim(),
    template_field_key: String(source.template_field_key || source.field_key || '').trim(),
    label: String(source.label || '').trim(),
    field_type: type,
    unit: String(source.unit || '').trim(),
    value_text: String(source.value_text || '').trim(),
    value_number: source.value_number === null || typeof source.value_number === 'undefined' || source.value_number === '' ? null : Number(source.value_number),
    value_bool: Boolean(source.value_bool),
    required: Boolean(source.required),
    options_json: String(source.options_json || '[]').trim() || '[]',
    show_in_price_list: source.show_in_price_list !== false,
    sort_order: Number(source.sort_order || index + 1),
  }
}

function templateFieldDefaultText(field = {}) {
  const fieldType = String(field.field_type || '').trim()
  if (!['text', 'textarea'].includes(fieldType)) return ''
  return fieldOptionsFromJSON(field.options_json)[0] || ''
}

function fieldOptionsFromJSON(raw = '[]') {
  try {
    const parsed = JSON.parse(String(raw || '[]'))
    return Array.isArray(parsed) ? parsed.map((item) => String(item || '').trim()).filter(Boolean) : []
  } catch (_) {
    return []
  }
}

function indexProductProductionConfigFields(fields = []) {
  const byKey = new Map()
  for (const field of Array.isArray(fields) ? fields : []) {
    const keys = [field?.template_field_key, field?.field_key]
      .map((value) => String(value || '').trim().toLowerCase())
      .filter(Boolean)
    for (const key of keys) {
      if (!byKey.has(key)) byKey.set(key, field)
    }
  }
  return { byKey }
}

function productProductionConfigTemplateFieldMatch(field = {}, index = {}) {
  const key = String(field.field_key || '').trim().toLowerCase()
  return index.byKey?.get(key) || {}
}

export function productProductionConfigFieldsFromTemplates(fields = [], templates = []) {
  const sourceTemplates = Array.isArray(templates) ? templates : (templates ? [templates] : [])
  if (!sourceTemplates.length) return []
  const existingIndex = indexProductProductionConfigFields(fields)
  const projected = []
  const seenKeys = new Set()
  for (const template of sourceTemplates) {
    const templateFields = Array.isArray(template?.fields) ? template.fields : []
    for (const field of templateFields
      .slice()
      .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0))) {
      const key = String(field.field_key || '').trim()
      const normalizedKey = key.toLowerCase()
      if (!normalizedKey || seenKeys.has(normalizedKey)) continue
      seenKeys.add(normalizedKey)
      const existing = productProductionConfigTemplateFieldMatch(field, existingIndex)
      projected.push(buildProductProductionConfigField({
        ...existing,
        field_key: key,
        template_field_key: key,
        label: field.label || key,
        field_type: field.field_type || existing.field_type || 'text',
        unit: field.unit || '',
        value_text: existing.value_text || templateFieldDefaultText(field),
        required: Boolean(field.required),
        options_json: field.options_json || '[]',
        show_in_price_list: existing.show_in_price_list !== false,
        sort_order: projected.length + 1,
      }, projected.length))
    }
  }
  return projected
}

export function productProductionConfigFieldsFromTemplate(fields = [], template = {}) {
  return productProductionConfigFieldsFromTemplates(fields, template ? [template] : [])
}

export function industryFieldTemplateIDsFromConfig(config = {}) {
  const source = config && typeof config === 'object' ? config : {}
  const rawIDs = source.industry_field_template_ids ?? source.industryFieldTemplateIDs
  if (Array.isArray(rawIDs)) {
    return [...new Set(rawIDs.map((id) => Number(id || 0)).filter((id) => id > 0))]
  }
  const legacyID = Number(source.industry_field_template_id ?? source.industryFieldTemplateID ?? 0)
  return legacyID > 0 ? [legacyID] : []
}

export function industryFieldTemplateOptionsForConfig(templates = [], config = {}) {
  const sourceTemplates = Array.isArray(templates) ? templates : []
  const selectedIDs = industryFieldTemplateIDsFromConfig(config)
  const byID = new Map(sourceTemplates
    .map((template) => [Number(template?.id || 0), template])
    .filter(([id]) => id > 0))
  const activeTemplates = sourceTemplates
    .filter((template) => Number(template?.id || 0) > 0 && String(template?.status || 'active') === 'active')
    .slice()
    .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || String(a.name || '').localeCompare(String(b.name || '')))
  const options = []
  const includedIDs = new Set()
  for (const [index, id] of selectedIDs.entries()) {
    const template = byID.get(id)
    options.push(template
      ? {
          ...template,
          selected_order: index + 1,
          unavailable: String(template.status || 'active') !== 'active',
        }
      : {
          id,
          name: `行业字段模板 #${id}`,
          status: 'missing',
          selected_order: index + 1,
          unavailable: true,
        })
    includedIDs.add(id)
  }
  for (const template of activeTemplates) {
    const id = Number(template.id || 0)
    if (includedIDs.has(id)) continue
    options.push({ ...template, selected_order: 0, unavailable: false })
    includedIDs.add(id)
  }
  return options
}

export function buildProductProductionConfigForm(config = {}, product = {}, industryFieldTemplates = []) {
  const sourceConfig = config && typeof config === 'object' ? config : {}
  const sourceProduct = product && typeof product === 'object' ? product : {}
  const industryTemplateIDs = industryFieldTemplateIDsFromConfig(sourceConfig)
  const sourceIndustryTemplates = Array.isArray(industryFieldTemplates)
    ? industryFieldTemplates
    : (industryFieldTemplates ? [industryFieldTemplates] : [])
  const industryTemplatesByID = new Map(sourceIndustryTemplates.map((template) => [Number(template?.id || 0), template]))
  const orderedIndustryTemplates = industryTemplateIDs.length
    ? industryTemplateIDs.map((id) => industryTemplatesByID.get(id)).filter(Boolean)
    : sourceIndustryTemplates
  const fields = Array.isArray(sourceConfig.fields) ? sourceConfig.fields : []
  const ruleOverride = parseJSONObject(sourceProduct.unit_rule_override_json || sourceProduct.unitRuleOverrideJSON)
  const salesUnitRules = parseJSONObject(sourceProduct.sales_unit_rules || sourceProduct.salesUnitRules || ruleOverride.sales_unit_rules || {})
  const inventoryUnit = String(sourceProduct.inventory_unit || 'kg').trim() || 'kg'
  const unitTemplateID = Number(sourceProduct.unit_template_id || sourceProduct.unitTemplateID || 0)
  const unitConversionRows = unitConversionRowsFromJSON(sourceProduct.unit_conversion_json || sourceProduct.unitConversionJSON || ruleOverride.unit_conversion_json || ruleOverride.conversion_json || '{}', inventoryUnit)
    .map((row) => ({
      ...row,
      integer_sales_unit: salesUnitIntegerFromRules(salesUnitRules, row.from_unit),
    }))
  return {
    product_id: Number(sourceConfig.product_id || sourceProduct.id || 0),
    sku_id: Number(sourceProduct.sku_id || sourceProduct.id || 0),
    parent_product_id: Number(sourceProduct.parent_product_id || sourceProduct.parentProductID || 0),
    effective_parent_product_id: Number(sourceProduct.effective_parent_product_id || sourceProduct.effectiveParentProductID || sourceProduct.parent_product_id || sourceProduct.id || 0),
    sku_name: String(sourceProduct.sku_name || sourceProduct.skuName || '').trim(),
    sku_code: String(sourceProduct.sku_code || sourceProduct.skuCode || '').trim(),
    barcode: String(sourceProduct.barcode || '').trim(),
    spec_label: String(sourceProduct.spec_label || sourceProduct.specLabel || '').trim(),
    net_content_qty: Number(sourceProduct.net_content_qty || sourceProduct.netContentQty || 0),
    net_content_unit: String(sourceProduct.net_content_unit || sourceProduct.netContentUnit || '').trim(),
    name: String(sourceProduct.name || '').trim(),
    remark: String(sourceProduct.remark || '').trim(),
    product_kind: sourceProduct.product_kind || 'roasted',
    unit_template_id: unitTemplateID,
    unit_template_name: String(sourceProduct.unit_template_name || sourceProduct.unitTemplateName || '').trim(),
    unit_rule_source: String(sourceProduct.unit_rule_source || sourceProduct.unitRuleSource || '').trim(),
    unit_rule_override_enabled: hasExplicitProductUnitRuleOverride(sourceProduct) || String(sourceProduct.unit_rule_source || sourceProduct.unitRuleSource || '') === 'product_override',
    inventory_unit: inventoryUnit,
    integer_inventory_unit: Boolean(sourceProduct.integer_inventory_unit || sourceProduct.integer_unit || sourceProduct.stock_integer_unit),
    default_sales_unit: String(sourceProduct.default_sales_unit || sourceProduct.defaultSalesUnit || sourceProduct.quote_unit || sourceProduct.order_unit || sourceProduct.inventory_unit || 'kg').trim() || 'kg',
    unit_conversion_json: sourceProduct.unit_conversion_json || sourceProduct.unitConversionJSON || '{}',
    unit_conversion_rows: unitConversionRows,
    sales_unit_rules: salesUnitRules,
    production_bom_id: Number(sourceConfig.production_bom_id || sourceProduct.production_bom_id || 0),
    production_bom_version_id: Number(sourceConfig.production_bom_version_id || sourceProduct.production_bom_version_id || 0),
    process_route_id: Number(sourceConfig.process_route_id || 0),
    industry_field_template_ids: industryTemplateIDs,
    industry_field_template_id: Number(industryTemplateIDs[0] || 0),
    note: String(sourceConfig.note || sourceProduct.production_config_note || '').trim(),
    fields: productProductionConfigFieldsFromTemplates(fields, orderedIndustryTemplates),
  }
}

export function buildProductBasicsPayload(row = {}) {
  const kind = normalizedProductKind(row)
  const payload = {
    product_kind: kind,
    remark: String(row.remark || '').trim(),
  }
  const unitTemplateID = normalizedProductUnitTemplateID(row)
  const shouldSaveUnitOverride = productUnitOverrideShouldSave(row)
	const name = String(row.name || '').trim()
	if (name) payload.name = name
	if (Object.prototype.hasOwnProperty.call(row, 'unit_template_id')) {
		payload.unit_template_id = unitTemplateID
	}
	if ((shouldSaveUnitOverride || unitTemplateID <= 0) && Object.prototype.hasOwnProperty.call(row, 'inventory_unit')) {
		payload.inventory_unit = String(row.inventory_unit || 'kg').trim() || 'kg'
	}
	if ((shouldSaveUnitOverride || unitTemplateID <= 0) && Object.prototype.hasOwnProperty.call(row, 'integer_inventory_unit')) {
		payload.integer_inventory_unit = Boolean(row.integer_inventory_unit)
	}
  if (shouldSaveUnitOverride) appendProductSalesUnitPayload(payload, row)
	if (Object.prototype.hasOwnProperty.call(row, 'unit_rule_override_json')) {
		payload.unit_rule_override_json = shouldSaveUnitOverride
      ? String(row.unit_rule_override_json || '{}').trim() || '{}'
      : stripProductUnitRuleOverrideJSON(row.unit_rule_override_json)
	}
  return payload
}

export function buildProductProductionConfigBasicsPayload(originalProduct = {}, form = {}) {
  const sourceProduct = originalProduct && typeof originalProduct === 'object' ? originalProduct : {}
  const sourceForm = form && typeof form === 'object' ? form : {}
  const payloadSource = {
    product_kind: sourceProduct.product_kind || sourceForm.product_kind || 'roasted',
    name: sourceForm.name,
    remark: sourceForm.remark,
    unit_template_id: Number(sourceForm.unit_template_id || 0),
    unit_rule_override_enabled: Boolean(sourceForm.unit_rule_override_enabled),
    unit_rule_override_json: sourceProduct.unit_rule_override_json || sourceProduct.unitRuleOverrideJSON || '{}',
    inventory_unit: sourceForm.inventory_unit,
    integer_inventory_unit: Boolean(sourceForm.integer_inventory_unit),
  }
  if (productUnitOverrideShouldSave(payloadSource) && productProductionConfigSalesUnitOverrideShouldSave(sourceProduct, sourceForm)) {
    payloadSource.default_sales_unit = sourceForm.default_sales_unit
    payloadSource.unit_conversion_rows = sourceForm.unit_conversion_rows
    payloadSource.sales_unit_rules = sourceForm.sales_unit_rules
  }
  return buildProductBasicsPayload(payloadSource)
}

function productProductionConfigSalesUnitOverrideShouldSave(product = {}, form = {}) {
  if (Number(form.unit_template_id || 0) > 0 && form.unit_rule_override_enabled === false) return false
  if (hasExplicitProductSalesUnitOverride(product)) return true
  if (form.unit_rule_override_enabled === true) return true
  const initial = buildProductProductionConfigForm({}, product)
  return normalizeUnitText(form.default_sales_unit, initial.default_sales_unit || initial.inventory_unit)
    !== normalizeUnitText(initial.default_sales_unit, initial.inventory_unit)
    || stableJSONObjectText(parseJSONObject(productSalesUnitConversionPayload(form, form.default_sales_unit, form.inventory_unit) || {}))
    !== stableJSONObjectText(parseJSONObject(productSalesUnitConversionPayload(initial, initial.default_sales_unit, initial.inventory_unit) || {}))
    || stableJSONObjectText(parseJSONObject(salesUnitRulesPayload(form) || {}))
    !== stableJSONObjectText(parseJSONObject(salesUnitRulesPayload(initial) || {}))
}

function hasExplicitProductSalesUnitOverride(product = {}) {
  const rule = parseJSONObject(product.unit_rule_override_json ?? product.unitRuleOverrideJSON)
  return [
    'default_sales_unit',
    'quote_unit',
    'order_unit',
    'unit_conversion_json',
    'conversion_json',
    'sales_unit_rules',
  ].some((key) => Object.prototype.hasOwnProperty.call(rule, key))
}

function hasExplicitProductUnitRuleOverride(product = {}) {
  const rule = parseJSONObject(product.unit_rule_override_json ?? product.unitRuleOverrideJSON)
  return productUnitRuleOverrideKeys.some((key) => Object.prototype.hasOwnProperty.call(rule, key))
}

const productUnitRuleOverrideKeys = [
  'inventory_unit',
  'integer_inventory_unit',
  'integer_unit',
  'default_sales_unit',
  'quote_unit',
  'order_unit',
  'unit_conversion_json',
  'conversion_json',
  'sales_unit_rules',
]

function normalizedProductUnitTemplateID(form = {}) {
  return Number(form.unit_template_id || form.unitTemplateID || 0)
}

function productUnitOverrideShouldSave(form = {}) {
  if (Object.prototype.hasOwnProperty.call(form, 'unit_rule_override_enabled')) {
    return Boolean(form.unit_rule_override_enabled)
  }
  return hasExplicitProductUnitRuleOverride(form) || String(form.unit_rule_source || form.unitRuleSource || '') === 'product_override'
}

function stripProductUnitRuleOverrideJSON(raw = '{}') {
  const rule = { ...parseJSONObject(raw) }
  for (const key of productUnitRuleOverrideKeys) delete rule[key]
  return stableJSONObjectText(rule)
}

function appendProductSalesUnitPayload(payload, form = {}) {
  if (!payload || !form) return payload
  const inventoryUnit = normalizeUnitText(form.inventory_unit ?? payload.inventory_unit, 'kg')
  if (
    Object.prototype.hasOwnProperty.call(form, 'default_sales_unit')
    || Object.prototype.hasOwnProperty.call(form, 'defaultSalesUnit')
    || Object.prototype.hasOwnProperty.call(form, 'sales_unit')
    || Object.prototype.hasOwnProperty.call(form, 'quote_unit')
    || Object.prototype.hasOwnProperty.call(form, 'order_unit')
  ) {
    payload.default_sales_unit = normalizeUnitText(
      form.default_sales_unit ?? form.defaultSalesUnit ?? form.sales_unit ?? form.quote_unit ?? form.order_unit,
      inventoryUnit,
    )
  }
  const conversion = productSalesUnitConversionPayload(form, payload.default_sales_unit || inventoryUnit, inventoryUnit)
  if (conversion !== null) payload.unit_conversion_json = conversion
  const salesRules = salesUnitRulesPayload(form)
  if (salesRules !== null) payload.sales_unit_rules = salesRules
  return payload
}

function productSalesUnitConversionPayload(form = {}, defaultSalesUnit = '', inventoryUnit = '') {
  if (Array.isArray(form.unit_conversion_rows)) {
    const parsed = parseJSONObject(unitConversionJSONFromRows(form.unit_conversion_rows))
    if (Object.keys(parsed).length) return parsed
    const salesUnit = normalizeOptionalUnitText(defaultSalesUnit)
    const stockUnit = normalizeOptionalUnitText(inventoryUnit)
    if (salesUnit && stockUnit && salesUnit === stockUnit) return { [salesUnit]: { [stockUnit]: 1 } }
    return {}
  }
  if (Object.prototype.hasOwnProperty.call(form, 'unit_conversion_json') || Object.prototype.hasOwnProperty.call(form, 'unitConversionJSON')) {
    return parseJSONObject(form.unit_conversion_json ?? form.unitConversionJSON)
  }
  return null
}

function salesUnitRulesPayload(form = {}) {
  const hasRawRules = Object.prototype.hasOwnProperty.call(form, 'sales_unit_rules') || Object.prototype.hasOwnProperty.call(form, 'salesUnitRules')
  if (!Array.isArray(form.unit_conversion_rows)) {
    return hasRawRules ? parseJSONObject(form.sales_unit_rules ?? form.salesUnitRules) : null
  }
  const out = hasRawRules ? { ...parseJSONObject(form.sales_unit_rules ?? form.salesUnitRules) } : {}
  for (const row of form.unit_conversion_rows) {
    const unit = normalizeOptionalUnitText(row?.from_unit ?? row?.sales_unit ?? row?.unit)
    if (!unit) continue
    if (Object.prototype.hasOwnProperty.call(row, 'integer_sales_unit') || Object.prototype.hasOwnProperty.call(row, 'integer_unit')) {
      out[unit] = { ...(parseJSONObject(out[unit])), integer_unit: Boolean(row?.integer_sales_unit ?? row?.integer_unit) }
    }
  }
  return Object.keys(out).length ? out : {}
}

function salesUnitIntegerFromRules(rules = {}, unit = '') {
  const normalizedUnit = normalizeOptionalUnitText(unit)
  if (!normalizedUnit) return false
  const rule = parseJSONObject(rules[normalizedUnit])
  if (Object.prototype.hasOwnProperty.call(rule, 'integer_unit')) return Boolean(rule.integer_unit)
  if (Object.prototype.hasOwnProperty.call(rule, 'integer')) return Boolean(rule.integer)
  return false
}

function rowCustomerID(row = {}) {
  return Number(row.customer_id ?? row.customerID ?? 0)
}

function rowOrderUsageCount(row = {}) {
  const raw = row.order_usage_count ?? row.orderUsageCount ?? row.order_count ?? row.orderCount ?? 0
  const value = Number(raw || 0)
  return Number.isFinite(value) ? value : 0
}

export function sortRowsForCustomerSkuPriority(rows = [], customerID = 0) {
  const selectedCustomerID = Number(customerID || 0)
  return [...rows].sort((a, b) => {
    if (selectedCustomerID > 0) {
      const aOwned = rowCustomerID(a) === selectedCustomerID ? 0 : 1
      const bOwned = rowCustomerID(b) === selectedCustomerID ? 0 : 1
      if (aOwned !== bOwned) return aOwned - bOwned
    }
    const usageDiff = rowOrderUsageCount(b) - rowOrderUsageCount(a)
    if (usageDiff !== 0) return usageDiff
    const positionDiff = Number(a.product_category_position || 0) - Number(b.product_category_position || 0)
    if (positionDiff !== 0) return positionDiff
    const numberDiff = Number(a.number || 0) - Number(b.number || 0)
    if (numberDiff !== 0) return numberDiff
    return String(a.name || '').localeCompare(String(b.name || ''))
  })
}

export function buildProductBomURL(currentHref = '', row = {}) {
  const url = new URL(currentHref || window.location.href)
  const bomID = Number(row.production_bom_id || row.bom_id || 0)
  url.searchParams.set('view', 'bom')
  if (bomID > 0) {
    url.searchParams.set('production_bom_id', String(bomID))
  } else {
    url.searchParams.delete('production_bom_id')
  }
  url.searchParams.delete('product_id')
  url.searchParams.delete('bom_filter_product_id')
  return url
}

function uniqueSorted(values = []) {
  return Array.from(new Set(values.map((value) => String(value || '').trim()).filter(Boolean)))
    .sort((a, b) => a.localeCompare(b))
}

function uniqueInOrder(values = []) {
  const out = []
  const seen = new Set()
  for (const value of values || []) {
    const normalized = String(value || '').trim()
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    out.push(normalized)
  }
  return out
}

export function priceListRuleFormFromJSON(value = {}) {
	const rule = parseJSONObject(value)
	const extra = { ...rule }
	for (const key of ['enabled', 'include_in_price_list', 'pricing_mode', 'display_mode', 'display_unit', 'rounding', 'tax_included', 'unit_price', 'price_per_unit', 'fixed_unit_price', 'fixed_price', 'cost_plus_rate', 'markup_rate', 'margin_rate']) {
    delete extra[key]
	}
  const fixedUnitPrice = normalizeOptionalNumber(rule.fixed_unit_price ?? rule.unit_price ?? rule.price_per_unit ?? rule.fixed_price)
  const costPlusRate = normalizeOptionalNumber(rule.cost_plus_rate ?? rule.markup_rate ?? rule.margin_rate)
	return {
		price_rule_pricing_mode: optionValue(rule.pricing_mode, priceListRulePricingModeOptions, 'inherit_gradient_template'),
		price_rule_fixed_unit_price: fixedUnitPrice === null ? '' : fixedUnitPrice,
		price_rule_cost_plus_percent: costPlusRate === null ? '' : Number((costPlusRate * 100).toFixed(4)),
    price_rule_rounding: optionValue(rule.rounding, priceListRuleRoundingOptions, 'none'),
    price_rule_tax_included: Boolean(rule.tax_included),
    price_rule_extra: extra,
  }
}

export function priceListRuleJSONFromForm(form = {}) {
	const out = sanitizeExtraObject(form.price_rule_extra)
	out.pricing_mode = optionValue(form.price_rule_pricing_mode, priceListRulePricingModeOptions, 'inherit_gradient_template')
  const fixedUnitPrice = normalizeOptionalNumber(form.price_rule_fixed_unit_price)
  if (out.pricing_mode === 'fixed_unit_price' && fixedUnitPrice !== null) {
    out.fixed_unit_price = trimDecimal(fixedUnitPrice)
  }
  const costPlusPercent = normalizeOptionalNumber(form.price_rule_cost_plus_percent)
  if (out.pricing_mode === 'cost_plus' && costPlusPercent !== null) {
    out.cost_plus_rate = trimDecimal(costPlusPercent / 100)
  }
  out.rounding = optionValue(form.price_rule_rounding, priceListRuleRoundingOptions, 'none')
  out.tax_included = Boolean(form.price_rule_tax_included)
  return JSON.stringify(out)
}

export function unitConversionRowsFromJSON(value = {}, defaultToUnit = '') {
  const conversion = parseJSONObject(value)
  const fallbackTargetUnit = normalizeOptionalUnitText(defaultToUnit)
  const rows = []
  for (const [fromUnit, targets] of Object.entries(conversion)) {
    const normalizedFromUnit = normalizeOptionalUnitText(fromUnit)
    if (!normalizedFromUnit) continue
    const directRatio = normalizePositiveNumber(targets)
    if (directRatio > 0 && fallbackTargetUnit) {
      rows.push({
        from_qty: 1,
        from_unit: normalizedFromUnit,
        to_qty: directRatio,
        to_unit: fallbackTargetUnit,
      })
      continue
    }
    const targetMap = parseJSONObject(targets)
    for (const [toUnit, ratio] of Object.entries(targetMap)) {
      const normalizedToUnit = normalizeOptionalUnitText(toUnit)
      const numericRatio = normalizePositiveNumber(ratio)
      if (!normalizedToUnit || numericRatio <= 0) continue
      rows.push({
        from_qty: 1,
        from_unit: normalizedFromUnit,
        to_qty: numericRatio,
        to_unit: normalizedToUnit,
      })
    }
  }
  return rows
}

export function unitConversionJSONFromRows(rows = []) {
  const out = {}
  for (const row of rows || []) {
    const fromQty = normalizePositiveNumber(row?.from_qty)
    const toQty = normalizePositiveNumber(row?.to_qty)
    const fromUnit = normalizeOptionalUnitText(row?.from_unit)
    const toUnit = normalizeOptionalUnitText(row?.to_unit)
    if (fromQty <= 0 || toQty <= 0 || !fromUnit || !toUnit) continue
    if (!out[fromUnit]) out[fromUnit] = {}
    out[fromUnit][toUnit] = trimDecimal(toQty / fromQty)
  }
  return JSON.stringify(out)
}

export function unitRuleFormFromJSON(value = {}) {
  const rule = parseJSONObject(value)
  const conversion = rule.unit_conversion_json ?? rule.conversion_json ?? {}
  const extra = { ...rule }
  for (const key of ['inventory_unit', 'quote_unit', 'order_unit', 'unit_conversion_json', 'conversion_json', 'integer_unit']) {
    delete extra[key]
  }
  const inventoryUnit = normalizeOptionalUnitText(rule.inventory_unit)
  return {
    inventory_unit: inventoryUnit,
    quote_unit: normalizeOptionalUnitText(rule.quote_unit),
    order_unit: normalizeOptionalUnitText(rule.order_unit),
    unit_conversion_rows: unitConversionRowsFromJSON(conversion, inventoryUnit),
    integer_unit_mode: integerUnitModeFromValue(rule.integer_unit),
    unit_rule_extra: extra,
  }
}

export function unitRuleJSONFromForm(form = {}) {
  const out = sanitizeExtraObject(form.unit_rule_extra)
  const inventoryUnit = normalizeOptionalUnitText(form.inventory_unit)
  const quoteUnit = normalizeOptionalUnitText(form.quote_unit)
  const orderUnit = normalizeOptionalUnitText(form.order_unit)
  if (inventoryUnit) out.inventory_unit = inventoryUnit
  if (quoteUnit) out.quote_unit = quoteUnit
  if (orderUnit) out.order_unit = orderUnit
  const conversionJSON = unitConversionJSONFromRows(form.unit_conversion_rows || [])
  const conversion = parseJSONObject(conversionJSON)
  if (Object.keys(conversion).length) out.unit_conversion_json = conversion
  const mode = String(form.integer_unit_mode || '').trim()
  if (mode === 'integer') out.integer_unit = true
  if (mode === 'decimal') out.integer_unit = false
  return JSON.stringify(out)
}

export function specialAttrSchemaRowsFromJSON(value = []) {
  const rows = parseJSONArray(value)
  return rows
    .map((row, index) => normalizeSpecialAttrSchemaRow(row, index + 1))
    .filter((row) => row.key)
    .sort((a, b) => Number(a.position || 0) - Number(b.position || 0) || a.key.localeCompare(b.key))
}

export function specialAttrSchemaJSONFromRows(rows = []) {
  const out = []
  const seen = new Set()
  for (const row of rows || []) {
    const normalized = normalizeSpecialAttrSchemaRow(row, out.length + 1)
    if (!normalized.key || seen.has(normalized.key)) continue
    seen.add(normalized.key)
    out.push({
      key: normalized.key,
      label: normalized.label,
      value_type: normalized.value_type,
      options: normalized.options,
      required: normalized.required,
      show_in_price_list: normalized.show_in_price_list,
      position: out.length + 1,
    })
  }
  return JSON.stringify(out)
}

export function specialAttrValuesFromJSON(value = {}) {
  const parsed = parseJSONObject(value)
  const out = {}
  for (const [key, raw] of Object.entries(parsed)) {
    const normalizedKey = normalizeSpecialAttrKey(key)
    if (!normalizedKey) continue
    const normalizedValue = normalizeSpecialAttrValue(raw)
    if (normalizedValue === '') continue
    out[normalizedKey] = normalizedValue
  }
  return out
}

export function specialAttrValuesJSONFromForm(value = {}) {
  const source = typeof value === 'string' ? specialAttrValuesFromJSON(value) : (value && typeof value === 'object' && !Array.isArray(value) ? value : {})
  const out = {}
  for (const [key, raw] of Object.entries(source)) {
    const normalizedKey = normalizeSpecialAttrKey(key)
    if (!normalizedKey) continue
    const normalizedValue = normalizeSpecialAttrValue(raw)
    if (normalizedValue === '') continue
    out[normalizedKey] = normalizedValue
  }
  return JSON.stringify(out)
}

function normalizeSpecialAttrSchemaRow(row = {}, fallbackPosition = 1) {
  const key = normalizeSpecialAttrKey(row?.key)
  const label = String(row?.label || key).trim()
  const valueType = ['text', 'select', 'number', 'boolean'].includes(String(row?.value_type || '').trim())
    ? String(row.value_type).trim()
    : 'text'
  const options = valueType === 'select' ? normalizeSpecialAttrOptions(row) : []
  return {
    key,
    label,
    value_type: valueType,
    options,
    options_text: options.join('\n'),
    required: Boolean(row?.required),
    show_in_price_list: Boolean(row?.show_in_price_list ?? row?.showInPriceList),
    position: Number(row?.position || fallbackPosition || 1),
  }
}

function normalizeSpecialAttrOptions(row = {}) {
  const source = Object.prototype.hasOwnProperty.call(row, 'options_text')
    ? row.options_text
    : row?.options
  if (Array.isArray(source)) {
    return source.map((item) => String(item || '').trim()).filter(Boolean)
  }
  return String(source || '')
    .split(/[\n,，;；]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function normalizeSpecialAttrKey(value) {
  return String(value || '').trim().replace(/\s+/g, '_')
}

function normalizeSpecialAttrValue(value) {
  if (value === null || typeof value === 'undefined') return ''
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  return String(value).trim()
}

function normalizeUnitText(value, fallback = 'kg') {
  const normalized = String(value || '').trim()
  if (normalized) return normalized
  return String(fallback || '').trim() || 'kg'
}

function normalizeOptionalUnitText(value) {
  return String(value || '').trim()
}

function normalizeInventoryConversionValue(value) {
  if (value && typeof value === 'object' && !Array.isArray(value)) return value
  return parseJSONObject(value)
}

function priceTableInventoryConversion(value = {}, priceUnit = '', inventoryUnit = '') {
  const parsed = normalizeInventoryConversionValue(value)
  if (Object.keys(parsed).length) return parsed
  const source = String(priceUnit || '').trim()
  const target = String(inventoryUnit || '').trim()
  if (!source || !target) return {}
  if (source === target) return { [source]: { [target]: 1 } }
  if ((source === 'lb' || source === '磅') && target === 'kg') return { lb: { kg: 0.454 } }
  if (source === 'kg' && (target === 'lb' || target === '磅')) return { kg: { lb: 2.20462 } }
  return {}
}

function formatProductFinalUnitPrice(value) {
  const n = Number(value || 0)
  if (!Number.isFinite(n)) return '0'
  return n.toFixed(4).replace(/\.?0+$/, '')
}

function normalizeJSONString(value) {
  const raw = String(value || '').trim()
  return raw || '{}'
}

function normalizeJSONArrayString(value) {
  const raw = String(value || '').trim()
  return raw || '[]'
}

function parseJSONObject(value) {
  if (!value) return {}
  if (typeof value === 'object' && !Array.isArray(value)) return value
  try {
    const parsed = JSON.parse(String(value || '{}'))
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function stableJSONObjectText(value = {}) {
  const normalized = stableJSONValue(value)
  return JSON.stringify(normalized)
}

function stableJSONValue(value) {
  if (Array.isArray(value)) return value.map(stableJSONValue)
  if (value && typeof value === 'object') {
    return Object.keys(value)
      .sort()
      .reduce((out, key) => {
        out[key] = stableJSONValue(value[key])
        return out
      }, {})
  }
  return value
}

function parseJSONArray(value) {
  if (!value) return []
  if (Array.isArray(value)) return value
  try {
    const parsed = JSON.parse(String(value || '[]'))
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function sanitizeExtraObject(value) {
  const parsed = parseJSONObject(value)
  return { ...parsed }
}

function optionValue(value, options = [], fallback = '') {
  const normalized = String(value || '').trim()
  return options.some((option) => option.value === normalized) ? normalized : fallback
}

function normalizePositiveNumber(value) {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) && numberValue > 0 ? numberValue : 0
}

function normalizeOptionalNumber(value) {
  if (value === null || typeof value === 'undefined' || value === '') return null
  const numberValue = Number(value)
  return Number.isFinite(numberValue) && numberValue >= 0 ? numberValue : null
}

function trimDecimal(value) {
  return Number(Number(value).toFixed(8))
}

function integerUnitModeFromValue(value) {
  if (typeof value === 'undefined' || value === null || value === '') return 'inherit'
  if (value === true) return 'integer'
  if (value === false) return 'decimal'
  const normalized = String(value).trim().toLowerCase()
  if (['true', '1', 'yes', 'integer'].includes(normalized)) return 'integer'
  if (['false', '0', 'no', 'decimal'].includes(normalized)) return 'decimal'
  return 'inherit'
}

function hasStructuredPriceRuleFields(row = {}) {
	return Object.prototype.hasOwnProperty.call(row, 'price_rule_pricing_mode')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_display_mode')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_fixed_unit_price')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_cost_plus_percent')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_rounding')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_tax_included')
}

function hasStructuredUnitRuleFields(row = {}) {
  return Object.prototype.hasOwnProperty.call(row, 'integer_unit_mode')
    || Object.prototype.hasOwnProperty.call(row, 'unit_conversion_rows')
}
