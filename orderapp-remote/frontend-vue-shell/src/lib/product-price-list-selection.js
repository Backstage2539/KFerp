export function priceListVisibleCategoryRows(categoryRows = []) {
  const rows = normalizeCategoryRows(categoryRows)
  const rowByItemID = new Map(rows.filter((row) => row.groupItemID > 0).map((row) => [row.groupItemID, row]))
  const includedItemIDs = new Set()
  const includedCodes = new Set()

  rows.forEach((row) => {
    if (!row.items.length) return
    includedCodes.add(row.code)
    if (row.groupItemID > 0) {
      includedItemIDs.add(row.groupItemID)
      for (const ancestor of ancestorRows(row, rowByItemID)) {
        includedItemIDs.add(ancestor.groupItemID)
        includedCodes.add(ancestor.code)
      }
    }
  })

  return rows
    .filter((row) => row.items.length > 0 || includedCodes.has(row.code) || (row.groupItemID > 0 && includedItemIDs.has(row.groupItemID)))
    .map((row) => row.source)
}

export function priceListCategoryProductIDs(categoryRows = [], category = null) {
  const rows = normalizeCategoryRows(categoryRows)
  const descendantCodes = new Set(priceListCategoryDescendantCodes(rows, category))
  const ids = []
  rows.forEach((row) => {
    if (!descendantCodes.has(row.code)) return
    row.items.forEach((item) => {
      const id = productIDOf(item)
      if (id && !ids.includes(id)) ids.push(id)
    })
  })
  return ids
}

export function priceListCategoryCodesForSelectedProducts(categoryRows = [], selectedProductIDs = []) {
  const selected = new Set(normalizeStringList(selectedProductIDs))
  if (!selected.size) return []
  return normalizeCategoryRows(categoryRows)
    .filter((row) => row.items.some((item) => selected.has(productIDOf(item))))
    .map((row) => row.code)
}

export function priceListCategoryHiddenByCollapsedAncestor(categoryRows = [], category = null, collapsedByKey = {}) {
  const rows = normalizeCategoryRows(categoryRows)
  const row = findCategoryRow(rows, category)
  if (!row || row.groupItemID <= 0) return false
  const rowByItemID = new Map(rows.filter((item) => item.groupItemID > 0).map((item) => [item.groupItemID, item]))
  return ancestorRows(row, rowByItemID).some((ancestor) => Boolean(collapsedByKey?.[ancestor.code]))
}

function priceListCategoryDescendantCodes(rows = [], category = null) {
  const row = findCategoryRow(rows, category)
  if (!row) {
    const code = categoryCode(category)
    return code ? [code] : []
  }
  if (row.groupItemID <= 0) return [row.code]
  const rowByItemID = new Map(rows.filter((item) => item.groupItemID > 0).map((item) => [item.groupItemID, item]))
  return rows
    .filter((candidate) => candidate.code === row.code || ancestorRows(candidate, rowByItemID).some((ancestor) => ancestor.groupItemID === row.groupItemID))
    .map((candidate) => candidate.code)
}

function ancestorRows(row, rowByItemID) {
  const ancestors = []
  const seen = new Set()
  let parentID = row.parentGroupItemID
  while (parentID > 0 && !seen.has(parentID)) {
    seen.add(parentID)
    const parent = rowByItemID.get(parentID)
    if (!parent) break
    ancestors.push(parent)
    parentID = parent.parentGroupItemID
  }
  return ancestors
}

function findCategoryRow(rows = [], category = null) {
  const code = categoryCode(category)
  const groupItemID = numberField(category?.group_item_id ?? category?.groupItemID ?? category)
  return rows.find((row) => (
    (code && row.code === code) ||
    (groupItemID > 0 && row.groupItemID === groupItemID)
  )) || null
}

function normalizeCategoryRows(categoryRows = []) {
  return (Array.isArray(categoryRows) ? categoryRows : []).map((row, index) => {
    const source = row || {}
    return {
      source,
      code: categoryCode(source) || String(index + 1),
      groupItemID: numberField(source.group_item_id ?? source.groupItemID),
      parentGroupItemID: numberField(source.parent_group_item_id ?? source.parentGroupItemID),
      items: Array.isArray(source.items) ? source.items : (Array.isArray(source.rows) ? source.rows : []),
    }
  })
}

function categoryCode(category = null) {
  if (typeof category === 'string' || typeof category === 'number') return String(category || '').trim()
  return String(category?.code || category?.key || category?.group_item_id || category?.groupItemID || '').trim()
}

function productIDOf(item = {}) {
  return String(item?.product_id ?? item?.productID ?? item?.productId ?? item?.id ?? item?.name ?? '').trim()
}

function normalizeStringList(values = []) {
  const raw = Array.isArray(values) ? values : String(values || '').split(/[\n,，]/)
  return raw.map((value) => String(value ?? '').trim()).filter(Boolean)
}

function numberField(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) ? n : 0
}
