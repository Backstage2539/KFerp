import { normalizePageSize } from './pagination.js'

export const UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID = -1
// Reserve a collision-free publication namespace while staying below Number.MAX_SAFE_INTEGER with ample room for group IDs.
export const PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE = 8_000_000_000_000_000
export const PUBLICATION_VERSION_PAGE_SIZE_OPTIONS = [5, 10, 20, 50, 100]

export function buildClassificationPriceListTypeOptions(sourceItems = []) {
  const seen = new Map()
  const unclassifiedItems = []
  ;(Array.isArray(sourceItems) ? sourceItems : []).forEach((item) => {
    const id = classificationTemplateIDOfItem(item)
    const label = classificationTemplateNameOfItem(item)
    if (!(id > 0) || !label) {
      unclassifiedItems.push(item)
      return
    }
    const listType = priceListRenderTypeForItem(item)
    const key = `classification-template:${id}`
    const current = seen.get(key) || {
      id,
      categoryID: id,
      key,
      label,
      listType,
      position: productTypePositionOfItem(item),
      itemCount: 0,
    }
    current.itemCount += 1
    current.position = Math.min(current.position || 999999, productTypePositionOfItem(item))
    seen.set(key, current)
  })
  if (unclassifiedItems.length) {
    seen.set('classification:unclassified', {
      id: UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID,
      categoryID: 0,
      key: 'classification:unclassified',
      label: '未分类商品',
      listType: dominantPriceListRenderType(unclassifiedItems),
      position: -1,
      itemCount: unclassifiedItems.length,
    })
  }
  return Array.from(seen.values())
    .sort((a, b) => {
      const positionDelta = Number(a.position || 999999) - Number(b.position || 999999)
      if (positionDelta !== 0) return positionDelta
      return String(a.label || '').localeCompare(String(b.label || ''), 'zh-Hans-CN')
    })
}

export function buildProductCatalogPriceListTypeOptions(sourceItems = [], {
  template = null,
  assignments = [],
} = {}) {
  const templateID = numberField(template?.id)
  if (!(templateID > 0)) return []
  const roots = topLevelBusinessGroupItems(template?.items || [])
  if (!roots.length) return []
  const rootByItemID = new Map()
  roots.forEach((root, index) => {
    businessGroupDescendantIDs(root).forEach((id) => {
      rootByItemID.set(id, { root, index })
    })
  })
  const groups = new Map()
  ;(Array.isArray(sourceItems) ? sourceItems : []).forEach((item) => {
    const assignment = productCatalogAssignmentForItem(item, assignments, templateID)
    const groupItemID = numberField(assignment?.group_item_id ?? assignment?.groupItemID)
    const match = rootByItemID.get(groupItemID)
    if (!match) return
    const rootID = numberField(match.root.id)
    const key = `product-catalog:${templateID}:${rootID}`
    const current = groups.get(key) || {
      id: productCatalogPriceListTypeID(rootID),
      categoryID: 0,
      key,
      label: stringField(match.root.name) || `分组 ${rootID}`,
      listType: priceListRenderTypeForItem(item),
      position: numberField(match.root.sort_order ?? match.root.sortOrder) || (match.index + 1) * 10,
      itemCount: 0,
      productCatalogGroupID: templateID,
      productCatalogGroupItemID: rootID,
      productCatalogGroupItemIDs: businessGroupDescendantIDs(match.root),
    }
    current.itemCount += 1
    current.listType = dominantPriceListRenderType([
      ...((current._items) || []),
      item,
    ])
    current._items = [...((current._items) || []), item]
    groups.set(key, current)
  })
  return Array.from(groups.values())
    .map(({ _items, ...option }) => option)
    .sort((a, b) => {
      const positionDelta = Number(a.position || 999999) - Number(b.position || 999999)
      if (positionDelta !== 0) return positionDelta
      return String(a.label || '').localeCompare(String(b.label || ''), 'zh-Hans-CN')
    })
}

