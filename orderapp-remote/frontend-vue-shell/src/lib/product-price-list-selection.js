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
    .filter((row) => row.items.some((item) => selectionIDsOf(item).some((id) => selected.has(id))))
    .map((row) => row.code)
}

export function buildPriceListProductFamilies(items = []) {
  const groups = new Map()
  ;(Array.isArray(items) ? items : []).forEach((item, index) => {
    const skuID = priceListSkuID(item)
    const parentProductID = priceListParentProductID(item)
    if (!(skuID > 0) || !(parentProductID > 0)) return
    if (!groups.has(parentProductID)) groups.set(parentProductID, [])
    groups.get(parentProductID).push({ item, index, skuID })
  })

  return Array.from(groups.entries()).map(([parentProductID, entries]) => {
    const parentEntry = entries.find(({ item, skuID }) => (
      skuID === parentProductID && directParentProductID(item) <= 0
    )) || entries.find(({ skuID }) => skuID === parentProductID) || entries[0]
    const parentItem = parentEntry?.item || {}
    const childEntries = entries.filter(({ item, skuID }) => (
      skuID !== parentProductID && directParentProductID(item) === parentProductID && isSelectableSpec(item)
    ))
    const selectableEntries = childEntries.length
      ? childEntries
      : entries.filter(({ item }) => isSelectableSpec(item))
    const explicitDefaultSkuID = firstPositiveNumber(
      parentItem?.default_sku_id,
      parentItem?.defaultSkuID,
      ...entries.flatMap(({ item }) => [item?.default_sku_id, item?.defaultSkuID]),
    )
    const parentDefaultUnit = normalizedSpecText(
      parentItem?.default_sales_unit ?? parentItem?.defaultSalesUnit ?? parentItem?.derived_sales_unit ?? parentItem?.derivedSalesUnit,
    )
    const matchingDefaultEntry = selectableEntries.find(({ skuID }) => skuID === explicitDefaultSkuID)
      || selectableEntries.find(({ item }) => (
        (item?.is_default_sku === true || item?.isDefaultSKU === true) &&
        (!parentDefaultUnit || normalizedSpecText(priceListProductSpecLabel(item)) === parentDefaultUnit)
      ))
      || selectableEntries.find(({ item }) => item?.is_default_sku === true || item?.isDefaultSKU === true)
      || selectableEntries.find(({ item }) => parentDefaultUnit && normalizedSpecText(priceListProductSpecLabel(item)) === parentDefaultUnit)
      || selectableEntries[0]
    const defaultSkuID = Number(matchingDefaultEntry?.skuID || 0)
    const skuOptions = selectableEntries
      .slice()
      .sort((a, b) => {
        if (a.skuID === defaultSkuID && b.skuID !== defaultSkuID) return -1
        if (b.skuID === defaultSkuID && a.skuID !== defaultSkuID) return 1
        const positionDelta = specPosition(a.item) - specPosition(b.item)
        if (positionDelta !== 0) return positionDelta
        return a.index - b.index
      })
      .map(({ item, skuID }) => ({
        ...item,
        sku_id: skuID,
        parent_product_id: parentProductID,
        effective_parent_product_id: parentProductID,
        __price_list_parent_product_id: parentProductID,
        __price_list_parent_product_name: String(parentItem?.name || parentItem?.product_name || item?.name || '').trim(),
      }))
    const name = String(
      parentItem?.customer_product_display_name ??
      parentItem?.customerProductDisplayName ??
      parentItem?.name ??
      parentItem?.product_name ??
      '',
    ).trim()
    return {
      ...parentItem,
      product_id: parentProductID,
      sku_id: parentProductID,
      parent_product_id: parentProductID,
      effective_parent_product_id: parentProductID,
      name,
      parent_product_name: name,
      product_key: `product:${parentProductID}`,
      default_sku_id: defaultSkuID,
      parent_item: parentItem,
      sku_options: skuOptions,
      __price_list_product_family: true,
    }
  })
}

export function priceListSkuID(item = {}) {
  return numberField(item?.sku_id ?? item?.skuID ?? item?.skuId ?? item?.product_id ?? item?.productID ?? item?.productId ?? item?.id)
}

