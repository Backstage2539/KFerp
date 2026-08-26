import { isProductBomSpecCutover, productSpecMigrationState } from './product-spec-cutover.js'

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
    const bomSpecAuthoritative = entries.some(({ item }) => (
      item?.bom_spec_authoritative === true || item?.bomSpecAuthoritative === true
    ))
    const hasSelectableBOMSpec = selectableEntries.some(({ item }) => numberField(item?.bom_spec_id ?? item?.bomSpecID) > 0)
    if (bomSpecAuthoritative && !hasSelectableBOMSpec) {
      return {
        ...parentItem,
        product_id: parentProductID,
        sku_id: parentProductID,
        parent_product_id: parentProductID,
        effective_parent_product_id: parentProductID,
        name: priceListParentDisplayName(parentItem, priceListParentCatalogName(parentItem)),
        parent_product_name: priceListParentDisplayName(parentItem, priceListParentCatalogName(parentItem)),
        product_key: `product:${parentProductID}`,
        default_sku_id: 0,
        spec_identity_mode: 'bom_spec',
        bom_spec_authoritative: true,
        parent_item: parentItem,
        sku_options: [],
        no_quoteable_bom_specs: true,
        __price_list_product_family: true,
      }
    }
    const explicitDefaultSkuID = firstPositiveNumber(
      parentItem?.default_bom_spec_id,
      parentItem?.defaultBomSpecID,
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
    const defaultSpecRow = matchingDefaultEntry?.item || {}
    const parentProductName = priceListParentCatalogName(parentItem)
    const parentDisplayName = priceListParentDisplayName(parentItem, parentProductName)
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
        ...(numberField(item?.bom_spec_id ?? item?.bomSpecID) > 0 ? {
          ...(numberField(item?.bom_id ?? item?.bomID) > 0 ? { bom_id: numberField(item?.bom_id ?? item?.bomID) } : {}),
          ...(numberField(item?.bom_version_id ?? item?.bomVersionID) > 0 ? { bom_version_id: numberField(item?.bom_version_id ?? item?.bomVersionID) } : {}),
          bom_spec_id: numberField(item?.bom_spec_id ?? item?.bomSpecID),
          bom_variant_id: numberField(item?.bom_variant_id ?? item?.bomVariantID),
          migration_state: productSpecMigrationState(item),
          ...((numberField(item?.bom_id ?? item?.bomID) > 0 || numberField(item?.bom_version_id ?? item?.bomVersionID) > 0 || item?.bom_spec_authoritative === true || item?.bomSpecAuthoritative === true)
            ? { spec_identity_mode: 'bom_spec', bom_spec_authoritative: true }
            : {}),
        } : {}),
        parent_product_id: parentProductID,
        effective_parent_product_id: parentProductID,
        __price_list_parent_product_id: parentProductID,
        __price_list_parent_product_name: parentDisplayName,
        __price_list_product_name: parentProductName,
      }))
    return {
      ...parentItem,
      product_id: parentProductID,
      sku_id: parentProductID,
      parent_product_id: parentProductID,
      effective_parent_product_id: parentProductID,
      name: parentDisplayName,
      parent_product_name: parentDisplayName,
      product_key: `product:${parentProductID}`,
      default_sku_id: defaultSkuID,
      ...(numberField(defaultSpecRow?.bom_id ?? defaultSpecRow?.bomID) > 0 ? { default_bom_id: numberField(defaultSpecRow?.bom_id ?? defaultSpecRow?.bomID) } : {}),
      ...(numberField(defaultSpecRow?.bom_version_id ?? defaultSpecRow?.bomVersionID) > 0 ? { default_bom_version_id: numberField(defaultSpecRow?.bom_version_id ?? defaultSpecRow?.bomVersionID) } : {}),
      ...(numberField(defaultSpecRow?.bom_spec_id ?? defaultSpecRow?.bomSpecID) > 0 ? { default_bom_spec_id: numberField(defaultSpecRow?.bom_spec_id ?? defaultSpecRow?.bomSpecID) } : {}),
      ...(numberField(defaultSpecRow?.bom_variant_id ?? defaultSpecRow?.bomVariantID) > 0 ? { default_bom_variant_id: numberField(defaultSpecRow?.bom_variant_id ?? defaultSpecRow?.bomVariantID) } : {}),
      ...(numberField(matchingDefaultEntry?.item?.bom_spec_id ?? matchingDefaultEntry?.item?.bomSpecID) > 0 ? {
        default_bom_spec_id: numberField(matchingDefaultEntry?.item?.bom_spec_id ?? matchingDefaultEntry?.item?.bomSpecID),
        migration_state: productSpecMigrationState(matchingDefaultEntry?.item),
        ...((numberField(matchingDefaultEntry?.item?.bom_id ?? matchingDefaultEntry?.item?.bomID) > 0 || numberField(matchingDefaultEntry?.item?.bom_version_id ?? matchingDefaultEntry?.item?.bomVersionID) > 0 || matchingDefaultEntry?.item?.bom_spec_authoritative === true || matchingDefaultEntry?.item?.bomSpecAuthoritative === true)
          ? { spec_identity_mode: 'bom_spec', bom_spec_authoritative: true }
          : {}),
      } : {}),
      parent_item: parentItem,
      sku_options: skuOptions,
      __price_list_product_family: true,
    }
  })
}

