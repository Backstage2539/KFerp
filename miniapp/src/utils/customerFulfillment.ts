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

export function processingPreviewValidationError(
  lines: ProcessingTargetLine[] = [],
  expectedCompletionDate = '',
): string {
  const items = mergeProcessingTargetLines(lines)
  if (!items.length) return '请选择至少一个目标商品规格'
  if (!String(expectedCompletionDate || '').trim()) return '请填写期望完成日期'
  return ''
}

export function processingPreviewErrorMessage(cause: unknown): string {
  const message = cause instanceof Error ? cause.message.trim() : ''
  if (!message || message === 'invalid request') {
    return 'BOM 试算请求无效，请检查商品、数量和期望完成日期'
  }
  return message
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
  awaiting_schedule: '待接单',
  planned: '待接单',
  released: '已接单',
  running: '开始生产',
  paused: '开始生产（暂停）',
  partially_completed: '开始生产（部分入库）',
  completed: '生产完成',
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
  return productionStatusLabels[value] || value || '待接单'
}

export function directShipStatusLabel(status?: string): string {
  const value = String(status || '').trim()
  return directShipStatusLabels[value] || value || '待处理'
}

type ProcessingProgressItem = {
  qty?: number
  actual_inbound_qty?: number
  output_reserved_qty?: number
  output_converted_qty?: number
  output_released_qty?: number
}

export function processingRequestProgress(request: { items?: ProcessingProgressItem[] } = {}) {
  const items = Array.isArray(request.items) ? request.items : []
  return items.reduce((result, item) => {
    const requested = Math.max(0, Number(item.qty || 0))
    const inbound = Math.min(requested, Math.max(0, Number(item.actual_inbound_qty || 0)))
    const reserved = Math.max(0, Number(item.output_reserved_qty || 0))
    const converted = Math.max(0, Number(item.output_converted_qty || 0))
    const released = Math.max(0, Number(item.output_released_qty || 0))
    result.requestedQty += requested
    result.inboundQty += inbound
    result.activeReservedQty += Math.max(0, reserved - converted - released)
    result.remainingReservableQty += Math.max(0, requested - inbound - Math.max(0, reserved - converted - released))
    return result
  }, { requestedQty: 0, inboundQty: 0, activeReservedQty: 0, remainingReservableQty: 0 })
}

type ProcessingTimelineInput = {
  status?: string
  created_at?: string
  accepted_at?: string
  started_at?: string
  completed_at?: string
}

export function processingRequestTimeline(request: ProcessingTimelineInput = {}) {
  const statusRank: Record<string, number> = {
    awaiting_schedule: 0,
    planned: 0,
    released: 1,
    paused: 2,
    running: 2,
    partially_completed: 2,
    completed: 3,
  }
  const currentRank = statusRank[String(request.status || '')] ?? 0
  const steps = [
    { key: 'submitted', label: '待接单', time: request.created_at },
    { key: 'accepted', label: '已接单', time: request.accepted_at },
    { key: 'running', label: '开始生产', time: request.started_at },
    { key: 'completed', label: '生产完成', time: request.completed_at },
  ]
  return steps.map((step, index) => ({
    ...step,
    time: String(step.time || '').trim() || '暂无记录',
    state: index < currentRank ? 'done' : index === currentRank ? 'current' : 'pending',
  }))
}

type DirectShipAvailabilityInput = {
  qty?: number
  stock_available_qty?: number
  production_available_qty?: number
  production_allocations?: Array<{ qty?: number; processing_request_id?: number }>
}

export function directShipAvailabilityBreakdown(input: DirectShipAvailabilityInput = {}) {
  const requested = Math.max(0, Number(input.qty || 0))
  const stockAvailable = Math.max(0, Number(input.stock_available_qty || 0))
  const productionAvailable = Math.max(0, Number(input.production_available_qty || 0))
  const stockQty = Math.min(requested, stockAvailable)
  const allocationQty = (input.production_allocations || []).reduce((sum, item) => sum + Math.max(0, Number(item.qty || 0)), 0)
  const productionQty = Math.min(Math.max(0, requested - stockQty), allocationQty || productionAvailable)
  return {
    stockQty,
    productionQty,
    orderableQty: stockAvailable + productionAvailable,
    remainingProductionQty: Math.max(0, productionAvailable - productionQty),
  }
}

export function canShowFactoryProductLinks(capabilities: Capability[] = []): boolean {
  const enabled = new Set(capabilities.filter((item) => item.enabled).map((item) => item.code))
  return enabled.has('product_order') && enabled.has('bean_list')
}
