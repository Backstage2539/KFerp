import { miniRequest } from './client'
import type { Capability } from '../utils/capabilities'
import type { MallOrderPayload, MallProduct } from '../utils/mall'
import type { ServiceKey } from '../utils/servicePage'
import type { MiniappThemeKey } from '../utils/themes'

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
  theme_key?: MiniappThemeKey | string
  miniapp_entry_mode?: MiniappEntryMode | string
  bindings: CustomerBinding[]
  capabilities: Capability[]
}

export type MeResponse = {
  mini_user_id: number
  current_customer_id: number
  current_customer_name: string
  theme_key?: MiniappThemeKey | string
  miniapp_entry_mode?: MiniappEntryMode | string
  bindings: CustomerBinding[]
  capabilities: Capability[]
}

export type MiniappEntryMode = 'services' | 'mall'

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
  pdf_url?: string
  cache_key: string
  title?: string
  subtitle?: string
  list_type_label?: string
  brand_name?: string
  brand_intro?: string
  layout_style?: 'card' | 'table' | string
  cards_per_row?: number
  show_version?: boolean
  show_changelog?: boolean
  show_category_numbers?: boolean
  background_color?: string
  font_color?: string
  background_image?: string
  logo_image?: string
  groups?: BeanListGroupSummary[]
}

export type BeanListGroupSummary = {
  category: string
  show_category?: boolean
  items: BeanListProductSummary[]
}

export type BeanListProductSummary = {
  code?: string
  name: string
  badge?: string
  badge_label?: string
  recommended_use?: string
  flavor?: string
  description?: string
  highlight_terms?: string[]
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
  receiver_name: string
  receiver_phone: string
  receiver_address: string
  process_status: string
  pay_status: string
  ship_status: string
  ship_tracking_no: string
  grand_total: string
  shipping_amount: string
  sales_order_url?: string
  delivery_note_url?: string
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

export type FulfillmentOrder = {
  order_id: number
  order_no: string
  portal_service_code: string
  source_warehouse: string
}

export type MallPageResponse = {
  theme_key?: MiniappThemeKey | string
  miniapp_entry_mode?: MiniappEntryMode | string
  current_customer_id: number
  current_customer_name: string
  products: MallProduct[]
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
  theme_key?: MiniappThemeKey | string
  miniapp_entry_mode?: MiniappEntryMode | string
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

export type CreateFulfillmentOrderPayload = {
  service_code: 'direct_ship' | 'processing_ship' | 'product_order' | string
  recipient_name: string
  recipient_phone: string
  recipient_address: string
  recipient_company?: string
  product_id: number
  product_name?: string
  spec_g: number
  qty: number
  shipping_amount?: number
  note?: string
}

export type ServicePageFilters = {
  q?: string
  date_from?: string
  date_to?: string
  process_status?: string
  pay_status?: string
  ship_status?: string
}

export function loginWithCode(code: string): Promise<LoginResponse> {
  return miniRequest<LoginResponse>('/api/mini/login', { method: 'POST', data: { code } })
}

export function buildPasswordLoginPath(): string {
  return '/api/mini/login/password'
}

export function loginWithPassword(login: string, password: string): Promise<LoginResponse> {
  return miniRequest<LoginResponse>(buildPasswordLoginPath(), {
    method: 'POST',
    data: { login, password },
  })
}

export function fetchMe(token: string): Promise<MeResponse> {
  return miniRequest<MeResponse>('/api/mini/me', { token })
}

export function buildServicePagePath(key: ServiceKey, filters: ServicePageFilters = {}): string {
  const params = [
    ['q', filters.q],
    ['date_from', filters.date_from],
    ['date_to', filters.date_to],
    ['process_status', filters.process_status],
    ['pay_status', filters.pay_status],
    ['ship_status', filters.ship_status],
  ]
    .filter(([, value]) => String(value || '').trim() !== '')
    .map(([name, value]) => `${name}=${encodeURIComponent(String(value).trim())}`)
  const suffix = params.length ? `?${params.join('&')}` : ''
  return `/api/mini/services/${key}${suffix}`
}

export function buildMallPagePath(): string {
  return '/api/mini/mall'
}

export function buildMallOrderPath(): string {
  return '/api/mini/mall/orders'
}

export function buildSwitchCustomerPath(): string {
  return '/api/mini/current-customer'
}

export function fetchServicePage(token: string, key: ServiceKey, filters: ServicePageFilters = {}): Promise<ServicePageResponse> {
  return miniRequest<ServicePageResponse>(buildServicePagePath(key, filters), { token })
}

export function fetchMallPage(token: string): Promise<MallPageResponse> {
  return miniRequest<MallPageResponse>(buildMallPagePath(), { token })
}

export function createMallOrder(token: string, payload: MallOrderPayload): Promise<FulfillmentOrder> {
  return miniRequest<FulfillmentOrder>(buildMallOrderPath(), {
    method: 'POST',
    token,
    data: payload,
  })
}

export function switchCurrentCustomer(token: string, customerID: number): Promise<MeResponse> {
  return miniRequest<MeResponse>(buildSwitchCustomerPath(), {
    method: 'POST',
    token,
    data: { customer_id: customerID },
  })
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

export function createFulfillmentOrder(
  token: string,
  payload: CreateFulfillmentOrderPayload,
): Promise<FulfillmentOrder> {
  return miniRequest<FulfillmentOrder>('/api/mini/fulfillment-orders', {
    method: 'POST',
    token,
    data: payload,
  })
}
