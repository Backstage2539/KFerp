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
  account_type?: 'employee' | 'customer' | string
  employee_id?: number
  employee_name?: string
  roles?: string[]
  permissions?: string[]
  bindings: CustomerBinding[]
  capabilities: Capability[]
}

export type MeResponse = {
  mini_user_id: number
  current_customer_id: number
  current_customer_name: string
  theme_key?: MiniappThemeKey | string
  miniapp_entry_mode?: MiniappEntryMode | string
  account_type?: 'employee' | 'customer' | string
  employee_id?: number
  employee_name?: string
  roles?: string[]
  permissions?: string[]
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
  copy_source_type?: 'factory_supply' | 'customer_resale' | string
  source: BeanListSummary
  price_source?: BeanListSummary
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
  clear_badge?: boolean
  recommended_use?: string
  description?: string
  highlight_terms?: string[]
  clear_highlight_terms?: boolean
}

export type ResaleBeanListCategoryDraft = {
  id?: string
  source_category?: string
  name: string
  item_codes?: string[]
  collapsed?: boolean
  deleted?: boolean
  sort_order?: number
}

export type ResaleBeanListCommand = {
  source_publication_id: number
  version_no: string
  gradient_template_id: number
  selected_item_codes: string[]
  category_drafts?: ResaleBeanListCategoryDraft[]
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
  items?: Array<ProcessingTargetItem & {
    id?: number
    line_no: number
    product_name: string
    status: string
    work_order_id?: number
    work_order_no?: string
    production_plan_id?: number
  }>
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

export type ProcessingTargetItem = {
  product_id: number
  spec_g: number
  qty: number
  product_name?: string
  sku_code?: string
  spec_label?: string
}

export type CreateProcessingRequestPayload = {
  items: ProcessingTargetItem[]
  note?: string
}

export type ProcessingMaterialPreview = {
  material_id: number
  material_name: string
  unit: string
  component_type: string
  required_g: number
  required_units: number
  factory_inventory_g: number
  factory_inventory_units: number
  factory_wip_g: number
  factory_wip_units: number
  customer_inventory_g: number
  customer_inventory_units: number
  customer_wip_g: number
  customer_wip_units: number
  reserved_g: number
  reserved_units: number
  available_g: number
  available_units: number
  shortage_g: number
  shortage_units: number
}

export type ProcessingTargetPreview = ProcessingTargetItem & {
  line_no: number
  product_name: string
  parent_product_id: number
  need_g: number
  target_warehouse: string
  bom_version_id: number
  bom_version_no: string
  bom_source_product_id: number
  bom_inherited: boolean
  max_producible_qty: number
  material_snapshot?: Record<string, unknown>
  materials: ProcessingMaterialPreview[]
}

export type ProcessingRequestPreview = {
  can_submit: boolean
  complete?: boolean
  items: ProcessingTargetPreview[]
  materials: ProcessingMaterialPreview[]
}

export type DirectShipRequestPayload = {
  idempotency_key: string
  recipient_name: string
  recipient_phone: string
  province?: string
  city?: string
  district?: string
  detail_address: string
  recipient_company?: string
  items: ProcessingTargetItem[]
  note?: string
}

export type DirectShipCatalog = {
  current_customer_id: number
  categories?: Array<{ key: string; label: string }>
  product_families: EmployeeOrderProductFamily[]
}

export type DirectShipPackage = {
  id: number
  order_id: number
  order_no: string
  warehouse: string
  status: string
  carrier_name?: string
  tracking_no?: string
  shipped_at?: string
  delivered_at?: string
  items?: ProcessingTargetItem[]
  events?: Array<{
    time?: string
    status?: string
    description?: string
    location?: string
  }>
}

export type DirectShipRequest = {
  id: number
  request_no: string
  status: string
  recipient_name: string
  recipient_phone: string
  province?: string
  city?: string
  district?: string
  detail_address: string
  items: ProcessingTargetItem[]
  packages?: DirectShipPackage[]
  created_at: string
  note?: string
}

export type DirectShipPreview = {
  can_submit: boolean
  warehouses: Array<{ warehouse: string; items: ProcessingTargetItem[] }>
  shortages?: Array<ProcessingTargetItem & { available_qty: number }>
}

export type CustomerInventorySummary = {
  product_id: number
  product_name: string
  parent_product_id?: number
  sku_code?: string
  spec_g: number
  available_qty: number
  reserved_qty: number
  total_qty: number
  warehouses: string[]
}

export type CustomerInventoryBatch = {
  batch_id: number
  batch_no: string
  product_id: number
  product_name: string
  sku_code?: string
  spec_g: number
  warehouse: string
  production_date?: string
  inbound_at?: string
  available_qty: number
  reserved_qty: number
  quality_status: string
  historical_without_production_date?: boolean
}

export type CustomerBillSummary = {
  id: number
  settlement_no: string
  status: string
  total_amount: string
  currency: string
  confirmed_at?: string
  paid_at?: string
  work_order_count: number
  summary?: string
}

export type CustomerBillDetail = CustomerBillSummary & {
  work_orders: Array<{ work_order_id: number; work_order_no: string; product_name: string; completed_at?: string }>
  lines: Array<{
    work_order_id: number
    fee_type: string
    fee_name: string
    basis: string
    base_quantity: string | number
    unit_price: string
    amount: string
  }>
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

export type EmployeeOrderForm = {
  today: string
  customers: EmployeeOrderCustomer[]
  sources: Array<{ id: number; name: string }>
  order_types: Array<{ id: number; name: string }>
  pay_statuses: Array<{ id: number; name: string }>
  ship_statuses: Array<{ id: number; name: string }>
  product_families: EmployeeOrderProductFamily[]
  products?: EmployeeOrderLegacyProduct[]
}

export type EmployeeOrderCustomer = {
  id: number
  name: string
  py?: string
  pyi?: string
  customer_type?: string
  default_source_id?: number
  default_order_type_id?: number
  contact?: string
  phone?: string
  address?: string
  company_name?: string
  company_address?: string
  company_phone?: string
  receiver_name?: string
  receiver_phone?: string
  receiver_address?: string
  receiver_company?: string
  responsible_employee_id?: number
  responsible_employee_name?: string
  can_maintain?: boolean
}

export type EmployeeCustomer = EmployeeOrderCustomer & {
  active?: boolean
  portal_enabled?: boolean
  capability_template_key?: string
  updated?: string
}

export type EmployeeCustomerOption = {
  id: number
  name: string
}

export type EmployeeCustomerTypeOption = {
  value: string
  label: string
}

export type EmployeeCustomersResponse = {
  rows: EmployeeCustomer[]
  sources: EmployeeCustomerOption[]
  order_types: EmployeeCustomerOption[]
  employees: EmployeeCustomerOption[]
  customer_type_options: EmployeeCustomerTypeOption[]
  is_admin: boolean
  total?: number
  has_next?: boolean
}

export type EmployeeCustomerListQuery = {
  q?: string
  page?: number
  limit?: number
}

export type EmployeeCustomerPayload = {
  name: string
  customer_type: string
  company_name?: string
  company_address?: string
  company_phone?: string
  contact?: string
  phone?: string
  address?: string
  default_source_id: number
  default_order_type_id: number
  responsible_employee_id?: number
  active?: boolean
  portal_enabled?: boolean
}

export type EmployeeCustomerRecipientParseResponse = {
  recipient_name: string
  phone: string
  address: string
  province?: string
  city?: string
  district?: string
  detail_address?: string
}

export type EmployeeOrderDraftItem = {
  item_id?: number
  key: string
  product_family_key: string
  product_family_id: number
  customer_product_alias_id: number
  product_id: number
  product_name: string
  product_kind: string
  spec_label: string
  spec_g: number
  sales_unit: string
  unit_bag_count: number
  unit_bean_g: number
  qty: number
  unit_price: number
  bean_list_publication_id?: number
  bean_list_version_no?: string
  price_override?: boolean
  price_source_json?: string
  discount_type?: string
  discount_value?: number
  discount_amount?: number
  retail_order?: boolean
  validation_error?: string
}

export type EmployeeOrderDraftPayload = {
  edit_revision?: string
  order_date: string
  customer_id: number
  source_id: number
  order_type_id: number
  pay_status_id: number
  ship_status_id: number
  receiver_name: string
  receiver_phone: string
  receiver_address: string
  receiver_company: string
  shipping_amount: number
  discount_amount: number
  notes: string
  items: EmployeeOrderDraftItem[]
}

export type EmployeeOrderDraft = {
  id: number
  payload: EmployeeOrderDraftPayload
  updated_at: string
}

export type EmployeeOrderLegacyProduct = {
  id: number
  name: string
  product_kind?: string
  sales_units?: string[]
  retail_specs?: number[]
}

export type EmployeeOrderProductTier = {
  id?: number
  unit_price: number
  price?: number
  min?: number
  max?: number | null
  min_qty?: number
  max_qty?: number | null
  sales_unit?: string
  unit_bag_count?: number
  publication_id?: number
  bean_list_publication_id?: number
  publication_version_no?: string | number
  bean_list_version_no?: string | number
  list_type?: string
  price_source_json?: string
}

export type EmployeeOrderProductSpec = {
  product_id: number
  sku_id?: number
  sku_code?: string
  sku_name?: string
  py?: string
  pyi?: string
  spec_label?: string
  net_content_qty?: number
  net_content_unit?: string
  is_default_sku?: boolean
  product_kind?: string
  sales_unit?: string
  unit_bag_count?: number
  unit_bean_g?: number
  default_publication_id?: number
  default_publication_version_no?: string | number
  tiers?: EmployeeOrderProductTier[]
}

export type EmployeeOrderProductFamily = {
  parent_product_id: number
  parent_product_name?: string
  name: string
  alias_name?: string
  customer_product_display_name?: string
  customer_item_code?: string
  code?: string
  py?: string
  pyi?: string
  product_code?: string
  product_type_name?: string
  customer_id?: number
  customer_product_alias_id?: number
  default_sku_id?: number
  product_kind?: string
  default_publication_id?: number
  default_publication_version_no?: string | number
  specs: EmployeeOrderProductSpec[]
}

export type EmployeeOrder = {
  id: number
  order_no: string
  order_date: string
  customer: string
  grand_total: string
  pay_status: string
  ship_status: string
  process_status: string
  responsible_name: string
}

export type EmployeeOrderDetailItem = {
  id?: number
  item_id?: number
  line_no?: number
  product_id: number
  customer_product_alias_id?: number
  product_name: string
  customer_product_display_name_snapshot?: string
  customer_item_code_snapshot?: string
  brand_name_snapshot?: string
  product_code_snapshot?: string
  product_name_snapshot?: string
  note?: string
  spec?: string
  qty: string
  unit?: string
  unit_price: string
  line_total: string
  price_override?: boolean
  bean_list_publication_id?: number
  bean_list_version_no?: string
  product_kind?: string
  sales_unit?: string
  unit_bag_count?: number
  unit_bean_g?: string
  matched_price_qty?: string
  unit_conversion_label?: string
  price_source_json?: string
  spec_g?: number | string
  discount_type?: string
  discount_value?: string | number
  discount_amount?: string | number
}

export type EmployeeOrderTrace = {
  product_id?: number
  product_name?: string
  productName?: string
  tier_label?: string
  tierLabel?: string
  price_list_publication_id?: number
  price_list_version?: string
  final_unit_price?: string | number
  price_unit?: string
  pricing_rule_version?: string
  manual_adjusted?: boolean
  source_label?: string
  bom_version_no?: string
  process_route_name?: string
  process_card_no?: string
  work_order_no?: string
  material_batch_no?: string
}

export type EmployeeOrderAsset = {
  id: number
  kind: string
  filename: string
  content_type: string
  bytes: number
  created_at: string
  created_by: string
  url?: string
}

export type EmployeeOrderDetail = EmployeeOrder & {
  edit_revision?: string
  document_date?: string
  customer_id?: number
  source_id?: number
  source?: string
  order_type_id?: number
  order_type?: string
  pay_status_id?: number
  payment_method?: string
  ship_status_id?: number
  process_status_id?: number
  invoice_status?: string
  receiver_name?: string
  receiver_phone?: string
  receiver_company?: string
  receiver_address?: string
  ship_method?: string
  ship_tracking_no?: string
  logistics_company_id?: number
  logistics_company?: string
  logistics_product_id?: number
  logistics_product?: string
  sender_id?: number
  sender_label?: string
  sender_name?: string
  payment_goods_amount?: string
  payment_shipping_amount?: string
  payment_voucher_asset_id?: number
  payment_voucher?: EmployeeOrderAsset
  responsible_type?: string
  responsible_id?: number
  portal_service_code?: string
  source_warehouse?: string
  bean_list_publication_id?: number
  bean_list_version_no?: string
  total_amount?: string
  shipping_amount?: string
  discount_amount?: string
  order_discount_amount?: string | number
  rounding_amount?: string
  round_to_int?: boolean
  express_fee?: string
  outsource_material_fee?: string
  outsource_roast_fee?: string
  outsource_packaging_fee?: string
  outsource_manual_fee?: string
  outsource_tax_fee?: string
  outsource_other_fee?: string
  outsource_total_fee?: string
  created_by_employee?: string
  notes?: string
  is_void?: boolean
  voided_at?: string
  void_reason?: string
  invoice_filename?: string
  invoice_file_url?: string
  items: EmployeeOrderDetailItem[]
  quote_source_trace?: EmployeeOrderTrace[]
  production_source_trace?: EmployeeOrderTrace[]
  can_edit?: boolean
  edit_block_reason?: string
}

export type EmployeeOrderDocumentKind = 'sales-order' | 'delivery-note'
export type EmployeeOrderDocumentFormat = 'pdf' | 'png'

export type EmployeeOrderDocumentAsset = {
  available?: boolean
  version_no?: number | string
  filename?: string
  content_type?: string
  generated?: boolean
  download_url?: string
  url?: string
  path?: string
}

export type EmployeeOrderDocumentGroup = {
  pdf?: EmployeeOrderDocumentAsset
  png?: EmployeeOrderDocumentAsset
}

export type EmployeeOrderDocuments = {
  sales_order?: EmployeeOrderDocumentGroup
  delivery_note?: EmployeeOrderDocumentGroup
  sales_order_pdf?: EmployeeOrderDocumentAsset
  sales_order_png?: EmployeeOrderDocumentAsset
  delivery_note_pdf?: EmployeeOrderDocumentAsset
  delivery_note_png?: EmployeeOrderDocumentAsset
}

export type EmployeeOrderDetailResponse = {
  order: EmployeeOrderDetail
  documents?: EmployeeOrderDocuments
  can_edit?: boolean
  edit_block_reason?: string
}

export type EmployeeOrderDocumentGenerateResponse = EmployeeOrderDocumentAsset & {
  document?: EmployeeOrderDocumentAsset
  asset?: EmployeeOrderDocumentAsset
}

export type EmployeeShareSettings = {
  image_need_show_entrance: boolean
}

export type EmployeeShareSettingsResponse = {
  settings: EmployeeShareSettings
  can_manage: boolean
}

const employeeOrderDocumentFiles: Record<
  EmployeeOrderDocumentKind,
  Record<EmployeeOrderDocumentFormat, string>
> = {
  'sales-order': { pdf: 'sales-order.pdf', png: 'sales-order.png' },
  'delivery-note': { pdf: 'delivery-note.pdf', png: 'delivery-note.png' },
}

export function fetchEmployeeOrderForm(token: string, customerID = 0): Promise<EmployeeOrderForm> {
  return miniRequest<EmployeeOrderForm>(buildEmployeeOrderFormPath(customerID), { token })
}

export function fetchEmployeeOrders(token: string, q = ''): Promise<{ rows: EmployeeOrder[]; has_next: boolean }> {
  const suffix = q.trim() ? `?q=${encodeURIComponent(q.trim())}` : ''
  return miniRequest(`${buildEmployeeOrdersPath()}${suffix}`, { token })
}

export function fetchEmployeeOrderDetail(token: string, orderID: number): Promise<EmployeeOrderDetailResponse> {
  return miniRequest<EmployeeOrderDetailResponse>(buildEmployeeOrderDetailPath(orderID), { token })
}

export function fetchEmployeeShareSettings(token: string): Promise<EmployeeShareSettingsResponse> {
  return miniRequest<EmployeeShareSettingsResponse>('/api/mini/employee/share-settings', { token })
}

export function saveEmployeeShareSettings(token: string, imageNeedShowEntrance: boolean): Promise<EmployeeShareSettingsResponse> {
  return miniRequest<EmployeeShareSettingsResponse>('/api/mini/employee/share-settings', {
    method: 'PUT',
    token,
    data: { image_need_show_entrance: imageNeedShowEntrance },
  })
}

export function generateEmployeeOrderDocument(
  token: string,
  orderID: number,
  kind: EmployeeOrderDocumentKind,
  format: EmployeeOrderDocumentFormat,
): Promise<EmployeeOrderDocumentGenerateResponse> {
  return miniRequest<EmployeeOrderDocumentGenerateResponse>(buildEmployeeOrderDocumentPath(orderID, kind, format), {
    method: 'POST',
    token,
    data: {},
  })
}

export function createEmployeeOrder(token: string, data: Record<string, unknown>): Promise<{ order_id: number; order_no: string }> {
  return miniRequest(buildEmployeeOrdersPath(), { method: 'POST', token, data })
}

export function updateEmployeeOrder(
  token: string,
  orderID: number,
  data: Record<string, unknown>,
): Promise<{ order_id: number; order_no: string }> {
  return miniRequest(buildEmployeeOrderUpdatePath(orderID), { method: 'PUT', token, data })
}

export function fetchEmployeeCustomers(token: string, query: EmployeeCustomerListQuery = {}): Promise<EmployeeCustomersResponse> {
  return miniRequest<EmployeeCustomersResponse>(buildEmployeeCustomersPath(query), { token })
}

export function fetchEmployeeCustomer(token: string, customerID: number): Promise<{ customer: EmployeeCustomer }> {
  return miniRequest<{ customer: EmployeeCustomer }>(buildEmployeeCustomerPath(customerID), { token })
}

export function parseEmployeeCustomerRecipient(
  token: string,
  text: string,
): Promise<EmployeeCustomerRecipientParseResponse> {
  return miniRequest<EmployeeCustomerRecipientParseResponse>('/api/customer-recipient/parse', {
    method: 'POST',
    token,
    data: { text: String(text || '').trim() },
  })
}

export function createEmployeeCustomer(token: string, data: EmployeeCustomerPayload): Promise<{ customer: EmployeeCustomer }> {
  return miniRequest<{ customer: EmployeeCustomer }>(buildEmployeeCustomersPath(), { method: 'POST', token, data })
}

export function updateEmployeeCustomer(token: string, customerID: number, data: EmployeeCustomerPayload): Promise<{ customer: EmployeeCustomer }> {
  return miniRequest<{ customer: EmployeeCustomer }>(buildEmployeeCustomerPath(customerID), { method: 'PUT', token, data })
}

export function fetchEmployeeOrderDraft(token: string): Promise<{ draft: EmployeeOrderDraft | null }> {
  return miniRequest<{ draft: EmployeeOrderDraft | null }>(buildEmployeeOrderDraftPath(), { token })
}

export function saveEmployeeOrderDraft(token: string, payload: EmployeeOrderDraftPayload): Promise<{ draft: EmployeeOrderDraft }> {
  return miniRequest<{ draft: EmployeeOrderDraft }>(buildEmployeeOrderDraftPath(), {
    method: 'PUT',
    token,
    data: { payload },
  })
}

export function deleteEmployeeOrderDraft(token: string): Promise<{ deleted: boolean }> {
  return miniRequest<{ deleted: boolean }>(buildEmployeeOrderDraftPath(), { method: 'DELETE', token })
}

export function buildEmployeeOrderFormPath(customerID = 0): string {
  const id = Number(customerID || 0)
  return `/api/mini/employee/order-form${id > 0 ? `?customer_id=${id}` : ''}`
}

export function buildEmployeeOrdersPath(): string {
  return '/api/mini/employee/orders'
}

export function buildEmployeeOrderDetailPath(orderID: number): string {
  return `${buildEmployeeOrdersPath()}/${Number(orderID || 0)}`
}

export function buildEmployeeOrderUpdatePath(orderID: number): string {
  return buildEmployeeOrderDetailPath(orderID)
}

export function buildEmployeeOrderDocumentPath(
  orderID: number,
  kind: EmployeeOrderDocumentKind,
  format: EmployeeOrderDocumentFormat,
): string {
  return `${buildEmployeeOrderDetailPath(orderID)}/documents/${employeeOrderDocumentFiles[kind][format]}`
}

export function buildEmployeeCustomersPath(query: EmployeeCustomerListQuery = {}): string {
  const params = [
    ['q', String(query.q || '').trim()],
    ['page', Number(query.page || 0) > 0 ? String(Number(query.page)) : ''],
    ['limit', Number(query.limit || 0) > 0 ? String(Number(query.limit)) : ''],
  ]
    .filter(([, value]) => value !== '')
    .map(([name, value]) => `${name}=${encodeURIComponent(value)}`)
  return `/api/mini/employee/customers${params.length ? `?${params.join('&')}` : ''}`
}

export function buildEmployeeCustomerPath(customerID: number): string {
  return `${buildEmployeeCustomersPath()}/${Number(customerID || 0)}`
}

export function buildEmployeeOrderDraftPath(): string {
  return '/api/mini/employee/order-draft'
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

export function buildDirectShipCatalogPath(): string {
  return '/api/mini/direct-ship/catalog'
}

export function buildDirectShipRequestsPath(): string {
  return '/api/mini/direct-ship/requests'
}

export function buildDirectShipRequestDetailPath(requestID: number): string {
  return `${buildDirectShipRequestsPath()}/${Number(requestID || 0)}`
}

export function buildProcessingRequestsPath(): string {
  return '/api/mini/processing-requests'
}

export function buildProcessingCatalogPath(): string {
  return '/api/mini/processing/catalog'
}

export function buildProcessingRequestDetailPath(requestID: number): string {
  return `${buildProcessingRequestsPath()}/${Number(requestID || 0)}`
}

export function buildCustomerInventoryPath(): string {
  return '/api/mini/customer-inventory'
}

export function buildCustomerInventoryBatchesPath(productID: number, specG = 0): string {
  const path = `${buildCustomerInventoryPath()}/${Number(productID || 0)}/batches`
  return Number(specG || 0) > 0 ? `${path}?spec_g=${Number(specG)}` : path
}

export function buildCustomerBillsPath(): string {
  return '/api/mini/customer-bills'
}

export function buildCustomerBillDetailPath(billID: number): string {
  return `${buildCustomerBillsPath()}/${Number(billID || 0)}`
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

export function buildBeanListPDFPath(publicationID: number): string {
  return `/api/mini/bean-lists/${Number(publicationID || 0)}.pdf`
}

export function buildBeanListPNGPath(publicationID: number): string {
  return `/api/mini/bean-lists/${Number(publicationID || 0)}.png`
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
  return miniRequest<ProcessingRequest>(buildProcessingRequestsPath(), {
    method: 'POST',
    token,
    data: payload,
  })
}

export function previewProcessingRequest(
  token: string,
  payload: CreateProcessingRequestPayload,
): Promise<ProcessingRequestPreview> {
  return miniRequest<ProcessingRequestPreview>(`${buildProcessingRequestsPath()}/preview`, {
    method: 'POST',
    token,
    data: payload,
  })
}

export function fetchProcessingRequests(token: string): Promise<{ rows: ProcessingRequest[] }> {
  return miniRequest<{ rows: ProcessingRequest[] }>(buildProcessingRequestsPath(), { token })
}

export function fetchProcessingCatalog(token: string): Promise<DirectShipCatalog> {
  return miniRequest<DirectShipCatalog>(buildProcessingCatalogPath(), { token })
}

export function fetchProcessingRequestDetail(token: string, requestID: number): Promise<{ request: ProcessingRequest }> {
  return miniRequest<{ request: ProcessingRequest }>(buildProcessingRequestDetailPath(requestID), { token })
}

export function fetchDirectShipCatalog(token: string): Promise<DirectShipCatalog> {
  return miniRequest<DirectShipCatalog>(buildDirectShipCatalogPath(), { token })
}

export function previewDirectShipRequest(token: string, payload: DirectShipRequestPayload): Promise<DirectShipPreview> {
  return miniRequest<DirectShipPreview>('/api/mini/direct-ship/preview', {
    method: 'POST',
    token,
    data: payload,
  })
}

export function createDirectShipRequest(token: string, payload: DirectShipRequestPayload): Promise<DirectShipRequest> {
  return miniRequest<DirectShipRequest>(buildDirectShipRequestsPath(), {
    method: 'POST',
    token,
    data: payload,
  })
}

export function fetchDirectShipRequests(token: string): Promise<{ rows: DirectShipRequest[] }> {
  return miniRequest<{ rows: DirectShipRequest[] }>(buildDirectShipRequestsPath(), { token })
}

export function fetchDirectShipRequestDetail(token: string, requestID: number): Promise<{ request: DirectShipRequest }> {
  return miniRequest<{ request: DirectShipRequest }>(buildDirectShipRequestDetailPath(requestID), { token })
}

export function cancelDirectShipRequest(token: string, requestID: number): Promise<DirectShipRequest> {
  return miniRequest<DirectShipRequest>(`${buildDirectShipRequestDetailPath(requestID)}/cancel`, {
    method: 'POST',
    token,
  })
}

export function fetchCustomerInventory(token: string): Promise<{ rows: CustomerInventorySummary[] }> {
  return miniRequest<{ rows: CustomerInventorySummary[] }>(buildCustomerInventoryPath(), { token })
}

export function fetchCustomerInventoryBatches(
  token: string,
  productID: number,
  specG = 0,
): Promise<{ rows: CustomerInventoryBatch[] }> {
  return miniRequest<{ rows: CustomerInventoryBatch[] }>(buildCustomerInventoryBatchesPath(productID, specG), { token })
}

export function fetchCustomerBills(token: string): Promise<{ rows: CustomerBillSummary[] }> {
  return miniRequest<{ rows: CustomerBillSummary[] }>(buildCustomerBillsPath(), { token })
}

export function fetchCustomerBillDetail(token: string, billID: number): Promise<{ bill: CustomerBillDetail }> {
  return miniRequest<{ bill: CustomerBillDetail }>(buildCustomerBillDetailPath(billID), { token })
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
