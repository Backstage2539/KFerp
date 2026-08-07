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
  product_name: string
  spec_g: number
  spec_label: string
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
  return {
    ...line,
    product_id: Number(spec.sku_id || spec.product_id || 0),
    spec_g: productSpecWeightG(spec),
    spec_label: productSpecLabel(spec),
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
    const specG = Number(line.spec_g || 0)
    const qty = Number(line.qty || 0)
    if (productID <= 0 || !Number.isFinite(qty) || qty <= 0) continue
    const key = `${productID}:${specG}`
    const current = merged.get(key)
    if (current) current.qty += qty
    else merged.set(key, { product_id: productID, spec_g: specG, qty })
  }
  return Array.from(merged.values())
}
