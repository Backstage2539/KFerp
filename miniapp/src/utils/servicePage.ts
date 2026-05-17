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

const labels: Record<ServiceKey, string> = {
  beanList: '我的豆单',
  orders: '我的订单',
  productOrder: '现货下单',
  directShip: '一件代发',
  processing: '代加工',
  inventory: '我的库存',
  settlement: '结算中心',
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
  addSection(sections, '豆单', payload.bean_lists)
  addSection(sections, '现货商品', payload.products)
  addSection(sections, orderSectionTitle(normalizeServiceKey(String(payload.key))), payload.orders)
  addSection(sections, '一件代发批次', payload.direct_ship_batches)
  addSection(sections, '库存', payload.inventory)
  addSection(sections, '加工申请', payload.processing_requests)
  addSection(sections, '费用明细', payload.fee_items)
  addSection(sections, '结算单', payload.settlement_batches)
  return sections
}

export function orderSectionTitle(key: ServiceKey): string {
  if (key === 'orders') return '我的订单'
  if (key === 'settlement') return '订单账单'
  return '订单 / 物流'
}

function addSection(sections: ServiceSection[], title: string, rows: unknown[] | undefined) {
  if (rows?.length) {
    sections.push({ title, count: rows.length })
  }
}