export function buildProductCatalogTemplatePriceListTypeOptions(sourceItems = [], {
  templates = [],
  assignments = [],
} = {}) {
  const rows = Array.isArray(sourceItems) ? sourceItems : []
  const activeTemplates = (Array.isArray(templates) ? templates : [])
    .filter((template) => template?.active !== false)
    .filter((template) => !isSystemDefaultBusinessGroup(template))
    .filter((template) => numberField(template?.id) > 0)
  if (!activeTemplates.length) {
    return [{
      id: 0,
      categoryID: 0,
      key: 'product-catalog:flat',
      label: '全部商品',
      listType: dominantPriceListRenderType(rows),
      position: 0,
      itemCount: uniqueProductCount(rows),
      productCatalogFlat: true,
      productCatalogScopeGroups: [],
      publicationProductTypeCategoryID: 0,
      publicationClassificationTemplateID: 0,
    }]
  }

  const scopeGroups = activeTemplates.map((template) => ({
    groupID: numberField(template.id),
    groupItemIDs: businessGroupDescendantIDsForTemplate(template),
  }))
  return activeTemplates.map((template, index) => {
    const templateID = numberField(template.id)
    const publicationTypeID = productCatalogPublicationTypeID(templateID)
    const matchedItems = rows.filter((item) => {
      const assignment = productCatalogAssignmentForItemInScope(item, assignments, scopeGroups)
      return numberField(assignment?.group_id ?? assignment?.groupID) === templateID
    })
    return {
      id: productCatalogTemplatePriceListTypeID(templateID),
      categoryID: 0,
      key: `product-catalog:${templateID}`,
      label: stringField(template.name) || `分组模板 ${templateID}`,
      listType: dominantPriceListRenderType(matchedItems),
      position: numberField(template.sort_order ?? template.sortOrder) || (index + 1) * 10,
      itemCount: uniqueProductCount(matchedItems),
      productCatalogGroupID: templateID,
      productCatalogGroupItemIDs: businessGroupDescendantIDsForTemplate(template),
      productCatalogScopeGroups: scopeGroups,
      publicationProductTypeCategoryID: publicationTypeID,
      publicationClassificationTemplateID: publicationTypeID,
    }
  })
}

export function matchesProductCatalogPriceListType(item = {}, type = {}, {
  assignments = [],
} = {}) {
  if (type?.productCatalogFlat === true || type?.product_catalog_flat === true) return true
  const scopeGroups = normalizeProductCatalogScopeGroups(type)
  const scopedAssignment = productCatalogAssignmentForItemInScope(item, assignments, scopeGroups)
  const groupID = numberField(type?.productCatalogGroupID ?? type?.product_catalog_group_id)
  if (!(groupID > 0)) return false
  const groupItemIDs = new Set((type?.productCatalogGroupItemIDs || type?.product_catalog_group_item_ids || [])
    .map((id) => numberField(id))
    .filter(Boolean))
  if (!groupItemIDs.size) return false
  const assignment = scopeGroups.length
    ? scopedAssignment
    : productCatalogAssignmentForItem(item, assignments, groupID)
  if (numberField(assignment?.group_id ?? assignment?.groupID) !== groupID) return false
  const groupItemID = numberField(assignment?.group_item_id ?? assignment?.groupItemID)
  return groupItemIDs.has(groupItemID)
}

export function priceListSelectionStateKey(typeOptions = [], listType = 'commercial', productTypeCategoryID = 0) {
  const selectedID = numberField(productTypeCategoryID)
  const selected = (Array.isArray(typeOptions) ? typeOptions : [])
    .find((type) => numberField(type?.id) === selectedID || numberField(type?.categoryID) === selectedID)
  const explicitKey = stringField(selected?.key)
  if (explicitKey.startsWith('product-catalog:')) return explicitKey
  const id = numberField(selected?.categoryID ?? selected?.id ?? productTypeCategoryID)
  if (id === UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID) return 'classification:unclassified'
  if (id > 0) return `product-type:${id}`
  return `legacy:${normalizeBeanListTypeForKey(selected?.listType || listType)}`
}

