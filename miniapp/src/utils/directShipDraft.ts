import type { EmployeeOrderProductFamily, EmployeeOrderProductSpec, ProcessingTargetItem } from '../api/customerPortal'
import {
  defaultProductSpec,
  employeeOrderProductFamilyKey,
  productSpecLabel,
  productSpecWeightG,
} from './employeeOrder'

export type DirectShipDraftLine = {
  key: string
  product_family_key: string
  product_id: number
  bom_spec_id?: number
  bom_variant_id?: number
  product_name: string
  spec_g: number
  spec_label: string
  inventory_unit?: string
  qty: number
}

let directShipDraftLineSequence = 0

export function createDirectShipDraftLine(key = ''): DirectShipDraftLine {
  directShipDraftLineSequence += 1
  return {
    key: key || `direct-ship-line-${Date.now()}-${directShipDraftLineSequence}`,
    product_family_key: '',
    product_id: 0,
    product_name: '',
    spec_g: 0,
    spec_label: '',
    qty: 1,
  }
}

export function selectDirectShipDraftSpec(
  line: DirectShipDraftLine,
  spec: EmployeeOrderProductSpec,
): DirectShipDraftLine {
  const bomSpecID = Number(spec.bom_spec_id || 0)
  const bomVariantID = Number(spec.bom_variant_id || 0)
  const canonical = bomSpecID > 0 || bomVariantID > 0 || spec.migration_state === 'cutover'
  return {
    ...line,
    product_id: Number(spec.sku_id || spec.product_id || 0),
    bom_spec_id: canonical ? bomSpecID : undefined,
    bom_variant_id: canonical ? bomVariantID : undefined,
    spec_g: canonical ? 0 : productSpecWeightG(spec),
    spec_label: productSpecLabel(spec),
    inventory_unit: canonical ? String(spec.inventory_unit || '').trim() : undefined,
  }
}

export function selectDirectShipDraftProduct(
  line: DirectShipDraftLine,
  family: EmployeeOrderProductFamily,
): DirectShipDraftLine | null {
  const spec = defaultProductSpec(family)
  if (!spec) return null
  return selectDirectShipDraftSpec({
    ...line,
    product_family_key: employeeOrderProductFamilyKey(family),
    product_name: String(
      family.customer_product_display_name
      || family.alias_name
      || family.name
      || '',
    ).trim(),
  }, spec)
}

function isBlankDirectShipDraftLine(line: DirectShipDraftLine): boolean {
  return Number(line.product_id || 0) <= 0
    && !String(line.product_family_key || '').trim()
    && !String(line.product_name || '').trim()
    && !String(line.spec_label || '').trim()
    && Number(line.qty) === 1
}

export function directShipDraftValidation(lines: DirectShipDraftLine[] = []): string {
  const entered = lines.filter((line) => !isBlankDirectShipDraftLine(line))
  if (!entered.length) return '请至少选择一个商品规格并填写数量'
  if (entered.some((line) => Number(line.product_id || 0) <= 0 || !String(line.spec_label || '').trim())) {
    return '请完整选择每一行的商品和规格'
  }
  if (entered.some((line) => !Number.isFinite(Number(line.qty)) || Number(line.qty) <= 0)) {
    return '商品数量必须大于 0'
  }
  return ''
}

export function buildDirectShipDraftItems(lines: DirectShipDraftLine[] = []): ProcessingTargetItem[] {
  const merged = new Map<string, ProcessingTargetItem>()
  for (const line of lines) {
    if (isBlankDirectShipDraftLine(line)) continue
    const productID = Number(line.product_id || 0)
    const bomSpecID = Number(line.bom_spec_id || 0)
    const bomVariantID = Number(line.bom_variant_id || 0)
    const specG = Number(line.spec_g || 0)
    const qty = Number(line.qty || 0)
    if (productID <= 0 || !Number.isFinite(qty) || qty <= 0) continue
    const canonical = bomSpecID > 0 || bomVariantID > 0
    if (canonical && (bomSpecID <= 0 || bomVariantID <= 0)) continue
    const key = canonical
      ? `${productID}:bom_spec:${bomSpecID}:${bomVariantID}`
      : `${productID}:legacy:${specG}`
    const current = merged.get(key)
    if (current) current.qty += qty
    else if (canonical) {
      merged.set(key, {
        product_id: productID,
        bom_spec_id: bomSpecID,
        bom_variant_id: bomVariantID,
        inventory_unit: String(line.inventory_unit || '').trim() || undefined,
        spec_g: 0,
        qty,
      })
    } else merged.set(key, { product_id: productID, spec_g: specG, qty })
  }
  return Array.from(merged.values())
}
