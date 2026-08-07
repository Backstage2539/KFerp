import type {
  CustomerInventorySummary,
  EmployeeOrderProductFamily,
} from '../api/customerPortal'
import { productSpecLabel, productSpecWeightG } from './employeeOrder'

export type ProcessingPrefillItem = {
  product_id: number
  spec_g: number
  product_name?: string
  sku_code?: string
}

export type ProcessingPrefillDraftLine = {
  product_id: number
  product_name: string
  spec_g: number
  spec_label: string
  qty: number
}

export type CustomerInventorySelection = Record<string, CustomerInventorySummary>

export function customerInventoryItemKey(item: Pick<CustomerInventorySummary, 'product_id' | 'spec_g'>): string {
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
  item: Pick<CustomerInventorySummary, 'product_id' | 'spec_g'>,
): string {
  return `/pages/customer-inventory-detail/customer-inventory-detail?product_id=${Number(item.product_id || 0)}&spec_g=${Number(item.spec_g || 0)}`
}

export function normalizeProcessingPrefillItems(
  items: Array<Partial<ProcessingPrefillItem>> = [],
): ProcessingPrefillItem[] {
  const normalized = new Map<string, ProcessingPrefillItem>()
  for (const item of items) {
    const productID = Number(item.product_id || 0)
    const specG = Number(item.spec_g || 0)
    if (productID <= 0 || specG < 0) continue
    const key = `${productID}:${specG}`
    if (normalized.has(key)) continue
    normalized.set(key, {
      product_id: productID,
      spec_g: specG,
      product_name: String(item.product_name || '').trim() || undefined,
      sku_code: String(item.sku_code || '').trim() || undefined,
    })
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
        const productID = Number(candidate.sku_id || candidate.product_id || 0)
        const specG = productSpecWeightG(candidate)
        return productID === item.product_id && (item.spec_g <= 0 || specG === item.spec_g)
      })
      if (!spec) continue
      matched = {
        product_id: Number(spec.sku_id || spec.product_id || 0),
        product_name: spec.sku_name || family.customer_product_display_name || family.name,
        spec_g: productSpecWeightG(spec),
        spec_label: productSpecLabel(spec),
        qty: 0,
      }
      break
    }
    if (matched) lines.push(matched)
    else unavailable.push(item)
  }

  return { lines, unavailable }
}