export function classificationTemplateIDOfItem(item = {}) {
  const currentID = Number(
    item?.classification_template_id ||
    item?.classificationTemplateID ||
    item?.current_classification_template_id ||
    item?.currentClassificationTemplateID ||
    item?.classification_template_id_snapshot ||
    0,
  )
  if (currentID > 0) return currentID
  if (directProductCategoryIDOfItem(item) <= 0) return 0
  return directProductTypeCategoryIDOfItem(item) || directProductCategoryIDOfItem(item)
}

export function classificationCategoryIDOfItem(item = {}) {
  const currentID = Number(
    item?.classification_category_id ||
    item?.classificationCategoryID ||
    item?.current_classification_category_id ||
    item?.currentClassificationCategoryID ||
    item?.classification_category_id_snapshot ||
    0,
  )
  if (currentID > 0) return currentID
  const categoryID = directProductCategoryIDOfItem(item)
  if (categoryID <= 0) return 0
  const subtypeID = Number(item?.product_subtype_category_id || item?.productSubtypeCategoryID || 0)
  if (subtypeID > 0) return subtypeID
  const typeID = directProductTypeCategoryIDOfItem(item)
  return typeID > 0 && categoryID === typeID ? 0 : categoryID
}

export function classificationTemplateNameOfItem(item = {}) {
  const currentName = stringField(
    item?.classification_template_name ??
    item?.classificationTemplateName ??
    item?.current_classification_template_name ??
    item?.currentClassificationTemplateName ??
    item?.classification_template_name_snapshot,
  )
  if (currentName) return currentName
  if (directProductCategoryIDOfItem(item) <= 0) return ''
  return stringField(item?.category_primary_name ?? item?.categoryPrimaryName ?? item?.product_type_name ?? item?.productTypeName)
}

export function classificationCategoryNameOfItem(item = {}) {
  const currentName = stringField(
    item?.classification_category_name ??
    item?.classificationCategoryName ??
    item?.current_classification_category_name ??
    item?.currentClassificationCategoryName ??
    item?.classification_category_name_snapshot,
  )
  if (currentName) return currentName
  if (directProductCategoryIDOfItem(item) <= 0) return ''
  return stringField(item?.category_secondary_name ?? item?.categorySecondaryName ?? item?.product_subtype_name ?? item?.productSubtypeName)
}

export function productTypeCategoryIDOfItem(item = {}) {
  return classificationTemplateIDOfItem(item)
}

export function productTypeNameOfItem(item = {}) {
  return classificationTemplateNameOfItem(item)
}

export function matchesProductTypeCategory(item = {}, productTypeCategoryID = 0) {
  const id = Number(productTypeCategoryID || 0)
  if (id === UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID) return classificationTemplateIDOfItem(item) <= 0
  if (id <= 0) return true
  return classificationTemplateIDOfItem(item) === id
}

export function classificationTemplateIDOfPublication(publication = {}) {
  const direct = Number(
    publication?.classification_template_id ||
    publication?.classificationTemplateID ||
    publication?.classification_template_id_snapshot ||
    0,
  )
  if (direct > 0) return direct
  const ids = new Set(publicationItems(publication).map((item) => classificationTemplateIDOfItem(item)).filter((id) => id > 0))
  return ids.size === 1 ? Array.from(ids)[0] : 0
}

export function classificationTemplateNameOfPublication(publication = {}) {
  const direct = stringField(
    publication?.classification_template_name ??
    publication?.classificationTemplateName ??
    publication?.classification_template_name_snapshot,
  )
  if (direct) return direct
  const names = new Set(publicationItems(publication).map((item) => classificationTemplateNameOfItem(item)).filter(Boolean))
  return names.size === 1 ? Array.from(names)[0] : ''
}

export function publicationTypeIdentityForPriceListType(type = {}) {
  const explicitProductTypeID = numberField(type?.publicationProductTypeCategoryID ?? type?.publication_product_type_category_id)
  const explicitClassificationID = numberField(type?.publicationClassificationTemplateID ?? type?.publication_classification_template_id)
  const productCatalogGroupID = numberField(type?.productCatalogGroupID ?? type?.product_catalog_group_id)
  const productCatalogPublicationID = productCatalogPublicationTypeID(productCatalogGroupID)
  const fallbackID = numberField(type?.categoryID ?? type?.id)
  const legacyID = fallbackID > 0 ? fallbackID : 0
  const productTypeCategoryID = explicitProductTypeID || productCatalogPublicationID || legacyID
  return {
    productTypeCategoryID,
    classificationTemplateID: explicitClassificationID || productCatalogPublicationID || productTypeCategoryID,
  }
}

