export type Capability = {
  code: string
  enabled: boolean
}

export type HomeEntry = {
  key: string
  label: string
  capability: string
}

const entries: HomeEntry[] = [
  { key: 'beanList', label: '我的豆单', capability: 'bean_list' },
  { key: 'productOrder', label: '现货下单', capability: 'product_order' },
  { key: 'directShip', label: '一件代发', capability: 'direct_ship' },
  { key: 'processing', label: '代加工', capability: 'processing' },
  { key: 'inventory', label: '我的库存', capability: 'inventory_custody' },
  { key: 'shipping', label: '物流查询', capability: 'shipping_query' },
  { key: 'settlement', label: '结算中心', capability: 'settlement' },
]

export function visibleHomeEntries(capabilities: Capability[] = []): HomeEntry[] {
  const enabled = new Set(capabilities.filter((item) => item.enabled).map((item) => item.code))
  return entries.filter((entry) => enabled.has(entry.capability))
}