export function priceListParentProductID(item = {}) {
  const annotated = numberField(item?.__price_list_parent_product_id)
  if (annotated > 0) return annotated
  const effective = numberField(item?.effective_parent_product_id ?? item?.effectiveParentProductID ?? item?.effectiveParentProductId)
  if (effective > 0) return effective
  const direct = directParentProductID(item)
  if (direct > 0) return direct
  return priceListSkuID(item)
}

export function priceListProductSpecLabel(item = {}) {
  const label = String(
    item?.derived_sales_unit ??
    item?.derivedSalesUnit ??
    item?.sku_name ??
    item?.skuName ??
    item?.spec_label ??
    item?.specLabel ??
    item?.default_sales_unit ??
    item?.defaultSalesUnit ??
    '',
  ).trim()
  return label || `SKU-${String(priceListSkuID(item)).padStart(6, '0')}`
}

export function defaultPriceListProductSpecSelections(families = []) {
  return (Array.isArray(families) ? families : []).flatMap((family) => {
    const parentProductID = priceListParentProductID(family)
    const defaultSkuID = numberField(family?.default_sku_id ?? family?.defaultSkuID)
    if (!(parentProductID > 0) || !(defaultSkuID > 0)) return []
    return [{
      parent_product_id: parentProductID,
      sku_id: defaultSkuID,
      selection_source: 'product_default',
      default_sku_id_at_selection: defaultSkuID,
    }]
  })
}

export function normalizePriceListProductSpecSelections(selections = [], families = [], options = {}) {
  void options
  const familyRows = Array.isArray(families) ? families : []
  const familyByParentID = new Map(familyRows.map((family) => [priceListParentProductID(family), family]))
  const familyBySkuID = new Map()
  familyRows.forEach((family) => {
    ;(Array.isArray(family?.sku_options) ? family.sku_options : []).forEach((spec) => {
      familyBySkuID.set(priceListSkuID(spec), family)
    })
  })
  const out = []
  const seen = new Set()
  ;(Array.isArray(selections) ? selections : []).forEach((selection) => {
    const requestedSkuID = numberField(selection?.sku_id ?? selection?.skuID)
    const requestedParentID = numberField(selection?.parent_product_id ?? selection?.parentProductID)
    const family = familyByParentID.get(requestedParentID) || familyBySkuID.get(requestedSkuID)
    if (!family) return
    const parentProductID = priceListParentProductID(family)
    const defaultSkuID = numberField(family.default_sku_id)
    const selectionSource = String(selection?.selection_source || selection?.selectionSource || '').trim() === 'product_default'
      ? 'product_default'
      : 'explicit'
    const skuID = requestedSkuID
    const defaultSkuIDAtSelection = numberField(selection?.default_sku_id_at_selection ?? selection?.defaultSkuIDAtSelection)
      || (selectionSource === 'product_default' ? requestedSkuID : defaultSkuID)
    const validSkuIDs = new Set((family.sku_options || []).map((spec) => priceListSkuID(spec)).filter(Boolean))
    const key = `${parentProductID}:${skuID}`
    if (seen.has(key)) return
    seen.add(key)
    const row = {
      parent_product_id: parentProductID,
      sku_id: skuID,
      selection_source: selectionSource,
      default_sku_id_at_selection: defaultSkuIDAtSelection,
    }
    if (!validSkuIDs.has(skuID)) {
      row.selection_issue = 'invalid_spec'
      row.current_default_sku_id = defaultSkuID
    } else if (selectionSource === 'product_default' && defaultSkuID > 0 && defaultSkuIDAtSelection !== defaultSkuID) {
      row.selection_issue = 'default_changed'
      row.current_default_sku_id = defaultSkuID
    }
    out.push(row)
  })
  return sortSelectionsByFamilies(out, familyRows)
}

export function priceListProductSpecSelectionIssue(family = {}, selections = []) {
  const parentProductID = priceListParentProductID(family)
  const selection = (Array.isArray(selections) ? selections : []).find((row) => (
    numberField(row?.parent_product_id ?? row?.parentProductID) === parentProductID
    && String(row?.selection_issue || '').trim()
  ))
  if (!selection) return null
  const type = String(selection.selection_issue || '').trim()
  const currentDefaultSkuID = numberField(selection.current_default_sku_id ?? family?.default_sku_id)
  const currentDefaultSpec = (family.sku_options || []).find((spec) => priceListSkuID(spec) === currentDefaultSkuID)
  return {
    type,
    selection,
    current_default_sku_id: currentDefaultSkuID,
    message: type === 'default_changed'
      ? `商品默认规格已变更为“${priceListProductSpecLabel(currentDefaultSpec || {})}”，请选择保留原规格或切换默认规格。`
      : '草稿中的销售规格已停用、被模板移除或不属于当前商品，请切换到当前默认规格。',
  }
}

