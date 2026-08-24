function positiveID(...values) {
  for (const value of values) {
    const id = Number(value || 0)
    if (Number.isFinite(id) && id > 0) return id
  }
  return 0
}

export function productSpecMigrationState(value = {}) {
  const raw = value?.migration_state
    ?? value?.migrationState
    ?? value?.bom_spec_migration_state
    ?? value?.bomSpecMigrationState
    ?? value?.migration?.state
    ?? value?.state
    ?? 'legacy'
  const state = String(raw || '').trim().toLowerCase()
  return ['preparing', 'ready', 'cutover'].includes(state) ? state : 'legacy'
}

export function isProductBomSpecCutover(value = {}) {
  if (value?.bom_spec_authoritative === true || value?.bomSpecAuthoritative === true) return true
  const legacyCatalogProduct = value?.legacy_catalog_product ?? value?.legacyCatalogProduct
  if (legacyCatalogProduct === false || legacyCatalogProduct === 0 || String(legacyCatalogProduct).toLowerCase() === 'false') return true
  return productSpecMigrationState(value) === 'cutover'
}

export function legacyProductTemplateWriteTargets(rows = [], selectedIDs = []) {
  const selected = new Set((Array.isArray(selectedIDs) ? selectedIDs : [])
    .map((value) => Number(value || 0))
    .filter((value) => value > 0))
  return (Array.isArray(rows) ? rows : [])
    .filter((row) => selected.has(productID(row)) && !isProductBomSpecCutover(row))
}

function legacyParentProductID(row = {}) {
  return positiveID(
    row.parent_product_id,
    row.parentProductID,
    row.legacy_parent_product_id,
    row.legacyParentProductID,
    row.parent_id,
    row.parentID,
  )
}

function productID(row = {}) {
  return positiveID(row.product_id, row.productID, row.id)
}

function isLegacyDerivedSKU(row = {}) {
  return row.auto_derived_sku === true
    || row.autoDerivedSKU === true
    || String(row.derived_spec_key || row.derivedSpecKey || '').trim() !== ''
}

export function visibleRowsForProductSpecMigration(rows = [], migrationByProductID = {}) {
  const source = Array.isArray(rows) ? rows : []
  const cutoverParents = new Set()
  for (const row of source) {
    const id = productID(row)
    if (id > 0 && isProductBomSpecCutover(migrationByProductID?.[String(id)] ?? migrationByProductID?.[id] ?? row)) cutoverParents.add(id)
  }
  return source.filter((row) => {
    if (!isLegacyDerivedSKU(row)) return true
    const parentID = legacyParentProductID(row)
    return parentID <= 0 || (!cutoverParents.has(parentID) && !isProductBomSpecCutover(migrationByProductID?.[String(parentID)] ?? migrationByProductID?.[parentID] ?? row))
  })
}

export function normalizeProductBomSpecs(value = {}) {
  const rows = Array.isArray(value)
    ? value
    : (value?.variants ?? value?.bom_specs ?? value?.bomSpecs ?? value?.specs ?? [])
  return (Array.isArray(rows) ? rows : [])
    .map((row) => {
      const bomID = positiveID(row?.bom_id, row?.bomID)
      const bomVersionID = positiveID(row?.bom_version_id, row?.bomVersionID)
      return {
        ...(bomID > 0 ? { bom_id: bomID } : {}),
        ...(bomVersionID > 0 ? { bom_version_id: bomVersionID } : {}),
        bom_spec_id: positiveID(row?.bom_spec_id, row?.bomSpecID, row?.spec_id, row?.id),
        bom_variant_id: positiveID(row?.bom_variant_id, row?.bomVariantID, row?.variant_id),
        code: String(row?.spec_code ?? row?.code ?? '').trim(),
        barcode: String(row?.barcode ?? '').trim(),
        name: String(row?.name ?? row?.spec_name_snapshot ?? row?.spec_name ?? row?.label ?? '').trim(),
        unit: String(row?.inventory_unit ?? row?.output_unit ?? row?.unit ?? '').trim(),
        is_default: row?.is_default === true || row?.isDefault === true,
        sort_order: Number(row?.sort_order ?? row?.sortOrder ?? 0) || 0,
      }
    })
    .filter((row) => row.bom_spec_id > 0)
    .sort((a, b) => a.sort_order - b.sort_order || a.bom_spec_id - b.bom_spec_id)
}

export function buildProductSpecWriteIdentity(row = {}) {
  const qty = Number(row?.qty ?? row?.quantity ?? 0) || 0
  const unit = String(row?.unit ?? row?.sales_unit ?? row?.salesUnit ?? '').trim()
  if (!isProductBomSpecCutover(row)) {
    return { product_id: productID(row), qty, unit }
  }
  return {
    product_id: positiveID(row?.parent_product_id, row?.parentProductID, row?.product_family_id, row?.productFamilyID, productID(row)),
    bom_spec_id: positiveID(row?.bom_spec_id, row?.bomSpecID),
    bom_variant_id: positiveID(row?.bom_variant_id, row?.bomVariantID),
    qty,
    unit,
  }
}

export function productSpecSelectionID(row = {}) {
  if (isProductBomSpecCutover(row)) return positiveID(row?.bom_spec_id, row?.bomSpecID)
  return productID(row)
}

export function productSpecSelectionsForWrite(rows = []) {
  return (Array.isArray(rows) ? rows : []).map((row) => {
    const bomSpecID = positiveID(row?.bom_spec_id, row?.bomSpecID)
    if (bomSpecID <= 0) return { ...row }
    const next = {
      ...row,
      product_id: positiveID(row?.parent_product_id, row?.parentProductID, row?.product_id),
      parent_product_id: positiveID(row?.parent_product_id, row?.parentProductID, row?.product_id),
      bom_spec_id: bomSpecID,
      bom_variant_id: positiveID(row?.bom_variant_id, row?.bomVariantID),
      ...(positiveID(row?.bom_id, row?.bomID) > 0 ? { bom_id: positiveID(row?.bom_id, row?.bomID) } : {}),
      ...(positiveID(row?.bom_version_id, row?.bomVersionID) > 0 ? { bom_version_id: positiveID(row?.bom_version_id, row?.bomVersionID) } : {}),
      migration_state: productSpecMigrationState(row),
      ...((positiveID(row?.bom_id, row?.bomID) > 0 || positiveID(row?.bom_version_id, row?.bomVersionID) > 0 || row?.bom_spec_authoritative === true || row?.bomSpecAuthoritative === true)
        ? { spec_identity_mode: 'bom_spec', bom_spec_authoritative: true }
        : {}),
    }
    delete next.sku_id
    delete next.default_sku_id_at_selection
    return next
  })
}
