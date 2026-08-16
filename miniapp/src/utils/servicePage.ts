import type { CreateFulfillmentOrderPayload, ProductSummary, SalesUnit } from '../api/customerPortal'

export type ServiceKey =
  | 'beanList'
  | 'orders'
  | 'productOrder'
  | 'directShip'
  | 'processing'
  | 'inventory'
  | 'settlement'

export type ServicePayload = {
  key: ServiceKey | string
  title: string
  capability?: string
  summary?: Array<{ label: string; value: string }>
  bean_lists?: unknown[]
  products?: unknown[]
  orders?: unknown[]
  direct_ship_batches?: unknown[]
  inventory?: unknown[]
  processing_requests?: unknown[]
  fee_items?: unknown[]
  settlement_batches?: unknown[]
}

export type ServiceSection = {
  title: string
  count: number
}

export type FulfillmentSalesUnitOption = {
  sales_unit: SalesUnit
  label: string
  unit_bag_count: number
  unit_bean_g: number
  spec_g: number
  quantity_label: string
}

export type FulfillmentOrderForm = {
  recipient_name: string
  recipient_phone: string
  recipient_address: string
  recipient_company?: string
  product_id: number
  bom_spec_id?: number
  bom_variant_id?: number
  inventory_unit?: string
  product_name?: string
  spec_g: number
  qty: number
  sales_unit?: SalesUnit | string
  unit_bag_count?: number
  unit_bean_g?: number
  note?: string
}

type FulfillmentProductLike = Partial<ProductSummary>

const labels: Record<ServiceKey, string> = {
  beanList: '我的商品',
  orders: '订单中心',
  productOrder: '现货下单',
  directShip: '一件代发',
  processing: '生产工单',
  inventory: '我的库存',
  settlement: '费用中心',
}

const capabilities: Record<ServiceKey, string> = {
  beanList: 'bean_list',
  orders: 'product_order',
  productOrder: 'product_order',
  directShip: 'direct_ship',
  processing: 'processing',
  inventory: 'inventory_custody',
  settlement: 'settlement',
}

export function normalizeServiceKey(value: string): ServiceKey {
  if (value === 'shipping' || value === 'shipping_query') return 'orders'
  if (value in labels) return value as ServiceKey
  return 'beanList'
}

export function serviceTitle(key: ServiceKey | string): string {
  return labels[normalizeServiceKey(String(key))]
}

export function serviceCapability(key: ServiceKey | string): string {
  return capabilities[normalizeServiceKey(String(key))]
}

export function visibleServiceSections(payload: ServicePayload): ServiceSection[] {
  const sections: ServiceSection[] = []
  const key = normalizeServiceKey(String(payload.key))
  if (key === 'directShip') return sections
  if (key === 'processing') {
    addSection(sections, '生产工单', payload.processing_requests)
    return sections
  }
  if (key === 'settlement') {
    addSection(sections, '账单', payload.settlement_batches)
    return sections
  }
  addSection(sections, '商品价格表', payload.bean_lists)
  addSection(sections, '现货商品', payload.products)
  addSection(sections, orderSectionTitle(key), payload.orders)
  addSection(sections, '一件代发批次', payload.direct_ship_batches)
  addSection(sections, '库存', payload.inventory)
  addSection(sections, '加工申请', payload.processing_requests)
  addSection(sections, '费用明细', payload.fee_items)
  addSection(sections, '结算单', payload.settlement_batches)
  return sections
}

export function orderSectionTitle(key: ServiceKey): string {
  if (key === 'orders') return '我的订单'
  if (key === 'settlement') return '账单'
  return '订单 / 物流'
}

export function fulfillmentSalesUnitOptions(product?: FulfillmentProductLike | null): FulfillmentSalesUnitOption[] {
	if (positiveInteger(product?.bom_spec_id)) return []
  if (product?.product_kind !== 'drip_bag') return []
  const bagGrams = positiveNumber(product.drip_bag_grams) || 10
  const boxBagCount = positiveInteger(product.drip_box_bag_count) || 10
  return normalizeSalesUnits(product.sales_units).map((salesUnit) => {
    const unitBagCount = salesUnit === 'box' ? boxBagCount : 1
    return {
      sales_unit: salesUnit,
      label: salesUnit === 'box' ? '盒' : '袋',
      unit_bag_count: unitBagCount,
      unit_bean_g: bagGrams,
      spec_g: bagGrams * unitBagCount,
      quantity_label: salesUnit === 'box' ? '盒数' : '袋数',
    }
  })
}

export function fulfillmentUnitOption(
  product?: FulfillmentProductLike | null,
  salesUnit?: SalesUnit | string,
): FulfillmentSalesUnitOption | null {
  const options = fulfillmentSalesUnitOptions(product)
  if (!options.length) return null
  return options.find((item) => item.sales_unit === salesUnit) || options[0]
}

export function buildFulfillmentOrderPayload(
  serviceCode: 'direct_ship' | 'processing_ship' | 'product_order' | string,
  form: FulfillmentOrderForm,
): CreateFulfillmentOrderPayload {
  const payload: CreateFulfillmentOrderPayload = {
    service_code: serviceCode,
    recipient_name: String(form.recipient_name || '').trim(),
    recipient_phone: String(form.recipient_phone || '').trim(),
    recipient_address: String(form.recipient_address || '').trim(),
    recipient_company: String(form.recipient_company || '').trim(),
    product_id: Number(form.product_id) || 0,
    product_name: String(form.product_name || '').trim(),
    spec_g: Number(form.spec_g) || 0,
    qty: Number(form.qty) || 0,
    note: String(form.note || ''),
  }
	const bomSpecID = positiveInteger(form.bom_spec_id)
	const bomVariantID = positiveInteger(form.bom_variant_id)
	const inventoryUnit = String(form.inventory_unit || '').trim()
	if (bomSpecID || bomVariantID) {
		payload.bom_spec_id = bomSpecID
		payload.bom_variant_id = bomVariantID
		payload.inventory_unit = inventoryUnit
		payload.spec_g = 0
		return payload
	}
  const salesUnit = normalizeSalesUnit(form.sales_unit)
  if (salesUnit) {
    payload.sales_unit = salesUnit
    payload.unit_bag_count = positiveInteger(form.unit_bag_count) || (salesUnit === 'box' ? 10 : 1)
    payload.unit_bean_g = positiveNumber(form.unit_bean_g) || 10
  }
  return payload
}

function addSection(sections: ServiceSection[], title: string, rows: unknown[] | undefined) {
  if (rows?.length) {
    sections.push({ title, count: rows.length })
  }
}

function normalizeSalesUnits(values?: SalesUnit[]): SalesUnit[] {
  const out: SalesUnit[] = []
  for (const value of values || ['bag', 'box']) {
    const normalized = normalizeSalesUnit(value)
    if (normalized && !out.includes(normalized)) out.push(normalized)
  }
  return out.length ? out : ['bag', 'box']
}

function normalizeSalesUnit(value?: SalesUnit | string): SalesUnit | '' {
  return value === 'bag' || value === 'box' ? value : ''
}

function positiveNumber(value: unknown): number {
  const n = Number(value || 0)
  return Number.isFinite(n) && n > 0 ? n : 0
}

function positiveInteger(value: unknown): number {
  const n = Number(value || 0)
  return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0
}