export function resolvePriceListProductSpecSelectionIssue(selections = [], family = {}, action = 'switch') {
  const parentProductID = priceListParentProductID(family)
  const issue = priceListProductSpecSelectionIssue(family, selections)
  if (!issue) return Array.isArray(selections) ? selections.slice() : []
  const issueSkuID = numberField(issue.selection?.sku_id)
  const currentDefaultSkuID = numberField(issue.current_default_sku_id ?? family?.default_sku_id)
  const source = Array.isArray(selections) ? selections : []
  const issueIndex = source.findIndex((row) => (
    numberField(row?.parent_product_id ?? row?.parentProductID) === parentProductID
    && numberField(row?.sku_id ?? row?.skuID) === issueSkuID
  ))
  if (action === 'keep' && issue.type === 'default_changed') {
    return source.map((row, index) => index === issueIndex ? {
      parent_product_id: parentProductID,
      sku_id: issueSkuID,
      selection_source: 'explicit',
      default_sku_id_at_selection: currentDefaultSkuID,
    } : row)
  }
  const defaultAlreadySelected = source.some((row, index) => index !== issueIndex && (
    numberField(row?.parent_product_id ?? row?.parentProductID) === parentProductID
    && numberField(row?.sku_id ?? row?.skuID) === currentDefaultSkuID
  ))
  if (!(currentDefaultSkuID > 0) || defaultAlreadySelected) return source.filter((_, index) => index !== issueIndex)
  return source.map((row, index) => index === issueIndex ? {
    parent_product_id: parentProductID,
    sku_id: currentDefaultSkuID,
    selection_source: 'product_default',
    default_sku_id_at_selection: currentDefaultSkuID,
  } : row)
}

export function togglePriceListProductSpecSelection(selections = [], family = {}, skuID = 0, checked = false) {
  const parentProductID = priceListParentProductID(family)
  const normalizedSkuID = numberField(skuID)
  const current = (Array.isArray(selections) ? selections : []).slice()
  if (!checked) {
    return current.filter((row) => !(
      numberField(row?.parent_product_id ?? row?.parentProductID) === parentProductID &&
      numberField(row?.sku_id ?? row?.skuID) === normalizedSkuID
    ))
  }
  const valid = (family.sku_options || []).some((spec) => priceListSkuID(spec) === normalizedSkuID)
  const exists = current.some((row) => (
    numberField(row?.parent_product_id ?? row?.parentProductID) === parentProductID &&
    numberField(row?.sku_id ?? row?.skuID) === normalizedSkuID
  ))
  if (!valid || exists) return current
  const defaultSkuID = numberField(family.default_sku_id)
  return [...current, {
    parent_product_id: parentProductID,
    sku_id: normalizedSkuID,
    selection_source: 'explicit',
    default_sku_id_at_selection: defaultSkuID,
  }]
}

export function setPriceListCategorySpecSelection(categoryRows = [], category = null, selections = [], checked = false) {
  const rows = normalizeCategoryRows(categoryRows)
  const descendantCodes = new Set(priceListCategoryDescendantCodes(rows, category))
  const targetFamilies = rows.filter((row) => descendantCodes.has(row.code)).flatMap((row) => row.items)
  const allFamilies = rows.flatMap((row) => row.items)
  const targetParentIDs = new Set(targetFamilies.map((family) => priceListParentProductID(family)).filter(Boolean))
  const current = normalizePriceListProductSpecSelections(selections, allFamilies)
  if (!checked) {
    return current.filter((selection) => !targetParentIDs.has(numberField(selection.parent_product_id)))
  }
  const selectedParents = new Set(current.map((selection) => numberField(selection.parent_product_id)))
  const additions = targetFamilies.flatMap((family) => {
    const parentProductID = priceListParentProductID(family)
    if (selectedParents.has(parentProductID)) return []
    selectedParents.add(parentProductID)
    return defaultPriceListProductSpecSelections([family])
  })
  return normalizePriceListProductSpecSelections([...current, ...additions], allFamilies)
}

