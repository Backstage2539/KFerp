type ProductSpecIdentitySource = {
  migration_state?: string
  migrationState?: string
  bom_spec_migration_state?: string
  parent_product_id?: number
  product_family_id?: number
  product_id?: number
  id?: number
  bom_spec_id?: number
  bom_variant_id?: number
  qty?: number
  unit?: string
  sales_unit?: string
}

function positiveID(...values: unknown[]): number {
  for (const value of values) {
    const id = Number(value || 0)
    if (Number.isFinite(id) && id > 0) return id
  }
  return 0
}

export function miniappProductMigrationState(value: Record<string, unknown> = {}): string {
  const raw = value.migration_state
    ?? value.migrationState
    ?? value.bom_spec_migration_state
    ?? (value.migration as Record<string, unknown> | undefined)?.state
    ?? 'legacy'
  return String(raw || '').trim().toLowerCase() === 'cutover' ? 'cutover' : 'legacy'
}

export function visibleMiniappProductFamilies<T extends Record<string, any>>(rows: T[] = []): T[] {
  const cutoverParents = new Set(rows
    .filter((row) => miniappProductMigrationState(row) === 'cutover')
    .map((row) => positiveID(row.parent_product_id, row.product_id, row.id))
    .filter((id) => id > 0))
  return rows.filter((row) => {
    const derived = row.auto_derived_sku === true || String(row.derived_spec_key || '').trim() !== ''
    if (!derived) return true
    const parentID = positiveID(row.legacy_parent_product_id, row.parent_id, row.parent_product_parent_id)
    return parentID <= 0 || !cutoverParents.has(parentID)
  }).map((row) => {
    if (miniappProductMigrationState(row) !== 'cutover' || !Array.isArray(row.specs)) return row
    return {
      ...row,
      specs: row.specs.filter((spec: Record<string, unknown>) => positiveID(spec.bom_spec_id, spec.bomSpecID) > 0),
    }
  }) as T[]
}

export function buildMiniappProductSpecIdentity(row: ProductSpecIdentitySource) {
  const qty = Number(row.qty || 0) || 0
  const unit = String(row.unit || row.sales_unit || '').trim()
  if (miniappProductMigrationState(row as Record<string, unknown>) !== 'cutover') {
    return { product_id: positiveID(row.product_id, row.id), qty, unit }
  }
  return {
    product_id: positiveID(row.parent_product_id, row.product_family_id, row.product_id, row.id),
    bom_spec_id: positiveID(row.bom_spec_id),
    bom_variant_id: positiveID(row.bom_variant_id),
    qty,
    unit,
  }
}