export function priceListSkuID(item = {}) {
  const bomSpecID = numberField(item?.bom_spec_id ?? item?.bomSpecID)
  if (bomSpecID > 0) return bomSpecID
  return numberField(item?.sku_id ?? item?.skuID ?? item?.skuId ?? item?.product_id ?? item?.productID ?? item?.productId ?? item?.id)
}

export function priceListParentProductID(item = {}) {
  const annotated = numberField(item?.__price_list_parent_product_id)
  if (annotated > 0) return annotated
  const effective = numberField(item?.effective_parent_product_id ?? item?.effectiveParentProductID ?? item?.effectiveParentProductId)
  if (effective > 0) return effective
  const direct = directParentProductID(item)
  if (direct > 0) return direct
  if (isProductBomSpecCutover(item) || numberField(item?.bom_spec_id ?? item?.bomSpecID) > 0) {
    return numberField(item?.product_id ?? item?.productID ?? item?.productId ?? item?.id)
  }
  return priceListSkuID(item)
}

export function priceListProductSpecLabel(item = {}) {
  const label = [
    item?.derived_sales_unit,
    item?.derivedSalesUnit,
    item?.sku_name,
    item?.skuName,
    item?.spec_label,
    item?.specLabel,
    item?.default_sales_unit,
    item?.defaultSalesUnit,
  ].map((value) => String(value ?? '').trim()).find(Boolean) || ''
  return label || `SKU-${String(priceListSkuID(item)).padStart(6, '0')}`
}