export function priceListProductSpecSelectionCounts(selections = []) {
  const normalized = Array.isArray(selections) ? selections : []
  return {
    productCount: new Set(normalized.map((row) => numberField(row?.parent_product_id ?? row?.parentProductID)).filter(Boolean)).size,
    specCount: new Set(normalized.map((row) => `${numberField(row?.parent_product_id ?? row?.parentProductID)}:${numberField(row?.sku_id ?? row?.skuID)}`).filter((key) => !key.endsWith(':0'))).size,
  }
}

export function priceListSelectedSkuCategoryRows(categoryRows = [], selections = []) {
  const rows = Array.isArray(categoryRows) ? categoryRows : []
  const families = rows.flatMap((row) => Array.isArray(row?.items) ? row.items : [])
  const normalized = normalizePriceListProductSpecSelections(selections, families)
  const selected = new Set(normalized.map((row) => `${row.parent_product_id}:${row.sku_id}`))
  return rows.map((category) => {
    const categoryCodeValue = categoryCode(category)
    const groupItemID = numberField(category?.group_item_id ?? category?.groupItemID)
    const items = (Array.isArray(category?.items) ? category.items : []).flatMap((family) => {
      const parentProductID = priceListParentProductID(family)
      return (Array.isArray(family?.sku_options) ? family.sku_options : [])
        .filter((spec) => selected.has(`${parentProductID}:${priceListSkuID(spec)}`))
        .map((spec) => ({
          ...spec,
          __price_list_parent_product_id: parentProductID,
          __price_list_parent_product_name: String(family?.name || family?.parent_product_name || '').trim(),
          __price_list_category_code: categoryCodeValue,
          __price_list_group_item_id: groupItemID,
        }))
    })
    return { ...category, items }
  })
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

function selectionIDsOf(item = {}) {
  const specs = Array.isArray(item?.sku_options) ? item.sku_options : []
  if (specs.length) return specs.map((spec) => String(priceListSkuID(spec))).filter(Boolean)
  const id = productIDOf(item)
  return id ? [id] : []
}

function normalizeStringList(values = []) {
  const raw = Array.isArray(values) ? values : String(values || '').split(/[\n,，]/)
  return raw.map((value) => String(value ?? '').trim()).filter(Boolean)
}

function numberField(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) ? n : 0
}

function directParentProductID(item = {}) {
  return numberField(item?.parent_product_id ?? item?.parentProductID ?? item?.parentProductId)
}

function isSelectableSpec(item = {}) {
  if (item?.active === false) return false
  const status = String(item?.derived_spec_status ?? item?.derivedSpecStatus ?? '').trim()
  return status === '' || status === 'active'
}

function normalizedSpecText(value) {
  return String(value || '').trim().toLowerCase().replace(/[\s（）()]/g, '')
}

function specPosition(item = {}) {
  const position = numberField(item?.derived_spec_position ?? item?.derivedSpecPosition ?? item?.spec_position ?? item?.specPosition)
  return position > 0 ? position : Number.MAX_SAFE_INTEGER
}

function firstPositiveNumber(...values) {
  for (const value of values) {
    const number = numberField(value)
    if (number > 0) return number
  }
  return 0
}

function sortSelectionsByFamilies(selections = [], families = []) {
  const familyOrder = new Map((Array.isArray(families) ? families : []).map((family, index) => [priceListParentProductID(family), index]))
  const skuOrder = new Map()
  ;(Array.isArray(families) ? families : []).forEach((family) => {
    const parentProductID = priceListParentProductID(family)
    ;(family.sku_options || []).forEach((spec, index) => skuOrder.set(`${parentProductID}:${priceListSkuID(spec)}`, index))
  })
  return selections.slice().sort((a, b) => {
    const aParent = numberField(a.parent_product_id)
    const bParent = numberField(b.parent_product_id)
    const familyDelta = (familyOrder.get(aParent) ?? Number.MAX_SAFE_INTEGER) - (familyOrder.get(bParent) ?? Number.MAX_SAFE_INTEGER)
    if (familyDelta !== 0) return familyDelta
    return (skuOrder.get(`${aParent}:${numberField(a.sku_id)}`) ?? Number.MAX_SAFE_INTEGER) - (skuOrder.get(`${bParent}:${numberField(b.sku_id)}`) ?? Number.MAX_SAFE_INTEGER)
  })
}
