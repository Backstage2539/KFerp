import type { EmployeeOrderProductFamily } from '../api/customerPortal'
import type { Capability } from './capabilities'
import {
  filterEmployeeOrderProductFamilies,
  type EmployeeOrderProductCategory,
} from './employeeOrder'

export type ProcessingTargetLine = {
  product_id: number
  bom_spec_id?: number
  bom_variant_id?: number
  inventory_unit?: string
  spec_g: number
  qty: number
}

export type ProductionPreviewMaterial = {
  material_id: number
  required_g?: number
  required_units?: number
  available_g?: number
  available_units?: number
  shortage_g?: number
  shortage_units?: number
}

export type ProductionPreviewLike = {
  complete?: boolean
  canSubmit?: boolean
  materials?: ProductionPreviewMaterial[]
}

export function scopedFulfillmentProductFamilies(
  families: EmployeeOrderProductFamily[] = [],
  customerID = 0,
  query = '',
  category: EmployeeOrderProductCategory = 'all',
): EmployeeOrderProductFamily[] {
  return filterEmployeeOrderProductFamilies(families, customerID, query, category)
}

export function mergeProcessingTargetLines(lines: ProcessingTargetLine[] = []): ProcessingTargetLine[] {
  const merged = new Map<string, ProcessingTargetLine>()
  for (const line of lines) {
    const productID = Number(line.product_id || 0)
    const bomSpecID = Number(line.bom_spec_id || 0)
    const bomVariantID = Number(line.bom_variant_id || 0)
    const specG = Number(line.spec_g || 0)
    const qty = Number(line.qty || 0)
    if (productID <= 0 || qty <= 0) continue
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

export function productionSubmissionBlockReason(preview?: ProductionPreviewLike | null): string {
  if (!preview) return '请先完成 BOM 试算'
  if (preview.complete === false) return '当前目标商品没有可用 BOM 配置'
  const hasShortage = (preview.materials || []).some((item) => (
    Number(item.shortage_g || 0) > 0
    || Number(item.shortage_units || 0) > 0
    || Number(item.required_g || 0) > Number(item.available_g || 0)
    || Number(item.required_units || 0) > Number(item.available_units || 0)
  ))
  if (hasShortage) return '物料库存不足，无法提交生产工单'
  return preview.canSubmit === false ? '当前生产配置无法提交' : ''
}

const productionStatusLabels: Record<string, string> = {
  awaiting_schedule: '待排产',
  planned: '已排产',
  released: '已下达',
  running: '生产中',
  paused: '已暂停',
  partially_completed: '部分完成',
  completed: '已完成',
  cancelled: '已取消',
  canceled: '已取消',
}

const directShipStatusLabels: Record<string, string> = {
  pending: '待处理',
  reserved: '待发货',
  partially_shipped: '部分发货',
  shipped: '已发货',
  delivered: '已签收',
  cancelled: '已取消',
  canceled: '已取消',
}

export function productionStatusLabel(status?: string): string {
  const value = String(status || '').trim()
  return productionStatusLabels[value] || value || '待排产'
}

export function directShipStatusLabel(status?: string): string {
  const value = String(status || '').trim()
  return directShipStatusLabels[value] || value || '待处理'
}

export function canShowFactoryProductLinks(capabilities: Capability[] = []): boolean {
  const enabled = new Set(capabilities.filter((item) => item.enabled).map((item) => item.code))
  return enabled.has('product_order') && enabled.has('bean_list')
}
