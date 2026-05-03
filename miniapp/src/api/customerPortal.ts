import { miniRequest } from './client'
import type { Capability } from '../utils/capabilities'
import type { ServiceKey } from '../utils/servicePage'

export type CustomerBinding = {
  customer_id: number
  customer_name: string
  role: string
  status: string
}

export type LoginResponse = {
  token: string
  mini_user_id: number
  current_customer_id: number
  bindings: CustomerBinding[]
  capabilities: Capability[]
}

export type MeResponse = {
  mini_user_id: number
  current_customer_id: number
  current_customer_name: string
  bindings: CustomerBinding[]
  capabilities: Capability[]
}

export type ServiceMetric = {
  label: string
  value: string
}

export type BeanListSummary = {
  id: number
  list_type: string
  version_no: string
  status: string
  published_at: string
  changelog: string
  groups?: BeanListGroupSummary[]
}

export type BeanListGroupSummary = {
  category: string
  items: BeanListProductSummary[]
}

export type BeanListProductSummary = {
  code?: string
  name: string
  badge_label?: string
  recommended_use?: string
  flavor?: string
  description?: string
  prices?: BeanListPriceSummary[]
}

export type BeanListPriceSummary = {
  label: string
  value: string
  red?: boolean
}

export type ProductSummary = {
  id: number
  name: string
  roast_level: string
  default_price: string
  retail_price_100g: string
  retail_price_200g: string
  retail_price_227g: string
  retail_price_250g: string
}

export type CustomerOrderSummary = {
  id: number
  order_no: string
  order_date: string
  process_status: string
  pay_status: string
  ship_status: string
  ship_tracking_no: string
  grand_total: string
  shipping_amount: string
  items?: CustomerOrderItemSummary[]
}

export type CustomerOrderItemSummary = {
  id: number
  item_name: string
  spec: string
  qty: string
  unit: string
  unit_price: string
  line_total: string
}

export type DirectShipBatch = {
  id: number
  batch_no: string
  source_name: string
  status: string
  total_rows: number
  valid_rows: number
  invalid_rows: number
  note: string
  created_at: string
}

export type InventoryItem = {
  id: number
  item_type: string
  item_id: number
  item_name: string
  spec_g: number
  warehouse: string
  qty_g: number
  qty_units: number
  status: string
  note: string
  updated_at: string
}

export type ProcessingRequest = {
  id: number
  request_no: string
  input_material_id: number
  input_material_name: string
  input_qty_g: number
  target_product_id: number
  target_product_name: string
  target_spec_g: number
  target_qty: number
  status: string
  note: string
  created_at: string
  accepted_at: string
  linked_work_order_id: number
}

export type FeeItem = {
  id: number
  source_type: string
  source_id: number
  fee_type: string
  amount: string
  currency: string
  occurred_at: string
  settlement_batch_id: number
  status: string
  note: string
}

export type SettlementBatch = {
  id: number
  settlement_no: string
  period_from: string
  period_to: string
  status: string
  total_amount: string
  confirmed_at: string
  paid_at: string
  created_at: string
}

export type ServicePageResponse = {
  key: ServiceKey
  title: string
  capability: string
  current_customer_id: number
  current_customer_name: string
  summary: ServiceMetric[]
  bean_lists?: BeanListSummary[]
  products?: ProductSummary[]
  orders?: CustomerOrderSummary[]
  direct_ship_batches?: DirectShipBatch[]
  inventory?: InventoryItem[]
  processing_requests?: ProcessingRequest[]
  fee_items?: FeeItem[]
  settlement_batches?: SettlementBatch[]
}

export type CreateDirectShipBatchPayload = {
  source_name: string
  total_rows: number
  note?: string
}

export type CreateProcessingRequestPayload = {
  input_material_id: number
  input_qty_g: number
  target_product_id: number
  target_spec_g: number
  target_qty: number
  note?: string
}

export function loginWithCode(code: string): Promise<LoginResponse> {
  return miniRequest<LoginResponse>('/api/mini/login', { method: 'POST', data: { code } })
}

export function fetchMe(token: string): Promise<MeResponse> {
  return miniRequest<MeResponse>('/api/mini/me', { token })
}

export function fetchServicePage(token: string, key: ServiceKey): Promise<ServicePageResponse> {
  return miniRequest<ServicePageResponse>(`/api/mini/services/${key}`, { token })
}

export function createDirectShipBatch(
  token: string,
  payload: CreateDirectShipBatchPayload,
): Promise<DirectShipBatch> {
  return miniRequest<DirectShipBatch>('/api/mini/direct-ship/batches', {
    method: 'POST',
    token,
    data: payload,
  })
}

export function createProcessingRequest(
  token: string,
  payload: CreateProcessingRequestPayload,
): Promise<ProcessingRequest> {
  return miniRequest<ProcessingRequest>('/api/mini/processing-requests', {
    method: 'POST',
    token,
    data: payload,
  })
}