export function defaultPriceListProductSpecSelections(families = []) {
  return (Array.isArray(families) ? families : []).flatMap((family) => {
    const parentProductID = priceListParentProductID(family)
    const defaultSkuID = numberField(family?.default_sku_id ?? family?.defaultSkuID)
    if (!(parentProductID > 0) || !(defaultSkuID > 0)) return []
    const defaultSpec = (family?.sku_options || []).find((spec) => priceListSkuID(spec) === defaultSkuID) || {}
    return [{
      parent_product_id: parentProductID,
      sku_id: defaultSkuID,
      ...(numberField(defaultSpec?.bom_spec_id ?? defaultSpec?.bomSpecID) > 0 ? {
        product_id: parentProductID,
        ...(numberField(defaultSpec?.bom_id ?? defaultSpec?.bomID) > 0 ? { bom_id: numberField(defaultSpec?.bom_id ?? defaultSpec?.bomID) } : {}),
        ...(numberField(defaultSpec?.bom_version_id ?? defaultSpec?.bomVersionID) > 0 ? { bom_version_id: numberField(defaultSpec?.bom_version_id ?? defaultSpec?.bomVersionID) } : {}),
        bom_spec_id: numberField(defaultSpec?.bom_spec_id ?? defaultSpec?.bomSpecID),
        bom_variant_id: numberField(defaultSpec?.bom_variant_id ?? defaultSpec?.bomVariantID),
        migration_state: productSpecMigrationState(defaultSpec),
        ...((numberField(defaultSpec?.bom_id ?? defaultSpec?.bomID) > 0 || numberField(defaultSpec?.bom_version_id ?? defaultSpec?.bomVersionID) > 0 || defaultSpec?.bom_spec_authoritative === true || defaultSpec?.bomSpecAuthoritative === true)
          ? { spec_identity_mode: 'bom_spec', bom_spec_authoritative: true }
          : {}),
      } : {}),
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
    const selectedSpec = (family.sku_options || []).find((spec) => priceListSkuID(spec) === skuID) || {}
    if (numberField(selectedSpec?.bom_spec_id ?? selectedSpec?.bomSpecID) > 0) {
      row.product_id = parentProductID
      if (numberField(selectedSpec?.bom_id ?? selectedSpec?.bomID) > 0) row.bom_id = numberField(selectedSpec?.bom_id ?? selectedSpec?.bomID)
      if (numberField(selectedSpec?.bom_version_id ?? selectedSpec?.bomVersionID) > 0) row.bom_version_id = numberField(selectedSpec?.bom_version_id ?? selectedSpec?.bomVersionID)
      row.bom_spec_id = numberField(selectedSpec?.bom_spec_id ?? selectedSpec?.bomSpecID)
      row.bom_variant_id = numberField(selectedSpec?.bom_variant_id ?? selectedSpec?.bomVariantID)
      row.migration_state = productSpecMigrationState(selectedSpec)
      if (numberField(selectedSpec?.bom_id ?? selectedSpec?.bomID) > 0 || numberField(selectedSpec?.bom_version_id ?? selectedSpec?.bomVersionID) > 0 || selectedSpec?.bom_spec_authoritative === true || selectedSpec?.bomSpecAuthoritative === true) {
        row.spec_identity_mode = 'bom_spec'
        row.bom_spec_authoritative = true
      }
    }
    const selectedBomSpecID = numberField(selectedSpec?.bom_spec_id ?? selectedSpec?.bomSpecID)
    const selectedBomVersionID = numberField(selectedSpec?.bom_version_id ?? selectedSpec?.bomVersionID)
    const selectedBomID = numberField(selectedSpec?.bom_id ?? selectedSpec?.bomID)
    const selectedBomVariantID = numberField(selectedSpec?.bom_variant_id ?? selectedSpec?.bomVariantID)
    const identityChanged = selectedBomSpecID > 0 && (
      (numberField(selection?.bom_id ?? selection?.bomID) > 0 && numberField(selection?.bom_id ?? selection?.bomID) !== selectedBomID)
      || (numberField(selection?.bom_version_id ?? selection?.bomVersionID) > 0 && numberField(selection?.bom_version_id ?? selection?.bomVersionID) !== selectedBomVersionID)
      || (numberField(selection?.bom_spec_id ?? selection?.bomSpecID) > 0 && numberField(selection?.bom_spec_id ?? selection?.bomSpecID) !== selectedBomSpecID)
      || (numberField(selection?.bom_variant_id ?? selection?.bomVariantID) > 0 && numberField(selection?.bom_variant_id ?? selection?.bomVariantID) !== selectedBomVariantID)
    )
    if (!validSkuIDs.has(skuID)) {
      row.selection_issue = 'invalid_spec'
      row.current_default_sku_id = defaultSkuID
    } else if (identityChanged) {
      row.selection_issue = 'default_bom_changed'
      row.current_default_sku_id = defaultSkuID
      row.current_default_bom_id = selectedBomID
      row.current_default_bom_version_id = selectedBomVersionID
      row.current_default_bom_spec_id = selectedBomSpecID
      row.current_default_bom_variant_id = selectedBomVariantID
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
    message: type === 'default_changed' || type === 'default_bom_changed'
      ? `商品默认规格已变更为“${priceListProductSpecLabel(currentDefaultSpec || {})}”，请选择保留原规格或切换默认规格。`
      : '草稿中的商品 BOM 规格已停用、被移除或不属于当前商品，请切换到当前默认规格。',
  }
}

export function resolvePriceListProductSpecSelectionIssue(selections = [], family = {}, action = 'switch') {
  const parentProductID = priceListParentProductID(family)
  const issue = priceListProductSpecSelectionIssue(family, selections)
  if (!issue) return Array.isArray(selections) ? selections.slice() : []
  const issueSkuID = numberField(issue.selection?.sku_id)
  const currentDefaultSkuID = numberField(issue.current_default_sku_id ?? family?.default_sku_id)
  const currentDefaultSpec = (family.sku_options || []).find((spec) => priceListSkuID(spec) === currentDefaultSkuID) || {}
  const source = Array.isArray(selections) ? selections : []
  const issueIndex = source.findIndex((row) => (
    numberField(row?.parent_product_id ?? row?.parentProductID) === parentProductID
    && numberField(row?.sku_id ?? row?.skuID) === issueSkuID
  ))
  const currentIdentity = currentDefaultSpec ? {
    ...(numberField(currentDefaultSpec?.bom_id ?? currentDefaultSpec?.bomID) > 0 ? { bom_id: numberField(currentDefaultSpec?.bom_id ?? currentDefaultSpec?.bomID) } : {}),
    ...(numberField(currentDefaultSpec?.bom_version_id ?? currentDefaultSpec?.bomVersionID) > 0 ? { bom_version_id: numberField(currentDefaultSpec?.bom_version_id ?? currentDefaultSpec?.bomVersionID) } : {}),
    ...(numberField(currentDefaultSpec?.bom_spec_id ?? currentDefaultSpec?.bomSpecID) > 0 ? { bom_spec_id: numberField(currentDefaultSpec?.bom_spec_id ?? currentDefaultSpec?.bomSpecID) } : {}),
    ...(numberField(currentDefaultSpec?.bom_variant_id ?? currentDefaultSpec?.bomVariantID) > 0 ? { bom_variant_id: numberField(currentDefaultSpec?.bom_variant_id ?? currentDefaultSpec?.bomVariantID) } : {}),
  } : {}
  if (action === 'keep' && (issue.type === 'default_changed' || issue.type === 'default_bom_changed')) {
    return source.map((row, index) => index === issueIndex ? {
      parent_product_id: parentProductID,
      sku_id: issueSkuID,
      selection_source: 'explicit',
      default_sku_id_at_selection: currentDefaultSkuID,
      ...currentIdentity,
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
    ...currentIdentity,
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
  const selectedSpec = (family.sku_options || []).find((spec) => priceListSkuID(spec) === normalizedSkuID)
  const valid = Boolean(selectedSpec)
  const exists = current.some((row) => (
    numberField(row?.parent_product_id ?? row?.parentProductID) === parentProductID &&
    numberField(row?.sku_id ?? row?.skuID) === normalizedSkuID
  ))
  if (!valid || exists) return current
  const defaultSkuID = numberField(family.default_sku_id)
  return [...current, {
    parent_product_id: parentProductID,
    sku_id: normalizedSkuID,
    ...(numberField(selectedSpec?.bom_spec_id ?? selectedSpec?.bomSpecID) > 0 ? {
      product_id: parentProductID,
      ...(numberField(selectedSpec?.bom_id ?? selectedSpec?.bomID) > 0 ? { bom_id: numberField(selectedSpec?.bom_id ?? selectedSpec?.bomID) } : {}),
      ...(numberField(selectedSpec?.bom_version_id ?? selectedSpec?.bomVersionID) > 0 ? { bom_version_id: numberField(selectedSpec?.bom_version_id ?? selectedSpec?.bomVersionID) } : {}),
      bom_spec_id: numberField(selectedSpec?.bom_spec_id ?? selectedSpec?.bomSpecID),
      bom_variant_id: numberField(selectedSpec?.bom_variant_id ?? selectedSpec?.bomVariantID),
      migration_state: productSpecMigrationState(selectedSpec),
      ...((numberField(selectedSpec?.bom_id ?? selectedSpec?.bomID) > 0 || numberField(selectedSpec?.bom_version_id ?? selectedSpec?.bomVersionID) > 0 || selectedSpec?.bom_spec_authoritative === true || selectedSpec?.bomSpecAuthoritative === true)
        ? { spec_identity_mode: 'bom_spec', bom_spec_authoritative: true }
        : {}),
    } : {}),
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
        .map((spec) => priceListSelectedSkuProjection(family, spec, {
          parentProductID,
          categoryCode: categoryCodeValue,
          groupItemID,
        }))
    })
    return { ...category, items }
  })
}

// Older price-list snapshots sometimes identify a concrete row with the
// parent product, or store a BOM specification id in the legacy sku_id field.
// Rewrite an exact BOM-spec match even when several specifications are
// selected; only the parent-only fallback remains limited to one candidate.
function selectedLegacyPriceListSKUsByParent(selections = []) {
  const selectedByParent = new Map()
  ;(Array.isArray(selections) ? selections : []).forEach((selection) => {
    const parentProductID = numberField(selection?.parent_product_id ?? selection?.parentProductID)
    const skuID = numberField(selection?.sku_id ?? selection?.skuID)
    if (!(parentProductID > 0) || !(skuID > 0)) return
    if (!selectedByParent.has(parentProductID)) selectedByParent.set(parentProductID, [])
    selectedByParent.get(parentProductID).push(selection)
  })
  return selectedByParent
}

function normalizePriceListPublicationRowIdentity(row, selectedByParent) {
  if (!row || typeof row !== 'object') return row
  const parentProductID = numberField(row?.parent_product_id ?? row?.parentProductID ?? row?.product_id ?? row?.productID)
  const skuID = numberField(row?.sku_id ?? row?.skuID ?? row?.product_id ?? row?.productID)
  const productID = numberField(row?.product_id ?? row?.productID)
  const existingBOMSpecID = numberField(row?.bom_spec_id ?? row?.bomSpecID)
  const existingBOMVariantID = numberField(row?.bom_variant_id ?? row?.bomVariantID)
  if (!(parentProductID > 0) || existingBOMSpecID > 0 || existingBOMVariantID > 0) return row
  const candidates = selectedByParent.get(parentProductID) || []
  const exactBOMSelection = candidates.find((candidate) => {
    const bomSpecID = numberField(candidate?.bom_spec_id ?? candidate?.bomSpecID)
    const selectedSKU = numberField(candidate?.sku_id ?? candidate?.skuID)
    const productMatchesPseudoSKU = !(productID > 0) || productID === parentProductID || productID === skuID || productID === bomSpecID || productID === selectedSKU
    return bomSpecID > 0 && skuID > 0 && productMatchesPseudoSKU && (skuID === bomSpecID || skuID === selectedSKU)
  })
  const selection = exactBOMSelection || (
    skuID === parentProductID &&
    (!(productID > 0) || productID === parentProductID) &&
    candidates.length === 1
      ? candidates[0]
      : null
  )
  if (!selection) return row
  const selectedBOMSpecID = numberField(selection?.bom_spec_id ?? selection?.bomSpecID)
  const selectedBOMVariantID = numberField(selection?.bom_variant_id ?? selection?.bomVariantID)
  if (selectedBOMSpecID > 0 && selectedBOMVariantID > 0) {
    const normalized = {
      ...row,
      product_id: parentProductID,
      parent_product_id: parentProductID,
      bom_spec_id: selectedBOMSpecID,
      bom_variant_id: selectedBOMVariantID,
    }
    const bomID = numberField(selection?.bom_id ?? selection?.bomID)
    const bomVersionID = numberField(selection?.bom_version_id ?? selection?.bomVersionID)
    if (bomID > 0) normalized.bom_id = bomID
    else delete normalized.bom_id
    if (bomVersionID > 0) normalized.bom_version_id = bomVersionID
    else delete normalized.bom_version_id
    const migrationState = String(selection?.migration_state ?? selection?.migrationState ?? '').trim()
    if (migrationState) normalized.migration_state = migrationState
    const identityMode = String(selection?.spec_identity_mode ?? selection?.specIdentityMode ?? '').trim()
    if (identityMode) normalized.spec_identity_mode = identityMode
    if (selection?.bom_spec_authoritative === true || selection?.bomSpecAuthoritative === true) normalized.bom_spec_authoritative = true
    delete normalized.sku_id
    return normalized
  }
  const selectedSKU = numberField(selection?.sku_id ?? selection?.skuID)
  if (!(selectedSKU > 0) || selectedSKU === parentProductID) return row
  return {
    ...row,
    product_id: selectedSKU,
    sku_id: selectedSKU,
    parent_product_id: parentProductID,
  }
}

export function normalizePriceListPublicationRows(rows = [], selections = []) {
  const selectedByParent = selectedLegacyPriceListSKUsByParent(selections)
  return (Array.isArray(rows) ? rows : []).map((row) => normalizePriceListPublicationRowIdentity(row, selectedByParent))
}

export function normalizePriceListPublicationGroups(groups = [], selections = []) {
  const selectedByParent = selectedLegacyPriceListSKUsByParent(selections)
  return (Array.isArray(groups) ? groups : []).map((group) => ({
    ...group,
    items: (Array.isArray(group?.items) ? group.items : []).map((item) => normalizePriceListPublicationRowIdentity(item, selectedByParent)),
  }))
}

function priceListSelectedSkuProjection(family = {}, spec = {}, context = {}) {
  const parentItem = family?.parent_item || family || {}
  const productName = priceListParentCatalogName(parentItem)
    || String(family?.__price_list_product_name || '').trim()
    || String(family?.parent_product_name || family?.name || '').trim()
  const displayName = firstNonEmptyText(
    priceListExplicitCustomerAliasName(spec),
    family?.name,
    productName,
  ) || productName
  const parentProductID = numberField(context.parentProductID) || priceListParentProductID(family)
  const productAttributes = priceListSelectedSkuProductAttributes(parentItem, spec)
  return {
    ...spec,
    name: displayName,
    product_name: productName,
    ...(productAttributes.length ? { product_attributes: productAttributes } : {}),
    __price_list_parent_product_id: parentProductID,
    __price_list_parent_product_name: displayName,
    __price_list_display_name: displayName,
    __price_list_product_name: productName,
    __price_list_sales_spec_label: priceListSalesSpecAttributeValue(spec),
    __price_list_category_code: String(context.categoryCode || '').trim(),
    __price_list_group_item_id: numberField(context.groupItemID),
  }
}

function priceListExplicitCustomerAliasName(item = {}) {
  const aliasID = numberField(item?.customer_product_alias_id ?? item?.customerProductAliasID)
  if (!(aliasID > 0)) return ''
  return firstNonEmptyText(
    item?.customer_product_display_name,
    item?.customerProductDisplayName,
    item?.brand_name,
    item?.brandName,
    item?.name,
  )
}

function priceListSelectedSkuProductAttributes(parentItem = {}, spec = {}) {
  const specAttributes = Array.isArray(spec?.product_attributes)
    ? spec.product_attributes
    : (Array.isArray(spec?.productAttributes) ? spec.productAttributes : [])
  const parentAttributes = Array.isArray(parentItem?.product_attributes)
    ? parentItem.product_attributes
    : (Array.isArray(parentItem?.productAttributes) ? parentItem.productAttributes : [])
  return [...specAttributes, ...parentAttributes].map((row) => ({ ...row }))
}

function priceListSalesSpecAttributeValue(item = {}) {
  const effectiveSalesSpec = item?.effective_sales_spec && typeof item.effective_sales_spec === 'object'
    ? item.effective_sales_spec
    : (item?.effectiveSalesSpec && typeof item.effectiveSalesSpec === 'object' ? item.effectiveSalesSpec : {})
  return firstNonEmptyText(
    effectiveSalesSpec?.spec_label,
    effectiveSalesSpec?.specLabel,
    item?.spec_label,
    item?.specLabel,
    effectiveSalesSpec?.spec_name,
    effectiveSalesSpec?.specName,
    item?.derived_sales_unit,
    item?.derivedSalesUnit,
    item?.sku_name,
    item?.skuName,
  ) || priceListProductSpecLabel(item)
}

function priceListParentCatalogName(parentItem = {}) {
  return firstNonEmptyText(parentItem?.product_name, parentItem?.productName, parentItem?.name)
}

function priceListParentDisplayName(parentItem = {}, fallback = '') {
  return firstNonEmptyText(
    parentItem?.customer_product_display_name,
    parentItem?.customerProductDisplayName,
    parentItem?.name,
    fallback,
  ) || String(fallback || '').trim()
}

function firstNonEmptyText(...values) {
  for (const value of values) {
    const text = String(value ?? '').trim()
    if (text) return text
  }
  return ''
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
