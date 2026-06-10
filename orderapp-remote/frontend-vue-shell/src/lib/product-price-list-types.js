import { normalizePageSize } from './pagination.js'

export const UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID = -1
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

export function matchesProductCatalogPriceListType(item = {}, type = {}, {
  assignments = [],
} = {}) {
  const groupID = numberField(type?.productCatalogGroupID ?? type?.product_catalog_group_id)
  if (!(groupID > 0)) return false
  const groupItemIDs = new Set((type?.productCatalogGroupItemIDs || type?.product_catalog_group_item_ids || [])
    .map((id) => numberField(id))
    .filter(Boolean))
  if (!groupItemIDs.size) return false
  const assignment = productCatalogAssignmentForItem(item, assignments, groupID)
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

export function matchesPublicationProductType(publication = {}, productTypeCategoryID = 0) {
  const id = Number(productTypeCategoryID || 0)
  const classificationID = classificationTemplateIDOfPublication(publication)
  if (id === UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID) return classificationID <= 0
  if (id <= 0) return true
  if (classificationID === id) return true
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

function productCatalogAssignmentForItem(item = {}, assignments = [], templateID = 0) {
  const productID = numberField(item?.product_id ?? item?.productID ?? item?.id)
  if (!(productID > 0)) return null
  return (Array.isArray(assignments) ? assignments : []).find((assignment) => (
    stringField(assignment?.usage_key ?? assignment?.usageKey) === 'product_catalog' &&
    stringField(assignment?.object_key ?? assignment?.objectKey) === 'product' &&
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
