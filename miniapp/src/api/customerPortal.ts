import { miniRequest } from './client'
import type { Capability } from '../utils/capabilities'
import type { MallOrderPayload, MallProduct } from '../utils/mall'
import type { ServiceKey } from '../utils/servicePage'
import type { MiniappThemeKey } from '../utils/themes'

export type ProductKind = 'roasted' | 'roasted_bean' | 'green_bean' | 'drip_bag'
export type SalesUnit = 'bag' | 'box'

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
  requires_acknowledgement?: boolean
  diff?: BeanListDiffSummary
  background_color?: string
  font_color?: string
  background_image?: string
  logo_image?: string
  groups?: BeanListGroupSummary[]
}

export type BeanListDiffSummary = {
  previous_version_no?: string
  added?: BeanListDiffItem[]
  removed?: BeanListDiffItem[]
  changed?: BeanListDiffChange[]
}

export type BeanListDiffItem = {
  code?: string
  name: string
}

export type BeanListDiffChange = {
  code?: string
  name: string
  fields?: string[]
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
  bean_list_quality?: BeanListQualitySummary
  highlight_terms?: string[]
  prices?: BeanListPriceSummary[]
}

export type BeanListQualitySummary = {
  factory_flavor_description?: string
  moisture?: string
  density?: string
  inspection_created_at?: string
  inspection_reference_no?: string
}

export type BeanListPriceSummary = {
  label: string
  value: string
  red?: boolean
}

export type ResaleGradientTemplateTier = {
  id: number
  label: string
  min_weight_g: number
  max_weight_g?: number | null
  position?: number
}

export type ResaleGradientTemplate = {
  id: number
  name: string
  display_unit: string
  tiers?: ResaleGradientTemplateTier[]
}

export type ResaleBeanListPage = {
  factory_supply_bean_lists: BeanListSummary[]
  customer_resale_bean_lists: BeanListSummary[]
  gradient_templates: ResaleGradientTemplate[]
  factory_price_table_groups?: CustomerPriceTableGroup[]
  customer_price_table_groups?: CustomerPriceTableGroup[]
  current_customer_id?: number
  current_customer_name?: string
}

export type CustomerProductClassificationTemplate = {
  id: number
  customer_id: number
  derived_from_template_id?: number
  name: string
  read_only?: boolean
}

export type CustomerProductCategory = {
  id: number
  template_id: number
  parent_id: number
  name: string
  level: number
  sort_order: number
  product_count?: number
}

export type CustomerProductSummary = {
  id: number
  product_id: number
  code?: string
  name: string
  product_kind?: string
  list_type: string
  list_type_label: string
  category_id?: number
  category_name?: string
  sort_order?: number
}

export type CustomerPriceTableGroup = {
  list_type: string
  list_type_label: string
  product_count: number
  price_table_count: number
  latest_version?: BeanListSummary
  versions?: BeanListSummary[]
}

export type CustomerProductsPage = {
  current_customer_id?: number
  current_customer_name?: string
  classification_template?: CustomerProductClassificationTemplate
  categories: CustomerProductCategory[]
  products: CustomerProductSummary[]
  factory_price_table_groups: CustomerPriceTableGroup[]
  customer_price_table_groups: CustomerPriceTableGroup[]
}

export type CustomerProductCategoryPayload = {
  name?: string
  parent_id?: number
  sort_order?: number
}

export type CustomerProductCategoryMovePayload = {
  direction?: 'up' | 'down' | string
  parent_id?: number
  sort_order?: number
}

export type CustomerProductCategoryAssignPayload = {
  category_id: number
  sort_order?: number
}

export type ResaleBeanListEditor = {
  source: BeanListSummary
  next_version_no: string
  gradient_templates?: ResaleGradientTemplate[]
}

export type ResaleBeanListPriceRule = {
  add_amount: number
  multiplier: number
}

export type ResaleBeanListItemOverride = {
  code: string
  label?: string
  price?: number
  badge_label?: string
  recommended_use?: string
  description?: string
  highlight_terms?: string[]
}