export function publicationTypeIdentityOfPublication(publication = {}) {
  const productTypeCategoryID = numberField(publication?.product_type_category_id ?? publication?.productTypeCategoryID)
  const classificationTemplateID = classificationTemplateIDOfPublication(publication)
  return {
    productTypeCategoryID,
    classificationTemplateID: classificationTemplateID || productTypeCategoryID,
  }
}

export function priceListTypeOptionForPublication(typeOptions = [], publication = {}) {
  const publicationIdentity = publicationTypeIdentityOfPublication(publication)
  return (Array.isArray(typeOptions) ? typeOptions : []).find((type) => {
    const typeIdentity = publicationTypeIdentityForPriceListType(type)
    if (publicationIdentity.classificationTemplateID > 0 && typeIdentity.classificationTemplateID === publicationIdentity.classificationTemplateID) return true
    return publicationIdentity.productTypeCategoryID > 0 && typeIdentity.productTypeCategoryID === publicationIdentity.productTypeCategoryID
  }) || null
}

export function preferredPublicationForPriceListType(rows = [], type = {}) {
  const publishedRows = (Array.isArray(rows) ? rows : []).filter((row) => String(row?.status || '') === 'published')
  const exact = publishedRows.find((row) => priceListTypeOptionForPublication([type], row))
  return exact || publishedRows[0] || null
}

export function matchesPublicationProductType(publication = {}, productTypeCategoryID = 0) {
  const type = productTypeCategoryID && typeof productTypeCategoryID === 'object' ? productTypeCategoryID : null
  const rawID = type ? numberField(type?.categoryID ?? type?.id) : Number(productTypeCategoryID || 0)
  const identity = type
    ? publicationTypeIdentityForPriceListType(type)
    : { productTypeCategoryID: rawID > 0 ? rawID : 0, classificationTemplateID: rawID > 0 ? rawID : 0 }
  const classificationID = classificationTemplateIDOfPublication(publication)
  const storedProductTypeID = numberField(publication?.product_type_category_id ?? publication?.productTypeCategoryID)
  if (rawID === UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID) return classificationID <= 0
  if (identity.classificationTemplateID <= 0 && identity.productTypeCategoryID <= 0) return true
  if (classificationID > 0 && classificationID === identity.classificationTemplateID) return true
  if (classificationID <= 0 && storedProductTypeID > 0 && storedProductTypeID === identity.productTypeCategoryID) return true
  return isLegacyGlobalCommercialPublication(publication, classificationID)
}

function isLegacyGlobalCommercialPublication(publication = {}, classificationID = classificationTemplateIDOfPublication(publication)) {
  const storedProductTypeID = Number(publication?.product_type_category_id || publication?.productTypeCategoryID || 0)
  const listType = stringField((publication?.list_type ?? publication?.listType) || 'commercial')
  return storedProductTypeID <= 0 && classificationID <= 0 && listType === 'commercial'
}

export function publicationVersionListState(rows = [], options = {}) {
  const sourceRows = Array.isArray(rows) ? rows : []
  const query = stringField(options.query).toLowerCase()
  const terms = query.split(/\s+/).filter(Boolean)
  const filteredRows = terms.length
    ? sourceRows.filter((row) => {
      const haystack = publicationSearchText(row)
      return terms.every((term) => haystack.includes(term))
    })
    : sourceRows
  const pageSize = normalizePageSize(options.pageSize, PUBLICATION_VERSION_PAGE_SIZE_OPTIONS)
  const totalPages = Math.max(1, Math.ceil(filteredRows.length / pageSize))
  const page = Math.min(Math.max(Number.parseInt(String(options.page || 1), 10) || 1, 1), totalPages)
  const start = (page - 1) * pageSize
  const collapsed = Boolean(options.collapsed)
  return {
    query,
    collapsed,
    total: filteredRows.length,
    page,
    pageSize,
    totalPages,
    rows: collapsed ? [] : filteredRows.slice(start, start + pageSize),
  }
}

