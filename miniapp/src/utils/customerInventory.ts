import type {
  CustomerInventorySummary,
  EmployeeOrderProductFamily,
} from '../api/customerPortal'
import { productSpecLabel, productSpecWeightG } from './employeeOrder'

export type ProcessingPrefillItem = {
  product_id: number
  bom_spec_id?: number
  bom_variant_id?: number
  spec_g: number
  spec_name?: string
  bom_spec_name?: string
  inventory_unit?: string
  product_name?: string
  sku_code?: string
}

export type ProcessingPrefillDraftLine = {
  product_id: number
  bom_spec_id?: number
  bom_variant_id?: number
  product_name: string
  spec_g: number
  spec_label: string
  inventory_unit?: string
  qty: number
}

export type CustomerInventorySelection = Record<string, CustomerInventorySummary>

type CustomerInventoryIdentity = Pick<CustomerInventorySummary, 'product_id' | 'spec_g' | 'bom_spec_id' | 'bom_variant_id' | 'inventory_unit'>

export function customerInventoryItemKey(item: CustomerInventoryIdentity): string {
  const bomSpecID = Number(item.bom_spec_id || 0)
  if (bomSpecID > 0) {
    return `${Number(item.product_id || 0)}:bom_spec:${bomSpecID}`
  }
  return `${Number(item.product_id || 0)}:${Number(item.spec_g || 0)}`
}

export function toggleCustomerInventorySelection(
  current: CustomerInventorySelection,
  item: CustomerInventorySummary,
): CustomerInventorySelection {
  const key = customerInventoryItemKey(item)
  const next = { ...current }
  if (next[key]) delete next[key]
  else next[key] = item
  return next
}

export function customerInventorySelectionItems(current: CustomerInventorySelection): CustomerInventorySummary[] {
  return Object.values(current)
}

export function customerInventoryDetailPath(
  item: CustomerInventoryIdentity,
): string {
  const bomSpecID = Number(item.bom_spec_id || 0)
  const bomVariantID = Number(item.bom_variant_id || 0)
  if (bomSpecID > 0) {
    const unit = encodeURIComponent(String(item.inventory_unit || '').trim())
    const variantQuery = bomVariantID > 0 ? `&bom_variant_id=${bomVariantID}` : ''
    return `/pages/customer-inventory-detail/customer-inventory-detail?product_id=${Number(item.product_id || 0)}&bom_spec_id=${bomSpecID}${variantQuery}&inventory_unit=${unit}`
  }
  return `/pages/customer-inventory-detail/customer-inventory-detail?product_id=${Number(item.product_id || 0)}&spec_g=${Number(item.spec_g || 0)}`
}

export function matchesCustomerInventoryIdentity(
  item: Pick<CustomerInventorySummary, 'product_id' | 'spec_g' | 'bom_spec_id' | 'bom_variant_id'>,
  target: Pick<CustomerInventorySummary, 'product_id' | 'spec_g' | 'bom_spec_id' | 'bom_variant_id'>,
): boolean {
  if (Number(item.product_id || 0) !== Number(target.product_id || 0)) return false
  const bomSpecID = Number(target.bom_spec_id || 0)
  if (bomSpecID > 0) return Number(item.bom_spec_id || 0) === bomSpecID
  return Number(item.bom_spec_id || 0) <= 0 && Number(item.spec_g || 0) === Number(target.spec_g || 0)
}

export function normalizeProcessingPrefillItems(
  items: Array<Partial<ProcessingPrefillItem>> = [],
): ProcessingPrefillItem[] {
  const normalized = new Map<string, ProcessingPrefillItem>()
  for (const item of items) {
    const productID = Number(item.product_id || 0)
    const bomSpecID = Number(item.bom_spec_id || 0)
    const bomVariantID = Number(item.bom_variant_id || 0)
    const specG = Number(item.spec_g || 0)
    if (productID <= 0 || specG < 0) continue
    const canonical = bomSpecID > 0 || bomVariantID > 0
    if (canonical && (bomSpecID <= 0 || bomVariantID <= 0)) continue
    const key = canonical
      ? `${productID}:bom_spec:${bomSpecID}:${bomVariantID}`
      : `${productID}:legacy:${specG}`
    if (normalized.has(key)) continue
    const row: ProcessingPrefillItem = {
      product_id: productID,
      spec_g: canonical ? 0 : specG,
      product_name: String(item.product_name || '').trim() || undefined,
      sku_code: String(item.sku_code || '').trim() || undefined,
    }
    if (canonical) {
      row.bom_spec_id = bomSpecID
      row.bom_variant_id = bomVariantID
      row.spec_name = String(item.spec_name || item.bom_spec_name || '').trim() || undefined
      row.inventory_unit = String(item.inventory_unit || '').trim() || undefined
    }
    normalized.set(key, row)
  }
  return Array.from(normalized.values())
}

export function resolveProcessingPrefillLines(
  items: Array<Partial<ProcessingPrefillItem>> = [],
  families: EmployeeOrderProductFamily[] = [],
): { lines: ProcessingPrefillDraftLine[]; unavailable: ProcessingPrefillItem[] } {
  const requested = normalizeProcessingPrefillItems(items)
  const lines: ProcessingPrefillDraftLine[] = []
  const unavailable: ProcessingPrefillItem[] = []

  for (const item of requested) {
    let matched: ProcessingPrefillDraftLine | null = null
    for (const family of families) {
      const spec = (family.specs || []).find((candidate) => {
        if (Number(item.bom_spec_id || 0) > 0) {
          return Number(candidate.bom_spec_id || 0) === Number(item.bom_spec_id)
            && Number(candidate.bom_variant_id || 0) === Number(item.bom_variant_id)
            && Number(family.parent_product_id || candidate.product_id || 0) === item.product_id
        }
        const productID = Number(candidate.sku_id || candidate.product_id || 0)
        const specG = productSpecWeightG(candidate)
        return productID === item.product_id && (item.spec_g <= 0 || specG === item.spec_g)
      })
      if (!spec) continue
      const canonical = Number(spec.bom_spec_id || 0) > 0
      matched = {
        product_id: canonical
          ? Number(family.parent_product_id || spec.product_id || 0)
          : Number(spec.sku_id || spec.product_id || 0),
        bom_spec_id: canonical ? Number(spec.bom_spec_id || 0) : undefined,
        bom_variant_id: canonical ? Number(spec.bom_variant_id || 0) : undefined,
        product_name: spec.sku_name || family.customer_product_display_name || family.name,
        spec_g: canonical ? 0 : productSpecWeightG(spec),
        spec_label: productSpecLabel(spec),
        inventory_unit: canonical ? String(spec.inventory_unit || '').trim() : undefined,
        qty: 0,
      }
      break
    }
    if (matched) lines.push(matched)
    else unavailable.push(item)
  }

  return { lines, unavailable }
}