export type ResaleBeanListCommand = {
  source_publication_id: number
  version_no: string
  gradient_template_id: number
  selected_item_codes: string[]
  config: Record<string, unknown>
  price_rule: ResaleBeanListPriceRule
  item_overrides?: ResaleBeanListItemOverride[]
  changelog?: string
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
  product_kind?: ProductKind
  sales_units?: SalesUnit[]
  drip_bag_grams?: number
  drip_box_bag_count?: number
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
  payment_method: string
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
  product_kind?: string
  spec: string
  qty: string
  unit: string
  unit_price: string
  line_total: string
  bean_list_publication_id: number
  bean_list_version_no: string
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
  sales_unit?: SalesUnit
  unit_bag_count?: number
  unit_bean_g?: number
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

export type MiniLoginMode = 'wechat' | 'phone_verify'

export type MiniLoginFields = {
  code: string
  phone?: string
  phoneCode?: string
  nickname?: string
}

export function buildMiniLoginPayload(mode: MiniLoginMode, fields: MiniLoginFields): Record<string, string> {
  const payload: Record<string, string> = {
    mode,
    code: String(fields.code || '').trim(),
  }
  if (String(fields.phone || '').trim()) payload.phone = String(fields.phone || '').trim()
  if (String(fields.phoneCode || '').trim()) payload.phone_code = String(fields.phoneCode || '').trim()
  if (String(fields.nickname || '').trim()) payload.nickname = String(fields.nickname || '').trim()
  return payload
}

export function loginWithCode(code: string, phone?: string, nickname?: string): Promise<LoginResponse> {
  return miniRequest<LoginResponse>('/api/mini/login', {
    method: 'POST',
    data: buildMiniLoginPayload('wechat', { code, phone, nickname }),
  })
}

export function loginWithPhoneVerify(code: string, phoneCode: string, nickname?: string): Promise<LoginResponse> {
  return miniRequest<LoginResponse>('/api/mini/login', {
    method: 'POST',
    data: buildMiniLoginPayload('phone_verify', { code, phoneCode, nickname }),
  })
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

export function buildBeanListAckPath(publicationID: number): string {
  return `/api/mini/bean-lists/${Number(publicationID || 0)}/ack`
}

export function buildResaleBeanListsPath(): string {
  return '/api/mini/resale-bean-lists'
}

export function buildResaleBeanListEditorPath(sourcePublicationID: number): string {
  return `/api/mini/resale-bean-lists/${Number(sourcePublicationID || 0)}/editor`
}

export function buildResaleBeanListPDFPath(publicationID: number): string {
  return `/api/mini/resale-bean-lists/${Number(publicationID || 0)}.pdf`
}

export function buildResaleBeanListPNGPath(publicationID: number): string {
  return `/api/mini/resale-bean-lists/${Number(publicationID || 0)}.png`
}

export function buildCustomerProductsPath(): string {
  return '/api/mini/customer-products'
}

export function buildCustomerProductCategoriesPath(): string {
  return '/api/mini/customer-products/categories'
}

export function buildCustomerProductCategoryPath(categoryID: number): string {
  return `/api/mini/customer-products/categories/${Number(categoryID || 0)}`
}

export function buildCustomerProductCategoryMovePath(categoryID: number): string {
  return `/api/mini/customer-products/categories/${Number(categoryID || 0)}/move`
}

export function buildCustomerProductCategoryAssignPath(productID: number): string {
  return `/api/mini/customer-products/${Number(productID || 0)}/category`
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

export function acknowledgeBeanListVersion(token: string, publicationID: number): Promise<{ acknowledged: boolean }> {
  return miniRequest<{ acknowledged: boolean }>(buildBeanListAckPath(publicationID), {
    method: 'POST',
    token,
  })
}

export function fetchResaleBeanLists(token: string): Promise<ResaleBeanListPage> {
  return miniRequest<ResaleBeanListPage>(buildResaleBeanListsPath(), { token })
}

export function fetchCustomerProducts(token: string): Promise<CustomerProductsPage> {
  return miniRequest<CustomerProductsPage>(buildCustomerProductsPath(), { token })
}

export function createCustomerProductCategory(token: string, payload: CustomerProductCategoryPayload): Promise<CustomerProductCategory> {
  return miniRequest<CustomerProductCategory>(buildCustomerProductCategoriesPath(), {
    method: 'POST',
    token,
    data: payload,
  })
}

export function updateCustomerProductCategory(token: string, categoryID: number, payload: CustomerProductCategoryPayload): Promise<CustomerProductCategory> {
  return miniRequest<CustomerProductCategory>(buildCustomerProductCategoryPath(categoryID), {
    method: 'PUT',
    token,
    data: payload,
  })
}

export function deleteCustomerProductCategory(token: string, categoryID: number): Promise<{ deleted: boolean }> {
  return miniRequest<{ deleted: boolean }>(buildCustomerProductCategoryPath(categoryID), {
    method: 'DELETE',
    token,
  })
}

export function moveCustomerProductCategory(token: string, categoryID: number, payload: CustomerProductCategoryMovePayload): Promise<CustomerProductCategory> {
  return miniRequest<CustomerProductCategory>(buildCustomerProductCategoryMovePath(categoryID), {
    method: 'POST',
    token,
    data: payload,
  })
}

export function assignCustomerProductCategory(token: string, productID: number, payload: CustomerProductCategoryAssignPayload): Promise<CustomerProductSummary> {
  return miniRequest<CustomerProductSummary>(buildCustomerProductCategoryAssignPath(productID), {
    method: 'POST',
    token,
    data: payload,
  })
}

export function fetchResaleBeanListEditor(token: string, sourcePublicationID: number): Promise<ResaleBeanListEditor> {
  return miniRequest<ResaleBeanListEditor>(buildResaleBeanListEditorPath(sourcePublicationID), { token })
}

export function saveResaleBeanListDraft(token: string, payload: ResaleBeanListCommand): Promise<BeanListSummary> {
  return miniRequest<BeanListSummary>('/api/mini/resale-bean-lists/drafts', {
    method: 'POST',
    token,
    data: payload,
  })
}

export function publishResaleBeanList(token: string, payload: ResaleBeanListCommand): Promise<BeanListSummary> {
  return miniRequest<BeanListSummary>('/api/mini/resale-bean-lists/publications', {
    method: 'POST',
    token,
    data: payload,
  })
}