export function priceListRenderTypeForItem(item = {}) {
  const subtypeName = String(classificationCategoryNameOfItem(item) || item?.product_subtype_name || item?.productSubtypeName || '').trim()
  const typeName = String(classificationTemplateNameOfItem(item) || '').trim()
  const kind = String(item?.product_kind || item?.productKind || '').trim().toLowerCase()

  const categoryHint = (subtypeName + typeName).toLowerCase()
  if (categoryHint.includes('生豆') || categoryHint.includes('green')) return 'green'
  if (categoryHint.includes('零售') || categoryHint.includes('retail')) return 'retail'

  if (kind === 'green_bean') return 'green'
  return 'commercial'
}

function dominantPriceListRenderType(items = []) {
  const counts = new Map()
  ;(Array.isArray(items) ? items : []).forEach((item) => {
    const type = priceListRenderTypeForItem(item)
    counts.set(type, (counts.get(type) || 0) + 1)
  })
  if (counts.size === 1) return Array.from(counts.keys())[0] || 'commercial'
  return 'commercial'
}

function productTypePositionOfItem(item = {}) {
  return Number(item?.category_primary_position || item?.product_type_position || item?.productTypePosition || 999999)
}

function productCatalogPriceListTypeID(groupItemID) {
  return -1000000 - numberField(groupItemID)
}

function productCatalogTemplatePriceListTypeID(groupID) {
  return -2000000 - numberField(groupID)
}

function productCatalogPublicationTypeID(groupID) {
  const normalizedGroupID = numberField(groupID)
  if (!(normalizedGroupID > 0) || !Number.isSafeInteger(normalizedGroupID)) return 0
  const id = PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE + normalizedGroupID
  return Number.isSafeInteger(id) ? id : 0
}

function uniqueProductCount(items = []) {
  const ids = new Set()
  let missingIDCount = 0
  ;(Array.isArray(items) ? items : []).forEach((item) => {
    const id = productCatalogObjectIDOfItem(item)
    if (id > 0) ids.add(id)
    else missingIDCount += 1
  })
  return ids.size + missingIDCount
}

function productCatalogObjectIDOfItem(item = {}) {
  const effectiveParentID = numberField(item?.effective_parent_product_id ?? item?.effectiveParentProductID)
  if (effectiveParentID > 0) return effectiveParentID
  const parentID = numberField(item?.parent_product_id ?? item?.parentProductID)
  if (parentID > 0) return parentID
  return numberField(item?.product_id ?? item?.productID ?? item?.id)
}

function businessGroupDescendantIDsForTemplate(template = {}) {
  return topLevelBusinessGroupItems(template?.items || [])
    .flatMap((root) => businessGroupDescendantIDs(root))
}

function normalizeProductCatalogScopeGroups(type = {}) {
  const source = Array.isArray(type?.productCatalogScopeGroups)
    ? type.productCatalogScopeGroups
    : (Array.isArray(type?.product_catalog_scope_groups) ? type.product_catalog_scope_groups : [])
  return source.map((group) => ({
    groupID: numberField(group?.groupID ?? group?.group_id),
    groupItemIDs: (Array.isArray(group?.groupItemIDs) ? group.groupItemIDs : (Array.isArray(group?.group_item_ids) ? group.group_item_ids : []))
      .map((id) => numberField(id))
      .filter(Boolean),
  })).filter((group) => group.groupID > 0)
}

function productCatalogAssignmentForItemInScope(item = {}, assignments = [], scopeGroups = []) {
  for (const scope of Array.isArray(scopeGroups) ? scopeGroups : []) {
    const groupID = numberField(scope?.groupID ?? scope?.group_id)
    if (!(groupID > 0)) continue
    const assignment = productCatalogAssignmentForItem(item, assignments, groupID)
    if (!assignment) continue
    const allowedItemIDs = new Set((scope?.groupItemIDs || scope?.group_item_ids || [])
      .map((id) => numberField(id))
      .filter(Boolean))
    const groupItemID = numberField(assignment?.group_item_id ?? assignment?.groupItemID)
    if (allowedItemIDs.has(groupItemID)) return assignment
  }
  return null
}

function productCatalogAssignmentForItem(item = {}, assignments = [], templateID = 0) {
  const productID = productCatalogObjectIDOfItem(item)
  if (!(productID > 0)) return null
  return (Array.isArray(assignments) ? assignments : []).find((assignment) => (
    stringField(assignment?.usage_key ?? assignment?.usageKey) === 'product_catalog' &&
    stringField(assignment?.object_key ?? assignment?.objectKey) === 'product' &&
    assignment?.active !== false &&
    numberField(assignment?.group_id ?? assignment?.groupID) === numberField(templateID) &&
    numberField(assignment?.object_id ?? assignment?.objectID) === productID
  )) || null
}

function topLevelBusinessGroupItems(items = []) {
  return businessGroupItemsTreeForPriceList(items)
}

function businessGroupDescendantIDs(root = {}) {
  return flattenBusinessGroupItems([root]).map((item) => numberField(item.id)).filter(Boolean)
}

function flattenBusinessGroupItems(items = []) {
  const out = []
  const visit = (item = {}, parentID = 0) => {
    const normalized = {
      ...item,
      parent_id: numberField(item.parent_id ?? item.parentID) || parentID,
    }
    out.push(normalized)
    ;(Array.isArray(item.children) ? item.children : []).forEach((child) => visit(child, numberField(item.id)))
  }
  ;(Array.isArray(items) ? items : []).forEach((item) => visit(item))
  return out
}

function businessGroupItemsTreeForPriceList(items = []) {
  const flat = flattenBusinessGroupItems(items)
    .filter((item) => item?.active !== false)
    .map((item) => ({
      ...item,
      id: numberField(item.id),
      parent_id: numberField(item.parent_id ?? item.parentID),
      sort_order: numberField(item.sort_order ?? item.sortOrder ?? item.position) || 100,
      children: [],
    }))
    .filter((item) => item.id > 0)
  const byID = new Map(flat.map((item) => [item.id, item]))
  const roots = []
  flat.forEach((item) => {
    const parent = byID.get(item.parent_id)
    if (parent && parent.id !== item.id) {
      parent.children.push(item)
    } else {
      roots.push(item)
    }
  })
  const sortRows = (rows = []) => {
    rows.sort((a, b) => numberField(a.sort_order ?? a.sortOrder) - numberField(b.sort_order ?? b.sortOrder) || numberField(a.id) - numberField(b.id))
    rows.forEach((row) => sortRows(row.children || []))
    return rows
  }
  return sortRows(roots)
}

function isSystemDefaultBusinessGroup(group = {}) {
  const code = stringField(group?.code).toLowerCase()
  if (code.startsWith('default_')) return true
  return ['商品默认分组', '生产 BOM 默认分组', '仓库库存默认分组'].includes(stringField(group?.name))
}

function numberField(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) ? n : 0
}

function normalizeBeanListTypeForKey(value) {
  if (value === 'green' || value === 'green_bean') return 'green'
  if (value === 'drip') return 'drip'
  if (value === 'retail') return 'retail'
  return 'commercial'
}

function directProductCategoryIDOfItem(item = {}) {
  return Number(item?.product_category_id || item?.productCategoryID || 0)
}

function directProductTypeCategoryIDOfItem(item = {}) {
  return Number(item?.product_type_category_id || item?.productTypeCategoryID || 0)
}

function publicationItems(publication = {}) {
  const groups = Array.isArray(publication?.content?.groups) ? publication.content.groups : []
  return groups.flatMap((group) => (Array.isArray(group?.items) ? group.items : []))
}

function publicationSearchText(row = {}) {
  return [
    row.id,
    row.version,
    row.version_no,
    row.changelog,
    row.owner_type,
    row.owner_key,
    row.status,
    row.publication_purpose,
    row.source_version,
    row.price_source_publication_id,
    row.style_source_publication_id,
    classificationTemplateNameOfPublication(row),
  ].map((item) => String(item || '').trim().toLowerCase()).filter(Boolean).join(' ')
}

function stringField(value) {
  return String(value || '').trim()
}
